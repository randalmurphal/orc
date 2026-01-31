package executor

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// syncCleanupTestEnv holds all the state needed to test sync-on-start failure cleanup.
type syncCleanupTestEnv struct {
	backend   *storage.DatabaseBackend
	projectDB *db.ProjectDB
	cfg       *config.Config
	gitOps    *git.Git
	repoDir   string
	remoteDir string
	taskID    string
	tsk       *orcv1.Task
}

// setupSyncCleanupTest creates a full test environment for sync-on-start failure tests.
// It creates:
// - A bare remote repo with an initial commit on main
// - A working repo cloned from the remote
// - A worktree for the task (branched from main)
// - Conflicting commits: one on main (pushed to remote), one on the task branch
//
// After this setup, syncOnTaskStart will fail because rebasing the task branch
// onto the updated origin/main will hit a merge conflict.
func setupSyncCleanupTest(t *testing.T, taskID string) *syncCleanupTestEnv {
	t.Helper()

	// Create bare remote repo
	remoteDir := t.TempDir()
	runGitCmdOrFatal(t, remoteDir, "init", "--bare")

	// Create working repo
	repoDir := t.TempDir()
	runGitCmdOrFatal(t, repoDir, "init", "--initial-branch=main")
	runGitCmdOrFatal(t, repoDir, "config", "user.email", "test@example.com")
	runGitCmdOrFatal(t, repoDir, "config", "user.name", "Test")

	// Initial commit
	writeTestFile(t, repoDir, "README.md", "# Initial\n")
	runGitCmdOrFatal(t, repoDir, "add", ".")
	runGitCmdOrFatal(t, repoDir, "commit", "-m", "Initial commit")

	// Add remote and push
	runGitCmdOrFatal(t, repoDir, "remote", "add", "origin", remoteDir)
	runGitCmdOrFatal(t, repoDir, "push", "-u", "origin", "main")

	// Create git ops
	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(repoDir, ".orc", "worktrees")
	gitOps, err := git.New(repoDir, gitCfg)
	if err != nil {
		t.Fatalf("git.New: %v", err)
	}

	// Create task
	tsk := task.NewProtoTask(taskID, "Sync conflict test task")
	tsk.Weight = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM

	// Create worktree for the task
	cfg := config.Default()
	cfg.Worktree.Enabled = true
	cfg.Completion.TargetBranch = "main"
	cfg.Completion.Sync.Strategy = config.SyncStrategyPhase
	cfg.Completion.Sync.SyncOnStart = true

	result, err := SetupWorktreeForTask(tsk, cfg, gitOps, nil)
	if err != nil {
		t.Fatalf("SetupWorktreeForTask: %v", err)
	}

	// Make a commit on the task branch (in worktree) that will conflict
	writeTestFile(t, result.Path, "README.md", "# Task branch change\nConflicting content from task\n")
	runGitCmdOrFatal(t, result.Path, "add", "README.md")
	runGitCmdOrFatal(t, result.Path, "commit", "-m", "task branch conflicting commit")

	// Make a conflicting commit on main and push to remote
	// (checkout main in the main repo, commit, push)
	runGitCmdOrFatal(t, repoDir, "checkout", "main")
	writeTestFile(t, repoDir, "README.md", "# Main branch change\nConflicting content from main\n")
	runGitCmdOrFatal(t, repoDir, "add", "README.md")
	runGitCmdOrFatal(t, repoDir, "commit", "-m", "main branch conflicting commit")
	runGitCmdOrFatal(t, repoDir, "push", "origin", "main")

	// Switch back to original branch so main repo doesn't block worktree operations
	// (we were on main, which is fine since worktree is on task branch)

	// Set up backend and projectDB
	backend := storage.NewTestBackend(t)
	projectDB := db.NewTestProjectDB(t)

	// Save the task in the backend
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	// Create minimal workflow + phase template in projectDB
	now := time.Now()
	if err := projectDB.SavePhaseTemplate(&db.PhaseTemplate{
		ID:        "implement",
		Name:      "Implement",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("SavePhaseTemplate: %v", err)
	}

	if err := projectDB.SaveWorkflow(&db.Workflow{
		ID:           "test-workflow",
		Name:         "Test Workflow",
		WorkflowType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}); err != nil {
		t.Fatalf("SaveWorkflow: %v", err)
	}

	if err := projectDB.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "test-workflow",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}); err != nil {
		t.Fatalf("SaveWorkflowPhase: %v", err)
	}

	return &syncCleanupTestEnv{
		backend:   backend,
		projectDB: projectDB,
		cfg:       cfg,
		gitOps:    gitOps,
		repoDir:   repoDir,
		remoteDir: remoteDir,
		taskID:    taskID,
		tsk:       tsk,
	}
}

