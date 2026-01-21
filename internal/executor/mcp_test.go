package executor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/task"
)

func TestGenerateWorktreeMCPConfig(t *testing.T) {
	t.Parallel()
	t.Run("generates config for UI testing task", func(t *testing.T) {
		tmpDir := t.TempDir()
		worktreePath := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(worktreePath, 0755); err != nil {
			t.Fatal(err)
		}

		tsk := &task.Task{
			ID:                "TASK-001",
			Title:             "Add login button",
			RequiresUITesting: true,
		}
		cfg := config.Default()

		err := GenerateWorktreeMCPConfig(worktreePath, tsk.ID, tsk, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify file was created
		mcpPath := filepath.Join(worktreePath, ".mcp.json")
		data, err := os.ReadFile(mcpPath)
		if err != nil {
			t.Fatalf("failed to read MCP config: %v", err)
		}

		var mcpConfig MCPConfig
		if err := json.Unmarshal(data, &mcpConfig); err != nil {
			t.Fatalf("failed to parse MCP config: %v", err)
		}

		// Verify playwright server is configured
		playwright, ok := mcpConfig.MCPServers["playwright"]
		if !ok {
			t.Fatal("playwright server not found in config")
		}

		if playwright.Command != "npx" {
			t.Errorf("expected command 'npx', got %q", playwright.Command)
		}

		// Verify isolation flags
		hasIsolated := false
		hasUserDataDir := false
		hasHeadless := false
		for i, arg := range playwright.Args {
			if arg == "--isolated" {
				hasIsolated = true
			}
			if arg == "--user-data-dir" && i+1 < len(playwright.Args) {
				hasUserDataDir = true
				expectedDir := "/tmp/playwright-TASK-001"
				if playwright.Args[i+1] != expectedDir {
					t.Errorf("expected user-data-dir %q, got %q", expectedDir, playwright.Args[i+1])
				}
			}
			if arg == "--headless" {
				hasHeadless = true
			}
		}

		if !hasIsolated {
			t.Error("--isolated flag not found")
		}
		if !hasUserDataDir {
			t.Error("--user-data-dir flag not found")
		}
		if !hasHeadless {
			t.Error("--headless flag not found")
		}
	})

	t.Run("skips config for non-UI task when MCP disabled", func(t *testing.T) {
		tmpDir := t.TempDir()
		worktreePath := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(worktreePath, 0755); err != nil {
			t.Fatal(err)
		}

		tsk := &task.Task{
			ID:                "TASK-002",
			Title:             "Fix backend bug",
			RequiresUITesting: false,
		}
		cfg := config.Default()
		cfg.MCP.Playwright.Enabled = false // Disable MCP

		err := GenerateWorktreeMCPConfig(worktreePath, tsk.ID, tsk, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify no file was created (no servers to configure)
		mcpPath := filepath.Join(worktreePath, ".mcp.json")
		if _, err := os.Stat(mcpPath); !os.IsNotExist(err) {
			t.Error("MCP config should not be created for non-UI task with MCP disabled")
		}
	})

	t.Run("preserves other MCP servers from project config", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create project root with .mcp.json
		projectRoot := tmpDir
		worktreeDir := filepath.Join(projectRoot, ".orc", "worktrees")
		worktreePath := filepath.Join(worktreeDir, "orc-TASK-003")
		if err := os.MkdirAll(worktreePath, 0755); err != nil {
			t.Fatal(err)
		}

		// Create project .mcp.json with github server
		projectMCP := MCPConfig{
			MCPServers: map[string]MCPServerConfig{
				"github": {
					Command: "gh",
					Args:    []string{"mcp"},
				},
				"playwright": {
					Command: "npx",
					Args:    []string{"@playwright/mcp@latest"}, // Will be replaced
				},
			},
		}
		projectMCPPath := filepath.Join(projectRoot, ".mcp.json")
		data, _ := json.Marshal(projectMCP)
		if err := os.WriteFile(projectMCPPath, data, 0644); err != nil {
			t.Fatal(err)
		}

		tsk := &task.Task{
			ID:                "TASK-003",
			Title:             "Add modal component",
			RequiresUITesting: true,
		}
		cfg := config.Default()

		err := GenerateWorktreeMCPConfig(worktreePath, tsk.ID, tsk, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Read generated config
		worktreeMCPPath := filepath.Join(worktreePath, ".mcp.json")
		data, err = os.ReadFile(worktreeMCPPath)
		if err != nil {
			t.Fatalf("failed to read MCP config: %v", err)
		}

		var mcpConfig MCPConfig
		if err := json.Unmarshal(data, &mcpConfig); err != nil {
			t.Fatalf("failed to parse MCP config: %v", err)
		}

		// Verify github server was preserved
		github, ok := mcpConfig.MCPServers["github"]
		if !ok {
			t.Fatal("github server not preserved from project config")
		}
		if github.Command != "gh" {
			t.Errorf("github command changed, expected 'gh', got %q", github.Command)
		}

		// Verify playwright was replaced with isolated version
		playwright, ok := mcpConfig.MCPServers["playwright"]
		if !ok {
			t.Fatal("playwright server not found")
		}

		// Check for task-specific user data dir
		hasTaskDir := false
		for i, arg := range playwright.Args {
			if arg == "--user-data-dir" && i+1 < len(playwright.Args) {
				if playwright.Args[i+1] == "/tmp/playwright-TASK-003" {
					hasTaskDir = true
				}
			}
		}
		if !hasTaskDir {
			t.Error("playwright should have task-specific user-data-dir")
		}
	})

	t.Run("respects headless=false config", func(t *testing.T) {
		tmpDir := t.TempDir()
		worktreePath := filepath.Join(tmpDir, "worktree")
		if err := os.MkdirAll(worktreePath, 0755); err != nil {
			t.Fatal(err)
		}

		tsk := &task.Task{
			ID:                "TASK-004",
			Title:             "Debug form submission",
			RequiresUITesting: true,
		}
		cfg := config.Default()
		cfg.MCP.Playwright.Headless = false // Headed mode for debugging

		err := GenerateWorktreeMCPConfig(worktreePath, tsk.ID, tsk, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Read and parse config
		mcpPath := filepath.Join(worktreePath, ".mcp.json")
		data, _ := os.ReadFile(mcpPath)
		var mcpConfig MCPConfig
		_ = json.Unmarshal(data, &mcpConfig)

		playwright := mcpConfig.MCPServers["playwright"]
		for _, arg := range playwright.Args {
			if arg == "--headless" {
				t.Error("--headless should not be present when headless=false")
			}
		}
	})
}

func TestShouldGenerateMCPConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		requiresUITesting bool
		mcpEnabled        bool
		want              bool
	}{
		{
			name:              "UI task with MCP enabled",
			requiresUITesting: true,
			mcpEnabled:        true,
			want:              true,
		},
		{
			name:              "UI task with MCP disabled",
			requiresUITesting: true,
			mcpEnabled:        false,
			want:              true, // Task-level flag takes precedence
		},
		{
			name:              "non-UI task with MCP enabled",
			requiresUITesting: false,
			mcpEnabled:        true,
			want:              true, // Global MCP enables for all tasks
		},
		{
			name:              "non-UI task with MCP disabled",
			requiresUITesting: false,
			mcpEnabled:        false,
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsk := &task.Task{
				ID:                "TASK-TEST",
				RequiresUITesting: tt.requiresUITesting,
			}
			cfg := config.Default()
			cfg.MCP.Playwright.Enabled = tt.mcpEnabled

			got := ShouldGenerateMCPConfig(tsk, cfg)
			if got != tt.want {
				t.Errorf("ShouldGenerateMCPConfig() = %v, want %v", got, tt.want)
			}
		})
	}
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
		testFile := filepath.Join(userDataDir, "test.txt")
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
