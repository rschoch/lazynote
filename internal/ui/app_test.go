package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rschoch/lazynote/internal/notes"
)

func TestDeleteSelectedNoteRequiresConfirmation(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("first", "first body"); err != nil {
		t.Fatalf("append first note: %v", err)
	}
	second, err := store.Append("second", "second body")
	if err != nil {
		t.Fatalf("append second note: %v", err)
	}

	app := loadedApp(t, store)
	app.selected = 1

	if err := app.delete(nil, nil); err != nil {
		t.Fatalf("delete first press: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes after first delete: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("kept %d notes after first delete, want 2", len(loaded))
	}
	if app.pendingDeleteID != second.ID {
		t.Fatalf("pendingDeleteID = %q, want %q", app.pendingDeleteID, second.ID)
	}
	if !strings.Contains(app.status, "Press d again") {
		t.Fatalf("status = %q, want delete confirmation", app.status)
	}

	if err := app.delete(nil, nil); err != nil {
		t.Fatalf("delete second press: %v", err)
	}

	loaded, err = store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("kept %d notes, want 1", len(loaded))
	}
	if loaded[0].ID == second.ID {
		t.Fatalf("deleted wrong note: %#v", loaded[0])
	}
}

func TestDeleteOnlyNoteClearsVisibleNotes(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("only note", "only body"); err != nil {
		t.Fatalf("append note: %v", err)
	}

	app := loadedApp(t, store)

	if err := app.delete(nil, nil); err != nil {
		t.Fatalf("delete first press: %v", err)
	}
	if err := app.delete(nil, nil); err != nil {
		t.Fatalf("delete second press: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded %d notes, want empty store", len(loaded))
	}
	if len(app.notes) != 0 {
		t.Fatalf("visible notes = %d, want empty list", len(app.notes))
	}
	if len(app.sourceNotes()) != 0 {
		t.Fatalf("source notes = %d, want empty source", len(app.sourceNotes()))
	}
	if _, ok := app.selectedNote(); ok {
		t.Fatal("selectedNote ok = true, want no selected note")
	}
}

func TestSelectionCancelsDeleteConfirmation(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("first", "first body"); err != nil {
		t.Fatalf("append first note: %v", err)
	}
	if _, err := store.Append("second", "second body"); err != nil {
		t.Fatalf("append second note: %v", err)
	}

	app := loadedApp(t, store)

	if err := app.delete(nil, nil); err != nil {
		t.Fatalf("delete first press: %v", err)
	}
	if app.pendingDeleteID == "" {
		t.Fatal("pendingDeleteID is empty, want armed delete")
	}

	if err := app.down(nil, nil); err != nil {
		t.Fatalf("down: %v", err)
	}
	if app.pendingDeleteID != "" {
		t.Fatalf("pendingDeleteID = %q, want canceled delete", app.pendingDeleteID)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("kept %d notes, want 2", len(loaded))
	}
}

func TestDetailPaneScrollsLongNote(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	body := strings.Repeat("long paragraph text that wraps across the detail pane\n\n", 20)
	if _, err := store.Append("long note", body); err != nil {
		t.Fatalf("append note: %v", err)
	}

	app := loadedApp(t, store)
	app.activePane = paneDetail
	note, _ := app.selectedNote()
	app.scrollDetailBy(note, 1, 30, 5)
	if app.detailOffset == 0 {
		t.Fatal("detailOffset = 0, want arrow down to scroll active detail pane")
	}

	app.scrollDetailBy(note, 5, 30, 5)

	if app.detailOffset <= 1 {
		t.Fatalf("detailOffset = %d, want page down to scroll farther", app.detailOffset)
	}

	app.scrollDetailBy(note, -100, 30, 5)
	if app.detailOffset != 0 {
		t.Fatalf("detailOffset = %d, want clamped to top", app.detailOffset)
	}
}

func TestDetailMetricsCacheTracksContentAndDimensions(t *testing.T) {
	app := &App{}
	note := notes.Note{ID: "one", Body: strings.Repeat("wrapped body ", 20)}
	initial := app.cachedDetailMaxOffset(note, 20, 5)
	if initial <= 0 {
		t.Fatalf("initial max offset = %d, want wrapped content", initial)
	}

	app.detailMetrics.maxOffset = 777
	if got := app.cachedDetailMaxOffset(note, 20, 5); got != 777 {
		t.Fatalf("cached max offset = %d, want cached value", got)
	}
	if got := app.cachedDetailMaxOffset(note, 40, 5); got == 777 {
		t.Fatal("width change reused stale detail metrics")
	}

	app.detailMetrics.maxOffset = 777
	note.Body += " changed"
	if got := app.cachedDetailMaxOffset(note, 40, 5); got == 777 {
		t.Fatal("body change reused stale detail metrics")
	}

	app.detailMetrics.maxOffset = 777
	note.ID = "two"
	if got := app.cachedDetailMaxOffset(note, 40, 5); got == 777 {
		t.Fatal("selection change reused stale detail metrics")
	}
}

func TestFocusControlsContextualUpDown(t *testing.T) {
	app := &App{
		notes: []notes.Note{
			{Title: "first", Body: strings.Repeat("first body line with enough repeated content\n", 80)},
			{Title: "second", Body: "second body"},
		},
		activePane:   paneDetail,
		detailOffset: 8,
	}

	if err := app.focusNotes(nil, nil); err != nil {
		t.Fatalf("focus notes: %v", err)
	}
	if err := app.down(nil, nil); err != nil {
		t.Fatalf("down: %v", err)
	}
	if app.selected != 1 {
		t.Fatalf("selected = %d, want notes pane to move selection", app.selected)
	}
	if app.detailOffset != 0 {
		t.Fatalf("detailOffset = %d, want selection change to reset detail scroll", app.detailOffset)
	}
}

