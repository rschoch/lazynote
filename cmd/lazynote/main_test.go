package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rschoch/lazynote/internal/notes"
)

func TestRunAppendsNote(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	var stdout bytes.Buffer
	if err := run([]string{"todo", "finish", "the", "slice"}, nil, &stdout); err != nil {
		t.Fatalf("run append: %v", err)
	}
	if got, want := stdout.String(), "noted\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}

	loaded, err := notes.NewStore(path).Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d notes, want 1", len(loaded))
	}
	if loaded[0].Title != "todo" || loaded[0].Body != "finish the slice" {
		t.Fatalf("stored unexpected note: %#v", loaded[0])
	}
}

func TestRunRequiresTitleAndBody(t *testing.T) {
	t.Setenv("LAZYNOTE_PATH", filepath.Join(t.TempDir(), "notes.json"))

	if err := run([]string{"title-only"}, nil, os.Stdout); err == nil {
		t.Fatal("run returned nil error, want usage error")
	}
}

func TestRunPrintsVersion(t *testing.T) {
	var stdout bytes.Buffer
	if err := run([]string{"--version"}, nil, &stdout); err != nil {
		t.Fatalf("run version: %v", err)
	}

	got := stdout.String()
	if !bytes.Contains([]byte(got), []byte("lazynote ")) {
		t.Fatalf("stdout = %q, want lazynote version", got)
	}
	if !bytes.Contains([]byte(got), []byte("commit ")) {
		t.Fatalf("stdout = %q, want commit metadata", got)
	}
}

func TestRunPrintsHelp(t *testing.T) {
	var stdout bytes.Buffer
	if err := run([]string{"--help"}, nil, &stdout); err != nil {
		t.Fatalf("run help: %v", err)
	}

	got := stdout.String()
	if !bytes.Contains([]byte(got), []byte("Usage:")) {
		t.Fatalf("stdout = %q, want usage text", got)
	}
	if !bytes.Contains([]byte(got), []byte("LAZYNOTE_PATH")) {
		t.Fatalf("stdout = %q, want environment help", got)
	}
}

func TestRunPrintsNotesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	var stdout bytes.Buffer
	if err := run([]string{"path"}, nil, &stdout); err != nil {
		t.Fatalf("run path: %v", err)
	}

	if got, want := stdout.String(), path+"\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
}

func TestRunAppendsNoteFromStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	var stdout bytes.Buffer
	stdin := strings.NewReader("first line\nsecond line\n")
	if err := run([]string{"summary"}, stdin, &stdout); err != nil {
		t.Fatalf("run append stdin: %v", err)
	}
	if got, want := stdout.String(), "noted\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}

	loaded, err := notes.NewStore(path).Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d notes, want 1", len(loaded))
	}
	if loaded[0].Title != "summary" || loaded[0].Body != "first line\nsecond line" {
		t.Fatalf("stored unexpected note: %#v", loaded[0])
	}
}

func TestRunAppendsNoteFromExplicitStdinBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	var stdout bytes.Buffer
	stdin := strings.NewReader("body from file\n")
	if err := run([]string{"from-file", "-"}, stdin, &stdout); err != nil {
		t.Fatalf("run append stdin dash: %v", err)
	}

	loaded, err := notes.NewStore(path).Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if loaded[0].Title != "from-file" || loaded[0].Body != "body from file" {
		t.Fatalf("stored unexpected note: %#v", loaded[0])
	}
}

func TestRunAppendsPipedNoteWithInferredTitle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	var stdout bytes.Buffer
	stdin := strings.NewReader("\n## Session abc123\n- discussed release packaging\n")
	if err := run(nil, stdin, &stdout); err != nil {
		t.Fatalf("run append inferred title: %v", err)
	}

	loaded, err := notes.NewStore(path).Load()
	if err != nil {
		t.Fatalf("load notes: %v", err)
	}
	if loaded[0].Title != "Session abc123" {
		t.Fatalf("title = %q, want inferred markdown heading", loaded[0].Title)
	}
	if loaded[0].Body != "\n## Session abc123\n- discussed release packaging" {
		t.Fatalf("body = %q, want piped body without final newline", loaded[0].Body)
	}
}

