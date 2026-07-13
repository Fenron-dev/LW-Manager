//go:build linux

package storage

import (
	"context"
	"encoding/json"
	"os/exec"
	"sort"
	"strings"
	"time"
)

func Identify(path string) (Identity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "findmnt", "-J", "-o", "UUID,FSTYPE,SOURCE,TARGET", "--target", path).Output()
	if err != nil {
		return Identity{}, err
	}
	var result struct {
		Filesystems []struct {
			UUID   string `json:"uuid"`
			FSType string `json:"fstype"`
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"filesystems"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return Identity{}, err
	}
	if len(result.Filesystems) == 0 {
		return Identity{}, nil
	}
	item := result.Filesystems[0]
	identity := Identity{UUID: item.UUID, FSType: item.FSType, Model: item.Source}
	blockData, blockErr := exec.CommandContext(ctx, "lsblk", "-J", "-o", "MODEL,SERIAL,VENDOR,TRAN", item.Source).Output()
	if blockErr == nil {
		var blocks struct {
			Devices []struct {
				Model     string `json:"model"`
				Serial    string `json:"serial"`
				Vendor    string `json:"vendor"`
				Transport string `json:"tran"`
			} `json:"blockdevices"`
		}
		if json.Unmarshal(blockData, &blocks) == nil && len(blocks.Devices) > 0 {
			block := blocks.Devices[0]
			identity.Model, identity.Serial, identity.Vendor = strings.TrimSpace(block.Model), strings.TrimSpace(block.Serial), strings.TrimSpace(block.Vendor)
			identity.DeviceType = strings.TrimSpace(block.Transport)
		}
	}
	return identity, nil
}

type linuxBlock struct {
	Path        string       `json:"path"`
	Label       string       `json:"label"`
	UUID        string       `json:"uuid"`
	FSType      string       `json:"fstype"`
	Mountpoints []string     `json:"mountpoints"`
	Transport   string       `json:"tran"`
	Removable   bool         `json:"rm"`
	Children    []linuxBlock `json:"children"`
}

func ListVolumes() ([]Volume, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	data, err := exec.CommandContext(ctx, "lsblk", "-J", "-o", "PATH,LABEL,UUID,FSTYPE,MOUNTPOINTS,TRAN,RM").Output()
	if err != nil {
		return nil, err
	}
	var result struct {
		BlockDevices []linuxBlock `json:"blockdevices"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	volumes := []Volume{}
	var collect func(linuxBlock, bool)
	collect = func(block linuxBlock, parentExternal bool) {
		external := parentExternal || block.Removable || block.Transport == "usb" || block.Transport == "mmc"
		if external {
			for _, mountpoint := range block.Mountpoints {
				if strings.TrimSpace(mountpoint) == "" || mountpoint == "/" {
					continue
				}
				total, used, _ := Usage(mountpoint)
				label := strings.TrimSpace(block.Label)
				if label == "" {
					label = strings.TrimSpace(block.Path)
				}
				volumes = append(volumes, Volume{Path: mountpoint, Label: label, UUID: block.UUID, FSType: block.FSType, TotalSize: total, UsedSize: used, External: true})
			}
		}
		for _, child := range block.Children {
			collect(child, external)
		}
	}
	for _, block := range result.BlockDevices {
		collect(block, false)
	}
	sort.Slice(volumes, func(i, j int) bool { return strings.ToLower(volumes[i].Label) < strings.ToLower(volumes[j].Label) })
	return volumes, nil
}
