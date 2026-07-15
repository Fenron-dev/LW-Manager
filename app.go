package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"github.com/dennis/vaultapp/internal/ai"
	"github.com/dennis/vaultapp/internal/backup"
	appconfig "github.com/dennis/vaultapp/internal/config"
	"github.com/dennis/vaultapp/internal/database"
	"github.com/dennis/vaultapp/internal/scanner"
	"github.com/dennis/vaultapp/internal/storage"
	"github.com/dennis/vaultapp/internal/thumbnail"
	"github.com/dennis/vaultapp/internal/vault"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx        context.Context
	scanMu     sync.Mutex
	aiMu       sync.Mutex
	settingsMu sync.RWMutex
	root       string
	configPath string
	settings   appconfig.Settings
	catalog    *database.Catalog
	initErr    error
}

type AppInfo struct {
	Version    string `json:"version"`
	Platform   string `json:"platform"`
	VaultRoot  string `json:"vaultRoot"`
	Ready      bool   `json:"ready"`
	Message    string `json:"message"`
	FileCount  int64  `json:"fileCount"`
	DriveCount int64  `json:"driveCount"`
}

type ScanResult struct {
	Cancelled       bool            `json:"cancelled"`
	Drive           string          `json:"drive"`
	Files           int             `json:"files"`
	Bytes           int64           `json:"bytes"`
	Skipped         int             `json:"skipped"`
	Excluded        int             `json:"excluded"`
	Issues          []scanner.Issue `json:"issues"`
	IssuesTruncated bool            `json:"issuesTruncated"`
	LogPath         string          `json:"logPath"`
	Message         string          `json:"message"`
}

type ScanDiagnostic struct {
	StartedAt       time.Time       `json:"startedAt"`
	FinishedAt      time.Time       `json:"finishedAt"`
	DurationMS      int64           `json:"durationMs"`
	Drive           string          `json:"drive"`
	Files           int             `json:"files"`
	Bytes           int64           `json:"bytes"`
	Skipped         int             `json:"skipped"`
	Excluded        int             `json:"excluded"`
	Issues          []scanner.Issue `json:"issues"`
	IssuesTruncated bool            `json:"issuesTruncated"`
	Error           string          `json:"error,omitempty"`
	LogPath         string          `json:"logPath"`
}

type FileLocation struct {
	RelativePath    string `json:"relativePath"`
	FullPath        string `json:"fullPath"`
	Available       bool   `json:"available"`
	FolderAvailable bool   `json:"folderAvailable"`
	Status          string `json:"status"`
}

type ConnectedVolumes struct {
	Enabled bool             `json:"enabled"`
	Volumes []storage.Volume `json:"volumes"`
}

type BackupResult struct {
	Cancelled bool   `json:"cancelled"`
	Path      string `json:"path"`
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
	Message   string `json:"message"`
}

type BackupInspection struct {
	Cancelled          bool   `json:"cancelled"`
	Path               string `json:"path"`
	CreatedAt          string `json:"createdAt"`
	ArchiveFiles       int    `json:"archiveFiles"`
	ArchiveBytes       int64  `json:"archiveBytes"`
	CatalogFiles       int64  `json:"catalogFiles"`
	CatalogDrives      int64  `json:"catalogDrives"`
	IncludesThumbnails bool   `json:"includesThumbnails"`
	Message            string `json:"message"`
}

type ManagedBackup struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

type AIProviderStatus struct {
	Enabled          bool   `json:"enabled"`
	Provider         string `json:"provider"`
	Endpoint         string `json:"endpoint"`
	Model            string `json:"model"`
	VisionEnabled    bool   `json:"visionEnabled"`
	VisionModel      string `json:"visionModel"`
	CredentialStored bool   `json:"credentialStored"`
}

type RestoreResult struct {
	RollbackPath string `json:"rollbackPath"`
	Files        int64  `json:"files"`
	Drives       int64  `json:"drives"`
	Message      string `json:"message"`
}

type DuplicateResult struct {
	Groups  []database.DuplicateGroup `json:"groups"`
	Hashed  int                       `json:"hashed"`
	Skipped int                       `json:"skipped"`
	Limited int                       `json:"limited"`
	Bytes   int64                     `json:"bytes"`
}

type DuplicateSelection struct {
	Hash   string `json:"hash"`
	FileID int64  `json:"fileId"`
}

type QuarantineFailure struct {
	FileID  int64  `json:"fileId"`
	Message string `json:"message"`
}

type QuarantineResult struct {
	Moved    int                 `json:"moved"`
	Bytes    int64               `json:"bytes"`
	Failures []QuarantineFailure `json:"failures"`
}

type LibraryResult struct {
	Files      []database.FileResult `json:"files"`
	Extensions []string              `json:"extensions"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"pageSize"`
}

type ExportResult struct {
	Cancelled bool   `json:"cancelled"`
	Path      string `json:"path"`
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
}

func NewApp() *App { return &App{} }

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.root, a.initErr = vault.ResolveRoot()
	if a.initErr != nil {
		return
	}
	if a.initErr = vault.EnsureLayout(a.root); a.initErr != nil {
		return
	}
	a.configPath, a.initErr = vault.DataPath(a.root, "config.json")
	if a.initErr != nil {
		return
	}
	a.settings, a.initErr = appconfig.Load(a.configPath)
	if a.initErr != nil {
		return
	}
	dbPath, err := vault.DataPath(a.root, "vault.db")
	if err != nil {
		a.initErr = err
		return
	}
	a.catalog, a.initErr = database.Open(dbPath)
}

func (a *App) Shutdown(context.Context) {
	if a.catalog != nil {
		_ = a.catalog.Close()
	}
}

func (a *App) GetAppInfo() AppInfo {
	info := AppInfo{Version: "0.46.0-dev", Platform: goruntime.GOOS, VaultRoot: a.root}
	if a.initErr != nil {
		info.Message = fmt.Sprintf("Vault kann nicht vorbereitet werden: %v", a.initErr)
		return info
	}
	files, drives, err := a.catalog.Stats()
	if err != nil {
		info.Message = fmt.Sprintf("Katalog kann nicht gelesen werden: %v", err)
		return info
	}
	info.Ready, info.Message = true, "Vault und Katalog sind bereit"
	info.FileCount, info.DriveCount = files, drives
	return info
}

func (a *App) SearchFiles(query, extension, tag string, driveID int64, includeContent bool, page int) (LibraryResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return LibraryResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	if page < 1 {
		page = 1
	}
	const pageSize = 50
	result, err := a.catalog.Search(query, extension, tag, driveID, includeContent, pageSize, (page-1)*pageSize)
	if err != nil {
		return LibraryResult{}, err
	}
	return LibraryResult{Files: result.Files, Extensions: result.Extensions, Total: result.Total, Page: page, PageSize: pageSize}, nil
}

