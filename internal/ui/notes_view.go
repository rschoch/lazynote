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
	if a.allNotes == nil && len(a.notes) > 0 {
		a.allNotes = append([]notes.Note(nil), a.notes...)
	}
	source := a.sourceNotes()
	if a.searchIndex == nil {
		a.rebuildSearchIndex()
	}
	a.notes = filterNotes(source, a.filterQuery, a.searchIndex, a.notes[:0])
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

type normalizedNote struct {
	title string
	body  string
	tags  []string
}

func (a *App) rebuildSearchIndex() {
	source := a.sourceNotes()
	index := make(map[string]normalizedNote, len(source))
	for _, note := range source {
		tags := make([]string, len(note.Tags))
		for i, tag := range note.Tags {
			tags[i] = notes.FoldSearchText(tag)
		}
		index[note.ID] = normalizedNote{
			title: notes.FoldSearchText(note.Title),
			body:  notes.FoldSearchText(note.Body),
			tags:  tags,
		}
	}
	a.searchIndex = index
}

func filterNotes(source []notes.Note, query string, index map[string]normalizedNote, dst []notes.Note) []notes.Note {
	query = notes.FoldSearchText(strings.TrimSpace(query))
	if query == "" {
		return append(dst, source...)
	}

	filtered := dst
	for _, note := range source {
		if matchesNormalizedNote(index[note.ID], query) {
			filtered = append(filtered, note)
		}
	}
	return filtered
}

func matchesNormalizedNote(note normalizedNote, query string) bool {
	if strings.Contains(note.title, query) || strings.Contains(note.body, query) {
		return true
	}

	tagQuery := strings.TrimPrefix(query, "#")
	if tagQuery == "" {
		return false
	}
	for _, tag := range note.tags {
		if strings.Contains(tag, tagQuery) {
			return true
		}
	}
	return false
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
