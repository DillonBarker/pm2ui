package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds user-tunable settings.
type Config struct {
	RefreshInterval  time.Duration // pm2 jlist poll interval
	DefaultTailLines int           // lines shown when opening single-service logs
	AllTailLines     int           // initial history per service in merged (all) mode
	MaxLogLines      int           // log buffer cap
	LogBatchInterval time.Duration // render flush interval
	SplitRatio       int           // logs pane weight vs process pane (1..5)
	MouseEnabled     bool          // mouse support (wheel scroll, click select)
}

// Default returns the built-in settings.
func Default() Config {
	return Config{
		RefreshInterval:  2 * time.Second,
		DefaultTailLines: 200,
		AllTailLines:     50,
		MaxLogLines:      5000,
		LogBatchInterval: 50 * time.Millisecond,
		SplitRatio:       2,
		MouseEnabled:     true,
	}
}

// fileConfig mirrors the YAML shape; durations are strings ("2s", "50ms").
// Pointers distinguish "absent" from explicit zero values.
type fileConfig struct {
	RefreshInterval  string `yaml:"refreshInterval"`
	DefaultTailLines *int   `yaml:"defaultTailLines"`
	AllTailLines     *int   `yaml:"allTailLines"`
	MaxLogLines      *int   `yaml:"maxLogLines"`
	LogBatchInterval string `yaml:"logBatchInterval"`
	SplitRatio       *int   `yaml:"splitRatio"`
	MouseEnabled     *bool  `yaml:"mouseEnabled"`
}

// Path returns the config file location: $XDG_CONFIG_HOME/pm2ui/config.yaml,
// falling back to ~/.config/pm2ui/config.yaml.
func Path() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "pm2ui", "config.yaml")
}

// Load reads the user config. A missing file yields defaults with no error;
// an unreadable or invalid file yields defaults plus the error.
func Load() (Config, error) {
	return loadFrom(Path())
}

func loadFrom(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}

	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return Default(), fmt.Errorf("parse %s: %w", path, err)
	}

	if fc.RefreshInterval != "" {
		d, err := time.ParseDuration(fc.RefreshInterval)
		if err != nil {
			return Default(), fmt.Errorf("refreshInterval: %w", err)
		}
		cfg.RefreshInterval = d
	}
	if fc.DefaultTailLines != nil {
		cfg.DefaultTailLines = *fc.DefaultTailLines
	}
	if fc.AllTailLines != nil {
		cfg.AllTailLines = *fc.AllTailLines
	}
	if fc.MaxLogLines != nil {
		cfg.MaxLogLines = *fc.MaxLogLines
	}
	if fc.SplitRatio != nil {
		cfg.SplitRatio = min(5, max(1, *fc.SplitRatio))
	}
	if fc.MouseEnabled != nil {
		cfg.MouseEnabled = *fc.MouseEnabled
	}
	if fc.LogBatchInterval != "" {
		d, err := time.ParseDuration(fc.LogBatchInterval)
		if err != nil {
			return Default(), fmt.Errorf("logBatchInterval: %w", err)
		}
		cfg.LogBatchInterval = d
	}
	return cfg, nil
}
