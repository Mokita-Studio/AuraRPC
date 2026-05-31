//go:build windows

package platform

import (
	"fmt"
	"os/exec"
)

// OpenURL opens url in the user's default browser. On Windows this goes
// through the shell's URL protocol handler via rundll32, which avoids the
// quoting pitfalls of "cmd /c start".
func OpenURL(url string) error {
	cmd := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("openurl: %w", err)
	}
	return nil
}
