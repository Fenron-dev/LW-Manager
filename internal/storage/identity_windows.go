//go:build windows

package storage

import (
	"fmt"
	"golang.org/x/sys/windows"
)

func Identify(path string) (Identity, error) {
	source, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return Identity{}, err
	}
	root := make([]uint16, windows.MAX_PATH)
	if err := windows.GetVolumePathName(source, &root[0], uint32(len(root))); err != nil {
		return Identity{}, err
	}
	volume := make([]uint16, 256)
	filesystem := make([]uint16, 64)
	var serial, maxComponent, flags uint32
	if err := windows.GetVolumeInformation(&root[0], &volume[0], uint32(len(volume)), &serial, &maxComponent, &flags, &filesystem[0], uint32(len(filesystem))); err != nil {
		return Identity{}, err
	}
	return Identity{UUID: fmt.Sprintf("%08X", serial), Label: windows.UTF16ToString(volume), FSType: windows.UTF16ToString(filesystem)}, nil
}
