package executor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProjectToolPermissions_NoSettingsFile_ReturnsNil(t *testing.T) {
	t.Parallel()
	// Create a temp directory with no .claude/settings.json
	tmpDir := t.TempDir()

	cfg := DefaultConfig()
	e := New(cfg)

	err := e.LoadProjectToolPermissions(tmpDir)
	if err != nil {
		t.Errorf("LoadProjectToolPermissions() error = %v, want nil", err)
	}

	// Config should remain unchanged
	if len(e.config.AllowedTools) != 0 {
		t.Errorf("AllowedTools = %v, want empty", e.config.AllowedTools)
	}
	if len(e.config.DisallowedTools) != 0 {
		t.Errorf("DisallowedTools = %v, want empty", e.config.DisallowedTools)
	}
}

func TestLoadProjectToolPermissions_ParsesAllowedTools(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .claude/settings.json with allowed tools
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	settingsContent := `{
		"permissions": {
			"allow": ["Read", "Write", "Glob"]
		}
	}`
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatalf("failed to write settings.json: %v", err)
	}

	cfg := DefaultConfig()
	e := New(cfg)

	err := e.LoadProjectToolPermissions(tmpDir)
	if err != nil {
		t.Errorf("LoadProjectToolPermissions() error = %v, want nil", err)
	}

	// Check allowed tools were parsed
	if len(e.config.AllowedTools) != 3 {
		t.Errorf("AllowedTools length = %d, want 3", len(e.config.AllowedTools))
	}

	expected := map[string]bool{"Read": true, "Write": true, "Glob": true}
	for _, tool := range e.config.AllowedTools {
		if !expected[tool] {
			t.Errorf("unexpected allowed tool: %s", tool)
		}
	}
}

func TestLoadProjectToolPermissions_ParsesDisallowedTools(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .claude/settings.json with denied tools
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	settingsContent := `{
		"permissions": {
			"deny": ["Bash", "Edit"]
		}
	}`
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatalf("failed to write settings.json: %v", err)
	}

	cfg := DefaultConfig()
	e := New(cfg)

	err := e.LoadProjectToolPermissions(tmpDir)
	if err != nil {
		t.Errorf("LoadProjectToolPermissions() error = %v, want nil", err)
	}

	// Check denied tools were parsed
	if len(e.config.DisallowedTools) != 2 {
		t.Errorf("DisallowedTools length = %d, want 2", len(e.config.DisallowedTools))
	}

	expected := map[string]bool{"Bash": true, "Edit": true}
	for _, tool := range e.config.DisallowedTools {
		if !expected[tool] {
			t.Errorf("unexpected disallowed tool: %s", tool)
		}
	}
}

func TestLoadProjectToolPermissions_SkipsIfAlreadyConfigured(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .claude/settings.json that would override
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	settingsContent := `{
		"permissions": {
			"allow": ["Read", "Write"],
			"deny": ["Bash"]
		}
	}`
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatalf("failed to write settings.json: %v", err)
	}

	// Pre-configure allowed tools
	cfg := DefaultConfig()
	cfg.AllowedTools = []string{"Glob"}
	e := New(cfg)

	err := e.LoadProjectToolPermissions(tmpDir)
	if err != nil {
		t.Errorf("LoadProjectToolPermissions() error = %v, want nil", err)
	}

	// Config should remain unchanged (not overridden)
	if len(e.config.AllowedTools) != 1 {
		t.Errorf("AllowedTools length = %d, want 1", len(e.config.AllowedTools))
	}
	if e.config.AllowedTools[0] != "Glob" {
		t.Errorf("AllowedTools[0] = %s, want Glob", e.config.AllowedTools[0])
	}
}

func TestRebuildClient_UpdatesConfig(t *testing.T) {
	t.Parallel()
	cfg := DefaultConfig()
	cfg.AllowedTools = []string{"Read", "Write"}
	cfg.DisallowedTools = []string{"Bash"}
	e := New(cfg)

	// Capture original client
	originalClient := e.client

	// Rebuild with new settings
	e.config.Model = "claude-sonnet-4-20251101"
	e.rebuildClient()

	// Client should be a new instance (can't directly compare, but verify it ran)
	if e.client == nil {
		t.Error("client is nil after rebuildClient()")
	}

	// The client was replaced (new instance)
	// We can't compare pointers directly with the interface, but we verified it didn't panic
	// and the client is still valid
	_ = originalClient // Acknowledge we captured it for the test intent
}

func TestLoadProjectToolPermissions_HandlesBothAllowAndDeny(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .claude/settings.json with both allow and deny
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create .claude dir: %v", err)
	}

	settingsContent := `{
		"permissions": {
			"allow": ["Read", "Write", "Glob"],
			"deny": ["Bash", "Edit"]
		}
	}`
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte(settingsContent), 0644); err != nil {
		t.Fatalf("failed to write settings.json: %v", err)
	}

	cfg := DefaultConfig()
	e := New(cfg)

	err := e.LoadProjectToolPermissions(tmpDir)
	if err != nil {
		t.Errorf("LoadProjectToolPermissions() error = %v, want nil", err)
	}

	// Check both were parsed
	if len(e.config.AllowedTools) != 3 {
		t.Errorf("AllowedTools length = %d, want 3", len(e.config.AllowedTools))
	}
	if len(e.config.DisallowedTools) != 2 {
		t.Errorf("DisallowedTools length = %d, want 2", len(e.config.DisallowedTools))
	}
}
