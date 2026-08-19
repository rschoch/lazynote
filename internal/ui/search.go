package ui

import (
	"errors"

	"github.com/awesome-gocui/gocui"
)

type inputEditor struct {
	app *App
}

func (e inputEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	if e.app == nil || e.app.inputMode == inputNormal {
		return
	}
	if e.app.inputMode == inputTag {
		e.editTag(key, ch, mod)
		return
	}

	switch key {
	case gocui.KeyBackspace, gocui.KeyBackspace2:
		if e.app.searchInput != "" {
			runes := []rune(e.app.searchInput)
			e.app.searchInput = string(runes[:len(runes)-1])
			e.app.setFilterQuery(e.app.searchInput)
		}
	case gocui.KeyCtrlU:
		e.app.searchInput = ""
		e.app.setFilterQuery("")
	default:
		if ch != 0 && mod == gocui.ModNone {
			e.app.searchInput += string(ch)
			e.app.setFilterQuery(e.app.searchInput)
		}
	}
}

func (e inputEditor) editTag(key gocui.Key, ch rune, mod gocui.Modifier) {
	switch key {
	case gocui.KeyBackspace, gocui.KeyBackspace2:
		if e.app.tagInput != "" {
			runes := []rune(e.app.tagInput)
			e.app.tagInput = string(runes[:len(runes)-1])
		}
	case gocui.KeyCtrlU:
		e.app.tagInput = ""
	default:
		if ch != 0 && mod == gocui.ModNone {
			e.app.tagInput += string(ch)
		}
	}
}

func (a *App) startSearch(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
		return nil
	}
	a.inputMode = inputSearch
	a.searchOriginal = a.filterQuery
	a.searchInput = a.filterQuery
	a.pendingDeleteID = ""
	a.status = ""
	a.statusMode = statusDefault

	if g == nil {
		return nil
	}
	if _, err := g.SetCurrentView(statusView); err != nil && !errors.Is(err, gocui.ErrUnknownView) {
		return err
	}
	return nil
}

func (a *App) confirmSearch(g *gocui.Gui, v *gocui.View) error {
	if a.inputMode == inputTag {
		return a.confirmNewTag(g)
	}
	if a.inputMode != inputSearch {
		return nil
	}

	a.inputMode = inputNormal
	a.setFilterQuery(a.searchInput)
	if a.filterQuery == "" {
		a.status = "Filter cleared"
	} else {
		a.status = "Filter applied"
	}
	a.statusMode = statusMessage
	return a.setCurrentView(g)
}

func (a *App) cancelSearch(g *gocui.Gui, v *gocui.View) error {
	if a.inputMode == inputTag {
		a.inputMode = inputNormal
		a.tagInput = ""
		a.tagTargetID = ""
		a.status = "Tag canceled"
		a.statusMode = statusMessage
		return a.setCurrentView(g)
	}
	if a.inputMode != inputSearch {
		return nil
	}

	a.inputMode = inputNormal
	a.searchInput = ""
	a.setFilterQuery(a.searchOriginal)
	a.status = "Search canceled"
	a.statusMode = statusMessage
	return a.setCurrentView(g)
}

func (a *App) clearFilterKey(g *gocui.Gui, v *gocui.View) error {
	if a.hasPopup() {
		return a.closePopup(g)
	}
	if a.inputMode == inputSearch {
		return a.cancelSearch(g, v)
	}
	a.clearFilter()
	return nil
}
