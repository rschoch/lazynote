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
