// Package api - TDD tests for TASK-721: API server missing GitOps and ClaudePath
// in startTask/resumeTask executor creation.
//
// Tests verify that prepareExecutorDeps returns correct gitOps and claudePath
// derived from the Server's orcConfig, matching CLI behavior.
//
// Coverage:
// - SC-1: TestPrepareExecutorDeps_CreatesGitOps
// - SC-2: TestPrepareExecutorDeps_ReturnsConfiguredClaudePath,
//         TestPrepareExecutorDeps_DefaultsClaudePathWhenEmpty
package api

import (
	"os/exec"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

// TestPrepareExecutorDeps_CreatesGitOps verifies SC-1:
// prepareExecutorDeps returns non-nil gitOps initialized from orcConfig,
// enabling worktree isolation for API-triggered task runs.
func TestPrepareExecutorDeps_CreatesGitOps(t *testing.T) {
	t.Parallel()

	// Create a temp git repo (required by git.New)
	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	cfg := config.Default()
	s := &Server{orcConfig: cfg}

	gitOps, _, _, err := s.prepareExecutorDeps(tmpDir)
	if err != nil {
		t.Fatalf("prepareExecutorDeps: %v", err)
	}

	if gitOps == nil {
		t.Fatal("expected non-nil gitOps, got nil — worktree isolation will be skipped")
	}

	// Verify gitOps is rooted at the correct project dir
	if gitOps.Context().RepoPath() != tmpDir {
		t.Errorf("gitOps repo path = %s, want %s", gitOps.Context().RepoPath(), tmpDir)
	}
}

// TestPrepareExecutorDeps_ReturnsConfiguredClaudePath verifies SC-2:
// When orcConfig.ClaudePath is set, prepareExecutorDeps returns that path,
// ensuring the correct Claude binary is used for phase execution.
func TestPrepareExecutorDeps_ReturnsConfiguredClaudePath(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	cfg := config.Default()
	cfg.ClaudePath = "/custom/bin/claude-stable"
	s := &Server{orcConfig: cfg}

	_, claudePath, _, err := s.prepareExecutorDeps(tmpDir)
	if err != nil {
		t.Fatalf("prepareExecutorDeps: %v", err)
	}

	if claudePath != "/custom/bin/claude-stable" {
		t.Errorf("claudePath = %q, want %q", claudePath, "/custom/bin/claude-stable")
	}
}

// TestPrepareExecutorDeps_DefaultsClaudePathWhenEmpty verifies SC-2 edge case:
// When orcConfig.ClaudePath is empty, defaults to "claude" (bare command name).
func TestPrepareExecutorDeps_DefaultsClaudePathWhenEmpty(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init: %v", err)
	}

	cfg := config.Default()
	cfg.ClaudePath = "" // Explicitly empty
	s := &Server{orcConfig: cfg}

	_, claudePath, _, err := s.prepareExecutorDeps(tmpDir)
	if err != nil {
		t.Fatalf("prepareExecutorDeps: %v", err)
	}

	if claudePath != "claude" {
		t.Errorf("claudePath = %q, want %q", claudePath, "claude")
	}
}
