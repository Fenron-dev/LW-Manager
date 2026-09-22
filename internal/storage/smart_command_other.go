//go:build !windows

package storage

import (
	"context"
	"os/exec"
)

func smartCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}
