package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/rschoch/lazynote/internal/notes"
)

const recentNoteLimit = 50

type noteViewKind int

const (
	viewActive noteViewKind = iota
	viewPinned
	viewRecent
	viewUntagged
	viewArchived
	viewTag
)

type noteView struct {
	kind noteViewKind
	tag  string
}

func (v noteView) label() string {
	switch v.kind {
	case viewPinned:
		return "Pinned"
	case viewRecent:
		return "Recent"
	case viewUntagged:
		return "Untagged"
	case viewArchived:
		return "Archived"
	case viewTag:
		return "#" + v.tag
	default:
		return "Active"
	}
}

func (v noteView) includes(note notes.Note) bool {
	switch v.kind {
	case viewPinned:
		return !note.Archived && note.Pinned
	case viewRecent:
		return !note.Archived
	case viewUntagged:
		return !note.Archived && len(note.Tags) == 0
	case viewArchived:
		return note.Archived
	case viewTag:
		return !note.Archived && containsTag(note.Tags, v.tag)
	default:
		return !note.Archived
	}
}

func containsTag(tags []string, target string) bool {
	for _, tag := range tags {
		if tag == target {
			return true
		}
	}
	return false
}

func (a *App) availableViews() []noteView {
	views := []noteView{
		{kind: viewActive},
		{kind: viewPinned},
		{kind: viewRecent},
		{kind: viewUntagged},
		{kind: viewArchived},
	}
	tags := a.knownTags(false)
	if a.currentView.kind == viewTag && !containsTag(tags, a.currentView.tag) {
		tags = append(tags, a.currentView.tag)
		sort.Strings(tags)
	}
	for _, tag := range tags {
		views = append(views, noteView{kind: viewTag, tag: tag})
	}
	return views
}

func (a *App) knownTags(includeArchived bool) []string {
	seen := map[string]struct{}{}
	for _, note := range a.sourceNotes() {
		if note.Archived && !includeArchived {
			continue
		}
		for _, tag := range note.Tags {
			seen[tag] = struct{}{}
		}
	}
	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func (a *App) viewCount(view noteView) int {
	count := 0
	for _, note := range a.sourceNotes() {
		if view.includes(note) {
			count++
		}
	}
	if view.kind == viewRecent && count > recentNoteLimit {
		return recentNoteLimit
	}
	return count
}

func (a *App) openViewPicker(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() || a.inputMode != inputNormal {
		return nil
	}
	views := a.availableViews()
	lines := make([]string, len(views))
	selected := 0
	for i, view := range views {
		lines[i] = fmt.Sprintf("%-18s %d", view.label(), a.viewCount(view))
		if view == a.currentView {
			selected = i
		}
	}
	a.openPopup(Popup{
		Title:    "Views: Enter select",
		Lines:    lines,
		Selected: selected,
		OnSelect: func(g *gocui.Gui, index int) error {
			if index < 0 || index >= len(views) {
				return nil
			}
			a.currentView = views[index]
			a.applyFilter("")
			a.status = "View: " + a.currentView.label()
			a.statusMode = statusMessage
			return a.closePopup(g)
		},
	})
	return a.setCurrentView(g)
}

func (a *App) toggleArchive(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() || a.inputMode != inputNormal {
		return nil
	}
	note, ok := a.selectedNote()
	if !ok {
		return nil
	}
	archived := !note.Archived
	updated, _, err := a.store.SetArchived(note.ID, archived)
	if err != nil {
		a.status = fmt.Sprintf("Archive failed: %v", err)
		a.statusMode = statusMessage
		return nil
	}
	a.applyLoadedNotes(updated, "")
	if archived {
		a.status = fmt.Sprintf("Archived %q", note.Title)
	} else {
		a.status = fmt.Sprintf("Restored %q", note.Title)
	}
	a.statusMode = statusMessage
	return nil
}

func (a *App) openTagPicker(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() || a.inputMode != inputNormal {
		return nil
	}
	note, ok := a.selectedNote()
	if !ok {
		a.status = "Nothing to tag"
		a.statusMode = statusMessage
		return nil
	}

	tags := a.knownTags(true)
	selectedTags := make(map[string]bool, len(note.Tags))
	for _, tag := range note.Tags {
		selectedTags[tag] = true
		if !containsTag(tags, tag) {
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)
	counts := tagCounts(a.sourceNotes())
	lines := make([]string, len(tags))
	refreshLines := func() {
		for i, tag := range tags {
			mark := " "
			if selectedTags[tag] {
				mark = "x"
			}
			lines[i] = fmt.Sprintf("[%s] #%-20s %d", mark, tag, counts[tag])
		}
	}
	chosenTags := func() []string {
		chosen := make([]string, 0, len(selectedTags))
		for _, tag := range tags {
			if selectedTags[tag] {
				chosen = append(chosen, tag)
			}
		}
		return chosen
	}
	refreshLines()

	a.openPopup(Popup{
		Title: "Tags: Space toggle, Enter save, n new",
		Lines: lines,
		OnToggle: func(g *gocui.Gui, index int) error {
			if index >= 0 && index < len(tags) {
				selectedTags[tags[index]] = !selectedTags[tags[index]]
				refreshLines()
			}
			return nil
		},
		OnSelect: func(g *gocui.Gui, index int) error {
			updated, changed, err := a.store.SetTags(note.ID, chosenTags())
			if err != nil {
				a.status = fmt.Sprintf("Tag save failed: %v", err)
				a.statusMode = statusMessage
				return a.closePopup(g)
			}
			if changed {
				a.applyLoadedNotes(updated, "Tags saved")
			} else {
				a.status = "Tags unchanged"
				a.statusMode = statusMessage
			}
			return a.closePopup(g)
		},
		OnNew: func(g *gocui.Gui) error {
			updated, changed, err := a.store.SetTags(note.ID, chosenTags())
			if err != nil {
				a.status = fmt.Sprintf("Tag save failed: %v", err)
				a.statusMode = statusMessage
				return a.closePopup(g)
			}
			if changed {
				a.applyLoadedNotes(updated, "")
			}
			a.tagTargetID = note.ID
			a.tagInput = ""
			a.inputMode = inputTag
			return a.closePopup(g)
		},
	})
	return a.setCurrentView(g)
}

func (a *App) confirmNewTag(g *gocui.Gui) error {
	tag := normalizedNewTag(a.tagInput)
	targetID := a.tagTargetID
	a.inputMode = inputNormal
	a.tagInput = ""
	a.tagTargetID = ""
	if tag == "" {
		a.status = "Tag cannot be empty"
		a.statusMode = statusMessage
		return a.setCurrentView(g)
	}
	updated, changed, err := a.store.AddTags(targetID, []string{tag})
	if err != nil {
		a.status = fmt.Sprintf("Tag save failed: %v", err)
		a.statusMode = statusMessage
		return a.setCurrentView(g)
	}
	if changed {
		a.applyLoadedNotes(updated, "Added #"+tag)
	} else {
		a.status = "Tag already present"
		a.statusMode = statusMessage
	}
	return a.setCurrentView(g)
}

func tagCounts(loaded []notes.Note) map[string]int {
	counts := map[string]int{}
	for _, note := range loaded {
		if note.Archived {
			continue
		}
		for _, tag := range note.Tags {
			counts[tag]++
		}
	}
	return counts
}

func normalizedNewTag(input string) string {
	tags := notes.NormalizeTags([]string{strings.TrimSpace(input)})
	if len(tags) == 0 {
		return ""
	}
	return tags[0]
}
