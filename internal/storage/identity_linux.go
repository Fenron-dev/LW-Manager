//go:build linux

package storage

import (
	"encoding/json"
	"os/exec"
	"strings"
)

func Identify(path string) (Identity, error) {
	data, err := exec.Command("findmnt", "-J", "-o", "UUID,FSTYPE,SOURCE,TARGET", "--target", path).Output()
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
	blockData, blockErr := exec.Command("lsblk", "-J", "-o", "MODEL,SERIAL,VENDOR,TRAN", item.Source).Output()
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
