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

func ListVolumes() ([]Volume, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	script := `$items = Get-Partition | Where-Object DriveLetter | ForEach-Object { $p=$_; $d=$p | Get-Disk; $v=Get-Volume -DriveLetter $p.DriveLetter; if ($d.BusType -in @('USB','SD','MMC') -or $v.DriveType -eq 'Removable') { $logical=Get-CimInstance Win32_LogicalDisk -Filter ("DeviceID='"+$p.DriveLetter+":'"); [PSCustomObject]@{Path=($p.DriveLetter+':\\'); Label=$v.FileSystemLabel; UUID=$logical.VolumeSerialNumber; FSType=$v.FileSystem; Total=[int64]$v.Size; Used=[int64]($v.Size-$v.SizeRemaining)} } }; @($items) | ConvertTo-Json -Compress`
	data, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script).Output()
	if err != nil {
		return nil, err
	}
	var items []struct {
		Path   string `json:"Path"`
		Label  string `json:"Label"`
		UUID   string `json:"UUID"`
		FSType string `json:"FSType"`
		Total  int64  `json:"Total"`
		Used   int64  `json:"Used"`
	}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	volumes := make([]Volume, 0, len(items))
	for _, item := range items {
		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = item.Path
		}
		volumes = append(volumes, Volume{Path: item.Path, Label: label, UUID: item.UUID, FSType: item.FSType, TotalSize: item.Total, UsedSize: item.Used, External: true})
	}
	return volumes, nil
}
