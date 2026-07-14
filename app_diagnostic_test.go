package main

import (
	"testing"
	"time"

	appconfig "github.com/dennis/vaultapp/internal/config"
	"github.com/dennis/vaultapp/internal/scanner"
	"github.com/dennis/vaultapp/internal/vault"
)

func TestScanDiagnosticCanBeStoredAndDisabled(t *testing.T) {
	root := t.TempDir()
	if err := vault.EnsureLayout(root); err != nil {
		t.Fatal(err)
	}
	settings := appconfig.Defaults()
	app := &App{root: root, settings: settings}
	path := app.writeScanDiagnostic(ScanDiagnostic{
		StartedAt: time.Now().Add(-time.Second), Drive: "/Volumes/Test", Files: 12, Skipped: 1,
		Issues: []scanner.Issue{{Path: "privat.txt", Operation: "Dateiinformationen lesen", Message: "keine Berechtigung"}},
	})
	if path == "" {
		t.Fatal("diagnostic was not stored")
	}
	reports, err := app.GetScanDiagnostics()
	if err != nil || len(reports) != 1 || reports[0].Files != 12 || len(reports[0].Issues) != 1 {
		t.Fatalf("reports = %#v, %v", reports, err)
	}
	settings.ScanDiagnosticsEnabled = false
	app.settings = settings
	if path := app.writeScanDiagnostic(ScanDiagnostic{StartedAt: time.Now()}); path != "" {
		t.Fatalf("disabled diagnostics returned %q", path)
	}
}
