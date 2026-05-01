package ui

import (
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/rschoch/lazynote/internal/notes"
)

const (
	notesView  = "notes"
	detailView = "detail"
	statusView = "status"

	defaultListWidth = 28
	minListWidth     = 22
	maxListWidth     = 34
	statusIcon       = "▤"
)

type pane int

const (
	paneNotes pane = iota
	paneDetail
)

var (
	roundedFrameRunes = []rune{'─', '│', '╭', '╮', '╰', '╯', '├', '┤', '┬', '┴', '┼'}

	colorText        = gocui.Get256Color(252)
	colorMuted       = gocui.Get256Color(248)
	colorFrame       = gocui.Get256Color(66)
	colorAccent      = gocui.Get256Color(80)
	colorTitle       = gocui.Get256Color(218)
	colorWarn        = gocui.Get256Color(215)
	colorStatusFg    = colorMuted
	colorSelectionBg = gocui.Get256Color(79)
	colorSelectionFg = gocui.Get256Color(234)
)

// App owns the lazynote terminal UI state.
type App struct {
	store           *notes.Store
	notes           []notes.Note
	selected        int
	detailOffset    int
	activePane      pane
	pendingDeleteID string
	status          string
	statusMode      statusMode
	copyText        func(string) error
}

type statusMode int

const (
	statusDefault statusMode = iota
	statusDeleteArmed
	statusMessage
)

// New creates a terminal UI app backed by store.
func New(store *notes.Store) *App {
	return &App{store: store}
}

// Run starts the terminal UI.
func (a *App) Run() error {
	g, err := a.newGUI(gocui.Output256)
	if err != nil {
		return err
	}
	defer g.Close()

	if err := g.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		return err
	}

	return nil
}

func (a *App) newGUI(mode gocui.OutputMode) (*gocui.Gui, error) {
	loaded, err := a.store.Load()
	if err != nil {
		return nil, err
	}
	a.notes = loaded
	a.clampSelection()

	g, err := gocui.NewGui(mode, true)
	if err != nil {
		return nil, fmt.Errorf("start terminal UI: %w", err)
	}

	g.Cursor = false
	g.Highlight = false
	g.FgColor = colorText
	g.FrameColor = colorFrame
	g.SelFrameColor = colorAccent
	g.SetManagerFunc(a.layout)

	if err := a.keybindings(g); err != nil {
		g.Close()
		return nil, err
	}

	return g, nil
}

func (a *App) keybindings(g *gocui.Gui) error {
	bindings := []struct {
		view    string
		key     interface{}
		handler func(*gocui.Gui, *gocui.View) error
	}{
		{"", 'q', quit},
		{"", gocui.KeyCtrlC, quit},
		{"", gocui.KeyArrowDown, a.down},
		{"", gocui.KeyArrowUp, a.up},
		{"", 'c', a.copy},
		{"", 'd', a.delete},
		{"", gocui.KeyDelete, a.delete},
		{"", gocui.KeyPgdn, a.detailDown},
		{"", gocui.KeyPgup, a.detailUp},
		{"", gocui.KeyArrowLeft, a.focusNotes},
		{"", gocui.KeyArrowRight, a.focusDetail},
	}

	for _, binding := range bindings {
		if err := g.SetKeybinding(binding.view, binding.key, gocui.ModNone, binding.handler); err != nil {
			return err
		}
	}

	return nil
}

func (a *App) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if maxX < 20 || maxY < 8 {
		return a.layoutSmall(g, maxX, maxY)
	}

	leftWidth := listWidth(maxX)
	statusTop := maxY - 2
	paneBottom := statusTop

	if err := a.layoutNotes(g, 0, 0, leftWidth, paneBottom); err != nil {
		return err
	}
	if err := a.layoutDetail(g, leftWidth+1, 0, maxX-1, paneBottom); err != nil {
		return err
	}
	if err := a.layoutStatus(g, -1, statusTop, maxX, maxY); err != nil {
		return err
	}

	return a.setCurrentView(g)
}

