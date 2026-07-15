package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
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
