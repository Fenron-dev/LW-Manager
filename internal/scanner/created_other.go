//go:build !darwin && !windows

package scanner

import (
	"io/fs"
	"time"
)

func fileCreatedAt(fs.FileInfo) time.Time { return time.Time{} }
