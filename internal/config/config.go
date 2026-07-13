package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Settings struct {
	Version        int  `json:"version"`
	ArchiveEnabled bool `json:"archiveEnabled"`
	MaxSnapshots   int  `json:"maxSnapshots"`
	ImagePreviewMB int  `json:"imagePreviewMB"`
	PDFPreviewMB   int  `json:"pdfPreviewMB"`
}

func Defaults() Settings {
	return Settings{Version: 1, ArchiveEnabled: true, MaxSnapshots: 10, ImagePreviewMB: 100, PDFPreviewMB: 40}
}

func Load(path string) (Settings, error) {
	settings := Defaults()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return settings, Save(path, settings)
	}
	if err != nil {
		return settings, err
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return Defaults(), fmt.Errorf("config.json: %w", err)
	}
	if err := settings.Validate(); err != nil {
		return Defaults(), err
	}
	return settings, nil
}

func Save(path string, settings Settings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	settings.Version = 1
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func (settings Settings) Validate() error {
	if settings.MaxSnapshots < 0 || settings.MaxSnapshots > 100 {
		return fmt.Errorf("maximale Archivstände müssen zwischen 0 und 100 liegen")
	}
	if settings.ImagePreviewMB < 1 || settings.ImagePreviewMB > 100 {
		return fmt.Errorf("Bildlimit muss zwischen 1 und 100 MB liegen")
	}
	if settings.PDFPreviewMB < 1 || settings.PDFPreviewMB > 100 {
		return fmt.Errorf("PDF-Limit muss zwischen 1 und 100 MB liegen")
	}
	return nil
}
