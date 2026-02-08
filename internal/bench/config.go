package bench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SuiteConfig is the top-level benchmark configuration loaded from suite.yaml.
// All model/variant definitions are config-driven: adding a new model means
// editing this file, not changing code.
type SuiteConfig struct {
	Projects []Project      `yaml:"projects"`
	Tasks    []Task         `yaml:"tasks,omitempty"`
	Variants []Variant      `yaml:"variants"`
	Throttle ThrottleConfig `yaml:"throttle,omitempty"`
}

// ThrottleConfig controls parallelism to respect API rate limits.
type ThrottleConfig struct {
	MaxParallelClaude    int `yaml:"max_parallel_claude"`
	MaxParallelCodex     int `yaml:"max_parallel_codex"`
	DelayBetweenClaudeMs int `yaml:"delay_between_claude_ms"`
}

// DefaultThrottle returns conservative defaults for Claude Max subscription.
func DefaultThrottle() ThrottleConfig {
	return ThrottleConfig{
		MaxParallelClaude:    1,
		MaxParallelCodex:     4,
		DelayBetweenClaudeMs: 5000,
	}
}

// DefaultSuiteConfigPath returns the default path for suite.yaml.
func DefaultSuiteConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".orc", "bench", "suite.yaml"), nil
}

// LoadSuiteConfig loads a suite configuration from a YAML file.
func LoadSuiteConfig(path string) (*SuiteConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read suite config %s: %w", path, err)
	}

	var cfg SuiteConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse suite config %s: %w", path, err)
	}

	// Apply defaults
	if cfg.Throttle.MaxParallelClaude == 0 {
		cfg.Throttle = DefaultThrottle()
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate suite config: %w", err)
	}

	return &cfg, nil
}

// Validate checks the suite configuration for errors.
func (c *SuiteConfig) Validate() error {
	if len(c.Variants) == 0 {
		return fmt.Errorf("at least one variant is required")
	}

	// Check for exactly one baseline
	baselineCount := 0
	for _, v := range c.Variants {
		if v.IsBaseline {
			baselineCount++
		}
	}
	if baselineCount == 0 {
		return fmt.Errorf("exactly one variant must be marked as baseline (is_baseline: true)")
	}
	if baselineCount > 1 {
		return fmt.Errorf("only one variant can be baseline, found %d", baselineCount)
	}

	// Validate projects
	projectIDs := make(map[string]bool)
	for i := range c.Projects {
		if err := c.Projects[i].Validate(); err != nil {
			return err
		}
		if projectIDs[c.Projects[i].ID] {
			return fmt.Errorf("duplicate project id: %s", c.Projects[i].ID)
		}
		projectIDs[c.Projects[i].ID] = true
	}

	// Validate tasks
	for i := range c.Tasks {
		if err := c.Tasks[i].Validate(); err != nil {
			return err
		}
		if !projectIDs[c.Tasks[i].ProjectID] && len(c.Projects) > 0 {
			return fmt.Errorf("task %s references unknown project %s", c.Tasks[i].ID, c.Tasks[i].ProjectID)
		}
	}

	// Validate variants
	variantIDs := make(map[string]bool)
	for i := range c.Variants {
		if err := c.Variants[i].Validate(); err != nil {
			return err
		}
		if variantIDs[c.Variants[i].ID] {
			return fmt.Errorf("duplicate variant id: %s", c.Variants[i].ID)
		}
		variantIDs[c.Variants[i].ID] = true
	}

	return nil
}

// ImportToStore saves all projects, tasks, and variants from config to the store.
func (c *SuiteConfig) ImportToStore(ctx context.Context, store *Store) error {
	for i := range c.Projects {
		if err := store.SaveProject(ctx, &c.Projects[i]); err != nil {
			return fmt.Errorf("import project %s: %w", c.Projects[i].ID, err)
		}
	}

	for i := range c.Tasks {
		if err := store.SaveTask(ctx, &c.Tasks[i]); err != nil {
			return fmt.Errorf("import task %s: %w", c.Tasks[i].ID, err)
		}
	}

	for i := range c.Variants {
		if err := store.SaveVariant(ctx, &c.Variants[i]); err != nil {
			return fmt.Errorf("import variant %s: %w", c.Variants[i].ID, err)
		}
	}

	return nil
}
