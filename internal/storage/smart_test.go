package storage

import (
	"errors"
	"strings"
	"testing"
)

func TestParseSmartctlHealth(t *testing.T) {
	tests := []struct {
		name   string
		json   string
		status string
		found  bool
	}{
		{"healthy ATA", `{"smart_status":{"passed":true},"smartctl":{"exit_status":0}}`, "ok", true},
		{"failing ATA", `{"smart_status":{"passed":false},"smartctl":{"exit_status":8}}`, "critical", true},
		{"NVMe warning", `{"nvme_smart_health_information_log":{"critical_warning":1},"smartctl":{"exit_status":0}}`, "critical", true},
		{"failed self-test", `{"smart_status":{"passed":true},"ata_smart_self_test_log":{"standard":{"table":[{"status":{"value":112}}]}},"smartctl":{"exit_status":128}}`, "warning", true},
		{"unsupported", `{"smartctl":{"exit_status":2}}`, "", false},
		{"missing health", `{"smartctl":{"exit_status":0}}`, "", false},
	}
	for _, item := range tests {
		t.Run(item.name, func(t *testing.T) {
			report, found := parseSmartctl([]byte(item.json), nil)
			if found != item.found || report.Status != item.status {
				t.Fatalf("report=%+v found=%t", report, found)
			}
		})
	}
	if _, found := parseSmartctl([]byte(`{"smart_status":{"passed":true}}`), errors.New("command failed")); found {
		t.Fatal("failed command without SMART exit status must not be trusted")
	}
	report, found := parseSmartctl([]byte(`{"smart_status":{"passed":true},"nvme_self_test_log":{"table":[{"self_test_result":{"value":7}}]},"smartctl":{"exit_status":0}}`), nil)
	if !found || report.Status != "warning" || !strings.Contains(report.Message, "Selbsttest") {
		t.Fatalf("NVMe self-test report=%+v found=%t", report, found)
	}
}
