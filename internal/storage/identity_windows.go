//go:build windows

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

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
	identity := Identity{UUID: fmt.Sprintf("%08X", serial), Label: windows.UTF16ToString(volume), FSType: windows.UTF16ToString(filesystem)}
	rootPath := windows.UTF16ToString(root)
	if len(rootPath) >= 2 && rootPath[1] == ':' {
		script := fmt.Sprintf("Get-Partition -DriveLetter '%s' | Get-Disk | Select-Object SerialNumber,FriendlyName,Manufacturer,BusType | ConvertTo-Json -Compress", strings.ToUpper(rootPath[:1]))
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if data, commandErr := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output(); commandErr == nil {
			var hardware struct {
				SerialNumber string
				FriendlyName string
				Manufacturer string
				BusType      string
			}
			if json.Unmarshal(data, &hardware) == nil {
				identity.Serial = strings.TrimSpace(hardware.SerialNumber)
				identity.Model = strings.TrimSpace(hardware.FriendlyName)
				identity.Vendor = strings.TrimSpace(hardware.Manufacturer)
				identity.DeviceType = strings.TrimSpace(hardware.BusType)
			}
		}
	}
	return identity, nil
}
