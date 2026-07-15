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
	if settings.ScanExclusionsEnabled || !settings.ScanExcludeSystem || !settings.ScanExcludeDevelopment || len(settings.ScanExcludedPatterns) != 0 {
		t.Fatalf("unexpected scan exclusion defaults: %+v", settings)
	}
	if !settings.ImageAnalysisEnabled || !settings.ImageJPEGEnabled || !settings.ImagePNGEnabled || !settings.ImageGIFEnabled || !settings.ImageHEICEnabled || settings.ImageHeaderMB != 4 {
		t.Fatalf("unexpected image analysis defaults: %+v", settings)
	}
	if settings.EXIFEnabled || settings.EXIFFileMB != 8 || !settings.EXIFTotalUnlimited {
		t.Fatalf("unexpected EXIF defaults: %+v", settings)
	}
	if settings.TextIndexEnabled || !settings.TextDocumentsEnabled || settings.TextPDFEnabled || settings.TextOfficeEnabled || !settings.TextDataEnabled || !settings.TextSourceEnabled || settings.TextFileMB != 2 || settings.TextTotalMB != 500 || settings.TextStoredMB != 500 || settings.TextStoredUnlimited {
		t.Fatalf("unexpected text index defaults: %+v", settings)
	}
	if !settings.ImagePreviewEnabled || !settings.HEICPreviewEnabled {
		t.Fatalf("unexpected preview defaults: %+v", settings)
	}
	if !settings.PDFPreviewEnabled || settings.PDFPreviewMB != 40 || settings.PDFPreviewUnlimited || !settings.VideoPreviewEnabled || settings.VideoPreviewMB != 50 || settings.VideoPreviewUnlimited {
		t.Fatalf("unexpected document preview defaults: %+v", settings)
	}
	if settings.AIEnabled || settings.AIProvider != "ollama" || settings.AIEndpoint != "http://127.0.0.1:11434" || settings.AIModel == "" || settings.AIFileMB != 2 || settings.AITotalMB != 100 || settings.AITimeoutSeconds != 30 {
		t.Fatalf("unexpected AI defaults: %+v", settings)
	}
	if settings.AIVisionEnabled || settings.AIVisionModel != "gemma3:4b" || settings.AIVisionFileMB != 25 || settings.AIVisionTotalMB != 100 {
		t.Fatalf("unexpected AI vision defaults: %+v", settings)
	}
	if !settings.CatalogExportEnabled || settings.CatalogExportMaxMB != 100 || settings.CatalogExportUnlimited {
		t.Fatalf("unexpected catalog export defaults: %+v", settings)
	}
	if !settings.DuplicateCheckEnabled || settings.DuplicateFileMB != 1024 || settings.DuplicateTotalMB != 2048 || settings.DuplicateFileUnlimited || settings.DuplicateTotalUnlimited {
		t.Fatalf("unexpected duplicate check defaults: %+v", settings)
	}
	if !settings.DuplicateQuarantineEnabled || settings.DuplicateQuarantineFileMB != 10_240 || settings.DuplicateQuarantineTotalMB != 102_400 || settings.DuplicateQuarantineFileUnlimited || settings.DuplicateQuarantineUnlimited {
		t.Fatalf("unexpected duplicate quarantine defaults: %+v", settings)
	}
	if settings.DuplicatePermanentDeleteEnabled {
		t.Fatal("permanent duplicate deletion must be disabled by default")
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

func TestScanExclusionValidation(t *testing.T) {
	settings := Defaults()
	settings.ScanExcludedPatterns = []string{"project/["}
	if err := settings.Validate(); err == nil {
		t.Fatal("expected malformed scan pattern validation error")
	}
	settings = Defaults()
	settings.ScanExcludedPatterns = []string{"../private"}
	if err := settings.Validate(); err == nil {
		t.Fatal("expected parent scan pattern validation error")
	}
}

func TestCatalogExportLimitValidation(t *testing.T) {
	settings := Defaults()
	settings.CatalogExportMaxMB = 0
	if err := settings.Validate(); err == nil {
		t.Fatal("expected catalog export limit validation error")
	}
	settings.CatalogExportMaxMB = 1_000_001
	if err := settings.Validate(); err == nil {
		t.Fatal("expected catalog export upper limit validation error")
	}
}

func TestTextIndexStorageLimitValidation(t *testing.T) {
	settings := Defaults()
	settings.TextStoredMB = 0
	if err := settings.Validate(); err == nil {
		t.Fatal("expected text index storage lower limit validation error")
	}
	settings = Defaults()
	settings.TextStoredMB = 1_000_001
	if err := settings.Validate(); err == nil {
		t.Fatal("expected text index storage upper limit validation error")
	}
}

func TestDuplicateCheckLimitValidation(t *testing.T) {
	settings := Defaults()
	settings.DuplicateFileMB = 0
	if err := settings.Validate(); err == nil {
		t.Fatal("expected duplicate file limit validation error")
	}
	settings = Defaults()
	settings.DuplicateTotalMB = 1_000_001
	if err := settings.Validate(); err == nil {
		t.Fatal("expected duplicate total limit validation error")
	}
	settings = Defaults()
	settings.DuplicateQuarantineFileMB = 0
	if err := settings.Validate(); err == nil {
		t.Fatal("expected quarantine file limit validation error")
	}
	settings = Defaults()
	settings.DuplicateQuarantineTotalMB = 1_000_001
	if err := settings.Validate(); err == nil {
		t.Fatal("expected quarantine total limit validation error")
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

func TestLegacyOpenRouterConfigGetsCompatibleVisionModel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"version":9,"aiProvider":"openrouter","aiEndpoint":"https://openrouter.ai/api/v1","aiModel":"openrouter/auto"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := Load(path)
	if err != nil || settings.AIVisionModel != "openrouter/auto" {
		t.Fatalf("legacy OpenRouter migration: %+v, %v", settings, err)
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
	settings = Defaults()
	settings.AIEndpoint = "file:///tmp/model"
	if err := settings.Validate(); err == nil {
		t.Fatal("expected endpoint validation error")
	}
	settings = Defaults()
	settings.AIVisionFileMB = 0
	if err := settings.Validate(); err == nil {
		t.Fatal("expected vision limit validation error")
	}
}
