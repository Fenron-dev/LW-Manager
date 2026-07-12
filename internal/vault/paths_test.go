package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindRootWalksUpToMarker(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, Marker), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "bin", "linux")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findRoot(nested); got != root {
		t.Fatalf("findRoot() = %q, want %q", got, root)
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	if _, err := safeJoin(base, filepath.Join("..", "secret")); err == nil {
		t.Fatal("expected traversal error")
	}
	if got, err := safeJoin(base, "cache/item"); err != nil || got != filepath.Join(base, "cache", "item") {
		t.Fatalf("unexpected result: %q, %v", got, err)
	}
}

func TestPortableRootFromMacOSBundle(t *testing.T) {
	executable := filepath.Join("Volumes", "Vault", "VaultApp.app", "Contents", "MacOS", "VaultApp")
	want := filepath.Join("Volumes", "Vault")
	if got := portableRootFromExecutable(executable); got != want {
		t.Fatalf("portableRootFromExecutable() = %q, want %q", got, want)
	}
}

func TestPortableRootFromDirectBinary(t *testing.T) {
	executable := filepath.Join("media", "Vault", "VaultApp")
	want := filepath.Join("media", "Vault")
	if got := portableRootFromExecutable(executable); got != want {
		t.Fatalf("portableRootFromExecutable() = %q, want %q", got, want)
	}
}
