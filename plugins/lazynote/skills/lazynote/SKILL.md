---
name: lazynote
description: Use when the user asks to save, recall, search, edit, export, summarize, or manage local notes with the lazynote CLI, especially for session summaries, coding-agent context, and durable terminal notes.
---

# lazynote

Use `lazynote` as a local notes layer for durable context that should survive the current agent session.

## Before using it

- If unsure whether the CLI exists, run `command -v lazynote`.
- If it is missing, tell the user to install the binary before using this skill.
- For agent workflows, use the CLI commands. Do not open the TUI unless the user explicitly asks.
- Do not inspect the lazynote source tree to figure out how to save notes; use the installed `lazynote` CLI.
- Ask before saving credentials, tokens, secrets, or sensitive personal data unless the user explicitly requested that exact save.
- Prefer short, descriptive titles.

## Save notes

For multiline content, pass the body on stdin and keep command output quiet unless the user wants confirmation:

```sh
lazynote --quiet 'session summary' - <<'EOF'
Summarize the useful context here.
EOF
```

Add tags when they will make later retrieval easier:

```sh
lazynote --quiet --tag work --tag release 'session summary' - <<'EOF'
Summarize the useful context here.
EOF
```

For command output:

```sh
some-command | lazynote --quiet 'diagnostic output'
```

Use single quotes around literal shell text. If a title would collide with a command word such as `list` or `search`, use `--` before the title.

## Retrieve and manage notes

- List notes: `lazynote list`
- Search notes: `lazynote search 'query'`
- Search by tag: `lazynote search '#tag'`
- Show a note: `lazynote show <id>`
- Fetch only the body for context: `lazynote show --body <id>`
- Edit in the configured editor: `lazynote edit <id>`
- Edit directly: `lazynote edit <id> 'new title' 'new body'`
- Delete a note: `lazynote delete <id>`
- Pin or unpin a note: `lazynote pin <id>` or `lazynote unpin <id>`
- Add or remove tags: `lazynote tag <id> work` or `lazynote untag <id> work`
- Print a note's tags: `lazynote tags <id>`
- Print the notes file path: `lazynote path`
- Back up the raw notes JSON file: `lazynote backup` or `lazynote backup <path>`
- Export everything: `lazynote export markdown` or `lazynote export json`

`lazynote list` prints tab-separated `id`, `created_at`, and `title` fields, plus metadata when present. Most note commands accept a full note ID or a unique ID prefix.

Do not edit the notes JSON file directly unless the user explicitly asks for low-level recovery or repair.
