package scanner

import (
	"context"
	"encoding/binary"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
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
	report, err := Scan(context.Background(), root, vault, ImageAnalysisOptions{Enabled: true, JPEG: true, PNG: true, GIF: true, PerFileBytes: 4 << 20, TotalUnlimited: true}, EXIFAnalysisOptions{}, TextIndexOptions{}, nil)
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

func TestScanCollectsOptionalEXIFMetadata(t *testing.T) {
	root := t.TempDir()
	tiff := make([]byte, 48)
	copy(tiff[:2], "II")
	binary.LittleEndian.PutUint16(tiff[2:4], 42)
	binary.LittleEndian.PutUint32(tiff[4:8], 8)
	binary.LittleEndian.PutUint16(tiff[8:10], 1)
	binary.LittleEndian.PutUint16(tiff[10:12], 0x010f)
	binary.LittleEndian.PutUint16(tiff[12:14], 2)
	binary.LittleEndian.PutUint32(tiff[14:18], 6)
	binary.LittleEndian.PutUint32(tiff[18:22], 30)
	copy(tiff[30:], "Canon\x00")
	segment := append([]byte("Exif\x00\x00"), tiff...)
	jpeg := []byte{0xff, 0xd8, 0xff, 0xe1, byte((len(segment) + 2) >> 8), byte(len(segment) + 2)}
	jpeg = append(jpeg, segment...)
	if err := os.WriteFile(filepath.Join(root, "photo.jpg"), jpeg, 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := Scan(context.Background(), root, filepath.Join(root, "vault"), ImageAnalysisOptions{}, EXIFAnalysisOptions{Enabled: true, PerFileBytes: 1 << 20, TotalUnlimited: true}, TextIndexOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 1 || !strings.Contains(report.Files[0].Metadata, `"manufacturer":"Canon"`) {
		t.Fatalf("EXIF metadata missing: %#v", report.Files)
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
	report, err := Scan(context.Background(), root, filepath.Join(root, "vault"), ImageAnalysisOptions{Enabled: true, PNG: true, PerFileBytes: 4 << 20, TotalUnlimited: true}, EXIFAnalysisOptions{}, TextIndexOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Files) != 1 || report.Files[0].Width != 320 || report.Files[0].Height != 180 {
		t.Fatalf("unexpected dimensions: %#v", report.Files)
	}
}

func TestScanCanDisableImageDimensions(t *testing.T) {
	root := t.TempDir()
	file, err := os.Create(filepath.Join(root, "image.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 20, 10))); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	report, err := Scan(context.Background(), root, filepath.Join(root, "vault"), ImageAnalysisOptions{}, EXIFAnalysisOptions{}, TextIndexOptions{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if report.Files[0].Width != 0 || report.Files[0].Height != 0 {
		t.Fatalf("dimensions should be disabled: %#v", report.Files[0])
	}
}

func TestScanIndexesSelectedTextFormats(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.md"), []byte("Ein eindeutig suchbarer Inhalt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.bin"), []byte("nicht indexieren"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := Scan(context.Background(), root, filepath.Join(root, "vault"), ImageAnalysisOptions{}, EXIFAnalysisOptions{}, TextIndexOptions{Enabled: true, Documents: true, PerFileBytes: 1 << 20, TotalUnlimited: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range report.Files {
		if file.Extension == "md" && file.TextContent != "Ein eindeutig suchbarer Inhalt" {
			t.Fatalf("text missing: %#v", file)
		}
		if file.Extension == "bin" && file.TextContent != "" {
			t.Fatalf("binary file indexed: %#v", file)
		}
	}
}
