package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withResolveTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
func withResolveTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .orc directory for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return tmpDir
}

// createResolveTestBackend creates a backend in the given directory.
func createResolveTestBackend(t *testing.T, dir string) storage.Backend {
	t.Helper()
	backend, err := storage.NewDatabaseBackend(dir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}
	return backend
}

func TestResolveCommand_Structure(t *testing.T) {
	cmd := newResolveCmd()

	// Verify command structure
	if cmd.Use != "resolve <task-id>" {
		t.Errorf("command Use = %q, want %q", cmd.Use, "resolve <task-id>")
	}

	// Verify flags exist
	if cmd.Flag("force") == nil {
		t.Error("missing --force flag")
	}
	if cmd.Flag("message") == nil {
		t.Error("missing --message flag")
	}
	if cmd.Flag("cleanup") == nil {
		t.Error("missing --cleanup flag")
	}

	// Verify shorthand flags
	if cmd.Flag("force").Shorthand != "f" {
		t.Errorf("force shorthand = %q, want 'f'", cmd.Flag("force").Shorthand)
	}
	if cmd.Flag("message").Shorthand != "m" {
		t.Errorf("message shorthand = %q, want 'm'", cmd.Flag("message").Shorthand)
	}
}

func TestResolveCommand_RequiresArg(t *testing.T) {
	cmd := newResolveCmd()

	// Should require exactly one argument
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("expected error for zero args")
	}
	if err := cmd.Args(cmd, []string{"TASK-001"}); err != nil {
		t.Errorf("unexpected error for one arg: %v", err)
	}
	if err := cmd.Args(cmd, []string{"TASK-001", "TASK-002"}); err == nil {
		t.Error("expected error for two args")
	}
}

func TestResolveCommand_FailedTask(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create backend and save test data
	backend := createResolveTestBackend(t, tmpDir)

	// Create a failed task
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run resolve with --force to skip confirmation
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify status
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want %s", reloaded.Status, task.StatusCompleted)
	}

	if reloaded.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}

	// Verify metadata
	if reloaded.Metadata["resolved"] != "true" {
		t.Errorf("metadata resolved = %q, want 'true'", reloaded.Metadata["resolved"])
	}

	if reloaded.Metadata["resolved_at"] == "" {
		t.Error("expected resolved_at metadata to be set")
	}

	// Verify resolved_at is a valid timestamp
	_, err = time.Parse(time.RFC3339, reloaded.Metadata["resolved_at"])
	if err != nil {
		t.Errorf("resolved_at is not valid RFC3339: %v", err)
	}
}

func TestResolveCommand_WithMessage(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create backend and save test data
	backend := createResolveTestBackend(t, tmpDir)

	// Create a failed task
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run resolve with message
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force", "-m", "Fixed manually by updating config"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify message
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	expectedMsg := "Fixed manually by updating config"
	if reloaded.Metadata["resolution_message"] != expectedMsg {
		t.Errorf("metadata resolution_message = %q, want %q",
			reloaded.Metadata["resolution_message"], expectedMsg)
	}
}

func TestResolveCommand_OnlyFailedTasks(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Test various non-failed statuses
	statuses := []task.Status{
		task.StatusCreated,
		task.StatusPlanned,
		task.StatusRunning,
		task.StatusPaused,
		task.StatusBlocked,
		task.StatusCompleted,
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			// Create backend and save task with this status
			backend := createResolveTestBackend(t, tmpDir)
			tk := task.New("TASK-001", "Test task")
			tk.Status = status
			if err := backend.SaveTask(tk); err != nil {
				t.Fatalf("failed to save task: %v", err)
			}
			_ = backend.Close()

			// Run resolve - should fail
			cmd := newResolveCmd()
			cmd.SetArgs([]string{"TASK-001", "--force"})
			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for status %s, got nil", status)
			}
		})
	}
}

