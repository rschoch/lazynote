package ui

import (
	"errors"
	"fmt"

	"github.com/awesome-gocui/gocui"
)

// Popup is the small modal surface used for help and future confirmations/menus.
type Popup struct {
	Title    string
	Lines    []string
	Selected int
	Offset   int
	OnSelect func(*gocui.Gui, int) error
	OnToggle func(*gocui.Gui, int) error
	OnNew    func(*gocui.Gui) error
	OnClose  func()
}

func (a *App) hasPopup() bool {
	return a.popup != nil
}

func (a *App) openPopup(popup Popup) {
	a.popup = &popup
}

func (a *App) closePopupKey(g *gocui.Gui, v *gocui.View) error {
	return a.closePopup(g)
}

func (a *App) selectPopupKey(g *gocui.Gui, v *gocui.View) error {
	if a.popup == nil || a.popup.OnSelect == nil {
		return a.closePopup(g)
	}
	return a.popup.OnSelect(g, a.popup.Selected)
}

func (a *App) togglePopupKey(g *gocui.Gui, v *gocui.View) error {
	if a.popup == nil || a.popup.OnToggle == nil {
		return nil
	}
	return a.popup.OnToggle(g, a.popup.Selected)
}

func (a *App) newPopupItemKey(g *gocui.Gui, v *gocui.View) error {
	if a.popup == nil || a.popup.OnNew == nil {
		return nil
	}
	return a.popup.OnNew(g)
}

func (a *App) movePopup(delta int) error {
	if a.popup == nil || a.popup.OnSelect == nil || len(a.popup.Lines) == 0 {
		return nil
	}
	a.popup.Selected += delta
	if a.popup.Selected < 0 {
		a.popup.Selected = len(a.popup.Lines) - 1
	}
	if a.popup.Selected >= len(a.popup.Lines) {
		a.popup.Selected = 0
	}
	return nil
}

func (a *App) closePopup(g *gocui.Gui) error {
	if a.popup == nil {
		return nil
	}

	onClose := a.popup.OnClose
	a.popup = nil
	if onClose != nil {
		onClose()
	}
	if g != nil {
		_ = g.DeleteView(popupView)
	}
	return a.setCurrentView(g)
}

func (a *App) toggleHelp(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
		return a.closePopup(g)
	}
	if a.inputMode != inputNormal {
		return nil
	}

	a.pendingDeleteID = ""
	a.status = "Help"
	a.statusMode = statusMessage
	a.openPopup(Popup{
		Title: "Help",
		Lines: []string{
			"↑↓        move selection or scroll body",
			"← →       switch list/body focus",
			"Pg        page through the body",
			"/         filter title, body, or #tag",
			"Esc       clear filter or close popup",
			"n         create note",
			"c         copy selected title/body",
			"e         edit selected note",
			"p         pin or unpin selected note",
			"t         add or remove tags",
			"a         archive or restore selected note",
			"v         switch note view",
			"d         delete; press twice to confirm",
			"r         reload notes from disk",
			"?         close this popup",
			"q         close this popup",
			"Enter     close this popup",
		},
		OnClose: func() {
			if a.status == "Help" {
				a.status = ""
				a.statusMode = statusDefault
			}
		},
	})
	return a.setCurrentView(g)
}

func (a *App) layoutPopup(g *gocui.Gui, maxX, maxY int) error {
	if a.popup == nil {
		_ = g.DeleteView(popupView)
		return nil
	}

	width, height := a.popupSize(maxX, maxY)
	x0 := (maxX - width) / 2
	y0 := (maxY - height) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	theme := a.themeColors()
	v, err := g.SetView(popupView, x0, y0, x0+width, y0+height, 0)
	if err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}

	v.Title = " " + a.popup.Title + " "
	v.TitleColor = theme.Title
	v.FrameColor = theme.ActiveBorder
	v.FrameRunes = roundedFrameRunes
	v.BgColor = theme.DefaultBg
	v.FgColor = theme.DefaultFg
	v.Wrap = false
	v.Clear()

	visibleLines := height - 2
	if visibleLines < 0 {
		visibleLines = 0
	}
	if a.popup.OnSelect != nil {
		start, end, cursor := listViewport(len(a.popup.Lines), a.popup.Selected, a.popup.Offset, visibleLines)
		a.popup.Offset = start
		for i := start; i < end; i++ {
			marker := "  "
			if i == a.popup.Selected {
				marker = "› "
			}
			_, _ = fmt.Fprintln(v, fitLine(marker+a.popup.Lines[i], width-2))
		}
		_ = v.SetCursor(0, cursor)
	} else {
		for i, line := range a.popup.Lines {
			if i >= visibleLines {
				break
			}
			_, _ = fmt.Fprintln(v, fitLine(line, width-2))
		}
	}
	return nil
}

func (a *App) popupSize(maxX, maxY int) (int, int) {
	width := 50
	if a.popup != nil {
		for _, line := range a.popup.Lines {
			if lineWidth := runeLen(line) + 2; lineWidth > width {
				width = lineWidth
			}
		}
		if titleWidth := runeLen(a.popup.Title) + 4; titleWidth > width {
			width = titleWidth
		}
	}
	if maxX < width+4 {
		width = maxX - 4
	}
	if width < 30 {
		width = maxX - 2
	}
	if width < 1 {
		width = 1
	}

	height := 10
	if a.popup != nil {
		height = len(a.popup.Lines) + 4
	}
	if maxY < height+4 {
		height = maxY - 4
	}
	if height < 6 {
		height = 6
	}
	if height > maxY-2 {
		height = maxY - 2
	}
	if height < 1 {
		height = 1
	}
	return width, height
}