func TestNoteSelectionWrapsAtListBoundaries(t *testing.T) {
	app := &App{
		notes: []notes.Note{
			{Title: "first"},
			{Title: "second"},
			{Title: "third"},
		},
		activePane: paneNotes,
	}

	if err := app.up(nil, nil); err != nil {
		t.Fatalf("up: %v", err)
	}
	if app.selected != 2 {
		t.Fatalf("selected = %d, want up from first note to wrap to last", app.selected)
	}

	if err := app.down(nil, nil); err != nil {
		t.Fatalf("down: %v", err)
	}
	if app.selected != 0 {
		t.Fatalf("selected = %d, want down from last note to wrap to first", app.selected)
	}
}

func TestNoteSelectionOnEmptyListDoesNothing(t *testing.T) {
	app := &App{activePane: paneNotes}

	if err := app.up(nil, nil); err != nil {
		t.Fatalf("up: %v", err)
	}
	if err := app.down(nil, nil); err != nil {
		t.Fatalf("down: %v", err)
	}
	if app.selected != 0 {
		t.Fatalf("selected = %d, want empty list selection unchanged", app.selected)
	}
}

func TestListViewportFollowsSelection(t *testing.T) {
	tests := []struct {
		name               string
		total, selected    int
		offset, height     int
		start, end, cursor int
	}{
		{name: "first page", total: 100, selected: 0, height: 10, start: 0, end: 10, cursor: 0},
		{name: "move below page", total: 100, selected: 10, height: 10, start: 1, end: 11, cursor: 9},
		{name: "last page", total: 100, selected: 99, height: 10, start: 90, end: 100, cursor: 9},
		{name: "wrap to first", total: 100, selected: 0, offset: 90, height: 10, start: 0, end: 10, cursor: 0},
		{name: "short list", total: 3, selected: 2, height: 10, start: 0, end: 3, cursor: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			start, end, cursor := listViewport(test.total, test.selected, test.offset, test.height)
			if start != test.start || end != test.end || cursor != test.cursor {
				t.Fatalf("listViewport() = (%d, %d, %d), want (%d, %d, %d)", start, end, cursor, test.start, test.end, test.cursor)
			}
		})
	}
}

func TestSelectionResetsDetailScroll(t *testing.T) {
	app := &App{
		notes: []notes.Note{
			{Title: "one"},
			{Title: "two"},
		},
		detailOffset: 8,
	}

	if err := app.down(nil, nil); err != nil {
		t.Fatalf("down: %v", err)
	}
	if app.detailOffset != 0 {
		t.Fatalf("detailOffset = %d, want reset after selection change", app.detailOffset)
	}
}

func TestStatusLineIncludesPositionAndKeys(t *testing.T) {
	createdAt := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	app := &App{
		notes: []notes.Note{
			{Title: "one", CreatedAt: createdAt},
			{Title: "two", CreatedAt: createdAt.Add(time.Hour)},
		},
		selected: 1,
	}

	got := app.statusLine()
	if strings.Contains(got, "2026") {
		t.Fatalf("statusLine() = %q, want no selected-note timestamp", got)
	}
	for _, want := range []string{"2/2", "↑↓ nav", "→ body", "new", "pin", "? help", "delete", "quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	}
	if strings.Contains(got, "…") {
		t.Fatalf("statusLine() = %q, want no ellipsis in full hints", got)
	}
}

func TestStatusLineUsesCompactHintsWhenNarrow(t *testing.T) {
	app := &App{
		notes: []notes.Note{
			{Title: "one"},
		},
	}

	width := 48
	got := app.statusLineForWidth(width)
	if runeLen(got) > width {
		t.Fatalf("statusLineForWidth() length = %d, want at most %d: %q", runeLen(got), width, got)
	}
	if strings.Contains(got, "nav") || strings.Contains(got, "copy") || strings.Contains(got, "quit") {
		t.Fatalf("statusLineForWidth() = %q, want compact hints", got)
	}
	for _, want := range []string{"1/1", "↑↓", "→", "/", "p", "q"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLineForWidth() = %q, want %q", got, want)
		}
	}
}

func TestStatusLineUsesSmartHintsBeforeCompact(t *testing.T) {
	app := &App{
		notes: []notes.Note{{Title: "one"}},
	}

	got := app.statusLineForWidth(80)
	if runeLen(got) > 80 {
		t.Fatalf("statusLineForWidth() length = %d, want at most 80: %q", runeLen(got), got)
	}
	for _, want := range []string{"↑↓ nav", "→ body", "/ filter", "views", "new", "? help"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLineForWidth() = %q, want smart hint %q", got, want)
		}
	}
	if strings.Contains(got, "archive") || strings.Contains(got, "reload") {
		t.Fatalf("statusLineForWidth() = %q, want reduced smart hints", got)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), "…") {
		t.Fatalf("statusLineForWidth() = %q, want permanent smart-tier ellipsis", got)
	}
}

func TestStatusLineIncludesEmptyState(t *testing.T) {
	app := &App{}

	got := app.statusLine()
	for _, want := range []string{"0/0", "quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	}
}

func TestStatusLineIncludesDetailScrollOffset(t *testing.T) {
	app := &App{
		notes: []notes.Note{
			{Title: "one", CreatedAt: time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)},
		},
		activePane:   paneDetail,
		detailOffset: 4,
	}

	got := app.statusLine()
	for _, want := range []string{"1/1", "scroll +4", "↑↓ scroll", "Pg page", "← list", "copy"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	}
}

