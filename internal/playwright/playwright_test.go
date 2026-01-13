package playwright

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// Helper functions for testing
func loadMCPConfig(projectRoot string) (*claudeconfig.MCPConfig, error) {
	return claudeconfig.LoadProjectMCPConfig(projectRoot)
}

func saveMCPConfig(projectRoot string, cfg *claudeconfig.MCPConfig) error {
	return claudeconfig.SaveProjectMCPConfig(projectRoot, cfg)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.Enabled {
		t.Error("expected Enabled to be true")
	}
	if !cfg.Headless {
		t.Error("expected Headless to be true")
	}
	if cfg.Browser != "chromium" {
		t.Errorf("expected Browser to be 'chromium', got %q", cfg.Browser)
	}
}

func TestGetScreenshotDir(t *testing.T) {
	tests := []struct {
		name       string
		projectDir string
		taskID     string
		want       string
	}{
		{
			name:       "current directory",
			projectDir: ".",
			taskID:     "TASK-001",
			want:       filepath.Join(".", ".orc", "tasks", "TASK-001", "test-results", "screenshots"),
		},
		{
			name:       "absolute path",
			projectDir: "/home/user/project",
			taskID:     "TASK-123",
			want:       filepath.Join("/home/user/project", ".orc", "tasks", "TASK-123", "test-results", "screenshots"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetScreenshotDir(tt.projectDir, tt.taskID)
			if got != tt.want {
				t.Errorf("GetScreenshotDir() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnsureScreenshotDir(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	screenshotDir := filepath.Join(tmpDir, "screenshots", "nested")

	// Ensure the directory doesn't exist yet
	if _, err := os.Stat(screenshotDir); !os.IsNotExist(err) {
		t.Fatal("expected directory to not exist")
	}

	// Create the directory
	err := EnsureScreenshotDir(screenshotDir)
	if err != nil {
		t.Fatalf("EnsureScreenshotDir() error = %v", err)
	}

	// Verify it exists
	info, err := os.Stat(screenshotDir)
	if err != nil {
		t.Fatalf("expected directory to exist, got error: %v", err)
	}
	if !info.IsDir() {
		t.Error("expected path to be a directory")
	}

	// Calling again should be idempotent
	err = EnsureScreenshotDir(screenshotDir)
	if err != nil {
		t.Fatalf("EnsureScreenshotDir() second call error = %v", err)
	}
}

func TestIsServerConfigured_NoConfig(t *testing.T) {
	// Create temp directory with no MCP config
	tmpDir := t.TempDir()

	configured := IsServerConfigured(tmpDir)
	if configured {
		t.Error("expected IsServerConfigured to return false when no config exists")
	}
}

func TestEnsureMCPServer(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()

	// Ensure MCP server is configured
	cfg := &Config{
		Enabled:       true,
		ScreenshotDir: filepath.Join(tmpDir, "screenshots"),
		Headless:      true,
		Browser:       "chromium",
	}

	mcpPath, err := EnsureMCPServer(tmpDir, cfg)
	if err != nil {
		t.Fatalf("EnsureMCPServer() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, ".mcp.json")
	if mcpPath != expectedPath {
		t.Errorf("EnsureMCPServer() path = %q, want %q", mcpPath, expectedPath)
	}

	// Verify file exists
	if _, err := os.Stat(mcpPath); err != nil {
		t.Fatalf("expected MCP config file to exist: %v", err)
	}

	// Check server is now configured
	configured := IsServerConfigured(tmpDir)
	if !configured {
		t.Error("expected IsServerConfigured to return true after EnsureMCPServer")
	}
}

func TestEnsureMCPServer_Disabled(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Enabled: false,
	}

	mcpPath, err := EnsureMCPServer(tmpDir, cfg)
	if err != nil {
		t.Fatalf("EnsureMCPServer() error = %v", err)
	}

	if mcpPath != "" {
		t.Errorf("expected empty path for disabled config, got %q", mcpPath)
	}
}

func TestEnsureMCPServer_NilConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Should use default config when nil is passed
	mcpPath, err := EnsureMCPServer(tmpDir, nil)
	if err != nil {
		t.Fatalf("EnsureMCPServer() error = %v", err)
	}

	expectedPath := filepath.Join(tmpDir, ".mcp.json")
	if mcpPath != expectedPath {
		t.Errorf("EnsureMCPServer() path = %q, want %q", mcpPath, expectedPath)
	}
}

func TestEnsureMCPServer_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &Config{
		Enabled:  true,
		Headless: true,
		Browser:  "chromium",
	}

	// First call
	mcpPath1, err := EnsureMCPServer(tmpDir, cfg)
	if err != nil {
		t.Fatalf("EnsureMCPServer() first call error = %v", err)
	}

	// Second call should be idempotent
	mcpPath2, err := EnsureMCPServer(tmpDir, cfg)
	if err != nil {
		t.Fatalf("EnsureMCPServer() second call error = %v", err)
	}

	if mcpPath1 != mcpPath2 {
		t.Errorf("expected same path, got %q and %q", mcpPath1, mcpPath2)
	}
}

func TestEnsureMCPServer_ReenableDisabled(t *testing.T) {
	tmpDir := t.TempDir()

	// First, create a config with Playwright server
	cfg := &Config{
		Enabled:  true,
		Headless: true,
		Browser:  "chromium",
	}

	_, err := EnsureMCPServer(tmpDir, cfg)
	if err != nil {
		t.Fatalf("EnsureMCPServer() initial error = %v", err)
	}

	// Manually disable the server by modifying the config
	mcpConfig, err := loadMCPConfig(tmpDir)
	if err != nil {
		t.Fatalf("loadMCPConfig() error = %v", err)
	}

	server := mcpConfig.GetServer(ServerName)
	if server == nil {
		t.Fatal("expected playwright server to exist")
	}
	server.Disabled = true

	if err := saveMCPConfig(tmpDir, mcpConfig); err != nil {
		t.Fatalf("saveMCPConfig() error = %v", err)
	}

	// Verify server is now disabled
	if IsServerConfigured(tmpDir) {
		t.Error("expected IsServerConfigured to return false after disabling")
	}

	// Re-enable by calling EnsureMCPServer again
	_, err = EnsureMCPServer(tmpDir, cfg)
	if err != nil {
		t.Fatalf("EnsureMCPServer() re-enable error = %v", err)
	}

	// Verify server is now enabled again
	if !IsServerConfigured(tmpDir) {
		t.Error("expected IsServerConfigured to return true after re-enabling")
	}
}
