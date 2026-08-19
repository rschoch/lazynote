package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/awesome-gocui/gocui"
	"github.com/rschoch/lazynote/internal/notes"
)

const (
	notesView  = "notes"
	detailView = "detail"
	statusView = "status"
	popupView  = "popup"

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
	viewSource      []notes.Note
	searchIndex     map[string]normalizedNote
	unreadIDs       map[string]struct{}
	notes           []notes.Note
	selected        int
	listOffset      int
	detailOffset    int
	detailMetrics   detailMetrics
	activePane      pane
	pendingDeleteID string
	status          string
	statusMode      statusMode
	popup           *Popup
	copyText        func(string) error
	filterQuery     string
	currentView     noteView
	searchInput     string
	searchOriginal  string
	tagInput        string
	tagTargetID     string
	inputMode       inputMode
	editor          string
	editNote        func(notes.Note) (string, string, bool, error)
	createNote      func() (string, string, bool, error)
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
	inputTag
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
	a.rebuildSearchIndex()
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
		{"", 'q', a.quitOrClosePopup},
		{"", gocui.KeyCtrlC, quit},
		{"", '?', a.toggleHelp},
		{"", gocui.KeyArrowDown, a.down},
		{"", gocui.KeyArrowUp, a.up},
		{"", 'c', a.copy},
		{"", 'd', a.delete},
		{"", 'e', a.edit},
		{"", 'n', a.create},
		{"", 'p', a.togglePin},
		{"", 'a', a.toggleArchive},
		{"", 't', a.openTagPicker},
		{"", 'v', a.openViewPicker},
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
		{popupView, 'q', a.closePopupKey},
		{popupView, '?', a.closePopupKey},
		{popupView, gocui.KeyEsc, a.closePopupKey},
		{popupView, gocui.KeyEnter, a.selectPopupKey},
		{popupView, gocui.KeySpace, a.togglePopupKey},
		{popupView, 'n', a.newPopupItemKey},
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
	if a.hasPopup() {
		if err := a.layoutPopup(g, maxX, maxY); err != nil {
			return err
		}
	} else {
		_ = g.DeleteView(popupView)
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
	_, _ = fmt.Fprintln(v, "Terminal too small.")
	return nil
}

func (a *App) layoutNotes(g *gocui.Gui, x0, y0, x1, y1 int) error {
	theme := a.themeColors()
	v, err := g.SetView(notesView, x0, y0, x1, y1, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	viewed := a.viewNotes()
	if a.filterQuery != "" {
		v.Title = fmt.Sprintf(" %s %d/%d ", a.currentView.label(), len(a.notes), len(viewed))
		v.Subtitle = " /" + fitLine(a.filterQuery, 18) + " "
	} else {
		v.Title = fmt.Sprintf(" %s %d ", a.currentView.label(), len(a.notes))
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
		_, _ = fmt.Fprintln(v, a.emptyNotesMessage())
		_ = v.SetOrigin(0, 0)
		_ = v.SetCursor(0, 0)
		return nil
	}

	width, height := v.Size()
	if width < 1 {
		width = 1
	}

	start, end, cursor := listViewport(len(a.notes), a.selected, a.listOffset, height)
	a.listOffset = start
	for i := start; i < end; i++ {
		note := a.notes[i]
		_, _ = fmt.Fprintln(v, listLine(note, i == a.selected, a.isUnread(note.ID), width))
	}
	_ = v.SetOrigin(0, 0)
	_ = v.SetCursor(0, cursor)

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
			_, _ = fmt.Fprintln(v, "No matching note.")
		} else {
			_, _ = fmt.Fprintln(v, a.emptyNotesMessage()+".")
		}
		return nil
	}

	width, _ := v.Size()
	title, subtitle := detailHeader(oneLine(note.Title), noteSubtitle(note), width)
	v.Title = " " + title + " "
	v.Subtitle = ""
	if subtitle != "" {
		v.Subtitle = " " + subtitle + " "
	}
	if note.Body != "" {
		_, _ = fmt.Fprintln(v, note.Body)
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
	v.Editable = a.inputMode != inputNormal
	if v.Editable {
		v.Editor = inputEditor{app: a}
	}
	v.Clear()

	width, _ := v.Size()
	if a.inputMode == inputSearch {
		g.Cursor = true
		line := "/" + a.searchInput
		_, _ = fmt.Fprint(v, fitLine(line, width))
		_ = v.SetCursor(runeLen(line), 0)
		return nil
	}
	if a.inputMode == inputTag {
		g.Cursor = true
		line := "New tag: #" + a.tagInput
		_, _ = fmt.Fprint(v, fitLine(line, width))
		_ = v.SetCursor(runeLen(line), 0)
		return nil
	}

	g.Cursor = false
	plain := fitLine(a.statusLineForWidth(width), width)
	_, _ = fmt.Fprint(v, a.renderStatusLine(plain))

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

	smart := fmt.Sprintf(" %s   %s  … ", status, a.smartStatusHints())
	if runeLen(smart) <= width {
		return smart
	}

	compact := fmt.Sprintf(" %s   %s ", status, a.compactStatusHints())
	if runeLen(compact) < runeLen(smart) {
		return compact
	}
	return smart
}

func (a *App) statusText() string {
	if a.status != "" {
		return a.status
	}

	status := statusIcon + " 0/0"
	if _, ok := a.selectedNote(); ok {
		status = fmt.Sprintf("%s %d/%d", statusIcon, a.selected+1, len(a.notes))
		if a.filterQuery != "" {
			status = fmt.Sprintf("%s of %d  filter %q", status, len(a.viewNotes()), a.filterQuery)
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
		return "delete  ↑↓ cancel │ ? help  quit"
	}

	if _, ok := a.selectedNote(); !ok {
		if a.filterQuery != "" {
			return "/ filter  Esc clear  views │ new │ reload │ ? help  quit"
		}
		return "/ filter  views │ new │ reload │ ? help  quit"
	}

	if a.activePane == paneDetail {
		return fmt.Sprintf("↑↓ scroll  Pg page  ← list │ new  edit  copy │ %s │ delete  reload │ ? help  quit", a.organizationHints())
	}
	if a.filterQuery != "" {
		return fmt.Sprintf("↑↓ nav  → body  / filter  Esc clear  views │ new  edit  copy │ %s │ delete  reload │ ? help  quit", a.organizationHints())
	}
	return fmt.Sprintf("↑↓ nav  → body  / filter  views │ new  edit  copy │ %s │ delete  reload │ ? help  quit", a.organizationHints())
}

func (a *App) organizationHints() string {
	note, ok := a.selectedNote()
	if !ok {
		return "tags"
	}
	if note.Archived {
		if note.Pinned {
			return "unpin  tags  unarchive"
		}
		return "tags  unarchive"
	}
	if note.Pinned {
		return "unpin  tags  archive"
	}
	return "pin  tags  archive"
}

func (a *App) smartStatusHints() string {
	switch a.statusMode {
	case statusDeleteArmed:
		return "delete  ↑↓ cancel │ ? help"
	}

	if _, ok := a.selectedNote(); !ok {
		if a.filterQuery != "" {
			return "/ filter  Esc clear  views │ new │ ? help"
		}
		return "/ filter  views │ new │ ? help"
	}

	if a.activePane == paneDetail {
		return "↑↓ scroll  ← list │ edit  copy │ ? help"
	}
	if a.filterQuery != "" {
		return "↑↓ nav  → body  / filter  Esc clear  views │ new │ ? help"
	}
	return "↑↓ nav  → body  / filter  views │ new │ ? help"
}

var highlightedStatusWords = []struct {
	word string
	key  rune
}{
	{"unarchive", 'a'},
	{"archive", 'a'},
	{"copy", 'c'},
	{"delete", 'd'},
	{"edit", 'e'},
	{"new", 'n'},
	{"unpin", 'p'},
	{"pin", 'p'},
	{"quit", 'q'},
	{"reload", 'r'},
	{"tags", 't'},
	{"views", 'v'},
}

func (a *App) emptyNotesMessage() string {
	if a.filterQuery != "" {
		return "No matches"
	}
	switch a.currentView.kind {
	case viewPinned:
		return "No pinned notes"
	case viewRecent:
		return "No recent notes"
	case viewUntagged:
		return "No untagged notes"
	case viewArchived:
		return "No archived notes"
	case viewTag:
		return "No notes tagged #" + a.currentView.tag
	default:
		return "No active notes"
	}
}

func (a *App) renderStatusLine(line string) string {
	statusPrefix := " " + a.statusText() + "   "
	if !strings.HasPrefix(line, statusPrefix) {
		return line
	}
	hints := strings.TrimPrefix(line, statusPrefix)
	return statusPrefix + a.highlightStatusHints(hints)
}

func (a *App) highlightStatusHints(hints string) string {
	accent := ansiAttribute(a.themeColors().ActiveBorder)
	if accent == "" {
		return hints
	}
	replacerArgs := make([]string, 0, len(highlightedStatusWords)*2)
	for _, hint := range highlightedStatusWords {
		runes := []rune(hint.word)
		keyIndex := -1
		for i, r := range runes {
			if r == hint.key {
				keyIndex = i
				break
			}
		}
		if keyIndex < 0 {
			continue
		}
		highlighted := string(runes[:keyIndex]) + accent + string(runes[keyIndex]) + "\x1b[0m" + string(runes[keyIndex+1:])
		replacerArgs = append(replacerArgs, hint.word, highlighted)
	}
	return strings.NewReplacer(replacerArgs...).Replace(hints)
}

func ansiAttribute(attr gocui.Attribute) string {
	params := make([]string, 0, 8)
	if attr.IsValidColor() {
		r, g, b := attr.RGB()
		if r >= 0 && g >= 0 && b >= 0 {
			params = append(params, "38", "2", strconv.Itoa(int(r)), strconv.Itoa(int(g)), strconv.Itoa(int(b)))
		}
	}
	for _, effect := range []struct {
		attr gocui.Attribute
		code string
	}{
		{gocui.AttrBold, "1"},
		{gocui.AttrDim, "2"},
		{gocui.AttrItalic, "3"},
		{gocui.AttrUnderline, "4"},
		{gocui.AttrReverse, "7"},
		{gocui.AttrStrikeThrough, "9"},
	} {
		if attr&effect.attr != 0 {
			params = append(params, effect.code)
		}
	}
	if len(params) == 0 {
		return ""
	}
	return "\x1b[" + strings.Join(params, ";") + "m"
}

func (a *App) compactStatusHints() string {
	switch a.statusMode {
	case statusDeleteArmed:
		return "d ↑↓ ?"
	}

	if _, ok := a.selectedNote(); !ok {
		if a.filterQuery != "" {
			return "/ Esc v │ n │ r │ ? q"
		}
		return "/ v │ n │ r │ ? q"
	}

	if a.activePane == paneDetail {
		return "↑↓ Pg ← │ n e c │ p t a │ d r │ ? q"
	}
	if a.filterQuery != "" {
		return "↑↓ → / Esc v │ n e c │ p t a │ d r │ ? q"
	}
	return "↑↓ → / v │ n e c │ p t a │ d r │ ? q"
}

func listViewport(total, selected, offset, height int) (start, end, cursor int) {
	if total <= 0 {
		return 0, 0, 0
	}
	if height < 1 {
		height = 1
	}
	if selected < 0 {
		selected = 0
	}
	if selected >= total {
		selected = total - 1
	}

	maxStart := total - height
	if maxStart < 0 {
		maxStart = 0
	}
	if offset < 0 {
		offset = 0
	}
	if offset > maxStart {
		offset = maxStart
	}
	if selected < offset {
		offset = selected
	}
	if selected >= offset+height {
		offset = selected - height + 1
	}

	end = offset + height
	if end > total {
		end = total
	}
	return offset, end, selected - offset
}

func (a *App) selectedNote() (notes.Note, bool) {
	if a.selected < 0 || a.selected >= len(a.notes) {
		return notes.Note{}, false
	}
	return a.notes[a.selected], true
}

func (a *App) up(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
		return a.movePopup(-1)
	}
	if a.activePane == paneDetail {
		return a.scrollDetail(g, -1)
	}

	if len(a.notes) > 0 {
		if a.selected <= 0 {
			a.selected = len(a.notes) - 1
		} else {
			a.selected--
		}
		a.markSelectedRead()
		a.detailOffset = 0
		a.pendingDeleteID = ""
		a.status = ""
		a.statusMode = statusDefault
	}
	return nil
}

func (a *App) down(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
		return a.movePopup(1)
	}
	if a.activePane == paneDetail {
		return a.scrollDetail(g, 1)
	}

	if len(a.notes) > 0 {
		if a.selected < 0 || a.selected >= len(a.notes)-1 {
			a.selected = 0
		} else {
			a.selected++
		}
		a.markSelectedRead()
		a.detailOffset = 0
		a.pendingDeleteID = ""
		a.status = ""
		a.statusMode = statusDefault
	}
	return nil
}

