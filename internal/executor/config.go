// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
)

// Config holds executor configuration.
type Config struct {
	// Claude CLI settings
	ClaudePath                 string
	Model                      string
	DangerouslySkipPermissions bool

	// Tool permissions (from project settings)
	AllowedTools    []string
	DisallowedTools []string

	// Execution settings
	MaxIterations int
	Timeout       time.Duration
	WorkDir       string

	// Git settings
	BranchPrefix string
	CommitPrefix string

	// Template settings
	TemplatesDir string

	// Checkpoint settings
	EnableCheckpoints bool

	// Storage backend (required)
	Backend storage.Backend

	// OrcConfig is a reference to the full orc config for model resolution
	OrcConfig *config.Config
}

// DefaultConfig returns the default executor configuration.
func DefaultConfig() *Config {
	return &Config{
		ClaudePath:                 "claude",
		Model:                      "opus",
		DangerouslySkipPermissions: true,
		MaxIterations:              30,
		Timeout:                    10 * time.Minute,
		WorkDir:                    ".",
		BranchPrefix:               "orc/",
		CommitPrefix:               "[orc]",
		TemplatesDir:               "templates",
		EnableCheckpoints:          true,
	}
}

// ConfigFromOrc creates an executor config from orc config.
func ConfigFromOrc(cfg *config.Config) *Config {
	return &Config{
		ClaudePath:                 cfg.ClaudePath,
		Model:                      cfg.Model,
		DangerouslySkipPermissions: cfg.DangerouslySkipPermissions,
		MaxIterations:              cfg.MaxIterations,
		Timeout:                    cfg.Timeout,
		WorkDir:                    ".",
		BranchPrefix:               cfg.BranchPrefix,
		CommitPrefix:               cfg.CommitPrefix,
		TemplatesDir:               cfg.TemplatesDir,
		EnableCheckpoints:          cfg.EnableCheckpoints,
		OrcConfig:                  cfg,
	}
}

// ResolveModelSetting returns the model and thinking settings for a specific phase and weight.
// Falls back to the default Model field if no orc config is set.
func (c *Config) ResolveModelSetting(weight, phase string) config.PhaseModelSetting {
	if c.OrcConfig != nil {
		return c.OrcConfig.ResolveModelSetting(weight, phase)
	}
	// Fallback to legacy behavior
	return config.PhaseModelSetting{
		Model:    c.Model,
		Thinking: false,
	}
}
