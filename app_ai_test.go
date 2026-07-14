package main

import (
	"os"
	"strings"
	"testing"

	appconfig "github.com/dennis/vaultapp/internal/config"
)

func TestAICredentialLifecycle(t *testing.T) {
	app := NewApp()
	app.root = t.TempDir()
	app.settings = appconfig.Defaults()
	if err := app.SaveAICredential("  secret-token  "); err != nil {
		t.Fatal(err)
	}
	status, err := app.GetAIProviderStatus()
	if err != nil || !status.CredentialStored {
		t.Fatalf("unexpected status: %+v, %v", status, err)
	}
	path, err := app.aiCredentialPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || strings.TrimSpace(string(data)) != "secret-token" {
		t.Fatalf("unexpected stored credential: %q, %v", data, err)
	}
	if err := app.ClearAICredential(); err != nil {
		t.Fatal(err)
	}
	status, err = app.GetAIProviderStatus()
	if err != nil || status.CredentialStored {
		t.Fatalf("credential was not cleared: %+v, %v", status, err)
	}
}

func TestEmptyAICredentialIsRejected(t *testing.T) {
	app := NewApp()
	app.root = t.TempDir()
	if err := app.SaveAICredential("  "); err == nil {
		t.Fatal("expected empty credential error")
	}
}
