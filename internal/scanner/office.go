package scanner

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
)

const maximumOfficeXMLBytes = 32 << 20

// extractOfficeText reads the XML payload of modern Office documents. It never
// runs formulas, macros, embedded objects or external converters.
func extractOfficeText(data []byte, extension string, maximum int64) string {
	if maximum <= 0 {
		return ""
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	if extension == ".xlsx" {
		return extractXLSXText(archive, maximum)
	}
	entries := make([]*zip.File, 0)
	for _, file := range archive.File {
		if !officeTextEntry(extension, file.Name) || file.FileInfo().IsDir() || file.UncompressedSize64 > maximumOfficeXMLBytes {
			continue
		}
		entries = append(entries, file)
	}
	sort.SliceStable(entries, func(left, right int) bool {
		return officeEntryRank(entries[left].Name) < officeEntryRank(entries[right].Name)
	})
	var output strings.Builder
	for _, file := range entries {
		reader, err := file.Open()
		if err != nil {
			continue
		}
		appendOfficeXMLText(&output, io.LimitReader(reader, maximumOfficeXMLBytes+1), maximum)
		_ = reader.Close()
		if int64(output.Len()) >= maximum {
			break
		}
	}
	return truncateUTF8(strings.Join(strings.Fields(output.String()), " "), maximum)
}

func officeEntryRank(name string) string {
	name = path.Clean(strings.ReplaceAll(name, "\\", "/"))
	switch name {
	case "word/document.xml", "content.xml":
		return "0-" + name
	case "word/footnotes.xml":
		return "1-" + name
	case "word/endnotes.xml":
		return "2-" + name
	default:
		return "3-" + name
	}
}

func officeTextEntry(extension, name string) bool {
	name = path.Clean(strings.ReplaceAll(name, "\\", "/"))
	switch extension {
	case ".docx":
		return name == "word/document.xml" || name == "word/footnotes.xml" || name == "word/endnotes.xml" ||
			strings.HasPrefix(name, "word/header") && strings.HasSuffix(name, ".xml") ||
			strings.HasPrefix(name, "word/footer") && strings.HasSuffix(name, ".xml")
	case ".odt", ".ods":
		return name == "content.xml"
	default:
		return false
	}
}

func extractXLSXText(archive *zip.Reader, maximum int64) string {
	var sharedFile *zip.File
	worksheets := make([]*zip.File, 0)
	for _, file := range archive.File {
		name := path.Clean(strings.ReplaceAll(file.Name, "\\", "/"))
		if file.FileInfo().IsDir() || file.UncompressedSize64 > maximumOfficeXMLBytes {
			continue
		}
		if name == "xl/sharedStrings.xml" {
			sharedFile = file
		} else if strings.HasPrefix(name, "xl/worksheets/sheet") && strings.HasSuffix(name, ".xml") {
			worksheets = append(worksheets, file)
		}
	}
	shared := readXLSXSharedStrings(sharedFile)
	sort.SliceStable(worksheets, func(left, right int) bool { return worksheets[left].Name < worksheets[right].Name })
	var output strings.Builder
	for _, worksheet := range worksheets {
		reader, err := worksheet.Open()
		if err != nil {
			continue
		}
		appendXLSXWorksheetText(&output, io.LimitReader(reader, maximumOfficeXMLBytes+1), shared, maximum)
		_ = reader.Close()
		if int64(output.Len()) >= maximum {
			break
		}
	}
	return truncateUTF8(strings.Join(strings.Fields(output.String()), " "), maximum)
}

func readXLSXSharedStrings(file *zip.File) []string {
	if file == nil {
		return nil
	}
	reader, err := file.Open()
	if err != nil {
		return nil
	}
	defer reader.Close()
	decoder := xml.NewDecoder(io.LimitReader(reader, maximumOfficeXMLBytes+1))
	result := make([]string, 0)
	var current strings.Builder
	inItem, inText := false, false
	for {
		token, err := decoder.Token()
		if err != nil {
			return result
		}
		switch value := token.(type) {
		case xml.StartElement:
			switch value.Name.Local {
			case "si":
				inItem = true
				current.Reset()
			case "t":
				inText = inItem
			}
		case xml.CharData:
			if inText {
				current.Write(value)
			}
		case xml.EndElement:
			switch value.Name.Local {
			case "t":
				inText = false
			case "si":
				result = append(result, strings.Join(strings.Fields(current.String()), " "))
				inItem = false
			}
		}
	}
}

func appendXLSXWorksheetText(output *strings.Builder, reader io.Reader, shared []string, maximum int64) {
	decoder := xml.NewDecoder(reader)
	var value, inline strings.Builder
	cellType := ""
	inCell, inValue, inInlineText := false, false, false
	for int64(output.Len()) < maximum {
		token, err := decoder.Token()
		if err != nil {
			return
		}
		switch token := token.(type) {
		case xml.StartElement:
			switch token.Name.Local {
			case "c":
				inCell = true
				cellType = ""
				value.Reset()
				inline.Reset()
				for _, attribute := range token.Attr {
					if attribute.Name.Local == "t" {
						cellType = attribute.Value
					}
				}
			case "v":
				inValue = inCell
			case "t":
				inInlineText = inCell && cellType == "inlineStr"
			}
		case xml.CharData:
			if inValue {
				value.Write(token)
			}
			if inInlineText {
				inline.Write(token)
			}
		case xml.EndElement:
			switch token.Name.Local {
			case "v":
				inValue = false
			case "t":
				inInlineText = false
			case "c":
				appendIndexedText(output, xlsxCellText(cellType, value.String(), inline.String(), shared), maximum)
				inCell = false
			}
		}
	}
}

func xlsxCellText(cellType, rawValue, inline string, shared []string) string {
	rawValue = strings.TrimSpace(rawValue)
	switch cellType {
	case "s":
		index, err := strconv.Atoi(rawValue)
		if err != nil || index < 0 || index >= len(shared) {
			return ""
		}
		return shared[index]
	case "inlineStr":
		return inline
	case "b":
		if rawValue == "1" {
			return "WAHR"
		}
		if rawValue == "0" {
			return "FALSCH"
		}
	}
	return rawValue
}

func appendIndexedText(output *strings.Builder, value string, maximum int64) {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" || int64(output.Len()) >= maximum {
		return
	}
	if output.Len() > 0 {
		output.WriteByte(' ')
	}
	output.WriteString(truncateUTF8(value, maximum-int64(output.Len())))
}

func appendOfficeXMLText(output *strings.Builder, reader io.Reader, maximum int64) {
	decoder := xml.NewDecoder(reader)
	for int64(output.Len()) < maximum {
		token, err := decoder.Token()
		if err != nil {
			return
		}
		switch value := token.(type) {
		case xml.CharData:
			text := strings.TrimSpace(string(value))
			if text == "" {
				continue
			}
			if output.Len() > 0 {
				output.WriteByte(' ')
			}
			output.WriteString(truncateUTF8(text, maximum-int64(output.Len())))
		case xml.StartElement:
			if value.Name.Local == "tab" && int64(output.Len()) < maximum {
				output.WriteByte(' ')
			}
		}
	}
}
