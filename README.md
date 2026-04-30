# lazynote

`lazynote` is a lightweight terminal notes app for quick personal notes and
agent-friendly CLI workflows. It has a small `gocui` TUI for browsing notes and
a scriptable command surface for saving and retrieving context from other tools.

## Install

Recommended install for Linux and macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/rschoch/lazynote/main/install.sh | sh
```

The installer downloads the latest release for your OS and architecture, checks
the release checksum when `sha256sum` or `shasum` is available, and installs the
binary to `~/.local/bin`.

To inspect the script first:

```sh
curl -fsSLO https://raw.githubusercontent.com/rschoch/lazynote/main/install.sh
sh install.sh
```

You can choose another directory or pin a version:

```sh
sh install.sh --dir /usr/local/bin
sh install.sh --version v0.1.0
```

System directories such as `/usr/local/bin` may require `sudo`.

Prebuilt archives and Linux packages are also available from the
[GitHub Releases](https://github.com/rschoch/lazynote/releases) page, including
`.deb`, `.rpm`, and `.apk` packages. Download a package and install it with your
system package manager, for example `sudo apt install ./lazynote_0.1.0_amd64.deb`.

The installer and direct package downloads do not add an apt/yum/apk repository
yet, so your system package manager will not discover future `lazynote` updates
automatically.

From source:

```sh
go install github.com/rschoch/lazynote/cmd/lazynote@latest
```

From a checkout:

```sh
make build
make install
```

`make install` writes the binary to `/usr/local/bin` by default. Set `PREFIX` to
choose another root:

```sh
make install PREFIX="$HOME/.local"
```

## Scriptable and agent-friendly

`lazynote` is intentionally composable: it can save text from stdin and expose
saved notes through plain CLI commands. That makes it useful from shell scripts,
terminal tools, and coding-agent workflows without requiring a tool to drive the
interactive UI.

Capture generated context:

```sh
some-command | lazynote "session summary"
lazynote "release notes" - < release-notes.md
```

Retrieve saved context later:

```sh
lazynote list
lazynote show <id>
lazynote search packaging
```

The goal is bidirectional interoperability: humans can browse notes in the TUI,
while tools can save and retrieve notes through stable text commands.

## Usage

Add a note:

```sh
lazynote mytitle "my note on something i shouldnt forget to do later"
```

Add a note from stdin:

```sh
echo "my note from another command" | lazynote mytitle
cat summary.md | lazynote "session summary"
lazynote "session summary" - < summary.md
```

If stdin is piped without a title, `lazynote` uses the first non-empty line as
the title:

```sh
printf '## Session abc123\n- shipped release prep\n' | lazynote
```

List saved notes:

```sh
lazynote list
```

`list` prints tab-separated `id`, `created_at`, and `title` fields. Use the ID
to print a full note:

```sh
lazynote show <id>
```

`show` also accepts a unique ID prefix. Search note titles and bodies:

```sh
lazynote search packaging
lazynote search "session summary"
```

Open the terminal UI:

```sh
lazynote
```

Print version metadata:

```sh
lazynote --version
```

The UI shows note titles on the left and the selected note body on the right.

Keys:

- left / `h`: focus the notes list
- right / `l`: focus the selected note body
- `j` / down: move down in the active pane
- `k` / up: move up in the active pane
- PageDown / Ctrl-D: scroll selected note body down
- PageUp / Ctrl-U: scroll selected note body up
- `d` / delete: arm deletion; press `d` again on the same note to confirm
- `q` / Ctrl-C: quit

Terminal fonts are controlled by your terminal emulator. `lazynote` uses rounded
borders, color, and simple glyphs where the terminal supports them.

## Storage

Notes are stored as JSON at:

```text
~/.local/share/lazynote/notes.json
```

Set `LAZYNOTE_PATH` to use a different notes file.

## Development

Run the tests:

```sh
make test
```

Build a local binary:

```sh
make build
bin/lazynote --version
```

Useful make targets:

- `make build`: build `bin/lazynote`
- `make test`: run `go test ./...`
- `make install`: install the binary under `$(PREFIX)/bin`
- `make uninstall`: remove the installed binary from `$(PREFIX)/bin`
- `make clean`: remove local build and release artifacts

## Releases

Release configuration lives in `.goreleaser.yaml`. It builds `lazynote` for
Linux, macOS, and Windows on `amd64` and `arm64`, injects version metadata, and
produces checksums, archives, and Linux `.deb`, `.rpm`, and `.apk` packages.
Install GoReleaser before running release targets.

Create a local release snapshot:

```sh
make release-snapshot
```

Tagged releases are built by GitHub Actions:

```sh
git tag v0.1.1
git push origin v0.1.1
```

GoReleaser publishes archives, checksums, and Linux packages to the GitHub
Release. Publishing installable apt/yum/apk repositories or Homebrew taps is a
separate distribution step after release artifacts exist.

## License

MIT
