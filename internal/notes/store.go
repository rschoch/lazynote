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
)

// Note is the persisted representation of a lazynote entry.
type Note struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
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
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read notes: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	var loaded []Note
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("decode notes: %w", err)
	}

	return loaded, nil
}

// Append adds a new note and persists the full list.
func (s *Store) Append(title, body string) (Note, error) {
	loaded, err := s.Load()
	if err != nil {
		return Note{}, err
	}

	now := time.Now().UTC()
	note := Note{
		ID:        newID(now, loaded),
		Title:     title,
		Body:      body,
		CreatedAt: now,
	}

	loaded = append(loaded, note)
	if err := s.Save(loaded); err != nil {
		return Note{}, err
	}

	return note, nil
}

// Delete removes a note by ID and returns the updated list.
func (s *Store) Delete(id string) ([]Note, error) {
	loaded, err := s.Load()
	if err != nil {
		return nil, err
	}

	updated := loaded[:0]
	for _, note := range loaded {
		if note.ID != id {
			updated = append(updated, note)
		}
	}

	if len(updated) == len(loaded) {
		return loaded, nil
	}

	if err := s.Save(updated); err != nil {
		return nil, err
	}

	return updated, nil
}

// Save replaces all persisted notes.
func (s *Store) Save(notes []Note) error {
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
