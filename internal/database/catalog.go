package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
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
	Path, Label   string
	Files         []scanner.File
	TotalSize     int64
	UsedSize      int64
	UUID          string
	FSType        string
	Vendor        string
	Model         string
	Serial        string
	DeviceType    string
	ScanProfileID string
	Archive       bool
	MaxSnapshots  int
}

type Drive struct {
	ID              int64    `json:"id"`
	Label           string   `json:"label"`
	DisplayName     string   `json:"displayName"`
	InventoryNumber string   `json:"inventoryNumber"`
	Path            string   `json:"path"`
	Manufacturer    string   `json:"manufacturer"`
	DeviceType      string   `json:"deviceType"`
	StorageLocation string   `json:"storageLocation"`
	Note            string   `json:"note"`
	ScanProfileID   string   `json:"scanProfileId"`
	Tags            []string `json:"tags"`
	UUID            string   `json:"uuid"`
	Serial          string   `json:"serial"`
	Vendor          string   `json:"vendor"`
	DetectedType    string   `json:"detectedType"`
	FSType          string   `json:"fsType"`
	Model           string   `json:"model"`
	TotalSize       int64    `json:"totalSize"`
	UsedSize        int64    `json:"usedSize"`
	FileCount       int64    `json:"fileCount"`
	UpdatedAt       string   `json:"updatedAt"`
	Online          bool     `json:"online"`
}

type FileResult struct {
	ID           int64    `json:"id"`
	Filename     string   `json:"filename"`
	Path         string   `json:"path"`
	Extension    string   `json:"extension"`
	MIMEType     string   `json:"mimeType"`
	Drive        string   `json:"drive"`
	Size         int64    `json:"size"`
	Width        int      `json:"width"`
	Height       int      `json:"height"`
	Metadata     string   `json:"metadata"`
	MatchSnippet string   `json:"matchSnippet"`
	Modified     string   `json:"modified"`
	AISummary    string   `json:"aiSummary"`
	AITags       []string `json:"aiTags"`
	AIProvider   string   `json:"aiProvider"`
	AIModel      string   `json:"aiModel"`
	AIAnalyzedAt string   `json:"aiAnalyzedAt"`
	AIInputBytes int64    `json:"aiInputBytes"`
	AITruncated  bool     `json:"aiTruncated"`
	AIImageBytes int64    `json:"aiImageBytes"`
	AIVision     bool     `json:"aiVision"`
	Tags         []string `json:"tags"`
}

type AIFileInput struct {
	ID          int64
	DriveID     int64
	Filename    string
	Path        string
	MIMEType    string
	Size        int64
	Width       int
	Height      int
	Metadata    string
	TextContent string
	Modified    string
}

type AIAnalysis struct {
	Summary        string
	Tags           []string
	Provider       string
	Model          string
	InputBytes     int64
	InputTruncated bool
	ImageBytes     int64
	Vision         bool
}

type SearchResult struct {
	Files      []FileResult `json:"files"`
	Extensions []string     `json:"extensions"`
	Total      int64        `json:"total"`
}

type ExportFile struct {
	Filename  string   `json:"filename"`
	Drive     string   `json:"drive"`
	Path      string   `json:"path"`
	Extension string   `json:"extension"`
	MIMEType  string   `json:"mimeType"`
	Size      int64    `json:"size"`
	Modified  string   `json:"modified"`
	Tags      []string `json:"tags"`
	AITags    []string `json:"aiTags"`
	AISummary string   `json:"aiSummary"`
}

type TagSummary struct {
	Name          string `json:"name"`
	DriveCount    int64  `json:"driveCount"`
	SnapshotCount int64  `json:"snapshotCount"`
	FileCount     int64  `json:"fileCount"`
	LibraryCount  int64  `json:"libraryCount"`
}

type HashCandidate struct {
	ID, Size         int64
	SourcePath, Hash string
}

type DuplicateFile struct {
	ID        int64  `json:"id"`
	Filename  string `json:"filename"`
	Path      string `json:"path"`
	Drive     string `json:"drive"`
	Preferred bool   `json:"preferred"`
}

type DuplicateGroup struct {
	Hash  string          `json:"hash"`
	Size  int64           `json:"size"`
	Files []DuplicateFile `json:"files"`
}

type QuarantineCandidate struct {
	ID, DriveID, Size, GroupCount        int64
	Root, Filename, Path, Modified, Hash string
	Preferred                            bool
}

type QuarantineItem struct {
	ID             int64  `json:"id"`
	Drive          string `json:"drive"`
	OriginalPath   string `json:"originalPath"`
	Filename       string `json:"filename"`
	Size           int64  `json:"size"`
	Modified       string `json:"modified"`
	Hash           string `json:"hash"`
	QuarantinePath string `json:"quarantinePath"`
	QuarantinedAt  string `json:"quarantinedAt"`
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
	Root, Relative, Path, MIMEType, Modified string
	Size                                     int64
}

type Snapshot struct {
	ID         int64    `json:"id"`
	CapturedAt string   `json:"capturedAt"`
	FileCount  int64    `json:"fileCount"`
	TotalBytes int64    `json:"totalBytes"`
	Protected  bool     `json:"protected"`
	Note       string   `json:"note"`
	Tags       []string `json:"tags"`
}