// TestResolveCommand_BlockedTask_GuidesToCorrectCommand verifies that running
// orc resolve on a blocked task provides helpful guidance to the correct command.
func TestResolveCommand_BlockedTask_GuidesToCorrectCommand(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create backend and save a blocked task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.New("TASK-BLOCKED", "Test blocked task")
	tk.Status = task.StatusBlocked
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve - should fail with helpful guidance
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-BLOCKED", "--force"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for blocked task, got nil")
	}

	errMsg := err.Error()

	// Verify error message contains the task ID
	if !strings.Contains(errMsg, "TASK-BLOCKED") {
		t.Errorf("error message should contain task ID, got: %s", errMsg)
	}

	// Verify error message indicates task is blocked, not failed
	if !strings.Contains(errMsg, "blocked") {
		t.Errorf("error message should mention 'blocked', got: %s", errMsg)
	}

	// Verify error message suggests orc approve with task ID
	if !strings.Contains(errMsg, "orc approve TASK-BLOCKED") {
		t.Errorf("error message should suggest 'orc approve TASK-BLOCKED', got: %s", errMsg)
	}

	// Verify error message suggests orc resume with task ID
	if !strings.Contains(errMsg, "orc resume TASK-BLOCKED") {
		t.Errorf("error message should suggest 'orc resume TASK-BLOCKED', got: %s", errMsg)
	}

	// Verify error message explains what resolve is for
	if !strings.Contains(errMsg, "marking failed tasks") {
		t.Errorf("error message should explain resolve is for failed tasks, got: %s", errMsg)
	}
}

func TestResolveCommand_TaskNotFound(t *testing.T) {
	withResolveTestDir(t)

	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-999", "--force"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestResolveCommand_PreservesExistingMetadata(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create backend and save test data
	backend := createResolveTestBackend(t, tmpDir)

	// Create a failed task with existing metadata
	tk := task.New("TASK-001", "Test task")
	tk.Status = task.StatusFailed
	tk.Metadata = map[string]string{
		"existing_key": "existing_value",
		"another_key":  "another_value",
	}
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Close backend before running command
	_ = backend.Close()

	// Run resolve
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force", "-m", "Test message"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify all metadata
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	// Original metadata should be preserved
	if reloaded.Metadata["existing_key"] != "existing_value" {
		t.Errorf("existing_key = %q, want 'existing_value'", reloaded.Metadata["existing_key"])
	}
	if reloaded.Metadata["another_key"] != "another_value" {
		t.Errorf("another_key = %q, want 'another_value'", reloaded.Metadata["another_key"])
	}

	// New metadata should be added
	if reloaded.Metadata["resolved"] != "true" {
		t.Errorf("resolved = %q, want 'true'", reloaded.Metadata["resolved"])
	}
	if reloaded.Metadata["resolution_message"] != "Test message" {
		t.Errorf("resolution_message = %q, want 'Test message'", reloaded.Metadata["resolution_message"])
	}
}

func TestCheckWorktreeStatus_NoGitOps(t *testing.T) {
	// When gitOps is nil, should return empty status without error
	status, err := checkWorktreeStatus("TASK-001", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.exists {
		t.Error("expected exists to be false with nil gitOps")
	}
}

func TestWorktreeStatus_Struct(t *testing.T) {
	// Test the struct can hold all expected values
	status := worktreeStatus{
		exists:         true,
		path:           "/tmp/worktree/orc-TASK-001",
		isDirty:        true,
		hasConflicts:   true,
		conflictFiles:  []string{"file1.go", "file2.go"},
		rebaseInProg:   false,
		mergeInProg:    true,
		uncommittedMsg: "3 uncommitted file(s)",
	}

	if !status.exists {
		t.Error("expected exists to be true")
	}
	if status.path != "/tmp/worktree/orc-TASK-001" {
		t.Errorf("path = %q, want '/tmp/worktree/orc-TASK-001'", status.path)
	}
	if !status.isDirty {
		t.Error("expected isDirty to be true")
	}
	if !status.hasConflicts {
		t.Error("expected hasConflicts to be true")
	}
	if len(status.conflictFiles) != 2 {
		t.Errorf("conflictFiles length = %d, want 2", len(status.conflictFiles))
	}
	if status.rebaseInProg {
		t.Error("expected rebaseInProg to be false")
	}
	if !status.mergeInProg {
		t.Error("expected mergeInProg to be true")
	}
	if status.uncommittedMsg != "3 uncommitted file(s)" {
		t.Errorf("uncommittedMsg = %q, want '3 uncommitted file(s)'", status.uncommittedMsg)
	}
}

func TestWorktreeStatus_HasWorktreeIssues(t *testing.T) {
	tests := []struct {
		name     string
		status   *worktreeStatus
		wantTrue bool
	}{
		{
			name:     "nil status",
			status:   nil,
			wantTrue: false,
		},
		{
			name:     "worktree does not exist",
			status:   &worktreeStatus{exists: false},
			wantTrue: false,
		},
		{
			name:     "clean worktree",
			status:   &worktreeStatus{exists: true, isDirty: false, hasConflicts: false, rebaseInProg: false, mergeInProg: false},
			wantTrue: false,
		},
		{
			name:     "dirty worktree",
			status:   &worktreeStatus{exists: true, isDirty: true},
			wantTrue: true,
		},
		{
			name:     "worktree with conflicts",
			status:   &worktreeStatus{exists: true, hasConflicts: true, conflictFiles: []string{"file.go"}},
			wantTrue: true,
		},
		{
			name:     "worktree with rebase in progress",
			status:   &worktreeStatus{exists: true, rebaseInProg: true},
			wantTrue: true,
		},
		{
			name:     "worktree with merge in progress",
			status:   &worktreeStatus{exists: true, mergeInProg: true},
			wantTrue: true,
		},
		{
			name: "worktree with multiple issues",
			status: &worktreeStatus{
				exists:       true,
				isDirty:      true,
				hasConflicts: true,
				rebaseInProg: true,
				mergeInProg:  false,
			},
			wantTrue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.hasWorktreeIssues()
			if got != tt.wantTrue {
				t.Errorf("hasWorktreeIssues() = %v, want %v", got, tt.wantTrue)
			}
		})
	}
}

