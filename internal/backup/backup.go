package backup

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrLimit = errors.New("Backup überschreitet das eingestellte Gesamtlimit")

type Limits struct {
	PerFileBytes int64
	TotalBytes   int64
}

type Source struct {
	Path string
	Name string
}

type Result struct {
	Files int   `json:"files"`
	Bytes int64 `json:"bytes"`
}

func DirectorySources(root, prefix string) ([]Source, error) {
	sources := []Source{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		sources = append(sources, Source{Path: path, Name: filepath.ToSlash(filepath.Join(prefix, relative))})
		return nil
	})
	sort.Slice(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
	return sources, err
}

type limitedWriter struct {
	writer  io.Writer
	maximum int64
	written int64
}

func (writer *limitedWriter) Write(data []byte) (int, error) {
	if writer.maximum > 0 && writer.written+int64(len(data)) > writer.maximum {
		allowed := writer.maximum - writer.written
		if allowed <= 0 {
			return 0, ErrLimit
		}
		count, err := writer.writer.Write(data[:allowed])
		writer.written += int64(count)
		if err != nil {
			return count, err
		}
		return count, ErrLimit
	}
	count, err := writer.writer.Write(data)
	writer.written += int64(count)
	return count, err
}

func Create(destination string, sources []Source, limits Limits) (Result, error) {
	if strings.TrimSpace(destination) == "" {
		return Result{}, fmt.Errorf("kein Backup-Ziel angegeben")
	}
	directory := filepath.Dir(destination)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return Result{}, err
	}
	temporary, err := os.CreateTemp(directory, ".vaultapp-backup-*.tmp")
	if err != nil {
		return Result{}, err
	}
	temporaryPath := temporary.Name()
	completed := false
	defer func() {
		_ = temporary.Close()
		if !completed {
			_ = os.Remove(temporaryPath)
		}
	}()
	limited := &limitedWriter{writer: temporary, maximum: limits.TotalBytes}
	archive := zip.NewWriter(limited)
	manifest, _ := json.MarshalIndent(map[string]any{
		"format": "VaultApp-Backup", "version": 1, "createdAt": time.Now().UTC().Format(time.RFC3339),
	}, "", "  ")
	allSources := append([]Source{{Name: "VaultApp-Backup/manifest.json"}}, sources...)
	result := Result{}
	for _, source := range allSources {
		name := filepath.ToSlash(strings.TrimLeft(source.Name, "/\\"))
		entry, createErr := archive.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Now()})
		if createErr != nil {
			_ = archive.Close()
			return Result{}, normalizeLimit(createErr)
		}
		if source.Path == "" {
			if _, err = entry.Write(manifest); err != nil {
				_ = archive.Close()
				return Result{}, normalizeLimit(err)
			}
		} else {
			if limits.PerFileBytes > 0 {
				info, statErr := os.Stat(source.Path)
				if statErr != nil {
					_ = archive.Close()
					return Result{}, statErr
				}
				if info.Size() > limits.PerFileBytes {
					_ = archive.Close()
					return Result{}, fmt.Errorf("%s überschreitet das Backup-Limit pro Datei", filepath.Base(source.Path))
				}
			}
			file, openErr := os.Open(source.Path)
			if openErr != nil {
				_ = archive.Close()
				return Result{}, openErr
			}
			_, copyErr := io.Copy(entry, file)
			closeErr := file.Close()
			if copyErr != nil {
				_ = archive.Close()
				return Result{}, normalizeLimit(copyErr)
			}
			if closeErr != nil {
				_ = archive.Close()
				return Result{}, closeErr
			}
		}
		result.Files++
	}
	if err := archive.Close(); err != nil {
		return Result{}, normalizeLimit(err)
	}
	if err := temporary.Sync(); err != nil {
		return Result{}, err
	}
	if err := temporary.Close(); err != nil {
		return Result{}, err
	}
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return Result{}, err
	}
	completed = true
	result.Bytes = limited.written
	return result, nil
}

func normalizeLimit(err error) error {
	if errors.Is(err, ErrLimit) {
		return ErrLimit
	}
	return err
}
