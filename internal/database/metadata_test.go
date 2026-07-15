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
	tags, err := catalog.Tags()
	if err != nil || len(tags) != 2 || tags[0].Name != "kunde A" || tags[0].DriveCount != 1 || tags[0].SnapshotCount != 0 {
		t.Fatalf("tags = %#v, %v", tags, err)
	}
	result, err := catalog.Search("", "", "KUNDE A", 0, false, 50, 0)
	if err != nil || result.Total != 1 {
		t.Fatalf("tagged search = %#v, %v", result, err)
	}
	result, err = catalog.Search("", "", "nicht vorhanden", 0, false, 50, 0)
	if err != nil || result.Total != 0 {
		t.Fatalf("missing tag search = %#v, %v", result, err)
	}
}

func TestRenameMergeAndDeleteTags(t *testing.T) {
	catalog := openMetadataTestCatalog(t)
	for _, volume := range []string{"volume-a", "volume-b"} {
		if err := catalog.ReplaceDriveScan(DriveScan{Path: t.TempDir(), Label: volume, UUID: volume}); err != nil {
			t.Fatal(err)
		}
	}
	drives, err := catalog.Drives()
	if err != nil || len(drives) != 2 {
		t.Fatalf("drives = %#v, %v", drives, err)
	}
	if err := catalog.UpdateDrive(drives[0].ID, "", "", "", "", "", "", []string{"Mobil", "Kunde"}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.UpdateDrive(drives[1].ID, "", "", "", "", "", "", []string{"Archiv"}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.RenameTag("mobil", "Unterwegs"); err != nil {
		t.Fatal(err)
	}
	if err := catalog.RenameTag("Unterwegs", "Archiv"); err != nil {
		t.Fatal(err)
	}
	tags, err := catalog.Tags()
	if err != nil || len(tags) != 2 || tags[0].Name != "Archiv" || tags[0].DriveCount != 2 {
		t.Fatalf("merged tags = %#v, %v", tags, err)
	}
	if err := catalog.DeleteTag("ARCHIV"); err != nil {
		t.Fatal(err)
	}
	tags, err = catalog.Tags()
	if err != nil || len(tags) != 1 || tags[0].Name != "Kunde" {
		t.Fatalf("remaining tags = %#v, %v", tags, err)
	}
}

func TestManualFileTagsSurviveRescanAndFilterLibrary(t *testing.T) {
	catalog := openMetadataTestCatalog(t)
	root := t.TempDir()
	modified := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	scan := func(files []scanner.File) {
		t.Helper()
		if err := catalog.ReplaceDriveScan(DriveScan{Path: root, Label: "DATEIEN", UUID: "file-tag-volume", Files: files}); err != nil {
			t.Fatal(err)
		}
	}
	scan([]scanner.File{{Path: "docs/plan.txt", Filename: "plan.txt", Size: 12, Modified: modified}})
	result, err := catalog.Search("plan", "", "", 0, false, 50, 0)
	if err != nil || len(result.Files) != 1 {
		t.Fatalf("search = %#v, %v", result, err)
	}
	if err := catalog.UpdateFileTags(result.Files[0].ID, []string{" Wichtig ", "Projekt", "wichtig"}); err != nil {
		t.Fatal(err)
	}
	details, err := catalog.FileDetails(result.Files[0].ID)
	if err != nil || !reflect.DeepEqual(details.Tags, []string{"Projekt", "Wichtig"}) {
		t.Fatalf("file tags = %#v, %v", details, err)
	}
	result, err = catalog.Search("", "", "WICHTIG", 0, false, 50, 0)
	if err != nil || result.Total != 1 {
		t.Fatalf("tag filtered search = %#v, %v", result, err)
	}
	tags, err := catalog.Tags()
	if err != nil || len(tags) != 2 || tags[1].FileCount != 1 || tags[1].LibraryCount != 1 {
		t.Fatalf("tag summary = %#v, %v", tags, err)
	}
	scan([]scanner.File{{Path: "docs/plan.txt", Filename: "plan.txt", Size: 13, Modified: modified.Add(time.Minute)}})
	result, _ = catalog.Search("plan", "", "", 0, false, 50, 0)
	details, err = catalog.FileDetails(result.Files[0].ID)
	if err != nil || len(details.Tags) != 2 {
		t.Fatalf("tags did not survive same-path rescan: %#v, %v", details, err)
	}
	if err := catalog.RenameTag("Wichtig", "Projekt"); err != nil {
		t.Fatal(err)
	}
	details, err = catalog.FileDetails(result.Files[0].ID)
	if err != nil || !reflect.DeepEqual(details.Tags, []string{"Projekt"}) {
		t.Fatalf("merged file tags = %#v, %v", details, err)
	}
	scan(nil)
	tags, err = catalog.Tags()
	if err != nil || len(tags) != 1 || tags[0].FileCount != 0 || tags[0].LibraryCount != 0 {
		t.Fatalf("removed file assignment remains: %#v, %v", tags, err)
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
	tags, err := catalog.Tags()
	if err != nil || len(tags) != 2 || tags[0].SnapshotCount != 1 {
		t.Fatalf("snapshot tags = %#v, %v", tags, err)
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

func TestAIAnalysisSurvivesOnlyUnchangedRescan(t *testing.T) {
	catalog := openMetadataTestCatalog(t)
	root := t.TempDir()
	modified := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	scan := func(size int64, changed time.Time) {
		t.Helper()
		if err := catalog.ReplaceDriveScan(DriveScan{Path: root, Label: "AI", UUID: "ai-volume", Files: []scanner.File{{Path: "docs/readme.txt", Filename: "readme.txt", Size: size, MIMEType: "text/plain", TextContent: "Projektinhalt", Modified: changed}}}); err != nil {
			t.Fatal(err)
		}
	}
	scan(12, modified)
	result, err := catalog.Search("readme", "", "", 0, false, 50, 0)
	if err != nil || len(result.Files) != 1 {
		t.Fatalf("search = %#v, %v", result, err)
	}
	input, err := catalog.AIFileInput(result.Files[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.SaveAIAnalysis(input, AIAnalysis{Summary: "Eine Projektbeschreibung.", Tags: []string{" Projekt ", "Dokument", "projekt"}, Provider: "ollama", Model: "test", ImageBytes: 123, Vision: true}); err != nil {
		t.Fatal(err)
	}
	aiSearch, err := catalog.Search("Projektbeschreibung", "", "", 0, true, 50, 0)
	if err != nil || aiSearch.Total != 1 {
		t.Fatalf("AI search = %#v, %v", aiSearch, err)
	}
	aiSearch, err = catalog.Search("Projektbeschreibung", "", "", 0, false, 50, 0)
	if err != nil || aiSearch.Total != 0 {
		t.Fatalf("disabled AI search = %#v, %v", aiSearch, err)
	}
	details, err := catalog.FileDetails(input.ID)
	if err != nil || details.AISummary == "" || !details.AIVision || details.AIImageBytes != 123 || !reflect.DeepEqual(details.AITags, []string{"Dokument", "Projekt"}) {
		t.Fatalf("details = %#v, %v", details, err)
	}
	scan(12, modified)
	result, _ = catalog.Search("readme", "", "", 0, false, 50, 0)
	details, err = catalog.FileDetails(result.Files[0].ID)
	if err != nil || details.AISummary == "" {
		t.Fatalf("analysis did not survive unchanged rescan: %#v, %v", details, err)
	}
	scan(13, modified.Add(time.Minute))
	result, _ = catalog.Search("readme", "", "", 0, false, 50, 0)
	details, err = catalog.FileDetails(result.Files[0].ID)
	if err != nil || details.AISummary != "" || len(details.AITags) != 0 {
		t.Fatalf("stale analysis survived changed rescan: %#v, %v", details, err)
	}
}
