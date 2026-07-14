package backup

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
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

type Manifest struct {
	Format    string `json:"format"`
	Version   int    `json:"version"`
	CreatedAt string `json:"createdAt"`
}

type Inspection struct {
	Manifest           Manifest `json:"manifest"`
	Files              int      `json:"files"`
	Bytes              int64    `json:"bytes"`
	IncludesThumbnails bool     `json:"includesThumbnails"`
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

// Extract validates a VaultApp backup and extracts only its known portable
// payload into destination. Callers should use a fresh staging directory.
func Extract(source, destination string, limits Limits) (Inspection, error) {
	archive, err := zip.OpenReader(source)
	if err != nil {
		return Inspection{}, fmt.Errorf("ZIP-Archiv öffnen: %w", err)
	}
	defer archive.Close()
	inspection := Inspection{}
	seen := make(map[string]bool)
	var manifestFile, databaseFile, configFile *zip.File
	for _, file := range archive.File {
		name := file.Name
		if strings.Contains(name, `\`) || strings.HasPrefix(name, "/") || path.Clean(name) != name || name == "." || strings.HasPrefix(name, "../") {
			return Inspection{}, fmt.Errorf("unsicherer Backup-Pfad: %q", name)
		}
		if seen[name] {
			return Inspection{}, fmt.Errorf("doppelter Backup-Eintrag: %s", name)
		}
		seen[name] = true
		if file.FileInfo().IsDir() {
			continue
		}
		if file.Mode()&os.ModeType != 0 {
			return Inspection{}, fmt.Errorf("unzulässiger Backup-Eintrag: %s", name)
		}
		if file.UncompressedSize64 > uint64(^uint64(0)>>1) {
			return Inspection{}, ErrLimit
		}
		size := int64(file.UncompressedSize64)
		if limits.PerFileBytes > 0 && size > limits.PerFileBytes {
			return Inspection{}, fmt.Errorf("%s überschreitet das Wiederherstellungslimit pro Datei", path.Base(name))
		}
		if size > 0 && inspection.Bytes > int64(^uint64(0)>>1)-size {
			return Inspection{}, ErrLimit
		}
		inspection.Bytes += size
		if limits.TotalBytes > 0 && inspection.Bytes > limits.TotalBytes {
			return Inspection{}, ErrLimit
		}
		switch {
		case name == "VaultApp-Backup/manifest.json":
			manifestFile = file
		case name == "VaultApp-Backup/data/vault.db":
			databaseFile = file
		case name == "VaultApp-Backup/data/config.json":
			configFile = file
		case strings.HasPrefix(name, "VaultApp-Backup/assets/thumbnails/"):
			inspection.IncludesThumbnails = true
		default:
			return Inspection{}, fmt.Errorf("unbekannter Backup-Eintrag: %s", name)
		}
		inspection.Files++
	}
	if manifestFile == nil || databaseFile == nil || configFile == nil {
		return Inspection{}, fmt.Errorf("Backup ist unvollständig: Manifest, Katalog oder Konfiguration fehlen")
	}
	if err := os.MkdirAll(destination, 0o700); err != nil {
		return Inspection{}, err
	}
	var actualBytes int64
	for _, file := range archive.File {
		if file.FileInfo().IsDir() {
			continue
		}
		relative := strings.TrimPrefix(file.Name, "VaultApp-Backup/")
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return Inspection{}, err
		}
		reader, err := file.Open()
		if err != nil {
			return Inspection{}, fmt.Errorf("%s lesen: %w", file.Name, err)
		}
		output, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			_ = reader.Close()
			return Inspection{}, err
		}
		maximum := int64(^uint64(0) >> 1)
		if limits.PerFileBytes > 0 && limits.PerFileBytes < maximum {
			maximum = limits.PerFileBytes
		}
		if limits.TotalBytes > 0 {
			remaining := limits.TotalBytes - actualBytes
			if remaining < 0 {
				remaining = 0
			}
			if remaining < maximum {
				maximum = remaining
			}
		}
		var payload io.Reader = reader
		if maximum < int64(^uint64(0)>>1) {
			payload = io.LimitReader(reader, maximum+1)
		}
		written, copyErr := io.Copy(output, payload)
		closeErr := output.Close()
		readerErr := reader.Close()
		if copyErr != nil {
			return Inspection{}, fmt.Errorf("%s prüfen: %w", file.Name, copyErr)
		}
		if written > maximum {
			return Inspection{}, ErrLimit
		}
		actualBytes += written
		if closeErr != nil {
			return Inspection{}, closeErr
		}
		if readerErr != nil {
			return Inspection{}, readerErr
		}
		if written != int64(file.UncompressedSize64) {
			return Inspection{}, fmt.Errorf("%s ist unvollständig", file.Name)
		}
	}
	manifestData, err := os.ReadFile(filepath.Join(destination, "manifest.json"))
	if err != nil {
		return Inspection{}, err
	}
	if err := json.Unmarshal(manifestData, &inspection.Manifest); err != nil {
		return Inspection{}, fmt.Errorf("ungültiges Backup-Manifest: %w", err)
	}
	if inspection.Manifest.Format != "VaultApp-Backup" || inspection.Manifest.Version != 1 {
		return Inspection{}, fmt.Errorf("nicht unterstütztes Backupformat %q, Version %d", inspection.Manifest.Format, inspection.Manifest.Version)
	}
	if _, err := time.Parse(time.RFC3339, inspection.Manifest.CreatedAt); err != nil {
		return Inspection{}, fmt.Errorf("ungültiger Erstellungszeitpunkt im Backup-Manifest")
	}
	return inspection, nil
}
