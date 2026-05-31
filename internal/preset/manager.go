package preset

import (
	"errors"
	"sync"
	"time"
)

// ErrNotFound is returned when a requested preset does not exist.
var ErrNotFound = errors.New("preset: not found")

// Manager keeps presets in memory and persists them as JSON. List and Get
// hand out copies so the UI cannot mutate internal state.
type Manager struct {
	dir string

	mu      sync.Mutex
	presets []*Preset
}

// NewManager builds a Manager backed by dir. Call Load to hydrate.
func NewManager(dir string) *Manager {
	return &Manager{dir: dir}
}

// Load reads from disk, replacing the in-memory state.
func (m *Manager) Load() error {
	list, err := Load(m.dir)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.presets = list
	m.mu.Unlock()
	return nil
}

// List returns copies of every preset in insertion order.
func (m *Manager) List() []*Preset {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Preset, len(m.presets))
	for i, p := range m.presets {
		cp := *p
		out[i] = &cp
	}
	return out
}

// Get returns a copy of the preset with the given id, or ErrNotFound.
func (m *Manager) Get(id string) (*Preset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.presets {
		if p.ID == id {
			cp := *p
			return &cp, nil
		}
	}
	return nil, ErrNotFound
}

// Upsert validates and persists a preset. Generates an ID and CreatedAt
// on new entries; updates UpdatedAt and writes to disk in every case.
func (m *Manager) Upsert(p *Preset) error {
	if err := Validate(p); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	if p.ID == "" {
		p.ID = newID()
		p.CreatedAt = now
	}
	p.UpdatedAt = now

	for i, existing := range m.presets {
		if existing.ID == p.ID {
			cp := *p
			if cp.CreatedAt.IsZero() {
				cp.CreatedAt = existing.CreatedAt
			}
			m.presets[i] = &cp
			return Save(m.dir, m.presets)
		}
	}
	cp := *p
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = now
	}
	m.presets = append(m.presets, &cp)
	return Save(m.dir, m.presets)
}

// Delete removes the preset with the given id and persists.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range m.presets {
		if p.ID == id {
			m.presets = append(m.presets[:i], m.presets[i+1:]...)
			return Save(m.dir, m.presets)
		}
	}
	return ErrNotFound
}