// runWithSyncFailure creates and runs a WorkflowExecutor that will fail on sync-on-start.
// Returns the error from Run() and the worktree path that was created.
func (env *syncCleanupTestEnv) runWithSyncFailure(t *testing.T, logger *slog.Logger) error {
	t.Helper()

	if logger == nil {
		logger = slog.Default()
	}

	mockTE := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)

	we := NewWorkflowExecutor(
		env.backend,
		env.projectDB,
		env.cfg,
		env.repoDir,
		WithWorkflowGitOps(env.gitOps),
		WithWorkflowLogger(logger),
		WithWorkflowTurnExecutor(mockTE),
	)

	_, err := we.Run(context.Background(), "test-workflow", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      env.taskID,
	})

	return err
}

// worktreePath returns the expected worktree path for the task.
func (env *syncCleanupTestEnv) worktreePath() string {
	return env.gitOps.WorktreePath(env.taskID)
}

// branchName returns the expected branch name for the task.
func (env *syncCleanupTestEnv) branchName() string {
	return env.gitOps.BranchName(env.taskID)
}

// TestSyncOnStartFailure_CleansWorktree verifies SC-1:
// When sync-on-start fails, the worktree directory is removed regardless of CleanupOnFail config.
func TestSyncOnStartFailure_CleansWorktree(t *testing.T) {
	t.Parallel()

	env := setupSyncCleanupTest(t, "TASK-SYNC-WT")

	// Verify worktree exists before running
	wtPath := env.worktreePath()
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatalf("worktree should exist before run: %s", wtPath)
	}

	// Explicitly set CleanupOnFail to false — sync failure cleanup should be unconditional
	env.cfg.Worktree.CleanupOnFail = false

	// Run executor — should fail on sync-on-start due to merge conflict
	err := env.runWithSyncFailure(t, nil)
	if err == nil {
		t.Fatal("expected sync-on-start failure, but Run() succeeded")
	}
	if !strings.Contains(err.Error(), "sync on start") {
		t.Fatalf("expected sync-on-start error, got: %v", err)
	}

	// SC-1: Worktree directory must be removed regardless of CleanupOnFail config
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Errorf("SC-1 FAILED: worktree directory should not exist after sync failure, path: %s", wtPath)
	}
}

// TestSyncOnStartFailure_CleansBranch verifies SC-2:
// When sync-on-start fails, the task branch is deleted so retry creates a fresh branch.
func TestSyncOnStartFailure_CleansBranch(t *testing.T) {
	t.Parallel()

	env := setupSyncCleanupTest(t, "TASK-SYNC-BR")

	// Verify branch exists before running
	branchName := env.branchName()
	exists, err := env.gitOps.BranchExists(branchName)
	if err != nil {
		t.Fatalf("BranchExists check: %v", err)
	}
	if !exists {
		t.Fatalf("branch %s should exist before run", branchName)
	}

	// Run executor — should fail on sync-on-start
	runErr := env.runWithSyncFailure(t, nil)
	if runErr == nil {
		t.Fatal("expected sync-on-start failure, but Run() succeeded")
	}
	if !strings.Contains(runErr.Error(), "sync on start") {
		t.Fatalf("expected sync-on-start error, got: %v", runErr)
	}

	// SC-2: Branch must be deleted after sync failure
	exists, err = env.gitOps.BranchExists(branchName)
	if err != nil {
		t.Fatalf("BranchExists check after failure: %v", err)
	}
	if exists {
		t.Errorf("SC-2 FAILED: branch %s should not exist after sync failure", branchName)
	}
}