// setupTestRepoForResolve creates a git repository for testing checkWorktreeStatus.
func setupTestRepoForResolve(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	return tmpDir
}

func TestCheckWorktreeStatus_WorktreeWithInjectedHooks(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Get base branch
	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree - this injects Claude Code hooks which creates .claude/ directory
	worktreePath, err := gitOps.CreateWorktree("TASK-001", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-001") }()

	// Check worktree status
	status, err := checkWorktreeStatus("TASK-001", gitOps)
	if err != nil {
		t.Fatalf("checkWorktreeStatus failed: %v", err)
	}

	if !status.exists {
		t.Error("expected exists to be true")
	}
	if status.path != worktreePath {
		t.Errorf("path = %q, want %q", status.path, worktreePath)
	}
	// Note: CreateWorktree injects Claude Code hooks which creates .claude/ directory.
	// The hooks call EnsureClaudeSettingsUntracked which marks .claude/settings.json
	// as assume-unchanged and adds it to git exclude. However, the .claude/hooks/
	// directory is newly created and shows as untracked.
	// The isDirty flag will be true due to these injected files.
	if status.isDirty && status.uncommittedMsg == "" {
		t.Error("if isDirty, uncommittedMsg should be set")
	}
	if status.hasConflicts {
		t.Error("expected hasConflicts to be false")
	}
	if status.rebaseInProg {
		t.Error("expected rebaseInProg to be false")
	}
	if status.mergeInProg {
		t.Error("expected mergeInProg to be false")
	}
	// hasWorktreeIssues() returns true because of the injected Claude Code hooks.
	// This is expected - the purpose of this test is to verify status detection works.
}

func TestCheckWorktreeStatus_CleanWorktree(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Get base branch
	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree with hook injection
	worktreePath, err := gitOps.CreateWorktree("TASK-CLEAN", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-CLEAN") }()

	// Commit the injected .claude/ directory to make the worktree "clean"
	wtGit := gitOps.InWorktree(worktreePath)
	ctx := wtGit.Context()
	_, _ = ctx.RunGit("add", ".claude/")
	_, _ = ctx.RunGit("commit", "-m", "Add Claude Code hooks")

	// Check worktree status - should be clean now
	status, err := checkWorktreeStatus("TASK-CLEAN", gitOps)
	if err != nil {
		t.Fatalf("checkWorktreeStatus failed: %v", err)
	}

	if !status.exists {
		t.Error("expected exists to be true")
	}
	if status.isDirty {
		t.Errorf("expected isDirty to be false after committing, got true (uncommittedMsg: %s)", status.uncommittedMsg)
	}
	if status.hasConflicts {
		t.Error("expected hasConflicts to be false")
	}
	if status.rebaseInProg {
		t.Error("expected rebaseInProg to be false")
	}
	if status.mergeInProg {
		t.Error("expected mergeInProg to be false")
	}
	if status.hasWorktreeIssues() {
		t.Error("expected hasWorktreeIssues to return false for clean worktree")
	}
}

func TestCheckWorktreeStatus_DirtyWorktree(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Get base branch
	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-002", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-002") }()

	// Create uncommitted changes
	dirtyFile := filepath.Join(worktreePath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty content"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Check dirty worktree status
	status, err := checkWorktreeStatus("TASK-002", gitOps)
	if err != nil {
		t.Fatalf("checkWorktreeStatus failed: %v", err)
	}

	if !status.exists {
		t.Error("expected exists to be true")
	}
	if !status.isDirty {
		t.Error("expected isDirty to be true for dirty worktree")
	}
	if status.uncommittedMsg == "" {
		t.Error("expected uncommittedMsg to be set")
	}
	if !status.hasWorktreeIssues() {
		t.Error("expected hasWorktreeIssues to return true for dirty worktree")
	}
}

func TestCheckWorktreeStatus_NonExistentWorktree(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	// Check non-existent worktree
	status, err := checkWorktreeStatus("TASK-NONEXISTENT", gitOps)
	if err != nil {
		t.Fatalf("checkWorktreeStatus failed: %v", err)
	}

	if status.exists {
		t.Error("expected exists to be false for non-existent worktree")
	}
	if status.hasWorktreeIssues() {
		t.Error("expected hasWorktreeIssues to return false for non-existent worktree")
	}
}

// =============================================================================
// Tests required by spec: Testing Requirements
// =============================================================================

// TestResolveCommand_DetectsDirtyWorktree verifies that orc resolve detects
// uncommitted changes in a task's worktree.
func TestResolveCommand_DetectsDirtyWorktree(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-DIRTY", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-DIRTY") }()

	// Commit the injected .claude/ directory to start clean
	wtGit := gitOps.InWorktree(worktreePath)
	ctx := wtGit.Context()
	_, _ = ctx.RunGit("add", ".claude/")
	_, _ = ctx.RunGit("commit", "-m", "Add Claude Code hooks")

	// Create uncommitted changes
	dirtyFile := filepath.Join(worktreePath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty content"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Check status - should detect dirty worktree
	status, err := checkWorktreeStatus("TASK-DIRTY", gitOps)
	if err != nil {
		t.Fatalf("checkWorktreeStatus failed: %v", err)
	}

	if !status.exists {
		t.Error("expected exists to be true")
	}
	if !status.isDirty {
		t.Error("expected isDirty to be true - worktree has uncommitted changes")
	}
	if status.uncommittedMsg == "" {
		t.Error("expected uncommittedMsg to describe the dirty state")
	}
	if !status.hasWorktreeIssues() {
		t.Error("expected hasWorktreeIssues() to return true for dirty worktree")
	}
}

