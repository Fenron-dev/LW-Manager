package main

import (
	"reflect"
	"testing"

	appconfig "github.com/dennis/vaultapp/internal/config"
)

func TestEffectiveScanSettingsUsesAssignedProfileAndGlobalLimits(t *testing.T) {
	settings := appconfig.Defaults()
	settings.ScanExclusionsEnabled = false
	settings.ImageAnalysisEnabled = false
	settings.TextFileMB = 37
	settings.ScanProfiles = []appconfig.ScanProfile{{
		ID: "profile-work", Name: "Arbeitsdaten", ExclusionsEnabled: true,
		ExcludeDevelopment: true, ExcludedPatterns: []string{"tmp-*"},
		ImageAnalysisEnabled: true, EXIFEnabled: true, TextIndexEnabled: true,
		TextDocumentsEnabled: true, TextPDFEnabled: true, TextOfficeEnabled: true,
	}}

	effective, name := effectiveScanSettings(settings, "profile-work")
	if name != "Arbeitsdaten" || !effective.ScanExclusionsEnabled || !effective.ScanExcludeDevelopment || !effective.ImageAnalysisEnabled || !effective.EXIFEnabled || !effective.TextIndexEnabled {
		t.Fatalf("profile was not applied: %q, %+v", name, effective)
	}
	if !reflect.DeepEqual(effective.ScanExcludedPatterns, []string{"tmp-*"}) || effective.TextFileMB != 37 {
		t.Fatalf("patterns or global limits changed unexpectedly: %+v", effective)
	}

	fallback, name := effectiveScanSettings(settings, "missing")
	if name != "Globale Einstellungen" || fallback.ScanExclusionsEnabled || fallback.ImageAnalysisEnabled {
		t.Fatalf("missing profile did not fall back to globals: %q, %+v", name, fallback)
	}
}
