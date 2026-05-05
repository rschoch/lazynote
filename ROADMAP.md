# Roadmap

`lazynote` should stay small: fast capture, plain local storage, a quiet TUI, and
commands that are easy for humans, scripts, and coding agents to use.

## Current Focus

- Keep CLI and TUI operations at parity for core note actions.
- Preserve the single JSON-file storage model.
- Prefer visible state only when it helps scanning, such as pinned and unread
  markers.

## Implemented Recently

- Auto-refresh in the TUI for notes written by other processes.
- Lock-file write coordination and raw JSON backups.
- TUI search, edit, manual refresh, pinned notes, and unread markers.
- Create notes from inside the TUI using the configured external editor.
- CLI edit, delete, pin, unpin, tag, untag, and tag inspection.
- Optional note metadata for `tags`, `updated_at`, and `pinned`.
- TUI help overlay via `?`.

## Future Candidates

- Fuzzy ranking for search results while keeping plain substring matching as the
  predictable baseline.
- A compact preview line option for the note list.
- Multi-select export or copy from the TUI.
- Installer support for package-manager repositories or Homebrew taps.
- Optional import helpers for Markdown or JSON from other note tools.

## Deferred For Now

- Archive/hide-from-list workflows.
- Filtered export subsets.
- Sync service, account system, or database-backed storage.
- Rich text editing.
