package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the application configuration.
type Config struct {
	// Scanning
	MinSize       int64    `json:"min_size"`
	IncludeHidden bool     `json:"include_hidden"`
	Categories    []string `json:"categories"`

	// Cleaning
	DryRun   bool `json:"dry_run"`
	UseTrash bool `json:"use_trash"`

	// UI
	ShowSizes   bool   `json:"show_sizes"`
	SortBy      string `json:"sort_by"` // size, name, date
	Parallelism int    `json:"parallelism"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		MinSize:       0,
		IncludeHidden: false,
		Categories:    []string{"caches", "logs", "xcode", "homebrew", "browsers"},
		DryRun:        false,
		UseTrash:      true,
		ShowSizes:     true,
		SortBy:        "size",
		Parallelism:   4,
	}
}

// Load loads configuration from the config file.
func Load() (*Config, error) {
	configPath, err := ConfigPath()
	if err != nil {
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save saves the configuration to the config file.
func (c *Config) Save() error {
	configPath, err := ConfigPath()
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if mkdirErr := os.MkdirAll(filepath.Dir(configPath), 0755); mkdirErr != nil {
		return mkdirErr
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// configDir returns the resolved ash config directory.
func configDir() (string, error) {
	if ashHome := os.Getenv("ASH_HOME"); ashHome != "" {
		return ashHome, nil
	}
	if toolsHome := os.Getenv("TOOLS_HOME"); toolsHome != "" {
		return filepath.Join(toolsHome, "ash"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tools", "ash"), nil
}

// ConfigPath returns the path to the config file.
func ConfigPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// ConfigDir returns the path to the config directory.
func ConfigDir() (string, error) {
	return configDir()
}
