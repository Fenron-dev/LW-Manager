//go:build !darwin

package thumbnail

import "fmt"

func convertHEIC(_, _ string) error {
	return fmt.Errorf("HEIC-Vorschauen werden auf dieser Plattform derzeit nicht unterstützt")
}
