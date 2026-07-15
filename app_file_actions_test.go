package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appconfig "github.com/dennis/vaultapp/internal/config"
	"github.com/dennis/vaultapp/internal/database"
	"github.com/dennis/vaultapp/internal/scanner"
	"github.com/dennis/vaultapp/internal/vault"
)

func catalogSourceForTest(t *testing.T, root, relative string) database.SourceFile {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Stat(full)
	if err != nil {
		t.Fatal(err)
	}
	return database.SourceFile{
		Root: root, Relative: relative, Path: full, Size: info.Size(),
		Modified: info.ModTime().UTC().Format(time.RFC3339Nano),
	}
}

func TestQuarantineAndRestoreDuplicate(t *testing.T) {
	vaultRoot := t.TempDir()
	if err := vault.EnsureLayout(vaultRoot); err != nil {
		t.Fatal(err)
	}
	databasePath, _ := vault.DataPath(vaultRoot, "vault.db")
	catalog, err := database.Open(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close() })
	sourceRoot := t.TempDir()
	content := []byte("identical duplicate content")
	files := make([]scanner.File, 0, 2)
	for _, name := range []string{"original.bin", "candidate.bin"} {
		path := filepath.Join(sourceRoot, name)
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		info, _ := os.Stat(path)
		files = append(files, scanner.File{Path: name, Filename: name, Size: info.Size(), Modified: info.ModTime()})
	}
	if err := catalog.ReplaceDriveScan(database.DriveScan{Path: sourceRoot, Label: "DUPLICATES", UUID: "duplicate-volume", Files: files}); err != nil {
		t.Fatal(err)
	}
	result, err := catalog.Search("", "", "", 0, false, 50, 0)
	if err != nil || len(result.Files) != 2 {
		t.Fatalf("files = %#v, %v", result, err)
	}
	hash, _, err := hashFile(filepath.Join(sourceRoot, "candidate.bin"))
	if err != nil {
		t.Fatal(err)
	}
	var candidateID int64
	for _, file := range result.Files {
		if err := catalog.SaveFileHash(file.ID, hash); err != nil {
			t.Fatal(err)
		}
		if file.Filename == "candidate.bin" {
			candidateID = file.ID
		}
	}
	app := &App{root: vaultRoot, catalog: catalog, settings: appconfig.Defaults()}
	quarantined, err := app.QuarantineDuplicates([]DuplicateSelection{{Hash: hash, FileID: candidateID}})
	if err != nil || quarantined.Moved != 1 || quarantined.Bytes != int64(len(content)) || len(quarantined.Failures) != 0 {
		t.Fatalf("quarantine = %#v, %v", quarantined, err)
	}
	if _, err := os.Stat(filepath.Join(sourceRoot, "candidate.bin")); !os.IsNotExist(err) {
		t.Fatalf("candidate still exists: %v", err)
	}
	items, err := app.GetQuarantineItems()
	if err != nil || len(items) != 1 || items[0].OriginalPath != "candidate.bin" {
		t.Fatalf("items = %#v, %v", items, err)
	}
	result, _ = catalog.Search("", "", "", 0, false, 50, 0)
	if result.Total != 1 {
		t.Fatalf("catalog still contains quarantined file: %#v", result)
	}
	if err := app.RestoreQuarantineItem(items[0].ID); err != nil {
		t.Fatal(err)
	}
	restored, err := os.ReadFile(filepath.Join(sourceRoot, "candidate.bin"))
	if err != nil || string(restored) != string(content) {
		t.Fatalf("restored = %q, %v", restored, err)
	}
	items, err = app.GetQuarantineItems()
	if err != nil || len(items) != 0 {
		t.Fatalf("quarantine not empty: %#v, %v", items, err)
	}
}

func TestVerifyRestoreDestinationRejectsExistingFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing.bin"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyRestoreDestination(root, "existing.bin"); err == nil || !strings.Contains(err.Error(), "bereits") {
		t.Fatalf("existing destination accepted: %v", err)
	}
}

func TestVerifyCatalogSourceAcceptsUnchangedRegularFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "document.txt"), []byte("vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyCatalogSource(catalogSourceForTest(t, root, "document.txt")); err != nil {
		t.Fatalf("unchanged file rejected: %v", err)
	}
}

func TestVerifyCatalogSourceRejectsChangedAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "document.txt")
	if err := os.WriteFile(filePath, []byte("vault"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := catalogSourceForTest(t, root, "document.txt")
	changed := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filePath, changed, changed); err != nil {
		t.Fatal(err)
	}
	if err := verifyCatalogSource(source); err == nil || !strings.Contains(err.Error(), "erneut scannen") {
		t.Fatalf("changed file was not rejected: %v", err)
	}
	if err := os.Remove(filePath); err != nil {
		t.Fatal(err)
	}
	if err := verifyCatalogSource(source); err == nil || !strings.Contains(err.Error(), "fehlt") {
		t.Fatalf("missing file was not reported: %v", err)
	}
}

func TestVerifyCatalogSourceRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.txt"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "redirect")); err != nil {
		t.Fatal(err)
	}
	source := catalogSourceForTest(t, root, "redirect/outside.txt")
	if err := verifyCatalogSource(source); err == nil || !strings.Contains(err.Error(), "heraus") {
		t.Fatalf("symlink escape was not rejected: %v", err)
	}
	if err := verifyContainingFolder(source); err == nil || !strings.Contains(err.Error(), "heraus") {
		t.Fatalf("symlink folder escape was not rejected: %v", err)
	}
}

func TestVerifyCatalogSourceReportsOfflineRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "removed-volume")
	source := database.SourceFile{Root: root, Relative: "file.txt", Path: filepath.Join(root, "file.txt")}
	if err := verifyCatalogSource(source); err == nil || !strings.Contains(err.Error(), "nicht angeschlossen") {
		t.Fatalf("offline root was not reported: %v", err)
	}
}
