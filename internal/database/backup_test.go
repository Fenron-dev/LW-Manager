package database

import (
	"path/filepath"
	"testing"
)

func TestBackupToCreatesReadableCatalog(t *testing.T) {
	directory := t.TempDir()
	catalog, err := Open(filepath.Join(directory, "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close()
	destination := filepath.Join(directory, "snapshot.db")
	if err := catalog.BackupTo(destination); err != nil {
		t.Fatal(err)
	}
	copy, err := Open(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer copy.Close()
	files, drives, err := copy.Stats()
	if err != nil || files != 0 || drives != 0 {
		t.Fatalf("snapshot stats = %d, %d, %v", files, drives, err)
	}
}