func (a *App) ExportLibraryCSV(query, extension, tag string, driveID int64, includeContent bool) (ExportResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return ExportResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	settings := a.currentSettings()
	if !settings.CatalogExportEnabled {
		return ExportResult{}, fmt.Errorf("Katalogexport ist in den Einstellungen deaktiviert")
	}
	destination, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Gefilterten VaultApp-Katalog exportieren",
		DefaultFilename: "VaultApp-Katalog-" + time.Now().Format("2006-01-02") + ".csv",
		Filters:         []wailsruntime.FileFilter{{DisplayName: "CSV-Tabelle", Pattern: "*.csv"}},
	})
	if err != nil {
		return ExportResult{}, err
	}
	if destination == "" {
		return ExportResult{Cancelled: true}, nil
	}
	if !strings.EqualFold(filepath.Ext(destination), ".csv") {
		destination += ".csv"
	}
	directory := filepath.Dir(destination)
	temporary, err := os.CreateTemp(directory, ".vaultapp-export-*.tmp")
	if err != nil {
		return ExportResult{}, fmt.Errorf("Exportdatei vorbereiten: %w", err)
	}
	temporaryPath := temporary.Name()
	cleanup := func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}
	maximum := int64(-1)
	if !settings.CatalogExportUnlimited {
		maximum = int64(settings.CatalogExportMaxMB) << 20
	}
	limited := &exportLimitWriter{writer: temporary, maximum: maximum}
	if _, err := limited.Write([]byte{0xef, 0xbb, 0xbf}); err != nil {
		cleanup()
		return ExportResult{}, err
	}
	writer := csv.NewWriter(limited)
	writer.Comma = ';'
	header := []string{"Dateiname", "Datenträger", "Relativer Pfad", "Dateiendung", "MIME-Typ", "Größe (Bytes)", "Geändert", "Manuelle Tags", "KI-Tags", "KI-Zusammenfassung"}
	if err := writer.Write(header); err != nil {
		cleanup()
		return ExportResult{}, err
	}
	result := ExportResult{Path: destination}
	err = a.catalog.ExportFiles(query, extension, tag, driveID, includeContent, func(file database.ExportFile) error {
		if err := writer.Write([]string{
			csvSafe(file.Filename), csvSafe(file.Drive), csvSafe(file.Path), csvSafe(file.Extension), csvSafe(file.MIMEType), strconv.FormatInt(file.Size, 10),
			csvSafe(file.Modified), csvSafe(strings.Join(file.Tags, ", ")), csvSafe(strings.Join(file.AITags, ", ")), csvSafe(file.AISummary),
		}); err != nil {
			return err
		}
		result.Files++
		return nil
	})
	writer.Flush()
	if err == nil {
		err = writer.Error()
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporaryPath)
		return ExportResult{}, fmt.Errorf("Katalog exportieren: %w", err)
	}
	if err := os.Chmod(temporaryPath, 0o644); err != nil {
		_ = os.Remove(temporaryPath)
		return ExportResult{}, err
	}
	if err := replaceExportFile(temporaryPath, destination); err != nil {
		_ = os.Remove(temporaryPath)
		return ExportResult{}, fmt.Errorf("Exportdatei ablegen: %w", err)
	}
	result.Bytes = limited.written
	return result, nil
}

type exportLimitWriter struct {
	writer           io.Writer
	written, maximum int64
}

func (w *exportLimitWriter) Write(data []byte) (int, error) {
	if w.maximum >= 0 && w.written+int64(len(data)) > w.maximum {
		return 0, fmt.Errorf("CSV-Export überschreitet das eingestellte Gesamtlimit von %d MB", w.maximum>>20)
	}
	n, err := w.writer.Write(data)
	w.written += int64(n)
	return n, err
}

func replaceExportFile(source, destination string) error {
	if _, err := os.Stat(destination); os.IsNotExist(err) {
		return os.Rename(source, destination)
	} else if err != nil {
		return err
	}
	backupFile, err := os.CreateTemp(filepath.Dir(destination), ".vaultapp-export-old-*.tmp")
	if err != nil {
		return err
	}
	backup := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		_ = os.Remove(backup)
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(destination, backup); err != nil {
		return err
	}
	if err := os.Rename(source, destination); err != nil {
		_ = os.Rename(backup, destination)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func csvSafe(value string) string {
	trimmed := strings.TrimLeftFunc(value, unicode.IsSpace)
	if trimmed != "" && strings.ContainsRune("=+-@", []rune(trimmed)[0]) {
		return "'" + value
	}
	return value
}

func (a *App) GetTags() ([]database.TagSummary, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.Tags()
}

func (a *App) GetScanDiagnostics() ([]ScanDiagnostic, error) {
	logs, err := vault.DataPath(a.root, "logs")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(logs)
	if err != nil {
		return nil, err
	}
	result := make([]ScanDiagnostic, 0, 20)
	for index := len(entries) - 1; index >= 0 && len(result) < 20; index-- {
		entry := entries[index]
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "scan-") || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(logs, entry.Name()))
		if readErr != nil {
			continue
		}
		var diagnostic ScanDiagnostic
		if json.Unmarshal(data, &diagnostic) == nil {
			result = append(result, diagnostic)
		}
	}
	return result, nil
}

func (a *App) writeScanDiagnostic(diagnostic ScanDiagnostic) string {
	settings := a.currentSettings()
	if !settings.ScanDiagnosticsEnabled {
		return ""
	}
	finished := time.Now()
	diagnostic.FinishedAt = finished
	diagnostic.DurationMS = finished.Sub(diagnostic.StartedAt).Milliseconds()
	filename := "scan-" + finished.UTC().Format("20060102T150405.000000000Z") + ".json"
	path, err := vault.DataPath(a.root, filepath.Join("logs", filename))
	if err != nil {
		return ""
	}
	diagnostic.LogPath = filepath.ToSlash(filepath.Join("data", "logs", filename))
	data, err := json.MarshalIndent(diagnostic, "", "  ")
	if !settings.ScanDiagnosticUnlimited {
		limit := int64(settings.ScanDiagnosticFileMB) << 20
		for err == nil && int64(len(data)) > limit && len(diagnostic.Issues) > 0 {
			diagnostic.IssuesTruncated = true
			diagnostic.Issues = diagnostic.Issues[:len(diagnostic.Issues)-1]
			data, err = json.MarshalIndent(diagnostic, "", "  ")
		}
	}
	if err != nil || os.WriteFile(path, data, 0o644) != nil {
		return ""
	}
	if !settings.ScanDiagnosticsUnlimited {
		a.trimScanDiagnostics(int64(settings.ScanDiagnosticsTotalMB) << 20)
		if _, err := os.Stat(path); err != nil {
			return ""
		}
	}
	return diagnostic.LogPath
}

func (a *App) trimScanDiagnostics(limit int64) {
	logs, err := vault.DataPath(a.root, "logs")
	if err != nil {
		return
	}
	entries, err := os.ReadDir(logs)
	if err != nil {
		return
	}
	total := int64(0)
	type logFile struct {
		path string
		size int64
	}
	files := make([]logFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "scan-") || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		files = append(files, logFile{path: filepath.Join(logs, entry.Name()), size: info.Size()})
		total += info.Size()
	}
	for _, file := range files {
		if total <= limit {
			break
		}
		if os.Remove(file.path) == nil {
			total -= file.size
		}
	}
}

func (a *App) RenameTag(currentName, newName string) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.RenameTag(currentName, newName)
}

func (a *App) DeleteTag(name string) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.DeleteTag(name)
}

func (a *App) UpdateFileTags(id int64, tags []string) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.UpdateFileTags(id, tags)
}

func (a *App) GetDrives() ([]database.Drive, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	drives, err := a.catalog.Drives()
	if err != nil {
		return nil, err
	}
	for index := range drives {
		info, statErr := os.Stat(drives[index].Path)
		drives[index].Online = statErr == nil && info.IsDir()
	}
	return drives, nil
}

func (a *App) GetConnectedVolumes() (ConnectedVolumes, error) {
	settings := a.currentSettings()
	result := ConnectedVolumes{Enabled: settings.VolumeDetectionEnabled, Volumes: []storage.Volume{}}
	if !result.Enabled {
		return result, nil
	}
	volumes, err := storage.ListVolumes()
	if err != nil {
		return result, fmt.Errorf("angeschlossene Datenträger erkennen: %w", err)
	}
	result.Volumes = volumes
	return result, nil
}

func (a *App) FindDuplicates() (DuplicateResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return DuplicateResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	settings := a.currentSettings()
	if !settings.DuplicateCheckEnabled {
		return DuplicateResult{}, fmt.Errorf("Duplikatprüfung ist in den Einstellungen deaktiviert")
	}
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	candidates, err := a.catalog.HashCandidates()
	if err != nil {
		return DuplicateResult{}, err
	}
	result := DuplicateResult{}
	for index, candidate := range candidates {
		if candidate.Hash == "" {
			if (!settings.DuplicateFileUnlimited && candidate.Size > int64(settings.DuplicateFileMB)<<20) ||
				(!settings.DuplicateTotalUnlimited && result.Bytes+candidate.Size > int64(settings.DuplicateTotalMB)<<20) {
				result.Limited++
				continue
			}
			hash, bytesRead, hashErr := hashFile(candidate.SourcePath)
			result.Bytes += bytesRead
			if hashErr != nil {
				result.Skipped++
				continue
			}
			if err := a.catalog.SaveFileHash(candidate.ID, hash); err != nil {
				return result, err
			}
			result.Hashed++
		}
		if index%25 == 0 {
			wailsruntime.EventsEmit(a.ctx, "duplicates:progress", map[string]any{"done": index + 1, "total": len(candidates)})
		}
	}
	result.Groups, err = a.catalog.DuplicateGroups()
	return result, err
}

