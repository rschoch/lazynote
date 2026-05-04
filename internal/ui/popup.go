package ui

import (
	"errors"
	"fmt"

	"github.com/awesome-gocui/gocui"
)

// Popup is the small modal surface used for help and future confirmations/menus.
type Popup struct {
	Title   string
	Lines   []string
	OnClose func()
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
	if a.inputMode == inputSearch {
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
			"c         copy selected title/body",
			"e         edit selected note",
			"p         pin or unpin selected note",
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
	for i, line := range a.popup.Lines {
		if i >= visibleLines {
			break
		}
		fmt.Fprintln(v, fitLine(line, width-2))
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
	if height < 10 {
		height = maxY - 2
	}
	if height < 1 {
		height = 1
	}
	return width, height
}
