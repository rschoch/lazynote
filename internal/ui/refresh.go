package ui

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/awesome-gocui/gocui"
	"github.com/rschoch/lazynote/internal/notes"
)

type fileSnapshot struct {
	exists  bool
	modTime int64
	size    int64
}

func (a *App) startAutoRefresh(g *gocui.Gui, interval time.Duration) func() {
	if interval <= 0 {
		return func() {}
	}

	path := a.store.Path()
	lastSnapshot, _ := snapshotFile(path)
	done := make(chan struct{})
	stopped := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		defer close(stopped)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
			}

			currentSnapshot, err := snapshotFile(path)
			if err != nil {
				refreshErr := err
				g.Update(func(*gocui.Gui) error {
					a.refreshFailed(refreshErr)
					return nil
				})
				continue
			}
			if currentSnapshot == lastSnapshot {
				continue
			}

			loaded, err := a.store.Load()
			if err != nil {
				refreshErr := err
				g.Update(func(*gocui.Gui) error {
					a.refreshFailed(refreshErr)
					return nil
				})
				continue
			}

			stableSnapshot, err := snapshotFile(path)
			if err != nil {
				refreshErr := err
				g.Update(func(*gocui.Gui) error {
					a.refreshFailed(refreshErr)
					return nil
				})
				continue
			}
			if stableSnapshot != currentSnapshot {
				continue
			}
			lastSnapshot = stableSnapshot

			g.Update(func(*gocui.Gui) error {
				a.applyLoadedNotes(loaded, "Notes updated")
				return nil
			})
		}
	}()

	return func() {
		stopOnce.Do(func() {
			close(done)
			<-stopped
		})
	}
}

func (a *App) refreshFailed(err error) {
	a.pendingDeleteID = ""
	a.status = fmt.Sprintf("Refresh failed: %v", err)
	a.statusMode = statusMessage
}

func (a *App) applyLoadedNotes(loaded []notes.Note, status string) bool {
	ordered := a.orderedNotes(loaded)
	if sameNotes(a.sourceNotes(), ordered) {
		return false
	}

	selectedID := ""
	if note, ok := a.selectedNote(); ok {
		selectedID = note.ID
	}
	addedIDs := addedNoteIDs(a.sourceNotes(), ordered)
	a.addUnread(addedIDs)
	if a.settings.AutoSelectNewNotes && len(addedIDs) > 0 {
		selectedID = newestNoteID(ordered, addedIDs)
	}

	a.allNotes = ordered
	a.applyFilter(selectedID)

	a.pendingDeleteID = ""
	if status != "" {
		if status == "Notes updated" && len(addedIDs) > 0 {
			status = newNotesStatus(len(addedIDs))
		}
		a.status = status
		a.statusMode = statusMessage
	}
	return true
}

func (a *App) reloadNotesFromDisk(status string) error {
	loaded, err := a.store.Load()
	if err != nil {
		return err
	}
	a.applyLoadedNotes(loaded, status)
	return nil
}

func (a *App) manualRefresh(g *gocui.Gui, v *gocui.View) error {
	if a.inputMode == inputSearch {
		return nil
	}

	loaded, err := a.store.Load()
	if err != nil {
		a.refreshFailed(err)
		return nil
	}
	if !a.applyLoadedNotes(loaded, "Notes refreshed") {
		a.status = "Notes already current"
		a.statusMode = statusMessage
	}
	return nil
}

func sameNotes(a, b []notes.Note) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].ID != b[i].ID ||
			a[i].Title != b[i].Title ||
			a[i].Body != b[i].Body ||
			a[i].Pinned != b[i].Pinned ||
			!a[i].CreatedAt.Equal(b[i].CreatedAt) {
			return false
		}
	}
	return true
}

func noteIndexByID(loaded []notes.Note, id string) int {
	for i, note := range loaded {
		if note.ID == id {
			return i
		}
	}
	return -1
}

func snapshotFile(path string) (fileSnapshot, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fileSnapshot{}, nil
	}
	if err != nil {
		return fileSnapshot{}, err
	}
	if info.IsDir() {
		return fileSnapshot{}, fmt.Errorf("notes path is a directory: %s", path)
	}

	return fileSnapshot{
		exists:  true,
		modTime: info.ModTime().UnixNano(),
		size:    info.Size(),
	}, nil
}
