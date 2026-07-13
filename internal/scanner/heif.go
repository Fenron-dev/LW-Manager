package scanner

import (
	"encoding/binary"
	"fmt"
	"io"
)

func heifDimensions(reader io.Reader) (int, int, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return 0, 0, err
	}
	width, height, ok := findISPE(data, 0, len(data))
	if !ok {
		return 0, 0, fmt.Errorf("HEIC-Abmessungen nicht gefunden")
	}
	return width, height, nil
}

func findISPE(data []byte, start, end int) (int, int, bool) {
	for offset := start; offset+8 <= end; {
		size := uint64(binary.BigEndian.Uint32(data[offset : offset+4]))
		boxType := string(data[offset+4 : offset+8])
		header := 8
		if size == 1 {
			if offset+16 > end {
				return 0, 0, false
			}
			size = binary.BigEndian.Uint64(data[offset+8 : offset+16])
			header = 16
		} else if size == 0 {
			size = uint64(end - offset)
		}
		if size < uint64(header) || size > uint64(end-offset) {
			return 0, 0, false
		}
		boxEnd := offset + int(size)
		payload := offset + header
		if boxType == "ispe" && payload+12 <= boxEnd {
			width := binary.BigEndian.Uint32(data[payload+4 : payload+8])
			height := binary.BigEndian.Uint32(data[payload+8 : payload+12])
			if width > 0 && height > 0 && width <= 1_000_000 && height <= 1_000_000 {
				return int(width), int(height), true
			}
		}
		if boxType == "meta" {
			payload += 4 // FullBox version and flags.
		}
		switch boxType {
		case "meta", "iprp", "ipco", "moov", "trak", "mdia", "minf", "stbl":
			if payload < boxEnd {
				if width, height, ok := findISPE(data, payload, boxEnd); ok {
					return width, height, true
				}
			}
		}
		offset = boxEnd
	}
	return 0, 0, false
}
