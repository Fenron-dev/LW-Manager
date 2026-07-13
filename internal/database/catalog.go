package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dennis/vaultapp/internal/scanner"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type Catalog struct{ db, readDB *sql.DB }
type DriveScan struct {
	Path, Label  string
	Files        []scanner.File
	TotalSize    int64
	UsedSize     int64
	UUID         string
	FSType       string
	Vendor       string
	Model        string
	Serial       string
	DeviceType   string
	Archive      bool
	MaxSnapshots int
}

type Drive struct {
	ID              int64  `json:"id"`
	Label           string `json:"label"`
	DisplayName     string `json:"displayName"`
	InventoryNumber string `json:"inventoryNumber"`
	Path            string `json:"path"`
	Manufacturer    string `json:"manufacturer"`
	DeviceType      string `json:"deviceType"`
	StorageLocation string `json:"storageLocation"`
	UUID            string `json:"uuid"`
	Serial          string `json:"serial"`
	Vendor          string `json:"vendor"`
	DetectedType    string `json:"detectedType"`
	FSType          string `json:"fsType"`
	Model           string `json:"model"`
	TotalSize       int64  `json:"totalSize"`
	UsedSize        int64  `json:"usedSize"`
	FileCount       int64  `json:"fileCount"`
	UpdatedAt       string `json:"updatedAt"`
}

type FileResult struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	Path      string `json:"path"`
	Extension string `json:"extension"`
	MIMEType  string `json:"mimeType"`
	Drive     string `json:"drive"`
	Size      int64  `json:"size"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Metadata  string `json:"metadata"`
	Modified  string `json:"modified"`
}

type SearchResult struct {
	Files      []FileResult `json:"files"`
	Extensions []string     `json:"extensions"`
	Total      int64        `json:"total"`
}

type HashCandidate struct {
	ID, Size         int64
	SourcePath, Hash string
}

type DuplicateFile struct {
	ID       int64  `json:"id"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
	Drive    string `json:"drive"`
}

type DuplicateGroup struct {
	Hash  string          `json:"hash"`
	Size  int64           `json:"size"`
	Files []DuplicateFile `json:"files"`
}

type DirectoryEntry struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"isDir"`
	Size      int64  `json:"size"`
	FileCount int64  `json:"fileCount"`
	Extension string `json:"extension"`
}

type SourceFile struct {
	Path, MIMEType, Modified string
	Size                     int64
}

type Snapshot struct {
	ID         int64  `json:"id"`
	CapturedAt string `json:"capturedAt"`
	FileCount  int64  `json:"fileCount"`
	TotalBytes int64  `json:"totalBytes"`
}

type ArchivedFile struct {
	Filename  string `json:"filename"`
	Path      string `json:"path"`
	Extension string `json:"extension"`
	MIMEType  string `json:"mimeType"`
	Modified  string `json:"modified"`
	Size      int64  `json:"size"`
}

type ArchiveResult struct {
	Files    []ArchivedFile `json:"files"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"pageSize"`
}

type ComparisonEntry struct {
	Path            string `json:"path"`
	Status          string `json:"status"`
	CurrentName     string `json:"currentName"`
	CurrentSize     int64  `json:"currentSize"`
	CurrentModified string `json:"currentModified"`
	ArchiveName     string `json:"archiveName"`
	ArchiveSize     int64  `json:"archiveSize"`
	ArchiveModified string `json:"archiveModified"`
}

type ComparisonResult struct {
	Entries  []ComparisonEntry `json:"entries"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"pageSize"`
}

type ComparisonTreeEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"isDir"`
	Status    string `json:"status"`
	Added     int64  `json:"added"`
	Removed   int64  `json:"removed"`
	Modified  int64  `json:"modified"`
	Unchanged int64  `json:"unchanged"`
}

func Open(path string) (*Catalog, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("Datenbankschema: %w", err)
	}
	readDB, err := sql.Open("sqlite", path)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	readDB.SetMaxOpenConns(2)
	if _, err := readDB.Exec("PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000; PRAGMA query_only=ON"); err != nil {
		_ = readDB.Close()
		_ = db.Close()
		return nil, err
	}
	catalog := &Catalog{db: db, readDB: readDB}
	if err := catalog.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("Datenbankmigration: %w", err)
	}
	return catalog, nil
}
func (c *Catalog) Close() error { _ = c.readDB.Close(); return c.db.Close() }
func (c *Catalog) Stats() (files, drives int64, err error) {
	if err = c.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&files); err != nil {
		return
	}
	err = c.db.QueryRow("SELECT COUNT(*) FROM drives").Scan(&drives)
	return
}

