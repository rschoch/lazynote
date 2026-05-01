package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/rschoch/lazynote/internal/config"
	"github.com/rschoch/lazynote/internal/notes"
	"github.com/rschoch/lazynote/internal/ui"
)

const (
	maxInferredTitleRunes = 80
	noteSuccessMessage    = "✎ Noted!"
)

type runOptions struct {
	quiet         bool
	literalAppend bool
}

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
	builtBy = "source"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	var opts runOptions
	args, opts = parseGlobalOptions(args)

	if handled, err := runMetaCommand(args, stdout); handled || err != nil {
		return err
	}

	path, err := notes.DefaultPath()
	if err != nil {
		return err
	}
	store := notes.NewStore(path)

	if !opts.literalAppend {
		if handled, err := runStoreCommand(store, args, stdout); handled || err != nil {
			return err
		}
	}

	return runAppendOrTUI(store, args, stdin, stdout, opts)
}

func runMetaCommand(args []string, stdout io.Writer) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "--help", "-h", "help":
		printUsage(stdout)
		return true, nil
	case "--version", "-v", "version":
		fmt.Fprintln(stdout, versionString())
		return true, nil
	default:
		return false, nil
	}
}

func runStoreCommand(store *notes.Store, args []string, stdout io.Writer) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch args[0] {
	case "list":
		return true, runList(store, args[1:], stdout)
	case "show":
		return true, runShow(store, args[1:], stdout)
	case "search":
		return true, runSearch(store, args[1:], stdout)
	case "path":
		return true, runPath(store, args[1:], stdout)
	case "export":
		return true, runExport(store, args[1:], stdout)
	default:
		return false, nil
	}
}

func runAppendOrTUI(store *notes.Store, args []string, stdin io.Reader, stdout io.Writer, opts runOptions) error {
	title, body, appendNote, err := noteInput(args, stdin)
	if err != nil {
		return err
	}
	if !appendNote {
		theme, err := config.LoadTheme()
		if err != nil {
			return err
		}
		return ui.New(store, ui.WithTheme(theme)).Run()
	}
	if _, err := store.Append(title, body); err != nil {
		return err
	}

	if !opts.quiet {
		fmt.Fprintln(stdout, noteSuccessMessage)
	}
	return nil
}

func parseGlobalOptions(args []string) ([]string, runOptions) {
	var opts runOptions
	parsed := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--quiet", "-q":
			opts.quiet = true
		case "--":
			opts.literalAppend = true
			parsed = append(parsed, args[i+1:]...)
			return parsed, opts
		default:
			parsed = append(parsed, args[i])
		}
	}
	return parsed, opts
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `Usage:
  lazynote [--quiet] <title> <note>
  lazynote [--quiet] <title> -
  lazynote -- <title> <note>
  command | lazynote <title>
  command | lazynote
  lazynote list
  lazynote show [--body] <id>
  lazynote search <query>
  lazynote path
  lazynote export markdown
  lazynote export json
  lazynote
  lazynote --version
  lazynote --help

Commands:
  <title> <note>  Append a note from arguments
  <title> -       Append a note using stdin as the body
  -- <title> <note>
                  Append a note whose title starts with a command or flag
  list            List note IDs, timestamps, and titles
  show <id>       Print one note by ID or unique ID prefix
  show --body <id>
                  Print only the note body
  search <query>  List notes matching title or body text
  path            Print the notes JSON file path
  export markdown Export all notes as Markdown
  export json     Export all notes as JSON
  version         Print version information
  help            Print this help text

Options:
  --quiet, -q     Suppress the success message after appending a note

Environment:
  LAZYNOTE_PATH   Override the notes JSON file path
`)
}

func versionString() string {
	return fmt.Sprintf("lazynote %s (commit %s, built %s by %s)", version, commit, date, builtBy)
}

func runList(store *notes.Store, args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: lazynote list")
	}

	loaded, err := store.Load()
	if err != nil {
		return err
	}
	for _, note := range loaded {
		fmt.Fprintln(stdout, noteSummary(note))
	}
	return nil
}

