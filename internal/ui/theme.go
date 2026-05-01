package ui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/awesome-gocui/gocui"
)

const defaultThemeName = "default"

// Theme holds semantic colors for the terminal UI.
type Theme struct {
	Name           string
	DefaultBg      gocui.Attribute
	DefaultFg      gocui.Attribute
	MutedFg        gocui.Attribute
	ActiveBorder   gocui.Attribute
	InactiveBorder gocui.Attribute
	Title          gocui.Attribute
	Warning        gocui.Attribute
	StatusFg       gocui.Attribute
	SelectedLineBg gocui.Attribute
	SelectedLineFg gocui.Attribute
}

// ThemeConfig is the user-facing theme override shape.
type ThemeConfig struct {
	DefaultBgColor      []string `json:"defaultBgColor,omitempty"`
	DefaultFgColor      []string `json:"defaultFgColor,omitempty"`
	MutedFgColor        []string `json:"mutedFgColor,omitempty"`
	ActiveBorderColor   []string `json:"activeBorderColor,omitempty"`
	InactiveBorderColor []string `json:"inactiveBorderColor,omitempty"`
	TitleColor          []string `json:"titleColor,omitempty"`
	WarningColor        []string `json:"warningColor,omitempty"`
	StatusFgColor       []string `json:"statusFgColor,omitempty"`
	SelectedLineBgColor []string `json:"selectedLineBgColor,omitempty"`
	SelectedLineFgColor []string `json:"selectedLineFgColor,omitempty"`
}

func DefaultTheme() Theme {
	return builtInThemes()[defaultThemeName]
}

func ResolveTheme(name string, overrides ThemeConfig) (Theme, error) {
	name = normalizeThemeName(name)
	if name == "" {
		name = defaultThemeName
	}

	theme, ok := builtInThemes()[name]
	if !ok {
		return Theme{}, fmt.Errorf("unknown theme %q (available: %s)", name, strings.Join(ThemeNames(), ", "))
	}

	if err := applyThemeConfig(&theme, overrides); err != nil {
		return Theme{}, err
	}
	return theme, nil
}

