// Package fsutil holds small filesystem helpers shared across packages.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path by first writing a temporary file in
// the same directory and then renaming it over path. The rename is atomic
// on a single filesystem, so a reader or a crash never sees a half-written
// file.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("fsutil: create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Remove the temp file if we return before the rename succeeds.
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("fsutil: write temp: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("fsutil: sync temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("fsutil: close temp: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("fsutil: chmod temp: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("fsutil: rename temp: %w", err)
	}
	return nil
}
