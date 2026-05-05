package ui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/awesome-gocui/gocui"
	"github.com/rschoch/lazynote/internal/notes"
)

func (a *App) edit(g *gocui.Gui, v *gocui.View) error {
	if a.inputMode == inputSearch || a.hasPopup() {
		return nil
	}

	note, ok := a.selectedNote()
	if !ok {
		a.pendingDeleteID = ""
		a.status = "Nothing to edit"
		a.statusMode = statusMessage
		return nil
	}

	a.pendingDeleteID = ""
	editNote := a.editNote
	if editNote == nil {
		editNote = func(note notes.Note) (string, string, bool, error) {
			return a.withSuspendedGUI(g, func() (string, string, bool, error) {
				return EditNoteInExternalEditor(note, a.editor)
			})
		}
	}

	title, body, changed, err := editNote(note)
	if err != nil {
		a.status = fmt.Sprintf("Edit failed: %v", err)
		a.statusMode = statusMessage
		return nil
	}
	if !changed {
		a.status = "Edit unchanged"
		a.statusMode = statusMessage
		return nil
	}

	updated, changed, err := a.store.Update(note.ID, title, body)
	if err != nil {
		a.status = fmt.Sprintf("Save failed: %v", err)
		a.statusMode = statusMessage
		return nil
	}
	if !changed {
		a.status = "Edit unchanged"
		a.statusMode = statusMessage
		return nil
	}

	a.applyLoadedNotes(updated, "Saved note")
	return a.setCurrentView(g)
}

func (a *App) create(g *gocui.Gui, v *gocui.View) error {
	if a.inputMode == inputSearch || a.hasPopup() {
		return nil
	}

	a.pendingDeleteID = ""
	createNote := a.createNote
	if createNote == nil {
		createNote = func() (string, string, bool, error) {
			return a.withSuspendedGUI(g, func() (string, string, bool, error) {
				return CreateNoteInExternalEditor(a.editor)
			})
		}
	}

	title, body, created, err := createNote()
	if err != nil {
		a.status = fmt.Sprintf("Create failed: %v", err)
		a.statusMode = statusMessage
		return nil
	}
	if !created {
		a.status = "Create canceled"
		a.statusMode = statusMessage
		return nil
	}

	note, err := a.store.Append(title, body)
	if err != nil {
		a.status = fmt.Sprintf("Save failed: %v", err)
		a.statusMode = statusMessage
		return nil
	}

	loaded, err := a.store.Load()
	if err != nil {
		a.status = fmt.Sprintf("Refresh failed: %v", err)
		a.statusMode = statusMessage
		return nil
	}

	a.allNotes = a.orderedNotes(loaded)
	if a.filterQuery != "" && noteIndexByID(filterNotes(a.allNotes, a.filterQuery), note.ID) < 0 {
		a.filterQuery = ""
	}
	a.applyFilter(note.ID)
	a.detailOffset = 0
	a.status = "Created note"
	a.statusMode = statusMessage
	return a.setCurrentView(g)
}

func (a *App) withSuspendedGUI(g *gocui.Gui, run func() (string, string, bool, error)) (string, string, bool, error) {
	if g != nil {
		gocui.Suspend()
		defer func() {
			if err := gocui.Resume(); err != nil {
				a.status = fmt.Sprintf("Resume failed: %v", err)
				a.statusMode = statusMessage
			}
		}()
	}
	return run()
}

// EditNoteInExternalEditor opens a temporary editable note file in an external editor.
func EditNoteInExternalEditor(note notes.Note, editor string) (title, body string, changed bool, err error) {
	editor = resolveEditor(editor)
	tmp, err := os.CreateTemp("", "lazynote-*.md")
	if err != nil {
		return "", "", false, fmt.Errorf("create edit file: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)

	original := formatEditableNote(note.Title, note.Body)
	if _, err := tmp.WriteString(original); err != nil {
		tmp.Close()
		return "", "", false, fmt.Errorf("write edit file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", "", false, fmt.Errorf("close edit file: %w", err)
	}

	if err := runExternalEditor(editor, path); err != nil {
		return "", "", false, fmt.Errorf("run editor: %w", err)
	}

	edited, err := os.ReadFile(path)
	if err != nil {
		return "", "", false, fmt.Errorf("read edit file: %w", err)
	}
	if string(edited) == original {
		return note.Title, note.Body, false, nil
	}

	title, body, err = parseEditableNote(string(edited))
	if err != nil {
		return "", "", false, err
	}
	if title == note.Title && body == note.Body {
		return title, body, false, nil
	}
	return title, body, true, nil
}

// CreateNoteInExternalEditor opens a blank temporary note file in an external editor.
func CreateNoteInExternalEditor(editor string) (title, body string, created bool, err error) {
	editor = resolveEditor(editor)
	tmp, err := os.CreateTemp("", "lazynote-*.md")
	if err != nil {
		return "", "", false, fmt.Errorf("create edit file: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)

	if err := tmp.Close(); err != nil {
		return "", "", false, fmt.Errorf("close edit file: %w", err)
	}

	if err := runExternalEditor(editor, path); err != nil {
		return "", "", false, fmt.Errorf("run editor: %w", err)
	}

	edited, err := os.ReadFile(path)
	if err != nil {
		return "", "", false, fmt.Errorf("read edit file: %w", err)
	}
	if strings.TrimSpace(string(edited)) == "" {
		return "", "", false, nil
	}

	title, body, err = parseEditableNote(string(edited))
	if err != nil {
		return "", "", false, err
	}
	return title, body, true, nil
}

func resolveEditor(editor string) string {
	if strings.TrimSpace(editor) != "" {
		return strings.TrimSpace(editor)
	}
	if editor := strings.TrimSpace(os.Getenv("VISUAL")); editor != "" {
		return editor
	}
	if editor := strings.TrimSpace(os.Getenv("EDITOR")); editor != "" {
		return editor
	}
	return "vi"
}

func runExternalEditor(editor, path string) error {
	cmd := editorCommand(editor, path)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func editorCommand(editor, path string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", editor+" "+shellQuote(path))
	}
	return exec.Command("sh", "-c", editor+" "+shellQuote(path))
}

func formatEditableNote(title, body string) string {
	return title + "\n\n" + body + "\n"
}

func parseEditableNote(content string) (title, body string, err error) {
	content = strings.TrimRight(content, "\r\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return "", "", fmt.Errorf("note title is empty")
	}

	title = strings.TrimSpace(lines[0])
	if title == "" {
		return "", "", fmt.Errorf("note title is empty")
	}

	bodyStart := 1
	if len(lines) > 1 && strings.TrimSpace(lines[1]) == "" {
		bodyStart = 2
	}
	if bodyStart < len(lines) {
		body = strings.Join(lines[bodyStart:], "\n")
	}
	return title, body, nil
}

func shellQuote(s string) string {
	if runtime.GOOS == "windows" {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
