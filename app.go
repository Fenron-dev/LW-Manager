package main

import (
	"context"
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"sync"

	"github.com/dennis/vaultapp/internal/database"
	"github.com/dennis/vaultapp/internal/scanner"
	"github.com/dennis/vaultapp/internal/storage"
	"github.com/dennis/vaultapp/internal/thumbnail"
	"github.com/dennis/vaultapp/internal/vault"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx     context.Context
	mu      sync.Mutex
	root    string
	catalog *database.Catalog
	initErr error
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
	Cancelled bool   `json:"cancelled"`
	Drive     string `json:"drive"`
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
	Skipped   int    `json:"skipped"`
	Message   string `json:"message"`
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
	info := AppInfo{Version: "0.8.0-dev", Platform: goruntime.GOOS, VaultRoot: a.root}
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

func (a *App) SearchFiles(query, extension string, driveID int64, page int) (LibraryResult, error) {
	if a.initErr != nil || a.catalog == nil {
		return LibraryResult{}, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	if page < 1 {
		page = 1
	}
	const pageSize = 50
	result, err := a.catalog.Search(query, extension, driveID, pageSize, (page-1)*pageSize)
	if err != nil {
		return LibraryResult{}, err
	}
	return LibraryResult{Files: result.Files, Extensions: result.Extensions, Total: result.Total, Page: page, PageSize: pageSize}, nil
}

func (a *App) GetDrives() ([]database.Drive, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.Drives()
}

func (a *App) UpdateDrive(id int64, displayName, inventoryNumber, manufacturer, deviceType string) error {
	if a.initErr != nil || a.catalog == nil {
		return fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.UpdateDrive(id, displayName, inventoryNumber, manufacturer, deviceType)
}

func (a *App) BrowseDrive(id int64, directory string) ([]database.DirectoryEntry, error) {
	if a.initErr != nil || a.catalog == nil {
		return nil, fmt.Errorf("Vault ist nicht bereit: %v", a.initErr)
	}
	return a.catalog.Directory(id, directory)
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
	return thumbnail.DataURL(source.Path, cache, fmt.Sprintf("%s:%d", source.Modified, source.Size))
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

	a.mu.Lock()
	defer a.mu.Unlock()
	wailsruntime.EventsEmit(a.ctx, "scan:progress", map[string]any{"phase": "scan", "files": 0, "path": selected})
	report, err := scanner.Scan(a.ctx, selected, a.root, func(count int, path string) {
		if count == 1 || count%250 == 0 {
			wailsruntime.EventsEmit(a.ctx, "scan:progress", map[string]any{"phase": "scan", "files": count, "path": path})
		}
	})
	if err != nil {
		return ScanResult{}, err
	}
	totalSize, usedSize, _ := storage.Usage(selected)
	wailsruntime.EventsEmit(a.ctx, "scan:progress", map[string]any{"phase": "save", "files": len(report.Files), "path": selected})
	if err := a.catalog.ReplaceDriveScan(database.DriveScan{Path: selected, Label: filepath.Base(filepath.Clean(selected)), Files: report.Files, TotalSize: totalSize, UsedSize: usedSize}); err != nil {
		return ScanResult{}, err
	}
	result := ScanResult{Drive: selected, Files: len(report.Files), Bytes: report.Bytes, Skipped: report.Skipped, Message: "Scan erfolgreich gespeichert"}
	wailsruntime.EventsEmit(a.ctx, "scan:complete", result)
	return result, nil
}
