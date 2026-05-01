package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rschoch/lazynote/internal/ui"
)

const (
	EnvConfigPath = "LAZYNOTE_CONFIG"
	EnvTheme      = "LAZYNOTE_THEME"

	appDirName = "lazynote"
	configFile = "config.json"
)

// Config contains user preferences that do not belong in notes storage.
type Config struct {
	Theme          string         `json:"theme,omitempty"`
	ThemeOverrides ui.ThemeConfig `json:"themeOverrides,omitempty"`
}

// LoadTheme returns the configured terminal UI theme.
func LoadTheme() (ui.Theme, error) {
	if themeName := strings.TrimSpace(os.Getenv(EnvTheme)); themeName != "" {
		return ui.ResolveTheme(themeName, ui.ThemeConfig{})
	}

	cfg, err := Load()
	if err != nil {
		return ui.Theme{}, err
	}
	return cfg.ResolveTheme()
}

// Load reads the default user config file. Missing config is not an error.
func Load() (Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, err
	}
	return LoadFrom(path)
}

// LoadFrom reads a user config file. Missing config is not an error.
func LoadFrom(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var cfg Config
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %s: %w", path, err)
	}
	return cfg, nil
}

// DefaultPath returns the default user config file location.
func DefaultPath() (string, error) {
	if path := os.Getenv(EnvConfigPath); path != "" {
		return path, nil
	}

	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}

	return filepath.Join(base, appDirName, configFile), nil
}

// ResolveTheme resolves Config into a concrete UI theme.
func (c Config) ResolveTheme() (ui.Theme, error) {
	return ui.ResolveTheme(c.Theme, c.ThemeOverrides)
}
