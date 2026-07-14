package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagedBackupsAreScopedToVaultRoot(t *testing.T) {
	root := t.TempDir()
	backupPath := filepath.Join(root, "VaultApp-Backup-2026.zip")
	rollbackPath := filepath.Join(root, "VaultApp-Rollback-2026.zip")
	for _, path := range []string{backupPath, rollbackPath, filepath.Join(root, "anderes.zip")} {
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{root: root}
	backups, err := app.GetManagedBackups()
	if err != nil || len(backups) != 2 {
		t.Fatalf("backups = %#v, %v", backups, err)
	}
	outside := filepath.Join(t.TempDir(), "VaultApp-Backup-outside.zip")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := app.DeleteManagedBackup(outside); err == nil {
		t.Fatal("backup outside vault was deleted")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside backup changed: %v", err)
	}
	if err := app.DeleteManagedBackup(backupPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Fatalf("managed backup still exists: %v", err)
	}
}