func (a *App) SetDuplicatePreference(hash string, fileID int64) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.SetDuplicatePreference(hash, fileID)
}

func (a *App) QuarantineDuplicates(selections []DuplicateSelection) (QuarantineResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return QuarantineResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	if len(selections) == 0 {
		return QuarantineResult{}, fmt.Errorf("keine Duplikate ausgewählt")
	}
	settings := a.currentSettings()
	if !settings.DuplicateQuarantineEnabled {
		return QuarantineResult{}, fmt.Errorf("Duplikat-Quarantäne ist in den Einstellungen deaktiviert")
	}
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	candidates := make([]database.QuarantineCandidate, 0, len(selections))
	usage, err := a.catalog.QuarantineUsage()
	if err != nil {
		return QuarantineResult{}, err
	}
	pendingBytes := int64(0)
	selectedByHash := make(map[string]int64)
	seen := make(map[int64]bool)
	for _, selection := range selections {
		if seen[selection.FileID] {
			return QuarantineResult{}, fmt.Errorf("Datei %d wurde mehrfach ausgewählt", selection.FileID)
		}
		seen[selection.FileID] = true
		candidate, err := a.catalog.QuarantineCandidate(selection.FileID, selection.Hash)
		if err != nil {
			return QuarantineResult{}, err
		}
		if candidate.Preferred {
			return QuarantineResult{}, fmt.Errorf("das bevorzugte Original %q darf nicht in Quarantäne verschoben werden", candidate.Path)
		}
		if !settings.DuplicateQuarantineFileUnlimited && candidate.Size > int64(settings.DuplicateQuarantineFileMB)<<20 {
			return QuarantineResult{}, fmt.Errorf("%q überschreitet das Quarantäne-Dateilimit von %d MB", candidate.Path, settings.DuplicateQuarantineFileMB)
		}
		pendingBytes += candidate.Size
		if !settings.DuplicateQuarantineUnlimited && usage+pendingBytes > int64(settings.DuplicateQuarantineTotalMB)<<20 {
			return QuarantineResult{}, fmt.Errorf("Quarantäne würde das Gesamtlimit von %d MB überschreiten", settings.DuplicateQuarantineTotalMB)
		}
		selectedByHash[candidate.Hash]++
		if selectedByHash[candidate.Hash] >= candidate.GroupCount {
			return QuarantineResult{}, fmt.Errorf("mindestens ein Fundort jeder Duplikatgruppe muss erhalten bleiben")
		}
		candidates = append(candidates, candidate)
	}
	// Every candidate is fully verified before the first source file is changed.
	for _, candidate := range candidates {
		if err := verifyQuarantineCandidate(candidate); err != nil {
			return QuarantineResult{}, fmt.Errorf("%s: %w", candidate.Path, err)
		}
	}
	result := QuarantineResult{Failures: make([]QuarantineFailure, 0)}
	for _, candidate := range candidates {
		if err := a.quarantineCandidate(candidate); err != nil {
			result.Failures = append(result.Failures, QuarantineFailure{FileID: candidate.ID, Message: err.Error()})
			continue
		}
		result.Moved++
		result.Bytes += candidate.Size
	}
	return result, nil
}

func verifyQuarantineCandidate(candidate database.QuarantineCandidate) error {
	source := database.SourceFile{Root: candidate.Root, Relative: candidate.Path, Path: filepath.Join(candidate.Root, filepath.FromSlash(candidate.Path)), Size: candidate.Size, Modified: candidate.Modified}
	if err := verifyCatalogSource(source); err != nil {
		return err
	}
	hash, _, err := hashFile(source.Path)
	if err != nil {
		return fmt.Errorf("SHA-256 konnte nicht erneut berechnet werden: %w", err)
	}
	if !strings.EqualFold(hash, candidate.Hash) {
		return fmt.Errorf("Inhalt stimmt nicht mehr mit der Duplikatprüfung überein")
	}
	return verifyCatalogSource(source)
}

func (a *App) quarantineCandidate(candidate database.QuarantineCandidate) error {
	if err := verifyQuarantineCandidate(candidate); err != nil {
		return err
	}
	quarantineRoot, err := vault.DataPath(a.root, "quarantine")
	if err != nil {
		return err
	}
	directory, err := os.MkdirTemp(quarantineRoot, fmt.Sprintf("%d-", candidate.ID))
	if err != nil {
		return fmt.Errorf("Quarantäne vorbereiten: %w", err)
	}
	_ = os.Chmod(directory, 0o700)
	destination := filepath.Join(directory, filepath.Base(candidate.Filename))
	source := filepath.Join(candidate.Root, filepath.FromSlash(candidate.Path))
	if err := moveVerifiedFile(source, destination, candidate.Hash); err != nil {
		_ = os.RemoveAll(directory)
		return fmt.Errorf("in Quarantäne verschieben: %w", err)
	}
	dataRoot, _ := vault.DataPath(a.root, "")
	relative, err := filepath.Rel(dataRoot, destination)
	if err != nil {
		_ = moveVerifiedFile(destination, source, candidate.Hash)
		return err
	}
	if err := a.catalog.RecordQuarantine(candidate, filepath.ToSlash(relative)); err != nil {
		if restoreErr := moveVerifiedFile(destination, source, candidate.Hash); restoreErr != nil {
			return fmt.Errorf("Katalog konnte nicht aktualisiert werden (%v); Rückverschiebung fehlgeschlagen: %w", err, restoreErr)
		}
		_ = os.RemoveAll(directory)
		return fmt.Errorf("Katalog konnte nicht aktualisiert werden: %w", err)
	}
	return nil
}

func (a *App) GetQuarantineItems() ([]database.QuarantineItem, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.QuarantineItems()
}

func (a *App) RestoreQuarantineItem(id int64) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	item, root, err := a.catalog.QuarantineItem(id)
	if err != nil {
		return err
	}
	quarantined, err := a.verifiedQuarantineFile(item)
	if err != nil {
		return err
	}
	destination, err := verifyRestoreDestination(root, item.OriginalPath)
	if err != nil {
		return err
	}
	if err := moveVerifiedFile(quarantined, destination, item.Hash); err != nil {
		return fmt.Errorf("Datei wiederherstellen: %w", err)
	}
	if err := a.catalog.DeleteQuarantineRecord(id); err != nil {
		if rollbackErr := moveVerifiedFile(destination, quarantined, item.Hash); rollbackErr != nil {
			return fmt.Errorf("Quarantäne-Katalog konnte nicht aktualisiert werden (%v); Rückverschiebung fehlgeschlagen: %w", err, rollbackErr)
		}
		return err
	}
	_ = os.Remove(filepath.Dir(quarantined))
	return nil
}

func (a *App) DeleteQuarantineItem(id int64, confirmation string) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	if !a.currentSettings().DuplicatePermanentDeleteEnabled {
		return fmt.Errorf("dauerhaftes Löschen ist in den Einstellungen deaktiviert")
	}
	if confirmation != "DAUERHAFT LÖSCHEN" {
		return fmt.Errorf("Bestätigungstext stimmt nicht überein")
	}
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	item, _, err := a.catalog.QuarantineItem(id)
	if err != nil {
		return err
	}
	quarantined, err := a.verifiedQuarantineFile(item)
	if err != nil {
		return err
	}
	if err := os.Remove(quarantined); err != nil {
		return fmt.Errorf("Quarantänedatei dauerhaft löschen: %w", err)
	}
	if err := a.catalog.DeleteQuarantineRecord(id); err != nil {
		return fmt.Errorf("Datei wurde gelöscht, aber der Quarantäne-Katalog konnte nicht bereinigt werden: %w", err)
	}
	_ = os.Remove(filepath.Dir(quarantined))
	return nil
}