func ThemeNames() []string {
	themes := builtInThemes()
	names := make([]string, 0, len(themes))
	for name := range themes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func builtInThemes() map[string]Theme {
	return map[string]Theme{
		"default": {
			Name:           "default",
			DefaultBg:      gocui.ColorDefault,
			DefaultFg:      gocui.Get256Color(252),
			MutedFg:        gocui.Get256Color(248),
			ActiveBorder:   gocui.Get256Color(80),
			InactiveBorder: gocui.Get256Color(66),
			Title:          gocui.Get256Color(218) | gocui.AttrBold,
			Warning:        gocui.Get256Color(215),
			StatusFg:       gocui.Get256Color(248),
			SelectedLineBg: gocui.Get256Color(79),
			SelectedLineFg: gocui.Get256Color(234),
		},
		"high-contrast": {
			Name:           "high-contrast",
			DefaultBg:      gocui.ColorDefault,
			DefaultFg:      gocui.ColorWhite,
			MutedFg:        gocui.ColorCyan,
			ActiveBorder:   gocui.ColorYellow | gocui.AttrBold,
			InactiveBorder: gocui.ColorWhite,
			Title:          gocui.ColorYellow | gocui.AttrBold,
			Warning:        gocui.ColorRed | gocui.AttrBold,
			StatusFg:       gocui.ColorWhite,
			SelectedLineBg: gocui.ColorWhite,
			SelectedLineFg: gocui.ColorBlack | gocui.AttrBold,
		},
		"mono": {
			Name:           "mono",
			DefaultBg:      gocui.ColorDefault,
			DefaultFg:      gocui.ColorDefault,
			MutedFg:        gocui.ColorDefault | gocui.AttrDim,
			ActiveBorder:   gocui.ColorDefault | gocui.AttrBold,
			InactiveBorder: gocui.ColorDefault,
			Title:          gocui.ColorDefault | gocui.AttrBold,
			Warning:        gocui.ColorDefault | gocui.AttrBold,
			StatusFg:       gocui.ColorDefault | gocui.AttrDim,
			SelectedLineBg: gocui.ColorDefault | gocui.AttrReverse,
			SelectedLineFg: gocui.ColorDefault,
		},
	}
}

func normalizeThemeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func applyThemeConfig(theme *Theme, cfg ThemeConfig) error {
	overrides := []struct {
		role   string
		tokens []string
		target *gocui.Attribute
	}{
		{"defaultBgColor", cfg.DefaultBgColor, &theme.DefaultBg},
		{"defaultFgColor", cfg.DefaultFgColor, &theme.DefaultFg},
		{"mutedFgColor", cfg.MutedFgColor, &theme.MutedFg},
		{"activeBorderColor", cfg.ActiveBorderColor, &theme.ActiveBorder},
		{"inactiveBorderColor", cfg.InactiveBorderColor, &theme.InactiveBorder},
		{"titleColor", cfg.TitleColor, &theme.Title},
		{"warningColor", cfg.WarningColor, &theme.Warning},
		{"statusFgColor", cfg.StatusFgColor, &theme.StatusFg},
		{"selectedLineBgColor", cfg.SelectedLineBgColor, &theme.SelectedLineBg},
		{"selectedLineFgColor", cfg.SelectedLineFgColor, &theme.SelectedLineFg},
	}

	for _, override := range overrides {
		if len(override.tokens) == 0 {
			continue
		}
		value, err := parseThemeAttribute(override.tokens)
		if err != nil {
			return fmt.Errorf("%s: %w", override.role, err)
		}
		*override.target = value
	}
	return nil
}

func parseThemeAttribute(tokens []string) (gocui.Attribute, error) {
	var attr gocui.Attribute
	for _, token := range tokens {
		value, err := themeTokenAttribute(token)
		if err != nil {
			return 0, err
		}
		attr |= value
	}
	return attr, nil
}

func themeTokenAttribute(token string) (gocui.Attribute, error) {
	key := strings.ToLower(strings.TrimSpace(token))
	if key == "" {
		return 0, fmt.Errorf("empty theme attribute")
	}

	if attr, ok := namedThemeAttributes[key]; ok {
		return attr, nil
	}
	if strings.HasPrefix(key, "#") {
		return parseHexThemeColor(key)
	}
	if index, ok, err := parse256ThemeColor(key); ok || err != nil {
		if err != nil {
			return 0, err
		}
		return gocui.Get256Color(index), nil
	}

	return 0, fmt.Errorf("unknown theme attribute %q", token)
}

var namedThemeAttributes = map[string]gocui.Attribute{
	"default":       gocui.ColorDefault,
	"black":         gocui.ColorBlack,
	"red":           gocui.ColorRed,
	"green":         gocui.ColorGreen,
	"yellow":        gocui.ColorYellow,
	"blue":          gocui.ColorBlue,
	"magenta":       gocui.ColorMagenta,
	"cyan":          gocui.ColorCyan,
	"white":         gocui.ColorWhite,
	"bold":          gocui.AttrBold,
	"reverse":       gocui.AttrReverse,
	"underline":     gocui.AttrUnderline,
	"dim":           gocui.AttrDim,
	"italic":        gocui.AttrItalic,
	"strikethrough": gocui.AttrStrikeThrough,
}

func parseHexThemeColor(token string) (gocui.Attribute, error) {
	hex := strings.TrimPrefix(token, "#")
	if len(hex) != 6 {
		return 0, fmt.Errorf("hex colors must use #rrggbb: %q", token)
	}

	value, err := strconv.ParseInt(hex, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid hex color %q", token)
	}
	return gocui.GetRGBColor(int32(value)), nil
}

func parse256ThemeColor(token string) (int32, bool, error) {
	raw, ok := strings.CutPrefix(token, "color")
	if !ok {
		raw, ok = strings.CutPrefix(token, "ansi")
	}
	if !ok {
		return 0, false, nil
	}

	raw = strings.TrimPrefix(raw, "-")
	index, err := strconv.ParseInt(raw, 10, 32)
	if err != nil {
		return 0, true, fmt.Errorf("invalid 256-color attribute %q", token)
	}
	if index < 0 || index > 255 {
		return 0, true, fmt.Errorf("256-color attribute out of range %q", token)
	}
	return int32(index), true, nil
}
