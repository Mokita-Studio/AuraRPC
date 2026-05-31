//go:build linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"
)

// autostartFile returns the path of the freedesktop autostart entry,
// ~/.config/autostart/AuraRPC.desktop — the mechanism honored by GNOME,
// KDE and most Linux desktop environments.
func autostartFile() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("autostart: config dir: %w", err)
	}
	return filepath.Join(base, "autostart", "AuraRPC.desktop"), nil
}

// SetAutoStart writes or removes the .desktop autostart entry. When
// enabled, its Exec points at the current executable.
func SetAutoStart(enabled bool) error {
	path, err := autostartFile()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("autostart: remove: %w", err)
		}
		return nil
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("autostart: executable path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("autostart: mkdir: %w", err)
	}
	// Exec is double-quoted so paths with spaces still launch correctly.
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=AuraRPC
Exec="%s"
X-GNOME-Autostart-enabled=true
`, exe)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("autostart: write: %w", err)
	}
	return nil
}

// IsAutoStartEnabled reports whether the autostart .desktop entry exists.
func IsAutoStartEnabled() (bool, error) {
	path, err := autostartFile()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("autostart: stat: %w", err)
	}
	return true, nil
}
