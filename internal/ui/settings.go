package ui

import "time"

const (
	OrderOldestFirst NoteOrder = "oldest-first"
	OrderNewestFirst NoteOrder = "newest-first"
)

// NoteOrder controls how notes are presented in the TUI.
type NoteOrder string

// Settings controls TUI behavior that is not part of the color theme.
type Settings struct {
	RefreshInterval    time.Duration
	NoteOrder          NoteOrder
	AutoSelectNewNotes bool
}

// DefaultSettings returns the default terminal UI behavior.
func DefaultSettings() Settings {
	return Settings{
		RefreshInterval: time.Second,
		NoteOrder:       OrderOldestFirst,
	}
}

func (s Settings) normalized() Settings {
	defaults := DefaultSettings()
	if s.RefreshInterval <= 0 {
		s.RefreshInterval = defaults.RefreshInterval
	}
	if s.NoteOrder == "" {
		s.NoteOrder = defaults.NoteOrder
	}
	return s
}
