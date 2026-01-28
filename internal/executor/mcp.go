// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/config"
)

// MergeMCPConfigSettings merges runtime orc config settings into phase MCP servers.
// This applies settings like headless mode and task-specific user-data-dir to
// MCP servers defined in phase templates.
//
// The phase template defines WHICH MCP servers are available.
// The orc config defines HOW those servers should behave at runtime.
func MergeMCPConfigSettings(
	mcpServers map[string]claude.MCPServerConfig,
	taskID string,
	cfg *config.Config,
) map[string]claude.MCPServerConfig {
	if len(mcpServers) == 0 {
		return mcpServers
	}

	// Deep copy to avoid mutating original
	result := make(map[string]claude.MCPServerConfig, len(mcpServers))
	for name, server := range mcpServers {
		// Copy the server config
		copied := claude.MCPServerConfig{
			Command: server.Command,
		}
		if server.Args != nil {
			copied.Args = make([]string, len(server.Args))
			copy(copied.Args, server.Args)
		}
		if server.Env != nil {
			copied.Env = make(map[string]string, len(server.Env))
			maps.Copy(copied.Env, server.Env)
		}
		result[name] = copied
	}

	// Apply runtime settings to playwright server if present
	if playwright, ok := result["playwright"]; ok {
		result["playwright"] = applyPlaywrightRuntimeSettings(playwright, taskID, cfg)
	}

	return result
}

// applyPlaywrightRuntimeSettings adds runtime-specific settings to a Playwright MCP config.
// - Adds --headless if configured (default: true)
// - Adds --user-data-dir for task isolation
// - Adds --browser if non-default browser specified
func applyPlaywrightRuntimeSettings(
	server claude.MCPServerConfig,
	taskID string,
	cfg *config.Config,
) claude.MCPServerConfig {
	args := server.Args
	if args == nil {
		args = []string{}
	}

	// Add task-specific user data directory for isolation (if not already present)
	if !slices.Contains(args, "--user-data-dir") && taskID != "" {
		userDataDir := fmt.Sprintf("/tmp/playwright-%s", taskID)
		args = append(args, "--user-data-dir", userDataDir)
	}

	// Apply config settings
	if cfg != nil {
		playwrightCfg := cfg.MCP.Playwright

		// Headless mode (default: true in config)
		if playwrightCfg.Headless && !slices.Contains(args, "--headless") {
			args = append(args, "--headless")
		}

		// Browser selection (default: chromium, only add if non-default)
		if playwrightCfg.Browser != "" && playwrightCfg.Browser != "chromium" {
			if !slices.Contains(args, "--browser") {
				args = append(args, "--browser", playwrightCfg.Browser)
			}
		}
	} else {
		// Default to headless when no config
		if !slices.Contains(args, "--headless") {
			args = append(args, "--headless")
		}
	}

	// Disable hardware video decode to prevent NVIDIA driver crashes (libnvcuvid.so bug)
	// This must be passed via env var, not CLI arg — Playwright MCP CLI doesn't accept browser flags
	if server.Env == nil {
		server.Env = make(map[string]string)
	}
	if _, ok := server.Env["PLAYWRIGHT_CHROMIUM_ARGS"]; !ok {
		server.Env["PLAYWRIGHT_CHROMIUM_ARGS"] = "--disable-gpu-video-decode"
	}

	server.Args = args
	return server
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
