//go:build windows

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func driveLetter(path string) (string, error) {
	if len(path) < 2 || path[1] != ':' {
		return "", fmt.Errorf("kein Laufwerksbuchstabe vorhanden")
	}
	letter := strings.ToUpper(path[:1])
	if letter[0] < 'A' || letter[0] > 'Z' {
		return "", fmt.Errorf("ungültiger Laufwerksbuchstabe")
	}
	return letter, nil
}

func smartDevice(ctx context.Context, path string) (string, error) {
	letter, err := driveLetter(path)
	if err != nil {
		return "", err
	}
	script := fmt.Sprintf("(Get-Partition -DriveLetter '%s' | Get-Disk).Number", letter)
	output, err := hiddenPowerShell(ctx, script).Output()
	if err != nil {
		return "", err
	}
	number, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || number < 0 {
		return "", fmt.Errorf("physische Laufwerksnummer nicht verfügbar")
	}
	return fmt.Sprintf("/dev/pd%d", number), nil
}

func nativeHealth(ctx context.Context, path string) HealthReport {
	letter, err := driveLetter(path)
	if err != nil {
		return unavailable("Kein physisches Laufwerk zugeordnet")
	}
	script := fmt.Sprintf(`$d=Get-Partition -DriveLetter '%s' | Get-Disk; $serial=[string]$d.SerialNumber; $p=Get-PhysicalDisk | Where-Object { $_.SerialNumber -and $serial -and $_.SerialNumber.Trim() -eq $serial.Trim() } | Select-Object -First 1; if ($p) { [PSCustomObject]@{Health=[string]$p.HealthStatus; Serial=[string]$p.SerialNumber} | ConvertTo-Json -Compress }`, letter)
	output, err := hiddenPowerShell(ctx, script).Output()
	if err != nil || len(output) == 0 {
		return unavailable("Windows stellt für dieses Laufwerk keinen Gesundheitsstatus bereit")
	}
	var payload struct{ Health string }
	if json.Unmarshal(output, &payload) != nil {
		return unavailable("Windows-Gesundheitsstatus konnte nicht gelesen werden")
	}
	report := HealthReport{Status: "unavailable", Message: "Windows meldet keinen eindeutigen Gesundheitsstatus", Source: "Windows Storage", CheckedAt: time.Now().Format(time.RFC3339)}
	switch strings.ToLower(payload.Health) {
	case "healthy":
		report.Status, report.Message = "ok", "Windows meldet das Laufwerk als gesund"
	case "warning":
		report.Status, report.Message = "warning", "Windows meldet eine Laufwerkswarnung"
	case "unhealthy":
		report.Status, report.Message = "critical", "Windows meldet das Laufwerk als fehlerhaft"
	}
	return report
}
