package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Settings struct {
	Version                  int    `json:"version"`
	VolumeDetectionEnabled   bool   `json:"volumeDetectionEnabled"`
	BackupEnabled            bool   `json:"backupEnabled"`
	BackupIncludeThumbnails  bool   `json:"backupIncludeThumbnails"`
	BackupFileMB             int    `json:"backupFileMB"`
	BackupFileUnlimited      bool   `json:"backupFileUnlimited"`
	BackupMaxMB              int    `json:"backupMaxMB"`
	BackupUnlimited          bool   `json:"backupUnlimited"`
	ArchiveEnabled           bool   `json:"archiveEnabled"`
	MaxSnapshots             int    `json:"maxSnapshots"`
	ScanDiagnosticsEnabled   bool   `json:"scanDiagnosticsEnabled"`
	ScanDiagnosticFileMB     int    `json:"scanDiagnosticFileMB"`
	ScanDiagnosticUnlimited  bool   `json:"scanDiagnosticUnlimited"`
	ScanDiagnosticsTotalMB   int    `json:"scanDiagnosticsTotalMB"`
	ScanDiagnosticsUnlimited bool   `json:"scanDiagnosticsUnlimited"`
	ImageAnalysisEnabled     bool   `json:"imageAnalysisEnabled"`
	ImageJPEGEnabled         bool   `json:"imageJPEGEnabled"`
	ImagePNGEnabled          bool   `json:"imagePNGEnabled"`
	ImageGIFEnabled          bool   `json:"imageGIFEnabled"`
	ImageHEICEnabled         bool   `json:"imageHEICEnabled"`
	ImageHeaderMB            int    `json:"imageHeaderMB"`
	ImageHeaderUnlimited     bool   `json:"imageHeaderUnlimited"`
	ImageScanBudgetMB        int    `json:"imageScanBudgetMB"`
	ImageScanBudgetUnlimited bool   `json:"imageScanBudgetUnlimited"`
	EXIFEnabled              bool   `json:"exifEnabled"`
	EXIFFileMB               int    `json:"exifFileMB"`
	EXIFFileUnlimited        bool   `json:"exifFileUnlimited"`
	EXIFTotalMB              int    `json:"exifTotalMB"`
	EXIFTotalUnlimited       bool   `json:"exifTotalUnlimited"`
	TextIndexEnabled         bool   `json:"textIndexEnabled"`
	TextDocumentsEnabled     bool   `json:"textDocumentsEnabled"`
	TextDataEnabled          bool   `json:"textDataEnabled"`
	TextSourceEnabled        bool   `json:"textSourceEnabled"`
	TextFileMB               int    `json:"textFileMB"`
	TextFileUnlimited        bool   `json:"textFileUnlimited"`
	TextTotalMB              int    `json:"textTotalMB"`
	TextTotalUnlimited       bool   `json:"textTotalUnlimited"`
	ImagePreviewEnabled      bool   `json:"imagePreviewEnabled"`
	HEICPreviewEnabled       bool   `json:"heicPreviewEnabled"`
	ImagePreviewMB           int    `json:"imagePreviewMB"`
	ImagePreviewUnlimited    bool   `json:"imagePreviewUnlimited"`
	ThumbnailCacheMB         int    `json:"thumbnailCacheMB"`
	ThumbnailCacheUnlimited  bool   `json:"thumbnailCacheUnlimited"`
	PDFPreviewMB             int    `json:"pdfPreviewMB"`
	VideoPreviewMB           int    `json:"videoPreviewMB"`
	AIEnabled                bool   `json:"aiEnabled"`
	AIProvider               string `json:"aiProvider"`
	AIEndpoint               string `json:"aiEndpoint"`
	AIModel                  string `json:"aiModel"`
	AIFileMB                 int    `json:"aiFileMB"`
	AIFileUnlimited          bool   `json:"aiFileUnlimited"`
	AITotalMB                int    `json:"aiTotalMB"`
	AITotalUnlimited         bool   `json:"aiTotalUnlimited"`
	AITimeoutSeconds         int    `json:"aiTimeoutSeconds"`
}

