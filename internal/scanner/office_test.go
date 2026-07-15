package scanner

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractOfficeTextFromDOCXAndODT(t *testing.T) {
	docx := officeTestArchive(t, map[string]string{
		"word/document.xml": `<?xml version="1.0"?><w:document xmlns:w="urn:w"><w:body><w:p><w:r><w:t>Hallo &amp; willkommen</w:t></w:r></w:p><w:p><w:r><w:t>im Vault</w:t></w:r></w:p></w:body></w:document>`,
		"word/header1.xml":  `<?xml version="1.0"?><w:hdr xmlns:w="urn:w"><w:p><w:r><w:t>Kopfzeile</w:t></w:r></w:p></w:hdr>`,
		"custom.xml":        `<ignored>Geheim</ignored>`,
	})
	if text := extractOfficeText(docx, ".docx", 1024); text != "Hallo & willkommen im Vault Kopfzeile" {
		t.Fatalf("DOCX text = %q", text)
	}
	odt := officeTestArchive(t, map[string]string{
		"content.xml": `<office:document xmlns:office="urn:o" xmlns:text="urn:t"><office:body><text:p>Lokaler ODT-Inhalt</text:p></office:body></office:document>`,
	})
	if text := extractOfficeText(odt, ".odt", 1024); text != "Lokaler ODT-Inhalt" {
		t.Fatalf("ODT text = %q", text)
	}
}

func TestOfficeTextRejectsInvalidArchiveAndHonorsLimit(t *testing.T) {
	if text := extractOfficeText([]byte("not a zip"), ".docx", 100); text != "" {
		t.Fatalf("invalid archive text = %q", text)
	}
	docx := officeTestArchive(t, map[string]string{"word/document.xml": `<w:document xmlns:w="urn:w"><w:t>Begrenzter Office-Text</w:t></w:document>`})
	if text := extractOfficeText(docx, ".docx", 10); text != "Begrenzter" {
		t.Fatalf("limited DOCX text = %q", text)
	}
}

func TestOfficeTextPreviewUsesOfficeToggleAndBudgets(t *testing.T) {
	docx := officeTestArchive(t, map[string]string{"word/document.xml": `<w:document xmlns:w="urn:w"><w:t>Indexierter Inhalt</w:t></w:document>`})
	file := filepath.Join(t.TempDir(), "document.docx")
	if err := os.WriteFile(file, docx, 0o600); err != nil {
		t.Fatal(err)
	}
	read, stored := int64(0), int64(0)
	options := TextIndexOptions{Enabled: true, Office: true, PerFileUnlimited: true, TotalUnlimited: true, StoredBytes: 1024, StoredLimitEnabled: true}
	if text := textPreview(file, ".docx", options, &read, &stored); text != "Indexierter Inhalt" || read != int64(len(docx)) || stored != int64(len(text)) {
		t.Fatalf("Office preview = %q, read %d, stored %d", text, read, stored)
	}
	read, stored = 0, 0
	options.Office = false
	if text := textPreview(file, ".docx", options, &read, &stored); text != "" || read != 0 || stored != 0 {
		t.Fatalf("disabled Office preview = %q, read %d, stored %d", text, read, stored)
	}
}

func officeTestArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for name, content := range files {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
