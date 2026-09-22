//go:build windows

package storage

import (
	"context"
	"os/exec"
	"syscall"
)

func smartCommand(ctx context.Context, name string, args ...string) *exec.Cmd {
	command := exec.CommandContext(ctx, name, args...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return command
}