func TestStatusLineIncludesDeleteConfirmationHints(t *testing.T) {
	app := &App{
		status:     "Press d again to delete \"one\"",
		statusMode: statusDeleteArmed,
	}

	got := app.statusLine()
	for _, want := range []string{"Press d again", "delete", "↑↓ cancel", "? help", "quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	}
}

func TestStatusLineIncludesMessageHints(t *testing.T) {
	app := &App{
		notes: []notes.Note{
			{Title: "one", Body: "body"},
		},
		status:     "Deleted \"one\"",
		statusMode: statusMessage,
	}

	got := app.statusLine()
	for _, want := range []string{"Deleted", "↑↓ nav", "→ body", "copy", "quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	}
}

func TestStatusLineHighlightsShortcutInitials(t *testing.T) {
	app := &App{
		theme: DefaultTheme(),
		notes: []notes.Note{{Title: "one"}},
	}

	rendered := app.renderStatusLine(app.statusLine())
	accent := ansiAttribute(DefaultTheme().ActiveBorder)
	for _, word := range []string{"copy", "pin", "archive", "views", "quit"} {
		runes := []rune(word)
		want := accent + string(runes[0]) + "\x1b[0m" + string(runes[1:])
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered status = %q, want highlighted %q", rendered, word)
		}
	}
	if !strings.Contains(rendered, "? help") {
		t.Fatalf("rendered status = %q, want unchanged help hint", rendered)
	}
}

func TestStatusHintsReflectSelectedNoteState(t *testing.T) {
	app := &App{notes: []notes.Note{{Title: "one", Pinned: true}}}
	if got := app.statusHints(); !strings.Contains(got, "unpin  tags  archive") {
		t.Fatalf("pinned status hints = %q, want unpin action", got)
	}

	app.notes[0].Archived = true
	app.notes[0].Pinned = false
	if got := app.statusHints(); !strings.Contains(got, "tags  unarchive") || strings.Contains(got, "pin  tags") {
		t.Fatalf("archived status hints = %q, want unarchive without pin", got)
	}
}

func TestContextualStatusHintsHighlightActualKeys(t *testing.T) {
	accent := ansiAttribute(DefaultTheme().ActiveBorder)
	app := &App{
		theme: DefaultTheme(),
		notes: []notes.Note{{Title: "one", Pinned: true}},
	}

	rendered := app.renderStatusLine(app.statusLine())
	if want := "un" + accent + "p\x1b[0min"; !strings.Contains(rendered, want) {
		t.Fatalf("pinned status = %q, want embedded p highlighted", rendered)
	}

	app.notes[0].Pinned = false
	app.notes[0].Archived = true
	rendered = app.renderStatusLine(app.statusLine())
	if want := "un" + accent + "a\x1b[0mrchive"; !strings.Contains(rendered, want) {
		t.Fatalf("archived status = %q, want embedded a highlighted", rendered)
	}
}

func TestEmptyNotesMessageDescribesCurrentView(t *testing.T) {
	tests := []struct {
		view noteView
		want string
	}{
		{noteView{kind: viewActive}, "No active notes"},
		{noteView{kind: viewPinned}, "No pinned notes"},
		{noteView{kind: viewRecent}, "No recent notes"},
		{noteView{kind: viewUntagged}, "No untagged notes"},
		{noteView{kind: viewArchived}, "No archived notes"},
		{noteView{kind: viewTag, tag: "work"}, "No notes tagged #work"},
	}
	for _, tt := range tests {
		app := &App{currentView: tt.view}
		if got := app.emptyNotesMessage(); got != tt.want {
			t.Fatalf("emptyNotesMessage(%#v) = %q, want %q", tt.view, got, tt.want)
		}
	}

	app := &App{currentView: noteView{kind: viewArchived}, filterQuery: "missing"}
	if got := app.emptyNotesMessage(); got != "No matches" {
		t.Fatalf("filtered emptyNotesMessage() = %q, want No matches", got)
	}
}

func TestCopyCopiesTitleFromNotesPane(t *testing.T) {
	var copied string
	app := &App{
		notes: []notes.Note{
			{Title: "release plan", Body: "ship packages"},
		},
		copyText: func(text string) error {
			copied = text
			return nil
		},
	}

	if err := app.copy(nil, nil); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if copied != "release plan" {
		t.Fatalf("copied = %q, want title", copied)
	}
	if app.status != "Copied title" {
		t.Fatalf("status = %q, want copied title", app.status)
	}
}

func TestCopyCopiesBodyFromDetailPane(t *testing.T) {
	var copied string
	app := &App{
		notes: []notes.Note{
			{Title: "release plan", Body: "ship packages"},
		},
		activePane: paneDetail,
		copyText: func(text string) error {
			copied = text
			return nil
		},
	}

	if err := app.copy(nil, nil); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if copied != "ship packages" {
		t.Fatalf("copied = %q, want body", copied)
	}
	if app.status != "Copied body" {
		t.Fatalf("status = %q, want copied body", app.status)
	}
}

func TestCopyHandlesEmptyNotes(t *testing.T) {
	app := &App{}

	if err := app.copy(nil, nil); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if app.status != "Nothing to copy" {
		t.Fatalf("status = %q, want nothing to copy", app.status)
	}
}

func TestReloadNotesFromDiskPicksUpExternalAppend(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	first, err := store.Append("first", "first body")
	if err != nil {
		t.Fatalf("append first note: %v", err)
	}
	if _, err := store.Append("second", "second body"); err != nil {
		t.Fatalf("append second note: %v", err)
	}

	app := loadedApp(t, store)
	app.selected = 0
	if _, err := store.Append("third", "third body"); err != nil {
		t.Fatalf("append external note: %v", err)
	}

	if err := app.reloadNotesFromDisk("Notes updated"); err != nil {
		t.Fatalf("reload notes: %v", err)
	}

	if len(app.notes) != 3 {
		t.Fatalf("loaded %d notes, want 3", len(app.notes))
	}
	if app.notes[app.selected].ID != first.ID {
		t.Fatalf("selected note = %q, want original selected note %q", app.notes[app.selected].ID, first.ID)
	}
	if app.status != "1 new note" {
		t.Fatalf("status = %q, want new note count", app.status)
	}
	if !app.isUnread(app.notes[2].ID) {
		t.Fatalf("new note %q is not marked unread", app.notes[2].ID)
	}
}

func TestReloadNotesFromDiskLeavesStatusWhenUnchanged(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("first", "first body"); err != nil {
		t.Fatalf("append first note: %v", err)
	}

	app := loadedApp(t, store)
	app.status = "Copied title"
	app.statusMode = statusMessage

	if err := app.reloadNotesFromDisk("Notes updated"); err != nil {
		t.Fatalf("reload notes: %v", err)
	}

	if app.status != "Copied title" {
		t.Fatalf("status = %q, want unchanged status", app.status)
	}
}

func TestReloadNotesFromDiskClampsDeletedSelection(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	first, err := store.Append("first", "first body")
	if err != nil {
		t.Fatalf("append first note: %v", err)
	}
	second, err := store.Append("second", "second body")
	if err != nil {
		t.Fatalf("append second note: %v", err)
	}

	app := loadedApp(t, store)
	app.selected = 0
	app.detailOffset = 4
	app.pendingDeleteID = first.ID
	app.statusMode = statusDeleteArmed
	if _, _, err := store.Delete(first.ID); err != nil {
		t.Fatalf("delete external note: %v", err)
	}

	if err := app.reloadNotesFromDisk("Notes updated"); err != nil {
		t.Fatalf("reload notes: %v", err)
	}

	if len(app.notes) != 1 {
		t.Fatalf("loaded %d notes, want 1", len(app.notes))
	}
	if app.notes[app.selected].ID != second.ID {
		t.Fatalf("selected note = %q, want remaining note %q", app.notes[app.selected].ID, second.ID)
	}
	if app.detailOffset != 0 {
		t.Fatalf("detailOffset = %d, want reset after selected note was removed", app.detailOffset)
	}
	if app.pendingDeleteID != "" {
		t.Fatalf("pendingDeleteID = %q, want cleared", app.pendingDeleteID)
	}
}

func TestReloadNotesFromDiskClearsLastDeletedNote(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	note, err := store.Append("only note", "only body")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}

	app := loadedApp(t, store)
	if _, _, err := store.Delete(note.ID); err != nil {
		t.Fatalf("delete external note: %v", err)
	}

	if err := app.reloadNotesFromDisk("Notes updated"); err != nil {
		t.Fatalf("reload notes: %v", err)
	}

	if len(app.notes) != 0 {
		t.Fatalf("visible notes = %d, want empty list", len(app.notes))
	}
	if len(app.sourceNotes()) != 0 {
		t.Fatalf("source notes = %d, want empty source", len(app.sourceNotes()))
	}
	if _, ok := app.selectedNote(); ok {
		t.Fatal("selectedNote ok = true, want no selected note")
	}
}

