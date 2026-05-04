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
	helpView   = "help"

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
)

// App owns the lazynote terminal UI state.
type App struct {
	store           *notes.Store
	theme           Theme
	settings        Settings
	allNotes        []notes.Note
	unreadIDs       map[string]struct{}
	notes           []notes.Note
	selected        int
	detailOffset    int
	activePane      pane
	pendingDeleteID string
	status          string
	statusMode      statusMode
	showHelp        bool
	copyText        func(string) error
	filterQuery     string
	searchInput     string
	searchOriginal  string
	inputMode       inputMode
	editor          string
	editNote        func(notes.Note) (string, string, bool, error)
}

type statusMode int

const (
	statusDefault statusMode = iota
	statusDeleteArmed
	statusMessage
)

type inputMode int

const (
	inputNormal inputMode = iota
	inputSearch
)

// Option configures an App.
type Option func(*App)

// WithTheme sets the terminal UI theme.
func WithTheme(theme Theme) Option {
	return func(a *App) {
		a.theme = theme
	}
}

// WithSettings sets terminal UI behavior.
func WithSettings(settings Settings) Option {
	return func(a *App) {
		a.settings = settings.normalized()
	}
}

// WithEditor sets the editor command used by the TUI edit action.
func WithEditor(editor string) Option {
	return func(a *App) {
		a.editor = editor
	}
}

// New creates a terminal UI app backed by store.
func New(store *notes.Store, opts ...Option) *App {
	app := &App{store: store, theme: DefaultTheme(), settings: DefaultSettings()}
	for _, opt := range opts {
		opt(app)
	}
	app.settings = app.settings.normalized()
	return app
}

// Run starts the terminal UI.
func (a *App) Run() error {
	g, err := a.newGUI(gocui.OutputTrue)
	if err != nil {
		return err
	}
	defer g.Close()
	stopRefresh := a.startAutoRefresh(g, a.settings.RefreshInterval)
	defer stopRefresh()

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
	a.allNotes = a.orderedNotes(loaded)
	a.applyFilter("")
	a.clampSelection()

	g, err := gocui.NewGui(mode, true)
	if err != nil {
		return nil, fmt.Errorf("start terminal UI: %w", err)
	}

	g.Cursor = false
	g.Highlight = false
	theme := a.themeColors()
	g.BgColor = theme.DefaultBg
	g.FgColor = theme.DefaultFg
	g.FrameColor = theme.InactiveBorder
	g.SelFgColor = theme.Title
	g.SelFrameColor = theme.ActiveBorder
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
		{"", 'q', a.quitOrCloseHelp},
		{"", gocui.KeyCtrlC, quit},
		{"", '?', a.toggleHelp},
		{"", gocui.KeyArrowDown, a.down},
		{"", gocui.KeyArrowUp, a.up},
		{"", 'c', a.copy},
		{"", 'd', a.delete},
		{"", 'e', a.edit},
		{"", 'p', a.togglePin},
		{"", 'r', a.manualRefresh},
		{"", '/', a.startSearch},
		{"", gocui.KeyEsc, a.clearFilterKey},
		{"", gocui.KeyDelete, a.delete},
		{"", gocui.KeyPgdn, a.detailDown},
		{"", gocui.KeyPgup, a.detailUp},
		{"", gocui.KeyArrowLeft, a.focusNotes},
		{"", gocui.KeyArrowRight, a.focusDetail},
		{statusView, gocui.KeyEnter, a.confirmSearch},
		{statusView, gocui.KeyEsc, a.cancelSearch},
		{helpView, 'q', a.closeHelp},
		{helpView, '?', a.closeHelp},
		{helpView, gocui.KeyEsc, a.closeHelp},
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
	if a.showHelp {
		if err := a.layoutHelp(g, maxX, maxY); err != nil {
			return err
		}
	} else {
		_ = g.DeleteView(helpView)
	}

	return a.setCurrentView(g)
}

