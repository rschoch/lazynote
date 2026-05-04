package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	envNotesPath = "LAZYNOTE_PATH"
	appDirName   = "lazynote"
	notesFile    = "notes.json"

	lockPollInterval = 25 * time.Millisecond
	lockTimeout      = 10 * time.Second
	lockStaleAfter   = 5 * time.Minute
)

// Note is the persisted representation of a lazynote entry.
type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	Pinned    bool      `json:"pinned,omitempty"`
}

// Store persists notes as a small JSON file.
type Store struct {
	path string
}

// NewStore creates a Store backed by path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// DefaultPath returns the default notes file location.
func DefaultPath() (string, error) {
	if path := os.Getenv(envNotesPath); path != "" {
		return path, nil
	}

	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		base = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(base, appDirName, notesFile), nil
}

// Path returns the backing file path.
func (s *Store) Path() string {
	return s.path
}

// Load returns all persisted notes, newest last.
func (s *Store) Load() ([]Note, error) {
	return s.loadUnlocked()
}

func (s *Store) loadUnlocked() ([]Note, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read notes %s: %w", s.path, err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var loaded []Note
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("decode notes %s: %w", s.path, err)
	}

	return loaded, nil
}

// Append adds a new note and persists the full list.
func (s *Store) Append(title, body string) (Note, error) {
	var note Note
	err := s.withLock(func() error {
		loaded, err := s.loadUnlocked()
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		note = Note{
			ID:        newID(now, loaded),
			Title:     title,
			Body:      body,
			CreatedAt: now,
		}

		loaded = append(loaded, note)
		return s.saveUnlocked(loaded)
	})
	if err != nil {
		return Note{}, err
	}

	return note, nil
}

// Delete removes a note by ID and returns the updated list.
func (s *Store) Delete(id string) ([]Note, error) {
	var updated []Note
	err := s.withLock(func() error {
		loaded, err := s.loadUnlocked()
		if err != nil {
			return err
		}

		updated = loaded[:0]
		for _, note := range loaded {
			if note.ID != id {
				updated = append(updated, note)
			}
		}

		if len(updated) == len(loaded) {
			return nil
		}

		return s.saveUnlocked(updated)
	})
	if err != nil {
		return nil, err
	}

	return updated, nil
}

// Update replaces the title and body for a note by ID and returns the updated list.
func (s *Store) Update(id, title, body string) ([]Note, bool, error) {
	var updated []Note
	var changed bool
	err := s.withLock(func() error {
		loaded, err := s.loadUnlocked()
		if err != nil {
			return err
		}

		updated = append([]Note(nil), loaded...)
		for i := range updated {
			if updated[i].ID != id {
				continue
			}

			if updated[i].Title == title && updated[i].Body == body {
				return nil
			}
			updated[i].Title = title
			updated[i].Body = body
			changed = true
			return s.saveUnlocked(updated)
		}

		return fmt.Errorf("note not found: %s", id)
	})
	if err != nil {
		return nil, false, err
	}

	return updated, changed, nil
}

// TogglePinned flips the pinned state for a note by ID and returns the updated list.
func (s *Store) TogglePinned(id string) ([]Note, bool, error) {
	var updated []Note
	var pinned bool
	err := s.withLock(func() error {
		loaded, err := s.loadUnlocked()
		if err != nil {
			return err
		}

		updated = append([]Note(nil), loaded...)
		for i := range updated {
			if updated[i].ID != id {
				continue
			}

			updated[i].Pinned = !updated[i].Pinned
			pinned = updated[i].Pinned
			return s.saveUnlocked(updated)
		}

		return fmt.Errorf("note not found: %s", id)
	})
	if err != nil {
		return nil, false, err
	}

	return updated, pinned, nil
}

// Save replaces all persisted notes.
func (s *Store) Save(notes []Note) error {
	return s.withLock(func() error {
		return s.saveUnlocked(notes)
	})
}

func (s *Store) saveUnlocked(notes []Note) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create notes directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".notes-*.json")
	if err != nil {
		return fmt.Errorf("create temporary notes file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(notes); err != nil {
		tmp.Close()
		return fmt.Errorf("encode notes: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary notes file: %w", err)
	}
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("set notes file permissions: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace notes file: %w", err)
	}

	return nil
}

// Backup copies the raw notes file to target. If target is empty, a timestamped
// backup is created in a backups directory next to the notes file.
func (s *Store) Backup(target string) (string, error) {
	var backupPath string
	err := s.withLock(func() error {
		data, err := os.ReadFile(s.path)
		if errors.Is(err, os.ErrNotExist) {
			data = []byte("[]\n")
		} else if err != nil {
			return fmt.Errorf("read notes %s: %w", s.path, err)
		}

		backupPath, err = s.backupPath(target, time.Now().UTC())
		if err != nil {
			return err
		}
		return writeFileAtomic(backupPath, data, 0o600)
	})
	if err != nil {
		return "", err
	}
	return backupPath, nil
}

func (s *Store) backupPath(target string, now time.Time) (string, error) {
	name := "notes-" + now.Format("20060102-150405") + ".json"
	if target == "" {
		return filepath.Join(filepath.Dir(s.path), "backups", name), nil
	}

	info, err := os.Stat(target)
	if err == nil && info.IsDir() {
		return filepath.Join(target, name), nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("stat backup target: %w", err)
	}
	if filepath.Ext(target) == "" {
		return filepath.Join(target, name), nil
	}
	return target, nil
}

func (s *Store) withLock(fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create notes directory: %w", err)
	}

	lockPath := s.path + ".lock"
	deadline := time.Now().Add(lockTimeout)
	for {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, writeErr := fmt.Fprintf(lock, "pid=%d\ncreated_at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339Nano))
			closeErr := lock.Close()
			if writeErr != nil {
				_ = os.Remove(lockPath)
				return fmt.Errorf("write notes lock: %w", writeErr)
			}
			if closeErr != nil {
				_ = os.Remove(lockPath)
				return fmt.Errorf("close notes lock: %w", closeErr)
			}
			defer os.Remove(lockPath)
			return fn()
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("create notes lock: %w", err)
		}
		if err := removeStaleLock(lockPath, time.Now()); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for notes lock: %s", lockPath)
		}
		time.Sleep(lockPollInterval)
	}
}

func removeStaleLock(lockPath string, now time.Time) error {
	info, err := os.Stat(lockPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat notes lock: %w", err)
	}
	if now.Sub(info.ModTime()) < lockStaleAfter {
		return nil
	}
	if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale notes lock: %w", err)
	}
	return nil
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create target directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return fmt.Errorf("set file permissions: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("replace file: %w", err)
	}
	return nil
}

func newID(t time.Time, existing []Note) string {
	seen := make(map[string]struct{}, len(existing))
	for _, note := range existing {
		seen[note.ID] = struct{}{}
	}

	for {
		id := strconv.FormatInt(t.UnixNano(), 36)
		if _, ok := seen[id]; !ok {
			return id
		}
		t = t.Add(time.Nanosecond)
	}
}
