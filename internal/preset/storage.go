package preset

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"aurarpc/internal/fsutil"
)

const (
	fileName = "presets.json"
	// appID tags files written by AuraRPC so shared or imported files can
	// be recognized.
	appID = "aurarpc"
	// schemaVersion is the current presets schema. Bump it when the
	// on-disk shape changes and add the conversion to migrate.
	schemaVersion = 1
)

// envelope is the on-disk and shareable presets container. The same shape
// is used for the local store and for export/import.
type envelope struct {
	App     string    `json:"app"`
	Version int       `json:"version"`
	Presets []*Preset `json:"presets"`
}

// Load reads dir/presets.json. Returns (nil, nil) if the file is missing.
func Load(dir string) ([]*Preset, error) {
	path := filepath.Join(dir, fileName)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	env, err := decodeEnvelope(data, path)
	if err != nil {
		return nil, err
	}
	return env.Presets, nil
}

// Save writes presets to dir/presets.json atomically, after copying the
// previous file to presets.json.bak as a single rolling backup.
func Save(dir string, presets []*Preset) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, fileName)

	// Keep one backup of the last good file before overwriting.
	if prev, err := os.ReadFile(path); err == nil {
		_ = os.WriteFile(path+".bak", prev, 0o644)
	}

	env := envelope{App: appID, Version: schemaVersion, Presets: presets}
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, data, 0o644)
}

// decodeEnvelope parses a presets file and migrates it to the current
// schema. Unknown fields from newer versions are ignored by the JSON
// decoder, so an older build can still read a newer file.
func decodeEnvelope(data []byte, src string) (envelope, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return env, fmt.Errorf("preset: parse %s: %w", src, err)
	}
	env.Presets = migrate(env.Version, env.Presets)
	return env, nil
}

// migrate upgrades presets parsed from an older schema to the current one.
// Files written before the envelope carried a version report 0 and share
// the version 1 shape, so no conversion is needed yet. Future schema bumps
// add their per-version conversions here.
func migrate(version int, presets []*Preset) []*Preset {
	_ = version
	return presets
}