func Defaults() Settings {
	return Settings{
		Version: 9, VolumeDetectionEnabled: true, BackupEnabled: true, BackupFileMB: 1024, BackupMaxMB: 2048, ArchiveEnabled: true, MaxSnapshots: 10,
		ScanDiagnosticsEnabled: true, ScanDiagnosticFileMB: 2, ScanDiagnosticsTotalMB: 50,
		ImageAnalysisEnabled: true, ImageJPEGEnabled: true, ImagePNGEnabled: true, ImageGIFEnabled: true, ImageHEICEnabled: true,
		ImageHeaderMB: 4, ImageScanBudgetMB: 256, ImageScanBudgetUnlimited: true,
		EXIFFileMB: 8, EXIFTotalMB: 256, EXIFTotalUnlimited: true,
		TextDocumentsEnabled: true, TextDataEnabled: true, TextSourceEnabled: true, TextFileMB: 2, TextTotalMB: 500,
		ImagePreviewEnabled: true, HEICPreviewEnabled: true, ImagePreviewMB: 100, ThumbnailCacheMB: 500, ThumbnailCacheUnlimited: true,
		PDFPreviewMB: 40, VideoPreviewMB: 50,
		AIProvider: "ollama", AIEndpoint: "http://127.0.0.1:11434", AIModel: "qwen2.5:1.5b", AIFileMB: 2, AITotalMB: 100, AITimeoutSeconds: 30,
	}
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
	settings.Version = 9
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
	if settings.ScanDiagnosticFileMB < 1 || settings.ScanDiagnosticFileMB > 1024 {
		return fmt.Errorf("Diagnose-Dateilimit muss zwischen 1 und 1024 MB liegen")
	}
	if settings.ScanDiagnosticsTotalMB < 1 || settings.ScanDiagnosticsTotalMB > 1_000_000 {
		return fmt.Errorf("Diagnose-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.BackupMaxMB < 1 || settings.BackupMaxMB > 1_000_000 {
		return fmt.Errorf("Backup-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.BackupFileMB < 1 || settings.BackupFileMB > 1_000_000 {
		return fmt.Errorf("Backup-Dateilimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.ImagePreviewMB < 1 || settings.ImagePreviewMB > 100 {
		return fmt.Errorf("Bildlimit muss zwischen 1 und 100 MB liegen")
	}
	if settings.ImageHeaderMB < 1 || settings.ImageHeaderMB > 1024 {
		return fmt.Errorf("Header-Limit muss zwischen 1 und 1024 MB liegen")
	}
	if settings.ImageScanBudgetMB < 1 || settings.ImageScanBudgetMB > 1_000_000 {
		return fmt.Errorf("Analyse-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.EXIFFileMB < 1 || settings.EXIFFileMB > 1024 {
		return fmt.Errorf("EXIF-Dateilimit muss zwischen 1 und 1024 MB liegen")
	}
	if settings.EXIFTotalMB < 1 || settings.EXIFTotalMB > 1_000_000 {
		return fmt.Errorf("EXIF-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.TextFileMB < 1 || settings.TextFileMB > 1024 {
		return fmt.Errorf("Textindex-Dateilimit muss zwischen 1 und 1024 MB liegen")
	}
	if settings.TextTotalMB < 1 || settings.TextTotalMB > 1_000_000 {
		return fmt.Errorf("Textindex-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.ThumbnailCacheMB < 1 || settings.ThumbnailCacheMB > 1_000_000 {
		return fmt.Errorf("Vorschau-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.PDFPreviewMB < 1 || settings.PDFPreviewMB > 100 {
		return fmt.Errorf("PDF-Limit muss zwischen 1 und 100 MB liegen")
	}
	if settings.VideoPreviewMB < 1 || settings.VideoPreviewMB > 250 {
		return fmt.Errorf("Videolimit muss zwischen 1 und 250 MB liegen")
	}
	if settings.AIProvider != "ollama" && settings.AIProvider != "openrouter" {
		return fmt.Errorf("KI-Anbieter muss Ollama oder OpenRouter sein")
	}
	if settings.AIEndpoint == "" {
		return fmt.Errorf("KI-Endpunkt darf nicht leer sein")
	}
	if settings.AIModel == "" {
		return fmt.Errorf("KI-Modell darf nicht leer sein")
	}
	if settings.AIFileMB < 1 || settings.AIFileMB > 1024 {
		return fmt.Errorf("KI-Dateilimit muss zwischen 1 und 1024 MB liegen")
	}
	if settings.AITotalMB < 1 || settings.AITotalMB > 1_000_000 {
		return fmt.Errorf("KI-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.AITimeoutSeconds < 5 || settings.AITimeoutSeconds > 300 {
		return fmt.Errorf("KI-Zeitlimit muss zwischen 5 und 300 Sekunden liegen")
	}
	return nil
}
