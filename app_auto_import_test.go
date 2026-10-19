package main

import (
	"path/filepath"
	"testing"

	"github.com/dennis/vaultapp/internal/storage"
)

func TestAttachmentTrackerProcessesOnlyNewStableAttachments(t *testing.T) {
	tracker := attachmentTracker{}
	first := storage.Volume{Path: filepath.Join(t.TempDir(), "dock"), UUID: "first", External: true}
	second := first; second.UUID = "second"
	if got := tracker.observe([]storage.Volume{first}, true); len(got) != 0 { t.Fatal("startup mount must be a baseline") }
	if got := tracker.observe([]storage.Volume{first, second}, true); len(got) != 0 { t.Fatal("mount must settle before scanning") }
	if got := tracker.observe([]storage.Volume{first, second}, true); len(got) != 1 || got[0].UUID != "second" { t.Fatalf("new volume not queued: %+v", got) }
	tracker.handled(second)
	if got := tracker.observe([]storage.Volume{first, second}, true); len(got) != 0 { t.Fatal("same attachment scanned twice") }
	tracker.observe([]storage.Volume{first}, true)
	tracker.observe([]storage.Volume{first, second}, true)
	if got := tracker.observe([]storage.Volume{first, second}, true); len(got) != 1 { t.Fatal("reattached drive must be scanned again") }
}

func TestAttachmentTrackerDoesNotRetroactivelyScanDisabledMounts(t *testing.T) {
	tracker := attachmentTracker{}
	tracker.observe(nil, false)
	volume := storage.Volume{Path: t.TempDir(), UUID: "disabled", External: true}
	tracker.observe([]storage.Volume{volume}, false)
	if got := tracker.observe([]storage.Volume{volume}, true); len(got) != 0 { t.Fatal("enabling must not scan existing mounts") }
	other := volume; other.UUID = "new-stick-same-drive-letter"
	tracker.observe([]storage.Volume{other}, true)
	if got := tracker.observe([]storage.Volume{other}, true); len(got) != 1 { t.Fatal("new identity at same mount must be recognized") }
}

func TestAutomaticEjectRequiresCompleteScanAndSMART(t *testing.T) {
	result := ScanResult{Health: storage.HealthReport{Status: "ok"}}
	if automaticEjectBlock(result, true) != "" { t.Fatal("completed healthy scan blocked") }
	result.Skipped = 1
	if automaticEjectBlock(result, false) == "" { t.Fatal("read errors must block eject") }
	result.Skipped = 0; result.Health.SelfTestRunning = true
	if automaticEjectBlock(result, false) == "" { t.Fatal("running self-test must block eject") }
	result.Health = storage.HealthReport{Status: "unavailable"}
	if automaticEjectBlock(result, true) == "" { t.Fatal("missing SMART must block by default") }
	if automaticEjectBlock(result, false) != "" { t.Fatal("explicit override not honoured") }
}

func TestVolumeContainsPathHonoursDirectoryBoundaries(t *testing.T) {
	root := t.TempDir()
	if !volumeContainsPath(root, filepath.Join(root, "Vault", "data")) { t.Fatal("vault on disk not recognized") }
	if volumeContainsPath(root, root+"-other") { t.Fatal("unrelated volume matched by prefix") }
}
