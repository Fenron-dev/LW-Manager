package config

import (
	"path/filepath"
	"testing"
)

func TestLoadCreatesPortableDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "config.json")
	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.ArchiveEnabled || settings.MaxSnapshots != 10 {
		t.Fatalf("unexpected defaults: %+v", settings)
	}
	settings.MaxSnapshots = 3
	if err := Save(path, settings); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.MaxSnapshots != 3 {
		t.Fatalf("reloaded settings: %+v, %v", loaded, err)
	}
}
