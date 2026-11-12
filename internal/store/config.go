package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// AppConfig is durable local settings (Go-side).
type AppConfig struct {
	Version        int    `json:"version"`
	Language       string `json:"language"`
	PollIntervalMS int    `json:"pollIntervalMS"`
	GameMode       bool   `json:"gameMode"`
	AutostartSync  bool   `json:"autostartSync"`
}

// ConfigPath returns config.json next to stats.json.
func ConfigPath() (string, error) {
	stats, err := DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(stats), "config.json"), nil
}

// LoadConfig reads config.json or returns defaults.
func LoadConfig() AppConfig {
	cfg := AppConfig{Version: 1, Language: "ru", PollIntervalMS: 1000}
	path, err := ConfigPath()
	if err != nil {
		return cfg
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) == 0 {
		return cfg
	}
	_ = json.Unmarshal(b, &cfg)
	if cfg.PollIntervalMS < 500 {
		cfg.PollIntervalMS = 500
	}
	if cfg.PollIntervalMS > 5000 {
		cfg.PollIntervalMS = 5000
	}
	if cfg.Language == "" {
		cfg.Language = "ru"
	}
	return cfg
}

// SaveConfig writes config.json.
func SaveConfig(cfg AppConfig) error {
	path, err := ConfigPath()
	if err != nil {
		return err
	}
	cfg.Version = 1
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
