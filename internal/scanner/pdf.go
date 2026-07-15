package scanner

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"io"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const maximumPDFDecodedBytes = 32 << 20

// extractPDFText handles literal and hexadecimal strings in PDF text objects.
// It intentionally does not execute external tools, interpret JavaScript or
// attempt OCR. Font-specific encodings without a Unicode representation may
// therefore yield incomplete text.
func extractPDFText(data []byte, maximum int64) string {
	if len(data) < 5 || !bytes.HasPrefix(data, []byte("%PDF-")) || bytes.Contains(data, []byte("/Encrypt")) || maximum <= 0 {
		return ""
	}
	parseLimit := maximumPDFDecodedBytes
	if maximum < int64(parseLimit) {
		parseLimit = int(maximum)
	}
	payloads := pdfStreams(data, parseLimit)
	if len(payloads) == 0 {
		payloads = [][]byte{data}
	}
	var output strings.Builder
	for _, payload := range payloads {
		appendPDFTextObjects(&output, payload, maximum)
		if int64(output.Len()) >= maximum {
			break
		}
	}
	return truncateUTF8(strings.Join(strings.Fields(output.String()), " "), maximum)
}

func pdfStreams(data []byte, maximum int) [][]byte {
	result := make([][]byte, 0)
	for offset := 0; offset < len(data); {
		relative := bytes.Index(data[offset:], []byte("stream"))
		if relative < 0 {
			break
		}
		marker := offset + relative
		start := marker + len("stream")
		if start < len(data) && data[start] == '\r' {
			start++
		}
		if start < len(data) && data[start] == '\n' {
			start++
		}
		endRelative := bytes.Index(data[start:], []byte("endstream"))
		if endRelative < 0 {
			break
		}
		end := start + endRelative
		stream := bytes.TrimRight(data[start:end], "\r\n")
		dictionaryStart := marker - 4096
		if dictionaryStart < 0 {
			dictionaryStart = 0
		}
		dictionary := data[dictionaryStart:marker]
		if start := bytes.LastIndex(dictionary, []byte("<<")); start >= 0 {
			dictionary = dictionary[start:]
		}
		if bytes.Contains(dictionary, []byte("/FlateDecode")) {
			if reader, err := zlib.NewReader(bytes.NewReader(stream)); err == nil {
				decoded, readErr := io.ReadAll(io.LimitReader(reader, int64(maximum)+1))
				_ = reader.Close()
				if readErr == nil && len(decoded) <= maximum {
					result = append(result, decoded)
				}
			}
		} else if len(stream) <= maximum {
			result = append(result, stream)
		}
		offset = end + len("endstream")
	}
	return result
}

func appendPDFTextObjects(output *strings.Builder, data []byte, maximum int64) {
	for offset := 0; offset < len(data) && int64(output.Len()) < maximum; {
		begin := bytes.Index(data[offset:], []byte("BT"))
		if begin < 0 {
			return
		}
		begin += offset + 2
		endRelative := bytes.Index(data[begin:], []byte("ET"))
		if endRelative < 0 {
			return
		}
		section := data[begin : begin+endRelative]
		for index := 0; index < len(section) && int64(output.Len()) < maximum; {
			switch section[index] {
			case '(':
				value, next := parsePDFLiteral(section, index+1)
				appendPDFString(output, value, maximum)
				index = next
			case '<':
				if index+1 < len(section) && section[index+1] == '<' {
					index += 2
					continue
				}
				end := bytes.IndexByte(section[index+1:], '>')
				if end < 0 {
					index = len(section)
					continue
				}
				raw := bytes.Map(func(value rune) rune {
					if unicode.IsSpace(value) {
						return -1
					}
					return value
				}, section[index+1:index+1+end])
				if len(raw)%2 == 1 {
					raw = append(raw, '0')
				}
				decoded := make([]byte, hex.DecodedLen(len(raw)))
				if _, err := hex.Decode(decoded, raw); err == nil {
					appendPDFString(output, decoded, maximum)
				}
				index += end + 2
			default:
				index++
			}
		}
		offset = begin + endRelative + 2
	}
}

func parsePDFLiteral(data []byte, index int) ([]byte, int) {
	result := make([]byte, 0, 64)
	depth := 1
	for index < len(data) && depth > 0 {
		value := data[index]
		index++
		if value == '\\' && index < len(data) {
			escaped := data[index]
			index++
			switch escaped {
			case 'n':
				result = append(result, '\n')
			case 'r':
				result = append(result, '\r')
			case 't':
				result = append(result, '\t')
			case 'b':
				result = append(result, '\b')
			case 'f':
				result = append(result, '\f')
			case '\r':
				if index < len(data) && data[index] == '\n' {
					index++
				}
			case '\n':
			default:
				if escaped >= '0' && escaped <= '7' {
					octal := int(escaped - '0')
					for count := 1; count < 3 && index < len(data) && data[index] >= '0' && data[index] <= '7'; count++ {
						octal = octal*8 + int(data[index]-'0')
						index++
					}
					result = append(result, byte(octal))
				} else {
					result = append(result, escaped)
				}
			}
			continue
		}
		if value == '(' {
			depth++
		} else if value == ')' {
			depth--
			if depth == 0 {
				break
			}
		}
		result = append(result, value)
	}
	return result, index
}

func appendPDFString(output *strings.Builder, raw []byte, maximum int64) {
	if len(raw) == 0 || int64(output.Len()) >= maximum {
		return
	}
	value := decodePDFString(raw)
	value = strings.Join(strings.FieldsFunc(value, func(r rune) bool { return unicode.IsControl(r) || unicode.IsSpace(r) }), " ")
	if value == "" {
		return
	}
	if output.Len() > 0 {
		output.WriteByte(' ')
	}
	remaining := maximum - int64(output.Len())
	output.WriteString(truncateUTF8(value, remaining))
}

func decodePDFString(raw []byte) string {
	if len(raw) >= 2 && raw[0] == 0xfe && raw[1] == 0xff {
		return decodeUTF16BE(raw[2:])
	}
	// Some PDF producers omit the UTF-16BE byte-order mark. Recognize the
	// common ASCII-range representation by its zero high bytes before the
	// generic UTF-8 branch sees those zeroes as control characters.
	if len(raw) >= 4 && len(raw)%2 == 0 {
		zeroHighBytes := 0
		for index := 0; index < len(raw); index += 2 {
			if raw[index] == 0 {
				zeroHighBytes++
			}
		}
		if zeroHighBytes*4 >= len(raw) {
			return decodeUTF16BE(raw)
		}
	}
	if utf8.Valid(raw) {
		return string(raw)
	}
	runes := make([]rune, len(raw))
	for index, value := range raw {
		runes[index] = rune(value)
	}
	return string(runes)
}

func decodeUTF16BE(raw []byte) string {
	units := make([]uint16, 0, len(raw)/2)
	for index := 0; index+1 < len(raw); index += 2 {
		units = append(units, uint16(raw[index])<<8|uint16(raw[index+1]))
	}
	return string(utf16.Decode(units))
}
