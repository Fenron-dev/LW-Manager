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
	return DataURLWithLimits(source, cacheDir, identity, 100, 40)
}

func DataURLWithLimits(source, cacheDir, identity string, imageLimitMB, pdfLimitMB int) (string, error) {
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}
	imageLimit := int64(imageLimitMB) << 20
	pdfLimit := int64(pdfLimitMB) << 20
	extension := filepath.Ext(source)
	if info.Size() > imageLimit && !strings.EqualFold(extension, ".pdf") {
		return "", fmt.Errorf("Bild ist größer als %d MB", imageLimitMB)
	}
	if strings.EqualFold(extension, ".pdf") {
		if info.Size() > pdfLimit {
			return "", fmt.Errorf("PDF-Vorschau ist größer als %d MB", pdfLimitMB)
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
