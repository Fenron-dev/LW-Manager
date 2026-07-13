package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Settings struct {
	Version                  int  `json:"version"`
	VolumeDetectionEnabled   bool `json:"volumeDetectionEnabled"`
	ArchiveEnabled           bool `json:"archiveEnabled"`
	MaxSnapshots             int  `json:"maxSnapshots"`
	ImageAnalysisEnabled     bool `json:"imageAnalysisEnabled"`
	ImageJPEGEnabled         bool `json:"imageJPEGEnabled"`
	ImagePNGEnabled          bool `json:"imagePNGEnabled"`
	ImageGIFEnabled          bool `json:"imageGIFEnabled"`
	ImageHEICEnabled         bool `json:"imageHEICEnabled"`
	ImageHeaderMB            int  `json:"imageHeaderMB"`
	ImageHeaderUnlimited     bool `json:"imageHeaderUnlimited"`
	ImageScanBudgetMB        int  `json:"imageScanBudgetMB"`
	ImageScanBudgetUnlimited bool `json:"imageScanBudgetUnlimited"`
	EXIFEnabled              bool `json:"exifEnabled"`
	EXIFFileMB               int  `json:"exifFileMB"`
	EXIFFileUnlimited        bool `json:"exifFileUnlimited"`
	EXIFTotalMB              int  `json:"exifTotalMB"`
	EXIFTotalUnlimited       bool `json:"exifTotalUnlimited"`
	TextIndexEnabled         bool `json:"textIndexEnabled"`
	TextDocumentsEnabled     bool `json:"textDocumentsEnabled"`
	TextDataEnabled          bool `json:"textDataEnabled"`
	TextSourceEnabled        bool `json:"textSourceEnabled"`
	TextFileMB               int  `json:"textFileMB"`
	TextFileUnlimited        bool `json:"textFileUnlimited"`
	TextTotalMB              int  `json:"textTotalMB"`
	TextTotalUnlimited       bool `json:"textTotalUnlimited"`
	ImagePreviewEnabled      bool `json:"imagePreviewEnabled"`
	HEICPreviewEnabled       bool `json:"heicPreviewEnabled"`
	ImagePreviewMB           int  `json:"imagePreviewMB"`
	ImagePreviewUnlimited    bool `json:"imagePreviewUnlimited"`
	ThumbnailCacheMB         int  `json:"thumbnailCacheMB"`
	ThumbnailCacheUnlimited  bool `json:"thumbnailCacheUnlimited"`
	PDFPreviewMB             int  `json:"pdfPreviewMB"`
	VideoPreviewMB           int  `json:"videoPreviewMB"`
}

func Defaults() Settings {
	return Settings{
		Version: 6, VolumeDetectionEnabled: true, ArchiveEnabled: true, MaxSnapshots: 10,
		ImageAnalysisEnabled: true, ImageJPEGEnabled: true, ImagePNGEnabled: true, ImageGIFEnabled: true, ImageHEICEnabled: true,
		ImageHeaderMB: 4, ImageScanBudgetMB: 256, ImageScanBudgetUnlimited: true,
		EXIFFileMB: 8, EXIFTotalMB: 256, EXIFTotalUnlimited: true,
		TextDocumentsEnabled: true, TextDataEnabled: true, TextSourceEnabled: true, TextFileMB: 2, TextTotalMB: 500,
		ImagePreviewEnabled: true, HEICPreviewEnabled: true, ImagePreviewMB: 100, ThumbnailCacheMB: 500, ThumbnailCacheUnlimited: true,
		PDFPreviewMB: 40, VideoPreviewMB: 50,
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
	settings.Version = 6
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
	return nil
}