func TestReloadNotesFromDiskClearsDeletedNotesFile(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("only note", "only body"); err != nil {
		t.Fatalf("append note: %v", err)
	}

	app := loadedApp(t, store)
	if err := os.Remove(store.Path()); err != nil {
		t.Fatalf("remove notes file: %v", err)
	}

	if err := app.reloadNotesFromDisk("Notes updated"); err != nil {
		t.Fatalf("reload notes: %v", err)
	}

	if len(app.notes) != 0 {
		t.Fatalf("visible notes = %d, want empty list", len(app.notes))
	}
	if len(app.sourceNotes()) != 0 {
		t.Fatalf("source notes = %d, want empty source", len(app.sourceNotes()))
	}
	if _, ok := app.selectedNote(); ok {
		t.Fatal("selectedNote ok = true, want no selected note")
	}
}

func TestReloadNotesFromDiskClearsDeletedFilteredMatch(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	work, err := store.AppendWithTags("work note", "body", []string{"work"})
	if err != nil {
		t.Fatalf("append work note: %v", err)
	}
	home, err := store.AppendWithTags("home note", "body", []string{"home"})
	if err != nil {
		t.Fatalf("append home note: %v", err)
	}

	app := loadedApp(t, store)
	app.setFilterQuery("#work")
	if len(app.notes) != 1 || app.notes[0].ID != work.ID {
		t.Fatalf("filtered notes = %#v, want work note", app.notes)
	}
	if _, _, err := store.Delete(work.ID); err != nil {
		t.Fatalf("delete external filtered note: %v", err)
	}

	if err := app.reloadNotesFromDisk("Notes updated"); err != nil {
		t.Fatalf("reload notes: %v", err)
	}

	if len(app.sourceNotes()) != 1 || app.sourceNotes()[0].ID != home.ID {
		t.Fatalf("source notes = %#v, want remaining home note", app.sourceNotes())
	}
	if len(app.notes) != 0 {
		t.Fatalf("visible notes = %d, want no filtered matches", len(app.notes))
	}
	if _, ok := app.selectedNote(); ok {
		t.Fatal("selectedNote ok = true, want no selected note")
	}
	if app.filterQuery != "#work" {
		t.Fatalf("filterQuery = %q, want preserved filter", app.filterQuery)
	}
}

func TestDeleteStaleSelectedNoteRefreshesVisibleState(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	note, err := store.Append("stale note", "body")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}

	app := loadedApp(t, store)
	if _, _, err := store.Delete(note.ID); err != nil {
		t.Fatalf("delete external note: %v", err)
	}

	if err := app.delete(nil, nil); err != nil {
		t.Fatalf("delete first press: %v", err)
	}
	if err := app.delete(nil, nil); err != nil {
		t.Fatalf("delete second press: %v", err)
	}

	if len(app.notes) != 0 {
		t.Fatalf("visible notes = %d, want empty list", len(app.notes))
	}
	if len(app.sourceNotes()) != 0 {
		t.Fatalf("source notes = %d, want empty source", len(app.sourceNotes()))
	}
	if _, ok := app.selectedNote(); ok {
		t.Fatal("selectedNote ok = true, want no selected note")
	}
	if !strings.Contains(app.status, "already deleted") {
		t.Fatalf("status = %q, want already deleted message", app.status)
	}
}

func TestFilterMatchesTitleAndBody(t *testing.T) {
	app := &App{
		allNotes: []notes.Note{
			{ID: "one", Title: "release plan", Body: "ship packages"},
			{ID: "two", Title: "grocery list", Body: "buy eggs"},
		},
	}

	app.setFilterQuery("PACKAGES")

	if len(app.notes) != 1 {
		t.Fatalf("filtered %d notes, want 1", len(app.notes))
	}
	if app.notes[0].ID != "one" {
		t.Fatalf("filtered note = %q, want one", app.notes[0].ID)
	}
}

