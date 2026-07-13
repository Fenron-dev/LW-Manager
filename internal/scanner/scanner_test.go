package scanner

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestScanCollectsMetadataAndSkipsVault(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "photos"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "photos", "IMAGE.JPG"), []byte("photo"), 0o644); err != nil {
		t.Fatal(err)
	}
	vault := filepath.Join(root, "vault")
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vault, "vault.db"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Scan(context.Background(), root, vault, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(report.Files))
	}
	if report.Files[0].Path != "photos/IMAGE.JPG" || report.Files[0].Extension != "jpg" {
		t.Fatalf("unexpected file: %#v", report.Files[0])
	}
}

func TestScanCollectsImageDimensions(t *testing.T) {
	root := t.TempDir()
	file, err := os.Create(filepath.Join(root, "image.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 320, 180))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	report, err := Scan(context.Background(), root, filepath.Join(root, "vault"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 1 || report.Files[0].Width != 320 || report.Files[0].Height != 180 {
		t.Fatalf("unexpected dimensions: %#v", report.Files)
	}
}
