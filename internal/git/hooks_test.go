package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratePrePushHook(t *testing.T) {
	hook := generatePrePushHook("orc/TASK-001", "TASK-001", []string{"main", "master"})

	// Check that hook contains expected elements
	if !strings.Contains(hook, "#!/bin/bash") {
		t.Error("hook should start with shebang")
	}
	if !strings.Contains(hook, "PROTECTED_BRANCHES=\"main master\"") {
		t.Error("hook should contain protected branches")
	}
	if !strings.Contains(hook, "TASK_BRANCH=\"orc/TASK-001\"") {
		t.Error("hook should contain task branch")
	}
	if !strings.Contains(hook, "TASK_ID=\"TASK-001\"") {
		t.Error("hook should contain task ID")
	}
	if !strings.Contains(hook, "BLOCKED") {
		t.Error("hook should contain blocking message")
	}
}

func TestGeneratePreCommitHook(t *testing.T) {
	hook := generatePreCommitHook("orc/TASK-001", "TASK-001")

	if !strings.Contains(hook, "#!/bin/bash") {
		t.Error("hook should start with shebang")
	}
	if !strings.Contains(hook, "EXPECTED_BRANCH=\"orc/TASK-001\"") {
		t.Error("hook should contain expected branch")
	}
	if !strings.Contains(hook, "git rev-parse") {
		t.Error("hook should check current branch")
	}
}

func TestIsProtectedBranch(t *testing.T) {
	tests := []struct {
		branch    string
		protected []string
		want      bool
	}{
		{"main", nil, true},                                    // Uses default list
		{"master", nil, true},                                  // Uses default list
		{"develop", nil, true},                                 // Uses default list
		{"release", nil, true},                                 // Uses default list
		{"orc/TASK-001", nil, false},                           // Task branch not protected
		{"feature/foo", nil, false},                            // Feature branch not protected
		{"main", []string{"main", "production"}, true},         // Custom list
		{"master", []string{"main", "production"}, false},      // Not in custom list
		{"production", []string{"main", "production"}, true},   // In custom list
	}

	for _, tt := range tests {
		got := IsProtectedBranch(tt.branch, tt.protected)
		if got != tt.want {
			t.Errorf("IsProtectedBranch(%q, %v) = %v, want %v",
				tt.branch, tt.protected, got, tt.want)
		}
	}
}

func TestWriteExecutableFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test-hook")

	content := "#!/bin/bash\necho test"
	err := writeExecutableFile(path, content)
	if err != nil {
		t.Fatalf("writeExecutableFile failed: %v", err)
	}

	// Check file exists
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	// Check file is executable
	mode := info.Mode()
	if mode&0111 == 0 {
		t.Error("file should be executable")
	}

	// Check content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if string(data) != content {
		t.Errorf("content mismatch: got %q, want %q", string(data), content)
	}
}

func TestHookConfig_Defaults(t *testing.T) {
	cfg := HookConfig{
		TaskBranch: "orc/TASK-001",
		TaskID:     "TASK-001",
		// ProtectedBranches left empty - should use defaults
	}

	hook := generatePrePushHook(cfg.TaskBranch, cfg.TaskID, cfg.ProtectedBranches)

	// Should contain default protected branches when empty
	for _, branch := range DefaultProtectedBranches {
		if !strings.Contains(hook, branch) {
			t.Errorf("hook should contain default protected branch %q", branch)
		}
	}
}

func TestHookConfig_CustomProtectedBranches(t *testing.T) {
	cfg := HookConfig{
		TaskBranch:        "orc/TASK-001",
		TaskID:            "TASK-001",
		ProtectedBranches: []string{"prod", "staging"},
	}

	hook := generatePrePushHook(cfg.TaskBranch, cfg.TaskID, cfg.ProtectedBranches)

	if !strings.Contains(hook, "prod") {
		t.Error("hook should contain custom protected branch 'prod'")
	}
	if !strings.Contains(hook, "staging") {
		t.Error("hook should contain custom protected branch 'staging'")
	}
}

func TestIsProtectedBranch_ReleaseBranch(t *testing.T) {
	// Specifically test release branch protection
	if !IsProtectedBranch("release", nil) {
		t.Error("release should be protected by default")
	}

	// release/1.0 should NOT be protected (it's not "release" exactly)
	if IsProtectedBranch("release/1.0", nil) {
		t.Error("release/1.0 should not be protected (not exact match)")
	}
}

func TestIsProtectedBranch_EmptyInput(t *testing.T) {
	// Empty branch name should not be protected
	if IsProtectedBranch("", nil) {
		t.Error("empty branch name should not be protected")
	}

	// Empty protected list with empty branch
	if IsProtectedBranch("", []string{}) {
		t.Error("empty branch name should not be protected even with empty list")
	}
}

func TestPrePushHook_BlocksAllProtectedBranches(t *testing.T) {
	hook := generatePrePushHook("orc/TASK-001", "TASK-001", nil)

	// Verify all default protected branches are in the hook
	for _, branch := range DefaultProtectedBranches {
		if !strings.Contains(hook, branch) {
			t.Errorf("pre-push hook should contain default protected branch %q", branch)
		}
	}
}

func TestPreCommitHook_WarningMessage(t *testing.T) {
	hook := generatePreCommitHook("orc/TASK-001", "TASK-001")

	// Verify warning elements
	if !strings.Contains(hook, "WARNING") {
		t.Error("pre-commit hook should contain WARNING for unexpected branch")
	}
	if !strings.Contains(hook, "Expected branch") {
		t.Error("pre-commit hook should mention expected branch")
	}
	if !strings.Contains(hook, "exit 0") {
		t.Error("pre-commit hook should exit 0 (allow commit with warning)")
	}
}
