//go:build linux

package storage

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func Eject(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	source, err := exec.CommandContext(ctx, "findmnt", "-n", "-o", "SOURCE", "--target", path).Output()
	if err != nil {
		return fmt.Errorf("Blockgerät bestimmen: %w", err)
	}
	device := strings.TrimSpace(string(source))
	if !strings.HasPrefix(device, "/dev/") {
		return fmt.Errorf("kein auswerfbares Blockgerät: %s", device)
	}
	output, err := exec.CommandContext(ctx, "udisksctl", "unmount", "-b", device).CombinedOutput()
	if err != nil {
		return fmt.Errorf("udisksctl: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
