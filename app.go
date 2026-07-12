package main

import (
	"context"
	"fmt"
	"runtime"

	"github.com/dennis/vaultapp/internal/vault"
)

type App struct {
	ctx context.Context
}

type AppInfo struct {
	Version   string `json:"version"`
	Platform  string `json:"platform"`
	VaultRoot string `json:"vaultRoot"`
	Ready     bool   `json:"ready"`
	Message   string `json:"message"`
}

func NewApp() *App { return &App{} }

func (a *App) Startup(ctx context.Context) { a.ctx = ctx }

// GetAppInfo is the first backend endpoint used by the Wails frontend.
func (a *App) GetAppInfo() AppInfo {
	root, err := vault.ResolveRoot()
	if err != nil {
		return AppInfo{Version: "0.1.0-dev", Platform: runtime.GOOS, Message: err.Error()}
	}
	if err := vault.EnsureLayout(root); err != nil {
		return AppInfo{Version: "0.1.0-dev", Platform: runtime.GOOS, VaultRoot: root, Message: fmt.Sprintf("Vault kann nicht vorbereitet werden: %v", err)}
	}
	return AppInfo{Version: "0.1.0-dev", Platform: runtime.GOOS, VaultRoot: root, Ready: true, Message: "Vault ist bereit"}
}
