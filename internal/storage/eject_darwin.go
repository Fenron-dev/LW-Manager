//go:build darwin

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
	output, err := exec.CommandContext(ctx, "/usr/sbin/diskutil", "eject", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("diskutil: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
