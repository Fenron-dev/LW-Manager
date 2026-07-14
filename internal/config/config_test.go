package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesPortableDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "config.json")
	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !settings.VolumeDetectionEnabled || !settings.BackupEnabled || settings.BackupFileMB != 1024 || settings.BackupMaxMB != 2048 || !settings.ArchiveEnabled || settings.MaxSnapshots != 10 {
		t.Fatalf("unexpected defaults: %+v", settings)
	}
	if !settings.ScanDiagnosticsEnabled || settings.ScanDiagnosticFileMB != 2 || settings.ScanDiagnosticsTotalMB != 50 {
		t.Fatalf("unexpected scan diagnostic defaults: %+v", settings)
	}
	if !settings.ImageAnalysisEnabled || !settings.ImageJPEGEnabled || !settings.ImagePNGEnabled || !settings.ImageGIFEnabled || !settings.ImageHEICEnabled || settings.ImageHeaderMB != 4 {
		t.Fatalf("unexpected image analysis defaults: %+v", settings)
	}
	if settings.EXIFEnabled || settings.EXIFFileMB != 8 || !settings.EXIFTotalUnlimited {
		t.Fatalf("unexpected EXIF defaults: %+v", settings)
	}
	if settings.TextIndexEnabled || !settings.TextDocumentsEnabled || !settings.TextDataEnabled || !settings.TextSourceEnabled || settings.TextFileMB != 2 || settings.TextTotalMB != 500 {
		t.Fatalf("unexpected text index defaults: %+v", settings)
	}
	if !settings.ImagePreviewEnabled || !settings.HEICPreviewEnabled {
		t.Fatalf("unexpected preview defaults: %+v", settings)
	}
	if settings.AIEnabled || settings.AIProvider != "ollama" || settings.AIEndpoint != "http://127.0.0.1:11434" || settings.AIModel == "" || settings.AIFileMB != 2 || settings.AITotalMB != 100 || settings.AITimeoutSeconds != 30 {
		t.Fatalf("unexpected AI defaults: %+v", settings)
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

func TestLoadAddsDefaultsToOlderConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"version":1,"archiveEnabled":false,"maxSnapshots":5,"imagePreviewMB":20,"pdfPreviewMB":10,"videoPreviewMB":15}`), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if settings.ArchiveEnabled || !settings.VolumeDetectionEnabled || !settings.ImageAnalysisEnabled || settings.ImageHeaderMB != 4 || !settings.ThumbnailCacheUnlimited || settings.AIProvider != "ollama" {
		t.Fatalf("legacy migration lost values or defaults: %+v", settings)
	}
}

func TestAISettingsValidation(t *testing.T) {
	settings := Defaults()
	settings.AIProvider = "unknown"
	if err := settings.Validate(); err == nil {
		t.Fatal("expected provider validation error")
	}
	settings = Defaults()
	settings.AITimeoutSeconds = 2
	if err := settings.Validate(); err == nil {
		t.Fatal("expected timeout validation error")
	}
}
