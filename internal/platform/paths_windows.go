//go:build windows

// Package platform isolates OS-specific APIs: %APPDATA% paths, auto-start
// registry, single-instance locking, native caption tint, window icon.
package platform

import (
	"errors"
	"os"
	"path/filepath"
)

const appFolder = "AuraRPC"

// AppDataDir returns %APPDATA%\AuraRPC, creating it if missing.
func AppDataDir() (string, error) {
	base := os.Getenv("APPDATA")
	if base == "" {
		return "", errors.New("platform: APPDATA env var is not set")
	}
	dir := filepath.Join(base, appFolder)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}
