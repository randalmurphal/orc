package executor

import (
	"testing"
	"time"
)

func TestDefaultConfig_SetsCorrectDefaults(t *testing.T) {
	cfg := DefaultConfig()

	// Claude CLI settings
	if cfg.ClaudePath != "claude" {
		t.Errorf("ClaudePath = %q, want %q", cfg.ClaudePath, "claude")
	}
	if cfg.Model != "claude-opus-4-5-20251101" {
		t.Errorf("Model = %q, want %q", cfg.Model, "claude-opus-4-5-20251101")
	}
	if !cfg.DangerouslySkipPermissions {
		t.Error("DangerouslySkipPermissions = false, want true")
	}

	// Tool permissions should be nil/empty by default
	if len(cfg.AllowedTools) != 0 {
		t.Errorf("AllowedTools = %v, want empty", cfg.AllowedTools)
	}
	if len(cfg.DisallowedTools) != 0 {
		t.Errorf("DisallowedTools = %v, want empty", cfg.DisallowedTools)
	}

	// Execution settings
	if cfg.MaxIterations != 30 {
		t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, 30)
	}
	if cfg.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 10*time.Minute)
	}
	if cfg.WorkDir != "." {
		t.Errorf("WorkDir = %q, want %q", cfg.WorkDir, ".")
	}

	// Git settings
	if cfg.BranchPrefix != "orc/" {
		t.Errorf("BranchPrefix = %q, want %q", cfg.BranchPrefix, "orc/")
	}
	if cfg.CommitPrefix != "[orc]" {
		t.Errorf("CommitPrefix = %q, want %q", cfg.CommitPrefix, "[orc]")
	}

	// Template settings
	if cfg.TemplatesDir != "templates" {
		t.Errorf("TemplatesDir = %q, want %q", cfg.TemplatesDir, "templates")
	}

	// Checkpoint settings
	if !cfg.EnableCheckpoints {
		t.Error("EnableCheckpoints = false, want true")
	}
}

func TestConfig_AllFieldsExported(t *testing.T) {
	// Verify that the struct can be created externally with all fields
	cfg := &Config{
		ClaudePath:                 "/custom/claude",
		Model:                      "custom-model",
		DangerouslySkipPermissions: false,
		AllowedTools:               []string{"Read", "Write"},
		DisallowedTools:            []string{"Bash"},
		MaxIterations:              50,
		Timeout:                    5 * time.Minute,
		WorkDir:                    "/custom/work",
		BranchPrefix:               "feature/",
		CommitPrefix:               "[custom]",
		TemplatesDir:               "/custom/templates",
		EnableCheckpoints:          false,
	}

	// Verify all fields were set correctly
	if cfg.ClaudePath != "/custom/claude" {
		t.Errorf("ClaudePath = %q, want %q", cfg.ClaudePath, "/custom/claude")
	}
	if cfg.Model != "custom-model" {
		t.Errorf("Model = %q, want %q", cfg.Model, "custom-model")
	}
	if cfg.DangerouslySkipPermissions {
		t.Error("DangerouslySkipPermissions = true, want false")
	}
	if len(cfg.AllowedTools) != 2 {
		t.Errorf("AllowedTools length = %d, want %d", len(cfg.AllowedTools), 2)
	}
	if len(cfg.DisallowedTools) != 1 {
		t.Errorf("DisallowedTools length = %d, want %d", len(cfg.DisallowedTools), 1)
	}
	if cfg.MaxIterations != 50 {
		t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, 50)
	}
	if cfg.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, 5*time.Minute)
	}
	if cfg.WorkDir != "/custom/work" {
		t.Errorf("WorkDir = %q, want %q", cfg.WorkDir, "/custom/work")
	}
	if cfg.BranchPrefix != "feature/" {
		t.Errorf("BranchPrefix = %q, want %q", cfg.BranchPrefix, "feature/")
	}
	if cfg.CommitPrefix != "[custom]" {
		t.Errorf("CommitPrefix = %q, want %q", cfg.CommitPrefix, "[custom]")
	}
	if cfg.TemplatesDir != "/custom/templates" {
		t.Errorf("TemplatesDir = %q, want %q", cfg.TemplatesDir, "/custom/templates")
	}
	if cfg.EnableCheckpoints {
		t.Error("EnableCheckpoints = true, want false")
	}
}
