package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"
	"sync"
	"time"

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
	Issues          []scanner.Issue `json:"issues"`
	IssuesTruncated bool            `json:"issuesTruncated"`
	Error           string          `json:"error,omitempty"`
	LogPath         string          `json:"logPath"`
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
}

type LibraryResult struct {
	Files      []database.FileResult `json:"files"`
	Extensions []string              `json:"extensions"`
	Total      int64                 `json:"total"`
	Page       int                   `json:"page"`
	PageSize   int                   `json:"pageSize"`
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
	info := AppInfo{Version: "0.32.0-dev", Platform: goruntime.GOOS, VaultRoot: a.root}
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

func (a *App) GetDrives() ([]database.Drive, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.Drives()
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
	candidates, err := a.catalog.HashCandidates()
	if err != nil {
		return DuplicateResult{}, err
	}
	result := DuplicateResult{}
	for index, candidate := range candidates {
		if candidate.Hash == "" {
			hash, hashErr := hashFile(candidate.SourcePath)
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

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
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
	cache, err := vault.AssetPath(a.root, "thumbnails")
	if err != nil {
		return "", err
	}
	settings := a.currentSettings()
	return thumbnail.DataURLWithLimits(source.Path, cache, fmt.Sprintf("%s:%d", source.Modified, source.Size), thumbnail.Limits{
		ImageEnabled: settings.ImagePreviewEnabled, HEICEnabled: settings.HEICPreviewEnabled, ImageMB: settings.ImagePreviewMB,
		ImageUnlimited: settings.ImagePreviewUnlimited, CacheMB: settings.ThumbnailCacheMB,
		CacheUnlimited: settings.ThumbnailCacheUnlimited, PDFMB: settings.PDFPreviewMB, VideoMB: settings.VideoPreviewMB,
	})
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
	report, err := scanner.Scan(a.ctx, selected, a.root, scanner.ImageAnalysisOptions{
		Enabled: settings.ImageAnalysisEnabled, JPEG: settings.ImageJPEGEnabled, PNG: settings.ImagePNGEnabled, GIF: settings.ImageGIFEnabled, HEIC: settings.ImageHEICEnabled,
		PerFileBytes: int64(settings.ImageHeaderMB) << 20, TotalBytes: int64(settings.ImageScanBudgetMB) << 20,
		PerFileUnlimited: settings.ImageHeaderUnlimited, TotalUnlimited: settings.ImageScanBudgetUnlimited,
	}, scanner.EXIFAnalysisOptions{
		Enabled: settings.EXIFEnabled, PerFileBytes: int64(settings.EXIFFileMB) << 20, TotalBytes: int64(settings.EXIFTotalMB) << 20,
		PerFileUnlimited: settings.EXIFFileUnlimited, TotalUnlimited: settings.EXIFTotalUnlimited,
	}, scanner.TextIndexOptions{
		Enabled: settings.TextIndexEnabled, Documents: settings.TextDocumentsEnabled, Data: settings.TextDataEnabled, SourceCode: settings.TextSourceEnabled,
		PerFileBytes: int64(settings.TextFileMB) << 20, TotalBytes: int64(settings.TextTotalMB) << 20,
		PerFileUnlimited: settings.TextFileUnlimited, TotalUnlimited: settings.TextTotalUnlimited,
	}, func(count int, path string) {
		if count == 1 || count%250 == 0 {
			wailsruntime.EventsEmit(a.ctx, "scan:progress", map[string]any{"phase": "scan", "files": count, "path": path})
		}
	})
	if err != nil {
		a.writeScanDiagnostic(ScanDiagnostic{StartedAt: started, Drive: selected, Files: len(report.Files), Bytes: report.Bytes, Skipped: report.Skipped, Issues: report.Issues, IssuesTruncated: report.IssuesTruncated, Error: err.Error()})
		return ScanResult{}, err
	}
	totalSize, usedSize, _ := storage.Usage(selected)
	identity, _ := storage.Identify(selected)
	label := filepath.Base(filepath.Clean(selected))
	if identity.Label != "" {
		label = identity.Label
	}
	wailsruntime.EventsEmit(a.ctx, "scan:progress", map[string]any{"phase": "save", "files": len(report.Files), "path": selected})
	if err := a.catalog.ReplaceDriveScan(database.DriveScan{Path: selected, Label: label, Files: report.Files, TotalSize: totalSize, UsedSize: usedSize, UUID: identity.UUID, FSType: identity.FSType, Vendor: identity.Vendor, Model: identity.Model, Serial: identity.Serial, DeviceType: identity.DeviceType, Archive: settings.ArchiveEnabled, MaxSnapshots: settings.MaxSnapshots}); err != nil {
		a.writeScanDiagnostic(ScanDiagnostic{StartedAt: started, Drive: selected, Files: len(report.Files), Bytes: report.Bytes, Skipped: report.Skipped, Issues: report.Issues, IssuesTruncated: report.IssuesTruncated, Error: err.Error()})
		return ScanResult{}, err
	}
	diagnostic := ScanDiagnostic{StartedAt: started, Drive: selected, Files: len(report.Files), Bytes: report.Bytes, Skipped: report.Skipped, Issues: report.Issues, IssuesTruncated: report.IssuesTruncated}
	result := ScanResult{Drive: selected, Files: len(report.Files), Bytes: report.Bytes, Skipped: report.Skipped, Issues: report.Issues, IssuesTruncated: report.IssuesTruncated, Message: "Scan erfolgreich gespeichert"}
	result.LogPath = a.writeScanDiagnostic(diagnostic)
	wailsruntime.EventsEmit(a.ctx, "scan:complete", result)
	return result, nil
}