func TestFilterMatchesTags(t *testing.T) {
	app := &App{
		allNotes: []notes.Note{
			{ID: "one", Title: "release plan", Tags: []string{"work"}},
			{ID: "two", Title: "grocery list", Tags: []string{"home"}},
		},
	}

	app.setFilterQuery("#work")

	if len(app.notes) != 1 {
		t.Fatalf("filtered %d notes, want 1", len(app.notes))
	}
	if app.notes[0].ID != "one" {
		t.Fatalf("filtered note = %q, want one", app.notes[0].ID)
	}
}

func TestFilterIgnoresAccents(t *testing.T) {
	app := &App{allNotes: []notes.Note{
		{ID: "one", Title: "Café planning", Body: "Résumé for Zoë", Tags: []string{"déjà-vu"}},
		{ID: "two", Title: "unrelated"},
	}}

	for _, query := range []string{"cafe", "RESUME", "zoe", "#deja-vu"} {
		app.setFilterQuery(query)
		if len(app.notes) != 1 || app.notes[0].ID != "one" {
			t.Fatalf("filter %q returned %#v, want accent-insensitive match", query, app.notes)
		}
	}
}

func TestSearchIndexRefreshesWhenNotesChange(t *testing.T) {
	app := &App{settings: DefaultSettings()}
	app.applyLoadedNotes([]notes.Note{{ID: "one", Title: "old title", Body: "old body"}}, "")
	app.setFilterQuery("old body")
	if len(app.notes) != 1 {
		t.Fatalf("initial filter returned %d notes, want 1", len(app.notes))
	}

	app.applyLoadedNotes([]notes.Note{{ID: "one", Title: "new title", Body: "new body"}}, "")
	if len(app.notes) != 0 {
		t.Fatalf("stale filter returned %d notes after update, want 0", len(app.notes))
	}

	app.setFilterQuery("NEW BODY")
	if len(app.notes) != 1 || app.notes[0].ID != "one" {
		t.Fatalf("updated filter returned %#v, want updated note", app.notes)
	}
}

func TestClearFilterRestoresNotes(t *testing.T) {
	app := &App{
		allNotes: []notes.Note{
			{ID: "one", Title: "release plan"},
			{ID: "two", Title: "grocery list"},
		},
	}
	app.setFilterQuery("release")

	app.clearFilter()

	if len(app.notes) != 2 {
		t.Fatalf("notes = %d, want restored list", len(app.notes))
	}
	if app.filterQuery != "" {
		t.Fatalf("filterQuery = %q, want cleared", app.filterQuery)
	}
}

func TestFilterPreservesSourceWhenInitializedWithVisibleNotes(t *testing.T) {
	app := &App{notes: []notes.Note{
		{ID: "one", Title: "release plan"},
		{ID: "two", Title: "grocery list"},
	}}

	app.setFilterQuery("release")
	app.clearFilter()

	if len(app.notes) != 2 {
		t.Fatalf("notes = %d, want restored source list", len(app.notes))
	}
}

func TestApplyLoadedNotesReportsNewNotesWithoutMovingSelection(t *testing.T) {
	createdAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	app := &App{
		allNotes: []notes.Note{
			{ID: "one", Title: "one", CreatedAt: createdAt},
		},
		notes: []notes.Note{
			{ID: "one", Title: "one", CreatedAt: createdAt},
		},
		settings: DefaultSettings(),
	}

	app.applyLoadedNotes([]notes.Note{
		{ID: "one", Title: "one", CreatedAt: createdAt},
		{ID: "two", Title: "two", CreatedAt: createdAt.Add(time.Minute)},
	}, "Notes updated")

	if app.status != "1 new note" {
		t.Fatalf("status = %q, want new note count", app.status)
	}
	if app.notes[app.selected].ID != "one" {
		t.Fatalf("selected = %q, want existing selection preserved", app.notes[app.selected].ID)
	}
	if !app.isUnread("two") {
		t.Fatal("new note was not marked unread")
	}
}

func TestApplyLoadedNotesCanAutoSelectNewNotes(t *testing.T) {
	createdAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	app := &App{
		allNotes: []notes.Note{
			{ID: "one", Title: "one", CreatedAt: createdAt},
		},
		notes: []notes.Note{
			{ID: "one", Title: "one", CreatedAt: createdAt},
		},
		settings: Settings{RefreshInterval: time.Second, NoteOrder: OrderOldestFirst, AutoSelectNewNotes: true},
	}

	app.applyLoadedNotes([]notes.Note{
		{ID: "one", Title: "one", CreatedAt: createdAt},
		{ID: "two", Title: "two", CreatedAt: createdAt.Add(time.Minute)},
	}, "Notes updated")

	if app.notes[app.selected].ID != "two" {
		t.Fatalf("selected = %q, want newest incoming note", app.notes[app.selected].ID)
	}
	if app.isUnread("two") {
		t.Fatal("auto-selected new note is still unread")
	}
}

func TestNoteOrderCanShowNewestFirst(t *testing.T) {
	createdAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	app := &App{
		settings: Settings{RefreshInterval: time.Second, NoteOrder: OrderNewestFirst},
	}

	app.applyLoadedNotes([]notes.Note{
		{ID: "one", Title: "one", CreatedAt: createdAt},
		{ID: "two", Title: "two", CreatedAt: createdAt.Add(time.Minute)},
	}, "")

	if app.notes[0].ID != "two" {
		t.Fatalf("first note = %q, want newest note first", app.notes[0].ID)
	}
}

