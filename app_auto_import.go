package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/dennis/vaultapp/internal/config"
	"github.com/dennis/vaultapp/internal/storage"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type AutoImportStatus struct {
	Phase string `json:"phase"`
	Path string `json:"path"`
	Label string `json:"label"`
	Message string `json:"message"`
	UpdatedAt string `json:"updatedAt"`
	Result ScanResult `json:"result"`
}

// Attachment tracking is independent of catalogue identity. A volume is handled
// once per attachment, after two successful observations. Existing mounts form
// a baseline at startup or after switching vaults.
type attachmentState struct { observations int; handled bool }
type attachmentTracker struct { initialized bool; entries map[string]attachmentState }

func attachmentKey(volume storage.Volume) string {
	return strings.ToLower(strings.TrimSpace(volume.UUID)) + "\x00" + filepath.Clean(volume.Path)
}

func (t *attachmentTracker) observe(volumes []storage.Volume, enabled bool) []storage.Volume {
	next := make(map[string]attachmentState)
	ready := []storage.Volume{}
	for _, volume := range volumes {
		if !volume.External { continue }
		key := attachmentKey(volume)
		state := t.entries[key]
		state.observations++
		if !t.initialized || !enabled { state.handled = true }
		if enabled && !state.handled && state.observations >= 2 { ready = append(ready, volume) }
		next[key] = state
	}
	t.entries, t.initialized = next, true
	return ready
}

func (t *attachmentTracker) handled(volume storage.Volume) {
	key := attachmentKey(volume)
	state := t.entries[key]; state.handled = true; t.entries[key] = state
}

func (a *App) GetAutoImportStatus() AutoImportStatus {
	a.autoMu.RLock(); defer a.autoMu.RUnlock()
	return a.autoStatus
}

func (a *App) publishAutoImport(status AutoImportStatus) {
	status.UpdatedAt = time.Now().Format(time.RFC3339Nano)
	a.autoMu.Lock(); a.autoStatus = status; a.autoMu.Unlock()
	wailsruntime.EventsEmit(a.ctx, "auto-import:status", status)
}

func (a *App) startAutoImport() {
	ctx, cancel := context.WithCancel(a.ctx)
	a.autoCancel, a.autoDone = cancel, make(chan struct{})
	go func() {
		defer close(a.autoDone)
		ticker := time.NewTicker(3 * time.Second); defer ticker.Stop()
		tracker := attachmentTracker{}
		lastRoot := ""
		for {
			if a.scanMu.TryLock() {
				if a.root != lastRoot { tracker = attachmentTracker{}; lastRoot = a.root }
				settings := a.currentSettings()
				if a.initErr == nil && a.catalog != nil && settings.VolumeDetectionEnabled {
					volumes, err := storage.ListVolumes()
					if err == nil {
						ready := tracker.observe(volumes, settings.AutoImportEnabled)
						if len(ready) > 0 {
							volume := ready[0]
							tracker.handled(volume)
							if ctx.Err() == nil { a.processAutomaticVolume(ctx, volume, settings) }
						}
					} else if settings.AutoImportEnabled {
						a.publishAutoImport(AutoImportStatus{Phase: "error", Message: "Datenträgererkennung fehlgeschlagen: " + err.Error()})
					}
				}
				a.scanMu.Unlock()
			}
			select { case <-ctx.Done(): return; case <-ticker.C: }
		}
	}()
}

