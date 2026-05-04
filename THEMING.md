# Theming

`lazynote` themes are small JSON configs for the terminal UI. They change the
colors and text attributes used by semantic UI roles such as borders, note text,
muted text, and the selected line.

## Built-In Themes

The built-in themes are:

- `default`
- `mono`
- `high-contrast`

Use a built-in theme for one run:

```sh
LAZYNOTE_THEME=mono lazynote
```

`LAZYNOTE_THEME` takes precedence over config files.

## Config File

The default config path is:

```text
~/.config/lazynote/config.json
```

Minimal config:

```json
{
  "theme": "mono"
}
```

The same config file also holds non-color TUI behavior under the `tui` key:

```json
{
  "theme": "mono",
  "tui": {
    "refreshIntervalSeconds": 1,
    "noteOrder": "newest-first",
    "autoSelectNewNotes": false
  }
}
```

Use another config file for one run:

```sh
LAZYNOTE_CONFIG=/path/to/config.json lazynote
```

## Theme Overrides

Start from a built-in theme, then override only the roles you care about:

```json
{
  "theme": "default",
  "themeOverrides": {
    "activeBorderColor": ["cyan", "bold"],
    "selectedLineBgColor": ["reverse"],
    "statusFgColor": ["#888888"]
  }
}
```

Supported roles:

- `defaultBgColor`: base TUI background
- `defaultFgColor`: normal note text
- `mutedFgColor`: empty states, inactive titles, secondary text
- `activeBorderColor`: focused pane border
- `inactiveBorderColor`: unfocused pane border
- `titleColor`: focused pane title
- `warningColor`: warning and small-terminal frame
- `statusFgColor`: bottom status line
- `selectedLineBgColor`: selected note background
- `selectedLineFgColor`: selected note text

Supported attributes:

- Terminal defaults: `default`
- ANSI colors: `black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`
- Styles: `bold`, `reverse`, `underline`, `dim`, `italic`, `strikethrough`
- RGB colors: `#rrggbb`
- 256-color tokens: `color80`, `ansi80`

Attributes can be combined:

```json
{
  "themeOverrides": {
    "activeBorderColor": ["#2563eb", "bold"]
  }
}
```

## Backgrounds

Use `defaultBgColor` when you want lazynote to paint the app background:

```json
{
  "themeOverrides": {
    "defaultBgColor": ["#f8fafc"]
  }
}
```

Omit `defaultBgColor`, or set it to `["default"]`, to keep the terminal
profile background. This preserves terminal transparency and acrylic effects in
terminals that support them.

`lazynote` can paint its own TUI background, but it does not change your
terminal profile or window chrome.

## Example Themes

Try the copyable light example from a checkout:

```sh
LAZYNOTE_CONFIG=examples/themes/light.json lazynote
```

Install it as your default:

```sh
mkdir -p ~/.config/lazynote
cp examples/themes/light.json ~/.config/lazynote/config.json
```

Switch back to the built-in default by removing that config, changing
`"theme"` to `"default"` without overrides, or running with no `LAZYNOTE_CONFIG`
or `LAZYNOTE_THEME` override.
