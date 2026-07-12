//go:build linux

package storage

import (
	"encoding/json"
	"os/exec"
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
	return Identity{UUID: item.UUID, FSType: item.FSType, Model: item.Source}, nil
}
