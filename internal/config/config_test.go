package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/awesome-gocui/gocui"
	"github.com/rschoch/lazynote/internal/ui"
)

func TestDefaultPathUsesXDGConfigHome(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	got, err := DefaultPath()
	if err != nil {
		t.Fatalf("default path: %v", err)
	}

	want := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "lazynote", "config.json")
	if got != want {
		t.Fatalf("DefaultPath() = %q, want %q", got, want)
	}
}

func TestLoadFromMissingFileUsesDefaults(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("load missing config: %v", err)
	}

	theme, err := cfg.ResolveTheme()
	if err != nil {
		t.Fatalf("resolve theme: %v", err)
	}
	if theme.Name != "default" {
		t.Fatalf("theme name = %q, want default", theme.Name)
	}
}

func TestLoadThemeUsesConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(EnvConfigPath, path)
	t.Setenv(EnvTheme, "")

	data := []byte(`{
  "theme": "mono",
  "themeOverrides": {
    "defaultBgColor": ["#f8fafc"],
    "activeBorderColor": ["cyan", "bold"]
  }
}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	theme, err := LoadTheme()
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}
	if theme.Name != "mono" {
		t.Fatalf("theme name = %q, want mono", theme.Name)
	}
	if want := gocui.ColorCyan | gocui.AttrBold; theme.ActiveBorder != want {
		t.Fatalf("active border = %v, want %v", theme.ActiveBorder, want)
	}
	if want := gocui.GetRGBColor(0xf8fafc); theme.DefaultBg != want {
		t.Fatalf("default bg = %v, want %v", theme.DefaultBg, want)
	}
}

func TestExampleLightThemeConfigLoads(t *testing.T) {
	cfg, err := LoadFrom(filepath.Join("..", "..", "examples", "themes", "light.json"))
	if err != nil {
		t.Fatalf("load example light theme: %v", err)
	}

	theme, err := cfg.ResolveTheme()
	if err != nil {
		t.Fatalf("resolve example light theme: %v", err)
	}
	if want := gocui.GetRGBColor(0xf8fafc); theme.DefaultBg != want {
		t.Fatalf("default bg = %v, want %v", theme.DefaultBg, want)
	}
}

func TestLoadThemeEnvOverridesConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	t.Setenv(EnvConfigPath, path)
	t.Setenv(EnvTheme, "high-contrast")

	if err := os.WriteFile(path, []byte(`{"theme": "missing"}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	theme, err := LoadTheme()
	if err != nil {
		t.Fatalf("load theme: %v", err)
	}
	if theme.Name != "high-contrast" {
		t.Fatalf("theme name = %q, want high-contrast", theme.Name)
	}
	if theme.ActiveBorder == ui.DefaultTheme().ActiveBorder {
		t.Fatal("active border matched default theme, want high-contrast theme")
	}
}