func (c *Catalog) Search(query, extension string, driveID int64, limit, offset int) (SearchResult, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	pattern := "%" + escapeLike(strings.TrimSpace(query)) + "%"
	extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
	where := `(LOWER(f.filename) LIKE LOWER(?) ESCAPE '\' OR LOWER(f.path) LIKE LOWER(?) ESCAPE '\') AND (? = '' OR f.extension = ?) AND (? = 0 OR f.drive_id = ?)`
	args := []any{pattern, pattern, extension, extension, driveID, driveID}
	result := SearchResult{Extensions: make([]string, 0)}
	if err := c.db.QueryRow("SELECT COUNT(*) FROM files f WHERE "+where, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := c.db.Query(`SELECT f.id,f.filename,f.path,COALESCE(f.extension,''),COALESCE(f.mime_type,''),COALESCE(NULLIF(d.display_name,''),d.label),f.size,COALESCE(f.width,0),COALESCE(f.height,0),COALESCE(f.metadata,''),COALESCE(f.modified_at,'')
		FROM files f JOIN drives d ON d.id=f.drive_id WHERE `+where+` ORDER BY f.filename COLLATE NOCASE,f.path LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	result.Files = make([]FileResult, 0, limit)
	for rows.Next() {
		var file FileResult
		if err := rows.Scan(&file.ID, &file.Filename, &file.Path, &file.Extension, &file.MIMEType, &file.Drive, &file.Size, &file.Width, &file.Height, &file.Metadata, &file.Modified); err != nil {
			return result, err
		}
		result.Files = append(result.Files, file)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	extensionRows, err := c.db.Query(`SELECT extension FROM files WHERE extension IS NOT NULL AND extension <> '' GROUP BY extension ORDER BY extension COLLATE NOCASE LIMIT 250`)
	if err != nil {
		return result, err
	}
	defer extensionRows.Close()
	for extensionRows.Next() {
		var value string
		if err := extensionRows.Scan(&value); err != nil {
			return result, err
		}
		result.Extensions = append(result.Extensions, value)
	}
	return result, extensionRows.Err()
}

func (c *Catalog) HashCandidates() ([]HashCandidate, error) {
	rows, err := c.readDB.Query(`SELECT f.id,f.size,d.vault_path,f.path,COALESCE(f.content_hash,'')
		FROM files f JOIN drives d ON d.id=f.drive_id
		WHERE f.size IN (SELECT size FROM files GROUP BY size HAVING COUNT(*)>1)
		ORDER BY f.size,f.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]HashCandidate, 0)
	for rows.Next() {
		var candidate HashCandidate
		var root, relative string
		if err := rows.Scan(&candidate.ID, &candidate.Size, &root, &relative, &candidate.Hash); err != nil {
			return nil, err
		}
		candidate.SourcePath = filepath.Join(root, filepath.FromSlash(relative))
		result = append(result, candidate)
	}
	return result, rows.Err()
}

func (c *Catalog) SaveFileHash(id int64, hash string) error {
	_, err := c.db.Exec("UPDATE files SET content_hash=? WHERE id=?", hash, id)
	return err
}

func (c *Catalog) DuplicateGroups() ([]DuplicateGroup, error) {
	rows, err := c.readDB.Query(`SELECT f.content_hash,f.size,f.id,f.filename,f.path,COALESCE(NULLIF(d.display_name,''),d.label)
		FROM files f JOIN drives d ON d.id=f.drive_id
		WHERE f.content_hash IN (SELECT content_hash FROM files WHERE content_hash IS NOT NULL AND content_hash<>'' GROUP BY content_hash HAVING COUNT(*)>1)
		ORDER BY f.content_hash,COALESCE(NULLIF(d.display_name,''),d.label),f.path`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]DuplicateGroup, 0)
	for rows.Next() {
		var hash string
		var size, id int64
		var filename, path, drive string
		if err := rows.Scan(&hash, &size, &id, &filename, &path, &drive); err != nil {
			return nil, err
		}
		if len(groups) == 0 || groups[len(groups)-1].Hash != hash {
			groups = append(groups, DuplicateGroup{Hash: hash, Size: size, Files: make([]DuplicateFile, 0, 2)})
		}
		group := &groups[len(groups)-1]
		group.Files = append(group.Files, DuplicateFile{ID: id, Filename: filename, Path: path, Drive: drive})
	}
	return groups, rows.Err()
}

