package scanner

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractPDFTextFromPlainAndCompressedStreams(t *testing.T) {
	plain := []byte("%PDF-1.4\n1 0 obj << /Length 48 >> stream\nBT /F1 12 Tf (Hallo\\040VaultApp) Tj ET\nendstream\nendobj\n%%EOF")
	if text := extractPDFText(plain, 1024); text != "Hallo VaultApp" {
		t.Fatalf("plain PDF text = %q", text)
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	_, _ = writer.Write([]byte("BT /F1 12 Tf <005000440046002D0054006500780074> Tj ET"))
	_ = writer.Close()
	pdf := []byte(fmt.Sprintf("%%PDF-1.4\n2 0 obj << /Filter /FlateDecode /Length %d >> stream\n", compressed.Len()))
	pdf = append(pdf, compressed.Bytes()...)
	pdf = append(pdf, []byte("\nendstream\nendobj\n%%EOF")...)
	if text := extractPDFText(pdf, 1024); text != "PDF-Text" {
		t.Fatalf("compressed PDF text = %q", text)
	}
}

func TestPDFIndexRespectsStoredBudgetAndEncryption(t *testing.T) {
	root := t.TempDir()
	pdfPath := filepath.Join(root, "document.pdf")
	pdf := []byte("%PDF-1.4\n1 0 obj <<>> stream\nBT (Ein langer lokaler PDF Inhalt) Tj ET\nendstream\n%%EOF")
	if err := os.WriteFile(pdfPath, pdf, 0o600); err != nil {
		t.Fatal(err)
	}
	read, stored := int64(0), int64(0)
	options := TextIndexOptions{Enabled: true, PDF: true, PerFileUnlimited: true, TotalUnlimited: true, StoredBytes: 10, StoredLimitEnabled: true}
	text := textPreview(pdfPath, ".pdf", options, &read, &stored)
	if text != "Ein langer" || stored != int64(len(text)) || read != int64(len(pdf)) {
		t.Fatalf("limited PDF index = %q, read %d, stored %d", text, read, stored)
	}
	encrypted := append([]byte("%PDF-1.4 /Encrypt "), pdf...)
	if text := extractPDFText(encrypted, 1024); text != "" {
		t.Fatalf("encrypted PDF unexpectedly indexed: %q", text)
	}
	if !strings.Contains(string(pdf), "lokaler") {
		t.Fatal("invalid PDF fixture")
	}
}

func TestTextIndexStopsAtExhaustedStoredBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "text.txt")
	if err := os.WriteFile(path, []byte("darf nicht gespeichert werden"), 0o600); err != nil {
		t.Fatal(err)
	}
	read, stored := int64(0), int64(0)
	options := TextIndexOptions{Enabled: true, Documents: true, PerFileUnlimited: true, TotalUnlimited: true, StoredLimitEnabled: true}
	if text := textPreview(path, ".txt", options, &read, &stored); text != "" || read != 0 || stored != 0 {
		t.Fatalf("exhausted stored budget indexed %q, read %d, stored %d", text, read, stored)
	}
}