func TestPinnedNotesSortBeforeUnpinnedNotes(t *testing.T) {
	createdAt := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	app := &App{
		settings: Settings{RefreshInterval: time.Second, NoteOrder: OrderOldestFirst},
	}

	app.applyLoadedNotes([]notes.Note{
		{ID: "old", Title: "old", CreatedAt: createdAt},
		{ID: "pinned", Title: "pinned", CreatedAt: createdAt.Add(time.Hour), Pinned: true},
	}, "")

	if app.notes[0].ID != "pinned" {
		t.Fatalf("first note = %q, want pinned note first", app.notes[0].ID)
	}
}

func TestTogglePinPinsSelectedNote(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	note, err := store.Append("pin me", "body")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}
	app := loadedApp(t, store)

	if err := app.togglePin(nil, nil); err != nil {
		t.Fatalf("toggle pin: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if loaded[0].ID != note.ID || !loaded[0].Pinned {
		t.Fatalf("loaded note = %#v, want pinned note", loaded[0])
	}
	if !strings.Contains(app.status, "Pinned") {
		t.Fatalf("status = %q, want pinned message", app.status)
	}
}

func TestSelectingUnreadNoteMarksItRead(t *testing.T) {
	app := &App{
		notes: []notes.Note{
			{ID: "one", Title: "one"},
			{ID: "two", Title: "two"},
		},
		unreadIDs: map[string]struct{}{"two": {}},
	}

	if err := app.down(nil, nil); err != nil {
		t.Fatalf("down: %v", err)
	}

	if app.isUnread("two") {
		t.Fatal("selected unread note is still unread")
	}
}

func TestEditUpdatesSelectedNote(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	note, err := store.Append("old", "old body")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}
	app := loadedApp(t, store)
	app.editNote = func(notes.Note) (string, string, bool, error) {
		return "new", "new body", true, nil
	}

	if err := app.edit(nil, nil); err != nil {
		t.Fatalf("edit: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if loaded[0].ID != note.ID || loaded[0].Title != "new" || loaded[0].Body != "new body" {
		t.Fatalf("updated note = %#v, want edited note", loaded[0])
	}
	if app.status != "Saved note" {
		t.Fatalf("status = %q, want Saved note", app.status)
	}
}

func TestCreateAppendsAndSelectsNewNote(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("old", "old body"); err != nil {
		t.Fatalf("append old note: %v", err)
	}
	app := loadedApp(t, store)
	app.settings.NoteOrder = OrderNewestFirst
	app.detailOffset = 7
	app.createNote = func() (string, string, bool, error) {
		return "new", "new body", true, nil
	}

	if err := app.create(nil, nil); err != nil {
		t.Fatalf("create: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d notes, want 2", len(loaded))
	}
	created := loaded[1]
	if created.Title != "new" || created.Body != "new body" {
		t.Fatalf("created note = %#v, want new title/body", created)
	}
	selected, ok := app.selectedNote()
	if !ok {
		t.Fatal("selected note missing")
	}
	if selected.ID != created.ID {
		t.Fatalf("selected note = %q, want created note %q", selected.ID, created.ID)
	}
	if app.status != "Created note" {
		t.Fatalf("status = %q, want Created note", app.status)
	}
	if app.detailOffset != 0 {
		t.Fatalf("detailOffset = %d, want reset for created note", app.detailOffset)
	}
}

func TestCreateCanceledDoesNotAppend(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	app := loadedApp(t, store)
	app.createNote = func() (string, string, bool, error) {
		return "", "", false, nil
	}

	if err := app.create(nil, nil); err != nil {
		t.Fatalf("create: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded %d notes, want none", len(loaded))
	}
	if app.status != "Create canceled" {
		t.Fatalf("status = %q, want Create canceled", app.status)
	}
}

func TestCreateClearsFilterWhenNewNoteWouldBeHidden(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("old", "old body"); err != nil {
		t.Fatalf("append old note: %v", err)
	}
	app := loadedApp(t, store)
	app.setFilterQuery("old")
	app.createNote = func() (string, string, bool, error) {
		return "new", "new body", true, nil
	}

	if err := app.create(nil, nil); err != nil {
		t.Fatalf("create: %v", err)
	}

	if app.filterQuery != "" {
		t.Fatalf("filterQuery = %q, want cleared filter", app.filterQuery)
	}
	selected, ok := app.selectedNote()
	if !ok {
		t.Fatal("selected note missing")
	}
	if selected.Title != "new" {
		t.Fatalf("selected title = %q, want new", selected.Title)
	}
}

func TestParseEditableNoteUsesFirstLineAsTitle(t *testing.T) {
	title, body, err := parseEditableNote("new title\n\nbody line one\nbody line two\n")
	if err != nil {
		t.Fatalf("parse editable note: %v", err)
	}
	if title != "new title" || body != "body line one\nbody line two" {
		t.Fatalf("parsed title/body = %q/%q", title, body)
	}
}

func TestCreateNoteInExternalEditorUsesFirstLineAsTitle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell editor command is unix-specific")
	}

	editor := `sh -c "printf 'new title\n\nnew body\n' > \"\$1\"" sh`
	title, body, created, err := CreateNoteInExternalEditor(editor)
	if err != nil {
		t.Fatalf("create note in editor: %v", err)
	}
	if !created {
		t.Fatal("created = false, want true")
	}
	if title != "new title" || body != "new body" {
		t.Fatalf("created title/body = %q/%q, want parsed editor content", title, body)
	}
}

func TestCreateNoteInExternalEditorTreatsEmptyFileAsCanceled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell editor command is unix-specific")
	}

	_, _, created, err := CreateNoteInExternalEditor("true")
	if err != nil {
		t.Fatalf("create note in editor: %v", err)
	}
	if created {
		t.Fatal("created = true, want canceled")
	}
}

func TestPaneColorsFollowActivePane(t *testing.T) {
	theme := DefaultTheme()
	app := &App{}
	if app.paneFrameColor(paneNotes) != theme.ActiveBorder {
		t.Fatal("notes pane should be active by default")
	}
	if app.paneFrameColor(paneDetail) != theme.InactiveBorder {
		t.Fatal("detail pane should be inactive by default")
	}

	if err := app.focusDetail(nil, nil); err != nil {
		t.Fatalf("focus detail: %v", err)
	}
	if app.paneFrameColor(paneDetail) != theme.ActiveBorder {
		t.Fatal("detail pane should be active after focus")
	}
	if app.paneFrameColor(paneNotes) != theme.InactiveBorder {
		t.Fatal("notes pane should be inactive after detail focus")
	}
}

