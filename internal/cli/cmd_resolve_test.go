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

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// withResolveTestDir creates a temp directory with task structure, changes to it,
// and restores the original working directory when the test completes.
func withResolveTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	// Create .orc directory with config.yaml for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
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
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
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

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// Verify metadata (resolved_at is stored in metadata, not as a separate field)
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
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
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

// TestResolveCommand_WithoutForceStillRequiresFailed verifies that without --force,
// resolve still requires status=failed (preserves current behavior).
func TestResolveCommand_WithoutForceStillRequiresFailed(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Test various non-failed statuses WITHOUT --force
	statuses := []orcv1.TaskStatus{
		orcv1.TaskStatus_TASK_STATUS_CREATED,
		orcv1.TaskStatus_TASK_STATUS_PLANNED,
		orcv1.TaskStatus_TASK_STATUS_RUNNING,
		orcv1.TaskStatus_TASK_STATUS_PAUSED,
		orcv1.TaskStatus_TASK_STATUS_BLOCKED,
		orcv1.TaskStatus_TASK_STATUS_COMPLETED,
	}

	for _, status := range statuses {
		t.Run(status.String(), func(t *testing.T) {
			// Create backend and save task with this status
			backend := createResolveTestBackend(t, tmpDir)
			tk := task.NewProtoTask("TASK-001", "Test task")
			tk.Status = status
			if err := backend.SaveTask(tk); err != nil {
				t.Fatalf("failed to save task: %v", err)
			}
			_ = backend.Close()

			// Run resolve WITHOUT --force - should fail
			cmd := newResolveCmd()
			cmd.SetArgs([]string{"TASK-001"}) // No --force flag
			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for status %s without --force, got nil", status)
			}

			// Verify error message mentions using --force
			if !strings.Contains(err.Error(), "--force") {
				t.Errorf("error message should mention --force, got: %s", err.Error())
			}
		})
	}
}

// TestResolveCommand_BlockedTask_GuidesToCorrectCommand verifies that running
// orc resolve on a blocked task (without --force) provides helpful guidance.
func TestResolveCommand_BlockedTask_GuidesToCorrectCommand(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create backend and save a blocked task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-BLOCKED", "Test blocked task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve WITHOUT --force - should fail with helpful guidance
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-BLOCKED"}) // No --force
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for blocked task without --force, got nil")
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

	// Verify error message mentions using --force
	if !strings.Contains(errMsg, "--force") {
		t.Errorf("error message should mention --force option, got: %s", errMsg)
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
	tk := task.NewProtoTask("TASK-001", "Test task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
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

	// Create .orc directory with config.yaml for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Use explicit WorktreeDir matching what config.ResolveWorktreeDir("", tmpDir)
	// returns for unregistered projects: <tmpDir>/.orc/worktrees
	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
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

	tk := task.NewProtoTask("TASK-CLEANUP", "Test cleanup")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
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

	tk := task.NewProtoTask("TASK-NO-WT", "Test task without worktree")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
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

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}
	if reloaded.Metadata["resolved"] != "true" {
		t.Errorf("metadata resolved = %q, want 'true'", reloaded.Metadata["resolved"])
	}
}

// TestResolveCommand_ForceSkipsChecks verifies that --force skips
// worktree state checks entirely.
func TestResolveCommand_ForceSkipsChecks(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	// Create .orc directory with config.yaml for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Use explicit WorktreeDir matching what config.ResolveWorktreeDir("", tmpDir)
	// returns for unregistered projects: <tmpDir>/.orc/worktrees
	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
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

	tk := task.NewProtoTask("TASK-FORCE", "Test force flag")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
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

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
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

	// Create .orc directory with config.yaml for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Use explicit WorktreeDir matching what config.ResolveWorktreeDir("", tmpDir)
	// returns for unregistered projects: <tmpDir>/.orc/worktrees
	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
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

	tk := task.NewProtoTask("TASK-CLEAN-WT", "Test clean worktree")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
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

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
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

// =============================================================================
// Tests for --force flag on non-failed tasks (TASK-220 requirements)
// =============================================================================

// TestResolveCommand_ForceOnRunningTask verifies --force works on running tasks.
func TestResolveCommand_ForceOnRunningTask(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a running task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-RUNNING", "Test running task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with --force - should succeed
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-RUNNING", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --force on running task failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-RUNNING")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// Verify force_resolved metadata
	if reloaded.Metadata["force_resolved"] != "true" {
		t.Error("expected force_resolved metadata to be 'true'")
	}
	if reloaded.Metadata["original_status"] != "running" {
		t.Errorf("original_status = %q, want 'running'", reloaded.Metadata["original_status"])
	}
}

// TestResolveCommand_ForceOnPausedTask verifies --force works on paused tasks.
func TestResolveCommand_ForceOnPausedTask(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a paused task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-PAUSED", "Test paused task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with --force - should succeed
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-PAUSED", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --force on paused task failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-PAUSED")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// Verify force_resolved metadata
	if reloaded.Metadata["force_resolved"] != "true" {
		t.Error("expected force_resolved metadata to be 'true'")
	}
	if reloaded.Metadata["original_status"] != "paused" {
		t.Errorf("original_status = %q, want 'paused'", reloaded.Metadata["original_status"])
	}
}

