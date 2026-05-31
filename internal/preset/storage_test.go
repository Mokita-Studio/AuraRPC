package preset

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	list, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if list != nil {
		t.Errorf("expected nil list, got %v", list)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := []*Preset{
		{ID: "a", Name: "First", ClientID: "1", Details: "hi"},
		{ID: "b", Name: "Second", ClientID: "123"},
	}
	if err := Save(dir, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	out, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(out) != 2 || out[0].ID != "a" || out[1].ID != "b" {
		t.Errorf("round-trip mismatch: %+v", out)
	}
}

func TestSaveCreatesBackup(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, []*Preset{{ID: "a", Name: "One", ClientID: "1"}}); err != nil {
		t.Fatal(err)
	}
	if err := Save(dir, []*Preset{{ID: "b", Name: "Two", ClientID: "2"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, fileName+".bak")); err != nil {
		t.Errorf("backup file missing: %v", err)
	}
}
