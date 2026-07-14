package database

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dennis/vaultapp/internal/scanner"
)

func openMetadataTestCatalog(t *testing.T) *Catalog {
	t.Helper()
	catalog, err := Open(filepath.Join(t.TempDir(), "vault.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = catalog.Close() })
	return catalog
}

func scanMetadataTestDrive(t *testing.T, catalog *Catalog, root string, maxSnapshots int, filename string) {
	t.Helper()
	err := catalog.ReplaceDriveScan(DriveScan{
		Path: root, Label: "TEST", UUID: "test-volume", Archive: true, MaxSnapshots: maxSnapshots,
		Files: []scanner.File{{Path: filename, Filename: filename, Size: 12, Modified: time.Now()}},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestDriveMetadataAndTags(t *testing.T) {
	catalog := openMetadataTestCatalog(t)
	root := t.TempDir()
	scanMetadataTestDrive(t, catalog, root, 0, "first.txt")
	drives, err := catalog.Drives()
	if err != nil || len(drives) != 1 {
		t.Fatalf("drives = %#v, %v", drives, err)
	}
	if err := catalog.UpdateDrive(drives[0].ID, "Archiv A", "17", "Acme", "USB-C Stick", "Schrank", "Übergabe an Team", []string{" Mobil ", "kunde A", "mobil"}); err != nil {
		t.Fatal(err)
	}
	drives, err = catalog.Drives()
	if err != nil {
		t.Fatal(err)
	}
	if drives[0].Note != "Übergabe an Team" || !reflect.DeepEqual(drives[0].Tags, []string{"kunde A", "Mobil"}) {
		t.Fatalf("metadata = note %q, tags %#v", drives[0].Note, drives[0].Tags)
	}
}

func TestProtectedSnapshotSurvivesCleanupAndDelete(t *testing.T) {
	catalog := openMetadataTestCatalog(t)
	root := t.TempDir()
	scanMetadataTestDrive(t, catalog, root, 1, "one.txt")
	scanMetadataTestDrive(t, catalog, root, 1, "two.txt")
	drives, _ := catalog.Drives()
	snapshots, err := catalog.Snapshots(drives[0].ID)
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("snapshots = %#v, %v", snapshots, err)
	}
	protectedID := snapshots[0].ID
	if err := catalog.UpdateSnapshot(protectedID, true, "Wichtiger Stand", []string{"Referenz", "Freigabe"}); err != nil {
		t.Fatal(err)
	}
	scanMetadataTestDrive(t, catalog, root, 1, "three.txt")
	scanMetadataTestDrive(t, catalog, root, 1, "four.txt")
	snapshots, err = catalog.Snapshots(drives[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("protected plus newest snapshots = %d, want 2", len(snapshots))
	}
	var found bool
	for _, snapshot := range snapshots {
		if snapshot.ID == protectedID {
			found = snapshot.Protected && snapshot.Note == "Wichtiger Stand" && reflect.DeepEqual(snapshot.Tags, []string{"Freigabe", "Referenz"})
		}
	}
	if !found {
		t.Fatalf("protected snapshot metadata missing: %#v", snapshots)
	}
	if err := catalog.DeleteSnapshot(protectedID); err == nil || !strings.Contains(err.Error(), "geschützt") {
		t.Fatalf("DeleteSnapshot error = %v", err)
	}
	if err := catalog.UpdateSnapshot(protectedID, false, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := catalog.DeleteSnapshot(protectedID); err != nil {
		t.Fatal(err)
	}
}