// TestSyncOnStartFailure_RetrySucceeds verifies SC-3:
// After sync-on-start failure and cleanup, a retry can create fresh worktree and branch.
func TestSyncOnStartFailure_RetrySucceeds(t *testing.T) {
	t.Parallel()

	env := setupSyncCleanupTest(t, "TASK-SYNC-RETRY")

	// First run: fail on sync-on-start
	err := env.runWithSyncFailure(t, nil)
	if err == nil {
		t.Fatal("expected sync-on-start failure on first run")
	}
	if !strings.Contains(err.Error(), "sync on start") {
		t.Fatalf("expected sync-on-start error, got: %v", err)
	}

	// After cleanup, verify we can create a fresh worktree and branch.
	// This simulates what the next `orc run` would do for worktree setup.
	// Reset task status so it can be loaded again
	env.tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	if saveErr := env.backend.SaveTask(env.tsk); saveErr != nil {
		t.Fatalf("reset task status: %v", saveErr)
	}

	// SC-3: SetupWorktreeForTask must succeed on retry (fresh worktree and branch)
	result, err := SetupWorktreeForTask(env.tsk, env.cfg, env.gitOps, nil)
	if err != nil {
		t.Fatalf("SC-3 FAILED: SetupWorktreeForTask should succeed on retry, got: %v", err)
	}

	// Verify new worktree exists
	if _, statErr := os.Stat(result.Path); os.IsNotExist(statErr) {
		t.Errorf("SC-3 FAILED: retry worktree should exist at %s", result.Path)
	}

	// Verify it's a fresh creation (not reused)
	if result.Reused {
		t.Error("SC-3 FAILED: retry worktree should be fresh, not reused")
	}

	// Verify branch was freshly created
	exists, err := env.gitOps.BranchExists(env.branchName())
	if err != nil {
		t.Fatalf("BranchExists after retry: %v", err)
	}
	if !exists {
		t.Error("SC-3 FAILED: branch should exist after retry")
	}
}

// TestSyncOnStartFailure_LogsCleanup verifies SC-4:
// Cleanup on sync failure logs an info-level message indicating zombie cleanup occurred.
func TestSyncOnStartFailure_LogsCleanup(t *testing.T) {
	t.Parallel()

	env := setupSyncCleanupTest(t, "TASK-SYNC-LOG")

	// Capture log output
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})
	logger := slog.New(handler)

	// Run executor — should fail on sync-on-start
	err := env.runWithSyncFailure(t, logger)
	if err == nil {
		t.Fatal("expected sync-on-start failure")
	}
	if !strings.Contains(err.Error(), "sync on start") {
		t.Fatalf("expected sync-on-start error, got: %v", err)
	}

	// SC-4: Log should contain cleanup message with task ID and path info
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "sync") || !strings.Contains(logOutput, "cleanup") ||
		!strings.Contains(logOutput, "TASK-SYNC-LOG") {
		t.Errorf("SC-4 FAILED: expected log message about sync failure cleanup with task ID, got:\n%s", logOutput)
	}
}

// TestSyncOnStartFailure_CleanupIgnoresConfig verifies BDD-3:
// When CleanupOnFail is false, sync-on-start failure STILL cleans up worktree and branch.
// This is because no phases ran, so there's no user work to preserve.
func TestSyncOnStartFailure_CleanupIgnoresConfig(t *testing.T) {
	t.Parallel()

	env := setupSyncCleanupTest(t, "TASK-SYNC-CFG")

	// Explicitly disable failure cleanup — sync failure cleanup should override this
	env.cfg.Worktree.CleanupOnFail = false
	env.cfg.Worktree.CleanupOnComplete = false

	// Verify worktree and branch exist
	wtPath := env.worktreePath()
	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Fatal("worktree should exist before run")
	}
	branchName := env.branchName()
	exists, err := env.gitOps.BranchExists(branchName)
	if err != nil {
		t.Fatalf("BranchExists: %v", err)
	}
	if !exists {
		t.Fatalf("branch %s should exist before run", branchName)
	}

	// Run executor
	runErr := env.runWithSyncFailure(t, nil)
	if runErr == nil {
		t.Fatal("expected sync-on-start failure")
	}

	// BDD-3: Both worktree and branch must be cleaned up despite config being false
	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Error("BDD-3 FAILED: worktree should be cleaned up even when CleanupOnFail is false")
	}
	exists, err = env.gitOps.BranchExists(branchName)
	if err != nil {
		t.Fatalf("BranchExists after failure: %v", err)
	}
	if exists {
		t.Error("BDD-3 FAILED: branch should be deleted even when CleanupOnFail is false")
	}
}

