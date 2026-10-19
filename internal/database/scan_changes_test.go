package database

import (
	"testing"
	"time"

	"github.com/dennis/vaultapp/internal/scanner"
)

func TestScanChangesDistinguishesNewDriveAndUpdatedCatalogue(t *testing.T) {
	catalog := openMetadataTestCatalog(t)
	root := t.TempDir()
	modified := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	files := []scanner.File{
		{Path: "same.txt", Filename: "same.txt", Size: 10, Modified: modified},
		{Path: "changed.txt", Filename: "changed.txt", Size: 20, Modified: modified},
		{Path: "removed.txt", Filename: "removed.txt", Size: 30, Modified: modified},
	}
	changes, err := catalog.ScanChanges("disk-one", root, files)
	if err != nil || !changes.NewDrive || changes.Added != 3 || changes.AddedBytes != 60 { t.Fatalf("new drive: %+v %v", changes, err) }
	if err := catalog.ReplaceDriveScan(DriveScan{Path: root, UUID: "disk-one", Label: "Test", Files: files}); err != nil { t.Fatal(err) }
	files[1].Size = 25
	files[2] = scanner.File{Path: "new.txt", Filename: "new.txt", Size: 40, Modified: modified}
	changes, err = catalog.ScanChanges("disk-one", root, files)
	if err != nil || changes.NewDrive || changes.Added != 1 || changes.Modified != 1 || changes.Removed != 1 || changes.Unchanged != 1 || changes.AddedBytes != 40 { t.Fatalf("updated drive: %+v %v", changes, err) }
	changes, err = catalog.ScanChanges("disk-two-same-mount", root, files)
	if err != nil || !changes.NewDrive { t.Fatalf("reused mount must be new: %+v %v", changes, err) }
	if err := catalog.ReplaceDriveScan(DriveScan{Path: root, UUID: "empty-disk", Label: "Empty"}); err != nil { t.Fatal(err) }
	changes, err = catalog.ScanChanges("empty-disk", root, nil)
	if err != nil || changes.NewDrive { t.Fatalf("known empty disk: %+v %v", changes, err) }
}
