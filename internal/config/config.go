package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type ScanProfile struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	ExclusionsEnabled    bool     `json:"exclusionsEnabled"`
	ExcludeSystem        bool     `json:"excludeSystem"`
	ExcludeDevelopment   bool     `json:"excludeDevelopment"`
	ExcludedPatterns     []string `json:"excludedPatterns"`
	ImageAnalysisEnabled bool     `json:"imageAnalysisEnabled"`
	EXIFEnabled          bool     `json:"exifEnabled"`
	TextIndexEnabled     bool     `json:"textIndexEnabled"`
	TextDocumentsEnabled bool     `json:"textDocumentsEnabled"`
	TextPDFEnabled       bool     `json:"textPDFEnabled"`
	TextOfficeEnabled    bool     `json:"textOfficeEnabled"`
	TextDataEnabled      bool     `json:"textDataEnabled"`
	TextSourceEnabled    bool     `json:"textSourceEnabled"`
}

type Settings struct {
	Version                          int           `json:"version"`
	VolumeDetectionEnabled           bool          `json:"volumeDetectionEnabled"`
	BackupEnabled                    bool          `json:"backupEnabled"`
	BackupIncludeThumbnails          bool          `json:"backupIncludeThumbnails"`
	BackupFileMB                     int           `json:"backupFileMB"`
	BackupFileUnlimited              bool          `json:"backupFileUnlimited"`
	BackupMaxMB                      int           `json:"backupMaxMB"`
	BackupUnlimited                  bool          `json:"backupUnlimited"`
	ArchiveEnabled                   bool          `json:"archiveEnabled"`
	MaxSnapshots                     int           `json:"maxSnapshots"`
	ScanDiagnosticsEnabled           bool          `json:"scanDiagnosticsEnabled"`
	ScanDiagnosticFileMB             int           `json:"scanDiagnosticFileMB"`
	ScanDiagnosticUnlimited          bool          `json:"scanDiagnosticUnlimited"`
	ScanDiagnosticsTotalMB           int           `json:"scanDiagnosticsTotalMB"`
	ScanDiagnosticsUnlimited         bool          `json:"scanDiagnosticsUnlimited"`
	ScanExclusionsEnabled            bool          `json:"scanExclusionsEnabled"`
	ScanExcludeSystem                bool          `json:"scanExcludeSystem"`
	ScanExcludeDevelopment           bool          `json:"scanExcludeDevelopment"`
	ScanExcludedPatterns             []string      `json:"scanExcludedPatterns"`
	ScanProfiles                     []ScanProfile `json:"scanProfiles"`
	ImageAnalysisEnabled             bool          `json:"imageAnalysisEnabled"`
	ImageJPEGEnabled                 bool          `json:"imageJPEGEnabled"`
	ImagePNGEnabled                  bool          `json:"imagePNGEnabled"`
	ImageGIFEnabled                  bool          `json:"imageGIFEnabled"`
	ImageHEICEnabled                 bool          `json:"imageHEICEnabled"`
	ImageHeaderMB                    int           `json:"imageHeaderMB"`
	ImageHeaderUnlimited             bool          `json:"imageHeaderUnlimited"`
	ImageScanBudgetMB                int           `json:"imageScanBudgetMB"`
	ImageScanBudgetUnlimited         bool          `json:"imageScanBudgetUnlimited"`
	EXIFEnabled                      bool          `json:"exifEnabled"`
	EXIFFileMB                       int           `json:"exifFileMB"`
	EXIFFileUnlimited                bool          `json:"exifFileUnlimited"`
	EXIFTotalMB                      int           `json:"exifTotalMB"`
	EXIFTotalUnlimited               bool          `json:"exifTotalUnlimited"`
	TextIndexEnabled                 bool          `json:"textIndexEnabled"`
	TextDocumentsEnabled             bool          `json:"textDocumentsEnabled"`
	TextPDFEnabled                   bool          `json:"textPDFEnabled"`
	TextOfficeEnabled                bool          `json:"textOfficeEnabled"`
	TextDataEnabled                  bool          `json:"textDataEnabled"`
	TextSourceEnabled                bool          `json:"textSourceEnabled"`
	TextFileMB                       int           `json:"textFileMB"`
	TextFileUnlimited                bool          `json:"textFileUnlimited"`
	TextTotalMB                      int           `json:"textTotalMB"`
	TextTotalUnlimited               bool          `json:"textTotalUnlimited"`
	TextStoredMB                     int           `json:"textStoredMB"`
	TextStoredUnlimited              bool          `json:"textStoredUnlimited"`
	ImagePreviewEnabled              bool          `json:"imagePreviewEnabled"`
	HEICPreviewEnabled               bool          `json:"heicPreviewEnabled"`
	ImagePreviewMB                   int           `json:"imagePreviewMB"`
	ImagePreviewUnlimited            bool          `json:"imagePreviewUnlimited"`
	ThumbnailCacheMB                 int           `json:"thumbnailCacheMB"`
	ThumbnailCacheUnlimited          bool          `json:"thumbnailCacheUnlimited"`
	PDFPreviewEnabled                bool          `json:"pdfPreviewEnabled"`
	PDFPreviewMB                     int           `json:"pdfPreviewMB"`
	PDFPreviewUnlimited              bool          `json:"pdfPreviewUnlimited"`
	VideoPreviewEnabled              bool          `json:"videoPreviewEnabled"`
	VideoPreviewMB                   int           `json:"videoPreviewMB"`
	VideoPreviewUnlimited            bool          `json:"videoPreviewUnlimited"`
	AIEnabled                        bool          `json:"aiEnabled"`
	AIProvider                       string        `json:"aiProvider"`
	AIEndpoint                       string        `json:"aiEndpoint"`
	AIModel                          string        `json:"aiModel"`
	AIFileMB                         int           `json:"aiFileMB"`
	AIFileUnlimited                  bool          `json:"aiFileUnlimited"`
	AITotalMB                        int           `json:"aiTotalMB"`
	AITotalUnlimited                 bool          `json:"aiTotalUnlimited"`
	AITimeoutSeconds                 int           `json:"aiTimeoutSeconds"`
	AIVisionEnabled                  bool          `json:"aiVisionEnabled"`
	AIVisionModel                    string        `json:"aiVisionModel"`
	AIVisionFileMB                   int           `json:"aiVisionFileMB"`
	AIVisionFileUnlimited            bool          `json:"aiVisionFileUnlimited"`
	AIVisionTotalMB                  int           `json:"aiVisionTotalMB"`
	AIVisionTotalUnlimited           bool          `json:"aiVisionTotalUnlimited"`
	CatalogExportEnabled             bool          `json:"catalogExportEnabled"`
	CatalogExportMaxMB               int           `json:"catalogExportMaxMB"`
	CatalogExportUnlimited           bool          `json:"catalogExportUnlimited"`
	DuplicateCheckEnabled            bool          `json:"duplicateCheckEnabled"`
	DuplicateFileMB                  int           `json:"duplicateFileMB"`
	DuplicateFileUnlimited           bool          `json:"duplicateFileUnlimited"`
	DuplicateTotalMB                 int           `json:"duplicateTotalMB"`
	DuplicateTotalUnlimited          bool          `json:"duplicateTotalUnlimited"`
	DuplicateQuarantineEnabled       bool          `json:"duplicateQuarantineEnabled"`
	DuplicateQuarantineFileMB        int           `json:"duplicateQuarantineFileMB"`
	DuplicateQuarantineFileUnlimited bool          `json:"duplicateQuarantineFileUnlimited"`
	DuplicateQuarantineTotalMB       int           `json:"duplicateQuarantineTotalMB"`
	DuplicateQuarantineUnlimited     bool          `json:"duplicateQuarantineUnlimited"`
	DuplicatePermanentDeleteEnabled  bool          `json:"duplicatePermanentDeleteEnabled"`
}

