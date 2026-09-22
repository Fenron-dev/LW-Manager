package vault

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type selection struct {
	Path string `json:"path"`
}

func SelectionFile() (string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(config, "LW-Manager", "vault.json"), nil
}

func LoadSelection() (string, error) {
	file, err := SelectionFile()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	var chosen selection
	if err := json.Unmarshal(data, &chosen); err != nil {
		return "", fmt.Errorf("Vault-Auswahl lesen: %w", err)
	}
	if !filepath.IsAbs(chosen.Path) {
		return "", fmt.Errorf("Vault-Auswahl enthält keinen absoluten Pfad")
	}
	return filepath.Clean(chosen.Path), nil
}

func SaveSelection(root string) error {
	file, err := SelectionFile()
	if err != nil {
		return err
	}
	if !filepath.IsAbs(root) {
		return fmt.Errorf("Vault-Pfad muss absolut sein")
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(selection{Path: filepath.Clean(root)})
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(file), "vault-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporary.Name(), file)
}

func IsExistingVault(root string) bool {
	marker, err := os.Stat(filepath.Join(root, Marker))
	if err != nil || marker.IsDir() {
		return false
	}
	info, err := os.Stat(filepath.Join(root, "data", "vault.db"))
	return err == nil && !info.IsDir()
}
