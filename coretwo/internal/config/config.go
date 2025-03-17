package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config represents the main configuration structure
type Config struct {
	BasePath       string `json:"basePath"`
	LogPath        string `json:"logPath"`
	ConfigPath     string `json:"configPath"`
	APIIP          string `json:"apiIP"`
	APIPort        string `json:"apiPort"`
	Minimal        bool   `json:"minimal"`
	ConsoleLogOnly bool   `json:"consoleLogOnly"`
}

// DefaultConfig returns a new Config with default values
func DefaultConfig() *Config {
	return &Config{
		APIIP:   "127.0.0.1",
		APIPort: "8080",
	}
}

// Load loads the configuration from disk
func Load(configPath, basePath string) (*Config, error) {
	cfg := DefaultConfig()

	// Set base path
	if basePath != "" {
		cfg.BasePath = basePath
	} else {
		// Use current directory as base path
		dir, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		cfg.BasePath = dir
	}

	// Set derived paths
	cfg.LogPath = filepath.Join(cfg.BasePath, "logs")
	cfg.ConfigPath = filepath.Join(cfg.BasePath, "config.json")

	// Load config file if it exists
	if configPath != "" {
		cfg.ConfigPath = configPath
	}

	if err := cfg.loadFromFile(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// loadFromFile loads the configuration from the config file
func (c *Config) loadFromFile() error {
	data, err := os.ReadFile(c.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config file
			return c.saveToFile()
		}
		return err
	}

	return json.Unmarshal(data, c)
}

// saveToFile saves the configuration to disk
func (c *Config) saveToFile() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(c.ConfigPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(c.ConfigPath, data, 0644)
}
