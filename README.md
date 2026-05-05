# lazynote

[![CI](https://github.com/rschoch/lazynote/actions/workflows/ci.yml/badge.svg)](https://github.com/rschoch/lazynote/actions/workflows/ci.yml)
[![GitHub Releases](https://img.shields.io/github/downloads/rschoch/lazynote/total)](https://github.com/rschoch/lazynote/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/rschoch/lazynote)](https://goreportcard.com/report/github.com/rschoch/lazynote)
[![GitHub tag](https://img.shields.io/github/v/tag/rschoch/lazynote?color=blue)](https://github.com/rschoch/lazynote/releases/latest)

`lazynote` is a local, terminal-first notes app for developer workflows.

Take notes instantly from the CLI, browse them in a small TUI, and expose the
same notes to shell scripts, terminal tools, and coding agents. It is built for
quick context capture without an account, server, database, or sync service.

Notes are stored locally as JSON. The released binary does not require a Go
toolchain.

## Contents

- [Why lazynote?](#why-lazynote)
- [Quick Example](#quick-example)
- [Install](#install)
- [CLI Workflows](#cli-workflows)
- [Agent Plugins](#agent-plugins)
  - [Codex Install](#codex-install)
  - [Claude Code Install](#claude-code-install)
- [TUI](#tui)
  - [TUI Behavior](#tui-behavior)
  - [Themes](#themes)
- [Storage](#storage)
- [Development](#development)
- [Roadmap](#roadmap)
- [Releases](#releases)
- [Acknowledgements](#acknowledgements)
- [License](#license)

## Why lazynote?

- Capture notes from arguments, stdin, and shell pipelines.
- Retrieve context with plain commands such as `list`, `show`, `search`, and
  `export`.
- Share one local notes file between humans, scripts, and coding agents.
- Browse notes in a fast terminal UI when you want a human view.

## Quick Example

```sh
# take simple note: $ lazynote <title> <body>
lazynote showerthought 'Running from the cops is the ultimate double or nothing.'

# tag a note while capturing it
lazynote --tag work idea 'Use a single notes file so agents and humans share context.'

# piped body with an inferred title
printf '## Session abc123\n- fixed flaky test\n' | lazynote

# retrieve context
lazynote list
lazynote search flaky
lazynote search '#work'
lazynote export json
```

![lazynote TUI screenshot](assets/screenshot.png)

## Install

Recommended for Linux and macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/rschoch/lazynote/main/install.sh | sh
```

The installer downloads the latest release, verifies checksums when possible,
and installs `lazynote` to `~/.local/bin`.

If your shell cannot find it:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Inspect the installer first:

```sh
curl -fsSLO https://raw.githubusercontent.com/rschoch/lazynote/main/install.sh
sh install.sh
```

Installer options:

```sh
sh install.sh --dir /usr/local/bin
sh install.sh --version vX.Y.Z
```

Uninstall a script-installed binary:

```sh
rm -f ~/.local/bin/lazynote
```

Prebuilt archives and Linux `.deb`, `.rpm`, and `.apk` packages are available on
the [GitHub Releases](https://github.com/rschoch/lazynote/releases) page. Direct
downloads do not add an apt/yum/apk repository, so package-manager auto-updates
are not configured yet.

From source:

```sh
go install github.com/rschoch/lazynote/cmd/lazynote@latest
```

From a checkout:

```sh
make build
make install
```

`make install` uses `/usr/local` by default. Use `PREFIX` for another root:

```sh
make install PREFIX="$HOME/.local"
```

## CLI Workflows

Capture a note:

```sh
lazynote idea 'Use a single notes file so agents and humans share context.'
lazynote --tag work --tag idea idea 'Keep note metadata small and useful.'
```

Capture from stdin:

```sh
echo 'Refactor release notes before tagging the next release.' | lazynote release
cat summary.md | lazynote 'session summary'
lazynote 'session summary' - < summary.md
```

If stdin is piped without a title, the first non-empty line becomes the title:

```sh
printf '## Session abc123\n- shipped release prep\n' | lazynote
```

Suppress success output for scripts and agents:

```sh
some-command | lazynote --quiet 'session summary'
lazynote --quiet release 'Tag after CI passes.'
```

Retrieve context:

```sh
lazynote list
lazynote show <id>
lazynote show --body <id>
lazynote search packaging
lazynote search '#work'
lazynote backup
lazynote export markdown
lazynote export json
lazynote path
```

`list` prints tab-separated `id`, `created_at`, and `title` fields, plus a
metadata field when a note is pinned or tagged. `show` accepts a full ID or a
unique ID prefix.

Manage existing notes:

```sh
lazynote edit <id>
lazynote edit <id> 'new title' 'new body'
lazynote edit <id> 'new title' - < body.md
lazynote delete <id>
lazynote pin <id>
lazynote unpin <id>
lazynote tag <id> work idea
lazynote untag <id> work
lazynote tags <id>
```

`edit <id>` opens `$VISUAL`, `$EDITOR`, or `vi`. Direct edit commands replace
the title and body without opening an editor. `tag` normalizes tags to
lower-case names and accepts optional leading `#`.

`backup` prints the backup file path. With no path it writes a timestamped JSON
copy under a `backups` directory next to the notes file. Pass a file path to
choose the exact destination, or an existing directory for a timestamped backup
inside that directory.

Command words such as `list`, `show`, `search`, `edit`, `delete`, `pin`,
`unpin`, `tag`, `untag`, `tags`, `path`, `backup`, and `export` are reserved
when they are the first argument. Use `--` to use one as a title:

```sh
lazynote -- search 'a note whose title is search'
```

Use single quotes for literal shell text, especially if the note contains
characters like `!`, `$`, or backticks.

## Agent Plugins

Agent plugins are optional. The `lazynote` CLI works on its own; plugins only
teach tools like Codex or Claude Code how to save and retrieve notes through the
CLI. Install the `lazynote` binary first, then install the plugin for your
agent.

### Codex Install

Add the GitHub marketplace source. This does not require cloning `lazynote`
first:

```sh
codex plugin marketplace add rschoch/lazynote
```

Then open `/plugins` in Codex and install `lazynote`.

To pick up newer plugin instructions later, upgrade the marketplace source and
update the installed plugin from `/plugins`:

```sh
codex plugin marketplace upgrade lazynote
```

Ask Codex to use `lazynote` when you want it to persist or retrieve project
context:

```text
"Use lazynote to save the proposed implementation plan."
"Search lazynote for notes about release packaging."
"Save a summary of this debugging session to lazynote."
```

### Claude Code Install

Add the GitHub marketplace source, install the plugin, and reload plugins. This
does not require cloning `lazynote` first:

```text
/plugin marketplace add rschoch/lazynote
/plugin install lazynote@lazynote
/reload-plugins
```

To pick up newer plugin instructions later, reinstall the plugin and reload:

```text
/plugin install lazynote@lazynote
/reload-plugins
```

Invoke the Claude Code skill with `/lazynote`:

```text
/lazynote Save the proposed implementation plan.
/lazynote Search for notes about release packaging.
/lazynote Save a summary of this debugging session.
```

## TUI

Open the terminal UI:

```sh
lazynote
```

The TUI shows note titles on the left and the selected note body on the right.
It automatically reloads when the notes file changes, so notes added from
another terminal tab, script, or coding agent appear while the TUI stays open.
The bottom status line shows context-specific key hints.

Keys:

- left: focus note list
- right: focus note body
- down: move or scroll down in the active pane
- up: move or scroll up in the active pane
- PageDown: scroll note body down
- PageUp: scroll note body up
- `/`: filter notes by title, body, or `#tag`; Enter applies, Esc cancels
- `r`: reload notes from disk now
- `n`: create a note in `$VISUAL`, `$EDITOR`, or `vi`
- `e`: edit the selected note in `$VISUAL`, `$EDITOR`, or `vi`
- `p`: pin or unpin the selected note
- `?`: show or hide the help overlay
- `c`: copy the selected title or note body
- `d` / delete: arm deletion; press `d` again to confirm
- Esc: clear the active filter
- `q` / Ctrl-C: quit

Copy uses terminal clipboard support. Creating and editing open a temporary
file whose first line is the note title and whose remaining content is the note
body. The note body pane shows tags and edited timestamps when present. Pinned
notes stay at the top of the list and use `▴` in the list gutter. Notes that
arrive from another process while the TUI is open use `●` until selected.
Fonts, glyph rendering, and colors depend on your terminal emulator.

Set your preferred external editor before launching `lazynote`:

```sh
export VISUAL="nvim"
export EDITOR="nano"
```

`VISUAL` is preferred over `EDITOR`; if neither is set, `lazynote` falls back to
`vi`. GUI editors should wait for the file to close, for example
`VISUAL="code --wait"`.

### TUI Behavior

TUI behavior is configured in `~/.config/lazynote/config.json`:

```json
{
  "tui": {
    "refreshIntervalSeconds": 1,
    "noteOrder": "oldest-first",
    "autoSelectNewNotes": false
  }
}
```

Supported values:

- `refreshIntervalSeconds`: how often the open TUI checks for external changes
- `noteOrder`: `oldest-first` or `newest-first`
- `autoSelectNewNotes`: jump to newly arrived notes after auto-refresh

### Themes

The TUI includes `default`, `mono`, and `high-contrast` themes:

```sh
LAZYNOTE_THEME=mono lazynote
```

Custom theme overrides are configured in `~/.config/lazynote/config.json`.
See [THEMING.md](THEMING.md) for the full format and examples.

## Storage

Default notes file:

```text
~/.local/share/lazynote/notes.json
```

Print the active path:

```sh
lazynote path
```

Use a different notes file:

```sh
LAZYNOTE_PATH=/tmp/lazynote-dev.json lazynote list
```

Back up your notes:

```sh
lazynote backup
lazynote backup ~/backup/lazynote
lazynote backup ~/backup/lazynote/notes.json
```

Because storage is one JSON file, it can be synced with tools like Syncthing,
Dropbox, iCloud Drive, or a private dotfiles repository. The TUI reloads when
the file changes, including atomic replacements. Writes use a small lock file
next to the notes file so concurrent CLI, TUI, script, and agent writes do not
silently overwrite each other. Newer versions may write optional `tags`,
`updated_at`, and `pinned` fields; older notes without these fields continue to
load normally.

## Development

Run tests and build a dev binary:

```sh
make test
make build
bin/lazynote --version
```

Try the dev binary without touching your real notes:

```sh
LAZYNOTE_PATH=/tmp/lazynote-dev.json bin/lazynote 'dev smoke' 'hello from local build'
LAZYNOTE_PATH=/tmp/lazynote-dev.json bin/lazynote list
LAZYNOTE_PATH=/tmp/lazynote-dev.json bin/lazynote export markdown
LAZYNOTE_PATH=/tmp/lazynote-dev.json bin/lazynote
```

If `go` is installed but not on `PATH`:

```sh
make GO=/usr/local/go/bin/go test
make GO=/usr/local/go/bin/go build
```

Check the installer script:

```sh
sh -n install.sh
sh install.sh --help
```

Useful targets:

- `make build`: build `bin/lazynote`
- `make test`: run `go test ./...`
- `make install`: install under `$(PREFIX)/bin`
- `make uninstall`: remove from `$(PREFIX)/bin`
- `make clean`: remove local build and release artifacts
- `make release-snapshot`: build local GoReleaser artifacts

## Roadmap

See [ROADMAP.md](ROADMAP.md) for lightweight future candidates and deliberately
deferred ideas.

## Releases

GoReleaser configuration lives in `.goreleaser.yaml`. Tagged releases build
Linux, macOS, and Windows binaries for `amd64` and `arm64`, plus checksums,
archives, and Linux packages.

Create a local snapshot:

```sh
make release-snapshot
```

Publish a tagged release:

```sh
git tag vX.Y.Z
git push origin vX.Y.Z
```

GitHub Actions runs tests and publishes release artifacts. Publishing apt/yum/apk
repositories or Homebrew taps is a separate distribution step.

## Acknowledgements

`lazynote` takes a lot of inspiration from
[LazyGit](https://github.com/jesseduffield/lazygit), especially around keeping a
terminal UI fast, keyboard-driven, and practical without making it feel heavy.

## License

MIT
