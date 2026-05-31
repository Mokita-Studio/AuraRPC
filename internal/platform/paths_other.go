//go:build !windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

const appFolder = "AuraRPC"

// AppDataDir returns the per-user config directory for AuraRPC, creating
// it if missing. os.UserConfigDir maps to $XDG_CONFIG_HOME (or ~/.config)
// on Linux and ~/Library/Application Support on macOS — the native
// convention on each platform.
func AppDataDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("platform: user config dir: %w", err)
	}
	dir := filepath.Join(base, appFolder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
