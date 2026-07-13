package backup

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateWritesPortableArchive(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "config.json")
	if err := os.WriteFile(source, []byte(`{"version":7}`), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(directory, "backup.zip")
	result, err := Create(destination, []Source{{Path: source, Name: "VaultApp-Backup/data/config.json"}}, Limits{PerFileBytes: 1 << 20, TotalBytes: 1 << 20})
	if err != nil || result.Files != 2 || result.Bytes <= 0 {
		t.Fatalf("result = %+v, %v", result, err)
	}
	archive, err := zip.OpenReader(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != 2 || archive.File[0].Name != "VaultApp-Backup/manifest.json" || archive.File[1].Name != "VaultApp-Backup/data/config.json" {
		t.Fatalf("unexpected entries: %#v", archive.File)
	}
}

func TestCreateHonorsTotalLimitAndRemovesPartialFile(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "random.bin")
	data := make([]byte, 32<<10)
	for index := range data {
		data[index] = byte(index * 31)
	}
	if err := os.WriteFile(source, data, 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(directory, "backup.zip")
	_, err := Create(destination, []Source{{Path: source, Name: "VaultApp-Backup/random.bin"}}, Limits{TotalBytes: 128})
	if !errors.Is(err, ErrLimit) {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(destination); !os.IsNotExist(err) {
		t.Fatalf("partial destination remains: %v", err)
	}
}

func TestCreateHonorsPerFileLimit(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "catalog.db")
	if err := os.WriteFile(source, make([]byte, 2048), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Create(filepath.Join(directory, "backup.zip"), []Source{{Path: source, Name: "VaultApp-Backup/data/vault.db"}}, Limits{PerFileBytes: 1024, TotalBytes: 1 << 20})
	if err == nil || !strings.Contains(err.Error(), "pro Datei") {
		t.Fatalf("unexpected error: %v", err)
	}
}
