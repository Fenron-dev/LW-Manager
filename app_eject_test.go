package main

import (
	"strings"
	"testing"

	"github.com/dennis/vaultapp/internal/storage"
)

func TestMatchingEjectVolumeRejectsReusedMountPath(t *testing.T) {
	volumes := []storage.Volume{{Path: "/Volumes/USB", UUID: "new-volume", External: true}}
	if _, err := matchingEjectVolume("/Volumes/USB", "old-volume", volumes); err == nil || !strings.Contains(err.Error(), "Identität") {
		t.Fatalf("reused mount path was not rejected: %v", err)
	}
	if matched, err := matchingEjectVolume("/Volumes/USB", "new-volume", volumes); err != nil || matched.UUID != "new-volume" {
		t.Fatalf("current volume not matched: %+v, %v", matched, err)
	}
}

func TestMatchingEjectVolumeRejectsUnknownAndNonExternal(t *testing.T) {
	volumes := []storage.Volume{{Path: "/Volumes/Internal", UUID: "internal", External: false}}
	for _, path := range []string{"/Volumes/Internal", "/Volumes/Missing"} {
		if _, err := matchingEjectVolume(path, "internal", volumes); err == nil {
			t.Errorf("unexpected match for %s", path)
		}
	}
}