func (c *Catalog) Drives() ([]Drive, error) {
	rows, err := c.db.Query(`SELECT d.id,d.label,COALESCE(d.display_name,''),COALESCE(d.inventory_number,''),COALESCE(d.vault_path,''),
		COALESCE(d.manufacturer,''),COALESCE(d.device_type,''),COALESCE(d.storage_location,''),d.uuid,COALESCE(d.serial,''),COALESCE(d.vendor,''),COALESCE(d.detected_type,''),COALESCE(d.fs_type,''),COALESCE(d.model,''),COALESCE(d.total_size,0),COALESCE(d.used_size,0),COUNT(f.id),d.updated_at
		FROM drives d LEFT JOIN files f ON f.drive_id=d.id GROUP BY d.id ORDER BY COALESCE(NULLIF(d.display_name,''),d.label) COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	drives := make([]Drive, 0)
	for rows.Next() {
		var drive Drive
		if err := rows.Scan(&drive.ID, &drive.Label, &drive.DisplayName, &drive.InventoryNumber, &drive.Path, &drive.Manufacturer, &drive.DeviceType, &drive.StorageLocation, &drive.UUID, &drive.Serial, &drive.Vendor, &drive.DetectedType, &drive.FSType, &drive.Model, &drive.TotalSize, &drive.UsedSize, &drive.FileCount, &drive.UpdatedAt); err != nil {
			return nil, err
		}
		drives = append(drives, drive)
	}
	return drives, rows.Err()
}

func (c *Catalog) UpdateDrive(id int64, displayName, inventoryNumber, manufacturer, deviceType, storageLocation string) error {
	result, err := c.db.Exec(`UPDATE drives SET display_name=?,inventory_number=?,manufacturer=?,device_type=?,storage_location=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		strings.TrimSpace(displayName), strings.TrimSpace(inventoryNumber), strings.TrimSpace(manufacturer), strings.TrimSpace(deviceType), strings.TrimSpace(storageLocation), id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("Datenträger %d wurde nicht gefunden", id)
	}
	return nil
}

