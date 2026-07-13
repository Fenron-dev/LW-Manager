package scanner

import (
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dennis/vaultapp/internal/exif"
)

type File struct {
	Path, Filename, Extension, MIMEType, Metadata, TextContent string
	Size                                                       int64
	Width, Height                                              int
	CreatedAt, Modified                                        time.Time
}

type Report struct {
	Files   []File
	Bytes   int64
	Skipped int
}

type ImageAnalysisOptions struct {
	Enabled                          bool
	JPEG, PNG, GIF, HEIC             bool
	PerFileBytes, TotalBytes         int64
	PerFileUnlimited, TotalUnlimited bool
}

type EXIFAnalysisOptions struct {
	Enabled                          bool
	PerFileBytes, TotalBytes         int64
	PerFileUnlimited, TotalUnlimited bool
}

type TextIndexOptions struct {
	Enabled                          bool
	Documents, Data, SourceCode      bool
	PerFileBytes, TotalBytes         int64
	PerFileUnlimited, TotalUnlimited bool
}

type countingReader struct {
	reader io.Reader
	read   int64
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.read += int64(count)
	return count, err
}

func Scan(ctx context.Context, sourceRoot, excludedRoot string, imageOptions ImageAnalysisOptions, exifOptions EXIFAnalysisOptions, textOptions TextIndexOptions, progress func(int, string)) (Report, error) {
	root, err := filepath.Abs(sourceRoot)
	if err != nil {
		return Report{}, err
	}
	excluded, _ := filepath.Abs(excludedRoot)
	report := Report{Files: make([]File, 0, 1024)}
	var imageBytesRead int64
	var exifBytesRead int64
	var textBytesRead int64
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			report.Skipped++
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			if sameOrChild(path, excluded) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			report.Skipped++
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			report.Skipped++
			return nil
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		mediaType := mime.TypeByExtension(extension)
		if separator := strings.IndexByte(mediaType, ';'); separator >= 0 {
			mediaType = mediaType[:separator]
		}
		width, height := imageDimensions(path, extension, imageOptions, &imageBytesRead)
		metadata := imageEXIF(path, extension, exifOptions, &exifBytesRead)
		textContent := textPreview(path, extension, textOptions, &textBytesRead)
		report.Files = append(report.Files, File{Path: filepath.ToSlash(relative), Filename: entry.Name(), Extension: strings.TrimPrefix(extension, "."), Size: info.Size(), MIMEType: mediaType, Metadata: metadata, TextContent: textContent, Width: width, Height: height, Modified: info.ModTime()})
		report.Bytes += info.Size()
		if progress != nil {
			progress(len(report.Files), relative)
		}
		return nil
	})
	return report, err
}

func textPreview(path, extension string, options TextIndexOptions, totalRead *int64) string {
	if !options.Enabled || !textExtensionEnabled(extension, options) {
		return ""
	}
	limit := options.PerFileBytes
	if options.PerFileUnlimited || limit <= 0 {
		limit = 1<<63 - 1
	}
	if !options.TotalUnlimited {
		remaining := options.TotalBytes - *totalRead
		if remaining <= 0 {
			return ""
		}
		if remaining < limit {
			limit = remaining
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	reader := &countingReader{reader: io.LimitReader(file, limit)}
	data, err := io.ReadAll(reader)
	*totalRead += reader.read
	if err != nil || len(data) == 0 {
		return ""
	}
	for _, value := range data {
		if value == 0 {
			return ""
		}
	}
	if !utf8.Valid(data) {
		valid := false
		for removed := 0; removed < 4 && len(data) > 0; removed++ {
			data = data[:len(data)-1]
			if utf8.Valid(data) {
				valid = true
				break
			}
		}
		if !valid {
			return ""
		}
	}
	return strings.TrimSpace(string(data))
}

func textExtensionEnabled(extension string, options TextIndexOptions) bool {
	switch extension {
	case ".txt", ".md", ".markdown", ".log", ".csv", ".tsv", ".rtf":
		return options.Documents
	case ".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".conf":
		return options.Data
	case ".js", ".jsx", ".ts", ".tsx", ".go", ".rs", ".py", ".java", ".c", ".h", ".cpp", ".hpp", ".cs", ".css", ".scss", ".html", ".htm", ".dart", ".sh", ".zsh", ".sql":
		return options.SourceCode
	default:
		return false
	}
}

func imageEXIF(path, extension string, options EXIFAnalysisOptions, totalRead *int64) string {
	if !options.Enabled || extension != ".jpg" && extension != ".jpeg" {
		return ""
	}
	limit := options.PerFileBytes
	if options.PerFileUnlimited || limit <= 0 {
		limit = 1<<63 - 1
	}
	if !options.TotalUnlimited {
		remaining := options.TotalBytes - *totalRead
		if remaining <= 0 {
			return ""
		}
		if remaining < limit {
			limit = remaining
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	reader := &countingReader{reader: io.LimitReader(file, limit)}
	metadata, err := exif.ParseJPEGReader(reader)
	*totalRead += reader.read
	if err != nil {
		return ""
	}
	return exif.Encode(metadata)
}

func imageDimensions(path, extension string, options ImageAnalysisOptions, totalRead *int64) (int, int) {
	if !options.Enabled {
		return 0, 0
	}
	switch extension {
	case ".jpg", ".jpeg":
		if !options.JPEG {
			return 0, 0
		}
	case ".png":
		if !options.PNG {
			return 0, 0
		}
	case ".gif":
		if !options.GIF {
			return 0, 0
		}
	case ".heic", ".heif":
		if !options.HEIC {
			return 0, 0
		}
	default:
		return 0, 0
	}
	limit := options.PerFileBytes
	if options.PerFileUnlimited || limit <= 0 {
		limit = 1<<63 - 1
	}
	if !options.TotalUnlimited {
		remaining := options.TotalBytes - *totalRead
		if remaining <= 0 {
			return 0, 0
		}
		if remaining < limit {
			limit = remaining
		}
	}
	file, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	reader := &countingReader{reader: io.LimitReader(file, limit)}
	if extension == ".heic" || extension == ".heif" {
		width, height, err := heifDimensions(reader)
		*totalRead += reader.read
		if err != nil || width <= 0 || height <= 0 {
			return 0, 0
		}
		return width, height
	}
	config, _, err := image.DecodeConfig(reader)
	*totalRead += reader.read
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return 0, 0
	}
	return config.Width, config.Height
}

func sameOrChild(path, parent string) bool {
	if parent == "" {
		return false
	}
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
