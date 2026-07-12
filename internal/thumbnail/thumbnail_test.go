package thumbnail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFitPreservesBounds(t *testing.T) {
	w, h := fit(4000, 2000, 900, 600)
	if w != 900 || h != 450 {
		t.Fatalf("fit = %dx%d", w, h)
	}
	w, h = fit(320, 200, 900, 600)
	if w != 320 || h != 200 {
		t.Fatalf("small fit = %dx%d", w, h)
	}
}

func TestWebPIsReturnedForDirectBrowserPreview(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "preview.webp")
	if err := os.WriteFile(source, []byte("RIFF-test-WEBP"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := DataURL(source, filepath.Join(directory, "cache"), "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "data:image/webp;base64,") {
		t.Fatalf("unexpected data URL: %s", result)
	}
}

func TestPDFIsReturnedForEmbeddedPreview(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "document.pdf")
	if err := os.WriteFile(source, []byte("%PDF-1.7\n%%EOF"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := DataURL(source, filepath.Join(directory, "cache"), "test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "data:application/pdf;base64,") {
		t.Fatalf("unexpected data URL: %s", result)
	}
}
