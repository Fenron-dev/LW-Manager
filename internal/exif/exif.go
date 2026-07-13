package exif

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

type Metadata struct {
	Manufacturer string `json:"manufacturer,omitempty"`
	Camera       string `json:"camera,omitempty"`
	CapturedAt   string `json:"capturedAt,omitempty"`
	Lens         string `json:"lens,omitempty"`
	Orientation  int    `json:"orientation,omitempty"`
}

func ParseJPEG(data []byte) (Metadata, error) {
	return ParseJPEGReader(bytes.NewReader(data))
}

func ParseJPEGReader(reader io.Reader) (Metadata, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil || header[0] != 0xff || header[1] != 0xd8 {
		return Metadata{}, fmt.Errorf("keine JPEG-Datei")
	}
	for {
		prefix := make([]byte, 1)
		if _, err := io.ReadFull(reader, prefix); err != nil {
			return Metadata{}, nil
		}
		if prefix[0] != 0xff {
			continue
		}
		markerByte := make([]byte, 1)
		if _, err := io.ReadFull(reader, markerByte); err != nil {
			return Metadata{}, nil
		}
		for markerByte[0] == 0xff {
			if _, err := io.ReadFull(reader, markerByte); err != nil {
				return Metadata{}, nil
			}
		}
		marker := markerByte[0]
		if marker == 0xd9 || marker == 0xda {
			break
		}
		if marker == 0x00 || marker == 0xd8 || marker >= 0xd0 && marker <= 0xd7 {
			continue
		}
		lengthBytes := make([]byte, 2)
		if _, err := io.ReadFull(reader, lengthBytes); err != nil {
			return Metadata{}, nil
		}
		length := int(binary.BigEndian.Uint16(lengthBytes))
		if length < 2 {
			return Metadata{}, fmt.Errorf("ungültiges JPEG-Segment")
		}
		if marker == 0xe1 {
			segment := make([]byte, length-2)
			if _, err := io.ReadFull(reader, segment); err != nil {
				return Metadata{}, nil
			}
			if len(segment) >= 6 && string(segment[:6]) == "Exif\x00\x00" {
				return parseTIFF(segment[6:])
			}
			continue
		}
		if _, err := io.CopyN(io.Discard, reader, int64(length-2)); err != nil {
			return Metadata{}, nil
		}
	}
	return Metadata{}, nil
}

func Encode(metadata Metadata) string {
	if metadata == (Metadata{}) {
		return ""
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	return string(data)
}

func parseTIFF(data []byte) (Metadata, error) {
	if len(data) < 8 {
		return Metadata{}, fmt.Errorf("unvollständiger TIFF-Header")
	}
	var order binary.ByteOrder
	switch string(data[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return Metadata{}, fmt.Errorf("ungültige TIFF-Byte-Reihenfolge")
	}
	if order.Uint16(data[2:4]) != 42 {
		return Metadata{}, fmt.Errorf("ungültiger TIFF-Header")
	}
	metadata := Metadata{}
	exifOffset := uint32(0)
	readIFD(data, order, order.Uint32(data[4:8]), func(tag, kind uint16, count, value uint32, raw []byte) {
		switch tag {
		case 0x010f:
			metadata.Manufacturer = textValue(data, order, kind, count, value, raw)
		case 0x0110:
			metadata.Camera = textValue(data, order, kind, count, value, raw)
		case 0x0112:
			metadata.Orientation = int(shortValue(order, kind, value, raw))
		case 0x8769:
			exifOffset = value
		}
	})
	if exifOffset > 0 {
		readIFD(data, order, exifOffset, func(tag, kind uint16, count, value uint32, raw []byte) {
			switch tag {
			case 0x9003:
				metadata.CapturedAt = textValue(data, order, kind, count, value, raw)
			case 0xa434:
				metadata.Lens = textValue(data, order, kind, count, value, raw)
			}
		})
	}
	metadata.Manufacturer = strings.TrimSpace(metadata.Manufacturer)
	metadata.Camera = strings.TrimSpace(metadata.Camera)
	metadata.CapturedAt = strings.TrimSpace(metadata.CapturedAt)
	metadata.Lens = strings.TrimSpace(metadata.Lens)
	return metadata, nil
}

func readIFD(data []byte, order binary.ByteOrder, offset uint32, visit func(tag, kind uint16, count, value uint32, raw []byte)) {
	start := int(offset)
	if start < 0 || start+2 > len(data) {
		return
	}
	count := int(order.Uint16(data[start : start+2]))
	start += 2
	for index := 0; index < count; index++ {
		position := start + index*12
		if position+12 > len(data) {
			return
		}
		raw := data[position : position+12]
		visit(order.Uint16(raw[0:2]), order.Uint16(raw[2:4]), order.Uint32(raw[4:8]), order.Uint32(raw[8:12]), raw[8:12])
	}
}

func textValue(data []byte, order binary.ByteOrder, kind uint16, count, value uint32, inline []byte) string {
	if kind != 2 || count == 0 {
		return ""
	}
	var bytes []byte
	if count <= 4 {
		bytes = inline[:count]
	} else {
		start, end := int(value), int(value+count)
		if start < 0 || end < start || end > len(data) {
			return ""
		}
		bytes = data[start:end]
	}
	return strings.TrimRight(string(bytes), "\x00 ")
}

func shortValue(order binary.ByteOrder, kind uint16, value uint32, inline []byte) uint16 {
	if kind != 3 {
		return 0
	}
	if len(inline) >= 2 {
		return order.Uint16(inline[:2])
	}
	return uint16(value)
}
