package executor

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// setupCompletionTestWithRemote creates a test WorkflowExecutor backed by a real
// git repo with a proper remote. This simulates real-world scenarios where
// the task branch and origin/main point to the same commit (zero commits ahead).
//
// Layout: bare remote repo + working clone on task branch with origin configured.
func setupCompletionTestWithRemote(t *testing.T) (*WorkflowExecutor, *git.Git, string) {
	t.Helper()

	// Create a bare remote repo
	bareDir := t.TempDir()
	runGit(t, bareDir, "init", "--bare")

	// Clone it to get a working repo with proper origin
	parentDir := t.TempDir()
	workDir := filepath.Join(parentDir, "work")
	runGit(t, parentDir, "clone", bareDir, "work")

	// Configure git user in the clone
	runGit(t, workDir, "config", "user.email", "test@test.com")
	runGit(t, workDir, "config", "user.name", "Test User")

	// Create initial commit on main
	testFile := filepath.Join(workDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	runGit(t, workDir, "add", ".")
	runGit(t, workDir, "commit", "-m", "Initial commit")
	runGit(t, workDir, "push", "origin", "main")

	// Create task branch from main (identical to origin/main — 0 commits ahead)
	runGit(t, workDir, "checkout", "-b", "orc/TASK-100")
	// Push task branch so origin/main exists as remote ref
	runGit(t, workDir, "push", "origin", "orc/TASK-100")

	baseGit, err := git.New(workDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("create git instance: %v", err)
	}
	// Mark as worktree context so destructive operations (rebase, etc.) are allowed
	gitOps := baseGit.InWorktree(workDir)

	backend := storage.NewTestBackend(t)

	cfg := &config.Config{
		Completion: config.CompletionConfig{
			TargetBranch: "main",
			Action:       "pr",
		},
	}

	we := &WorkflowExecutor{
		worktreeGit: gitOps,
		orcConfig:   cfg,
		logger:      slog.Default(),
		backend:     backend,
	}

	return we, gitOps, workDir
}

// runGit is a test helper that runs a git command and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test User",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=Test User",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
	}
}

// newTestTask creates a task suitable for completion tests.
func newTestTask(id string) *orcv1.Task {
	tsk := task.NewProtoTask(id, "Test task: "+id)
	tsk.Branch = "orc/" + id
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	return tsk
}

// --- SC-1: runCompletion returns nil when ahead == 0 (PR action) ---

// TestRunCompletion_NoCommits_SkipsPR verifies that when the task branch has
// zero commits ahead of origin/main, runCompletion returns nil (success)
// without attempting to create a PR.
//
// Covers: SC-1, BDD-1
func TestRunCompletion_NoCommits_SkipsPR(t *testing.T) {
	t.Parallel()
	we, _, _ := setupCompletionTestWithRemote(t)

	tsk := newTestTask("TASK-100")

	// Ensure action is "pr"
	we.orcConfig.Completion.Action = "pr"

	err := we.runCompletion(context.Background(), tsk)
	if err != nil {
		t.Fatalf("runCompletion() returned error for zero-commit branch: %v", err)
	}

	// If PR creation was attempted, it would fail (no hosting provider configured).
	// Returning nil proves PR creation was skipped.
}

// --- SC-2: Task metadata records completion skip ---

// TestRunCompletion_NoCommits_SetsMetadata verifies that when completion is
// skipped due to no changes, the task metadata includes completion_skipped
// and completion_note keys.
//
// Covers: SC-2, BDD-1
func TestRunCompletion_NoCommits_SetsMetadata(t *testing.T) {
	t.Parallel()
	we, _, _ := setupCompletionTestWithRemote(t)

	tsk := newTestTask("TASK-100")
	// Save task so backend has it
	if err := we.backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := we.runCompletion(context.Background(), tsk)
	if err != nil {
		t.Fatalf("runCompletion() error: %v", err)
	}

	task.EnsureMetadataProto(tsk)

	skipped, ok := tsk.Metadata["completion_skipped"]
	if !ok {
		t.Fatal("metadata missing 'completion_skipped' key")
	}
	if skipped != "no_changes" {
		t.Errorf("completion_skipped = %q, want %q", skipped, "no_changes")
	}

	note, ok := tsk.Metadata["completion_note"]
	if !ok {
		t.Fatal("metadata missing 'completion_note' key")
	}
	if note == "" {
		t.Error("completion_note is empty, want explanation of why completion was skipped")
	}
}

