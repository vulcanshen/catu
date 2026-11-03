package ui

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// config is the user's settings (tdp E3): the two keys the family shares that
// viewu uses.
type config struct {
	// TabWidth is input_tab_width: the spaces a tab in a text is drawn as
	// (tdp note.2).
	TabWidth int `yaml:"input_tab_width"`
	// HideSecrets is hide_secrets_in_preview: in the file-picker's preview a
	// private key shows its first line only, a file only its owner may read
	// shows nothing. It is the popup previews' only, not the panel's (tdp
	// file-picker.5).
	HideSecrets bool `yaml:"hide_secrets_in_preview"`
}

func defaults() config { return config{TabWidth: 4} }

// configDir is the settings directory (tdp E2, E3): VIEWU__CONFIG, else
// $XDG_CONFIG_HOME/viewu, else ~/.config/viewu.
func configDir() string {
	if d := os.Getenv("VIEWU__CONFIG"); d != "" {
		return d
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "viewu")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "viewu")
}

// readConfig reads config.yaml in the settings directory; with no file it is
// the defaults. viewu never writes it. err is the file's that could not be
// read, path where it is.
func readConfig() (cfg config, path string, err error) {
	cfg, path = defaults(), filepath.Join(configDir(), "config.yaml")
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, path, nil
	}
	if err != nil {
		return defaults(), path, err
	}
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return defaults(), path, err
	}
	if cfg.TabWidth < 1 {
		cfg.TabWidth = defaults().TabWidth
	}
	return cfg, path, nil
}
