package agent

import (
	"encoding/json"
	"os"
	"os/user"
	"path/filepath"
)

type Config struct {
	MasterURL string   `json:"master_url"`
	UUID      string   `json:"uuid"`
	Token     string   `json:"token"`
	Port      int      `json:"port"`
	ScanDirs  []string `json:"scan_dirs"`
	RateLimit int64    `json:"rate_limit"`
}

func configPath() string {
	home := resolveHomeDir()
	return filepath.Join(home, ".cloudnest", "agent.json")
}

func resolveHomeDir() string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		return home
	}

	currentUser, err := user.Current()
	if err == nil && currentUser.HomeDir != "" {
		return currentUser.HomeDir
	}

	return "."
}

func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func SaveConfig(cfg *Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
