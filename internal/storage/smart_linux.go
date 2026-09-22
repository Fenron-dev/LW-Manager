//go:build linux

package storage

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

func smartDevice(ctx context.Context, path string) (string, error) {
	output, err := exec.CommandContext(ctx, "findmnt", "-n", "-o", "SOURCE", "--target", path).Output()
	if err != nil {
		return "", err
	}
	source := strings.TrimSpace(string(output))
	if !strings.HasPrefix(source, "/dev/") {
		return "", fmt.Errorf("Quelle ist kein lokales Blockgerät")
	}
	if resolved, err := filepath.EvalSymlinks(source); err == nil {
		source = resolved
	}
	parent, err := exec.CommandContext(ctx, "lsblk", "-n", "-o", "PKNAME", source).Output()
	if err == nil && strings.TrimSpace(string(parent)) != "" {
		source = filepath.Join("/dev", strings.Fields(string(parent))[0])
	}
	return source, nil
}

func nativeHealth(context.Context, string) HealthReport {
	return unavailable("Für SMART unter Linux wird smartctl benötigt")
}
