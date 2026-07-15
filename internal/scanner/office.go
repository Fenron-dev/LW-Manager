package scanner

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"sort"
	"strings"
)

const maximumOfficeXMLBytes = 32 << 20

// extractOfficeText reads the XML payload of modern Word/Writer documents.
// It intentionally supports only ZIP based DOCX and ODT files and never runs
// macros, embedded objects or external converters.
func extractOfficeText(data []byte, extension string, maximum int64) string {
	if maximum <= 0 {
		return ""
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
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
	case ".odt":
		return name == "content.xml"
	default:
		return false
	}
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
