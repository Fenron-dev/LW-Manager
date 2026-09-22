//go:build darwin

package storage

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func darwinDiskInfo(ctx context.Context, path string) (map[string]string, error) {
	data, err := exec.CommandContext(ctx, "/usr/sbin/diskutil", "info", "-plist", path).Output()
	if err != nil {
		return nil, err
	}
	return darwinPlistValues(data), nil
}

func smartDevice(ctx context.Context, path string) (string, error) {
	values, err := darwinDiskInfo(ctx, path)
	if err != nil {
		return "", err
	}
	device := values["ParentWholeDisk"]
	if device == "" {
		device = values["DeviceIdentifier"]
	}
	if !strings.HasPrefix(device, "disk") || strings.ContainsAny(device, "/\\ ") {
		return "", fmt.Errorf("kein physisches Laufwerk gefunden")
	}
	return "/dev/" + device, nil
}

func nativeHealth(ctx context.Context, path string) HealthReport {
	values, err := darwinDiskInfo(ctx, path)
	if err != nil {
		return unavailable("SMART-Status konnte nicht gelesen werden")
	}
	device := values["ParentWholeDisk"]
	if device != "" {
		if whole, err := darwinDiskInfo(ctx, device); err == nil {
			values = whole
		}
	}
	state := strings.ToLower(strings.TrimSpace(values["SMARTStatus"]))
	report := HealthReport{Status: "unavailable", Message: "SMART wird für dieses Laufwerk oder USB-Gehäuse nicht bereitgestellt", Source: "diskutil", CheckedAt: time.Now().Format(time.RFC3339)}
	switch state {
	case "verified":
		report.Status, report.Message = "ok", "macOS meldet SMART: überprüft"
	case "failing":
		report.Status, report.Message = "critical", "macOS meldet SMART: Laufwerk fällt aus"
	}
	return report
}
