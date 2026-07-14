package database

import (
	"os"
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
	files, drives, err = Validate(destination)
	if err != nil || files != 0 || drives != 0 {
		t.Fatalf("validated stats = %d, %d, %v", files, drives, err)
	}
}

func TestValidateRejectsInvalidCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.db")
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Validate(path); err == nil {
		t.Fatal("invalid database accepted")
	}
}
