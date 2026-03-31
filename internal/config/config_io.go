package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Load loads the config using the full loader with proper path expansion.
func Load() (*Config, error) {
	tc, err := LoadWithSources()
	if err != nil {
		return nil, err
	}
	return tc.Config, nil
}

// LoadFrom loads the config from a specific project directory.
func LoadFrom(projectDir string) (*Config, error) {
	tc, err := LoadWithSourcesFrom(projectDir)
	if err != nil {
		return nil, err
	}
	return tc.Config, nil
}

// LoadFile loads config from a specific file path (for config editing).
func LoadFile(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	cfg.ClaudePath = ExpandPath(cfg.ClaudePath)
	cfg.CodexPath = ExpandPath(cfg.CodexPath)

	return cfg, nil
}

// Save saves the config to the default location.
func (c *Config) Save() error {
	return c.SaveTo(filepath.Join(OrcDir, ConfigFileName))
}

// SaveTo saves the config to a specific path.
func (c *Config) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}
