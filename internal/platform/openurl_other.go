//go:build !windows

package platform

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenURL opens url in the user's default browser. macOS uses "open";
// other Unix systems use "xdg-open" (freedesktop standard, present on the
// Linux desktops AuraRPC targets for the v2 port).
func OpenURL(url string) error {
	var name string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	default:
		name = "xdg-open"
	}
	if err := exec.Command(name, url).Start(); err != nil {
		return fmt.Errorf("openurl: %w", err)
	}
	return nil
}
