//go:build windows

package scanner

import (
	"io/fs"
	"syscall"
	"time"
)

func fileCreatedAt(info fs.FileInfo) time.Time {
	if data, ok := info.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, data.CreationTime.Nanoseconds())
	}
	return time.Time{}
}
