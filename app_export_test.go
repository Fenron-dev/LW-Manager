package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dennis/vaultapp/internal/database"
)

func TestExportLimitWriterStopsBeforeLimit(t *testing.T) {
	var destination bytes.Buffer
	writer := &exportLimitWriter{writer: &destination, maximum: 4}
	if _, err := writer.Write([]byte("1234")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("5")); err == nil {
		t.Fatal("expected configured limit to reject more data")
	}
	if writer.written != 4 || destination.String() != "1234" {
		t.Fatalf("writer changed after rejection: %d, %q", writer.written, destination.String())
	}
}

func TestWriteCatalogJSONStreamsValidDocument(t *testing.T) {
	var destination bytes.Buffer
	header := catalogJSONHeader{Format: "vaultapp.catalog", Version: 1, ExportedAt: "2026-07-16T12:00:00Z"}
	header.Filters.Query = "foto"
	header.Filters.DriveID = 7
	count, err := writeCatalogJSON(&destination, header, func(handle func(database.ExportFile) error) error {
		for _, file := range []database.ExportFile{
			{Filename: "eins.jpg", Drive: "Fotos", Path: "Sommer/eins.jpg", Extension: "jpg", Size: 42, Tags: []string{"Urlaub"}},
			{Filename: "zwei.png", Drive: "Fotos", Path: "Sommer/zwei.png", Extension: "png", Size: 84},
		} {
			if err := handle(file); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil || count != 2 {
		t.Fatalf("writeCatalogJSON = %d, %v", count, err)
	}
	var document struct {
		Format  string                `json:"format"`
		Filters map[string]any        `json:"filters"`
		Files   []database.ExportFile `json:"files"`
	}
	if err := json.Unmarshal(destination.Bytes(), &document); err != nil {
		t.Fatalf("invalid JSON %q: %v", destination.String(), err)
	}
	if document.Format != "vaultapp.catalog" || document.Filters["query"] != "foto" || len(document.Files) != 2 || document.Files[0].Tags[0] != "Urlaub" {
		t.Fatalf("unexpected document: %+v", document)
	}
}

func TestWriteComparisonJSONStreamsValidReport(t *testing.T) {
	var destination bytes.Buffer
	header := comparisonJSONHeader{Format: "vaultapp.archive-comparison", Version: 1, ExportedAt: "2026-07-16T12:00:00Z"}
	header.Snapshot = database.ComparisonSnapshot{Snapshot: database.Snapshot{ID: 12, CapturedAt: "2026-07-15T12:00:00Z", Tags: []string{"Referenz"}}, DriveID: 3, DriveName: "Fotos"}
	header.Filters.Status = "modified"
	count, err := writeComparisonJSON(&destination, header, func(handle func(database.ComparisonEntry) error) (int, error) {
		entries := []database.ComparisonEntry{{Path: "Sommer/foto.jpg", Status: "modified", CurrentName: "foto.jpg", CurrentSize: 84, ArchiveName: "foto.jpg", ArchiveSize: 42}}
		for _, entry := range entries {
			if err := handle(entry); err != nil {
				return 0, err
			}
		}
		return len(entries), nil
	})
	if err != nil || count != 1 {
		t.Fatalf("writeComparisonJSON = %d, %v", count, err)
	}
	var document struct {
		Format   string                      `json:"format"`
		Snapshot database.ComparisonSnapshot `json:"snapshot"`
		Filters  map[string]any              `json:"filters"`
		Entries  []database.ComparisonEntry  `json:"entries"`
	}
	if err := json.Unmarshal(destination.Bytes(), &document); err != nil {
		t.Fatalf("invalid JSON %q: %v", destination.String(), err)
	}
	if document.Format != "vaultapp.archive-comparison" || document.Snapshot.DriveName != "Fotos" || document.Filters["status"] != "modified" || len(document.Entries) != 1 {
		t.Fatalf("unexpected document: %+v", document)
	}
}

func TestCSVSafePreventsSpreadsheetFormulas(t *testing.T) {
	for _, value := range []string{"=1+1", "+SUM(A1:A2)", "-2+3", "@command", "  =hidden"} {
		if got := csvSafe(value); got == value || got[0] != '\'' {
			t.Fatalf("csvSafe(%q) = %q", value, got)
		}
	}
	if got := csvSafe("normaler Dateiname.txt"); got != "normaler Dateiname.txt" {
		t.Fatalf("safe value changed to %q", got)
	}
}

func TestReplaceExportFileReplacesExistingDestination(t *testing.T) {
	directory := t.TempDir()
	source := filepath.Join(directory, "new.tmp")
	destination := filepath.Join(directory, "catalog.csv")
	if err := os.WriteFile(source, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := replaceExportFile(source, destination); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "new" {
		t.Fatalf("destination = %q, %v", data, err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
}
