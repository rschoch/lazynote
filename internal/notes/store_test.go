package notes

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestStoreConcurrentAppendsKeepEveryNote(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "notes.json"))
	const count = 20

	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := store.Append("note", "body"); err != nil {
				t.Errorf("append %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if len(loaded) != count {
		t.Fatalf("loaded %d notes, want %d", len(loaded), count)
	}
	seen := map[string]struct{}{}
	for _, note := range loaded {
		if _, ok := seen[note.ID]; ok {
			t.Fatalf("duplicate ID %q in %#v", note.ID, loaded)
		}
		seen[note.ID] = struct{}{}
	}
}

func TestStoreUpdateChangesExistingNote(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "notes.json"))
	note, err := store.Append("old", "old body")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}

	updated, changed, err := store.Update(note.ID, "new", "new body")
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if updated[0].Title != "new" || updated[0].Body != "new body" {
		t.Fatalf("updated note = %#v, want new title and body", updated[0])
	}
}

func TestStoreBackupCopiesRawMalformedNotesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	store := NewStore(path)
	raw := []byte(`{"not":"valid notes"`)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create notes dir: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write malformed notes: %v", err)
	}

	backupPath, err := store.Backup(filepath.Join(t.TempDir(), "backup.json"))
	if err != nil {
		t.Fatalf("backup malformed notes: %v", err)
	}
	copied, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(copied) != string(raw) {
		t.Fatalf("backup = %q, want raw malformed contents %q", copied, raw)
	}
}

func TestStoreBackupTreatsExtensionlessTargetAsDirectory(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("release", "ship"); err != nil {
		t.Fatalf("append note: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "backups")
	backupPath, err := store.Backup(backupDir)
	if err != nil {
		t.Fatalf("backup to directory target: %v", err)
	}

	if filepath.Dir(backupPath) != backupDir {
		t.Fatalf("backup path = %q, want inside %q", backupPath, backupDir)
	}
	if filepath.Ext(backupPath) != ".json" {
		t.Fatalf("backup path = %q, want json backup file", backupPath)
	}
}

func TestStoreLoadMalformedNotesIncludesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create notes dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"not":"valid notes"`), 0o600); err != nil {
		t.Fatalf("write malformed notes: %v", err)
	}

	_, err := NewStore(path).Load()
	if err == nil {
		t.Fatal("Load returned nil error, want malformed JSON error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %q, want notes path", err)
	}
}