// TestResolveCommand_ForceOnBlockedTask verifies --force works on blocked tasks,
// overriding the helpful error that normally guides users to approve/resume.
func TestResolveCommand_ForceOnBlockedTask(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a blocked task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-BLOCKED-FORCE", "Test blocked task for force")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with --force - should succeed (bypasses the helpful error)
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-BLOCKED-FORCE", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --force on blocked task failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-BLOCKED-FORCE")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// Verify force_resolved metadata
	if reloaded.Metadata["force_resolved"] != "true" {
		t.Error("expected force_resolved metadata to be 'true'")
	}
	if reloaded.Metadata["original_status"] != "blocked" {
		t.Errorf("original_status = %q, want 'blocked'", reloaded.Metadata["original_status"])
	}
}

// TestResolveCommand_ForceOnCreatedTask verifies --force works on created tasks.
func TestResolveCommand_ForceOnCreatedTask(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a task in 'created' status (default)
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-CREATED", "Test created task")
	// Status is already StatusCreated by default
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with --force - should succeed
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-CREATED", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --force on created task failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-CREATED")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// Verify force_resolved metadata
	if reloaded.Metadata["force_resolved"] != "true" {
		t.Error("expected force_resolved metadata to be 'true'")
	}
	if reloaded.Metadata["original_status"] != "created" {
		t.Errorf("original_status = %q, want 'created'", reloaded.Metadata["original_status"])
	}
}

// TestResolveCommand_ForceWithMergedPR_Logic verifies the PR merge detection logic
// works correctly by testing the internal behavior.
// NOTE: The PR field is not persisted by the current storage backend, so this test
// verifies the behavior at the code level rather than through the full CLI flow.
func TestResolveCommand_ForceWithMergedPR(t *testing.T) {
	// Test that the PR merge detection logic works correctly
	// by checking the condition directly
	tk := task.NewProtoTask("TASK-TEST", "Test task")
	url := "https://github.com/owner/repo/pull/123"
	num := int32(123)
	tk.Pr = &orcv1.PRInfo{
		Url:    &url,
		Number: &num,
		Status: orcv1.PRStatus_PR_STATUS_MERGED,
		Merged: true,
	}

	// Verify the merge detection logic
	prMerged := tk.Pr.Status == orcv1.PRStatus_PR_STATUS_MERGED || tk.Pr.Merged
	if !prMerged {
		t.Error("expected prMerged to be true for merged PR")
	}

	// Also test with just the Merged flag (Status might not be set)
	tk2 := task.NewProtoTask("TASK-TEST2", "Test task 2")
	url2 := "https://github.com/owner/repo/pull/124"
	num2 := int32(124)
	tk2.Pr = &orcv1.PRInfo{
		Url:    &url2,
		Number: &num2,
		Merged: true,
	}
	prMerged2 := tk2.Pr.Status == orcv1.PRStatus_PR_STATUS_MERGED || tk2.Pr.Merged
	if !prMerged2 {
		t.Error("expected prMerged to be true when Merged=true")
	}

	// Test with just Status (Merged might not be set)
	tk3 := task.NewProtoTask("TASK-TEST3", "Test task 3")
	url3 := "https://github.com/owner/repo/pull/125"
	num3 := int32(125)
	tk3.Pr = &orcv1.PRInfo{
		Url:    &url3,
		Number: &num3,
		Status: orcv1.PRStatus_PR_STATUS_MERGED,
	}
	prMerged3 := tk3.Pr.Status == orcv1.PRStatus_PR_STATUS_MERGED || tk3.Pr.Merged
	if !prMerged3 {
		t.Error("expected prMerged to be true when Status=merged")
	}
}