func runShow(store *notes.Store, args []string, stdout io.Writer) error {
	id, bodyOnly, err := parseShowArgs(args)
	if err != nil {
		return err
	}

	loaded, err := store.Load()
	if err != nil {
		return err
	}
	note, err := findNote(loaded, id)
	if err != nil {
		return err
	}

	if bodyOnly {
		fmt.Fprintln(stdout, note.Body)
		return nil
	}

	fmt.Fprintf(stdout, "id: %s\n", note.ID)
	fmt.Fprintf(stdout, "created_at: %s\n", note.CreatedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(stdout, "title: %s\n\n", note.Title)
	fmt.Fprintln(stdout, note.Body)
	return nil
}

func parseShowArgs(args []string) (id string, bodyOnly bool, err error) {
	for _, arg := range args {
		switch arg {
		case "--body", "-b":
			bodyOnly = true
		default:
			if strings.HasPrefix(arg, "-") {
				return "", false, fmt.Errorf("usage: lazynote show [--body] <id>")
			}
			if id != "" {
				return "", false, fmt.Errorf("usage: lazynote show [--body] <id>")
			}
			id = arg
		}
	}
	if id == "" {
		return "", false, fmt.Errorf("usage: lazynote show [--body] <id>")
	}
	return id, bodyOnly, nil
}

func runSearch(store *notes.Store, args []string, stdout io.Writer) error {
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return fmt.Errorf("usage: lazynote search <query>")
	}

	loaded, err := store.Load()
	if err != nil {
		return err
	}

	needle := strings.ToLower(query)
	for _, note := range loaded {
		if strings.Contains(strings.ToLower(note.Title), needle) || strings.Contains(strings.ToLower(note.Body), needle) {
			fmt.Fprintln(stdout, noteSummary(note))
		}
	}
	return nil
}

func runPath(store *notes.Store, args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: lazynote path")
	}

	fmt.Fprintln(stdout, store.Path())
	return nil
}

func runExport(store *notes.Store, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: lazynote export <markdown|json>")
	}

	loaded, err := store.Load()
	if err != nil {
		return err
	}

	switch args[0] {
	case "markdown", "md":
		return writeMarkdownExport(stdout, loaded)
	case "json":
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(loaded)
	default:
		return fmt.Errorf("usage: lazynote export <markdown|json>")
	}
}

func writeMarkdownExport(w io.Writer, loaded []notes.Note) error {
	if _, err := fmt.Fprintln(w, "# lazynote export"); err != nil {
		return err
	}

	for _, note := range loaded {
		if _, err := fmt.Fprintf(w, "\n## %s\n\n", note.Title); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- id: `%s`\n", note.ID); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- created_at: `%s`\n\n", note.CreatedAt.UTC().Format(time.RFC3339)); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, note.Body); err != nil {
			return err
		}
	}

	return nil
}

func noteSummary(note notes.Note) string {
	return strings.Join([]string{
		note.ID,
		note.CreatedAt.UTC().Format(time.RFC3339),
		oneLine(note.Title),
	}, "\t")
}

func findNote(loaded []notes.Note, id string) (notes.Note, error) {
	for _, note := range loaded {
		if note.ID == id {
			return note, nil
		}
	}

	var matches []notes.Note
	for _, note := range loaded {
		if strings.HasPrefix(note.ID, id) {
			matches = append(matches, note)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return notes.Note{}, fmt.Errorf("note not found: %s", id)
	default:
		return notes.Note{}, fmt.Errorf("ambiguous note ID prefix: %s", id)
	}
}

func noteInput(args []string, stdin io.Reader) (title, body string, appendNote bool, err error) {
	switch {
	case len(args) == 0:
		if !stdinHasData(stdin) {
			return "", "", false, nil
		}

		body, err := readBody(stdin)
		if err != nil {
			return "", "", false, err
		}
		return inferTitle(body), body, true, nil
	case len(args) == 1:
		if !stdinHasData(stdin) {
			return "", "", false, fmt.Errorf("usage: lazynote <title> <note>")
		}

		body, err := readBody(stdin)
		if err != nil {
			return "", "", false, err
		}
		return args[0], body, true, nil
	case len(args) == 2 && args[1] == "-":
		body, err := readBody(stdin)
		if err != nil {
			return "", "", false, err
		}
		return args[0], body, true, nil
	default:
		return args[0], strings.Join(args[1:], " "), true, nil
	}
}

func stdinHasData(stdin io.Reader) bool {
	if stdin == nil {
		return false
	}

	file, ok := stdin.(*os.File)
	if !ok {
		return true
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice == 0
}

func readBody(stdin io.Reader) (string, error) {
	if stdin == nil {
		return "", fmt.Errorf("note body is empty")
	}

	data, err := io.ReadAll(stdin)
	if err != nil {
		return "", fmt.Errorf("read note body from stdin: %w", err)
	}

	body := strings.TrimRight(string(data), "\r\n")
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("note body is empty")
	}
	return body, nil
}

func inferTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = cleanTitleLine(line)
		if line != "" {
			return truncateRunes(line, maxInferredTitleRunes)
		}
	}
	return "Untitled"
}

func cleanTitleLine(line string) string {
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "`*_")
	return strings.TrimSpace(line)
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}

	runes := []rune(s)
	return string(runes[:max])
}