func (c *Catalog) StorageLocations() ([]string, error) {
	rows, err := c.readDB.Query(`SELECT name FROM storage_locations ORDER BY name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	locations := make([]string, 0)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		locations = append(locations, name)
	}
	return locations, rows.Err()
}

func (c *Catalog) AddStorageLocation(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("Lagerort darf nicht leer sein")
	}
	_, err := c.db.Exec(`INSERT INTO storage_locations(name) VALUES(?) ON CONFLICT(name) DO NOTHING`, name)
	return err
}

func (c *Catalog) Directory(ctx context.Context, driveID int64, directory string) ([]DirectoryEntry, error) {
	directory = strings.Trim(strings.ReplaceAll(directory, `\`, "/"), "/")
	if directory == ".." || strings.HasPrefix(directory, "../") || strings.Contains(directory, "/../") {
		return nil, fmt.Errorf("ungültiger Verzeichnispfad")
	}
	prefix := ""
	if directory != "" {
		prefix = directory + "/"
	}
	rows, err := c.readDB.QueryContext(ctx, `SELECT id,path,size,COALESCE(extension,'') FROM files WHERE drive_id=? AND path LIKE ? ESCAPE '\' ORDER BY path`, driveID, escapeLike(prefix)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := map[string]*DirectoryEntry{}
	for rows.Next() {
		var path, extension string
		var size int64
		var id int64
		if err := rows.Scan(&id, &path, &size, &extension); err != nil {
			return nil, err
		}
		remainder := strings.TrimPrefix(path, prefix)
		parts := strings.SplitN(remainder, "/", 2)
		name := parts[0]
		if name == "" {
			continue
		}
		entry, exists := entries[name]
		if !exists {
			entry = &DirectoryEntry{ID: id, Name: name, Path: prefix + name, IsDir: len(parts) == 2, Extension: extension}
			entries[name] = entry
		}
		entry.Size += size
		entry.FileCount++
		if len(parts) == 2 {
			entry.ID = 0
			entry.IsDir = true
			entry.Extension = ""
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func (c *Catalog) SourceFile(id int64) (SourceFile, error) {
	var source SourceFile
	var root, relative string
	err := c.db.QueryRow(`SELECT d.vault_path,f.path,COALESCE(f.mime_type,''),f.size,COALESCE(f.modified_at,'') FROM files f JOIN drives d ON d.id=f.drive_id WHERE f.id=?`, id).
		Scan(&root, &relative, &source.MIMEType, &source.Size, &source.Modified)
	if err != nil {
		return source, err
	}
	if filepath.IsAbs(relative) {
		return source, fmt.Errorf("ungültiger absoluter Dateipfad")
	}
	root = filepath.Clean(root)
	source.Path = filepath.Join(root, filepath.FromSlash(relative))
	inside, err := filepath.Rel(root, source.Path)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return SourceFile{}, fmt.Errorf("Dateipfad verlässt den Datenträger")
	}
	return source, nil
}

func (c *Catalog) FileDetails(id int64) (FileResult, error) {
	var file FileResult
	err := c.readDB.QueryRow(`SELECT f.id,f.filename,f.path,COALESCE(f.extension,''),COALESCE(f.mime_type,''),COALESCE(NULLIF(d.display_name,''),d.label),f.size,COALESCE(f.width,0),COALESCE(f.height,0),COALESCE(f.metadata,''),COALESCE(f.modified_at,'')
		FROM files f JOIN drives d ON d.id=f.drive_id WHERE f.id=?`, id).
		Scan(&file.ID, &file.Filename, &file.Path, &file.Extension, &file.MIMEType, &file.Drive, &file.Size, &file.Width, &file.Height, &file.Metadata, &file.Modified)
	return file, err
}

func (c *Catalog) Snapshots(driveID int64) ([]Snapshot, error) {
	rows, err := c.db.Query(`SELECT id,captured_at,file_count,total_bytes FROM scan_snapshots WHERE drive_id=? ORDER BY captured_at DESC,id DESC`, driveID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Snapshot, 0)
	for rows.Next() {
		var snapshot Snapshot
		if err := rows.Scan(&snapshot.ID, &snapshot.CapturedAt, &snapshot.FileCount, &snapshot.TotalBytes); err != nil {
			return nil, err
		}
		result = append(result, snapshot)
	}
	return result, rows.Err()
}

func (c *Catalog) DeleteSnapshot(id int64) error {
	result, err := c.db.Exec("DELETE FROM scan_snapshots WHERE id=?", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("Archivstand %d wurde nicht gefunden", id)
	}
	return nil
}

func (c *Catalog) SearchArchive(snapshotID int64, query string, page, pageSize int) (ArchiveResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 100
	}
	pattern := "%" + escapeLike(strings.TrimSpace(query)) + "%"
	args := []any{snapshotID, pattern, pattern}
	result := ArchiveResult{Files: make([]ArchivedFile, 0), Page: page, PageSize: pageSize}
	where := `snapshot_id=? AND (LOWER(filename) LIKE LOWER(?) ESCAPE '\' OR LOWER(path) LIKE LOWER(?) ESCAPE '\')`
	if err := c.db.QueryRow("SELECT COUNT(*) FROM archived_files WHERE "+where, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := c.db.Query(`SELECT filename,path,COALESCE(extension,''),size,COALESCE(mime_type,''),COALESCE(modified_at,'') FROM archived_files WHERE `+where+` ORDER BY filename COLLATE NOCASE,path LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var file ArchivedFile
		if err := rows.Scan(&file.Filename, &file.Path, &file.Extension, &file.Size, &file.MIMEType, &file.Modified); err != nil {
			return result, err
		}
		result.Files = append(result.Files, file)
	}
	return result, rows.Err()
}

func (c *Catalog) CompareSnapshot(ctx context.Context, snapshotID int64, status, query string, page, pageSize int) (ComparisonResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 100
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && status != "added" && status != "removed" && status != "modified" && status != "unchanged" {
		return ComparisonResult{}, fmt.Errorf("ungültiger Vergleichsfilter")
	}
	const comparison = `
		SELECT f.path,
		 CASE WHEN a.id IS NULL THEN 'added' WHEN f.size<>a.size OR COALESCE(f.modified_at,'')<>COALESCE(a.modified_at,'') THEN 'modified' ELSE 'unchanged' END status,
		 f.filename current_name,f.size current_size,COALESCE(f.modified_at,'') current_modified,
		 COALESCE(a.filename,'') archive_name,COALESCE(a.size,0) archive_size,COALESCE(a.modified_at,'') archive_modified
		FROM files f JOIN scan_snapshots s ON s.id=? AND s.drive_id=f.drive_id LEFT JOIN archived_files a ON a.snapshot_id=s.id AND a.path=f.path
		UNION ALL
		SELECT a.path,'removed','',0,'',a.filename,a.size,COALESCE(a.modified_at,'')
		FROM archived_files a JOIN scan_snapshots s ON s.id=a.snapshot_id LEFT JOIN files f ON f.drive_id=s.drive_id AND f.path=a.path
		WHERE a.snapshot_id=? AND f.id IS NULL`
	pattern := "%" + escapeLike(strings.TrimSpace(query)) + "%"
	where := `(?='' OR status=?) AND LOWER(path) LIKE LOWER(?) ESCAPE '\'`
	args := []any{snapshotID, snapshotID, status, status, pattern}
	result := ComparisonResult{Entries: make([]ComparisonEntry, 0), Page: page, PageSize: pageSize}
	if err := c.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM ("+comparison+") WHERE "+where, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := c.readDB.QueryContext(ctx, "SELECT path,status,current_name,current_size,current_modified,archive_name,archive_size,archive_modified FROM ("+comparison+") WHERE "+where+" ORDER BY path COLLATE NOCASE LIMIT ? OFFSET ?", append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry ComparisonEntry
		if err := rows.Scan(&entry.Path, &entry.Status, &entry.CurrentName, &entry.CurrentSize, &entry.CurrentModified, &entry.ArchiveName, &entry.ArchiveSize, &entry.ArchiveModified); err != nil {
			return result, err
		}
		result.Entries = append(result.Entries, entry)
	}
	return result, rows.Err()
}

func (c *Catalog) CompareDirectory(ctx context.Context, snapshotID int64, directory, status string) ([]ComparisonTreeEntry, error) {
	directory = strings.Trim(strings.ReplaceAll(directory, `\`, "/"), "/")
	if directory == ".." || strings.HasPrefix(directory, "../") || strings.Contains(directory, "/../") {
		return nil, fmt.Errorf("ungültiger Verzeichnispfad")
	}
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && status != "added" && status != "removed" && status != "modified" && status != "unchanged" {
		return nil, fmt.Errorf("ungültiger Vergleichsfilter")
	}
	const comparison = `
		SELECT f.path,CASE WHEN a.id IS NULL THEN 'added' WHEN f.size<>a.size OR COALESCE(f.modified_at,'')<>COALESCE(a.modified_at,'') THEN 'modified' ELSE 'unchanged' END status
		FROM files f JOIN scan_snapshots s ON s.id=? AND s.drive_id=f.drive_id LEFT JOIN archived_files a ON a.snapshot_id=s.id AND a.path=f.path
		UNION ALL
		SELECT a.path,'removed' FROM archived_files a JOIN scan_snapshots s ON s.id=a.snapshot_id LEFT JOIN files f ON f.drive_id=s.drive_id AND f.path=a.path
		WHERE a.snapshot_id=? AND f.id IS NULL`
	prefix := ""
	if directory != "" {
		prefix = directory + "/"
	}
	query := "SELECT path,status FROM (" + comparison + `) WHERE (?='' OR status=?) AND path LIKE ? ESCAPE '\'`
	rows, err := c.readDB.QueryContext(ctx, query, snapshotID, snapshotID, status, status, escapeLike(prefix)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := map[string]*ComparisonTreeEntry{}
	for rows.Next() {
		var path, itemStatus string
		if err := rows.Scan(&path, &itemStatus); err != nil {
			return nil, err
		}
		remainder := strings.TrimPrefix(path, prefix)
		parts := strings.SplitN(remainder, "/", 2)
		name := parts[0]
		if name == "" {
			continue
		}
		entry := entries[name]
		if entry == nil {
			entry = &ComparisonTreeEntry{Name: name, Path: prefix + name, IsDir: len(parts) == 2}
			entries[name] = entry
		}
		if len(parts) == 2 {
			entry.IsDir = true
		}
		switch itemStatus {
		case "added":
			entry.Added++
		case "removed":
			entry.Removed++
		case "modified":
			entry.Modified++
		default:
			entry.Unchanged++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result := make([]ComparisonTreeEntry, 0, len(entries))
	for _, entry := range entries {
		entry.Status = treeStatus(entry)
		result = append(result, *entry)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func treeStatus(entry *ComparisonTreeEntry) string {
	count, status := 0, "unchanged"
	for _, candidate := range []struct {
		name  string
		value int64
	}{{"added", entry.Added}, {"removed", entry.Removed}, {"modified", entry.Modified}, {"unchanged", entry.Unchanged}} {
		if candidate.value > 0 {
			count++
			status = candidate.name
		}
	}
	if count > 1 {
		return "mixed"
	}
	return status
}

func (c *Catalog) migrate() error {
	columns := map[string]string{
		"display_name": "TEXT", "inventory_number": "TEXT", "manufacturer": "TEXT", "device_type": "TEXT",
		"storage_location": "TEXT", "total_size": "INTEGER NOT NULL DEFAULT 0", "used_size": "INTEGER NOT NULL DEFAULT 0",
		"serial": "TEXT", "vendor": "TEXT", "model": "TEXT", "fs_type": "TEXT",
		"detected_type": "TEXT",
	}
	rows, err := c.db.Query("PRAGMA table_info(drives)")
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			rows.Close()
			return err
		}
		existing[name] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}
	for name, definition := range columns {
		if !existing[name] {
			if _, err := c.db.Exec("ALTER TABLE drives ADD COLUMN " + name + " " + definition); err != nil {
				return err
			}
		}
	}
	for _, name := range []string{"width", "height"} {
		var count int
		if err := c.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('files') WHERE name=?", name).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := c.db.Exec("ALTER TABLE files ADD COLUMN " + name + " INTEGER NOT NULL DEFAULT 0"); err != nil {
				return err
			}
		}
	}
	return nil
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}
func (c *Catalog) ReplaceDriveScan(scan DriveScan) error {
	absolute, err := filepath.Abs(scan.Path)
	if err != nil {
		return err
	}
	uuid := strings.TrimSpace(scan.UUID)
	if uuid == "" {
		uuid = fmt.Sprintf("path:%x", sha256.Sum256([]byte(filepath.Clean(absolute))))
	} else {
		uuid = "volume:" + strings.ToLower(uuid)
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if scan.UUID != "" {
		_, err = tx.Exec(`UPDATE drives SET uuid=? WHERE vault_path=? AND uuid<>? AND NOT EXISTS(SELECT 1 FROM drives WHERE uuid=?)`, uuid, absolute, uuid, uuid)
		if err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT INTO drives(uuid,label,vault_path,total_size,used_size,fs_type,vendor,model,serial,detected_type,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(uuid) DO UPDATE SET label=excluded.label,vault_path=excluded.vault_path,total_size=excluded.total_size,used_size=excluded.used_size,fs_type=excluded.fs_type,vendor=excluded.vendor,model=excluded.model,serial=excluded.serial,detected_type=excluded.detected_type,updated_at=CURRENT_TIMESTAMP`, uuid, scan.Label, absolute, scan.TotalSize, scan.UsedSize, scan.FSType, scan.Vendor, scan.Model, scan.Serial, scan.DeviceType); err != nil {
		return err
	}
	var driveID int64
	if err = tx.QueryRow("SELECT id FROM drives WHERE uuid=?", uuid).Scan(&driveID); err != nil {
		return err
	}
	var previousCount, previousBytes int64
	if err = tx.QueryRow("SELECT COUNT(*),COALESCE(SUM(size),0) FROM files WHERE drive_id=?", driveID).Scan(&previousCount, &previousBytes); err != nil {
		return err
	}
	if previousCount > 0 && scan.Archive {
		result, err := tx.Exec("INSERT INTO scan_snapshots(drive_id,file_count,total_bytes) VALUES(?,?,?)", driveID, previousCount, previousBytes)
		if err != nil {
			return err
		}
		snapshotID, err := result.LastInsertId()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO archived_files(snapshot_id,path,filename,extension,size,mime_type,modified_at)
			SELECT ?,path,filename,extension,size,mime_type,modified_at FROM files WHERE drive_id=?`, snapshotID, driveID); err != nil {
			return err
		}
		if scan.MaxSnapshots > 0 {
			if _, err = tx.Exec(`DELETE FROM scan_snapshots WHERE drive_id=? AND id NOT IN
				(SELECT id FROM scan_snapshots WHERE drive_id=? ORDER BY id DESC LIMIT ?)`, driveID, driveID, scan.MaxSnapshots); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec("DELETE FROM files WHERE drive_id=?", driveID); err != nil {
		return err
	}
	statement, err := tx.Prepare(`INSERT INTO files(drive_id,path,filename,extension,size,width,height,mime_type,metadata,created_at,modified_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, file := range scan.Files {
		var created any
		if !file.CreatedAt.IsZero() {
			created = file.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		if _, err = statement.Exec(driveID, file.Path, file.Filename, file.Extension, file.Size, file.Width, file.Height, file.MIMEType, file.Metadata, created, file.Modified.UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
