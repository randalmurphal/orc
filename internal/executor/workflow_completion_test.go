package executor

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"google.golang.org/protobuf/proto"
)

// setupWorkflowExecutorTest creates a test WorkflowExecutor with a real git repo
func setupWorkflowExecutorTest(t *testing.T) (*WorkflowExecutor, *git.Git, string) {
	t.Helper()

	// Create temporary git repository
	tmpDir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user
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

	// Create a task branch
	cmd = exec.Command("git", "checkout", "-b", "orc/TASK-001")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}

	// Create git.Git instance
	gitOps, err := git.New(tmpDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("failed to create git instance: %v", err)
	}

	// Add a fake remote (required for runCompletion to proceed)
	// Use file:// protocol to avoid triggering HTTPS auth prompts (askpass)
	cmd = exec.Command("git", "remote", "add", "origin", "file:///tmp/fake-remote.git")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	// Create WorkflowExecutor
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
	}

	return we, gitOps, tmpDir
}

// TestAutoCommitBeforeCompletion_DetectsAndCommitsChanges verifies that
// autoCommitBeforeCompletion detects uncommitted changes and commits them
// with the correct message format.
// Covers: SC-1 (auto-commit detects uncommitted changes before PR/merge)
func TestAutoCommitBeforeCompletion_DetectsAndCommitsChanges(t *testing.T) {
	t.Parallel()
	we, gitOps, tmpDir := setupWorkflowExecutorTest(t)

	// Create uncommitted changes
	newFile := filepath.Join(tmpDir, "feature.go")
	if err := os.WriteFile(newFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create task
	tsk := &orcv1.Task{
		Id:     "TASK-001",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
	}

	// Call autoCommitBeforeCompletion (this function doesn't exist yet - test will fail)
	err := we.autoCommitBeforeCompletion(gitOps, tsk)
	if err != nil {
		t.Fatalf("autoCommitBeforeCompletion() error: %v", err)
	}

	// Verify changes were committed
	hasChanges, err := gitOps.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	if hasChanges {
		t.Error("HasUncommittedChanges() = true after auto-commit, want false")
	}

	// Verify commit message format using git log
	cmd := exec.Command("git", "log", "-1", "--pretty=format:%s")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get commit message: %v", err)
	}

	expectedPrefix := "[orc] TASK-001: Auto-commit before PR creation"
	if !strings.HasPrefix(string(out), expectedPrefix) {
		t.Errorf("commit message = %q, want prefix %q", string(out), expectedPrefix)
	}

	// Verify commit body contains Co-Authored-By
	cmd = exec.Command("git", "log", "-1", "--pretty=format:%B")
	cmd.Dir = tmpDir
	fullMsg, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get full commit message: %v", err)
	}

	if !strings.Contains(string(fullMsg), "Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>") {
		t.Error("commit message missing Co-Authored-By line")
	}
}

// TestAutoCommitBeforeCompletion_SkipsCleanWorktree verifies that
// autoCommitBeforeCompletion returns early without creating a commit
// when the worktree is already clean.
// Covers: SC-3 (auto-commit skipped when worktree is clean)
func TestAutoCommitBeforeCompletion_SkipsCleanWorktree(t *testing.T) {
	t.Parallel()
	we, gitOps, tmpDir := setupWorkflowExecutorTest(t)

	// Get initial commit count
	cmd := exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to count commits: %v", err)
	}
	initialCount := strings.TrimSpace(string(out))

	// Create task
	tsk := &orcv1.Task{
		Id:     "TASK-001",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
	}

	// Call autoCommitBeforeCompletion with clean worktree
	err = we.autoCommitBeforeCompletion(gitOps, tsk)
	if err != nil {
		t.Fatalf("autoCommitBeforeCompletion() error: %v", err)
	}

	// Verify no new commit was created
	cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	cmd.Dir = tmpDir
	out, err = cmd.Output()
	if err != nil {
		t.Fatalf("failed to count commits: %v", err)
	}
	finalCount := strings.TrimSpace(string(out))

	if initialCount != finalCount {
		t.Errorf("commit count changed from %s to %s, want no change for clean worktree", initialCount, finalCount)
	}
}

// TestAutoCommitBeforeCompletion_HandlesGitErrors verifies that
// autoCommitBeforeCompletion handles git command errors gracefully
// and returns appropriate errors.
// Covers: Failure mode - git command failures
func TestAutoCommitBeforeCompletion_HandlesGitErrors(t *testing.T) {
	t.Parallel()

	// This test verifies error propagation from HasUncommittedChanges
	// We can't easily simulate git failures in a real repo, so we test
	// that errors are properly wrapped and returned
	// Implementation should wrap errors with context like:
	// "check uncommitted changes: %w" or "stage changes: %w"

	// Note: This is a design test - the actual implementation must
	// properly wrap and propagate errors from git operations
	t.Skip("Requires mock git implementation to simulate failures")
}