func TestRunRejectsEmptyStdinBody(t *testing.T) {
	t.Setenv("LAZYNOTE_PATH", filepath.Join(t.TempDir(), "notes.json"))

	var stdout bytes.Buffer
	if err := run([]string{"empty"}, strings.NewReader("\n\n"), &stdout); err == nil {
		t.Fatal("run returned nil error, want empty body error")
	}
}

func TestRunListsNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	store := notes.NewStore(path)
	first, err := store.Append("first note", "first body")
	if err != nil {
		t.Fatalf("append first note: %v", err)
	}
	second, err := store.Append("second note", "second body")
	if err != nil {
		t.Fatalf("append second note: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"list"}, nil, &stdout); err != nil {
		t.Fatalf("run list: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{first.ID, "first note", second.ID, "second note"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
	if !strings.Contains(got, "\t") {
		t.Fatalf("stdout = %q, want tab-separated output", got)
	}
}

func TestRunShowsNoteByIDPrefix(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	note, err := notes.NewStore(path).Append("session summary", "line one\nline two")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"show", note.ID[:6]}, nil, &stdout); err != nil {
		t.Fatalf("run show: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{"id: " + note.ID, "title: session summary", "line one\nline two"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
}

func TestRunShowsOnlyNoteBody(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	note, err := notes.NewStore(path).Append("session summary", "line one\nline two")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"show", note.ID[:6], "--body"}, nil, &stdout); err != nil {
		t.Fatalf("run show body: %v", err)
	}

	if got, want := stdout.String(), "line one\nline two\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if strings.Contains(stdout.String(), "id:") || strings.Contains(stdout.String(), "title:") {
		t.Fatalf("stdout = %q, want body only", stdout.String())
	}
}

func TestRunSearchesNotes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	store := notes.NewStore(path)
	matching, err := store.Append("release plan", "ship packages")
	if err != nil {
		t.Fatalf("append matching note: %v", err)
	}
	other, err := store.Append("grocery list", "eggs")
	if err != nil {
		t.Fatalf("append other note: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"search", "PACKAGES"}, nil, &stdout); err != nil {
		t.Fatalf("run search: %v", err)
	}

	got := stdout.String()
	if !strings.Contains(got, matching.ID) || !strings.Contains(got, "release plan") {
		t.Fatalf("stdout = %q, want matching note", got)
	}
	if strings.Contains(got, other.ID) || strings.Contains(got, "grocery list") {
		t.Fatalf("stdout = %q, want non-matching note omitted", got)
	}
}

func TestRunExportsNotesAsMarkdown(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	note, err := notes.NewStore(path).Append("release plan", "ship packages")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"export", "markdown"}, nil, &stdout); err != nil {
		t.Fatalf("run export markdown: %v", err)
	}

	got := stdout.String()
	for _, want := range []string{"# lazynote export", "## release plan", "- id: `" + note.ID + "`", "ship packages"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stdout = %q, want %q", got, want)
		}
	}
}

func TestRunExportsNotesAsJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.json")
	t.Setenv("LAZYNOTE_PATH", path)

	note, err := notes.NewStore(path).Append("release plan", "ship packages")
	if err != nil {
		t.Fatalf("append note: %v", err)
	}

	var stdout bytes.Buffer
	if err := run([]string{"export", "json"}, nil, &stdout); err != nil {
		t.Fatalf("run export json: %v", err)
	}

	var exported []notes.Note
	if err := json.Unmarshal(stdout.Bytes(), &exported); err != nil {
		t.Fatalf("unmarshal exported json: %v\n%s", err, stdout.String())
	}
	if len(exported) != 1 {
		t.Fatalf("exported %d notes, want 1", len(exported))
	}
	if exported[0].ID != note.ID || exported[0].Title != "release plan" || exported[0].Body != "ship packages" {
		t.Fatalf("exported unexpected note: %#v", exported[0])
	}
}

func TestRunShowReturnsErrorForMissingNote(t *testing.T) {
	t.Setenv("LAZYNOTE_PATH", filepath.Join(t.TempDir(), "notes.json"))

	var stdout bytes.Buffer
	if err := run([]string{"show", "missing"}, nil, &stdout); err == nil {
		t.Fatal("run returned nil error, want missing note error")
	}
}
