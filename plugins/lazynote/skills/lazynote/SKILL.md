---
name: lazynote
description: Use when the user asks to save, recall, search, export, or summarize notes with the local lazynote CLI, especially for session summaries, coding-agent context, and durable terminal notes.
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

For multiline content, pass the body on stdin and keep command output quiet:

```sh
lazynote --quiet 'session summary' - <<'EOF'
Summarize the useful context here.
EOF
```

For command output:

```sh
some-command | lazynote --quiet 'diagnostic output'
```

Use single quotes around literal shell text. If a title would collide with a command word such as `list` or `search`, use `--` before the title.

## Retrieve notes

- List notes: `lazynote list`
- Search notes: `lazynote search 'query'`
- Show a note: `lazynote show <id>`
- Fetch only the body for context: `lazynote show --body <id>`
- Print the notes file path: `lazynote path`
- Export everything: `lazynote export markdown` or `lazynote export json`

`lazynote list` prints tab-separated `id`, `created_at`, and `title` fields. `show` accepts a full note ID or a unique ID prefix.

Do not edit the notes JSON file directly unless the user explicitly asks for low-level recovery or repair.
