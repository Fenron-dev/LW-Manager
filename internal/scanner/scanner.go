package scanner

import (
	"context"
	"io/fs"
	"mime"
	"path/filepath"
	"strings"
	"time"
)

type File struct {
	Path, Filename, Extension, MIMEType string
	Size                                int64
	CreatedAt, Modified                 time.Time
}

type Report struct {
	Files   []File
	Bytes   int64
	Skipped int
}

func Scan(ctx context.Context, sourceRoot, excludedRoot string, progress func(int, string)) (Report, error) {
	root, err := filepath.Abs(sourceRoot)
	if err != nil {
		return Report{}, err
	}
	excluded, _ := filepath.Abs(excludedRoot)
	report := Report{Files: make([]File, 0, 1024)}
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
		report.Files = append(report.Files, File{Path: filepath.ToSlash(relative), Filename: entry.Name(), Extension: strings.TrimPrefix(extension, "."), Size: info.Size(), MIMEType: mediaType, Modified: info.ModTime()})
		report.Bytes += info.Size()
		if progress != nil {
			progress(len(report.Files), relative)
		}
		return nil
	})
	return report, err
}

func sameOrChild(path, parent string) bool {
	if parent == "" {
		return false
	}
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
