package ui

import (
	"path/filepath"
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
	for _, want := range []string{"2/2", "↑/↓ select", "→ body", "c copy title", "d delete", "q quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
	}
}

func TestStatusLineIncludesEmptyState(t *testing.T) {
	app := &App{}

	got := app.statusLine()
	for _, want := range []string{"0/0", "q quit"} {
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
	for _, want := range []string{"1/1", "scroll +4", "↑/↓ scroll", "pg page", "← list", "c copy body"} {
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
	for _, want := range []string{"Press d again", "d confirm", "arrows cancel", "q quit"} {
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
	for _, want := range []string{"Deleted", "↑/↓ select", "→ body", "c copy title", "q quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
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

func TestPaneColorsFollowActivePane(t *testing.T) {
	app := &App{}
	if app.paneFrameColor(paneNotes) != colorAccent {
		t.Fatal("notes pane should be active by default")
	}
	if app.paneFrameColor(paneDetail) != colorFrame {
		t.Fatal("detail pane should be inactive by default")
	}

	if err := app.focusDetail(nil, nil); err != nil {
		t.Fatalf("focus detail: %v", err)
	}
	if app.paneFrameColor(paneDetail) != colorAccent {
		t.Fatal("detail pane should be active after focus")
	}
	if app.paneFrameColor(paneNotes) != colorFrame {
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
	got := listLine("abc", true, 10)
	if runeLen(got) != 10 {
		t.Fatalf("listLine length = %d, want 10: %q", runeLen(got), got)
	}
	if !strings.HasPrefix(got, "› abc") {
		t.Fatalf("listLine() = %q, want selected prefix and title", got)
	}
}

func TestListLineTruncatesLongTitle(t *testing.T) {
	got := listLine("abcdef", false, 5)
	if got != "  ab…" {
		t.Fatalf("listLine() = %q, want truncated padded title", got)
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

	return &App{store: store, notes: loaded}
}
