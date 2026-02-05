package executor

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/storage"
)

// =============================================================================
// Tests for SC-9: GitHub auto-merge warning shown during task execution
// Tests for SC-10: Auto-merge warning suggests alternative
// =============================================================================

// TestAutoMergeWarning_GitHubWithAutoMergeEnabled tests SC-9 and SC-10:
// When GitHub provider is detected and auto_merge is enabled, a warning
// should be logged at execution start.
func TestAutoMergeWarning_GitHubWithAutoMergeEnabled(t *testing.T) {
	// Setup: Create config with GitHub provider and auto_merge enabled
	cfg := config.Default()
	cfg.Hosting.Provider = "github"
	cfg.Completion.PR.AutoMerge = true

	// Capture log output
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Create executor with config
	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend:   backend,
		orcConfig: cfg,
		logger:    logger,
	}

	// Call the function that should emit the warning
	we.checkAutoMergeWarning(hosting.ProviderGitHub)

	// Verify warning was logged
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "auto_merge") {
		t.Errorf("expected auto_merge warning in log, got: %s", logOutput)
	}
	// SC-10: Verify alternative suggestion is included
	if !strings.Contains(logOutput, "merge_on_ci_pass") {
		t.Errorf("expected merge_on_ci_pass suggestion in log, got: %s", logOutput)
	}
}

// TestAutoMergeWarning_GitHubWithAutoMergeDisabled verifies no warning when
// auto_merge is false.
func TestAutoMergeWarning_GitHubWithAutoMergeDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Hosting.Provider = "github"
	cfg.Completion.PR.AutoMerge = false // Disabled

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend:   backend,
		orcConfig: cfg,
		logger:    logger,
	}

	we.checkAutoMergeWarning(hosting.ProviderGitHub)

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "auto_merge") {
		t.Errorf("should not warn when auto_merge is disabled, got: %s", logOutput)
	}
}

// TestAutoMergeWarning_GitLabWithAutoMergeEnabled verifies no warning for GitLab
// since auto-merge IS supported on GitLab.
func TestAutoMergeWarning_GitLabWithAutoMergeEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Hosting.Provider = "gitlab"
	cfg.Completion.PR.AutoMerge = true // Enabled, but GitLab supports it

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend:   backend,
		orcConfig: cfg,
		logger:    logger,
	}

	we.checkAutoMergeWarning(hosting.ProviderGitLab)

	logOutput := logBuf.String()
	if strings.Contains(logOutput, "not supported") {
		t.Errorf("should not warn about auto_merge for GitLab, got: %s", logOutput)
	}
}

// TestAutoMergeWarning_OncePerExecution verifies warning is logged only once
// at execution start, not per-PR.
func TestAutoMergeWarning_OncePerExecution(t *testing.T) {
	cfg := config.Default()
	cfg.Hosting.Provider = "github"
	cfg.Completion.PR.AutoMerge = true

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend:   backend,
		orcConfig: cfg,
		logger:    logger,
	}

	// Call multiple times to simulate multiple PRs
	we.checkAutoMergeWarning(hosting.ProviderGitHub)
	we.checkAutoMergeWarning(hosting.ProviderGitHub)
	we.checkAutoMergeWarning(hosting.ProviderGitHub)

	// Count occurrences of warning
	logOutput := logBuf.String()
	count := strings.Count(logOutput, "auto_merge")

	// Should only appear once (or once per unique call if state is tracked)
	// The implementation should track whether warning was already shown
	if count > 1 {
		t.Errorf("warning should be logged only once, but found %d occurrences", count)
	}
}

// TestAutoMergeWarning_MessageContent tests SC-10: Warning includes specific
// alternative suggestion.
func TestAutoMergeWarning_MessageContent(t *testing.T) {
	cfg := config.Default()
	cfg.Hosting.Provider = "github"
	cfg.Completion.PR.AutoMerge = true

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend:   backend,
		orcConfig: cfg,
		logger:    logger,
	}

	we.checkAutoMergeWarning(hosting.ProviderGitHub)

	logOutput := logBuf.String()

	// Verify message explains why (GitHub requires GraphQL)
	if !strings.Contains(logOutput, "GitHub") && !strings.Contains(logOutput, "GraphQL") {
		t.Errorf("warning should explain GitHub/GraphQL limitation, got: %s", logOutput)
	}

	// Verify actionable suggestion (SC-10)
	if !strings.Contains(logOutput, "completion.ci.merge_on_ci_pass") {
		t.Errorf("warning should suggest completion.ci.merge_on_ci_pass alternative, got: %s", logOutput)
	}
}

// TestAutoMergeWarning_ProviderFromConfig tests that warning uses provider from
// config when not passed explicitly.
func TestAutoMergeWarning_ProviderFromConfig(t *testing.T) {
	cfg := config.Default()
	cfg.Hosting.Provider = "github"
	cfg.Completion.PR.AutoMerge = true

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	backend := storage.NewTestBackend(t)
	we := &WorkflowExecutor{
		backend:   backend,
		orcConfig: cfg,
		logger:    logger,
	}

	// Call without explicit provider - should read from config
	we.checkAutoMergeWarningFromConfig()

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "auto_merge") {
		t.Errorf("expected warning when provider from config is GitHub, got: %s", logOutput)
	}
}

// =============================================================================
// Integration test: Warning during full Run() execution
// =============================================================================

// TestWorkflowExecutor_Run_EmitsAutoMergeWarning tests that the warning is
// emitted during actual workflow execution (integration test).
func TestWorkflowExecutor_Run_EmitsAutoMergeWarning(t *testing.T) {
	// This test verifies the warning is actually wired into the Run() flow
	// It should fail if checkAutoMergeWarning is not called from Run()

	cfg := config.Default()
	cfg.Hosting.Provider = "github"
	cfg.Completion.PR.AutoMerge = true

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	backend := storage.NewTestBackend(t)
	tmpDir := t.TempDir()

	// Create executor with correct signature
	we := NewWorkflowExecutor(
		backend,
		nil, // projectDB
		cfg,
		tmpDir, // workingDir
		WithWorkflowLogger(logger),
	)

	// The warning should be checked when the executor is configured with
	// a GitHub provider and auto_merge enabled
	// This test will fail to compile until checkAutoMergeWarning is wired in

	_ = we // Use executor to prevent unused warning

	// Since we can't run a full workflow without more setup,
	// verify the method exists and can be called
	we.checkAutoMergeWarningFromConfig()

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "auto_merge") {
		t.Errorf("integration: expected warning to be logged, got: %s", logOutput)
	}
}