// TestResolveCommand_DetectsRebaseInProgress verifies that orc resolve detects
// an in-progress rebase in a task's worktree.
func TestResolveCommand_DetectsRebaseInProgress(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-REBASE", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-REBASE") }()

	// Commit the injected .claude/ directory first
	wtGit := gitOps.InWorktree(worktreePath)
	ctx := wtGit.Context()
	_, _ = ctx.RunGit("add", ".claude/")
	_, _ = ctx.RunGit("commit", "-m", "Add Claude Code hooks")

	// Modify README on task branch
	readmeFile := filepath.Join(worktreePath, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Task branch changes\n"), 0644); err != nil {
		t.Fatalf("failed to modify README: %v", err)
	}
	_, _ = ctx.RunGit("add", "README.md")
	_, _ = ctx.RunGit("commit", "-m", "modify readme on task")

	// Switch back to base branch and make conflicting change
	cmd := exec.Command("git", "checkout", baseBranch)
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to checkout base branch: %v", err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Base branch changes\n"), 0644); err != nil {
		t.Fatalf("failed to modify README on base: %v", err)
	}
	cmd = exec.Command("git", "add", "README.md")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "modify readme on base")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit on base: %v", err)
	}

	// Start a rebase that will conflict
	_, _ = ctx.RunGit("rebase", baseBranch)
	// This should leave us in a conflicted rebase state

	// Check status - should detect rebase in progress
	status, err := checkWorktreeStatus("TASK-REBASE", gitOps)
	if err != nil {
		t.Fatalf("checkWorktreeStatus failed: %v", err)
	}

	if !status.exists {
		t.Error("expected exists to be true")
	}
	if !status.rebaseInProg {
		t.Error("expected rebaseInProg to be true - worktree has rebase in progress")
	}
	if !status.hasWorktreeIssues() {
		t.Error("expected hasWorktreeIssues() to return true for rebase in progress")
	}

	// Cleanup: abort the rebase
	_, _ = ctx.RunGit("rebase", "--abort")
}

