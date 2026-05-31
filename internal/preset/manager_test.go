package preset

import (
	"errors"
	"testing"
)

func TestManagerUpsertAndGet(t *testing.T) {
	mgr := NewManager(t.TempDir())
	p := &Preset{Name: "Hello", ClientID: "123"}
	if err := mgr.Upsert(p); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if p.ID == "" {
		t.Fatal("ID not assigned")
	}
	if p.CreatedAt.IsZero() {
		t.Fatal("CreatedAt not set")
	}
	got, err := mgr.Get(p.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Hello" {
		t.Errorf("Name = %q, want Hello", got.Name)
	}
}

func TestManagerListReturnsCopies(t *testing.T) {
	mgr := NewManager(t.TempDir())
	_ = mgr.Upsert(&Preset{Name: "A", ClientID: "123"})
	list := mgr.List()
	if len(list) != 1 {
		t.Fatalf("len = %d", len(list))
	}
	list[0].Name = "Mutated"
	again := mgr.List()
	if again[0].Name != "A" {
		t.Errorf("internal state mutated: %s", again[0].Name)
	}
}

func TestManagerDelete(t *testing.T) {
	mgr := NewManager(t.TempDir())
	p := &Preset{Name: "Bye", ClientID: "123"}
	_ = mgr.Upsert(p)
	if err := mgr.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := mgr.Get(p.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestManagerLoadPersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	a := NewManager(dir)
	_ = a.Upsert(&Preset{Name: "Persist", ClientID: "123"})
	b := NewManager(dir)
	if err := b.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	list := b.List()
	if len(list) != 1 || list[0].Name != "Persist" {
		t.Errorf("not persisted: %+v", list)
	}
}

func TestManagerUpsertRejectsInvalid(t *testing.T) {
	mgr := NewManager(t.TempDir())
	if err := mgr.Upsert(&Preset{Name: ""}); !errors.Is(err, ErrInvalid) {
		t.Errorf("expected ErrInvalid, got %v", err)
	}
}
