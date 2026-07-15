package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dennis/vaultapp/internal/database"
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
