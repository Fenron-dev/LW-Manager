package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExistingVaultRequiresMarkerAndCatalog(t *testing.T) {
	root := t.TempDir()
	if IsExistingVault(root) {
		t.Fatal("empty directory is not a vault")
	}
	if err := EnsureLayout(root); err != nil {
		t.Fatal(err)
	}
	if IsExistingVault(root) {
		t.Fatal("marker alone is not an existing catalog")
	}
	if err := os.WriteFile(filepath.Join(root, "data", "vault.db"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !IsExistingVault(root) {
		t.Fatal("marked directory with catalog must be recognized")
	}
}