// TestAutoCommitBeforeCompletion_IncludesAllChanges verifies that
// autoCommitBeforeCompletion stages and commits all types of changes:
// untracked files, modified files, and deleted files.
// Tests edge case: mixed change types
// Covers: SC-1 (auto-commit detects all uncommitted changes)
func TestAutoCommitBeforeCompletion_IncludesAllChanges(t *testing.T) {
	t.Parallel()
	we, gitOps, tmpDir := setupWorkflowExecutorTest(t)

	// Create various types of changes
	// 1. Untracked file
	newFile := filepath.Join(tmpDir, "new.txt")
	if err := os.WriteFile(newFile, []byte("new file"), 0644); err != nil {
		t.Fatalf("failed to create new file: %v", err)
	}

	// 2. Modified file
	readmeFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(readmeFile, []byte("# Modified\n"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	// 3. Deleted file - first create and commit it
	deleteFile := filepath.Join(tmpDir, "to_delete.txt")
	if err := os.WriteFile(deleteFile, []byte("will be deleted"), 0644); err != nil {
		t.Fatalf("failed to create file to delete: %v", err)
	}
	cmd := exec.Command("git", "add", "to_delete.txt")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "Add file to delete")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit file: %v", err)
	}
	if err := os.Remove(deleteFile); err != nil {
		t.Fatalf("failed to delete file: %v", err)
	}

	// Create task
	tsk := &orcv1.Task{
		Id:     "TASK-001",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
	}

	// Call autoCommitBeforeCompletion
	err := we.autoCommitBeforeCompletion(gitOps, tsk)
	if err != nil {
		t.Fatalf("autoCommitBeforeCompletion() error: %v", err)
	}

	// Verify all changes were committed
	hasChanges, err := gitOps.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	if hasChanges {
		t.Error("HasUncommittedChanges() = true after auto-commit, want false (all changes should be committed)")
	}

	// Verify the auto-commit is the latest commit (not the "Add file to delete" commit)
	cmd = exec.Command("git", "log", "-1", "--pretty=format:%s")
	cmd.Dir = tmpDir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("failed to get commit message: %v", err)
	}

	if !strings.Contains(string(out), "Auto-commit before PR creation") {
		t.Errorf("latest commit = %q, want auto-commit message", string(out))
	}
}