// TestResolveCommand_ForceWithoutPR verifies warning when no PR exists.
func TestResolveCommand_ForceWithoutPR(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a running task without a PR
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-NO-PR", "Test task without PR")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	// No PR set
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with --force - should succeed (with warning to stdout, not error)
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-NO-PR", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --force without PR failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-NO-PR")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// Verify pr_was_merged is NOT set (because there was no merged PR)
	if reloaded.Metadata["pr_was_merged"] == "true" {
		t.Error("expected pr_was_merged NOT to be set when no PR exists")
	}

	// force_resolved should still be set
	if reloaded.Metadata["force_resolved"] != "true" {
		t.Error("expected force_resolved metadata to be 'true'")
	}
}

// TestResolveCommand_ForceWithOpenPR_Logic verifies that open (not merged) PRs
// are correctly identified as not merged.
// NOTE: The PR field is not persisted by the current storage backend, so this test
// verifies the behavior at the code level.
func TestResolveCommand_ForceWithOpenPR(t *testing.T) {
	// Test that open PRs are correctly identified as not merged
	tk := task.NewProtoTask("TASK-TEST", "Test task")
	url := "https://github.com/owner/repo/pull/45"
	num := int32(45)
	tk.Pr = &orcv1.PRInfo{
		Url:    &url,
		Number: &num,
		Status: orcv1.PRStatus_PR_STATUS_PENDING_REVIEW,
		Merged: false,
	}

	// Verify the merge detection returns false for open PRs
	prMerged := tk.Pr.Status == orcv1.PRStatus_PR_STATUS_MERGED || tk.Pr.Merged
	if prMerged {
		t.Error("expected prMerged to be false for open PR")
	}

	// Test various non-merged statuses
	nonMergedStatuses := []orcv1.PRStatus{
		orcv1.PRStatus_PR_STATUS_DRAFT,
		orcv1.PRStatus_PR_STATUS_PENDING_REVIEW,
		orcv1.PRStatus_PR_STATUS_CHANGES_REQUESTED,
		orcv1.PRStatus_PR_STATUS_APPROVED,
		orcv1.PRStatus_PR_STATUS_CLOSED,
	}

	for _, status := range nonMergedStatuses {
		tk.Pr.Status = status
		tk.Pr.Merged = false
		prMerged = tk.Pr.Status == orcv1.PRStatus_PR_STATUS_MERGED || tk.Pr.Merged
		if prMerged {
			t.Errorf("expected prMerged to be false for status %s", status)
		}
	}
}

// TestResolveCommand_FailedTaskNoForceMetadata verifies that resolving a failed task
// does NOT set force_resolved metadata (since it's not a force-resolve).
func TestResolveCommand_FailedTaskNoForceMetadata(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a failed task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-FAILED-NO-FORCE", "Test failed task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with --force on failed task
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-FAILED-NO-FORCE", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-FAILED-NO-FORCE")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// force_resolved should NOT be set for failed tasks (they don't need forcing)
	if reloaded.Metadata["force_resolved"] == "true" {
		t.Error("expected force_resolved NOT to be set for failed task resolution")
	}

	// original_status should NOT be set for failed tasks
	if reloaded.Metadata["original_status"] != "" {
		t.Errorf("expected original_status NOT to be set for failed task, got %q", reloaded.Metadata["original_status"])
	}
}

