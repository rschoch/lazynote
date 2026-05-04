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
	for _, want := range []string{"2/2", "↑↓ nav", "→ body", "p pin", "? help", "d del", "q quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
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
	for _, want := range []string{"1/1", "scroll +4", "↑↓ scroll", "Pg page", "← list", "c copy"} {
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
	for _, want := range []string{"Press d again", "d confirm", "↑↓ cancel", "q quit"} {
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
	for _, want := range []string{"Deleted", "↑↓ nav", "→ body", "c copy", "q quit"} {
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
	if _, err := store.Delete(first.ID); err != nil {
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

func TestParseEditableNoteUsesFirstLineAsTitle(t *testing.T) {
	title, body, err := parseEditableNote("new title\n\nbody line one\nbody line two\n")
	if err != nil {
		t.Fatalf("parse editable note: %v", err)
	}
	if title != "new title" || body != "body line one\nbody line two" {
		t.Fatalf("parsed title/body = %q/%q", title, body)
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

func TestHelpToggleSetsHelpMode(t *testing.T) {
	app := &App{}

	if err := app.toggleHelp(nil, nil); err != nil {
		t.Fatalf("toggle help: %v", err)
	}
	if !app.showHelp {
		t.Fatal("showHelp = false, want true")
	}

	if err := app.closeHelp(nil, nil); err != nil {
		t.Fatalf("close help: %v", err)
	}
	if app.showHelp {
		t.Fatal("showHelp = true, want false")
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