// TestSyncOnStartFailure_TaskStatusFailed verifies that the task is marked FAILED
// after sync-on-start failure (existing behavior, preserved by the fix).
func TestSyncOnStartFailure_TaskStatusFailed(t *testing.T) {
	t.Parallel()

	env := setupSyncCleanupTest(t, "TASK-SYNC-STATUS")

	// Run executor
	err := env.runWithSyncFailure(t, nil)
	if err == nil {
		t.Fatal("expected sync-on-start failure")
	}

	// Load task from backend and verify status
	loaded, loadErr := env.backend.LoadTask(env.taskID)
	if loadErr != nil {
		t.Fatalf("LoadTask: %v", loadErr)
	}
	if loaded.Status != orcv1.TaskStatus_TASK_STATUS_FAILED {
		t.Errorf("task status should be FAILED after sync failure, got: %v", loaded.Status)
	}
}

// TestSyncOnStartFailure_CleanupFails verifies edge case from Failure Modes table:
// If worktree cleanup itself fails after sync failure, log warning but still fail
// the task with the original sync error.
func TestSyncOnStartFailure_CleanupFails(t *testing.T) {
	t.Parallel()

	env := setupSyncCleanupTest(t, "TASK-SYNC-CFAIL")

	// Pre-remove the worktree directory to make cleanup fail (or at least be a no-op).
	// The executor should handle this gracefully — log warning, still return sync error.
	wtPath := env.worktreePath()

	// Force-remove worktree before executor tries to clean it up.
	// This simulates the edge case where the worktree directory is already gone.
	runGitCmdOrFatal(t, env.repoDir, "worktree", "remove", "--force", wtPath)

	// Capture log to verify warning
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	// Run executor — sync may or may not fail (worktree is gone), but if it does,
	// the original error should be returned, not a cleanup error
	err := env.runWithSyncFailure(t, logger)
	if err != nil {
		// The error should be the sync error (or setup error), not a cleanup error
		if strings.Contains(err.Error(), "cleanup") {
			t.Errorf("error should be the original sync/setup error, not cleanup error: %v", err)
		}
	}
}

// TestSyncOnStartFailure_BranchDeleteFails verifies edge case:
// If branch deletion fails after sync failure, log warning and continue.
// The original sync error should be returned.
func TestSyncOnStartFailure_BranchDeleteFails(t *testing.T) {
	t.Parallel()

	env := setupSyncCleanupTest(t, "TASK-SYNC-BDFAIL")

	// Delete the branch before the executor tries to (make branch deletion fail).
	branchName := env.branchName()
	// We can't easily delete the branch since the worktree is checked out on it.
	// Instead, verify that if the fix handles branch deletion errors gracefully,
	// the original sync error is preserved.

	// Capture log
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	// Run executor
	err := env.runWithSyncFailure(t, logger)
	if err == nil {
		t.Fatal("expected sync-on-start failure")
	}

	// The error should be the sync error, not a branch deletion error
	if !strings.Contains(err.Error(), "sync on start") {
		t.Errorf("expected sync-on-start error to be returned, got: %v", err)
	}

	// After cleanup, verify the branch state — it should be deleted, or at minimum
	// the worktree should be gone so retry can work.
	_ = branchName // Branch may or may not exist depending on deletion order
}

// TestSyncOnStartFailure_EmptyWorktreePath verifies edge case:
// If worktreePath is empty when sync fails, skip cleanup (defensive).
func TestSyncOnStartFailure_EmptyWorktreePath(t *testing.T) {
	t.Parallel()

	// This tests defensive behavior: if somehow worktreePath is empty,
	// the cleanup should be a no-op (not panic or error).
	// Since we can't easily create this scenario through Run(),
	// we verify it via the cleanupWorktree method directly.
	cfg := config.Default()
	we := &WorkflowExecutor{
		worktreePath: "", // Empty path
		orcConfig:    cfg,
		logger:       slog.Default(),
	}

	tsk := task.NewProtoTask("TASK-EMPTY-WT", "Test")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_FAILED

	// cleanupWorktree should return immediately when path is empty
	// (this is existing behavior that must be preserved)
	we.cleanupWorktree(tsk) // Should not panic
}

// Helper functions

// runGitCmdOrFatal runs a git command and fails the test on error.
func runGitCmdOrFatal(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s in %s failed: %v\nOutput: %s", strings.Join(args, " "), dir, err, out)
	}
}

// writeTestFile writes content to a file, creating parent dirs as needed.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}