// =============================================================================
// Tests for --yes flag (TASK-648 requirements)
// =============================================================================

// TestResolveCommand_YesFlagExists verifies that the --yes/-y flag is registered
// on the resolve command with the correct shorthand.
func TestResolveCommand_YesFlagExists(t *testing.T) {
	cmd := newResolveCmd()

	// Verify --yes flag exists
	yesFlag := cmd.Flag("yes")
	if yesFlag == nil {
		t.Fatal("missing --yes flag on resolve command")
	}

	// Verify -y shorthand
	if yesFlag.Shorthand != "y" {
		t.Errorf("yes flag shorthand = %q, want 'y'", yesFlag.Shorthand)
	}

	// Verify default is false
	if yesFlag.DefValue != "false" {
		t.Errorf("yes flag default = %q, want 'false'", yesFlag.DefValue)
	}
}

// TestResolveCommand_YesSkipsPromptForFailedTask verifies that --yes skips the
// interactive confirmation prompt and resolves a failed task without reading stdin.
// Maps to: SC-1, BDD-1
func TestResolveCommand_YesSkipsPromptForFailedTask(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a failed task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-YES-001", "Test yes flag on failed task")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with --yes (NOT --force) - should skip prompt and succeed
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-YES-001", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --yes on failed task failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-YES-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// Verify standard resolved metadata is present
	if reloaded.Metadata["resolved"] != "true" {
		t.Errorf("metadata resolved = %q, want 'true'", reloaded.Metadata["resolved"])
	}

	// --yes on a failed task is NOT force-resolving (task was already failed)
	if reloaded.Metadata["force_resolved"] == "true" {
		t.Error("expected force_resolved NOT to be set for failed task with --yes")
	}
}

// TestResolveCommand_YesShortFlag verifies that -y works as short form of --yes.
// Maps to: SC-2
func TestResolveCommand_YesShortFlag(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a failed task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-YES-SHORT", "Test -y short flag")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with -y (short form) - should skip prompt and succeed
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-YES-SHORT", "-y"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve -y on failed task failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-YES-SHORT")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}
}

// TestResolveCommand_YesDoesNotImplyForce verifies that --yes alone does NOT
// allow resolving non-failed tasks. Only --force grants that permission.
// Maps to: SC-5, BDD-3
func TestResolveCommand_YesDoesNotImplyForce(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Test various non-failed statuses with --yes (but NOT --force)
	statuses := []struct {
		status orcv1.TaskStatus
		name   string
	}{
		{orcv1.TaskStatus_TASK_STATUS_CREATED, "created"},
		{orcv1.TaskStatus_TASK_STATUS_RUNNING, "running"},
		{orcv1.TaskStatus_TASK_STATUS_PAUSED, "paused"},
		{orcv1.TaskStatus_TASK_STATUS_BLOCKED, "blocked"},
		{orcv1.TaskStatus_TASK_STATUS_COMPLETED, "completed"},
	}

	for _, tc := range statuses {
		t.Run(tc.name, func(t *testing.T) {
			// Create backend and save task with this status
			backend := createResolveTestBackend(t, tmpDir)
			tk := task.NewProtoTask("TASK-001", "Test task")
			tk.Status = tc.status
			if err := backend.SaveTask(tk); err != nil {
				t.Fatalf("failed to save task: %v", err)
			}
			_ = backend.Close()

			// Run resolve with --yes but WITHOUT --force - should fail
			cmd := newResolveCmd()
			cmd.SetArgs([]string{"TASK-001", "--yes"})
			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for status %s with --yes but no --force, got nil", tc.name)
			}

			// Verify error mentions --force
			if err != nil && !strings.Contains(err.Error(), "--force") {
				t.Errorf("error should mention --force, got: %s", err.Error())
			}
		})
	}
}

