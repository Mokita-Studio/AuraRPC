// Package config reads and writes the user preferences file at
// %APPDATA%\AuraRPC\config.json.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"aurarpc/internal/fsutil"
)

const fileName = "config.json"

// saveMu serializes writes and the field access around them so concurrent
// callers never tear the struct during encoding or the file on disk.
var saveMu sync.Mutex

// Config holds user preferences. Missing JSON fields fall back to the zero value.
type Config struct {
	Language        string `json:"language"`
	Theme           string `json:"theme"` // "light" | "dark"
	StartWithSystem bool   `json:"start_with_system"`
	StartMinimized  bool   `json:"start_minimized"`
	LastPresetID    string `json:"last_preset_id"`
	AutoConnect     bool   `json:"auto_connect"`
	SidebarOpen     bool   `json:"sidebar_open"`
	CheckUpdates    bool   `json:"check_updates"`
}

// Default returns the first-run defaults. Once the user saves config.json
// its values override these.
func Default() *Config {
	return &Config{
		Language:     "en",
		Theme:        "dark",
		AutoConnect:  true,
		SidebarOpen:  true,
		CheckUpdates: true,
	}
}

// Load reads dir/config.json. Returns Default() with no error if missing.
func Load(dir string) (*Config, error) {
	path := filepath.Join(dir, fileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return nil, err
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Save writes cfg to dir/config.json. Safe for concurrent use.
func Save(dir string, cfg *Config) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	return saveLocked(dir, cfg)
}

// Apply mutates cfg and persists it as one step under the config lock, so
// the change and the write never race with another goroutine. Use this
// instead of mutating cfg and calling Save separately.
func Apply(dir string, cfg *Config, mutate func(*Config)) error {
	saveMu.Lock()
	defer saveMu.Unlock()
	mutate(cfg)
	return saveLocked(dir, cfg)
}

// Read runs fn with the config lock held, for consistent reads of fields
// another goroutine may update through Apply.
func Read(cfg *Config, fn func(*Config)) {
	saveMu.Lock()
	defer saveMu.Unlock()
	fn(cfg)
}

// saveLocked encodes cfg and writes dir/config.json atomically. Callers
// must hold saveMu.
func saveLocked(dir string, cfg *Config) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(dir, fileName), data, 0o644)
}