type ComparisonSnapshot struct {
	Snapshot
	DriveID   int64  `json:"driveId"`
	DriveName string `json:"driveName"`
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

// BackupTo creates a transactionally consistent standalone SQLite copy.
func (c *Catalog) BackupTo(destination string) error {
	if err := os.Remove(destination); err != nil && !os.IsNotExist(err) {
		return err
	}
	if _, err := c.db.Exec("VACUUM INTO ?", destination); err != nil {
		return fmt.Errorf("Datenbank-Sicherung: %w", err)
	}
	return nil
}

// Validate checks a restored catalog without modifying or migrating it.
func Validate(path string) (files, drives int64, err error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return 0, 0, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err = db.Exec("PRAGMA query_only=ON; PRAGMA busy_timeout=5000"); err != nil {
		return 0, 0, err
	}
	var integrity string
	if err = db.QueryRow("PRAGMA integrity_check").Scan(&integrity); err != nil {
		return 0, 0, err
	}
	if integrity != "ok" {
		return 0, 0, fmt.Errorf("SQLite-Integritätsprüfung: %s", integrity)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM files").Scan(&files); err != nil {
		return 0, 0, fmt.Errorf("Dateikatalog prüfen: %w", err)
	}
	if err = db.QueryRow("SELECT COUNT(*) FROM drives").Scan(&drives); err != nil {
		return 0, 0, fmt.Errorf("Datenträgerkatalog prüfen: %w", err)
	}
	return files, drives, nil
}
func (c *Catalog) Stats() (files, drives int64, err error) {
	if err = c.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&files); err != nil {
		return
	}
	err = c.db.QueryRow("SELECT COUNT(*) FROM drives").Scan(&drives)
	return
}

// StoredTextBytesExcludingDrive returns the UTF-8 byte size of index text that
// will remain when the identified drive is replaced by a new scan.
func (c *Catalog) StoredTextBytesExcludingDrive(uuid, root string) (int64, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return 0, err
	}
	key := strings.TrimSpace(uuid)
	if key == "" {
		key = fmt.Sprintf("path:%x", sha256.Sum256([]byte(filepath.Clean(absolute))))
	} else {
		key = "volume:" + strings.ToLower(key)
	}
	var bytes int64
	err = c.readDB.QueryRow(`SELECT COALESCE(SUM(LENGTH(CAST(COALESCE(f.text_content,'') AS BLOB))),0)
		FROM files f JOIN drives d ON d.id=f.drive_id WHERE d.uuid<>?`, key).Scan(&bytes)
	return bytes, err
}

