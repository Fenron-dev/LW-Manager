//go:build darwin

package storage

import "testing"

func TestDarwinPlistValues(t *testing.T) {
	values := darwinPlistValues([]byte(`<?xml version="1.0"?><plist><dict><key>VolumeName</key><string>ARCHIV</string><key>VolumeUUID</key><string>ABC-123</string><key>Internal</key><false/><key>Ejectable</key><true/><key>TotalSize</key><integer>2048</integer></dict></plist>`))
	if values["VolumeName"] != "ARCHIV" || values["VolumeUUID"] != "ABC-123" || values["Internal"] != "false" || values["Ejectable"] != "true" || values["TotalSize"] != "2048" {
		t.Fatalf("unexpected plist values: %#v", values)
	}
}