func volumeContainsPath(volumePath, target string) bool {
	if target == "" { return false }
	volumePath, target = filepath.Clean(volumePath), filepath.Clean(target)
	// Resolve the macOS /var -> /private/var alias where possible.
	if resolved, err := filepath.EvalSymlinks(volumePath); err == nil { volumePath = resolved }
	if resolved, err := filepath.EvalSymlinks(target); err == nil { target = resolved }
	relative, err := filepath.Rel(volumePath, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func automaticEjectBlock(result ScanResult, requireSMART bool) string {
	if result.Skipped > 0 { return "Scan enthält Lesefehler. Datenträger bleibt zur Prüfung verbunden." }
	if result.Health.SelfTestRunning { return "SMART-Selbsttest läuft noch. Datenträger bleibt verbunden." }
	if requireSMART && (result.Health.Status == "" || result.Health.Status == "unavailable") {
		return "SMART-Befund nicht verfügbar. Datenträger bleibt verbunden; siehe SMART-Details."
	}
	return ""
}

// Called with scanMu held, so scan, health check and eject form one operation.
func (a *App) processAutomaticVolume(ctx context.Context, volume storage.Volume, settings appconfig.Settings) {
	status := AutoImportStatus{Phase: "scanning", Path: volume.Path, Label: volume.Label, Message: "Datenträger wird automatisch eingelesen; anschließend folgt die SMART-Prüfung."}
	fail := func(message string) { status.Phase, status.Message = "error", message; a.publishAutoImport(status) }
	executable, _ := os.Executable()
	if volumeContainsPath(volume.Path, a.root) || volumeContainsPath(volume.Path, executable) {
		fail("Datenträger enthält den geöffneten Vault oder die laufende App und wird nicht automatisch verarbeitet."); return
	}
	if strings.TrimSpace(volume.UUID) == "" { fail("Keine eindeutige Volume-ID verfügbar. Bitte den Datenträger manuell einlesen."); return }
	volumes, err := storage.ListVolumes()
	if err != nil { fail(err.Error()); return }
	if _, err := matchingEjectVolume(volume.Path, volume.UUID, volumes); err != nil { fail(err.Error()); return }
	a.publishAutoImport(status)
	result, err := a.scanPathLocked(ctx, volume.Path, storage.Identity{UUID: volume.UUID, Label: volume.Label, FSType: volume.FSType}, true)
	if err != nil { fail("Automatisches Einlesen fehlgeschlagen: " + err.Error()); return }
	status.Result = result
	if ctx.Err() != nil { fail("Automatik abgebrochen. Datenträger bleibt verbunden."); return }
	// Honour disabling the workflow/ejection while a long scan was running.
	current := a.currentSettings()
	if !current.AutoImportEnabled || !current.AutoImportEject || !settings.AutoImportEject {
		status.Phase, status.Message = "complete", "Einlesen und SMART-Prüfung abgeschlossen. Datenträger bleibt verbunden."
		a.publishAutoImport(status); return
	}
	if message := automaticEjectBlock(result, current.AutoImportRequireSMART); message != "" { fail(message); return }
	if err := automaticEjectSiblings(ctx, volume); err != nil { fail(err.Error()); return }
	status.Phase, status.Message = "ejecting", "Katalog gespeichert und SMART geprüft. Datenträger wird sicher ausgeworfen …"
	a.publishAutoImport(status)
	if err := a.ejectVolumeLocked(volume.Path, volume.UUID); err != nil { fail(fmt.Sprintf("Einlesen abgeschlossen; Auswerfen fehlgeschlagen: %v", err)); return }
	status.Phase, status.Message = "ejected", "Sicher ausgeworfen – Datenträger kann aus dem USB-Dock entnommen werden."
	if result.Health.Status == "warning" || result.Health.Status == "critical" { status.Message += " SMART meldet Auffälligkeiten; siehe Datenträgerdetails." }
	a.publishAutoImport(status)
}

func automaticEjectSiblings(ctx context.Context, volume storage.Volume) error {
	volumes, err := storage.ListVolumes()
	if err != nil { return err }
	if len(volumes) <= 1 { return nil }
	checkCtx, cancel := context.WithTimeout(ctx, 20*time.Second); defer cancel()
	device, err := storage.PhysicalDevice(checkCtx, volume.Path)
	if err != nil { return fmt.Errorf("physische Zuordnung vor dem Auswerfen nicht verfügbar; Datenträger bleibt verbunden") }
	for _, other := range volumes {
		if attachmentKey(other) == attachmentKey(volume) { continue }
		parent, err := storage.PhysicalDevice(checkCtx, other.Path)
		if err != nil { return fmt.Errorf("weitere Volumes konnten vor dem Auswerfen nicht sicher zugeordnet werden; Datenträger bleibt verbunden") }
		if parent == device { return fmt.Errorf("weitere Partitionen derselben Festplatte sind eingebunden. Bitte nach deren Einlesen manuell auswerfen") }
	}
	return nil
}