func (a *App) verifiedQuarantineFile(item database.QuarantineItem) (string, error) {
	quarantineRoot, err := vault.DataPath(a.root, "quarantine")
	if err != nil {
		return "", err
	}
	quarantined, err := vault.DataPath(a.root, filepath.FromSlash(item.QuarantinePath))
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(quarantineRoot, quarantined)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("Quarantänepfad verlässt das Quarantäneverzeichnis")
	}
	info, err := os.Lstat(quarantined)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() != item.Size {
		return "", fmt.Errorf("Quarantänedatei fehlt oder wurde verändert")
	}
	if err := verifyResolvedInside(quarantineRoot, quarantined); err != nil {
		return "", fmt.Errorf("Quarantänepfad ist unsicher: %w", err)
	}
	hash, _, err := hashFile(quarantined)
	if err != nil || !strings.EqualFold(hash, item.Hash) {
		return "", fmt.Errorf("Prüfsumme der Quarantänedatei stimmt nicht mehr")
	}
	return quarantined, nil
}

func verifyRestoreDestination(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("ungültiger absoluter Wiederherstellungspfad")
	}
	root = filepath.Clean(root)
	destination := filepath.Join(root, filepath.FromSlash(relative))
	inside, err := filepath.Rel(root, destination)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("Wiederherstellungspfad verlässt den Datenträger")
	}
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return "", fmt.Errorf("Datenträger ist nicht angeschlossen")
	}
	if info, err := os.Stat(filepath.Dir(destination)); err != nil || !info.IsDir() {
		return "", fmt.Errorf("ursprünglicher Ordner ist nicht mehr verfügbar")
	}
	if err := verifyResolvedInside(root, filepath.Dir(destination)); err != nil {
		return "", err
	}
	if _, err := os.Lstat(destination); err == nil {
		return "", fmt.Errorf("am ursprünglichen Pfad existiert bereits eine Datei")
	} else if !os.IsNotExist(err) {
		return "", err
	}
	return destination, nil
}

func moveVerifiedFile(source, destination, expectedHash string) error {
	info, err := os.Lstat(source)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("Quelle ist keine reguläre Datei")
	}
	if err := os.Link(source, destination); err == nil {
		hash, _, hashErr := hashFile(destination)
		if hashErr != nil || !strings.EqualFold(hash, expectedHash) {
			_ = os.Remove(destination)
			return fmt.Errorf("Quelle wurde während der Verschiebung verändert")
		}
		if err := os.Remove(source); err != nil {
			_ = os.Remove(destination)
			return err
		}
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		if _, destinationErr := os.Lstat(destination); destinationErr == nil {
			return fmt.Errorf("Zieldatei existiert bereits")
		}
		// Some removable filesystems do not support hard links. The exclusive
		// destination creation below still guarantees that nothing is replaced.
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".vaultapp-move-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	cleanup := func() { _ = temporary.Close(); _ = os.Remove(temporaryPath) }
	input, err := os.Open(source)
	if err != nil {
		cleanup()
		return err
	}
	_, copyErr := io.Copy(temporary, input)
	closeInputErr := input.Close()
	if copyErr == nil {
		copyErr = closeInputErr
	}
	if copyErr == nil {
		copyErr = temporary.Sync()
	}
	if closeErr := temporary.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		cleanup()
		return copyErr
	}
	if err := os.Chmod(temporaryPath, info.Mode().Perm()); err != nil {
		cleanup()
		return err
	}
	_ = os.Chtimes(temporaryPath, info.ModTime(), info.ModTime())
	hash, _, err := hashFile(temporaryPath)
	if err != nil || !strings.EqualFold(hash, expectedHash) {
		cleanup()
		return fmt.Errorf("kopierte Datei hat eine abweichende Prüfsumme")
	}
	verified, err := os.Open(temporaryPath)
	if err != nil {
		cleanup()
		return err
	}
	output, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		_ = verified.Close()
		cleanup()
		return err
	}
	_, installErr := io.Copy(output, verified)
	if installErr == nil {
		installErr = output.Sync()
	}
	if closeErr := output.Close(); installErr == nil {
		installErr = closeErr
	}
	if closeErr := verified.Close(); installErr == nil {
		installErr = closeErr
	}
	if installErr != nil {
		_ = os.Remove(destination)
		cleanup()
		return installErr
	}
	_ = os.Chtimes(destination, info.ModTime(), info.ModTime())
	installedHash, _, err := hashFile(destination)
	if err != nil || !strings.EqualFold(installedHash, expectedHash) {
		_ = os.Remove(destination)
		cleanup()
		return fmt.Errorf("installierte Datei hat eine abweichende Prüfsumme")
	}
	cleanup()
	if err := os.Remove(source); err != nil {
		_ = os.Remove(destination)
		return err
	}
	return nil
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	bytesRead, err := io.Copy(hash, file)
	if err != nil {
		return "", bytesRead, err
	}
	return hex.EncodeToString(hash.Sum(nil)), bytesRead, nil
}

func (a *App) UpdateDrive(id int64, displayName, inventoryNumber, manufacturer, deviceType, storageLocation, note string, tags []string) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.UpdateDrive(id, displayName, inventoryNumber, manufacturer, deviceType, storageLocation, note, tags)
}

func (a *App) GetStorageLocations() ([]string, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.StorageLocations()
}
func (a *App) AddStorageLocation(name string) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.AddStorageLocation(name)
}

func (a *App) BrowseDrive(id int64, directory string) ([]database.DirectoryEntry, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 12*time.Second)
	defer cancel()
	return a.catalog.Directory(ctx, id, directory)
}

