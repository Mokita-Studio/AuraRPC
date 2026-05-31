//go:build !windows && !linux

package platform

// Auto-start stubs for platforms without an implementation (currently
// macOS). Windows uses the registry; Linux writes a .desktop entry.

// SetAutoStart is a no-op on these platforms.
func SetAutoStart(bool) error { return nil }

// IsAutoStartEnabled always reports false on these platforms.
func IsAutoStartEnabled() (bool, error) { return false, nil }
