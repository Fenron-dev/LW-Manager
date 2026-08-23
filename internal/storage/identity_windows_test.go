//go:build windows

package storage

import "testing"

func TestCanonicalWindowsVolumeID(t *testing.T) {
	tests := map[string]string{
		`\\?\Volume{A1B2-C3D4}\`: "a1b2-c3d4",
		`volume{A1B2-C3D4}`:      "a1b2-c3d4",
		" 7F00AB12 ":              "7f00ab12",
	}
	for input, expected := range tests {
		if actual := canonicalWindowsVolumeID(input); actual != expected {
			t.Errorf("canonicalWindowsVolumeID(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestHiddenPowerShellSuppressesConsoleWindow(t *testing.T) {
	command := hiddenPowerShell(t.Context(), "Write-Output ok")
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow || command.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatalf("PowerShell process is not configured as hidden: %#v", command.SysProcAttr)
	}
}