func TestFitLineTruncatesLongText(t *testing.T) {
	got := fitLine("abcdef", 4)
	if got != "abc…" {
		t.Fatalf("fitLine() = %q, want truncated text", got)
	}
}

func TestListLinePadsToFullWidth(t *testing.T) {
	got := listLine(notes.Note{Title: "abc"}, true, false, 10)
	if runeLen(got) != 10 {
		t.Fatalf("listLine length = %d, want 10: %q", runeLen(got), got)
	}
	if !strings.HasPrefix(got, "›   abc") {
		t.Fatalf("listLine() = %q, want selected prefix and title", got)
	}
}

func TestListLineTruncatesLongTitle(t *testing.T) {
	got := listLine(notes.Note{Title: "abcdef"}, false, false, 6)
	if got != "    a…" {
		t.Fatalf("listLine() = %q, want truncated padded title", got)
	}
}

func TestListLineShowsUnreadAndPinnedGutter(t *testing.T) {
	if got := listLine(notes.Note{Title: "abc", Pinned: true}, false, false, 10); !strings.HasPrefix(got, "  ▴ abc") {
		t.Fatalf("pinned listLine() = %q, want pin glyph", got)
	}
	if got := listLine(notes.Note{Title: "abc", Pinned: true}, false, true, 10); !strings.HasPrefix(got, "  ● abc") {
		t.Fatalf("unread listLine() = %q, want unread glyph", got)
	}
}

func TestNoteSubtitleIncludesTagsAndUpdatedAt(t *testing.T) {
	updatedAt := time.Date(2026, 5, 4, 13, 0, 0, 0, time.Local)
	note := notes.Note{
		CreatedAt: time.Date(2026, 5, 4, 12, 0, 0, 0, time.Local),
		UpdatedAt: &updatedAt,
		Tags:      []string{"work", "idea"},
	}

	got := noteSubtitle(note)
	for _, want := range []string{"2026-05-04 12:00", "edited 2026-05-04 13:00", "#work #idea"} {
		if !strings.Contains(got, want) {
			t.Fatalf("noteSubtitle() = %q, want %q", got, want)
		}
	}
}

func TestDetailHeaderPreventsTitleSubtitleCollision(t *testing.T) {
	title := strings.Repeat("long title ", 8)
	subtitle := "2026-08-20 12:00  edited 2026-08-20 13:00  #work #release"

	gotTitle, gotSubtitle := detailHeader(title, subtitle, 60)
	if runeLen(gotTitle)+runeLen(gotSubtitle)+2 > 48 {
		t.Fatalf("detail header uses %d columns, want at most 48", runeLen(gotTitle)+runeLen(gotSubtitle)+2)
	}
	if runeLen(gotTitle) < 12 {
		t.Fatalf("detail title width = %d, want title priority", runeLen(gotTitle))
	}

	narrowTitle, narrowSubtitle := detailHeader(title, subtitle, 28)
	if narrowSubtitle != "" {
		t.Fatalf("narrow subtitle = %q, want metadata hidden", narrowSubtitle)
	}
	if runeLen(narrowTitle) > 16 {
		t.Fatalf("narrow title width = %d, want at most 16", runeLen(narrowTitle))
	}
}

func TestHelpToggleUsesPopup(t *testing.T) {
	app := &App{}

	if err := app.toggleHelp(nil, nil); err != nil {
		t.Fatalf("toggle help: %v", err)
	}
	if app.popup == nil {
		t.Fatal("popup is nil, want help popup")
	}
	if app.popup.Title != "Help" {
		t.Fatalf("popup title = %q, want Help", app.popup.Title)
	}

	if err := app.closePopup(nil); err != nil {
		t.Fatalf("close popup: %v", err)
	}
	if app.popup != nil {
		t.Fatal("popup is not nil, want closed popup")
	}
}

func TestShortPopupUsesCompactHeight(t *testing.T) {
	app := &App{popup: &Popup{Title: "Menu", Lines: []string{"one", "two"}}}
	_, height := app.popupSize(100, 30)
	if height != 6 {
		t.Fatalf("popup height = %d, want compact height 6", height)
	}
}

func TestViewsSeparateActiveArchivedPinnedAndTags(t *testing.T) {
	app := &App{allNotes: []notes.Note{
		{ID: "active", Title: "active", Pinned: true, Tags: []string{"work"}},
		{ID: "plain", Title: "plain"},
		{ID: "archived", Title: "archived", Archived: true, Pinned: true, Tags: []string{"work"}},
	}}

	app.applyFilter("")
	if got := len(app.notes); got != 2 {
		t.Fatalf("active notes = %d, want 2", got)
	}
	app.currentView = noteView{kind: viewPinned}
	app.applyFilter("")
	if got := len(app.notes); got != 1 || app.notes[0].ID != "active" {
		t.Fatalf("pinned view = %#v, want active pinned note", app.notes)
	}
	app.currentView = noteView{kind: viewArchived}
	app.applyFilter("")
	if got := len(app.notes); got != 1 || app.notes[0].ID != "archived" {
		t.Fatalf("archived view = %#v, want archived note", app.notes)
	}
	app.currentView = noteView{kind: viewTag, tag: "work"}
	app.applyFilter("")
	if got := len(app.notes); got != 1 || app.notes[0].ID != "active" {
		t.Fatalf("tag view = %#v, want active tagged note", app.notes)
	}
}

