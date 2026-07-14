package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeRestoreTestFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readRestoreTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestApplyRestoreSwapsCanRollbackPartialFailure(t *testing.T) {
	directory := t.TempDir()
	firstDestination := filepath.Join(directory, "data", "first")
	secondDestination := filepath.Join(directory, "data", "second")
	firstSource := filepath.Join(directory, "stage", "first")
	writeRestoreTestFile(t, firstDestination, "old-first")
	writeRestoreTestFile(t, secondDestination, "old-second")
	writeRestoreTestFile(t, firstSource, "new-first")
	swaps := []restoreSwap{
		{source: firstSource, destination: firstDestination, previous: firstDestination + ".old"},
		{source: filepath.Join(directory, "stage", "missing"), destination: secondDestination, previous: secondDestination + ".old"},
	}
	completed, err := applyRestoreSwaps(swaps)
	if err == nil {
		t.Fatal("missing restore source accepted")
	}
	if err := rollbackRestoreSwaps(swaps, completed); err != nil {
		t.Fatal(err)
	}
	if value := readRestoreTestFile(t, firstDestination); value != "old-first" {
		t.Fatalf("first = %q", value)
	}
	if value := readRestoreTestFile(t, secondDestination); value != "old-second" {
		t.Fatalf("second = %q", value)
	}
}

func TestApplyRestoreSwapsReplacesFiles(t *testing.T) {
	directory := t.TempDir()
	destination := filepath.Join(directory, "data", "catalog")
	source := filepath.Join(directory, "stage", "catalog")
	writeRestoreTestFile(t, destination, "old")
	writeRestoreTestFile(t, source, "new")
	swaps := []restoreSwap{{source: source, destination: destination, previous: destination + ".old"}}
	completed, err := applyRestoreSwaps(swaps)
	if err != nil || completed != 1 {
		t.Fatalf("completed = %d, err = %v", completed, err)
	}
	if value := readRestoreTestFile(t, destination); value != "new" {
		t.Fatalf("destination = %q", value)
	}
	if value := readRestoreTestFile(t, destination+".old"); value != "old" {
		t.Fatalf("previous = %q", value)
	}
}