func (a *App) GetImagePreview(id int64) (string, error) {
	if a.initErr != nil || a.catalog == nil {
		return "", fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	source, err := a.catalog.SourceFile(id)
	if err != nil {
		return "", err
	}
	if err := verifyCatalogSource(source); err != nil {
		return "", err
	}
	cache, err := vault.AssetPath(a.root, "thumbnails")
	if err != nil {
		return "", err
	}
	settings := a.currentSettings()
	return thumbnail.DataURLWithLimits(source.Path, cache, fmt.Sprintf("%s:%d", source.Modified, source.Size), thumbnail.Limits{
		ImageEnabled: settings.ImagePreviewEnabled, HEICEnabled: settings.HEICPreviewEnabled, ImageMB: settings.ImagePreviewMB,
		ImageUnlimited: settings.ImagePreviewUnlimited, CacheMB: settings.ThumbnailCacheMB,
		CacheUnlimited: settings.ThumbnailCacheUnlimited,
		PDFEnabled:     settings.PDFPreviewEnabled, PDFMB: settings.PDFPreviewMB, PDFUnlimited: settings.PDFPreviewUnlimited,
		VideoEnabled: settings.VideoPreviewEnabled, VideoMB: settings.VideoPreviewMB, VideoUnlimited: settings.VideoPreviewUnlimited,
	})
}

func (a *App) GetFileLocation(id int64) (FileLocation, error) {
	if a.initErr != nil || a.catalog == nil {
		return FileLocation{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	source, err := a.catalog.SourceFile(id)
	if err != nil {
		return FileLocation{}, err
	}
	result := FileLocation{RelativePath: source.Relative, FullPath: source.Path, Available: true, Status: "Datei ist verfügbar"}
	result.FolderAvailable = verifyContainingFolder(source) == nil
	if err := verifyCatalogSource(source); err != nil {
		result.Available = false
		result.Status = err.Error()
	}
	return result, nil
}

func (a *App) CopyFilePath(id int64, fullPath bool) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	source, err := a.catalog.SourceFile(id)
	if err != nil {
		return err
	}
	value := source.Relative
	if fullPath {
		value = source.Path
	}
	return wailsruntime.ClipboardSetText(a.ctx, value)
}

func (a *App) RevealFile(id int64) error {
	source, err := a.availableSource(id)
	if err != nil {
		return err
	}
	return openFileManager(source.Path, true)
}

func (a *App) OpenContainingFolder(id int64) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	source, err := a.catalog.SourceFile(id)
	if err != nil {
		return err
	}
	if err := verifyContainingFolder(source); err != nil {
		return err
	}
	return openFileManager(filepath.Dir(source.Path), false)
}

func (a *App) availableSource(id int64) (database.SourceFile, error) {
	if a.initErr != nil || a.catalog == nil {
		return database.SourceFile{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	source, err := a.catalog.SourceFile(id)
	if err != nil {
		return database.SourceFile{}, err
	}
	if err := verifyCatalogSource(source); err != nil {
		return database.SourceFile{}, err
	}
	return source, nil
}

func verifyCatalogSource(source database.SourceFile) error {
	rootInfo, err := os.Stat(source.Root)
	if err != nil || !rootInfo.IsDir() {
		return fmt.Errorf("Datenträger ist nicht angeschlossen")
	}
	info, err := os.Lstat(source.Path)
	if os.IsNotExist(err) {
		return fmt.Errorf("Datei fehlt am katalogisierten Pfad")
	}
	if err != nil {
		return fmt.Errorf("Datei kann nicht geprüft werden: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("Datei ist keine reguläre Datei oder eine Verknüpfung")
	}
	if err := verifyResolvedInside(source.Root, source.Path); err != nil {
		return err
	}
	if info.Size() != source.Size || info.ModTime().UTC().Format(time.RFC3339Nano) != source.Modified {
		return fmt.Errorf("Datei wurde seit dem letzten Scan geändert; bitte Datenträger erneut scannen")
	}
	return nil
}

func verifyContainingFolder(source database.SourceFile) error {
	rootInfo, err := os.Stat(source.Root)
	if err != nil || !rootInfo.IsDir() {
		return fmt.Errorf("Datenträger ist nicht angeschlossen")
	}
	folder := filepath.Dir(source.Path)
	info, err := os.Stat(folder)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("übergeordneter Ordner ist nicht verfügbar")
	}
	return verifyResolvedInside(source.Root, folder)
}

func verifyResolvedInside(root, candidate string) error {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("Datenträgerpfad kann nicht geprüft werden: %w", err)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return fmt.Errorf("Dateipfad kann nicht geprüft werden: %w", err)
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedCandidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("Dateipfad führt über eine Verknüpfung aus dem Datenträger heraus")
	}
	return nil
}

func openFileManager(path string, selectFile bool) error {
	var command *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		if selectFile {
			command = exec.Command("/usr/bin/open", "-R", path)
		} else {
			command = exec.Command("/usr/bin/open", path)
		}
	case "windows":
		if selectFile {
			command = exec.Command("explorer.exe", "/select,", path)
		} else {
			command = exec.Command("explorer.exe", path)
		}
	default:
		target := path
		if selectFile {
			target = filepath.Dir(path)
		}
		command = exec.Command("xdg-open", target)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("Dateimanager konnte nicht geöffnet werden: %w", err)
	}
	return command.Process.Release()
}

func (a *App) GetFileDetails(id int64) (database.FileResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return database.FileResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.FileDetails(id)
}

func (a *App) GetSettings() appconfig.Settings {
	return a.currentSettings()
}

func (a *App) SaveSettings(settings appconfig.Settings) error {
	if a.initErr != nil || a.configPath == "" {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	if !settings.ThumbnailCacheUnlimited {
		cache, err := vault.AssetPath(a.root, "thumbnails")
		if err != nil {
			return err
		}
		if err := thumbnail.TrimCache(cache, int64(settings.ThumbnailCacheMB)<<20); err != nil {
			return fmt.Errorf("Vorschau-Cache begrenzen: %w", err)
		}
	}
	if err := appconfig.Save(a.configPath, settings); err != nil {
		return err
	}
	a.settingsMu.Lock()
	a.settings = settings
	a.settingsMu.Unlock()
	if settings.ScanDiagnosticsEnabled && !settings.ScanDiagnosticsUnlimited {
		a.trimScanDiagnostics(int64(settings.ScanDiagnosticsTotalMB) << 20)
	}
	return nil
}

func (a *App) GetAIProviderStatus() (AIProviderStatus, error) {
	settings := a.currentSettings()
	status := AIProviderStatus{Enabled: settings.AIEnabled, Provider: settings.AIProvider, Endpoint: settings.AIEndpoint, Model: settings.AIModel, VisionEnabled: settings.AIVisionEnabled, VisionModel: settings.AIVisionModel}
	path, err := a.aiCredentialPath()
	if err != nil {
		return status, err
	}
	data, err := os.ReadFile(path)
	if err == nil {
		status.CredentialStored = strings.TrimSpace(string(data)) != ""
	} else if !os.IsNotExist(err) {
		return status, err
	}
	return status, nil
}

func (a *App) SaveAICredential(credential string) error {
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return fmt.Errorf("API-Schlüssel darf nicht leer sein")
	}
	path, err := a.aiCredentialPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, []byte(credential+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

func (a *App) ClearAICredential() error {
	path, err := a.aiCredentialPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (a *App) TestAIProvider() (ai.ConnectionResult, error) {
	settings := a.currentSettings()
	if !settings.AIEnabled {
		return ai.ConnectionResult{}, fmt.Errorf("KI-Funktionen sind in den Einstellungen deaktiviert")
	}
	credential, err := a.aiCredential(settings.AIProvider)
	if err != nil {
		return ai.ConnectionResult{}, err
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return ai.TestConnection(ctx, ai.Config{Provider: settings.AIProvider, Endpoint: settings.AIEndpoint, Model: settings.AIModel, TimeoutSeconds: settings.AITimeoutSeconds}, credential)
}

func (a *App) AnalyzeFile(id int64) (ai.AnalysisResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return ai.AnalysisResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	settings := a.currentSettings()
	if !settings.AIEnabled {
		return ai.AnalysisResult{}, fmt.Errorf("KI-Funktionen sind in den Einstellungen deaktiviert")
	}
	if !a.aiMu.TryLock() {
		return ai.AnalysisResult{}, fmt.Errorf("es läuft bereits eine KI-Analyse")
	}
	defer a.aiMu.Unlock()
	input, err := a.catalog.AIFileInput(id)
	if err != nil {
		return ai.AnalysisResult{}, fmt.Errorf("Datei für KI-Analyse laden: %w", err)
	}
	content := input.TextContent
	limit := int64(-1)
	if !settings.AIFileUnlimited {
		limit = int64(settings.AIFileMB) << 20
	}
	if !settings.AITotalUnlimited {
		total := int64(settings.AITotalMB) << 20
		if limit < 0 || total < limit {
			limit = total
		}
	}
	truncated := false
	if limit >= 0 && int64(len(content)) > limit {
		content = truncateUTF8(content, limit)
		truncated = true
	}
	credential, err := a.aiCredential(settings.AIProvider)
	if err != nil {
		return ai.AnalysisResult{}, err
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := ai.Analyze(ctx, ai.Config{Provider: settings.AIProvider, Endpoint: settings.AIEndpoint, Model: settings.AIModel, TimeoutSeconds: settings.AITimeoutSeconds}, credential, ai.AnalysisRequest{
		Filename: input.Filename, Path: input.Path, MIMEType: input.MIMEType, Size: input.Size, Width: input.Width, Height: input.Height, Metadata: input.Metadata, Content: content,
	})
	if err != nil {
		return ai.AnalysisResult{}, err
	}
	result.InputBytes = int64(len(content))
	result.InputTruncated = truncated
	result.AnalyzedAt = time.Now().UTC().Format(time.RFC3339)
	if err := a.catalog.SaveAIAnalysis(input, database.AIAnalysis{Summary: result.Summary, Tags: result.Tags, Provider: result.Provider, Model: result.Model, InputBytes: result.InputBytes, InputTruncated: result.InputTruncated}); err != nil {
		return ai.AnalysisResult{}, fmt.Errorf("KI-Analyse speichern: %w", err)
	}
	return result, nil
}

func (a *App) AnalyzeImage(id int64) (ai.AnalysisResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return ai.AnalysisResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	settings := a.currentSettings()
	if !settings.AIEnabled || !settings.AIVisionEnabled {
		return ai.AnalysisResult{}, fmt.Errorf("Vision-Analyse ist in den Einstellungen deaktiviert")
	}
	if !a.aiMu.TryLock() {
		return ai.AnalysisResult{}, fmt.Errorf("es läuft bereits eine KI-Analyse")
	}
	defer a.aiMu.Unlock()
	input, err := a.catalog.AIFileInput(id)
	if err != nil {
		return ai.AnalysisResult{}, fmt.Errorf("Bild für Vision-Analyse laden: %w", err)
	}
	source, err := a.catalog.SourceFile(id)
	if err != nil {
		return ai.AnalysisResult{}, err
	}
	info, err := os.Lstat(source.Path)
	if err != nil {
		return ai.AnalysisResult{}, fmt.Errorf("Bilddatei prüfen: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return ai.AnalysisResult{}, fmt.Errorf("Vision-Analyse verarbeitet nur reguläre Bilddateien, keine Verknüpfungen")
	}
	if info.Size() != source.Size || info.ModTime().UTC().Format(time.RFC3339Nano) != source.Modified {
		return ai.AnalysisResult{}, fmt.Errorf("Bild wurde seit dem letzten Scan geändert; bitte Datenträger erneut scannen")
	}
	extension := strings.ToLower(filepath.Ext(source.Path))
	if !strings.HasPrefix(input.MIMEType, "image/") && extension != ".jpg" && extension != ".jpeg" && extension != ".png" && extension != ".gif" && extension != ".webp" && extension != ".heic" && extension != ".heif" {
		return ai.AnalysisResult{}, fmt.Errorf("Vision-Analyse unterstützt JPEG, PNG, GIF, WebP und HEIC/HEIF")
	}
	if !settings.AIVisionFileUnlimited && source.Size > int64(settings.AIVisionFileMB)<<20 {
		return ai.AnalysisResult{}, fmt.Errorf("Bild ist größer als das Vision-Dateilimit von %d MB", settings.AIVisionFileMB)
	}
	if !settings.AIVisionTotalUnlimited && source.Size > int64(settings.AIVisionTotalMB)<<20 {
		return ai.AnalysisResult{}, fmt.Errorf("Bild überschreitet das Vision-Gesamtlimit von %d MB", settings.AIVisionTotalMB)
	}
	cache, err := vault.AssetPath(a.root, "thumbnails")
	if err != nil {
		return ai.AnalysisResult{}, err
	}
	dataURL, err := thumbnail.DataURLWithLimits(source.Path, cache, fmt.Sprintf("vision:%s:%d", source.Modified, source.Size), thumbnail.Limits{
		ImageEnabled: true, HEICEnabled: true, ImageMB: settings.AIVisionFileMB, ImageUnlimited: settings.AIVisionFileUnlimited,
		CacheMB: settings.ThumbnailCacheMB, CacheUnlimited: settings.ThumbnailCacheUnlimited,
		PDFEnabled: settings.PDFPreviewEnabled, PDFMB: settings.PDFPreviewMB, PDFUnlimited: settings.PDFPreviewUnlimited,
		VideoEnabled: settings.VideoPreviewEnabled, VideoMB: settings.VideoPreviewMB, VideoUnlimited: settings.VideoPreviewUnlimited,
	})
	if err != nil {
		return ai.AnalysisResult{}, fmt.Errorf("Bild für Vision vorbereiten: %w", err)
	}
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return ai.AnalysisResult{}, fmt.Errorf("Bildvorschau hat ein ungültiges Format")
	}
	imageBytes := base64.StdEncoding.DecodedLen(len(parts[1]))
	if !settings.AIVisionTotalUnlimited && int64(imageBytes) > int64(settings.AIVisionTotalMB)<<20 {
		return ai.AnalysisResult{}, fmt.Errorf("Aufbereitete Bilddaten überschreiten das Vision-Gesamtlimit")
	}
	credential, err := a.aiCredential(settings.AIProvider)
	if err != nil {
		return ai.AnalysisResult{}, err
	}
	ctx := a.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := ai.AnalyzeImage(ctx, ai.Config{Provider: settings.AIProvider, Endpoint: settings.AIEndpoint, Model: settings.AIVisionModel, TimeoutSeconds: settings.AITimeoutSeconds}, credential, ai.AnalysisRequest{
		Filename: input.Filename, Path: input.Path, MIMEType: input.MIMEType, Size: input.Size, Width: input.Width, Height: input.Height, Metadata: input.Metadata,
	}, dataURL)
	if err != nil {
		return ai.AnalysisResult{}, err
	}
	result.InputBytes = int64(imageBytes)
	result.AnalyzedAt = time.Now().UTC().Format(time.RFC3339)
	if err := a.catalog.SaveAIAnalysis(input, database.AIAnalysis{Summary: result.Summary, Tags: result.Tags, Provider: result.Provider, Model: result.Model, ImageBytes: result.InputBytes, Vision: true}); err != nil {
		return ai.AnalysisResult{}, fmt.Errorf("Vision-Analyse speichern: %w", err)
	}
	return result, nil
}

func truncateUTF8(value string, limit int64) string {
	if limit <= 0 {
		return ""
	}
	if int64(len(value)) <= limit {
		return value
	}
	end := int(limit)
	for end > 0 && (value[end]&0xc0) == 0x80 {
		end--
	}
	return value[:end]
}

func (a *App) aiCredential(provider string) (string, error) {
	if provider != "openrouter" {
		return "", nil
	}
	path, err := a.aiCredentialPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("für OpenRouter ist noch kein API-Schlüssel gespeichert")
		}
		return "", err
	}
	return string(data), nil
}

func (a *App) aiCredentialPath() (string, error) {
	if a.root == "" {
		return "", fmt.Errorf("Vault ist nicht bereit")
	}
	return vault.DataPath(a.root, filepath.Join("secrets", "ai-provider.key"))
}

func (a *App) CreateBackup() (BackupResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return BackupResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	settings := a.currentSettings()
	if !settings.BackupEnabled {
		return BackupResult{}, fmt.Errorf("Datensicherungen sind in den Einstellungen deaktiviert")
	}
	destination, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:            "VaultApp-Datensicherung speichern",
		DefaultDirectory: a.root,
		DefaultFilename:  fmt.Sprintf("VaultApp-Backup-%s.zip", time.Now().Format("2006-01-02_15-04")),
		Filters:          []wailsruntime.FileFilter{{DisplayName: "ZIP-Archiv", Pattern: "*.zip"}},
	})
	if err != nil {
		return BackupResult{}, err
	}
	if destination == "" {
		return BackupResult{Cancelled: true, Message: "Datensicherung abgebrochen"}, nil
	}
	if !strings.EqualFold(filepath.Ext(destination), ".zip") {
		destination += ".zip"
	}

	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	limits := backupLimits(settings)
	result, err := a.createBackupAt(destination, settings.BackupIncludeThumbnails, limits)
	if err != nil {
		return BackupResult{}, err
	}
	return BackupResult{Path: destination, Files: result.Files, Bytes: result.Bytes, Message: "Datensicherung erfolgreich erstellt"}, nil
}

func (a *App) GetManagedBackups() ([]ManagedBackup, error) {
	entries, err := os.ReadDir(a.root)
	if err != nil {
		return nil, err
	}
	result := make([]ManagedBackup, 0)
	for _, entry := range entries {
		kind, ok := managedBackupKind(entry.Name())
		if !ok || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil || !info.Mode().IsRegular() {
			continue
		}
		result = append(result, ManagedBackup{Name: entry.Name(), Path: filepath.Join(a.root, entry.Name()), Kind: kind, Size: info.Size(), Modified: info.ModTime().Format(time.RFC3339)})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Modified > result[j].Modified })
	return result, nil
}

func managedBackupKind(name string) (string, bool) {
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".zip") {
		return "", false
	}
	switch {
	case strings.HasPrefix(lower, "vaultapp-backup-"):
		return "Backup", true
	case strings.HasPrefix(lower, "vaultapp-rollback-"):
		return "Rückfallsicherung", true
	default:
		return "", false
	}
}