// TestRunCompletion_CallsAutoCommitBeforePR verifies that runCompletion
// calls autoCommitBeforeCompletion before attempting to create a PR.
// Covers: SC-2 (auto-commit only runs when completion action is 'pr' or 'merge')
func TestRunCompletion_CallsAutoCommitBeforePR(t *testing.T) {
	t.Parallel()
	we, gitOps, tmpDir := setupWorkflowExecutorTest(t)

	// Create uncommitted changes
	newFile := filepath.Join(tmpDir, "uncommitted.go")
	if err := os.WriteFile(newFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create task with PR action
	tsk := &orcv1.Task{
		Id:     "TASK-001",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_MEDIUM, // maps to "pr" action in test config
	}

	// Mock remote (runCompletion checks for remote and skips if missing)
	// We'll skip the actual PR creation by having no gh CLI, but we should
	// see the auto-commit happen

	// Note: This test will fail during PR creation because we don't have
	// a real remote or gh CLI, but that's expected. We're testing that
	// auto-commit happens BEFORE the PR creation attempt.
	// The implementation should call autoCommitBeforeCompletion before
	// the sync and PR creation logic.

	ctx := context.Background()
	_ = we.runCompletion(ctx, tsk) // Error expected due to no remote

	// Verify changes were auto-committed (even though PR creation failed)
	// If auto-commit was called, the worktree should be clean
	hasChanges, err := gitOps.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	// After runCompletion, if auto-commit worked, changes should be committed
	// Note: This test may need adjustment based on actual implementation
	// The key is that auto-commit should happen before PR creation
	if hasChanges {
		t.Error("HasUncommittedChanges() = true after runCompletion with pr action, auto-commit should have committed changes")
	}
}

// TestRunCompletion_SkipsAutoCommitWhenActionNone verifies that runCompletion
// does not attempt auto-commit when the completion action is "none".
// Covers: SC-2 (auto-commit only runs when action is 'pr' or 'merge')
func TestRunCompletion_SkipsAutoCommitWhenActionNone(t *testing.T) {
	t.Parallel()
	we, gitOps, tmpDir := setupWorkflowExecutorTest(t)

	// Override config to use "none" action
	we.orcConfig.Completion.Action = "none"

	// Create uncommitted changes
	newFile := filepath.Join(tmpDir, "uncommitted.go")
	if err := os.WriteFile(newFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	// Create task with "none" action
	tsk := &orcv1.Task{
		Id:     "TASK-001",
		Weight: orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
	}

	ctx := context.Background()
	err := we.runCompletion(ctx, tsk)
	if err != nil {
		t.Fatalf("runCompletion() error: %v", err)
	}

	// Verify changes were NOT committed (auto-commit should be skipped)
	hasChanges, err := gitOps.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges() error: %v", err)
	}

	if !hasChanges {
		t.Error("HasUncommittedChanges() = false after runCompletion with action=none, want true (changes should remain uncommitted)")
	}
}

// TestAutoCommitBeforeCompletion_NonFatalErrors verifies that errors from
// autoCommitBeforeCompletion are treated as warnings and don't block completion.
// Covers: Failure mode - auto-commit errors should log warning but continue
func TestAutoCommitBeforeCompletion_NonFatalErrors(t *testing.T) {
	t.Parallel()

	// This test documents the expected behavior:
	// - autoCommitBeforeCompletion errors should be logged as warnings
	// - runCompletion should continue to PR creation even if auto-commit fails
	// - This is "best effort" - Claude might have committed some changes manually

	// Implementation note: In runCompletion, the call should be:
	// if err := we.autoCommitBeforeCompletion(...); err != nil {
	//     we.logger.Warn("auto-commit failed, continuing anyway", "error", err)
	// }

	t.Skip("Requires integration test with logger inspection")
}

func TestResolvePROptions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		task              *orcv1.Task
		config            *config.Config
		expectedDraft     bool
		expectedLabels    []string
		expectedReviewers []string
	}{
		{
			name: "config defaults only - no task overrides",
			task: &orcv1.Task{Id: "TASK-001"},
			config: &config.Config{
				Completion: config.CompletionConfig{
					PR: config.PRConfig{
						Draft:     false,
						Labels:    []string{"auto"},
						Reviewers: []string{"alice"},
					},
				},
			},
			expectedDraft:     false,
			expectedLabels:    []string{"auto"},
			expectedReviewers: []string{"alice"},
		},
		{
			name: "task overrides draft to true",
			task: &orcv1.Task{
				Id:      "TASK-001",
				PrDraft: proto.Bool(true),
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					PR: config.PRConfig{
						Draft:     false,
						Labels:    []string{"auto"},
						Reviewers: []string{"alice"},
					},
				},
			},
			expectedDraft:     true,
			expectedLabels:    []string{"auto"},
			expectedReviewers: []string{"alice"},
		},
		{
			name: "task overrides draft to false",
			task: &orcv1.Task{
				Id:      "TASK-001",
				PrDraft: proto.Bool(false),
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					PR: config.PRConfig{
						Draft: true,
					},
				},
			},
			expectedDraft:     false,
			expectedLabels:    nil,
			expectedReviewers: nil,
		},
		{
			name: "task overrides labels",
			task: &orcv1.Task{
				Id:          "TASK-001",
				PrLabels:    []string{"urgent", "hotfix"},
				PrLabelsSet: true,
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					PR: config.PRConfig{
						Labels: []string{"auto"},
					},
				},
			},
			expectedDraft:     false,
			expectedLabels:    []string{"urgent", "hotfix"},
			expectedReviewers: nil,
		},
		{
			name: "task clears labels with empty set",
			task: &orcv1.Task{
				Id:          "TASK-001",
				PrLabels:    nil,
				PrLabelsSet: true,
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					PR: config.PRConfig{
						Labels: []string{"auto", "ci"},
					},
				},
			},
			expectedDraft:     false,
			expectedLabels:    nil,
			expectedReviewers: nil,
		},
		{
			name: "task overrides reviewers",
			task: &orcv1.Task{
				Id:             "TASK-001",
				PrReviewers:    []string{"bob", "charlie"},
				PrReviewersSet: true,
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					PR: config.PRConfig{
						Reviewers: []string{"alice"},
					},
				},
			},
			expectedDraft:     false,
			expectedLabels:    nil,
			expectedReviewers: []string{"bob", "charlie"},
		},
		{
			name: "all task overrides applied",
			task: &orcv1.Task{
				Id:             "TASK-001",
				PrDraft:        proto.Bool(true),
				PrLabels:       []string{"feature"},
				PrLabelsSet:    true,
				PrReviewers:    []string{"dave"},
				PrReviewersSet: true,
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					PR: config.PRConfig{
						Draft:     false,
						Labels:    []string{"auto"},
						Reviewers: []string{"alice", "bob"},
					},
				},
			},
			expectedDraft:     true,
			expectedLabels:    []string{"feature"},
			expectedReviewers: []string{"dave"},
		},
		{
			name: "PrLabelsSet false does not override config",
			task: &orcv1.Task{
				Id:          "TASK-001",
				PrLabels:    []string{"ignored"},
				PrLabelsSet: false,
			},
			config: &config.Config{
				Completion: config.CompletionConfig{
					PR: config.PRConfig{
						Labels: []string{"from-config"},
					},
				},
			},
			expectedDraft:     false,
			expectedLabels:    []string{"from-config"},
			expectedReviewers: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := ResolvePROptions(tt.task, tt.config)

			if opts.Draft != tt.expectedDraft {
				t.Errorf("Draft = %v, want %v", opts.Draft, tt.expectedDraft)
			}
			if !reflect.DeepEqual(opts.Labels, tt.expectedLabels) {
				t.Errorf("Labels = %v, want %v", opts.Labels, tt.expectedLabels)
			}
			if !reflect.DeepEqual(opts.Reviewers, tt.expectedReviewers) {
				t.Errorf("Reviewers = %v, want %v", opts.Reviewers, tt.expectedReviewers)
			}
		})
	}
}
