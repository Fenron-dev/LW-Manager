package backup

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func createRestoreTestBackup(t *testing.T, destination string, entries map[string][]byte) {
	t.Helper()
	file, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, data := range entries {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func validRestoreEntries(t *testing.T) map[string][]byte {
	t.Helper()
	manifest, err := json.Marshal(Manifest{Format: "VaultApp-Backup", Version: 1, CreatedAt: "2026-07-14T12:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{
		"VaultApp-Backup/manifest.json":           manifest,
		"VaultApp-Backup/data/vault.db":           []byte("sqlite-test"),
		"VaultApp-Backup/data/config.json":        []byte(`{"version":7}`),
		"VaultApp-Backup/assets/thumbnails/a.jpg": []byte("preview"),
	}
}

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

func TestExtractValidatesAndStagesKnownPayload(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "backup.zip")
	createRestoreTestBackup(t, archivePath, validRestoreEntries(t))
	staging := filepath.Join(directory, "staging")
	inspection, err := Extract(archivePath, staging, Limits{PerFileBytes: 1 << 20, TotalBytes: 2 << 20})
	if err != nil {
		t.Fatal(err)
	}
	if inspection.Files != 4 || !inspection.IncludesThumbnails || inspection.Manifest.Version != 1 {
		t.Fatalf("inspection = %#v", inspection)
	}
	for _, name := range []string{"manifest.json", "data/vault.db", "data/config.json", "assets/thumbnails/a.jpg"} {
		if _, err := os.Stat(filepath.Join(staging, filepath.FromSlash(name))); err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
	}
}

func TestExtractRejectsTraversalUnknownAndMissingPayload(t *testing.T) {
	for name, mutate := range map[string]func(map[string][]byte){
		"traversal":        func(entries map[string][]byte) { entries["../outside"] = []byte("bad") },
		"unknown":          func(entries map[string][]byte) { entries["VaultApp-Backup/data/other"] = []byte("bad") },
		"missing database": func(entries map[string][]byte) { delete(entries, "VaultApp-Backup/data/vault.db") },
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			entries := validRestoreEntries(t)
			mutate(entries)
			archivePath := filepath.Join(directory, "backup.zip")
			createRestoreTestBackup(t, archivePath, entries)
			if _, err := Extract(archivePath, filepath.Join(directory, "stage"), Limits{}); err == nil {
				t.Fatal("invalid backup accepted")
			}
		})
	}
}

func TestExtractHonorsUncompressedLimits(t *testing.T) {
	directory := t.TempDir()
	archivePath := filepath.Join(directory, "backup.zip")
	createRestoreTestBackup(t, archivePath, validRestoreEntries(t))
	if _, err := Extract(archivePath, filepath.Join(directory, "stage"), Limits{TotalBytes: 8}); !errors.Is(err, ErrLimit) {
		t.Fatalf("limit error = %v", err)
	}
}
