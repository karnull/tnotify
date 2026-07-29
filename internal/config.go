// internal/config.go

package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/karnull/tnotify/resources"
)

// Config mirrors the layout of resources/default.toml and a user's
// ~/.config/tnotify/config.toml.
type Config struct {
	Colors struct {
		Border    string `toml:"border"`
		Head      string `toml:"head"`
		Message   string `toml:"message"`
		Author    string `toml:"author"`
		Selection string `toml:"selection"`
		Footer    string `toml:"footer"`
		Expired   string `toml:"expired"`
	} `toml:"colors"`

	Sidepanel struct {
		Direction string `toml:"direction"`
		Width     int    `toml:"width"`

		// How the time a notification arrived is drawn on its box: a name the
		// presets know, or a Go time layout of the user's own.
		Clock string `toml:"clock"`
		Date  string `toml:"date"`
	} `toml:"sidepanel"`

	Overlay struct {
		Location  string `toml:"location"`
		Stack     bool   `toml:"stack"`
		Timeout   int    `toml:"timeout"`
		MinWidth  int    `toml:"min_width"`
		MaxWidth  int    `toml:"max_width"`
		MinHeight int    `toml:"min_height"`
		MaxHeight int    `toml:"max_height"`
	} `toml:"overlay"`
}

//- Private Helpers --------------------------------------------------------------------------------

// Return the path to tnotify's config file: ~/.config/tnotify/config.toml
func configFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	return filepath.Join(home, ".config", "tnotify", "config.toml"), nil
}

// Write the embedded default config to path, creating parent directories as
// needed.
func writeDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	return os.WriteFile(path, []byte(resources.DefaultConfig), 0o644)
}

// Implements --config: report the config path, creating it from the embedded
// default when the user has none yet.
func showConfig() {
	path, err := configFilePath()
	if err != nil {
		reportError(err)
		return
	}

	if _, err := os.Stat(path); err == nil {
		fmt.Printf("config exists at %s\n", path)
		return
	}

	if err := writeDefaultConfig(path); err != nil {
		reportError(err)
		return
	}
	fmt.Printf("created default config at %s\n", path)
}

// Implements --defaults: back up any existing config, then write the default in
// its place.
func resetConfig() {
	path, err := configFilePath()
	if err != nil {
		reportError(err)
		return
	}

	if _, err := os.Stat(path); err == nil {
		backup := path + ".backup_" + time.Now().Format("02012006_1504")
		if err := os.Rename(path, backup); err != nil {
			reportError(err)
			return
		}
		fmt.Printf("backed up existing config to %s\n", backup)
	}

	if err := writeDefaultConfig(path); err != nil {
		reportError(err)
		return
	}
	fmt.Printf("wrote default config to %s\n", path)
}

// The config as shipped, for when the user's own cannot be read. The embedded
// default is known good, so a failure to decode it is not worth reporting.
func defaultConfig() Config {
	var cfg Config
	toml.Decode(resources.DefaultConfig, &cfg)
	return cfg
}

//- Public Calls -----------------------------------------------------------------------------------

// Read the user's config file, falling back to the embedded default when they
// have none.
func LoadConfig() (Config, error) {
	var cfg Config

	path, err := configFilePath()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return cfg, fmt.Errorf("reading config: %w", err)
		}
		data = []byte(resources.DefaultConfig)
	}

	if _, err := toml.Decode(string(data), &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}
