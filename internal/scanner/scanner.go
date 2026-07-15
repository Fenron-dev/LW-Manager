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
	"path"
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
	Files           []File
	Bytes           int64
	Skipped         int
	Excluded        int
	Issues          []Issue
	IssuesTruncated bool
}

type Issue struct {
	Path      string `json:"path"`
	Operation string `json:"operation"`
	Message   string `json:"message"`
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
	Enabled                                  bool
	Documents, PDF, Office, Data, SourceCode bool
	PerFileBytes, TotalBytes                 int64
	StoredBytes                              int64
	PerFileUnlimited, TotalUnlimited         bool
	StoredLimitEnabled                       bool
}

type ExclusionOptions struct {
	Enabled, System, Development bool
	Patterns                     []string
}

var systemExclusions = []string{".Spotlight-V100", ".Trashes", ".fseventsd", "$RECYCLE.BIN", "System Volume Information", "Thumbs.db", ".DS_Store"}
var developmentExclusions = []string{".git", ".svn", ".hg", "node_modules", ".dart_tool", ".gradle", "Pods", "DerivedData", "__pycache__", "build", "dist", "target", ".next", ".nuxt"}

type countingReader struct {
	reader io.Reader
	read   int64
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.read += int64(count)
	return count, err
}

func Scan(ctx context.Context, sourceRoot, excludedRoot string, imageOptions ImageAnalysisOptions, exifOptions EXIFAnalysisOptions, textOptions TextIndexOptions, exclusions ExclusionOptions, progress func(int, string)) (Report, error) {
	root, err := filepath.Abs(sourceRoot)
	if err != nil {
		return Report{}, err
	}
	excluded, _ := filepath.Abs(excludedRoot)
	report := Report{Files: make([]File, 0, 1024), Issues: make([]Issue, 0)}
	addIssue := func(path, operation string, issue error) {
		report.Skipped++
		if len(report.Issues) >= 500 {
			report.IssuesTruncated = true
			return
		}
		relative := path
		if value, relErr := filepath.Rel(root, path); relErr == nil {
			relative = value
		}
		report.Issues = append(report.Issues, Issue{Path: filepath.ToSlash(relative), Operation: operation, Message: issue.Error()})
	}
	var imageBytesRead int64
	var exifBytesRead int64
	var textBytesRead int64
	var textBytesStored int64
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if walkErr != nil {
			if path == root {
				return walkErr
			}
			addIssue(path, "Verzeichnis lesen", walkErr)
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr == nil && relative != "." && excludedByRule(filepath.ToSlash(relative), exclusions) {
			report.Excluded++
			if entry.IsDir() {
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
			addIssue(path, "Dateiinformationen lesen", err)
			return nil
		}
		if relErr != nil {
			addIssue(path, "Relativen Pfad bestimmen", relErr)
			return nil
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		mediaType := mime.TypeByExtension(extension)
		if separator := strings.IndexByte(mediaType, ';'); separator >= 0 {
			mediaType = mediaType[:separator]
		}
		width, height := imageDimensions(path, extension, imageOptions, &imageBytesRead)
		metadata := imageEXIF(path, extension, exifOptions, &exifBytesRead)
		textContent := textPreview(path, extension, textOptions, &textBytesRead, &textBytesStored)
		report.Files = append(report.Files, File{Path: filepath.ToSlash(relative), Filename: entry.Name(), Extension: strings.TrimPrefix(extension, "."), Size: info.Size(), MIMEType: mediaType, Metadata: metadata, TextContent: textContent, Width: width, Height: height, Modified: info.ModTime()})
		report.Bytes += info.Size()
		if progress != nil {
			progress(len(report.Files), relative)
		}
		return nil
	})
	return report, err
}

func excludedByRule(relative string, options ExclusionOptions) bool {
	if !options.Enabled {
		return false
	}
	patterns := make([]string, 0, len(options.Patterns)+len(systemExclusions)+len(developmentExclusions))
	if options.System {
		patterns = append(patterns, systemExclusions...)
	}
	if options.Development {
		patterns = append(patterns, developmentExclusions...)
	}
	patterns = append(patterns, options.Patterns...)
	segments := strings.Split(relative, "/")
	for _, raw := range patterns {
		pattern := strings.Trim(strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/")), "/")
		if pattern == "" {
			continue
		}
		if strings.Contains(pattern, "/") {
			if matched, _ := path.Match(pattern, relative); matched {
				return true
			}
			continue
		}
		for _, segment := range segments {
			if matched, _ := path.Match(pattern, segment); matched {
				return true
			}
		}
	}
	return false
}

func textPreview(path, extension string, options TextIndexOptions, totalRead, totalStored *int64) string {
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
	storedLimit := int64(1<<63 - 1)
	if options.StoredLimitEnabled {
		storedLimit = options.StoredBytes - *totalStored
		if storedLimit <= 0 {
			return ""
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
	if extension == ".pdf" {
		text := extractPDFText(data, storedLimit)
		*totalStored += int64(len(text))
		return text
	}
	if extension == ".docx" || extension == ".odt" {
		text := extractOfficeText(data, extension, storedLimit)
		*totalStored += int64(len(text))
		return text
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
	text := strings.TrimSpace(string(data))
	text = truncateUTF8(text, storedLimit)
	*totalStored += int64(len(text))
	return text
}

func truncateUTF8(value string, maximum int64) string {
	if maximum < 0 || int64(len(value)) <= maximum {
		return value
	}
	value = value[:maximum]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value)
}

func textExtensionEnabled(extension string, options TextIndexOptions) bool {
	switch extension {
	case ".pdf":
		return options.PDF
	case ".docx", ".odt":
		return options.Office
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
