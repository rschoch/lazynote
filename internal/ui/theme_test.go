package ui

import (
	"strings"
	"testing"

	"github.com/awesome-gocui/gocui"
)

func TestResolveThemeDefaultsToDefaultTheme(t *testing.T) {
	got, err := ResolveTheme("", ThemeConfig{})
	if err != nil {
		t.Fatalf("resolve default theme: %v", err)
	}
	if got.Name != "default" {
		t.Fatalf("theme name = %q, want default", got.Name)
	}
	if got.ActiveBorder != DefaultTheme().ActiveBorder {
		t.Fatalf("active border = %v, want default theme active border", got.ActiveBorder)
	}
}

func TestDefaultThemeUsesTerminalColors(t *testing.T) {
	got := DefaultTheme()

	wants := map[string]struct {
		got  gocui.Attribute
		want gocui.Attribute
	}{
		"background":    {got.DefaultBg, gocui.ColorDefault},
		"foreground":    {got.DefaultFg, gocui.ColorDefault},
		"muted text":    {got.MutedFg, gocui.ColorDefault},
		"active border": {got.ActiveBorder, gocui.GetRGBColor(0x3a6ea5) | gocui.AttrBold},
		"inactive border": {
			got.InactiveBorder,
			gocui.ColorDefault | gocui.AttrDim,
		},
		"title":                {got.Title, gocui.GetRGBColor(0x3a6ea5) | gocui.AttrBold},
		"status":               {got.StatusFg, gocui.ColorDefault},
		"selection background": {got.SelectedLineBg, gocui.GetRGBColor(0x3a6ea5)},
		"selection foreground": {got.SelectedLineFg, gocui.GetRGBColor(0xffffff)},
	}
	for role, attrs := range wants {
		if attrs.got != attrs.want {
			t.Errorf("%s = %v, want %v", role, attrs.got, attrs.want)
		}
	}
}

func TestBuiltInLightAndDarkThemesSetBackgrounds(t *testing.T) {
	light, err := ResolveTheme("light", ThemeConfig{})
	if err != nil {
		t.Fatalf("resolve light theme: %v", err)
	}
	dark, err := ResolveTheme("dark", ThemeConfig{})
	if err != nil {
		t.Fatalf("resolve dark theme: %v", err)
	}

	if light.DefaultBg != gocui.GetRGBColor(0xf8fafc) {
		t.Fatalf("light background = %v, want fixed light background", light.DefaultBg)
	}
	if dark.DefaultBg != gocui.Get256Color(234) {
		t.Fatalf("dark background = %v, want fixed dark background", dark.DefaultBg)
	}
}

func TestHighContrastThemeUsesTerminalColors(t *testing.T) {
	got, err := ResolveTheme("high-contrast", ThemeConfig{})
	if err != nil {
		t.Fatalf("resolve high-contrast theme: %v", err)
	}

	if got.DefaultBg != gocui.ColorDefault || got.DefaultFg != gocui.ColorDefault {
		t.Fatalf("high-contrast foreground/background = %v/%v, want terminal defaults", got.DefaultFg, got.DefaultBg)
	}
	if got.MutedFg != gocui.ColorDefault {
		t.Fatalf("high-contrast muted text = %v, want undimmed terminal foreground", got.MutedFg)
	}
}

func TestResolveThemeAppliesAttributeOverrides(t *testing.T) {
	got, err := ResolveTheme("mono", ThemeConfig{
		DefaultBgColor:      []string{"#f8fafc"},
		ActiveBorderColor:   []string{"color80", "bold"},
		SelectedLineBgColor: []string{"#112233", "reverse"},
	})
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}

	if want := gocui.Get256Color(80) | gocui.AttrBold; got.ActiveBorder != want {
		t.Fatalf("active border = %v, want %v", got.ActiveBorder, want)
	}
	if want := gocui.GetRGBColor(0xf8fafc); got.DefaultBg != want {
		t.Fatalf("default bg = %v, want %v", got.DefaultBg, want)
	}
	if want := gocui.GetRGBColor(0x112233) | gocui.AttrReverse; got.SelectedLineBg != want {
		t.Fatalf("selected line bg = %v, want %v", got.SelectedLineBg, want)
	}
	if got.Name != "mono" {
		t.Fatalf("theme name = %q, want mono", got.Name)
	}
}

func TestResolveThemeRejectsUnknownTheme(t *testing.T) {
	_, err := ResolveTheme("missing", ThemeConfig{})
	if err == nil {
		t.Fatal("resolve theme returned nil error, want unknown theme error")
	}
	if !strings.Contains(err.Error(), "high-contrast") {
		t.Fatalf("error = %q, want available theme names", err)
	}
}

func TestResolveThemeRejectsUnknownAttribute(t *testing.T) {
	_, err := ResolveTheme("default", ThemeConfig{
		TitleColor: []string{"sparkle"},
	})
	if err == nil {
		t.Fatal("resolve theme returned nil error, want unknown attribute error")
	}
	if !strings.Contains(err.Error(), "titleColor") {
		t.Fatalf("error = %q, want role name", err)
	}
}
