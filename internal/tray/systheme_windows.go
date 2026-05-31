//go:build windows

package tray

import (
	"log"
	"sync/atomic"

	"golang.org/x/sys/windows/registry"
)

// lastSystemDark caches the last successful read of SystemUsesLightTheme.
// On transient registry failures we return this value to avoid flicker
// while Windows propagates a theme change.
var lastSystemDark atomic.Bool

func init() { lastSystemDark.Store(true) }

// systemDark reports whether the Windows taskbar is using the dark theme.
// Reads SystemUsesLightTheme under
// HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize (1 =
// light, 0 = dark). Resilient: returns the last known value on any error
// or panic instead of crashing the tray goroutine.
func systemDark() (dark bool) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("tray.systemDark: panic recovered: %v (using last known)", r)
			dark = lastSystemDark.Load()
		}
	}()

	k, err := registry.OpenKey(
		registry.CURRENT_USER,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return lastSystemDark.Load()
	}
	defer k.Close()

	v, _, err := k.GetIntegerValue("SystemUsesLightTheme")
	if err != nil {
		return lastSystemDark.Load()
	}

	dark = v == 0
	lastSystemDark.Store(dark)
	return dark
}
