//go:build windows

package storage

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func Eject(path string) error {
	if len(path) < 2 || path[1] != ':' || !((path[0] >= 'A' && path[0] <= 'Z') || (path[0] >= 'a' && path[0] <= 'z')) {
		return fmt.Errorf("ungültiger Laufwerksbuchstabe")
	}
	letter := strings.ToUpper(path[:1])
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	// Shell.Application performs the same safe-eject action as Windows Explorer.
	// Check disappearance; InvokeVerb itself has no reliable return value.
	script := fmt.Sprintf(`$drive='%s:'; $item=(New-Object -ComObject Shell.Application).Namespace(17).ParseName($drive); if ($null -eq $item) { throw 'Laufwerk nicht gefunden' }; $item.InvokeVerb('Eject'); for ($i=0; $i -lt 20; $i++) { Start-Sleep -Milliseconds 500; if (-not (Test-Path ($drive+'\'))) { exit 0 } }; throw 'Auswerfen nicht bestätigt: Laufwerk ist noch verbunden'`, letter)
	output, err := hiddenPowerShell(ctx, script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Windows-Auswurf fehlgeschlagen: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}
