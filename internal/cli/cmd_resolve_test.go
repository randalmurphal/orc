package cli

// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.

import (
	"os"
	"os/exec"
	"path/filepath"
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
	backend.Close()

	// Run resolve with --force to skip confirmation
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify status
	backend = createResolveTestBackend(t, tmpDir)
	defer backend.Close()

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
	backend.Close()

	// Run resolve with message
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force", "-m", "Fixed manually by updating config"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify message
	backend = createResolveTestBackend(t, tmpDir)
	defer backend.Close()

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
			backend.Close()

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
	backend.Close()

	// Run resolve
	cmd := newResolveCmd()
	cmd.SetArgs([]string{"TASK-001", "--force", "-m", "Test message"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("resolve command failed: %v", err)
	}

	// Reload task and verify all metadata
	backend = createResolveTestBackend(t, tmpDir)
	defer backend.Close()

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
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

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
	defer gitOps.CleanupWorktree("TASK-001")

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
	defer gitOps.CleanupWorktree("TASK-CLEAN")

	// Commit the injected .claude/ directory to make the worktree "clean"
	wtGit := gitOps.InWorktree(worktreePath)
	ctx := wtGit.Context()
	ctx.RunGit("add", ".claude/")
	ctx.RunGit("commit", "-m", "Add Claude Code hooks")

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
	defer gitOps.CleanupWorktree("TASK-002")

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