func (a *App) DeleteManagedBackup(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(a.root)
	if err != nil {
		return err
	}
	if filepath.Dir(absolute) != root && !(goruntime.GOOS == "windows" && strings.EqualFold(filepath.Dir(absolute), root)) {
		return fmt.Errorf("nur Sicherungen direkt im Vault-Ordner dürfen gelöscht werden")
	}
	if _, ok := managedBackupKind(filepath.Base(absolute)); !ok {
		return fmt.Errorf("Datei ist keine verwaltete VaultApp-Sicherung")
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Sicherung ist keine reguläre Datei")
	}
	return os.Remove(absolute)
}

func (a *App) InspectBackup(source string) (BackupInspection, error) {
	if a.initErr != nil || a.catalog == nil {
		return BackupInspection{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	if !a.currentSettings().BackupEnabled {
		return BackupInspection{}, fmt.Errorf("Datensicherungen sind in den Einstellungen deaktiviert")
	}
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	return a.inspectBackup(source)
}

func backupLimits(settings appconfig.Settings) backup.Limits {
	limits := backup.Limits{}
	if !settings.BackupFileUnlimited {
		limits.PerFileBytes = int64(settings.BackupFileMB) << 20
	}
	if !settings.BackupUnlimited {
		limits.TotalBytes = int64(settings.BackupMaxMB) << 20
	}
	return limits
}

func (a *App) createBackupAt(destination string, includeThumbnails bool, limits backup.Limits) (backup.Result, error) {
	snapshot, err := os.CreateTemp("", "vaultapp-catalog-*.db")
	if err != nil {
		return backup.Result{}, err
	}
	snapshotPath := snapshot.Name()
	_ = snapshot.Close()
	_ = os.Remove(snapshotPath)
	defer os.Remove(snapshotPath)
	if err := a.catalog.BackupTo(snapshotPath); err != nil {
		return backup.Result{}, err
	}
	sources := []backup.Source{
		{Path: snapshotPath, Name: "VaultApp-Backup/data/vault.db"},
		{Path: a.configPath, Name: "VaultApp-Backup/data/config.json"},
	}
	if includeThumbnails {
		thumbnailRoot, pathErr := vault.AssetPath(a.root, "thumbnails")
		if pathErr != nil {
			return backup.Result{}, pathErr
		}
		thumbnails, collectErr := backup.DirectorySources(thumbnailRoot, "VaultApp-Backup/assets/thumbnails")
		if collectErr != nil {
			return backup.Result{}, fmt.Errorf("Vorschaubilder sammeln: %w", collectErr)
		}
		sources = append(sources, thumbnails...)
	}
	return backup.Create(destination, sources, limits)
}

func (a *App) SelectBackupForRestore() (BackupInspection, error) {
	if a.initErr != nil || a.catalog == nil {
		return BackupInspection{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	if !a.currentSettings().BackupEnabled {
		return BackupInspection{}, fmt.Errorf("Datensicherungen sind in den Einstellungen deaktiviert")
	}
	selected, err := wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "VaultApp-Datensicherung prüfen", Filters: []wailsruntime.FileFilter{{DisplayName: "ZIP-Archiv", Pattern: "*.zip"}}})
	if err != nil {
		return BackupInspection{}, err
	}
	if selected == "" {
		return BackupInspection{Cancelled: true, Message: "Prüfung abgebrochen"}, nil
	}
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	return a.inspectBackup(selected)
}

func (a *App) inspectBackup(selected string) (BackupInspection, error) {
	staging, inspection, _, files, drives, err := a.prepareBackup(selected)
	if staging != "" {
		defer os.RemoveAll(staging)
	}
	if err != nil {
		return BackupInspection{}, err
	}
	return BackupInspection{Path: selected, CreatedAt: inspection.Manifest.CreatedAt, ArchiveFiles: inspection.Files, ArchiveBytes: inspection.Bytes, CatalogFiles: files, CatalogDrives: drives, IncludesThumbnails: inspection.IncludesThumbnails, Message: "Datensicherung ist gültig"}, nil
}

func (a *App) prepareBackup(source string) (string, backup.Inspection, appconfig.Settings, int64, int64, error) {
	staging, err := os.MkdirTemp(a.root, ".vaultapp-restore-*")
	if err != nil {
		return "", backup.Inspection{}, appconfig.Settings{}, 0, 0, err
	}
	inspection, err := backup.Extract(source, staging, backupLimits(a.currentSettings()))
	if err != nil {
		return staging, inspection, appconfig.Settings{}, 0, 0, err
	}
	settings, err := appconfig.Load(filepath.Join(staging, "data", "config.json"))
	if err != nil {
		return staging, inspection, settings, 0, 0, fmt.Errorf("Backup-Konfiguration prüfen: %w", err)
	}
	files, drives, err := database.Validate(filepath.Join(staging, "data", "vault.db"))
	if err != nil {
		return staging, inspection, settings, 0, 0, fmt.Errorf("Backup-Katalog prüfen: %w", err)
	}
	return staging, inspection, settings, files, drives, nil
}

type restoreSwap struct {
	source, destination, previous string
	hadPrevious                   bool
}

func applyRestoreSwaps(swaps []restoreSwap) (int, error) {
	for index := range swaps {
		swap := &swaps[index]
		affected := index
		if _, err := os.Stat(swap.destination); err == nil {
			swap.hadPrevious = true
			if err := os.Rename(swap.destination, swap.previous); err != nil {
				return index, err
			}
			affected = index + 1
		} else if !os.IsNotExist(err) {
			return index, err
		}
		if err := os.MkdirAll(filepath.Dir(swap.destination), 0o755); err != nil {
			return affected, err
		}
		if err := os.Rename(swap.source, swap.destination); err != nil {
			return index + 1, err
		}
	}
	return len(swaps), nil
}

func rollbackRestoreSwaps(swaps []restoreSwap, completed int) error {
	if completed > len(swaps) {
		completed = len(swaps)
	}
	var firstErr error
	for index := completed - 1; index >= 0; index-- {
		swap := swaps[index]
		if err := os.RemoveAll(swap.destination); err != nil && firstErr == nil {
			firstErr = err
		}
		if swap.hadPrevious {
			if err := os.Rename(swap.previous, swap.destination); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (a *App) reopenAfterFailedRestore(databasePath string, rollbackErr error) error {
	if rollbackErr != nil {
		a.initErr = fmt.Errorf("automatisches Zurücksetzen fehlgeschlagen: %w", rollbackErr)
		return a.initErr
	}
	catalog, err := database.Open(databasePath)
	if err != nil {
		a.initErr = fmt.Errorf("bisherigen Katalog erneut öffnen: %w", err)
		return a.initErr
	}
	a.catalog = catalog
	return nil
}

func (a *App) RestoreBackup(source string) (RestoreResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return RestoreResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	settings := a.currentSettings()
	if !settings.BackupEnabled {
		return RestoreResult{}, fmt.Errorf("Datensicherungen sind in den Einstellungen deaktiviert")
	}
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	staging, inspection, restoredSettings, files, drives, err := a.prepareBackup(source)
	if staging != "" {
		defer os.RemoveAll(staging)
	}
	if err != nil {
		return RestoreResult{}, err
	}
	rollbackPath := filepath.Join(a.root, fmt.Sprintf("VaultApp-Rollback-%s.zip", time.Now().Format("2006-01-02_15-04-05.000000000")))
	if _, err := a.createBackupAt(rollbackPath, true, backupLimits(settings)); err != nil {
		return RestoreResult{}, fmt.Errorf("Rückfallsicherung konnte nicht erstellt werden: %w", err)
	}
	databasePath, _ := vault.DataPath(a.root, "vault.db")
	thumbnailPath, _ := vault.AssetPath(a.root, "thumbnails")
	suffix := fmt.Sprintf(".restore-old-%d", time.Now().UnixNano())
	swaps := []restoreSwap{
		{source: filepath.Join(staging, "data", "vault.db"), destination: databasePath, previous: databasePath + suffix},
		{source: filepath.Join(staging, "data", "config.json"), destination: a.configPath, previous: a.configPath + suffix},
	}
	if inspection.IncludesThumbnails {
		swaps = append(swaps, restoreSwap{source: filepath.Join(staging, "assets", "thumbnails"), destination: thumbnailPath, previous: thumbnailPath + suffix})
	}
	_ = a.catalog.Close()
	a.catalog = nil
	completed, swapErr := applyRestoreSwaps(swaps)
	if swapErr != nil {
		rollbackErr := rollbackRestoreSwaps(swaps, completed)
		if reopenErr := a.reopenAfterFailedRestore(databasePath, rollbackErr); reopenErr != nil {
			return RestoreResult{}, fmt.Errorf("Wiederherstellung fehlgeschlagen; Rückfallsicherung liegt unter %s: %w", rollbackPath, reopenErr)
		}
		return RestoreResult{}, fmt.Errorf("Wiederherstellung austauschen: %w", swapErr)
	}
	restoredCatalog, openErr := database.Open(databasePath)
	if openErr != nil {
		rollbackErr := rollbackRestoreSwaps(swaps, len(swaps))
		if reopenErr := a.reopenAfterFailedRestore(databasePath, rollbackErr); reopenErr != nil {
			return RestoreResult{}, fmt.Errorf("Wiederherstellung fehlgeschlagen; Rückfallsicherung liegt unter %s: %w", rollbackPath, reopenErr)
		}
		return RestoreResult{}, fmt.Errorf("wiederhergestellten Katalog öffnen: %w", openErr)
	}
	a.catalog = restoredCatalog
	a.settingsMu.Lock()
	a.settings = restoredSettings
	a.settingsMu.Unlock()
	for _, swap := range swaps {
		_ = os.RemoveAll(swap.previous)
	}
	return RestoreResult{RollbackPath: rollbackPath, Files: files, Drives: drives, Message: "Datensicherung erfolgreich wiederhergestellt"}, nil
}

func (a *App) currentSettings() appconfig.Settings {
	a.settingsMu.RLock()
	defer a.settingsMu.RUnlock()
	return a.settings
}

func (a *App) GetDriveSnapshots(driveID int64) ([]database.Snapshot, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.Snapshots(driveID)
}

func (a *App) SearchSnapshot(snapshotID int64, query string, page int) (database.ArchiveResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return database.ArchiveResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.SearchArchive(snapshotID, query, page, 100)
}

func (a *App) DeleteSnapshot(snapshotID int64) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.DeleteSnapshot(snapshotID)
}

func (a *App) UpdateSnapshot(snapshotID int64, protected bool, note string, tags []string) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.UpdateSnapshot(snapshotID, protected, note, tags)
}

func (a *App) CompareSnapshot(snapshotID int64, status, query string, page int) (database.ComparisonResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return database.ComparisonResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	return a.catalog.CompareSnapshot(ctx, snapshotID, status, query, page, 100)
}

func (a *App) CompareSnapshotDirectory(snapshotID int64, directory, status string) ([]database.ComparisonTreeEntry, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	return a.catalog.CompareDirectory(ctx, snapshotID, directory, status)
}

// SelectAndScan catalogs metadata only. Source files are never modified.
func (a *App) SelectAndScan() (ScanResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return ScanResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	selected, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{Title: "Datenträger oder Ordner zum Scannen auswählen"})
	if err != nil {
		return ScanResult{}, err
	}
	if selected == "" {
		return ScanResult{Cancelled: true, Message: "Scan abgebrochen"}, nil
	}
	return a.scanPath(selected)
}

// ScanVolume starts a scan only for a path reported by the current volume list.
func (a *App) ScanVolume(path string) (ScanResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return ScanResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	if !a.currentSettings().VolumeDetectionEnabled {
		return ScanResult{}, fmt.Errorf("automatische Datenträgererkennung ist deaktiviert")
	}
	volumes, err := storage.ListVolumes()
	if err != nil {
		return ScanResult{}, fmt.Errorf("angeschlossene Datenträger erkennen: %w", err)
	}
	requested := filepath.Clean(path)
	for _, volume := range volumes {
		candidate := filepath.Clean(volume.Path)
		if requested == candidate || (goruntime.GOOS == "windows" && strings.EqualFold(requested, candidate)) {
			return a.scanPath(candidate)
		}
	}
	return ScanResult{}, fmt.Errorf("Datenträger ist nicht mehr angeschlossen oder wurde nicht erkannt")
}

func (a *App) scanPath(selected string) (ScanResult, error) {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	started := time.Now()
	wailsruntime.EventsEmit(a.ctx, "scan:progress", map[string]any{"phase": "scan", "files": 0, "path": selected})
	settings := a.currentSettings()
	identity, _ := storage.Identify(selected)
	storedTextBytes := int64(settings.TextStoredMB) << 20
	if settings.TextIndexEnabled && !settings.TextStoredUnlimited {
		used, usageErr := a.catalog.StoredTextBytesExcludingDrive(identity.UUID, selected)
		if usageErr != nil {
			return ScanResult{}, fmt.Errorf("Textindex-Speicherbelegung bestimmen: %w", usageErr)
		}
		storedTextBytes -= used
		if storedTextBytes < 0 {
			storedTextBytes = 0
		}
	}
	report, err := scanner.Scan(a.ctx, selected, a.root, scanner.ImageAnalysisOptions{
		Enabled: settings.ImageAnalysisEnabled, JPEG: settings.ImageJPEGEnabled, PNG: settings.ImagePNGEnabled, GIF: settings.ImageGIFEnabled, HEIC: settings.ImageHEICEnabled,
		PerFileBytes: int64(settings.ImageHeaderMB) << 20, TotalBytes: int64(settings.ImageScanBudgetMB) << 20,
		PerFileUnlimited: settings.ImageHeaderUnlimited, TotalUnlimited: settings.ImageScanBudgetUnlimited,
	}, scanner.EXIFAnalysisOptions{
		Enabled: settings.EXIFEnabled, PerFileBytes: int64(settings.EXIFFileMB) << 20, TotalBytes: int64(settings.EXIFTotalMB) << 20,
		PerFileUnlimited: settings.EXIFFileUnlimited, TotalUnlimited: settings.EXIFTotalUnlimited,
	}, scanner.TextIndexOptions{
		Enabled: settings.TextIndexEnabled, Documents: settings.TextDocumentsEnabled, PDF: settings.TextPDFEnabled, Office: settings.TextOfficeEnabled, Data: settings.TextDataEnabled, SourceCode: settings.TextSourceEnabled,
		PerFileBytes: int64(settings.TextFileMB) << 20, TotalBytes: int64(settings.TextTotalMB) << 20, StoredBytes: storedTextBytes,
		PerFileUnlimited: settings.TextFileUnlimited, TotalUnlimited: settings.TextTotalUnlimited, StoredLimitEnabled: !settings.TextStoredUnlimited,
	}, scanner.ExclusionOptions{
		Enabled: settings.ScanExclusionsEnabled, System: settings.ScanExcludeSystem, Development: settings.ScanExcludeDevelopment, Patterns: settings.ScanExcludedPatterns,
	}, func(count int, path string) {
		if count == 1 || count%250 == 0 {
			wailsruntime.EventsEmit(a.ctx, "scan:progress", map[string]any{"phase": "scan", "files": count, "path": path})
		}
	})
	if err != nil {
		a.writeScanDiagnostic(ScanDiagnostic{StartedAt: started, Drive: selected, Files: len(report.Files), Bytes: report.Bytes, Skipped: report.Skipped, Excluded: report.Excluded, Issues: report.Issues, IssuesTruncated: report.IssuesTruncated, Error: err.Error()})
		return ScanResult{}, err
	}
	totalSize, usedSize, _ := storage.Usage(selected)
	label := filepath.Base(filepath.Clean(selected))
	if identity.Label != "" {
		label = identity.Label
	}
	wailsruntime.EventsEmit(a.ctx, "scan:progress", map[string]any{"phase": "save", "files": len(report.Files), "path": selected})
	if err := a.catalog.ReplaceDriveScan(database.DriveScan{Path: selected, Label: label, Files: report.Files, TotalSize: totalSize, UsedSize: usedSize, UUID: identity.UUID, FSType: identity.FSType, Vendor: identity.Vendor, Model: identity.Model, Serial: identity.Serial, DeviceType: identity.DeviceType, Archive: settings.ArchiveEnabled, MaxSnapshots: settings.MaxSnapshots}); err != nil {
		a.writeScanDiagnostic(ScanDiagnostic{StartedAt: started, Drive: selected, Files: len(report.Files), Bytes: report.Bytes, Skipped: report.Skipped, Excluded: report.Excluded, Issues: report.Issues, IssuesTruncated: report.IssuesTruncated, Error: err.Error()})
		return ScanResult{}, err
	}
	diagnostic := ScanDiagnostic{StartedAt: started, Drive: selected, Files: len(report.Files), Bytes: report.Bytes, Skipped: report.Skipped, Excluded: report.Excluded, Issues: report.Issues, IssuesTruncated: report.IssuesTruncated}
	result := ScanResult{Drive: selected, Files: len(report.Files), Bytes: report.Bytes, Skipped: report.Skipped, Excluded: report.Excluded, Issues: report.Issues, IssuesTruncated: report.IssuesTruncated, Message: "Scan erfolgreich gespeichert"}
	result.LogPath = a.writeScanDiagnostic(diagnostic)
	wailsruntime.EventsEmit(a.ctx, "scan:complete", result)
	return result, nil
}
