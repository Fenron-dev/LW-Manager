//go:build windows

package storage

import "testing"

func TestDecodeWindowsVolumesAcceptsSingleObject(t *testing.T) {
	items, err := decodeWindowsVolumes([]byte(`{"Path":"E:\\\\","Label":"Backup","UUID":"volume{A1B2}","FSType":"exFAT","Total":1000,"Used":250}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Path != `E:\\` || items[0].Label != "Backup" || items[0].Total != 1000 {
		t.Fatalf("unexpected decoded volume: %#v", items)
	}
}

func TestDecodeWindowsVolumesAcceptsArrayAndEmptyResult(t *testing.T) {
	items, err := decodeWindowsVolumes([]byte(`[{"Path":"E:\\\\"},{"Path":"F:\\\\"}]`))
	if err != nil || len(items) != 2 {
		t.Fatalf("unexpected decoded array: %#v, %v", items, err)
	}
	for _, data := range [][]byte{nil, []byte("null"), []byte("  ")} {
		items, err = decodeWindowsVolumes(data)
		if err != nil || len(items) != 0 {
			t.Fatalf("unexpected empty result for %q: %#v, %v", data, items, err)
		}
	}
}

func TestCanonicalWindowsVolumeID(t *testing.T) {
	tests := map[string]string{
		`\\?\Volume{A1B2-C3D4}\`: "a1b2-c3d4",
		`volume{A1B2-C3D4}`:      "a1b2-c3d4",
		" 7F00AB12 ":             "7f00ab12",
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