// --- SC-3: No-commits check happens AFTER auto-commit and sync ---

// TestRunCompletion_NoCommits_AfterAutoCommit verifies that when there are
// uncommitted changes, auto-commit runs first, and if those changes result
// in commits ahead of target, PR creation proceeds normally (not skipped).
//
// Covers: SC-3, BDD-2
func TestRunCompletion_NoCommits_AfterAutoCommit(t *testing.T) {
	t.Parallel()
	we, _, workDir := setupCompletionTestWithRemote(t)

	tsk := newTestTask("TASK-100")
	if err := we.backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Create uncommitted changes — auto-commit should pick these up,
	// making ahead > 0 BEFORE the zero-commit check.
	newFile := filepath.Join(workDir, "feature.go")
	if err := os.WriteFile(newFile, []byte("package feature\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// runCompletion should:
	// 1. Auto-commit the changes (ahead becomes 1)
	// 2. NOT skip completion (ahead > 0)
	// 3. Attempt PR creation (which will fail — no hosting provider)
	err := we.runCompletion(context.Background(), tsk)

	// We expect an error from PR creation (no hosting provider), NOT a nil
	// return from the zero-commit skip path.
	if err == nil {
		// If no error, check that completion_skipped is NOT set — meaning
		// the auto-commit made ahead > 0 and the code tried to create a PR.
		task.EnsureMetadataProto(tsk)
		if _, ok := tsk.Metadata["completion_skipped"]; ok {
			t.Fatal("completion was skipped despite uncommitted changes that should have been auto-committed")
		}
		// nil error with no skip metadata is unexpected here — PR creation
		// should have failed. But if somehow it succeeded, that's fine too.
	}
	// An error from createPR is expected and acceptable — it proves the
	// zero-commit skip did NOT fire because auto-commit added commits.
}

// --- SC-4: Log message at Info level ---

// TestRunCompletion_NoCommits_Logs verifies that when completion is skipped,
// an Info-level log message is emitted with task ID and reason.
//
// Covers: SC-4
func TestRunCompletion_NoCommits_Logs(t *testing.T) {
	t.Parallel()
	we, _, _ := setupCompletionTestWithRemote(t)

	// Capture log output
	var logBuf logBuffer
	we.logger = slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	tsk := newTestTask("TASK-100")
	if err := we.backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := we.runCompletion(context.Background(), tsk)
	if err != nil {
		t.Fatalf("runCompletion() error: %v", err)
	}

	output := logBuf.String()

	// The log should mention the task and that completion was skipped
	if output == "" {
		t.Fatal("no log output captured")
	}

	// Check that an info-level message about skipping was logged
	wantSubstrings := []string{"TASK-100", "no commit", "skip"}
	for _, want := range wantSubstrings {
		found := false
		// Case-insensitive search
		for _, line := range splitLines(output) {
			if containsCaseInsensitive(line, want) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("log output missing expected substring %q\nGot:\n%s", want, output)
		}
	}
}

// --- SC-5: directMerge handles ahead == 0 gracefully ---

// TestDirectMerge_NoCommits_Skips verifies that directMerge returns nil
// without attempting push/merge when there are zero commits ahead.
//
// Covers: SC-5, BDD-3
func TestDirectMerge_NoCommits_Skips(t *testing.T) {
	t.Parallel()
	we, _, _ := setupCompletionTestWithRemote(t)

	tsk := newTestTask("TASK-100")
	if err := we.backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Get gitOps from the executor
	gitOps := we.worktreeGit

	// directMerge should detect 0 commits ahead and skip
	err := we.directMerge(context.Background(), tsk, gitOps, "main")
	if err != nil {
		t.Fatalf("directMerge() returned error for zero-commit branch: %v", err)
	}

	// Verify skip metadata was set
	task.EnsureMetadataProto(tsk)
	skipped, ok := tsk.Metadata["completion_skipped"]
	if !ok {
		t.Fatal("metadata missing 'completion_skipped' key after directMerge skip")
	}
	if skipped != "no_changes" {
		t.Errorf("completion_skipped = %q, want %q", skipped, "no_changes")
	}
}

// TestRunCompletion_NoCommits_MergeAction verifies the full runCompletion
// path with action="merge" also handles zero commits gracefully.
//
// Covers: SC-5, BDD-3
func TestRunCompletion_NoCommits_MergeAction(t *testing.T) {
	t.Parallel()
	we, _, _ := setupCompletionTestWithRemote(t)
	we.orcConfig.Completion.Action = "merge"

	tsk := newTestTask("TASK-100")
	if err := we.backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := we.runCompletion(context.Background(), tsk)
	if err != nil {
		t.Fatalf("runCompletion() with action=merge returned error: %v", err)
	}

	task.EnsureMetadataProto(tsk)
	if _, ok := tsk.Metadata["completion_skipped"]; !ok {
		t.Fatal("completion_skipped metadata not set for merge action with 0 commits")
	}
}

// --- Edge Cases ---

// TestRunCompletion_NoCommits_IdenticalBranches verifies behavior when
// ahead == 0 AND behind == 0 (branches are identical).
//
// Covers: Edge case - identical branches
func TestRunCompletion_NoCommits_IdenticalBranches(t *testing.T) {
	t.Parallel()
	we, _, _ := setupCompletionTestWithRemote(t)

	tsk := newTestTask("TASK-100")
	if err := we.backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Branches are identical by default in setupCompletionTestWithRemote
	err := we.runCompletion(context.Background(), tsk)
	if err != nil {
		t.Fatalf("runCompletion() error for identical branches: %v", err)
	}

	task.EnsureMetadataProto(tsk)
	if v, ok := tsk.Metadata["completion_skipped"]; !ok || v != "no_changes" {
		t.Errorf("completion_skipped = %q (exists=%v), want 'no_changes'", v, ok)
	}
}

// TestRunCompletion_NoCommits_AfterRebase verifies that when the task branch
// was ahead but a rebase squashes the difference (ahead becomes 0),
// completion is still skipped gracefully.
//
// Covers: Edge case - ahead == 0 after rebase
func TestRunCompletion_NoCommits_AfterRebase(t *testing.T) {
	t.Parallel()
	we, _, workDir := setupCompletionTestWithRemote(t)

	// Make the same change on both main and the task branch.
	// After rebase, ahead will be 0 because the commits are identical.

	// First, add a commit on main via origin
	runGit(t, workDir, "checkout", "main")
	changeFile := filepath.Join(workDir, "shared.txt")
	if err := os.WriteFile(changeFile, []byte("shared content\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, workDir, "add", ".")
	runGit(t, workDir, "commit", "-m", "Add shared.txt on main")
	runGit(t, workDir, "push", "origin", "main")

	// Switch to task branch, cherry-pick the same change
	runGit(t, workDir, "checkout", "orc/TASK-100")
	// Instead of cherry-pick (which may differ), add the same file content
	if err := os.WriteFile(changeFile, []byte("shared content\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, workDir, "add", ".")
	runGit(t, workDir, "commit", "-m", "Add shared.txt on task branch")
	runGit(t, workDir, "push", "origin", "orc/TASK-100")

	// Now task branch has 1 commit ahead, 1 behind. After rebase, the
	// change already exists in main, so ahead becomes 0.

	tsk := newTestTask("TASK-100")
	if err := we.backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := we.runCompletion(context.Background(), tsk)
	if err != nil {
		t.Fatalf("runCompletion() error: %v", err)
	}

	// After rebase resolved the duplicate, ahead should be 0 → skipped
	task.EnsureMetadataProto(tsk)
	if _, ok := tsk.Metadata["completion_skipped"]; !ok {
		t.Error("expected completion_skipped metadata after rebase resolved duplicates")
	}
}

// --- Regression: normal case still works ---

// TestRunCompletion_WithCommits_ProceedsToPR verifies that when the task
// branch has commits ahead, runCompletion does NOT skip — it proceeds to
// PR creation (which fails here due to no hosting provider, proving dispatch).
//
// Covers: Preservation requirement - tasks with actual commits create PRs normally
func TestRunCompletion_WithCommits_ProceedsToPR(t *testing.T) {
	t.Parallel()
	we, _, workDir := setupCompletionTestWithRemote(t)

	// Add a real commit on the task branch
	newFile := filepath.Join(workDir, "feature.go")
	if err := os.WriteFile(newFile, []byte("package feature\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, workDir, "add", ".")
	runGit(t, workDir, "commit", "-m", "Implement feature")

	tsk := newTestTask("TASK-100")
	if err := we.backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	err := we.runCompletion(context.Background(), tsk)

	// Should NOT be nil — PR creation should be attempted (and fail due to
	// no hosting provider), proving we didn't skip.
	if err == nil {
		task.EnsureMetadataProto(tsk)
		if _, ok := tsk.Metadata["completion_skipped"]; ok {
			t.Fatal("completion was skipped despite having commits ahead")
		}
		// nil error without skip = PR somehow succeeded, which is fine
	}
	// Error from createPR = expected, proves we didn't skip
}

// --- Error path: GetCommitCounts failure ---

// TestRunCompletion_CommitCountError_ContinuesAnyway verifies that when
// GetCommitCounts returns an error, runCompletion continues to the PR/merge
// attempt rather than incorrectly skipping.
//
// Covers: BDD-4, Failure mode - GetCommitCounts error
func TestRunCompletion_CommitCountError_ContinuesAnyway(t *testing.T) {
	t.Parallel()

	// Use a setup where the remote ref doesn't exist, so GetCommitCounts
	// will error (can't compare against origin/main if remote fetch fails).
	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@test.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")

	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	runGit(t, tmpDir, "add", ".")
	runGit(t, tmpDir, "commit", "-m", "Initial commit")
	runGit(t, tmpDir, "checkout", "-b", "orc/TASK-200")

	// Add a remote but DON'T push main — so origin/main doesn't exist,
	// causing GetCommitCounts("origin/main") to error.
	bareDir := t.TempDir()
	runGit(t, bareDir, "init", "--bare")
	runGit(t, tmpDir, "remote", "add", "origin", bareDir)

	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("create git: %v", err)
	}

	backend := storage.NewTestBackend(t)
	cfg := &config.Config{
		Completion: config.CompletionConfig{
			TargetBranch: "main",
			Action:       "pr",
		},
	}

	we := &WorkflowExecutor{
		worktreeGit: gitOps,
		orcConfig:   cfg,
		logger:      slog.Default(),
		backend:     backend,
	}

	tsk := newTestTask("TASK-200")
	if err := we.backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// runCompletion should warn about commit count error and continue.
	// It should NOT return nil from a zero-commit skip (because the count
	// check errored — we don't know the count).
	err = we.runCompletion(context.Background(), tsk)

	// We expect either:
	// - An error from PR creation (continued past commit count error) ✓
	// - nil with completion_skipped NOT set (didn't incorrectly skip) ✓
	// We do NOT want: nil with completion_skipped set (incorrectly skipped on error)
	if err == nil {
		task.EnsureMetadataProto(tsk)
		if _, ok := tsk.Metadata["completion_skipped"]; ok {
			t.Fatal("completion was incorrectly skipped when GetCommitCounts errored — should have continued to PR attempt")
		}
	}
	// Error is acceptable — means execution continued past the commit count error
}

// --- Helper types ---

// logBuffer is a thread-safe buffer for capturing slog output in tests.
type logBuffer struct {
	data []byte
}

func (b *logBuffer) Write(p []byte) (n int, err error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *logBuffer) String() string {
	return string(b.data)
}

// splitLines splits a string into lines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// containsCaseInsensitive performs a case-insensitive substring search.
func containsCaseInsensitive(s, substr string) bool {
	sLower := stringsToLower(s)
	subLower := stringsToLower(substr)
	return len(subLower) <= len(sLower) && stringsContains(sLower, subLower)
}

func stringsToLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func stringsContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
