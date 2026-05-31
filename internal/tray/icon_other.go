//go:build !windows

package tray

import "aurarpc/internal/tray/icons"

// trayIcon returns raw PNG bytes for systray.SetIcon. On Linux and macOS
// systray expects a PNG (the ICO container is a Windows-only requirement).
func trayIcon(dark bool) []byte {
	if dark {
		return icons.ForDark16
	}
	return icons.ForLight16
}