// TestResolveCommand_DetectsMergeInProgress verifies that orc resolve detects
// an in-progress merge in a task's worktree.
func TestResolveCommand_DetectsMergeInProgress(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-MERGE", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-MERGE") }()

	// Commit the injected .claude/ directory first
	wtGit := gitOps.InWorktree(worktreePath)
	ctx := wtGit.Context()
	_, _ = ctx.RunGit("add", ".claude/")
	_, _ = ctx.RunGit("commit", "-m", "Add Claude Code hooks")

	// Create a branch from task branch to merge
	_, _ = ctx.RunGit("branch", "feature-to-merge")

	// Modify README on task branch
	readmeFile := filepath.Join(worktreePath, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Task branch changes\n"), 0644); err != nil {
		t.Fatalf("failed to modify README: %v", err)
	}
	_, _ = ctx.RunGit("add", "README.md")
	_, _ = ctx.RunGit("commit", "-m", "modify readme on task")

	// Switch to feature branch and make conflicting change
	_, _ = ctx.RunGit("checkout", "feature-to-merge")

	if err := os.WriteFile(readmeFile, []byte("# Feature branch changes\n"), 0644); err != nil {
		t.Fatalf("failed to modify README on feature: %v", err)
	}
	_, _ = ctx.RunGit("add", "README.md")
	_, _ = ctx.RunGit("commit", "-m", "modify readme on feature")

	// Switch back to task branch
	_, _ = ctx.RunGit("checkout", "orc/TASK-MERGE")

	// Start a merge that will conflict
	_, _ = ctx.RunGit("merge", "feature-to-merge")
	// This should leave us in a conflicted merge state

	// Check status - should detect merge in progress
	status, err := checkWorktreeStatus("TASK-MERGE", gitOps)
	if err != nil {
		t.Fatalf("checkWorktreeStatus failed: %v", err)
	}

	if !status.exists {
		t.Error("expected exists to be true")
	}
	if !status.mergeInProg {
		t.Error("expected mergeInProg to be true - worktree has merge in progress")
	}
	if !status.hasWorktreeIssues() {
		t.Error("expected hasWorktreeIssues() to return true for merge in progress")
	}

	// Cleanup: abort the merge
	_, _ = ctx.RunGit("merge", "--abort")
}

// TestResolveCommand_CleanupFlag verifies that --cleanup aborts in-progress
// operations and discards uncommitted changes.
func TestResolveCommand_CleanupFlag(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	// Create .orc directory for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-CLEANUP", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-CLEANUP") }()

	// Commit the injected .claude/ directory first
	wtGit := gitOps.InWorktree(worktreePath)
	ctx := wtGit.Context()
	_, _ = ctx.RunGit("add", ".claude/")
	_, _ = ctx.RunGit("commit", "-m", "Add Claude Code hooks")

	// Create uncommitted changes
	dirtyFile := filepath.Join(worktreePath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty content"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Verify worktree is dirty
	status, _ := checkWorktreeStatus("TASK-CLEANUP", gitOps)
	if !status.isDirty {
		t.Fatal("expected worktree to be dirty before cleanup")
	}

	// Create a failed task in the backend
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}

	tk := task.New("TASK-CLEANUP", "Test cleanup")
	tk.Status = task.StatusFailed
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Change to project dir and run resolve with --cleanup
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-CLEANUP", "--force", "--cleanup"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Verify worktree is now clean
	clean, _ := wtGit.IsClean()
	if !clean {
		t.Error("expected worktree to be clean after --cleanup")
	}

	// Verify dirty file was removed
	if _, err := os.Stat(dirtyFile); !os.IsNotExist(err) {
		t.Error("expected dirty.txt to be removed after --cleanup")
	}

	// Verify worktree still exists (--cleanup should NOT remove worktree)
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("worktree should still exist after --cleanup (--cleanup discards changes, doesn't remove worktree)")
	}
}

