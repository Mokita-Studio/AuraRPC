package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestLoadMissingReturnsDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Language != "en" {
		t.Errorf("Language = %q, want en", cfg.Language)
	}
	if !cfg.AutoConnect {
		t.Error("AutoConnect should default to true")
	}
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	in := Default()
	in.SidebarOpen = false
	in.LastPresetID = "abc"
	in.Theme = "dark"
	if err := Save(dir, in); err != nil {
		t.Fatal(err)
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.SidebarOpen || out.LastPresetID != "abc" || out.Theme != "dark" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

// TestConcurrentApply hammers Apply from many goroutines. With the lock
// and atomic write in place the file is never corrupted; run with -race to
// also confirm there is no data race on the shared Config. (cgo required.)
func TestConcurrentApply(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = Apply(dir, cfg, func(c *Config) {
				c.LastPresetID = "id"
				c.Theme = "dark"
			})
		}(i)
	}
	wg.Wait()

	// The file must remain valid and loadable after the storm.
	if _, err := Load(dir); err != nil {
		t.Fatalf("config corrupted under concurrency: %v", err)
	}
}

func TestLastPresetSaverCoalesces(t *testing.T) {
	dir := t.TempDir()
	cfg := Default()
	s := NewLastPresetSaver(dir, cfg, 60*time.Millisecond, nil)

	s.Set("a")
	s.Set("b")
	s.Set("c")

	// In-memory value updates immediately to the latest.
	if cfg.LastPresetID != "c" {
		t.Errorf("in-memory LastPresetID = %q, want c", cfg.LastPresetID)
	}
	// Nothing is written to disk before the debounce window elapses.
	if _, err := os.Stat(filepath.Join(dir, fileName)); !os.IsNotExist(err) {
		t.Error("config written before debounce elapsed; not coalesced")
	}

	// After the window, the file holds the final value (single write).
	time.Sleep(150 * time.Millisecond)
	out, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if out.LastPresetID != "c" {
		t.Errorf("persisted LastPresetID = %q, want c", out.LastPresetID)
	}
}
