package ui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awesome-gocui/gocui"
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

	app := New(store)
	g, err := app.newGUI(gocui.OutputSimulator)
	if err != nil {
		t.Fatalf("new gui: %v", err)
	}
	defer g.Close()

	screen := g.GetTestingScreen()
	stop := screen.StartGui()
	defer stop()

	screen.SendStringAsKeys("j")
	screen.WaitSync()
	screen.SendStringAsKeys("d")
	screen.WaitSync()

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load notes after first delete key: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("kept %d notes after first delete key, want 2", len(loaded))
	}
	if app.pendingDeleteID != second.ID {
		t.Fatalf("pendingDeleteID = %q, want %q", app.pendingDeleteID, second.ID)
	}
	if !strings.Contains(app.status, "Press d again") {
		t.Fatalf("status = %q, want delete confirmation", app.status)
	}

	screen.SendStringAsKeys("d")
	screen.WaitSync()

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

	app := New(store)
	g, err := app.newGUI(gocui.OutputSimulator)
	if err != nil {
		t.Fatalf("new gui: %v", err)
	}
	defer g.Close()

	screen := g.GetTestingScreen()
	stop := screen.StartGui()
	defer stop()

	screen.SendStringAsKeys("d")
	screen.WaitSync()
	if app.pendingDeleteID == "" {
		t.Fatal("pendingDeleteID is empty, want armed delete")
	}

	screen.SendStringAsKeys("j")
	screen.WaitSync()
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

	app := New(store)
	g, err := app.newGUI(gocui.OutputSimulator)
	if err != nil {
		t.Fatalf("new gui: %v", err)
	}
	defer g.Close()

	screen := g.GetTestingScreen()
	stop := screen.StartGui()
	defer stop()

	screen.WaitSync()
	screen.SendKeySync(gocui.KeyArrowRight)
	screen.WaitSync()
	screen.SendKeySync(gocui.KeyArrowDown)
	screen.WaitSync()

	if app.activePane != paneDetail {
		t.Fatalf("activePane = %v, want detail pane", app.activePane)
	}
	if app.detailOffset == 0 {
		t.Fatal("detailOffset = 0, want arrow down to scroll active detail pane")
	}

	screen.SendKeySync(gocui.KeyPgdn)
	screen.WaitSync()

	if app.detailOffset <= 1 {
		t.Fatalf("detailOffset = %d, want page down to scroll farther", app.detailOffset)
	}

	screen.SendKeySync(gocui.KeyPgup)
	screen.WaitSync()
	screen.SendKeySync(gocui.KeyArrowUp)
	screen.WaitSync()
	if app.detailOffset < 0 {
		t.Fatalf("detailOffset = %d, want non-negative offset", app.detailOffset)
	}
}

func TestFocusControlsContextualUpDown(t *testing.T) {
	store := notes.NewStore(filepath.Join(t.TempDir(), "notes.json"))
	if _, err := store.Append("first", strings.Repeat("first body line with enough repeated content\n", 80)); err != nil {
		t.Fatalf("append first note: %v", err)
	}
	if _, err := store.Append("second", "second body"); err != nil {
		t.Fatalf("append second note: %v", err)
	}

	app := New(store)
	g, err := app.newGUI(gocui.OutputSimulator)
	if err != nil {
		t.Fatalf("new gui: %v", err)
	}
	defer g.Close()

	screen := g.GetTestingScreen()
	stop := screen.StartGui()
	defer stop()

	screen.WaitSync()
	screen.SendKeySync(gocui.KeyArrowRight)
	screen.WaitSync()
	screen.SendKeySync(gocui.KeyArrowDown)
	screen.WaitSync()
	if app.selected != 0 {
		t.Fatalf("selected = %d, want detail scrolling to keep note selection", app.selected)
	}
	if app.detailOffset == 0 {
		t.Fatal("detailOffset = 0, want detail pane to scroll")
	}

	screen.SendKeySync(gocui.KeyArrowLeft)
	screen.WaitSync()
	screen.SendKeySync(gocui.KeyArrowDown)
	screen.WaitSync()
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
	app := &App{
		notes: []notes.Note{
			{Title: "one", CreatedAt: time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)},
			{Title: "two", CreatedAt: time.Date(2026, 4, 30, 13, 0, 0, 0, time.UTC)},
		},
		selected: 1,
	}

	got := app.statusLine()
	for _, want := range []string{"notes", "2/2", "←/→ focus", "j/k move/scroll", "d delete", "q quit"} {
		if !strings.Contains(got, want) {
			t.Fatalf("statusLine() = %q, want %q", got, want)
		}
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