func (c *Catalog) Search(query, extension, tag string, driveID int64, includeContent bool, limit, offset int) (SearchResult, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	where, args, query := searchFilter(query, extension, tag, driveID, includeContent)
	result := SearchResult{Extensions: make([]string, 0)}
	if err := c.db.QueryRow("SELECT COUNT(*) FROM files f WHERE "+where, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	selectArgs := []any{query, includeContent, query, query}
	rows, err := c.db.Query(`SELECT f.id,f.filename,f.path,COALESCE(f.extension,''),COALESCE(f.mime_type,''),COALESCE(NULLIF(d.display_name,''),d.label),f.size,COALESCE(f.width,0),COALESCE(f.height,0),COALESCE(f.metadata,''),
		CASE WHEN ?<>'' AND ?=1 AND instr(LOWER(COALESCE(f.text_content,'')),LOWER(?))>0 THEN substr(f.text_content,MAX(instr(LOWER(f.text_content),LOWER(?))-80,1),240) ELSE '' END,
		COALESCE(f.modified_at,'') FROM files f JOIN drives d ON d.id=f.drive_id WHERE `+where+` ORDER BY f.filename COLLATE NOCASE,f.path LIMIT ? OFFSET ?`, append(append(selectArgs, args...), limit, offset)...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	result.Files = make([]FileResult, 0, limit)
	for rows.Next() {
		var file FileResult
		if err := rows.Scan(&file.ID, &file.Filename, &file.Path, &file.Extension, &file.MIMEType, &file.Drive, &file.Size, &file.Width, &file.Height, &file.Metadata, &file.MatchSnippet, &file.Modified); err != nil {
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

func searchFilter(query, extension, tag string, driveID int64, includeContent bool) (string, []any, string) {
	query = strings.TrimSpace(query)
	pattern := "%" + escapeLike(query) + "%"
	extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
	tag = strings.TrimSpace(tag)
	where := `(LOWER(f.filename) LIKE LOWER(?) ESCAPE '\' OR LOWER(f.path) LIKE LOWER(?) ESCAPE '\' OR (? <> '' AND ? = 1 AND LOWER(COALESCE(f.text_content,'')) LIKE LOWER(?) ESCAPE '\') OR
		(? <> '' AND ? = 1 AND EXISTS(SELECT 1 FROM file_ai_analyses ai WHERE ai.drive_id=f.drive_id AND ai.path=f.path AND ai.source_size=f.size AND ai.source_modified=COALESCE(f.modified_at,'') AND (LOWER(ai.summary) LIKE LOWER(?) ESCAPE '\' OR LOWER(ai.tags) LIKE LOWER(?) ESCAPE '\'))))
		AND (? = '' OR f.extension = ?) AND (? = 0 OR f.drive_id = ?) AND (? = '' OR
		EXISTS(SELECT 1 FROM drive_tags dt JOIN tags t ON t.id=dt.tag_id WHERE dt.drive_id=f.drive_id AND t.name=? COLLATE NOCASE) OR
		EXISTS(SELECT 1 FROM file_tags ft JOIN tags t ON t.id=ft.tag_id WHERE ft.drive_id=f.drive_id AND ft.path=f.path AND t.name=? COLLATE NOCASE))`
	args := []any{pattern, pattern, query, includeContent, pattern, query, includeContent, pattern, pattern, extension, extension, driveID, driveID, tag, tag, tag}
	return where, args, query
}

func (c *Catalog) ExportFiles(query, extension, tag string, driveID int64, includeContent bool, handle func(ExportFile) error) error {
	where, args, _ := searchFilter(query, extension, tag, driveID, includeContent)
	rows, err := c.readDB.Query(`SELECT f.filename,COALESCE(NULLIF(d.display_name,''),d.label),f.path,COALESCE(f.extension,''),COALESCE(f.mime_type,''),f.size,COALESCE(f.modified_at,''),
		COALESCE((SELECT GROUP_CONCAT(name, char(31)) FROM (SELECT t.name name FROM tags t JOIN file_tags ft ON ft.tag_id=t.id WHERE ft.drive_id=f.drive_id AND ft.path=f.path ORDER BY t.name COLLATE NOCASE)),''),
		COALESCE(a.summary,''),COALESCE(a.tags,'[]')
		FROM files f JOIN drives d ON d.id=f.drive_id LEFT JOIN file_ai_analyses a ON a.drive_id=f.drive_id AND a.path=f.path AND a.source_size=f.size AND a.source_modified=COALESCE(f.modified_at,'')
		WHERE `+where+` ORDER BY f.filename COLLATE NOCASE,f.path`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var file ExportFile
		var tags, aiTags string
		if err := rows.Scan(&file.Filename, &file.Drive, &file.Path, &file.Extension, &file.MIMEType, &file.Size, &file.Modified, &tags, &file.AISummary, &aiTags); err != nil {
			return err
		}
		file.Tags = splitStoredTags(tags)
		if json.Unmarshal([]byte(aiTags), &file.AITags) != nil {
			file.AITags = []string{}
		}
		if err := handle(file); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (c *Catalog) Tags() ([]TagSummary, error) {
	rows, err := c.readDB.Query(`SELECT t.name,
		(SELECT COUNT(*) FROM drive_tags dt WHERE dt.tag_id=t.id),
		(SELECT COUNT(*) FROM snapshot_tags st WHERE st.tag_id=t.id),
		(SELECT COUNT(*) FROM file_tags ft WHERE ft.tag_id=t.id),
		(SELECT COUNT(*) FROM files f WHERE EXISTS(SELECT 1 FROM drive_tags dt WHERE dt.drive_id=f.drive_id AND dt.tag_id=t.id) OR EXISTS(SELECT 1 FROM file_tags ft WHERE ft.drive_id=f.drive_id AND ft.path=f.path AND ft.tag_id=t.id))
		FROM tags t ORDER BY t.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]TagSummary, 0)
	for rows.Next() {
		var tag TagSummary
		if err := rows.Scan(&tag.Name, &tag.DriveCount, &tag.SnapshotCount, &tag.FileCount, &tag.LibraryCount); err != nil {
			return nil, err
		}
		result = append(result, tag)
	}
	return result, rows.Err()
}

// RenameTag changes a tag globally. If the destination already exists, all
// assignments are merged into it and the old tag is removed.
func (c *Catalog) RenameTag(currentName, newName string) error {
	currentName = strings.TrimSpace(currentName)
	normalized, err := normalizeTags([]string{newName})
	if err != nil {
		return err
	}
	if currentName == "" || len(normalized) != 1 {
		return fmt.Errorf("Tagname darf nicht leer sein")
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var sourceID int64
	if err := tx.QueryRow(`SELECT id FROM tags WHERE name=? COLLATE NOCASE`, currentName).Scan(&sourceID); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("Tag %q wurde nicht gefunden", currentName)
		}
		return err
	}
	var targetID int64
	err = tx.QueryRow(`SELECT id FROM tags WHERE name=? COLLATE NOCASE`, normalized[0]).Scan(&targetID)
	if err == sql.ErrNoRows {
		_, err = tx.Exec(`UPDATE tags SET name=? WHERE id=?`, normalized[0], sourceID)
	} else if err == nil && targetID == sourceID {
		_, err = tx.Exec(`UPDATE tags SET name=? WHERE id=?`, normalized[0], sourceID)
	} else if err == nil {
		if _, err = tx.Exec(`INSERT OR IGNORE INTO drive_tags(drive_id,tag_id) SELECT drive_id,? FROM drive_tags WHERE tag_id=?`, targetID, sourceID); err == nil {
			_, err = tx.Exec(`INSERT OR IGNORE INTO snapshot_tags(snapshot_id,tag_id) SELECT snapshot_id,? FROM snapshot_tags WHERE tag_id=?`, targetID, sourceID)
		}
		if err == nil {
			_, err = tx.Exec(`INSERT OR IGNORE INTO file_tags(drive_id,path,tag_id,source) SELECT drive_id,path,?,source FROM file_tags WHERE tag_id=?`, targetID, sourceID)
		}
		if err == nil {
			_, err = tx.Exec(`DELETE FROM tags WHERE id=?`, sourceID)
		}
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteTag removes a tag and all of its drive, snapshot and file assignments.
func (c *Catalog) DeleteTag(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("Tagname darf nicht leer sein")
	}
	result, err := c.db.Exec(`DELETE FROM tags WHERE name=? COLLATE NOCASE`, name)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("Tag %q wurde nicht gefunden", name)
	}
	return nil
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
	rows, err := c.readDB.Query(`SELECT f.content_hash,f.size,f.id,f.filename,f.path,COALESCE(NULLIF(d.display_name,''),d.label),
		CASE WHEN p.content_hash IS NULL THEN 0 ELSE 1 END
		FROM files f JOIN drives d ON d.id=f.drive_id
		LEFT JOIN duplicate_preferences p ON p.content_hash=f.content_hash AND p.drive_id=f.drive_id AND p.path=f.path
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
		var preferred bool
		if err := rows.Scan(&hash, &size, &id, &filename, &path, &drive, &preferred); err != nil {
			return nil, err
		}
		if len(groups) == 0 || groups[len(groups)-1].Hash != hash {
			groups = append(groups, DuplicateGroup{Hash: hash, Size: size, Files: make([]DuplicateFile, 0, 2)})
		}
		group := &groups[len(groups)-1]
		group.Files = append(group.Files, DuplicateFile{ID: id, Filename: filename, Path: path, Drive: drive, Preferred: preferred})
	}
	return groups, rows.Err()
}

// SetDuplicatePreference stores the catalog location that should be retained for a hash.
// The location is saved by drive and relative path because file IDs may change on a rescan.
func (c *Catalog) SetDuplicatePreference(hash string, fileID int64) error {
	hash = strings.ToLower(strings.TrimSpace(hash))
	if len(hash) != sha256.Size*2 {
		return fmt.Errorf("ungültige SHA-256-Prüfsumme")
	}
	var driveID int64
	var path, actualHash string
	if err := c.readDB.QueryRow(`SELECT drive_id,path,COALESCE(content_hash,'') FROM files WHERE id=?`, fileID).Scan(&driveID, &path, &actualHash); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("Datei wurde im aktuellen Katalog nicht gefunden")
		}
		return err
	}
	if !strings.EqualFold(actualHash, hash) {
		return fmt.Errorf("Datei gehört nicht mehr zu dieser Duplikatgruppe; bitte erneut prüfen")
	}
	_, err := c.db.Exec(`INSERT INTO duplicate_preferences(content_hash,drive_id,path,updated_at) VALUES(?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(content_hash) DO UPDATE SET drive_id=excluded.drive_id,path=excluded.path,updated_at=CURRENT_TIMESTAMP`, hash, driveID, path)
	return err
}

func (c *Catalog) QuarantineCandidate(fileID int64, expectedHash string) (QuarantineCandidate, error) {
	var candidate QuarantineCandidate
	expectedHash = strings.ToLower(strings.TrimSpace(expectedHash))
	err := c.readDB.QueryRow(`SELECT f.id,f.drive_id,f.size,d.vault_path,f.filename,f.path,COALESCE(f.modified_at,''),COALESCE(f.content_hash,''),
		(SELECT COUNT(*) FROM files sibling WHERE sibling.content_hash=f.content_hash),
		CASE WHEN p.content_hash IS NULL THEN 0 ELSE 1 END
		FROM files f JOIN drives d ON d.id=f.drive_id
		LEFT JOIN duplicate_preferences p ON p.content_hash=f.content_hash AND p.drive_id=f.drive_id AND p.path=f.path
		WHERE f.id=?`, fileID).Scan(&candidate.ID, &candidate.DriveID, &candidate.Size, &candidate.Root, &candidate.Filename, &candidate.Path,
		&candidate.Modified, &candidate.Hash, &candidate.GroupCount, &candidate.Preferred)
	if err != nil {
		if err == sql.ErrNoRows {
			return candidate, fmt.Errorf("Duplikatkandidat wurde im aktuellen Katalog nicht gefunden")
		}
		return candidate, err
	}
	if len(expectedHash) != sha256.Size*2 || !strings.EqualFold(candidate.Hash, expectedHash) || candidate.GroupCount < 2 {
		return candidate, fmt.Errorf("Duplikatgruppe ist nicht mehr aktuell; bitte erneut prüfen")
	}
	return candidate, nil
}

func (c *Catalog) RecordQuarantine(candidate QuarantineCandidate, quarantinePath string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`DELETE FROM files WHERE id=? AND drive_id=? AND path=? AND size=? AND COALESCE(modified_at,'')=? AND content_hash=?`,
		candidate.ID, candidate.DriveID, candidate.Path, candidate.Size, candidate.Modified, candidate.Hash)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("Katalogeintrag wurde zwischen Prüfung und Quarantäne geändert")
	}
	if _, err := tx.Exec(`INSERT INTO quarantined_files(drive_id,original_path,filename,size,modified_at,content_hash,quarantine_path)
		VALUES(?,?,?,?,?,?,?)`, candidate.DriveID, candidate.Path, candidate.Filename, candidate.Size, candidate.Modified, candidate.Hash, quarantinePath); err != nil {
		return err
	}
	return tx.Commit()
}

func (c *Catalog) QuarantineItems() ([]QuarantineItem, error) {
	rows, err := c.readDB.Query(`SELECT q.id,COALESCE(NULLIF(d.display_name,''),d.label),q.original_path,q.filename,q.size,q.modified_at,q.content_hash,q.quarantine_path,q.quarantined_at
		FROM quarantined_files q JOIN drives d ON d.id=q.drive_id ORDER BY q.quarantined_at DESC,q.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]QuarantineItem, 0)
	for rows.Next() {
		var item QuarantineItem
		if err := rows.Scan(&item.ID, &item.Drive, &item.OriginalPath, &item.Filename, &item.Size, &item.Modified, &item.Hash, &item.QuarantinePath, &item.QuarantinedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (c *Catalog) QuarantineUsage() (int64, error) {
	var total int64
	err := c.readDB.QueryRow(`SELECT COALESCE(SUM(size),0) FROM quarantined_files`).Scan(&total)
	return total, err
}

func (c *Catalog) QuarantineItem(id int64) (QuarantineItem, string, error) {
	var item QuarantineItem
	var root string
	err := c.readDB.QueryRow(`SELECT q.id,COALESCE(NULLIF(d.display_name,''),d.label),q.original_path,q.filename,q.size,q.modified_at,q.content_hash,q.quarantine_path,q.quarantined_at,d.vault_path
		FROM quarantined_files q JOIN drives d ON d.id=q.drive_id WHERE q.id=?`, id).
		Scan(&item.ID, &item.Drive, &item.OriginalPath, &item.Filename, &item.Size, &item.Modified, &item.Hash, &item.QuarantinePath, &item.QuarantinedAt, &root)
	if err == sql.ErrNoRows {
		err = fmt.Errorf("Quarantäne-Eintrag wurde nicht gefunden")
	}
	return item, root, err
}

func (c *Catalog) DeleteQuarantineRecord(id int64) error {
	result, err := c.db.Exec(`DELETE FROM quarantined_files WHERE id=?`, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed != 1 {
		return fmt.Errorf("Quarantäne-Eintrag wurde nicht gefunden")
	}
	return nil
}

func (c *Catalog) Drives() ([]Drive, error) {
	rows, err := c.db.Query(`SELECT d.id,d.label,COALESCE(d.display_name,''),COALESCE(d.inventory_number,''),COALESCE(d.vault_path,''),
		COALESCE(d.manufacturer,''),COALESCE(d.device_type,''),COALESCE(d.storage_location,''),COALESCE(d.note,''),COALESCE(d.scan_profile_id,''),
		COALESCE((SELECT GROUP_CONCAT(name, char(31)) FROM (SELECT t.name name FROM tags t JOIN drive_tags dt ON dt.tag_id=t.id WHERE dt.drive_id=d.id ORDER BY t.name COLLATE NOCASE)),''),
		d.uuid,COALESCE(d.serial,''),COALESCE(d.vendor,''),COALESCE(d.detected_type,''),COALESCE(d.fs_type,''),COALESCE(d.model,''),COALESCE(d.total_size,0),COALESCE(d.used_size,0),COUNT(f.id),d.updated_at
		FROM drives d LEFT JOIN files f ON f.drive_id=d.id GROUP BY d.id ORDER BY COALESCE(NULLIF(d.display_name,''),d.label) COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	drives := make([]Drive, 0)
	for rows.Next() {
		var drive Drive
		var tags string
		if err := rows.Scan(&drive.ID, &drive.Label, &drive.DisplayName, &drive.InventoryNumber, &drive.Path, &drive.Manufacturer, &drive.DeviceType, &drive.StorageLocation, &drive.Note, &drive.ScanProfileID, &tags, &drive.UUID, &drive.Serial, &drive.Vendor, &drive.DetectedType, &drive.FSType, &drive.Model, &drive.TotalSize, &drive.UsedSize, &drive.FileCount, &drive.UpdatedAt); err != nil {
			return nil, err
		}
		drive.Tags = splitStoredTags(tags)
		drives = append(drives, drive)
	}
	return drives, rows.Err()
}

func (c *Catalog) UpdateDrive(id int64, displayName, inventoryNumber, manufacturer, deviceType, storageLocation, note, scanProfileID string, tags []string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE drives SET display_name=?,inventory_number=?,manufacturer=?,device_type=?,storage_location=?,note=?,scan_profile_id=?,updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		strings.TrimSpace(displayName), strings.TrimSpace(inventoryNumber), strings.TrimSpace(manufacturer), strings.TrimSpace(deviceType), strings.TrimSpace(storageLocation), strings.TrimSpace(note), strings.TrimSpace(scanProfileID), id)
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
	if err := replaceTags(tx, "drive_tags", "drive_id", id, tags); err != nil {
		return err
	}
	return tx.Commit()
}

func (c *Catalog) ScanProfileID(uuid, path string) (string, error) {
	identity := ""
	if strings.TrimSpace(uuid) != "" {
		identity = "volume:" + strings.TrimPrefix(strings.ToLower(strings.TrimSpace(uuid)), "volume:")
	}
	if identity != "" {
		var profile string
		err := c.readDB.QueryRow(`SELECT COALESCE(scan_profile_id,'') FROM drives WHERE uuid=?`, identity).Scan(&profile)
		if err == nil {
			return profile, nil
		}
		if err != sql.ErrNoRows {
			return "", err
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	var profile string
	err = c.readDB.QueryRow(`SELECT COALESCE(scan_profile_id,'') FROM drives WHERE vault_path=?`, absolute).Scan(&profile)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return profile, err
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
	source.Root = root
	source.Relative = filepath.ToSlash(relative)
	source.Path = filepath.Join(root, filepath.FromSlash(relative))
	inside, err := filepath.Rel(root, source.Path)
	if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return SourceFile{}, fmt.Errorf("Dateipfad verlässt den Datenträger")
	}
	return source, nil
}

func (c *Catalog) FileDetails(id int64) (FileResult, error) {
	var file FileResult
	var aiTags, tags string
	err := c.readDB.QueryRow(`SELECT f.id,f.filename,f.path,COALESCE(f.extension,''),COALESCE(f.mime_type,''),COALESCE(NULLIF(d.display_name,''),d.label),f.size,COALESCE(f.width,0),COALESCE(f.height,0),COALESCE(f.metadata,''),COALESCE(f.modified_at,''),
		COALESCE(a.summary,''),COALESCE(a.tags,'[]'),COALESCE(a.provider,''),COALESCE(a.model,''),COALESCE(a.analyzed_at,''),COALESCE(a.input_bytes,0),COALESCE(a.input_truncated,0),COALESCE(a.image_bytes,0),COALESCE(a.vision,0),
		COALESCE((SELECT GROUP_CONCAT(name, char(31)) FROM (SELECT t.name name FROM tags t JOIN file_tags ft ON ft.tag_id=t.id WHERE ft.drive_id=f.drive_id AND ft.path=f.path ORDER BY t.name COLLATE NOCASE)),'')
		FROM files f JOIN drives d ON d.id=f.drive_id LEFT JOIN file_ai_analyses a ON a.drive_id=f.drive_id AND a.path=f.path AND a.source_size=f.size AND a.source_modified=COALESCE(f.modified_at,'') WHERE f.id=?`, id).
		Scan(&file.ID, &file.Filename, &file.Path, &file.Extension, &file.MIMEType, &file.Drive, &file.Size, &file.Width, &file.Height, &file.Metadata, &file.Modified, &file.AISummary, &aiTags, &file.AIProvider, &file.AIModel, &file.AIAnalyzedAt, &file.AIInputBytes, &file.AITruncated, &file.AIImageBytes, &file.AIVision, &tags)
	if err == nil && json.Unmarshal([]byte(aiTags), &file.AITags) != nil {
		file.AITags = []string{}
	}
	file.Tags = splitStoredTags(tags)
	return file, err
}

func (c *Catalog) UpdateFileTags(id int64, tags []string) error {
	normalized, err := normalizeTags(tags)
	if err != nil {
		return err
	}
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var driveID int64
	var path string
	if err := tx.QueryRow(`SELECT drive_id,path FROM files WHERE id=?`, id).Scan(&driveID, &path); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("Datei wurde nicht gefunden")
		}
		return err
	}
	if _, err := tx.Exec(`DELETE FROM file_tags WHERE drive_id=? AND path=?`, driveID, path); err != nil {
		return err
	}
	for _, name := range normalized {
		if _, err := tx.Exec(`INSERT INTO tags(name) VALUES(?) ON CONFLICT(name) DO NOTHING`, name); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO file_tags(drive_id,path,tag_id,source) SELECT ?,?,id,'manual' FROM tags WHERE name=? COLLATE NOCASE`, driveID, path, name); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (c *Catalog) AIFileInput(id int64) (AIFileInput, error) {
	var input AIFileInput
	err := c.readDB.QueryRow(`SELECT f.id,f.drive_id,f.filename,f.path,COALESCE(f.mime_type,''),f.size,COALESCE(f.width,0),COALESCE(f.height,0),COALESCE(f.metadata,''),COALESCE(f.text_content,''),COALESCE(f.modified_at,'') FROM files f WHERE f.id=?`, id).
		Scan(&input.ID, &input.DriveID, &input.Filename, &input.Path, &input.MIMEType, &input.Size, &input.Width, &input.Height, &input.Metadata, &input.TextContent, &input.Modified)
	return input, err
}

func (c *Catalog) SaveAIAnalysis(input AIFileInput, analysis AIAnalysis) error {
	tags, err := normalizeTags(analysis.Tags)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(tags)
	if err != nil {
		return err
	}
	result, err := c.db.Exec(`INSERT INTO file_ai_analyses(drive_id,path,source_size,source_modified,summary,tags,provider,model,input_bytes,input_truncated,image_bytes,vision,analyzed_at)
		SELECT drive_id,path,size,COALESCE(modified_at,''),?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP FROM files WHERE id=? AND drive_id=? AND path=? AND size=? AND COALESCE(modified_at,'')=?
		ON CONFLICT(drive_id,path) DO UPDATE SET source_size=excluded.source_size,source_modified=excluded.source_modified,summary=excluded.summary,tags=excluded.tags,provider=excluded.provider,model=excluded.model,input_bytes=excluded.input_bytes,input_truncated=excluded.input_truncated,image_bytes=excluded.image_bytes,vision=excluded.vision,analyzed_at=CURRENT_TIMESTAMP`,
		strings.TrimSpace(analysis.Summary), string(encoded), strings.TrimSpace(analysis.Provider), strings.TrimSpace(analysis.Model), analysis.InputBytes, analysis.InputTruncated, analysis.ImageBytes, analysis.Vision, input.ID, input.DriveID, input.Path, input.Size, input.Modified)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("Datei wurde während der KI-Analyse geändert oder entfernt")
	}
	return nil
}

func (c *Catalog) Snapshots(driveID int64) ([]Snapshot, error) {
	rows, err := c.db.Query(`SELECT s.id,s.captured_at,s.file_count,s.total_bytes,s.protected,COALESCE(s.note,''),
		COALESCE((SELECT GROUP_CONCAT(name, char(31)) FROM (SELECT t.name name FROM tags t JOIN snapshot_tags st ON st.tag_id=t.id WHERE st.snapshot_id=s.id ORDER BY t.name COLLATE NOCASE)),'')
		FROM scan_snapshots s WHERE s.drive_id=? ORDER BY s.captured_at DESC,s.id DESC`, driveID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Snapshot, 0)
	for rows.Next() {
		var snapshot Snapshot
		var tags string
		if err := rows.Scan(&snapshot.ID, &snapshot.CapturedAt, &snapshot.FileCount, &snapshot.TotalBytes, &snapshot.Protected, &snapshot.Note, &tags); err != nil {
			return nil, err
		}
		snapshot.Tags = splitStoredTags(tags)
		result = append(result, snapshot)
	}
	return result, rows.Err()
}

func (c *Catalog) DeleteSnapshot(id int64) error {
	result, err := c.db.Exec("DELETE FROM scan_snapshots WHERE id=? AND protected=0", id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		var protected bool
		if err := c.db.QueryRow("SELECT protected FROM scan_snapshots WHERE id=?", id).Scan(&protected); err == nil && protected {
			return fmt.Errorf("Archivstand %d ist geschützt und kann nicht gelöscht werden", id)
		}
		return fmt.Errorf("Archivstand %d wurde nicht gefunden", id)
	}
	return nil
}

func (c *Catalog) UpdateSnapshot(id int64, protected bool, note string, tags []string) error {
	tx, err := c.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.Exec(`UPDATE scan_snapshots SET protected=?,note=? WHERE id=?`, protected, strings.TrimSpace(note), id)
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
	if err := replaceTags(tx, "snapshot_tags", "snapshot_id", id, tags); err != nil {
		return err
	}
	return tx.Commit()
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

func (c *Catalog) ComparisonSnapshot(snapshotID int64) (ComparisonSnapshot, error) {
	var result ComparisonSnapshot
	var tags string
	err := c.readDB.QueryRow(`SELECT s.id,s.captured_at,s.file_count,s.total_bytes,s.protected,COALESCE(s.note,''),s.drive_id,
		COALESCE(NULLIF(d.display_name,''),d.label),
		COALESCE((SELECT GROUP_CONCAT(name, char(31)) FROM (SELECT t.name name FROM tags t JOIN snapshot_tags st ON st.tag_id=t.id WHERE st.snapshot_id=s.id ORDER BY t.name COLLATE NOCASE)),'')
		FROM scan_snapshots s JOIN drives d ON d.id=s.drive_id WHERE s.id=?`, snapshotID).Scan(
		&result.ID, &result.CapturedAt, &result.FileCount, &result.TotalBytes, &result.Protected, &result.Note,
		&result.DriveID, &result.DriveName, &tags)
	if err != nil {
		return result, err
	}
	result.Tags = splitStoredTags(tags)
	return result, nil
}

func (c *Catalog) ExportComparison(ctx context.Context, snapshotID int64, status, query string, handle func(ComparisonEntry) error) (int, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	if status != "" && status != "added" && status != "removed" && status != "modified" && status != "unchanged" {
		return 0, fmt.Errorf("ungültiger Vergleichsfilter")
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
	rows, err := c.readDB.QueryContext(ctx, "SELECT path,status,current_name,current_size,current_modified,archive_name,archive_size,archive_modified FROM ("+comparison+") WHERE (?='' OR status=?) AND LOWER(path) LIKE LOWER(?) ESCAPE '\\' ORDER BY path COLLATE NOCASE", snapshotID, snapshotID, status, status, pattern)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var entry ComparisonEntry
		if err := rows.Scan(&entry.Path, &entry.Status, &entry.CurrentName, &entry.CurrentSize, &entry.CurrentModified, &entry.ArchiveName, &entry.ArchiveSize, &entry.ArchiveModified); err != nil {
			return count, err
		}
		if err := handle(entry); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
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
		"detected_type": "TEXT", "note": "TEXT", "scan_profile_id": "TEXT NOT NULL DEFAULT ''",
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
	fileColumns := map[string]string{"width": "INTEGER NOT NULL DEFAULT 0", "height": "INTEGER NOT NULL DEFAULT 0", "text_content": "TEXT"}
	for name, definition := range fileColumns {
		var count int
		if err := c.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('files') WHERE name=?", name).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := c.db.Exec("ALTER TABLE files ADD COLUMN " + name + " " + definition); err != nil {
				return err
			}
		}
	}
	snapshotColumns := map[string]string{"protected": "INTEGER NOT NULL DEFAULT 0", "note": "TEXT"}
	for name, definition := range snapshotColumns {
		var count int
		if err := c.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('scan_snapshots') WHERE name=?", name).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := c.db.Exec("ALTER TABLE scan_snapshots ADD COLUMN " + name + " " + definition); err != nil {
				return err
			}
		}
	}
	aiColumns := map[string]string{"image_bytes": "INTEGER NOT NULL DEFAULT 0", "vision": "INTEGER NOT NULL DEFAULT 0"}
	for name, definition := range aiColumns {
		var count int
		if err := c.db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('file_ai_analyses') WHERE name=?", name).Scan(&count); err != nil {
			return err
		}
		if count == 0 {
			if _, err := c.db.Exec("ALTER TABLE file_ai_analyses ADD COLUMN " + name + " " + definition); err != nil {
				return err
			}
		}
	}
	return nil
}

func normalizeTags(tags []string) ([]string, error) {
	seen := make(map[string]bool)
	result := make([]string, 0, len(tags))
	for _, raw := range tags {
		name := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
		if name == "" {
			continue
		}
		if len([]rune(name)) > 50 {
			return nil, fmt.Errorf("Tag %q ist länger als 50 Zeichen", name)
		}
		key := strings.ToLower(name)
		if !seen[key] {
			seen[key] = true
			result = append(result, name)
		}
	}
	if len(result) > 30 {
		return nil, fmt.Errorf("höchstens 30 Tags sind erlaubt")
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i]) < strings.ToLower(result[j]) })
	return result, nil
}

func replaceTags(tx *sql.Tx, relation, ownerColumn string, ownerID int64, tags []string) error {
	normalized, err := normalizeTags(tags)
	if err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM "+relation+" WHERE "+ownerColumn+"=?", ownerID); err != nil {
		return err
	}
	for _, name := range normalized {
		if _, err := tx.Exec(`INSERT INTO tags(name) VALUES(?) ON CONFLICT(name) DO NOTHING`, name); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO "+relation+"("+ownerColumn+",tag_id) SELECT ?,id FROM tags WHERE name=? COLLATE NOCASE", ownerID, name); err != nil {
			return err
		}
	}
	_, err = tx.Exec(`DELETE FROM tags WHERE id NOT IN (SELECT tag_id FROM drive_tags UNION SELECT tag_id FROM snapshot_tags)`)
	return err
}

func splitStoredTags(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, string(rune(31)))
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
	type previousHash struct {
		size     int64
		modified string
		hash     string
	}
	previousHashes := make(map[string]previousHash)
	hashRows, err := tx.Query(`SELECT path,size,COALESCE(modified_at,''),content_hash FROM files WHERE drive_id=? AND content_hash IS NOT NULL AND content_hash<>''`, driveID)
	if err != nil {
		return err
	}
	for hashRows.Next() {
		var path string
		var value previousHash
		if err := hashRows.Scan(&path, &value.size, &value.modified, &value.hash); err != nil {
			_ = hashRows.Close()
			return err
		}
		previousHashes[path] = value
	}
	if err := hashRows.Err(); err != nil {
		_ = hashRows.Close()
		return err
	}
	if err := hashRows.Close(); err != nil {
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
			if _, err = tx.Exec(`DELETE FROM scan_snapshots WHERE drive_id=? AND protected=0 AND id NOT IN
				(SELECT id FROM scan_snapshots WHERE drive_id=? AND protected=0 ORDER BY id DESC LIMIT ?)`, driveID, driveID, scan.MaxSnapshots); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec("DELETE FROM files WHERE drive_id=?", driveID); err != nil {
		return err
	}
	statement, err := tx.Prepare(`INSERT INTO files(drive_id,path,filename,extension,size,width,height,mime_type,metadata,text_content,created_at,modified_at,content_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, file := range scan.Files {
		var created any
		if !file.CreatedAt.IsZero() {
			created = file.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		modified := file.Modified.UTC().Format(time.RFC3339Nano)
		contentHash := ""
		if previous, exists := previousHashes[file.Path]; exists && previous.size == file.Size && previous.modified == modified {
			contentHash = previous.hash
		}
		if _, err = statement.Exec(driveID, file.Path, file.Filename, file.Extension, file.Size, file.Width, file.Height, file.MIMEType, file.Metadata, file.TextContent, created, modified, contentHash); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`DELETE FROM file_ai_analyses WHERE drive_id=? AND NOT EXISTS (
		SELECT 1 FROM files f WHERE f.drive_id=file_ai_analyses.drive_id AND f.path=file_ai_analyses.path AND f.size=file_ai_analyses.source_size AND COALESCE(f.modified_at,'')=file_ai_analyses.source_modified
	)`, driveID); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM file_tags WHERE drive_id=? AND NOT EXISTS (
		SELECT 1 FROM files f WHERE f.drive_id=file_tags.drive_id AND f.path=file_tags.path
	)`, driveID); err != nil {
		return err
	}
	return tx.Commit()
}