func Defaults() Settings {
	return Settings{
		Version: 19, VolumeDetectionEnabled: true, BackupEnabled: true, BackupFileMB: 1024, BackupMaxMB: 2048, ArchiveEnabled: true, MaxSnapshots: 10,
		ScanDiagnosticsEnabled: true, ScanDiagnosticFileMB: 2, ScanDiagnosticsTotalMB: 50,
		ScanExcludeSystem: true, ScanExcludeDevelopment: true, ScanExcludedPatterns: []string{}, ScanProfiles: []ScanProfile{},
		ImageAnalysisEnabled: true, ImageJPEGEnabled: true, ImagePNGEnabled: true, ImageGIFEnabled: true, ImageHEICEnabled: true,
		ImageHeaderMB: 4, ImageScanBudgetMB: 256, ImageScanBudgetUnlimited: true,
		EXIFFileMB: 8, EXIFTotalMB: 256, EXIFTotalUnlimited: true,
		TextDocumentsEnabled: true, TextDataEnabled: true, TextSourceEnabled: true, TextFileMB: 2, TextTotalMB: 500, TextStoredMB: 500,
		ImagePreviewEnabled: true, HEICPreviewEnabled: true, ImagePreviewMB: 100, ThumbnailCacheMB: 500, ThumbnailCacheUnlimited: true,
		PDFPreviewEnabled: true, PDFPreviewMB: 40, VideoPreviewEnabled: true, VideoPreviewMB: 50,
		AIProvider: "ollama", AIEndpoint: "http://127.0.0.1:11434", AIModel: "qwen2.5:1.5b", AIFileMB: 2, AITotalMB: 100, AITimeoutSeconds: 30,
		AIVisionModel: "gemma3:4b", AIVisionFileMB: 25, AIVisionTotalMB: 100,
		CatalogExportEnabled: true, CatalogExportMaxMB: 100,
		DuplicateCheckEnabled: true, DuplicateFileMB: 1024, DuplicateTotalMB: 2048,
		DuplicateQuarantineEnabled: true, DuplicateQuarantineFileMB: 10_240, DuplicateQuarantineTotalMB: 102_400,
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
	if settings.Version < 10 && settings.AIProvider == "openrouter" {
		settings.AIVisionModel = "openrouter/auto"
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
	settings.Version = 19
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
	if err := validatePatterns(settings.ScanExcludedPatterns); err != nil {
		return err
	}
	if len(settings.ScanProfiles) > 50 {
		return fmt.Errorf("höchstens 50 Scanprofile sind erlaubt")
	}
	profileIDs := make(map[string]bool, len(settings.ScanProfiles))
	profileNames := make(map[string]bool, len(settings.ScanProfiles))
	for _, profile := range settings.ScanProfiles {
		id := strings.TrimSpace(profile.ID)
		name := strings.TrimSpace(profile.Name)
		if id == "" || len(id) > 80 || strings.ContainsAny(id, " /\\") {
			return fmt.Errorf("Scanprofil-ID %q ist ungültig", profile.ID)
		}
		if profileIDs[id] {
			return fmt.Errorf("Scanprofil-ID %q ist doppelt vorhanden", id)
		}
		profileIDs[id] = true
		if name == "" || len(name) > 100 {
			return fmt.Errorf("Scanprofilname muss 1 bis 100 Zeichen lang sein")
		}
		normalizedName := strings.ToLower(name)
		if profileNames[normalizedName] {
			return fmt.Errorf("Scanprofilname %q ist doppelt vorhanden", name)
		}
		profileNames[normalizedName] = true
		if err := validatePatterns(profile.ExcludedPatterns); err != nil {
			return fmt.Errorf("Scanprofil %q: %w", name, err)
		}
	}
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
	if settings.TextStoredMB < 1 || settings.TextStoredMB > 1_000_000 {
		return fmt.Errorf("Textindex-Speicherlimit muss zwischen 1 und 1.000.000 MB liegen")
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
	endpoint, err := url.Parse(settings.AIEndpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return fmt.Errorf("KI-Endpunkt muss eine gültige HTTP-Adresse sein")
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
	if settings.AIVisionModel == "" {
		return fmt.Errorf("Vision-Modell darf nicht leer sein")
	}
	if settings.AIVisionFileMB < 1 || settings.AIVisionFileMB > 1024 {
		return fmt.Errorf("Vision-Dateilimit muss zwischen 1 und 1024 MB liegen")
	}
	if settings.AIVisionTotalMB < 1 || settings.AIVisionTotalMB > 1_000_000 {
		return fmt.Errorf("Vision-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.CatalogExportMaxMB < 1 || settings.CatalogExportMaxMB > 1_000_000 {
		return fmt.Errorf("Katalogexport-Limit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.DuplicateFileMB < 1 || settings.DuplicateFileMB > 1_000_000 {
		return fmt.Errorf("Duplikat-Dateilimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.DuplicateTotalMB < 1 || settings.DuplicateTotalMB > 1_000_000 {
		return fmt.Errorf("Duplikat-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.DuplicateQuarantineFileMB < 1 || settings.DuplicateQuarantineFileMB > 1_000_000 {
		return fmt.Errorf("Quarantäne-Dateilimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	if settings.DuplicateQuarantineTotalMB < 1 || settings.DuplicateQuarantineTotalMB > 1_000_000 {
		return fmt.Errorf("Quarantäne-Gesamtlimit muss zwischen 1 und 1.000.000 MB liegen")
	}
	return nil
}

func validatePatterns(patterns []string) error {
	if len(patterns) > 100 {
		return fmt.Errorf("höchstens 100 eigene Scan-Ausschlussmuster sind erlaubt")
	}
	for _, raw := range patterns {
		pattern := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
		if pattern == "" {
			continue
		}
		if len(pattern) > 200 || strings.HasPrefix(pattern, "/") {
			return fmt.Errorf("ungültiges Scan-Ausschlussmuster %q", raw)
		}
		for _, component := range strings.Split(pattern, "/") {
			if component == ".." {
				return fmt.Errorf("Scan-Ausschlussmuster dürfen kein .. enthalten")
			}
		}
		if _, err := path.Match(pattern, pattern); err != nil {
			return fmt.Errorf("ungültiges Scan-Ausschlussmuster %q: %w", raw, err)
		}
	}
	return nil
}
