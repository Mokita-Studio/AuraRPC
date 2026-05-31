//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

// Per-user Run key — no elevation required.
const (
	runKey       = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "AuraRPC"
)

// SetAutoStart writes or removes the auto-start registry entry. When
// enabled is true the value points at the current executable.
func SetAutoStart(enabled bool) error {
	if enabled {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("autostart: executable path: %w", err)
		}
		k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
		if err != nil {
			return fmt.Errorf("autostart: open run key: %w", err)
		}
		defer k.Close()
		if err := k.SetStringValue(runValueName, exe); err != nil {
			return fmt.Errorf("autostart: write value: %w", err)
		}
		return nil
	}

	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("autostart: open run key: %w", err)
	}
	defer k.Close()
	if err := k.DeleteValue(runValueName); err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("autostart: delete value: %w", err)
	}
	return nil
}

// IsAutoStartEnabled reports whether the auto-start entry exists.
func IsAutoStartEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("autostart: open run key: %w", err)
	}
	defer k.Close()
	val, _, err := k.GetStringValue(runValueName)
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("autostart: read value: %w", err)
	}
	return val != "", nil
}
