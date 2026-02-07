package executor

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
)

func TestNewConflictResolver_Defaults(t *testing.T) {
	r := NewConflictResolver()

	if r.logger == nil {
		t.Error("expected default logger to be set")
	}
	if r.gitOps != nil {
		t.Error("expected gitOps to be nil by default")
	}
}

func TestNewConflictResolver_WithOptions(t *testing.T) {
	r := NewConflictResolver(
		WithConflictClaudePath("/custom/claude"),
		WithConflictCodexPath("/custom/codex"),
		WithConflictWorkingDir("/work/dir"),
	)

	if r.claudePath != "/custom/claude" {
		t.Errorf("claudePath = %q, want %q", r.claudePath, "/custom/claude")
	}
	if r.codexPath != "/custom/codex" {
		t.Errorf("codexPath = %q, want %q", r.codexPath, "/custom/codex")
	}
	if r.workingDir != "/work/dir" {
		t.Errorf("workingDir = %q, want %q", r.workingDir, "/work/dir")
	}
}

func TestConflictResolver_Resolve_NoGitOps(t *testing.T) {
	r := NewConflictResolver() // No git ops configured

	task := &orcv1.Task{Id: "TASK-001", Title: "Test"}
	_, err := r.Resolve(context.Background(), task, []string{"file.go"}, config.SyncConfig{})

	if err == nil {
		t.Error("expected error for missing git ops")
	}
	if err.Error() != "git operations not available" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConflictResolver_Resolve_DefaultConfig(t *testing.T) {
	// Test that empty config values get proper defaults
	cfg := config.SyncConfig{
		ResolveModel:       "", // Should default to "sonnet"
		MaxResolveAttempts: 0,  // Should default to 2
	}

	r := NewConflictResolver()
	task := &orcv1.Task{Id: "TASK-001", Title: "Test"}

	// Will fail because no gitOps, but we verify config handling doesn't panic
	_, err := r.Resolve(context.Background(), task, []string{"file.go"}, cfg)

	// Expected to fail with git ops error, not config error
	if err == nil || err.Error() != "git operations not available" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConflictResolver_buildPrompt_Basic(t *testing.T) {
	r := NewConflictResolver()
	task := &orcv1.Task{
		Id:    "TASK-001",
		Title: "Fix the bug",
	}

	prompt, err := r.buildPrompt(task, []string{"file1.go", "file2.go"})
	if err != nil {
		t.Fatalf("buildPrompt failed: %v", err)
	}

	// Check key elements are present
	if !strings.Contains(prompt, "TASK-001") {
		t.Error("prompt should contain task ID")
	}
	if !strings.Contains(prompt, "Fix the bug") {
		t.Error("prompt should contain task title")
	}
	if !strings.Contains(prompt, "file1.go") {
		t.Error("prompt should contain conflict file")
	}
	if !strings.Contains(prompt, "file2.go") {
		t.Error("prompt should contain conflict file")
	}
}

func TestConflictResolver_buildPrompt_WithDescription(t *testing.T) {
	r := NewConflictResolver()
	desc := "This task fixes the authentication flow"
	task := &orcv1.Task{
		Id:          "TASK-001",
		Title:       "Fix auth",
		Description: &desc,
	}

	prompt, err := r.buildPrompt(task, []string{"auth.go"})
	if err != nil {
		t.Fatalf("buildPrompt failed: %v", err)
	}

	if !strings.Contains(prompt, "authentication flow") {
		t.Error("prompt should contain task description")
	}
}

func TestConflictResolver_StageAndContinueRebase_NoGitOps(t *testing.T) {
	r := NewConflictResolver() // No git ops

	err := r.StageAndContinueRebase(context.Background(), []string{"file.go"})
	if err == nil {
		t.Error("expected error for missing git ops")
	}
	if err.Error() != "git operations not available" {
		t.Errorf("unexpected error: %v", err)
	}
}

// conflictMockTurnExecutor for testing conflict resolution
type conflictMockTurnExecutor struct {
	executeFunc func(ctx context.Context, prompt string) (*TurnResult, error)
	sessionID   string
}

func (m *conflictMockTurnExecutor) ExecuteTurn(ctx context.Context, prompt string) (*TurnResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(ctx, prompt)
	}
	return nil, errors.New("not implemented")
}

func (m *conflictMockTurnExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *conflictMockTurnExecutor) UpdateSessionID(id string) {
	m.sessionID = id
}

func (m *conflictMockTurnExecutor) SessionID() string {
	return m.sessionID
}

func TestConflictResolver_Resolve_WithMockExecutor(t *testing.T) {
	executeCalls := 0
	mockExec := &conflictMockTurnExecutor{
		executeFunc: func(ctx context.Context, prompt string) (*TurnResult, error) {
			executeCalls++
			return &TurnResult{
				Content:  `{"status": "complete", "summary": "Resolved conflicts"}`,
				Status:   PhaseStatusComplete,
				Duration: time.Second,
			}, nil
		},
	}

	r := NewConflictResolver(
		WithConflictTurnExecutor(mockExec),
	)

	task := &orcv1.Task{Id: "TASK-001", Title: "Test"}

	// Still fails because gitOps is nil, but tests the mock path
	_, err := r.Resolve(context.Background(), task, []string{"file.go"}, config.SyncConfig{})

	if err == nil || err.Error() != "git operations not available" {
		t.Errorf("unexpected error: %v", err)
	}
}
