//go:build darwin

package thumbnail

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

func convertHEIC(source, destination string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/sips", "-s", "format", "jpeg", "--resampleHeightWidthMax", "900", source, "--out", destination).CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("HEIC-Vorschau hat das Zeitlimit überschritten")
	}
	if err != nil {
		return fmt.Errorf("HEIC-Vorschau konnte nicht erzeugt werden: %s", string(output))
	}
	return nil
}
