// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

// MCPServerConfig represents an MCP server configuration.
type MCPServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// MCPConfig represents the .mcp.json file structure.
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// GenerateWorktreeMCPConfig creates a task-specific .mcp.json in the worktree.
// It preserves non-Playwright servers from the project config and adds
// an isolated Playwright server for UI testing tasks.
//
// The generated config ensures each task has its own isolated browser profile,
// preventing conflicts when multiple tasks run in parallel.
func GenerateWorktreeMCPConfig(worktreePath, taskID string, t *task.Task, cfg *config.Config) error {
	if worktreePath == "" {
		return fmt.Errorf("worktree path is required")
	}

	// Start with empty config
	mcpConfig := MCPConfig{
		MCPServers: make(map[string]MCPServerConfig),
	}

	// Load project .mcp.json if it exists (preserves other MCP servers)
	projectMCPPath := filepath.Join(filepath.Dir(worktreePath), "..", "..", ".mcp.json")
	if data, err := os.ReadFile(projectMCPPath); err == nil {
		if err := json.Unmarshal(data, &mcpConfig); err != nil {
			// Invalid JSON in project config - start fresh
			mcpConfig.MCPServers = make(map[string]MCPServerConfig)
		}
	}

	// Check if UI testing is needed
	needsPlaywright := t != nil && t.RequiresUITesting
	if cfg != nil && cfg.MCP.Playwright.Enabled {
		// Global setting can override task-level setting
		needsPlaywright = needsPlaywright || cfg.MCP.Playwright.Enabled
	}

	if needsPlaywright {
		// Configure isolated Playwright server for this task
		playwrightConfig := buildPlaywrightConfig(taskID, cfg)
		mcpConfig.MCPServers["playwright"] = playwrightConfig
	} else {
		// Remove playwright from config if not needed
		delete(mcpConfig.MCPServers, "playwright")
	}

	// Only write config if there are servers to configure
	if len(mcpConfig.MCPServers) == 0 {
		return nil
	}

	// Write to worktree
	worktreeMCPPath := filepath.Join(worktreePath, ".mcp.json")
	data, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal MCP config: %w", err)
	}

	if err := os.WriteFile(worktreeMCPPath, data, 0644); err != nil {
		return fmt.Errorf("write MCP config to %s: %w", worktreeMCPPath, err)
	}

	return nil
}

// buildPlaywrightConfig creates the Playwright MCP server configuration
// with task-specific isolation settings.
func buildPlaywrightConfig(taskID string, cfg *config.Config) MCPServerConfig {
	args := []string{"@playwright/mcp@latest"}

	// Always add isolation flags
	args = append(args, "--isolated")

	// Task-specific user data directory
	userDataDir := fmt.Sprintf("/tmp/playwright-%s", taskID)
	args = append(args, "--user-data-dir", userDataDir)

	// Add optional settings from config
	if cfg != nil {
		playwrightCfg := cfg.MCP.Playwright

		// Headless mode (default: true)
		if playwrightCfg.Headless {
			args = append(args, "--headless")
		}

		// Browser selection (default: chromium)
		if playwrightCfg.Browser != "" && playwrightCfg.Browser != "chromium" {
			args = append(args, "--browser", playwrightCfg.Browser)
		}
	} else {
		// Default to headless when no config
		args = append(args, "--headless")
	}

	return MCPServerConfig{
		Command: "npx",
		Args:    args,
	}
}

// CleanupPlaywrightUserData removes the task-specific Playwright user data directory.
// This is called on task completion to clean up temporary browser profiles.
func CleanupPlaywrightUserData(taskID string) error {
	userDataDir := fmt.Sprintf("/tmp/playwright-%s", taskID)
	if err := os.RemoveAll(userDataDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cleanup playwright user data for %s: %w", taskID, err)
	}
	return nil
}

// ShouldGenerateMCPConfig determines if MCP config should be generated for a task.
// Returns true if the task requires UI testing or if MCP is globally enabled.
func ShouldGenerateMCPConfig(t *task.Task, cfg *config.Config) bool {
	if t != nil && t.RequiresUITesting {
		return true
	}
	if cfg != nil && cfg.MCP.Playwright.Enabled {
		return true
	}
	return false
}
