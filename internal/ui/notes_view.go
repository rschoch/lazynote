package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rschoch/lazynote/internal/notes"
)

func (a *App) sourceNotes() []notes.Note {
	if a.allNotes != nil {
		return a.allNotes
	}
	return a.notes
}

func (a *App) orderedNotes(loaded []notes.Note) []notes.Note {
	ordered := make([]notes.Note, len(loaded))
	copy(ordered, loaded)
	switch a.settings.NoteOrder {
	case OrderNewestFirst:
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].Pinned != ordered[j].Pinned {
				return ordered[i].Pinned
			}
			return ordered[i].CreatedAt.After(ordered[j].CreatedAt)
		})
	default:
		sort.SliceStable(ordered, func(i, j int) bool {
			if ordered[i].Pinned != ordered[j].Pinned {
				return ordered[i].Pinned
			}
			return ordered[i].CreatedAt.Before(ordered[j].CreatedAt)
		})
	}
	return ordered
}

func (a *App) applyFilter(selectedID string) {
	source := a.sourceNotes()
	a.notes = filterNotes(source, a.filterQuery)
	if selectedID != "" {
		if index := noteIndexByID(a.notes, selectedID); index >= 0 {
			if a.selected != index {
				a.detailOffset = 0
			}
			a.selected = index
			a.markSelectedRead()
			return
		}
	}

	a.clampSelection()
	a.markSelectedRead()
	a.detailOffset = 0
}

func (a *App) setFilterQuery(query string) {
	selectedID := ""
	if note, ok := a.selectedNote(); ok {
		selectedID = note.ID
	}
	a.filterQuery = strings.TrimSpace(query)
	a.applyFilter(selectedID)
}

func (a *App) clearFilter() {
	if a.filterQuery == "" {
		return
	}
	a.setFilterQuery("")
	a.status = "Filter cleared"
	a.statusMode = statusMessage
}

func filterNotes(source []notes.Note, query string) []notes.Note {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return append([]notes.Note(nil), source...)
	}

	filtered := make([]notes.Note, 0, len(source))
	for _, note := range source {
		if notes.MatchesQuery(note, query) {
			filtered = append(filtered, note)
		}
	}
	return filtered
}

func addedNoteIDs(oldNotes, newNotes []notes.Note) map[string]struct{} {
	seen := make(map[string]struct{}, len(oldNotes))
	for _, note := range oldNotes {
		seen[note.ID] = struct{}{}
	}

	added := map[string]struct{}{}
	for _, note := range newNotes {
		if _, ok := seen[note.ID]; !ok {
			added[note.ID] = struct{}{}
		}
	}
	return added
}

func newestNoteID(loaded []notes.Note, ids map[string]struct{}) string {
	var newest notes.Note
	ok := false
	for _, note := range loaded {
		if _, included := ids[note.ID]; !included {
			continue
		}
		if !ok || note.CreatedAt.After(newest.CreatedAt) {
			newest = note
			ok = true
		}
	}
	return newest.ID
}

func newNotesStatus(count int) string {
	if count == 1 {
		return "1 new note"
	}
	return fmt.Sprintf("%d new notes", count)
}

func (a *App) addUnread(ids map[string]struct{}) {
	if len(ids) == 0 {
		return
	}
	if a.unreadIDs == nil {
		a.unreadIDs = map[string]struct{}{}
	}
	for id := range ids {
		a.unreadIDs[id] = struct{}{}
	}
}

func (a *App) isUnread(id string) bool {
	if a.unreadIDs == nil {
		return false
	}
	_, ok := a.unreadIDs[id]
	return ok
}

func (a *App) markSelectedRead() {
	if a.unreadIDs == nil {
		return
	}
	note, ok := a.selectedNote()
	if !ok {
		return
	}
	delete(a.unreadIDs, note.ID)
}
