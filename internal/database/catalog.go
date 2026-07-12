package database

import (
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"
	"path/filepath"
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
	return &Catalog{db: db}, nil
}
func (c *Catalog) Close() error { return c.db.Close() }
func (c *Catalog) Stats() (files, drives int64, err error) {
	if err = c.db.QueryRow("SELECT COUNT(*) FROM files").Scan(&files); err != nil {
		return
	}
	err = c.db.QueryRow("SELECT COUNT(*) FROM drives").Scan(&drives)
	return
}

func (c *Catalog) Search(query, extension string, limit, offset int) (SearchResult, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	pattern := "%" + escapeLike(strings.TrimSpace(query)) + "%"
	extension = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(extension)), ".")
	where := `(LOWER(f.filename) LIKE LOWER(?) ESCAPE '\' OR LOWER(f.path) LIKE LOWER(?) ESCAPE '\') AND (? = '' OR f.extension = ?)`
	args := []any{pattern, pattern, extension, extension}
	result := SearchResult{Extensions: make([]string, 0)}
	if err := c.db.QueryRow("SELECT COUNT(*) FROM files f WHERE "+where, args...).Scan(&result.Total); err != nil {
		return result, err
	}
	rows, err := c.db.Query(`SELECT f.id,f.filename,f.path,COALESCE(f.extension,''),COALESCE(f.mime_type,''),d.label,f.size,COALESCE(f.modified_at,'')
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
	if _, err = tx.Exec(`INSERT INTO drives(uuid,label,vault_path,updated_at) VALUES(?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(uuid) DO UPDATE SET label=excluded.label,vault_path=excluded.vault_path,updated_at=CURRENT_TIMESTAMP`, uuid, scan.Label, absolute); err != nil {
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
