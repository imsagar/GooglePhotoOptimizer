// Package config loads and saves the runner's local settings from
// ~/.gpoptimizer/config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	ServerURL   string `json:"server_url"`
	Token       string `json:"token"`
	StoragePath string `json:"storage_path"`
}

// Dir returns the directory holding the runner's config file.
func Dir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gpoptimizer")
}

func Path() string {
	return filepath.Join(Dir(), "config.json")
}

func Load() (*Config, error) {
	data, err := os.ReadFile(Path())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func Save(cfg *Config) error {
	if err := os.MkdirAll(Dir(), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), data, 0600)
}