func TestRecentViewKeepsFiftyNewestActiveNotes(t *testing.T) {
	app := &App{}
	for i := 0; i < 60; i++ {
		app.allNotes = append(app.allNotes, notes.Note{
			ID:        fmt.Sprintf("note-%02d", i),
			CreatedAt: time.Unix(int64(i), 0),
		})
	}
	app.allNotes = append(app.allNotes, notes.Note{
		ID:        "archived-newest",
		CreatedAt: time.Unix(100, 0),
		Archived:  true,
	})
	app.currentView = noteView{kind: viewRecent}
	app.applyFilter("")

	if got := len(app.notes); got != recentNoteLimit {
		t.Fatalf("recent notes = %d, want %d", got, recentNoteLimit)
	}
	if got := app.notes[0].ID; got != "note-59" {
		t.Fatalf("first recent note = %q, want newest active note", got)
	}
	if noteIndexByID(app.notes, "archived-newest") >= 0 {
		t.Fatal("recent view included archived note")
	}
}

func TestToggleArchiveMovesNoteBetweenViews(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("archive me", "body"); err != nil {
		t.Fatalf("append note: %v", err)
	}
	app := loadedApp(t, store)
	if err := app.togglePin(nil, nil); err != nil {
		t.Fatalf("pin before archive: %v", err)
	}
	if !app.notes[0].Pinned {
		t.Fatal("note was not pinned before archive")
	}

	if err := app.toggleArchive(nil, nil); err != nil {
		t.Fatalf("archive: %v", err)
	}
	if len(app.notes) != 0 {
		t.Fatalf("active notes = %d, want archived note hidden", len(app.notes))
	}
	app.currentView = noteView{kind: viewArchived}
	app.applyFilter("")
	if len(app.notes) != 1 || !app.notes[0].Archived || app.notes[0].Pinned {
		t.Fatalf("archived notes = %#v, want archived and unpinned note", app.notes)
	}
	if err := app.togglePin(nil, nil); err != nil {
		t.Fatalf("pin archived note: %v", err)
	}
	if app.notes[0].Pinned {
		t.Fatal("archived note was pinned")
	}
	if app.status != "Restore note before pinning" {
		t.Fatalf("status = %q, want restore guidance", app.status)
	}
	if err := app.toggleArchive(nil, nil); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if len(app.notes) != 0 {
		t.Fatalf("archived notes = %d, want restored note hidden", len(app.notes))
	}
}

func TestStaleRefreshCannotRestoreArchivedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	store := notes.NewStore(path)
	note, err := store.Append("archive me", "body")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}
	archived, _, err := store.SetArchived(note.ID, true)
	if err != nil {
		t.Fatalf("archive note: %v", err)
	}
	staleSnapshot, err := snapshotFile(path)
	if err != nil {
		t.Fatalf("snapshot archived file: %v", err)
	}

	active, _, err := store.SetArchived(note.ID, false)
	if err != nil {
		t.Fatalf("restore note: %v", err)
	}
	app := &App{store: store, theme: DefaultTheme(), settings: DefaultSettings()}
	app.applyLoadedNotes(active, "")

	if app.applyRefreshSnapshot(path, staleSnapshot, archived) {
		t.Fatal("stale refresh snapshot was applied")
	}
	if len(app.sourceNotes()) != 1 || app.sourceNotes()[0].Archived {
		t.Fatalf("notes after stale refresh = %#v, want restored note", app.sourceNotes())
	}
}

func TestTagPickerTogglesAndSavesTags(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	note, err := store.AppendWithTags("tag me", "body", []string{"work"})
	if err != nil {
		t.Fatalf("append note: %v", err)
	}
	if _, err := store.AppendWithTags("other", "body", []string{"home"}); err != nil {
		t.Fatalf("append other note: %v", err)
	}
	app := loadedApp(t, store)
	app.selected = noteIndexByID(app.notes, note.ID)

	if err := app.openTagPicker(nil, nil); err != nil {
		t.Fatalf("open tag picker: %v", err)
	}
	if app.popup == nil || len(app.popup.Lines) != 2 {
		t.Fatalf("popup = %#v, want two tags", app.popup)
	}
	workIndex := 0
	if strings.Contains(app.popup.Lines[0], "#home") {
		workIndex = 1
	}
	if err := app.popup.OnToggle(nil, workIndex); err != nil {
		t.Fatalf("toggle tag: %v", err)
	}
	if err := app.popup.OnSelect(nil, workIndex); err != nil {
		t.Fatalf("save tags: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	updated := loaded[noteIndexByID(loaded, note.ID)]
	if len(updated.Tags) != 0 {
		t.Fatalf("tags = %#v, want removed", updated.Tags)
	}
}

func TestConfirmNewTagAddsNormalizedTag(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	note, err := store.Append("tag me", "body")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}
	app := loadedApp(t, store)
	app.inputMode = inputTag
	app.tagTargetID = note.ID
	app.tagInput = " #Project "

	if err := app.confirmNewTag(nil); err != nil {
		t.Fatalf("confirm tag: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if got, want := strings.Join(loaded[0].Tags, ","), "project"; got != want {
		t.Fatalf("tags = %q, want %q", got, want)
	}
}

func TestListWidthIsStableOnWideScreens(t *testing.T) {
	if got := listWidth(120); got != defaultListWidth {
		t.Fatalf("listWidth(120) = %d, want %d", got, defaultListWidth)
	}
}

func TestVisualLineCountIncludesWrappedParagraphsAndBlankLines(t *testing.T) {
	got := visualLineCount("abcd\n\nefghij", 3)
	if got != 5 {
		t.Fatalf("visualLineCount() = %d, want wrapped paragraphs and blank line counted", got)
	}
}

func TestListWidthCapsAtMaximum(t *testing.T) {
	if got := listWidth(89); got > maxListWidth {
		t.Fatalf("listWidth(89) = %d, want at most %d", got, maxListWidth)
	}
}

func loadedApp(t *testing.T, store *notes.Store) *App {
	t.Helper()

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}

	app := &App{store: store, theme: DefaultTheme(), settings: DefaultSettings()}
	app.allNotes = app.orderedNotes(loaded)
	app.applyFilter("")
	return app
}
