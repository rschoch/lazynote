package notes

import (
	"path/filepath"
	"testing"
)

func TestStoreAppendLoadDelete(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "notes.json"))

	first, err := store.Append("first", "remember the first thing")
	if err != nil {
		t.Fatalf("append first note: %v", err)
	}
	if _, err := store.Append("second", "remember the second thing"); err != nil {
		t.Fatalf("append second note: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d notes, want 2", len(loaded))
	}
	if loaded[0].Title != "first" || loaded[1].Body != "remember the second thing" {
		t.Fatalf("loaded unexpected notes: %#v", loaded)
	}

	updated, err := store.Delete(first.ID)
	if err != nil {
		t.Fatalf("delete note: %v", err)
	}
	if len(updated) != 1 {
		t.Fatalf("kept %d notes, want 1", len(updated))
	}
	if updated[0].Title != "second" {
		t.Fatalf("kept unexpected note: %#v", updated[0])
	}
}
