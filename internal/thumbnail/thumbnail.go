package thumbnail

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
)

func DataURL(source, cacheDir, identity string) (string, error) {
	return DataURLWithLimits(source, cacheDir, identity, Limits{ImageEnabled: true, ImageMB: 100, CacheUnlimited: true, PDFMB: 40, VideoMB: 50})
}

type Limits struct {
	ImageEnabled, ImageUnlimited, CacheUnlimited bool
	ImageMB, CacheMB, PDFMB, VideoMB             int
}

func DataURLWithLimits(source, cacheDir, identity string, limits Limits) (string, error) {
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	imageLimit := int64(limits.ImageMB) << 20
	pdfLimit := int64(limits.PDFMB) << 20
	videoLimit := int64(limits.VideoMB) << 20
	extension := filepath.Ext(source)
	videoMIME := videoMIMEType(extension)
	isImage := !strings.EqualFold(extension, ".pdf") && videoMIME == ""
	if isImage && !limits.ImageEnabled {
		return "", fmt.Errorf("Bildvorschauen sind in den Einstellungen deaktiviert")
	}
	if isImage && !limits.ImageUnlimited && info.Size() > imageLimit {
		return "", fmt.Errorf("Bild ist größer als %d MB", limits.ImageMB)
	}
	if strings.EqualFold(extension, ".pdf") {
		if info.Size() > pdfLimit {
			return "", fmt.Errorf("PDF-Vorschau ist größer als %d MB", limits.PDFMB)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return "", err
		}
		return encodeMIME(data, "application/pdf"), nil
	}
	if strings.EqualFold(extension, ".webp") {
		data, err := os.ReadFile(source)
		if err != nil {
			return "", err
		}
		return encodeMIME(data, "image/webp"), nil
	}
	if videoMIME != "" {
		if info.Size() > videoLimit {
			return "", fmt.Errorf("Video-Vorschau ist größer als %d MB", limits.VideoMB)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			return "", err
		}
		return encodeMIME(data, videoMIME), nil
	}
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(source+identity)))
	cachePath := filepath.Join(cacheDir, key+".jpg")
	if cached, err := os.ReadFile(cachePath); err == nil {
		return encode(cached), nil
	}
	file, err := os.Open(source)
	if err != nil {
		return "", err
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return "", fmt.Errorf("Bildformat nicht unterstützt: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 100_000_000 {
		return "", fmt.Errorf("Bildabmessungen sind zu groß")
	}
	if _, err := file.Seek(0, 0); err != nil {
		return "", err
	}
	sourceImage, _, err := image.Decode(file)
	if err != nil {
		return "", err
	}
	width, height := fit(config.Width, config.Height, 900, 600)
	preview := image.NewRGBA(image.Rect(0, 0, width, height))
	bounds := sourceImage.Bounds()
	for y := 0; y < height; y++ {
		sy := bounds.Min.Y + y*bounds.Dy()/height
		for x := 0; x < width; x++ {
			sx := bounds.Min.X + x*bounds.Dx()/width
			preview.Set(x, y, sourceImage.At(sx, sy))
		}
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", err
	}
	temporary := cachePath + ".tmp"
	output, err := os.Create(temporary)
	if err != nil {
		return "", err
	}
	encodeErr := jpeg.Encode(output, preview, &jpeg.Options{Quality: 82})
	closeErr := output.Close()
	if encodeErr != nil {
		_ = os.Remove(temporary)
		return "", encodeErr
	}
	if closeErr != nil {
		_ = os.Remove(temporary)
		return "", closeErr
	}
	if !limits.CacheUnlimited {
		temporaryInfo, statErr := os.Stat(temporary)
		if statErr != nil {
			_ = os.Remove(temporary)
			return "", statErr
		}
		if err := makeCacheSpace(cacheDir, int64(limits.CacheMB)<<20, temporaryInfo.Size(), cachePath); err != nil {
			_ = os.Remove(temporary)
			return "", err
		}
	}
	if err := os.Rename(temporary, cachePath); err != nil {
		_ = os.Remove(temporary)
		return "", err
	}
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return "", err
	}
	return encode(data), nil
}

func makeCacheSpace(cacheDir string, maximum, incoming int64, keep string) error {
	if incoming > maximum {
		return fmt.Errorf("Vorschaudatei überschreitet das Cache-Gesamtlimit")
	}
	entries, err := os.ReadDir(cacheDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	type cachedFile struct {
		path     string
		size     int64
		modified int64
	}
	files := make([]cachedFile, 0, len(entries))
	var used int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".jpg") {
			continue
		}
		path := filepath.Join(cacheDir, entry.Name())
		if path == keep {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		used += info.Size()
		files = append(files, cachedFile{path: path, size: info.Size(), modified: info.ModTime().UnixNano()})
	}
	for used+incoming > maximum && len(files) > 0 {
		oldest := 0
		for index := 1; index < len(files); index++ {
			if files[index].modified < files[oldest].modified {
				oldest = index
			}
		}
		if err := os.Remove(files[oldest].path); err != nil && !os.IsNotExist(err) {
			return err
		}
		used -= files[oldest].size
		files = append(files[:oldest], files[oldest+1:]...)
	}
	return nil
}

func TrimCache(cacheDir string, maximum int64) error {
	return makeCacheSpace(cacheDir, maximum, 0, "")
}

func videoMIMEType(extension string) string {
	switch strings.ToLower(extension) {
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".ogv", ".ogg":
		return "video/ogg"
	case ".mov":
		return "video/quicktime"
	default:
		return ""
	}
}

func fit(width, height, maxWidth, maxHeight int) (int, int) {
	if width <= maxWidth && height <= maxHeight {
		return width, height
	}
	ratio := min(float64(maxWidth)/float64(width), float64(maxHeight)/float64(height))
	return max(1, int(float64(width)*ratio)), max(1, int(float64(height)*ratio))
}

func encode(data []byte) string {
	return encodeMIME(data, "image/jpeg")
}

func encodeMIME(data []byte, mimeType string) string {
	return "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
}
