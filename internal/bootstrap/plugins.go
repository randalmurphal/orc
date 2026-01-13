package bootstrap

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// ClaudeSettingsFile is the path to Claude's settings file.
	ClaudeSettingsFile = ".claude/settings.json"

	// OrcMarketplace is the GitHub repo for the orc marketplace.
	OrcMarketplace = "randalmurphal/orc-claude-plugin"

	// OrcPluginName is the name of the orc plugin.
	OrcPluginName = "orc"
)

// ClaudeSettings represents the .claude/settings.json file structure.
type ClaudeSettings struct {
	ExtraKnownMarketplaces map[string]MarketplaceSource `json:"extraKnownMarketplaces,omitempty"`
	EnabledPlugins         map[string]bool              `json:"enabledPlugins,omitempty"`
}

// MarketplaceSource defines where to fetch a marketplace from.
type MarketplaceSource struct {
	Source GitHubSource `json:"source"`
}

// GitHubSource defines a GitHub-based source.
type GitHubSource struct {
	Source string `json:"source"`
	Repo   string `json:"repo"`
}

// InstallPlugins configures the orc marketplace in .claude/settings.json.
func InstallPlugins(projectDir string) error {
	settingsPath := filepath.Join(projectDir, ClaudeSettingsFile)

	// Ensure .claude directory exists
	claudeDir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		return fmt.Errorf("create .claude directory: %w", err)
	}

	// Load existing settings or create new
	settings := &ClaudeSettings{
		ExtraKnownMarketplaces: make(map[string]MarketplaceSource),
		EnabledPlugins:         make(map[string]bool),
	}

	if data, err := os.ReadFile(settingsPath); err == nil {
		if err := json.Unmarshal(data, settings); err != nil {
			return fmt.Errorf("parse existing settings: %w", err)
		}
		// Ensure maps are initialized
		if settings.ExtraKnownMarketplaces == nil {
			settings.ExtraKnownMarketplaces = make(map[string]MarketplaceSource)
		}
		if settings.EnabledPlugins == nil {
			settings.EnabledPlugins = make(map[string]bool)
		}
	}

	// Add orc marketplace
	settings.ExtraKnownMarketplaces["orc"] = MarketplaceSource{
		Source: GitHubSource{
			Source: "github",
			Repo:   OrcMarketplace,
		},
	}

	// Enable orc plugin
	settings.EnabledPlugins["orc@orc"] = true

	// Write settings
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	return nil
}

// PluginsInstalled checks if orc marketplace is configured.
func PluginsInstalled(projectDir string) bool {
	settingsPath := filepath.Join(projectDir, ClaudeSettingsFile)

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return false
	}

	var settings ClaudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return false
	}

	_, hasMarketplace := settings.ExtraKnownMarketplaces["orc"]
	return hasMarketplace
}