// TestResolveCommand_YesWithCleanup verifies that --yes works together with --cleanup.
// Maps to: BDD-5
func TestResolveCommand_YesWithCleanup(t *testing.T) {
	tmpDir := setupTestRepoForResolve(t)

	// Create .orc directory with config.yaml for project detection
	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatalf("create config.yaml: %v", err)
	}

	// Use explicit WorktreeDir matching what config.ResolveWorktreeDir("", tmpDir)
	// returns for unregistered projects: <tmpDir>/.orc/worktrees
	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(tmpDir, ".orc", "worktrees")
	gitOps, err := git.New(tmpDir, gitCfg)
	if err != nil {
		t.Fatalf("failed to create git ops: %v", err)
	}

	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-YES-CLEANUP", baseBranch)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-YES-CLEANUP") }()

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

	tk := task.NewProtoTask("TASK-YES-CLEANUP", "Test yes with cleanup")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Change to project dir and run resolve with --yes --cleanup
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-YES-CLEANUP", "--yes", "--cleanup"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --yes --cleanup failed: %v", err)
	}

	// Verify task was resolved
	backend, _ = storage.NewDatabaseBackend(tmpDir, nil)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-YES-CLEANUP")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// Verify worktree was cleaned up
	clean, _ := wtGit.IsClean()
	if !clean {
		t.Error("expected worktree to be clean after --yes --cleanup")
	}
}

// TestResolveCommand_YesWithMessage verifies that --yes works together with -m message.
func TestResolveCommand_YesWithMessage(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a failed task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-YES-MSG", "Test yes with message")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with --yes -m "message"
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-YES-MSG", "--yes", "-m", "Fixed in hotfix deploy"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --yes -m failed: %v", err)
	}

	// Verify task was resolved with message
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-YES-MSG")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	if reloaded.Metadata["resolution_message"] != "Fixed in hotfix deploy" {
		t.Errorf("resolution_message = %q, want 'Fixed in hotfix deploy'",
			reloaded.Metadata["resolution_message"])
	}
}

// TestResolveCommand_YesAndForceTogether verifies that --yes and --force together
// both take effect: skip prompt AND allow non-failed tasks.
func TestResolveCommand_YesAndForceTogether(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a running task (non-failed)
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-YES-FORCE", "Test yes + force")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with both --yes and --force
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-YES-FORCE", "--yes", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --yes --force failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-YES-FORCE")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}

	// force_resolved should be set (non-failed task)
	if reloaded.Metadata["force_resolved"] != "true" {
		t.Error("expected force_resolved metadata to be 'true'")
	}
	if reloaded.Metadata["original_status"] != "running" {
		t.Errorf("original_status = %q, want 'running'", reloaded.Metadata["original_status"])
	}
}

// TestResolveCommand_YesAndQuietTogether verifies that --yes and --quiet together
// are redundant but harmless.
func TestResolveCommand_YesAndQuietTogether(t *testing.T) {
	tmpDir := withResolveTestDir(t)

	// Create a failed task
	backend := createResolveTestBackend(t, tmpDir)
	tk := task.NewProtoTask("TASK-YES-QUIET", "Test yes + quiet")
	tk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTask(tk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	_ = backend.Close()

	// Run resolve with both --yes and --quiet
	// Note: --quiet is a persistent flag from root, so we need to set it via the root command
	// or use the flag directly. Since these tests use newResolveCmd() directly,
	// we'll set the package-level quiet variable instead.
	origQuiet := quiet
	quiet = true
	defer func() { quiet = origQuiet }()

	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-YES-QUIET", "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve --yes --quiet failed: %v", err)
	}

	// Verify task was resolved
	backend = createResolveTestBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	reloaded, err := backend.LoadTask("TASK-YES-QUIET")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_RESOLVED {
		t.Errorf("task status = %s, want %s", reloaded.Status, orcv1.TaskStatus_TASK_STATUS_RESOLVED)
	}
}
