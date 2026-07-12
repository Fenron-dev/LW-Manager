//go:build windows

package storage

import (
	"golang.org/x/sys/windows"
)

func Usage(path string) (total, used int64, err error) {
	pointer, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var available, totalBytes, freeBytes uint64
	if err = windows.GetDiskFreeSpaceEx(pointer, &available, &totalBytes, &freeBytes); err != nil {
		return
	}
	return int64(totalBytes), int64(totalBytes - freeBytes), nil
}
