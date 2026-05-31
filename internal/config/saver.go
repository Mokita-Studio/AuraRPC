package config

import (
	"sync"
	"time"
)

// LastPresetSaver persists LastPresetID while coalescing rapid changes.
// Set updates the value in memory immediately and schedules a single
// atomic disk write after a quiet period, so fast preset switching causes
// neither a storm of writes nor an out-of-order save.
type LastPresetSaver struct {
	dir   string
	cfg   *Config
	delay time.Duration
	onErr func(error)

	mu    sync.Mutex
	timer *time.Timer
}

// NewLastPresetSaver builds a saver for cfg stored in dir. delay is the
// coalescing window; onErr (optional) receives disk-write errors.
func NewLastPresetSaver(dir string, cfg *Config, delay time.Duration, onErr func(error)) *LastPresetSaver {
	return &LastPresetSaver{dir: dir, cfg: cfg, delay: delay, onErr: onErr}
}

// Set records id as the last applied preset. The in-memory value updates
// at once; the disk write is debounced so a burst collapses to one write.
func (s *LastPresetSaver) Set(id string) {
	saveMu.Lock()
	s.cfg.LastPresetID = id
	saveMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer == nil {
		s.timer = time.AfterFunc(s.delay, s.flush)
	} else {
		s.timer.Reset(s.delay)
	}
}

// flush writes the current config to disk. It runs from the debounce timer
// and persists whatever LastPresetID was set last.
func (s *LastPresetSaver) flush() {
	s.mu.Lock()
	s.timer = nil
	s.mu.Unlock()
	if err := Save(s.dir, s.cfg); err != nil && s.onErr != nil {
		s.onErr(err)
	}
}
