// Package playwright provides Playwright MCP server configuration for UI testing.
package playwright

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

const (
	// ServerName is the name of the Playwright MCP server.
	ServerName = "playwright"

	// DefaultCommand is the default command to run the Playwright MCP server.
	DefaultCommand = "npx"

	// DefaultPackage is the default npm package for the Playwright MCP server.
	DefaultPackage = "@anthropic/mcp-playwright"
)

// Config holds Playwright MCP configuration options.
type Config struct {
	// Enabled indicates if Playwright MCP should be active.
	Enabled bool

	// ScreenshotDir is the directory where screenshots will be saved.
	// If empty, screenshots are saved to the current working directory.
	ScreenshotDir string

	// Headless controls whether the browser runs in headless mode.
	// Defaults to true for automated testing.
	Headless bool

	// Browser specifies the browser to use (chromium, firefox, webkit).
	// Defaults to chromium.
	Browser string
}

// DefaultConfig returns the default Playwright configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:  true,
		Headless: true,
		Browser:  "chromium",
	}
}

// EnsureMCPServer ensures the Playwright MCP server is configured in .mcp.json.
// It creates or updates the MCP configuration file at the specified project root.
// Returns the path to the MCP config file and any error encountered.
func EnsureMCPServer(projectRoot string, cfg *Config) (string, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	if !cfg.Enabled {
		return "", nil
	}

	mcpConfigPath := filepath.Join(projectRoot, ".mcp.json")

	// Load existing config or create new one
	mcpConfig, err := claudeconfig.LoadProjectMCPConfig(projectRoot)
	if err != nil {
		// Create new config if none exists
		mcpConfig = claudeconfig.NewMCPConfig()
	}

	// Check if Playwright server already exists
	existing := mcpConfig.GetServer(ServerName)
	if existing != nil && !existing.Disabled {
		// Server already configured and enabled
		return mcpConfigPath, nil
	}

	// Build server configuration
	server := &claudeconfig.MCPServer{
		Type:    "stdio",
		Command: DefaultCommand,
		Args:    []string{"-y", DefaultPackage},
		Env:     make(map[string]string),
	}

	// Set environment variables based on config
	if cfg.Headless {
		server.Env["PLAYWRIGHT_HEADLESS"] = "true"
	} else {
		server.Env["PLAYWRIGHT_HEADLESS"] = "false"
	}

	if cfg.Browser != "" {
		server.Env["PLAYWRIGHT_BROWSER"] = cfg.Browser
	}

	if cfg.ScreenshotDir != "" {
		server.Env["PLAYWRIGHT_SCREENSHOT_DIR"] = cfg.ScreenshotDir
	}

	// Add or update server
	if existing != nil {
		// Re-enable and update existing server
		existing.Disabled = false
		existing.Args = server.Args
		existing.Env = server.Env
	} else {
		if err := mcpConfig.AddServer(ServerName, server); err != nil {
			return "", fmt.Errorf("add playwright server: %w", err)
		}
	}

	// Save config
	if err := claudeconfig.SaveProjectMCPConfig(projectRoot, mcpConfig); err != nil {
		return "", fmt.Errorf("save MCP config: %w", err)
	}

	return mcpConfigPath, nil
}

// IsServerConfigured checks if the Playwright MCP server is configured.
func IsServerConfigured(projectRoot string) bool {
	mcpConfig, err := claudeconfig.LoadProjectMCPConfig(projectRoot)
	if err != nil {
		return false
	}

	server := mcpConfig.GetServer(ServerName)
	return server != nil && !server.Disabled
}

// GetScreenshotDir returns the screenshot directory for a task.
// It returns the absolute path to the task's attachments directory.
func GetScreenshotDir(projectDir, taskID string) string {
	return filepath.Join(projectDir, ".orc", "tasks", taskID, "attachments")
}

// EnsureScreenshotDir creates the screenshot directory if it doesn't exist.
func EnsureScreenshotDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}