func (a *App) layoutSmall(g *gocui.Gui, maxX, maxY int) error {
	theme := a.themeColors()
	v, err := g.SetView(statusView, 0, 0, maxX-1, maxY-1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	v.Title = " lazynote "
	v.BgColor = theme.DefaultBg
	v.TitleColor = theme.Title
	v.FrameColor = theme.Warning
	v.FrameRunes = roundedFrameRunes
	v.Wrap = true
	v.Clear()
	fmt.Fprintln(v, "Terminal too small.")
	return nil
}

func (a *App) layoutNotes(g *gocui.Gui, x0, y0, x1, y1 int) error {
	theme := a.themeColors()
	v, err := g.SetView(notesView, x0, y0, x1, y1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	if a.filterQuery != "" {
		v.Title = fmt.Sprintf(" Notes %d/%d ", len(a.notes), len(a.sourceNotes()))
		v.Subtitle = " /" + fitLine(a.filterQuery, 18) + " "
	} else {
		v.Title = fmt.Sprintf(" Notes %d ", len(a.notes))
		v.Subtitle = ""
	}
	v.TitleColor = a.paneTitleColor(paneNotes)
	v.FrameColor = a.paneFrameColor(paneNotes)
	v.FrameRunes = roundedFrameRunes
	v.BgColor = theme.DefaultBg
	v.FgColor = theme.DefaultFg
	v.Highlight = a.activePane == paneNotes
	v.SelBgColor = theme.SelectedLineBg
	v.SelFgColor = theme.SelectedLineFg
	v.Clear()

	if len(a.notes) == 0 {
		v.FgColor = theme.MutedFg
		if a.filterQuery != "" {
			fmt.Fprintln(v, "No matches")
		} else {
			fmt.Fprintln(v, "No notes yet")
		}
		_ = v.SetOrigin(0, 0)
		_ = v.SetCursor(0, 0)
		return nil
	}

	width, _ := v.Size()
	if width < 1 {
		width = 1
	}

	for i, note := range a.notes {
		fmt.Fprintln(v, listLine(note, i == a.selected, a.isUnread(note.ID), width))
	}
	a.syncListCursor(v)

	return nil
}

func (a *App) layoutDetail(g *gocui.Gui, x0, y0, x1, y1 int) error {
	theme := a.themeColors()
	v, err := g.SetView(detailView, x0, y0, x1, y1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	v.Wrap = true
	v.TitleColor = a.paneTitleColor(paneDetail)
	v.FrameColor = a.paneFrameColor(paneDetail)
	v.FrameRunes = roundedFrameRunes
	v.BgColor = theme.DefaultBg
	v.FgColor = theme.DefaultFg
	v.Clear()

	note, ok := a.selectedNote()
	if !ok {
		v.Title = " Note "
		v.Subtitle = ""
		a.detailOffset = 0
		v.FgColor = theme.MutedFg
		if a.filterQuery != "" {
			fmt.Fprintln(v, "No matching note.")
		} else {
			fmt.Fprintln(v, "Nothing saved yet.")
		}
		return nil
	}

	v.Title = " " + oneLine(note.Title) + " "
	width, _ := v.Size()
	v.Subtitle = " " + fitLine(noteSubtitle(note), width-2) + " "
	if note.Body != "" {
		fmt.Fprintln(v, note.Body)
	}
	a.clampDetailOffset(v, note)
	_ = v.SetOrigin(0, a.detailOffset)
	return nil
}

func (a *App) layoutStatus(g *gocui.Gui, x0, y0, x1, y1 int) error {
	theme := a.themeColors()
	v, err := g.SetView(statusView, x0, y0, x1, y1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	v.Frame = false
	v.BgColor = theme.DefaultBg
	v.FgColor = theme.StatusFg
	v.Editable = a.inputMode == inputSearch
	if v.Editable {
		v.Editor = searchEditor{app: a}
	}
	v.Clear()

	width, _ := v.Size()
	if a.inputMode == inputSearch {
		g.Cursor = true
		line := "/" + a.searchInput
		fmt.Fprint(v, fitLine(line, width))
		_ = v.SetCursor(runeLen(line), 0)
		return nil
	}

	g.Cursor = false
	fmt.Fprint(v, fitLine(a.statusLineForWidth(width), width))

	return nil
}

func (a *App) layoutHelp(g *gocui.Gui, maxX, maxY int) error {
	theme := a.themeColors()
	width := 50
	if maxX < width+4 {
		width = maxX - 4
	}
	if width < 30 {
		width = maxX - 2
	}
	height := 17
	if maxY < height+4 {
		height = maxY - 4
	}
	if height < 10 {
		height = maxY - 2
	}

	x0 := (maxX - width) / 2
	y0 := (maxY - height) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	v, err := g.SetView(helpView, x0, y0, x0+width, y0+height, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	v.Title = " Help "
	v.TitleColor = theme.Title
	v.FrameColor = theme.ActiveBorder
	v.FrameRunes = roundedFrameRunes
	v.BgColor = theme.DefaultBg
	v.FgColor = theme.DefaultFg
	v.Wrap = false
	v.Clear()

	lines := []string{
		"↑↓        move selection or scroll body",
		"← →       switch list/body focus",
		"Pg        page through the body",
		"/         filter title, body, or #tag",
		"Esc       clear filter or close help",
		"c         copy selected title/body",
		"e         edit selected note",
		"p         pin or unpin selected note",
		"d         delete; press twice to confirm",
		"r         reload notes from disk",
		"?         close this help",
		"q         close this help",
	}
	for _, line := range lines {
		fmt.Fprintln(v, fitLine(line, width-2))
	}
	return nil
}

func (a *App) statusLine() string {
	return a.statusLineForWidth(0)
}

func (a *App) statusLineForWidth(width int) string {
	status := a.statusText()
	line := fmt.Sprintf(" %s   %s ", status, a.statusHints())
	if width <= 0 || runeLen(line) <= width {
		return line
	}

	compact := fmt.Sprintf(" %s   %s ", status, a.compactStatusHints())
	if runeLen(compact) < runeLen(line) {
		return compact
	}
	return line
}

func (a *App) statusText() string {
	if a.status != "" {
		return a.status
	}

	status := statusIcon + " 0/0"
	if _, ok := a.selectedNote(); ok {
		status = fmt.Sprintf("%s %d/%d", statusIcon, a.selected+1, len(a.notes))
		if a.filterQuery != "" {
			status = fmt.Sprintf("%s of %d  filter %q", status, len(a.sourceNotes()), a.filterQuery)
		}
		if a.activePane == paneDetail && a.detailOffset > 0 {
			status = fmt.Sprintf("%s  scroll +%d", status, a.detailOffset)
		}
	}
	return status
}

func (a *App) statusHints() string {
	switch a.statusMode {
	case statusDeleteArmed:
		return "d confirm   ↑↓ cancel   q quit"
	}

	if _, ok := a.selectedNote(); !ok {
		if a.filterQuery != "" {
			return "/ filter   Esc clear   r reload   ? help   q quit"
		}
		return "/ filter   r reload   ? help   q quit"
	}

	if a.activePane == paneDetail {
		return "↑↓ scroll   Pg page   ← list   c copy   e edit   p pin   r reload   ? help   q quit"
	}
	if a.filterQuery != "" {
		return "↑↓ nav   → body   / filter   Esc clear   c copy   p pin   e edit   d del   r reload   ? help   q quit"
	}
	return "↑↓ nav   → body   / filter   c copy   p pin   e edit   d del   r reload   ? help   q quit"
}

func (a *App) compactStatusHints() string {
	switch a.statusMode {
	case statusDeleteArmed:
		return "d ok   ↑↓ cancel   q"
	}

	if _, ok := a.selectedNote(); !ok {
		if a.filterQuery != "" {
			return "/   Esc   r   ?   q"
		}
		return "/   r   ?   q"
	}

	if a.activePane == paneDetail {
		return "↑↓   Pg   ←   c   e   p   r   ?   q"
	}
	if a.filterQuery != "" {
		return "↑↓   →   /   Esc   c   p   e   d   r   ?   q"
	}
	return "↑↓   →   /   c   p   e   d   r   ?   q"
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
	if a.showHelp {
		return nil
	}
	if a.activePane == paneDetail {
		return a.scrollDetail(g, -1)
	}

	if a.selected > 0 {
		a.selected--
		a.markSelectedRead()
		a.detailOffset = 0
		a.pendingDeleteID = ""
		a.status = ""
		a.statusMode = statusDefault
	}
	return nil
}

func (a *App) down(g *gocui.Gui, v *gocui.View) error {
	if a.showHelp {
		return nil
	}
	if a.activePane == paneDetail {
		return a.scrollDetail(g, 1)
	}

	if a.selected < len(a.notes)-1 {
		a.selected++
		a.markSelectedRead()
		a.detailOffset = 0
		a.pendingDeleteID = ""
		a.status = ""
		a.statusMode = statusDefault
	}
	return nil
}

func (a *App) focusNotes(g *gocui.Gui, v *gocui.View) error {
	if a.showHelp {
		return nil
	}
	a.activePane = paneNotes
	a.pendingDeleteID = ""
	a.status = ""
	a.statusMode = statusDefault
	return a.setCurrentView(g)
}

func (a *App) focusDetail(g *gocui.Gui, v *gocui.View) error {
	if a.showHelp {
		return nil
	}
	a.activePane = paneDetail
	a.pendingDeleteID = ""
	a.status = ""
	a.statusMode = statusDefault
	return a.setCurrentView(g)
}

func (a *App) detailUp(g *gocui.Gui, v *gocui.View) error {
	if a.showHelp {
		return nil
	}
	return a.scrollDetail(g, -detailPageSize(g))
}

func (a *App) detailDown(g *gocui.Gui, v *gocui.View) error {
	if a.showHelp {
		return nil
	}
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
	if a.showHelp {
		return nil
	}
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

	a.applyLoadedNotes(updated, "")
	a.pendingDeleteID = ""
	a.status = fmt.Sprintf("Deleted %q", note.Title)
	a.statusMode = statusMessage
	return nil
}

func (a *App) togglePin(g *gocui.Gui, v *gocui.View) error {
	if a.showHelp {
		return nil
	}
	note, ok := a.selectedNote()
	if !ok {
		return nil
	}

	updated, pinned, err := a.store.TogglePinned(note.ID)
	if err != nil {
		a.status = fmt.Sprintf("Pin failed: %v", err)
		a.statusMode = statusMessage
		return nil
	}

	a.applyLoadedNotes(updated, "")
	a.pendingDeleteID = ""
	if pinned {
		a.status = fmt.Sprintf("Pinned %q", note.Title)
	} else {
		a.status = fmt.Sprintf("Unpinned %q", note.Title)
	}
	a.statusMode = statusMessage
	return nil
}

func (a *App) copy(g *gocui.Gui, v *gocui.View) error {
	if a.showHelp {
		return nil
	}
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

func (a *App) quitOrCloseHelp(g *gocui.Gui, v *gocui.View) error {
	if a.showHelp {
		return a.closeHelp(g, v)
	}
	return gocui.ErrQuit
}

func (a *App) toggleHelp(g *gocui.Gui, v *gocui.View) error {
	if a.showHelp {
		return a.closeHelp(g, v)
	}
	if a.inputMode == inputSearch {
		return nil
	}
	a.showHelp = true
	a.pendingDeleteID = ""
	a.status = "Help"
	a.statusMode = statusMessage
	return a.setCurrentView(g)
}

func (a *App) closeHelp(g *gocui.Gui, v *gocui.View) error {
	a.showHelp = false
	if a.status == "Help" {
		a.status = ""
		a.statusMode = statusDefault
	}
	if g != nil {
		_ = g.DeleteView(helpView)
	}
	return a.setCurrentView(g)
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

	if a.showHelp {
		_, err := g.SetCurrentView(helpView)
		if errors.Is(err, gocui.ErrUnknownView) {
			return nil
		}
		return err
	}

	if a.inputMode == inputSearch {
		_, err := g.SetCurrentView(statusView)
		return err
	}

	_, err := g.SetCurrentView(a.activePane.viewName())
	return err
}

func (a *App) themeColors() Theme {
	if a.theme == (Theme{}) {
		return DefaultTheme()
	}
	return a.theme
}

func (a *App) paneFrameColor(p pane) gocui.Attribute {
	theme := a.themeColors()
	if a.activePane == p {
		return theme.ActiveBorder
	}
	return theme.InactiveBorder
}

func (a *App) paneTitleColor(p pane) gocui.Attribute {
	theme := a.themeColors()
	if a.activePane == p {
		return theme.Title
	}
	return theme.MutedFg
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

func noteSubtitle(note notes.Note) string {
	parts := []string{note.CreatedAt.Local().Format("2006-01-02 15:04")}
	if note.UpdatedAt != nil {
		parts = append(parts, "edited "+note.UpdatedAt.Local().Format("2006-01-02 15:04"))
	}
	if tags := notes.FormatTags(note.Tags); tags != "" {
		parts = append(parts, tags)
	}
	return strings.Join(parts, "  ")
}

func listLine(note notes.Note, selected, unread bool, width int) string {
	selector := " "
	if selected {
		selector = "›"
	}
	state := " "
	switch {
	case unread:
		state = "●"
	case note.Pinned:
		state = "▴"
	}
	prefix := selector + " " + state + " "

	available := width - runeLen(prefix)
	if available < 1 {
		available = 1
	}
	return prefix + padLine(fitLine(oneLine(note.Title), available), available)
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
