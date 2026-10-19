package database

import (
	"database/sql"
	"time"

	"github.com/dennis/vaultapp/internal/scanner"
)

type ScanChanges struct {
	NewDrive   bool  `json:"newDrive"`
	Added      int   `json:"added"`
	Modified   int   `json:"modified"`
	Removed    int   `json:"removed"`
	Unchanged  int   `json:"unchanged"`
	AddedBytes int64 `json:"addedBytes"`
}

// Compare against the current catalogue before replacement, even when archives
// are disabled. Metadata changes are detected by path, size and modification time.
func (c *Catalog) ScanChanges(uuid, root string, files []scanner.File) (ScanChanges, error) {
	result := ScanChanges{}
	id, err := c.DriveIDForIdentity(uuid, root)
	if err == sql.ErrNoRows {
		result.NewDrive = true
	} else if err != nil {
		return result, err
	}
	type previousFile struct {
		size     int64
		modified string
	}
	previous := make(map[string]previousFile)
	if !result.NewDrive {
		rows, err := c.readDB.Query("SELECT path,size,COALESCE(modified_at,'') FROM files WHERE drive_id=?", id)
		if err != nil {
			return result, err
		}
		defer rows.Close()
		for rows.Next() {
			var path string
			var file previousFile
			if err := rows.Scan(&path, &file.size, &file.modified); err != nil {
				return result, err
			}
			previous[path] = file
		}
		if err := rows.Err(); err != nil {
			return result, err
		}
	}
	for _, file := range files {
		old, exists := previous[file.Path]
		if !exists {
			result.Added++
			result.AddedBytes += file.Size
		} else if old.size != file.Size || old.modified != file.Modified.UTC().Format(time.RFC3339Nano) {
			result.Modified++
		} else {
			result.Unchanged++
		}
		delete(previous, file.Path)
	}
	result.Removed = len(previous)
	return result, nil
}
