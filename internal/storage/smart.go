package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type HealthReport struct {
	Status    string `json:"status"`
	Message   string `json:"message"`
	Source    string `json:"source"`
	CheckedAt string `json:"checkedAt"`
}

func unavailable(message string) HealthReport {
	return HealthReport{Status: "unavailable", Message: message, Source: "SMART", CheckedAt: time.Now().Format(time.RFC3339)}
}

func CheckHealth(path string) HealthReport {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	device, err := smartDevice(ctx, path)
	if err == nil && device != "" {
		if tool := smartctlPath(); tool != "" {
			output, commandErr := smartCommand(ctx, tool, "-j", "-H", "-A", "-l", "selftest", device).Output()
			if report, ok := parseSmartctl(output, commandErr); ok {
				return report
			}
		}
	}
	return nativeHealth(ctx, path)
}

func StartShortSelfTest(path string) (string, error) {
	tool := smartctlPath()
	if tool == "" {
		return "", fmt.Errorf("für einen SMART-Selbsttest wird smartctl benötigt")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	device, err := smartDevice(ctx, path)
	if err != nil || device == "" {
		return "", fmt.Errorf("physisches Laufwerk konnte nicht sicher bestimmt werden: %v", err)
	}
	output, err := smartCommand(ctx, tool, "-t", "short", device).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("SMART-Kurztest konnte nicht gestartet werden: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return "SMART-Kurztest gestartet. Das Ergebnis kann nach Abschluss mit „SMART prüfen“ aktualisiert werden.", nil
}

func smartctlPath() string {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		for _, base := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
			if base != "" {
				candidates = append(candidates, filepath.Join(base, "smartmontools", "bin", "smartctl.exe"))
			}
		}
	case "darwin":
		candidates = []string{"/opt/homebrew/bin/smartctl", "/usr/local/bin/smartctl", "/opt/homebrew/sbin/smartctl", "/usr/local/sbin/smartctl"}
	default:
		candidates = []string{"/usr/sbin/smartctl", "/usr/bin/smartctl", "/sbin/smartctl", "/bin/smartctl"}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && (runtime.GOOS == "windows" || info.Mode()&0o111 != 0) {
			return candidate
		}
	}
	return ""
}

func parseSmartctl(output []byte, commandErr error) (HealthReport, bool) {
	var payload struct {
		SmartStatus struct {
			Passed *bool `json:"passed"`
		} `json:"smart_status"`
		NVMe struct {
			CriticalWarning *int `json:"critical_warning"`
		} `json:"nvme_smart_health_information_log"`
		Smartctl struct {
			ExitStatus int `json:"exit_status"`
		} `json:"smartctl"`
		ATA struct {
			Standard struct {
				Table []struct {
					Status struct {
						Value  int    `json:"value"`
						String string `json:"string"`
					} `json:"status"`
				} `json:"table"`
			} `json:"standard"`
		} `json:"ata_smart_self_test_log"`
		NVMeSelfTest struct {
			CurrentOperation struct {
				Value int `json:"value"`
			} `json:"current_self_test_operation"`
			Table []struct {
				Result struct {
					Value int `json:"value"`
				} `json:"self_test_result"`
			} `json:"table"`
		} `json:"nvme_self_test_log"`
	}
	if json.Unmarshal(output, &payload) != nil {
		return HealthReport{}, false
	}
	code := payload.Smartctl.ExitStatus
	if code == 0 && commandErr != nil {
		return HealthReport{}, false
	}
	if code&0x03 != 0 {
		return HealthReport{}, false
	}
	if payload.SmartStatus.Passed == nil && payload.NVMe.CriticalWarning == nil {
		return HealthReport{}, false
	}
	report := HealthReport{Status: "ok", Message: "SMART meldet keinen kritischen Zustand", Source: "smartctl", CheckedAt: time.Now().Format(time.RFC3339)}
	if code&0x18 != 0 || payload.SmartStatus.Passed != nil && !*payload.SmartStatus.Passed || payload.NVMe.CriticalWarning != nil && *payload.NVMe.CriticalWarning != 0 {
		report.Status, report.Message = "critical", "SMART meldet einen kritischen Laufwerkszustand"
	} else if code&0xe0 != 0 {
		report.Status, report.Message = "warning", "SMART meldet frühere Fehler oder einen auffälligen Selbsttest"
	}
	if len(payload.ATA.Standard.Table) > 0 {
		state := payload.ATA.Standard.Table[0].Status.Value >> 4
		switch {
		case state >= 4 && state <= 8:
			if report.Status == "ok" {
				report.Status = "warning"
			}
			report.Message += "; letzter Selbsttest fehlgeschlagen"
		case state == 15:
			report.Message += "; Selbsttest läuft"
		case state == 0:
			report.Message += "; letzter Selbsttest erfolgreich"
		}
	} else if payload.NVMeSelfTest.CurrentOperation.Value != 0 {
		report.Message += "; Selbsttest läuft"
	} else if len(payload.NVMeSelfTest.Table) > 0 {
		state := payload.NVMeSelfTest.Table[0].Result.Value
		if state >= 5 && state <= 7 {
			if report.Status == "ok" {
				report.Status = "warning"
			}
			report.Message += "; letzter Selbsttest fehlgeschlagen"
		} else if state == 0 {
			report.Message += "; letzter Selbsttest erfolgreich"
		}
	}
	return report, true
}
