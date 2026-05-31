//go:build !windows

package tray

// Outside Windows we assume a dark taskbar — the ForDark icon stays
// legible on most Linux/macOS desktops.
func systemDark() bool { return true }
