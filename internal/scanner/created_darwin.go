//go:build darwin

package scanner

import (
	"io/fs"
	"syscall"
	"time"
)

func fileCreatedAt(info fs.FileInfo) time.Time {
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
	}
	return time.Time{}
}