func (a *App) focusNotes(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
		return nil
	}
	a.activePane = paneNotes
	a.pendingDeleteID = ""
	a.status = ""
	a.statusMode = statusDefault
	return a.setCurrentView(g)
}

func (a *App) focusDetail(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
		return nil
	}
	a.activePane = paneDetail
	a.pendingDeleteID = ""
	a.status = ""
	a.statusMode = statusDefault
	return a.setCurrentView(g)
}

func (a *App) detailUp(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
		return nil
	}
	return a.scrollDetail(g, -detailPageSize(g))
}

func (a *App) detailDown(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
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
	if a.hasPopup() {
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

	updated, _, err := a.store.Delete(note.ID)
	if err != nil {
		if errors.Is(err, notes.ErrNoteNotFound) {
			a.applyLoadedNotes(updated, "")
			a.pendingDeleteID = ""
			a.status = fmt.Sprintf("%q was already deleted", note.Title)
			a.statusMode = statusMessage
			return nil
		}
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
	if a.hasPopup() {
		return nil
	}
	note, ok := a.selectedNote()
	if !ok {
		return nil
	}
	if note.Archived && !note.Pinned {
		a.status = "Restore note before pinning"
		a.statusMode = statusMessage
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
	if a.hasPopup() {
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
	maxOffset := a.cachedDetailMaxOffset(note, width, height)
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

	maxOffset := a.cachedDetailMaxOffset(note, width, height)
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

func (a *App) quitOrClosePopup(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
		return a.closePopup(g)
	}
	return gocui.ErrQuit
}

func (a *App) writeClipboard(text string) error {
	if a.copyText != nil {
		return a.copyText(text)
	}
	return writePlatformClipboard(text)
}

func (a *App) setCurrentView(g *gocui.Gui) error {
	if g == nil {
		return nil
	}

	if a.hasPopup() {
		_, err := g.SetCurrentView(popupView)
		if errors.Is(err, gocui.ErrUnknownView) {
			return nil
		}
		return err
	}

	if a.inputMode != inputNormal {
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

func detailHeader(title, subtitle string, width int) (string, string) {
	// gocui positions titles and subtitles with additional frame padding that
	// is not included in View.Size. Reserve enough room for both labels and a
	// visible run of frame characters between them.
	available := width - 12
	if available < 1 {
		return fitLine(title, 1), ""
	}
	if subtitle == "" {
		return fitLine(title, available), ""
	}
	if runeLen(title)+runeLen(subtitle)+2 <= available {
		return title, subtitle
	}

	const minTitleWidth = 12
	subtitleBudget := available / 3
	if subtitleBudget < 16 {
		subtitleBudget = 16
	}
	titleBudget := available - subtitleBudget - 2
	if titleBudget < minTitleWidth {
		return fitLine(title, available), ""
	}
	if runeLen(title) < titleBudget {
		titleBudget = runeLen(title)
		subtitleBudget = available - titleBudget - 2
	}
	return fitLine(title, titleBudget), fitLine(subtitle, subtitleBudget)
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
	return utf8.RuneCountInString(s)
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

type detailMetrics struct {
	valid     bool
	noteID    string
	body      string
	width     int
	height    int
	maxOffset int
}

func (a *App) cachedDetailMaxOffset(note notes.Note, width, height int) int {
	cache := &a.detailMetrics
	if cache.valid && cache.noteID == note.ID && cache.body == note.Body && cache.width == width && cache.height == height {
		return cache.maxOffset
	}

	maxOffset := maxDetailOffset(note.Body, width, height)
	*cache = detailMetrics{
		valid:     true,
		noteID:    note.ID,
		body:      note.Body,
		width:     width,
		height:    height,
		maxOffset: maxOffset,
	}
	return maxOffset
}

func visualLineCount(s string, width int) int {
	if width < 1 || s == "" {
		return 0
	}

	total := 0
	for {
		line := s
		hasMore := false
		if newline := strings.IndexByte(s, '\n'); newline >= 0 {
			line = s[:newline]
			s = s[newline+1:]
			hasMore = true
		} else {
			s = ""
		}
		length := runeLen(line)
		if length == 0 {
			total++
		} else {
			total += (length + width - 1) / width
		}
		if !hasMore {
			break
		}
	}
	return total
}
