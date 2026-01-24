package executor

import (
	"os"
	"testing"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeMCPConfigSettings(t *testing.T) {
	t.Parallel()

	t.Run("nil servers returns nil", func(t *testing.T) {
		result := MergeMCPConfigSettings(nil, "TASK-001", nil)
		assert.Nil(t, result)
	})

	t.Run("empty servers returns empty", func(t *testing.T) {
		result := MergeMCPConfigSettings(map[string]claude.MCPServerConfig{}, "TASK-001", nil)
		assert.Empty(t, result)
	})

	t.Run("adds headless and user-data-dir to playwright", func(t *testing.T) {
		servers := map[string]claude.MCPServerConfig{
			"playwright": {
				Command: "npx",
				Args:    []string{"@playwright/mcp@latest", "--isolated"},
			},
		}

		cfg := &config.Config{}
		cfg.MCP.Playwright.Headless = true

		result := MergeMCPConfigSettings(servers, "TASK-001", cfg)

		require.Contains(t, result, "playwright")
		pw := result["playwright"]
		assert.Equal(t, "npx", pw.Command)
		assert.Contains(t, pw.Args, "--headless")
		assert.Contains(t, pw.Args, "--user-data-dir")
		assert.Contains(t, pw.Args, "/tmp/playwright-TASK-001")
	})

	t.Run("does not duplicate existing flags", func(t *testing.T) {
		servers := map[string]claude.MCPServerConfig{
			"playwright": {
				Command: "npx",
				Args:    []string{"@playwright/mcp@latest", "--isolated", "--headless"},
			},
		}

		cfg := &config.Config{}
		cfg.MCP.Playwright.Headless = true

		result := MergeMCPConfigSettings(servers, "TASK-001", cfg)

		pw := result["playwright"]
		// Count occurrences of --headless
		headlessCount := 0
		for _, arg := range pw.Args {
			if arg == "--headless" {
				headlessCount++
			}
		}
		assert.Equal(t, 1, headlessCount, "should not duplicate --headless")
	})

	t.Run("defaults to headless when no config", func(t *testing.T) {
		servers := map[string]claude.MCPServerConfig{
			"playwright": {
				Command: "npx",
				Args:    []string{"@playwright/mcp@latest"},
			},
		}

		result := MergeMCPConfigSettings(servers, "TASK-001", nil)

		pw := result["playwright"]
		assert.Contains(t, pw.Args, "--headless")
	})

	t.Run("respects headless=false in config", func(t *testing.T) {
		servers := map[string]claude.MCPServerConfig{
			"playwright": {
				Command: "npx",
				Args:    []string{"@playwright/mcp@latest"},
			},
		}

		cfg := &config.Config{}
		cfg.MCP.Playwright.Headless = false

		result := MergeMCPConfigSettings(servers, "TASK-001", cfg)

		pw := result["playwright"]
		assert.NotContains(t, pw.Args, "--headless")
	})

	t.Run("adds non-default browser", func(t *testing.T) {
		servers := map[string]claude.MCPServerConfig{
			"playwright": {
				Command: "npx",
				Args:    []string{"@playwright/mcp@latest"},
			},
		}

		cfg := &config.Config{}
		cfg.MCP.Playwright.Browser = "firefox"

		result := MergeMCPConfigSettings(servers, "TASK-001", cfg)

		pw := result["playwright"]
		assert.Contains(t, pw.Args, "--browser")
		assert.Contains(t, pw.Args, "firefox")
	})

	t.Run("does not add browser flag for chromium default", func(t *testing.T) {
		servers := map[string]claude.MCPServerConfig{
			"playwright": {
				Command: "npx",
				Args:    []string{"@playwright/mcp@latest"},
			},
		}

		cfg := &config.Config{}
		cfg.MCP.Playwright.Browser = "chromium"

		result := MergeMCPConfigSettings(servers, "TASK-001", cfg)

		pw := result["playwright"]
		assert.NotContains(t, pw.Args, "--browser")
	})

	t.Run("preserves non-playwright servers unchanged", func(t *testing.T) {
		servers := map[string]claude.MCPServerConfig{
			"github": {
				Command: "gh",
				Args:    []string{"mcp"},
				Env:     map[string]string{"GH_TOKEN": "secret"},
			},
			"playwright": {
				Command: "npx",
				Args:    []string{"@playwright/mcp@latest"},
			},
		}

		cfg := &config.Config{}
		cfg.MCP.Playwright.Headless = true

		result := MergeMCPConfigSettings(servers, "TASK-001", cfg)

		// GitHub server should be unchanged
		require.Contains(t, result, "github")
		gh := result["github"]
		assert.Equal(t, "gh", gh.Command)
		assert.Equal(t, []string{"mcp"}, gh.Args)
		assert.Equal(t, map[string]string{"GH_TOKEN": "secret"}, gh.Env)

		// Playwright should have runtime settings
		require.Contains(t, result, "playwright")
		pw := result["playwright"]
		assert.Contains(t, pw.Args, "--headless")
	})

	t.Run("does not mutate original map", func(t *testing.T) {
		original := map[string]claude.MCPServerConfig{
			"playwright": {
				Command: "npx",
				Args:    []string{"@playwright/mcp@latest"},
			},
		}

		cfg := &config.Config{}
		cfg.MCP.Playwright.Headless = true

		result := MergeMCPConfigSettings(original, "TASK-001", cfg)

		// Original should be unchanged
		assert.NotContains(t, original["playwright"].Args, "--headless")

		// Result should have the new args
		assert.Contains(t, result["playwright"].Args, "--headless")
	})
}

func TestCleanupPlaywrightUserData(t *testing.T) {
	t.Parallel()

	t.Run("removes existing directory", func(t *testing.T) {
		// Create temp dir simulating playwright user data
		taskID := "TASK-CLEANUP-TEST"
		userDataDir := "/tmp/playwright-" + taskID
		if err := os.MkdirAll(userDataDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Create a file inside
		testFile := userDataDir + "/test.txt"
		_ = os.WriteFile(testFile, []byte("test"), 0644)

		err := CleanupPlaywrightUserData(taskID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(userDataDir); !os.IsNotExist(err) {
			t.Error("user data directory should be removed")
		}
	})

	t.Run("handles non-existent directory gracefully", func(t *testing.T) {
		err := CleanupPlaywrightUserData("TASK-NONEXISTENT")
		if err != nil {
			t.Errorf("should not error on non-existent directory: %v", err)
		}
	})
}
