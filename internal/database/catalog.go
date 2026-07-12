package database

import (
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

type Catalog struct{ db *sql.DB }
type DriveScan struct {
	Path, Label string
	Files       []scanner.File
	TotalSize   int64
	UsedSize    int64
}

type Drive struct {
	ID              int64  `json:"id"`
	Label           string `json:"label"`
	DisplayName     string `json:"displayName"`
	InventoryNumber string `json:"inventoryNumber"`
	Path            string `json:"path"`
	Manufacturer    string `json:"manufacturer"`
	DeviceType      string `json:"deviceType"`
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
	Modified  string `json:"modified"`
}

type SearchResult struct {
	Files      []FileResult `json:"files"`
	Extensions []string     `json:"extensions"`
	Total      int64        `json:"total"`
}

type DirectoryEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"isDir"`
	Size      int64  `json:"size"`
	FileCount int64  `json:"fileCount"`
	Extension string `json:"extension"`
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
	catalog := &Catalog{db: db}
	if err := catalog.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("Datenbankmigration: %w", err)
	}
	return catalog, nil
}
func (c *Catalog) Close() error { return c.db.Close() }
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
	rows, err := c.db.Query(`SELECT f.id,f.filename,f.path,COALESCE(f.extension,''),COALESCE(f.mime_type,''),COALESCE(NULLIF(d.display_name,''),d.label),f.size,COALESCE(f.modified_at,'')
		FROM files f JOIN drives d ON d.id=f.drive_id WHERE `+where+` ORDER BY f.filename COLLATE NOCASE,f.path LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	result.Files = make([]FileResult, 0, limit)
	for rows.Next() {
		var file FileResult
		if err := rows.Scan(&file.ID, &file.Filename, &file.Path, &file.Extension, &file.MIMEType, &file.Drive, &file.Size, &file.Modified); err != nil {
			return result, err
		}
		result.Files = append(result.Files, file)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	extensionRows, err := c.db.Query(`SELECT extension FROM files WHERE extension IS NOT NULL AND extension <> '' GROUP BY extension ORDER BY COUNT(*) DESC,extension LIMIT 100`)
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

func (c *Catalog) Drives() ([]Drive, error) {
	rows, err := c.db.Query(`SELECT d.id,d.label,COALESCE(d.display_name,''),COALESCE(d.inventory_number,''),COALESCE(d.vault_path,''),
		COALESCE(d.manufacturer,''),COALESCE(d.device_type,''),COALESCE(d.total_size,0),COALESCE(d.used_size,0),COUNT(f.id),d.updated_at
		FROM drives d LEFT JOIN files f ON f.drive_id=d.id GROUP BY d.id ORDER BY COALESCE(NULLIF(d.display_name,''),d.label) COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	drives := make([]Drive, 0)
	for rows.Next() {
		var drive Drive
		if err := rows.Scan(&drive.ID, &drive.Label, &drive.DisplayName, &drive.InventoryNumber, &drive.Path, &drive.Manufacturer, &drive.DeviceType, &drive.TotalSize, &drive.UsedSize, &drive.FileCount, &drive.UpdatedAt); err != nil {
			return nil, err
		}
		drives = append(drives, drive)
	}
	return drives, rows.Err()
}

func (c *Catalog) UpdateDrive(id int64, displayName, inventoryNumber, manufacturer, deviceType string) error {
	result, err := c.db.Exec(`UPDATE drives SET display_name=?,inventory_number=?,manufacturer=?,device_type=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		strings.TrimSpace(displayName), strings.TrimSpace(inventoryNumber), strings.TrimSpace(manufacturer), strings.TrimSpace(deviceType), id)
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

func (c *Catalog) Directory(driveID int64, directory string) ([]DirectoryEntry, error) {
	directory = strings.Trim(strings.ReplaceAll(directory, `\`, "/"), "/")
	if directory == ".." || strings.HasPrefix(directory, "../") || strings.Contains(directory, "/../") {
		return nil, fmt.Errorf("ungültiger Verzeichnispfad")
	}
	prefix := ""
	if directory != "" {
		prefix = directory + "/"
	}
	rows, err := c.db.Query(`SELECT path,size,COALESCE(extension,'') FROM files WHERE drive_id=? AND path LIKE ? ESCAPE '\' ORDER BY path`, driveID, escapeLike(prefix)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := map[string]*DirectoryEntry{}
	for rows.Next() {
		var path, extension string
		var size int64
		if err := rows.Scan(&path, &size, &extension); err != nil {
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
			entry = &DirectoryEntry{Name: name, Path: prefix + name, IsDir: len(parts) == 2, Extension: extension}
			entries[name] = entry
		}
		entry.Size += size
		entry.FileCount++
		if len(parts) == 2 {
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

func (c *Catalog) migrate() error {
	columns := map[string]string{
		"display_name": "TEXT", "inventory_number": "TEXT", "manufacturer": "TEXT", "device_type": "TEXT",
		"total_size": "INTEGER NOT NULL DEFAULT 0", "used_size": "INTEGER NOT NULL DEFAULT 0",
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
	uuid := fmt.Sprintf("%x", sha256.Sum256([]byte(filepath.Clean(absolute))))
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO drives(uuid,label,vault_path,total_size,used_size,updated_at) VALUES(?,?,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(uuid) DO UPDATE SET label=excluded.label,vault_path=excluded.vault_path,total_size=excluded.total_size,used_size=excluded.used_size,updated_at=CURRENT_TIMESTAMP`, uuid, scan.Label, absolute, scan.TotalSize, scan.UsedSize); err != nil {
		return err
	}
	var driveID int64
	if err = tx.QueryRow("SELECT id FROM drives WHERE uuid=?", uuid).Scan(&driveID); err != nil {
		return err
	}
	if _, err = tx.Exec("DELETE FROM files WHERE drive_id=?", driveID); err != nil {
		return err
	}
	statement, err := tx.Prepare(`INSERT INTO files(drive_id,path,filename,extension,size,mime_type,created_at,modified_at) VALUES(?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, file := range scan.Files {
		var created any
		if !file.CreatedAt.IsZero() {
			created = file.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		if _, err = statement.Exec(driveID, file.Path, file.Filename, file.Extension, file.Size, file.MIMEType, created, file.Modified.UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