func (a *App) layoutSmall(g *gocui.Gui, maxX, maxY int) error {
	v, err := g.SetView(statusView, 0, 0, maxX-1, maxY-1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	v.Title = " lazynote "
	v.TitleColor = colorTitle | gocui.AttrBold
	v.FrameColor = colorWarn
	v.FrameRunes = roundedFrameRunes
	v.Wrap = true
	v.Clear()
	fmt.Fprintln(v, "Terminal too small.")
	return nil
}

func (a *App) layoutNotes(g *gocui.Gui, x0, y0, x1, y1 int) error {
	v, err := g.SetView(notesView, x0, y0, x1, y1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	v.Title = fmt.Sprintf(" Notes %d ", len(a.notes))
	v.Subtitle = ""
	v.TitleColor = a.paneTitleColor(paneNotes)
	v.FrameColor = a.paneFrameColor(paneNotes)
	v.FrameRunes = roundedFrameRunes
	v.FgColor = colorText
	v.Highlight = a.activePane == paneNotes
	v.SelBgColor = colorSelectionBg
	v.SelFgColor = colorSelectionFg
	v.Clear()

	if len(a.notes) == 0 {
		v.FgColor = colorMuted
		fmt.Fprintln(v, "No notes yet")
		_ = v.SetOrigin(0, 0)
		_ = v.SetCursor(0, 0)
		return nil
	}

	width, _ := v.Size()
	if width < 1 {
		width = 1
	}

	for i, note := range a.notes {
		fmt.Fprintln(v, listLine(note.Title, i == a.selected, width))
	}
	a.syncListCursor(v)

	return nil
}

func (a *App) layoutDetail(g *gocui.Gui, x0, y0, x1, y1 int) error {
	v, err := g.SetView(detailView, x0, y0, x1, y1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	v.Wrap = true
	v.TitleColor = a.paneTitleColor(paneDetail)
	v.FrameColor = a.paneFrameColor(paneDetail)
	v.FrameRunes = roundedFrameRunes
	v.FgColor = colorText
	v.Clear()

	note, ok := a.selectedNote()
	if !ok {
		v.Title = " Note "
		v.Subtitle = ""
		a.detailOffset = 0
		v.FgColor = colorMuted
		fmt.Fprintln(v, "Nothing saved yet.")
		return nil
	}

	v.Title = " " + oneLine(note.Title) + " "
	v.Subtitle = note.CreatedAt.Local().Format("2006-01-02 15:04")
	if note.Body != "" {
		fmt.Fprintln(v, note.Body)
	}
	a.clampDetailOffset(v, note)
	_ = v.SetOrigin(0, a.detailOffset)
	return nil
}

func (a *App) layoutStatus(g *gocui.Gui, x0, y0, x1, y1 int) error {
	v, err := g.SetView(statusView, x0, y0, x1, y1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	v.Frame = false
	v.FgColor = colorStatusFg
	v.Clear()

	width, _ := v.Size()
	fmt.Fprint(v, fitLine(a.statusLine(), width))

	return nil
}

func (a *App) statusLine() string {
	return fmt.Sprintf(" %s   %s ", a.statusText(), a.statusHints())
}

func (a *App) statusText() string {
	if a.status != "" {
		return a.status
	}

	status := statusIcon + " 0/0"
	if _, ok := a.selectedNote(); ok {
		status = fmt.Sprintf("%s %d/%d", statusIcon, a.selected+1, len(a.notes))
		if a.activePane == paneDetail && a.detailOffset > 0 {
			status = fmt.Sprintf("%s  scroll +%d", status, a.detailOffset)
		}
	}
	return status
}

func (a *App) statusHints() string {
	switch a.statusMode {
	case statusDeleteArmed:
		return "d confirm   arrows cancel   q quit"
	}

	if _, ok := a.selectedNote(); !ok {
		return "q quit"
	}

	if a.activePane == paneDetail {
		return "↑/↓ scroll   pg page   ← list   c copy body   q quit"
	}
	return "↑/↓ select   → body   c copy title   d delete   q quit"
}

func (a *App) syncListCursor(v *gocui.View) {
	_, originY := v.Origin()
	_, height := v.Size()
	if height < 1 {
		height = 1
	}

	if a.selected < originY {
		originY = a.selected
	}
	if a.selected >= originY+height {
		originY = a.selected - height + 1
	}
	if originY < 0 {
		originY = 0
	}

	_ = v.SetOrigin(0, originY)
	_ = v.SetCursor(0, a.selected-originY)
}

func (a *App) selectedNote() (notes.Note, bool) {
	if a.selected < 0 || a.selected >= len(a.notes) {
		return notes.Note{}, false
	}
	return a.notes[a.selected], true
}

func (a *App) up(g *gocui.Gui, v *gocui.View) error {
	if a.activePane == paneDetail {
		return a.scrollDetail(g, -1)
	}

	if a.selected > 0 {
		a.selected--
		a.detailOffset = 0
		a.pendingDeleteID = ""
		a.status = ""
		a.statusMode = statusDefault
	}
	return nil
}

func (a *App) down(g *gocui.Gui, v *gocui.View) error {
	if a.activePane == paneDetail {
		return a.scrollDetail(g, 1)
	}

	if a.selected < len(a.notes)-1 {
		a.selected++
		a.detailOffset = 0
		a.pendingDeleteID = ""
		a.status = ""
		a.statusMode = statusDefault
	}
	return nil
}

func (a *App) focusNotes(g *gocui.Gui, v *gocui.View) error {
	a.activePane = paneNotes
	a.pendingDeleteID = ""
	a.status = ""
	a.statusMode = statusDefault
	return a.setCurrentView(g)
}

func (a *App) focusDetail(g *gocui.Gui, v *gocui.View) error {
	a.activePane = paneDetail
	a.pendingDeleteID = ""
	a.status = ""
	a.statusMode = statusDefault
	return a.setCurrentView(g)
}

func (a *App) detailUp(g *gocui.Gui, v *gocui.View) error {
	return a.scrollDetail(g, -detailPageSize(g))
}

func (a *App) detailDown(g *gocui.Gui, v *gocui.View) error {
	return a.scrollDetail(g, detailPageSize(g))
}

func (a *App) scrollDetail(g *gocui.Gui, delta int) error {
	if g == nil {
		return nil
	}

	note, ok := a.selectedNote()
	if !ok {
		return nil
	}

	v, err := g.View(detailView)
	if err != nil {
		return nil
	}

	width, height := v.Size()
	maxOffset := a.scrollDetailBy(note, delta, width, height)
	if maxOffset > 0 {
		a.status = fmt.Sprintf("Scroll %d/%d", a.detailOffset, maxOffset)
		a.statusMode = statusMessage
	}
	return nil
}

func (a *App) delete(g *gocui.Gui, v *gocui.View) error {
	note, ok := a.selectedNote()
	if !ok {
		return nil
	}

	if a.pendingDeleteID != note.ID {
		a.pendingDeleteID = note.ID
		a.status = fmt.Sprintf("Press d again to delete %q", note.Title)
		a.statusMode = statusDeleteArmed
		return nil
	}

	updated, err := a.store.Delete(note.ID)
	if err != nil {
		a.status = fmt.Sprintf("Delete failed: %v", err)
		a.statusMode = statusMessage
		return nil
	}

	a.notes = updated
	a.clampSelection()
	a.detailOffset = 0
	a.pendingDeleteID = ""
	a.status = fmt.Sprintf("Deleted %q", note.Title)
	a.statusMode = statusMessage
	return nil
}

func (a *App) copy(g *gocui.Gui, v *gocui.View) error {
	note, ok := a.selectedNote()
	if !ok {
		a.pendingDeleteID = ""
		a.status = "Nothing to copy"
		a.statusMode = statusMessage
		return nil
	}

	label := "title"
	text := note.Title
	if a.activePane == paneDetail {
		label = "body"
		text = note.Body
	}

	a.pendingDeleteID = ""
	if err := a.writeClipboard(text); err != nil {
		a.status = fmt.Sprintf("Copy failed: %v", err)
		a.statusMode = statusMessage
		return nil
	}

	a.status = fmt.Sprintf("Copied %s", label)
	a.statusMode = statusMessage
	return nil
}

func (a *App) clampSelection() {
	if len(a.notes) == 0 {
		a.selected = 0
		return
	}
	if a.selected < 0 {
		a.selected = 0
	}
	if a.selected >= len(a.notes) {
		a.selected = len(a.notes) - 1
	}
}

func (a *App) clampDetailOffset(v *gocui.View, note notes.Note) int {
	width, height := v.Size()
	maxOffset := maxDetailOffset(note.Body, width, height)
	if a.detailOffset < 0 {
		a.detailOffset = 0
	}
	if a.detailOffset > maxOffset {
		a.detailOffset = maxOffset
	}
	return maxOffset
}

func (a *App) scrollDetailBy(note notes.Note, delta, width, height int) int {
	a.detailOffset += delta
	a.pendingDeleteID = ""
	if a.statusMode == statusDeleteArmed {
		a.status = ""
		a.statusMode = statusDefault
	}

	maxOffset := maxDetailOffset(note.Body, width, height)
	if a.detailOffset < 0 {
		a.detailOffset = 0
	}
	if a.detailOffset > maxOffset {
		a.detailOffset = maxOffset
	}
	return maxOffset
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func (a *App) writeClipboard(text string) error {
	if a.copyText != nil {
		return a.copyText(text)
	}
	return writeOSC52Clipboard(text)
}

func writeOSC52Clipboard(text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	sequence := "\x1b]52;c;" + encoded + "\a"

	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err == nil {
		defer tty.Close()
		_, err = tty.WriteString(sequence)
		return err
	}

	_, err = os.Stdout.WriteString(sequence)
	return err
}

func (a *App) setCurrentView(g *gocui.Gui) error {
	if g == nil {
		return nil
	}

	_, err := g.SetCurrentView(a.activePane.viewName())
	return err
}

func (a *App) paneFrameColor(p pane) gocui.Attribute {
	if a.activePane == p {
		return colorAccent
	}
	return colorFrame
}

func (a *App) paneTitleColor(p pane) gocui.Attribute {
	if a.activePane == p {
		return colorTitle | gocui.AttrBold
	}
	return colorMuted
}

func (p pane) viewName() string {
	if p == paneDetail {
		return detailView
	}
	return notesView
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func listLine(title string, selected bool, width int) string {
	prefix := "  "
	if selected {
		prefix = "› "
	}

	available := width - runeLen(prefix)
	if available < 1 {
		available = 1
	}
	return prefix + padLine(fitLine(oneLine(title), available), available)
}

func listWidth(maxX int) int {
	leftWidth := defaultListWidth
	if maxX < 90 {
		leftWidth = maxX / 3
	}
	if leftWidth < minListWidth {
		leftWidth = minListWidth
	}
	if leftWidth > maxListWidth {
		leftWidth = maxListWidth
	}
	if leftWidth > maxX-30 {
		leftWidth = maxX / 2
	}
	return leftWidth
}

func fitLine(s string, width int) string {
	if width <= 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}

func padLine(s string, width int) string {
	if width <= 0 {
		return ""
	}

	length := runeLen(s)
	if length >= width {
		return s
	}
	return s + strings.Repeat(" ", width-length)
}

func runeLen(s string) int {
	return len([]rune(s))
}

func detailPageSize(g *gocui.Gui) int {
	if g == nil {
		return 5
	}

	v, err := g.View(detailView)
	if err != nil {
		return 5
	}

	_, height := v.Size()
	if height <= 1 {
		return 1
	}
	return height - 1
}

func maxDetailOffset(body string, width, height int) int {
	if width < 1 || height < 1 {
		return 0
	}

	lines := visualLineCount(body, width)
	if lines <= height {
		return 0
	}
	return lines - height
}

func visualLineCount(s string, width int) int {
	if width < 1 || s == "" {
		return 0
	}

	total := 0
	for _, line := range strings.Split(s, "\n") {
		length := runeLen(line)
		if length == 0 {
			total++
			continue
		}
		total += (length + width - 1) / width
	}
	return total
}
