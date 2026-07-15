package vault

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const Marker = ".vaultapp"

// ResolveRoot searches upwards from the executable, then checks VAULT_ROOT and
// the working directory. A marker prevents accidentally treating arbitrary
// parent directories as a vault.
func ResolveRoot() (string, error) {
	var executable string
	if exe, err := os.Executable(); err == nil {
		executable = exe
		if root := findRoot(filepath.Dir(exe)); root != "" {
			return root, nil
		}
	}
	if configured := os.Getenv("VAULT_ROOT"); configured != "" {
		return filepath.Abs(configured)
	}
	if cwd, err := os.Getwd(); err == nil {
		if root := findRoot(cwd); root != "" {
			return root, nil
		}
	}
	// Finder starts macOS applications with "/" as their working directory.
	// Derive the portable folder from the bundle in that case. Direct Windows
	// and Linux binaries use the directory containing the executable.
	if executable != "" {
		return portableRootFromExecutable(executable), nil
	}
	return "", errors.New("Vault-Stammverzeichnis wurde nicht gefunden")
}

func portableRootFromExecutable(executable string) string {
	current := filepath.Dir(filepath.Clean(executable))
	for {
		if strings.HasSuffix(strings.ToLower(filepath.Base(current)), ".app") {
			return filepath.Dir(current)
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
		current = parent
	}
	return filepath.Dir(filepath.Clean(executable))
}

func findRoot(start string) string {
	current, err := filepath.Abs(start)
	if err != nil {
		return ""
	}
	for {
		if info, err := os.Stat(filepath.Join(current, Marker)); err == nil && !info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func EnsureLayout(root string) error {
	for _, dir := range []string{"data", "data/logs", "data/cache", "data/quarantine", "assets", "assets/models", "assets/thumbnails"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			return err
		}
	}
	marker := filepath.Join(root, Marker)
	file, err := os.OpenFile(marker, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	return file.Close()
}

func DataPath(root, relative string) (string, error) {
	return safeJoin(filepath.Join(root, "data"), relative)
}

func AssetPath(root, relative string) (string, error) {
	return safeJoin(filepath.Join(root, "assets"), relative)
}

func safeJoin(base, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", errors.New("nur relative Vault-Pfade sind erlaubt")
	}
	cleanBase := filepath.Clean(base)
	result := filepath.Join(cleanBase, filepath.Clean(relative))
	rel, err := filepath.Rel(cleanBase, result)
	if err != nil || rel == ".." || len(rel) > 3 && rel[:3] == ".."+string(filepath.Separator) {
		return "", errors.New("Pfad verlässt den Vault")
	}
	return result, nil
}