// TestResolveCommand_NoWorktree verifies that orc resolve works when
// the task doesn't have an associated worktree.
func TestResolveCommand_NoWorktree(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create backend and save a failed task (no worktree)
	backend := createResolveTestBackend(t, tmpDir)

	tk := task.New("TASK-NO-WT", "Test task without worktree")
	tk.Status = task.StatusFailed
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve - should succeed without worktree
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-NO-WT", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-NO-WT")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want %s", reloaded.Status, task.StatusCompleted)
	}
	if reloaded.Metadata["resolved"] != "true" {
		t.Errorf("metadata resolved = %q, want 'true'", reloaded.Metadata["resolved"])
	}
}

// TestResolveCommand_ForceSkipsChecks verifies that --force skips
// worktree state checks entirely.
func TestResolveCommand_ForceSkipsChecks(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	// Create .orc directory for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-FORCE", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-FORCE") }()

	// Commit the injected .claude/ directory first
	wtGit := gitOps.InWorktree(worktreePath)
	ctx := wtGit.Context()
	_, _ = ctx.RunGit("add", ".claude/")
	_, _ = ctx.RunGit("commit", "-m", "Add Claude Code hooks")

	// Create uncommitted changes
	dirtyFile := filepath.Join(worktreePath, "dirty.txt")
	if err := os.WriteFile(dirtyFile, []byte("dirty content"), 0644); err != nil {
		t.Fatalf("failed to create dirty file: %v", err)
	}

	// Create a failed task
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}

	tk := task.New("TASK-FORCE", "Test force flag")
	tk.Status = task.StatusFailed
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Change to project dir and run resolve with --force
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-FORCE", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Verify task was resolved (even though worktree is dirty)
	backend, _ = storage.NewDatabaseBackend(tmpDir, nil)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-FORCE")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want %s", reloaded.Status, task.StatusCompleted)
	}

	// Verify dirty file still exists (--force doesn't clean up)
	if _, err := os.Stat(dirtyFile); os.IsNotExist(err) {
		t.Error("expected dirty.txt to still exist with --force (no cleanup)")
	}

	// Verify metadata indicates worktree was dirty
	if reloaded.Metadata["worktree_was_dirty"] != "true" {
		t.Error("expected worktree_was_dirty metadata to be set")
	}
}

// TestResolveCommand_CleanWorktree verifies that orc resolve doesn't
// display warnings when the worktree is clean.
func TestResolveCommand_CleanWorktree(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	// Create .orc directory for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-CLEAN-WT", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-CLEAN-WT") }()

	// Commit the injected .claude/ directory to make worktree clean
	wtGit := gitOps.InWorktree(worktreePath)
	ctx := wtGit.Context()
	_, _ = ctx.RunGit("add", ".claude/")
	_, _ = ctx.RunGit("commit", "-m", "Add Claude Code hooks")

	// Verify worktree is clean
	status, _ := checkWorktreeStatus("TASK-CLEAN-WT", gitOps)
	if status.hasWorktreeIssues() {
		t.Fatal("expected worktree to be clean before test")
	}

	// Create a failed task
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("create backend: %v", err)
	}

	tk := task.New("TASK-CLEAN-WT", "Test clean worktree")
	tk.Status = task.StatusFailed
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Change to project dir and run resolve
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-CLEAN-WT", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Verify task was resolved
	backend, _ = storage.NewDatabaseBackend(tmpDir, nil)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-CLEAN-WT")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want %s", reloaded.Status, task.StatusCompleted)
	}

	// Verify no worktree issues recorded in metadata
	if reloaded.Metadata["worktree_was_dirty"] == "true" {
		t.Error("expected worktree_was_dirty to NOT be set for clean worktree")
	}
	if reloaded.Metadata["worktree_had_conflicts"] == "true" {
		t.Error("expected worktree_had_conflicts to NOT be set for clean worktree")
	}
	if reloaded.Metadata["worktree_had_incomplete_operation"] == "true" {
		t.Error("expected worktree_had_incomplete_operation to NOT be set for clean worktree")
	}
}
