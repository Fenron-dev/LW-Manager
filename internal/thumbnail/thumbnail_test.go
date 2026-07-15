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

func TestVideoIsReturnedForEmbeddedPreview(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "clip.mp4")
	if err := os.WriteFile(source, []byte("test-video"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := DataURLWithLimits(source, filepath.Join(directory, "cache"), "test", Limits{ImageEnabled: true, ImageMB: 100, CacheUnlimited: true, PDFEnabled: true, PDFMB: 40, VideoEnabled: true, VideoMB: 50})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(result, "data:video/mp4;base64,") {
		t.Fatalf("unexpected data URL: %s", result)
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

func TestPDFAndVideoPreviewsCanBeDisabled(t *testing.T) {
	directory := t.TempDir()
	for name, limits := range map[string]Limits{
		"document.pdf": {PDFEnabled: false, PDFMB: 40},
		"clip.mp4":     {VideoEnabled: false, VideoMB: 50},
	} {
		source := filepath.Join(directory, name)
		if err := os.WriteFile(source, []byte("preview"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := DataURLWithLimits(source, filepath.Join(directory, "cache"), "test", limits); err == nil || !strings.Contains(err.Error(), "deaktiviert") {
			t.Fatalf("%s was not disabled: %v", name, err)
		}
	}
}

func TestPDFAndVideoUnlimitedIgnoreFileLimit(t *testing.T) {
	directory := t.TempDir()
	for name, limits := range map[string]Limits{
		"document.pdf": {PDFEnabled: true, PDFMB: 1, PDFUnlimited: true},
		"clip.mp4":     {VideoEnabled: true, VideoMB: 1, VideoUnlimited: true},
	} {
		source := filepath.Join(directory, name)
		if err := os.WriteFile(source, make([]byte, (1<<20)+1), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := DataURLWithLimits(source, filepath.Join(directory, "cache"), "test", limits); err != nil {
			t.Fatalf("%s unlimited preview failed: %v", name, err)
		}
	}
}

func TestHEICPreviewCanBeDisabled(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "photo.heic")
	if err := os.WriteFile(source, []byte("test-heic"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := DataURLWithLimits(source, filepath.Join(directory, "cache"), "test", Limits{ImageEnabled: true, HEICEnabled: false, ImageMB: 100, CacheUnlimited: true})
	if err == nil || !strings.Contains(err.Error(), "deaktiviert") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTrimCacheHonorsTotalLimit(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"first.jpg", "second.jpg"} {
		if err := os.WriteFile(filepath.Join(directory, name), make([]byte, 700), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := TrimCache(directory, 800); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache contains %d files, want 1", len(entries))
	}
}
