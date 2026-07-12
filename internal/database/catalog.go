package database

import (
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"fmt"
	"path/filepath"
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
