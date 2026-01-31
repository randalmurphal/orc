package executor

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
)

// TestDivergentBranchPush_SucceedsWithForceFallback tests the scenario from the bug report:
// 1. Run task, make commits, push to remote
// 2. Task interrupted, worktree cleaned up
// 3. Re-run task from scratch
// 4. syncOnTaskStart rebases onto main, rewriting commits
// 5. Push should succeed using force-with-lease fallback
//
// Covers: SC-3 (When remote feature branch exists and local has divergent history, push succeeds)
func TestDivergentBranchPush_SucceedsWithForceFallback(t *testing.T) {
	t.Parallel()

	// Setup: Create bare "remote" and local clone
	remoteDir, localDir, cleanup := setupRemoteAndLocalReposForExecutor(t)
	defer cleanup()

	// Create Git instance for local repo
	gitCfg := git.DefaultConfig()
	gitCfg.WorktreeDir = filepath.Join(localDir, ".orc", "worktrees")
	gitOps, err := git.New(localDir, gitCfg)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Create worktree (simulating first task run)
	baseBranch, _ := gitOps.GetCurrentBranch()
	worktreePath, err := gitOps.CreateWorktree("TASK-DIV", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	// Note: cleanup is done manually before creating second worktree to simulate interruption

	wtGit := gitOps.InWorktree(worktreePath)

	// Make commits A, B in worktree (simulating first task execution)
	for _, name := range []string{"fileA.txt", "fileB.txt"} {
		testFile := filepath.Join(worktreePath, name)
		if err := os.WriteFile(testFile, []byte("content for "+name), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		cmd := exec.Command("git", "add", name)
		cmd.Dir = worktreePath
		_ = cmd.Run()
		cmd = exec.Command("git", "commit", "-m", "Add "+name)
		cmd.Dir = worktreePath
		_ = cmd.Run()
	}

	// Push to remote (first run - normal push)
	if err := wtGit.Push("origin", "orc/TASK-DIV", true); err != nil {
		t.Fatalf("initial push failed: %v", err)
	}

	// Verify remote has commits
	cmd := exec.Command("git", "ls-remote", "--refs", "origin", "refs/heads/orc/TASK-DIV")
	cmd.Dir = localDir
	out, err := cmd.Output()
	if err != nil || len(out) == 0 {
		t.Fatal("branch should exist on remote after first push")
	}

	// --- Simulate interruption and re-run ---

	// Clean up first worktree (simulating task interruption cleanup)
	if err := gitOps.CleanupWorktree("TASK-DIV"); err != nil {
		t.Fatalf("cleanup first worktree failed: %v", err)
	}

	// Make a new commit on main to trigger rebase scenario
	cmd = exec.Command("git", "checkout", "main")
	cmd.Dir = localDir
	_ = cmd.Run()

	newFile := filepath.Join(localDir, "main_change.txt")
	if err := os.WriteFile(newFile, []byte("change on main"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Change on main after task started")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Push main to remote
	cmd = exec.Command("git", "push", "origin", "main")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Create new worktree (simulating re-run after cleanup)
	worktreePath2, err := gitOps.CreateWorktree("TASK-DIV-2", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() second time failed: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-DIV-2") }()

	wtGit2 := gitOps.InWorktree(worktreePath2)

	// Checkout the task branch (exists from first run - use -B to reset it to current position)
	// This simulates the real scenario where CreateWorktree might create from main
	// and then syncOnTaskStart would fetch and merge the remote feature branch
	cmd = exec.Command("git", "checkout", "-B", "orc/TASK-DIV")
	cmd.Dir = worktreePath2
	if err := cmd.Run(); err != nil {
		t.Fatalf("checkout -B failed: %v", err)
	}

	// Simulate syncOnTaskStart: fetch, merge remote feature branch, rebase onto target
	cmd = exec.Command("git", "fetch", "origin")
	cmd.Dir = worktreePath2
	_ = cmd.Run()

	// Merge remote feature branch (this gets commits A, B)
	cmd = exec.Command("git", "merge", "origin/orc/TASK-DIV", "--no-edit")
	cmd.Dir = worktreePath2
	_ = cmd.Run()

	// Rebase onto origin/main (this REWRITES commits A, B to A', B')
	cmd = exec.Command("git", "rebase", "origin/main")
	cmd.Dir = worktreePath2
	if err := cmd.Run(); err != nil {
		t.Logf("rebase output: %v", err) // May have conflicts, but that's ok for this test
	}

	// Make new commits C, D (simulating second execution)
	for _, name := range []string{"fileC.txt", "fileD.txt"} {
		testFile := filepath.Join(worktreePath2, name)
		if err := os.WriteFile(testFile, []byte("content for "+name), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		cmd = exec.Command("git", "add", name)
		cmd.Dir = worktreePath2
		_ = cmd.Run()
		cmd = exec.Command("git", "commit", "-m", "Add "+name)
		cmd.Dir = worktreePath2
		_ = cmd.Run()
	}

	// Now local has A', B', C, D but remote has A, B - DIVERGENT
	// Normal push should fail
	err = wtGit2.Push("origin", "orc/TASK-DIV", false)
	if err == nil {
		t.Fatal("normal push should fail due to divergent history")
	}
	if !git.IsNonFastForwardError(err) {
		t.Fatalf("expected non-fast-forward error, got: %v", err)
	}

	// PushWithForceFallback should succeed
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	err = wtGit2.PushWithForceFallback("origin", "orc/TASK-DIV", false, logger)
	if err != nil {
		t.Fatalf("PushWithForceFallback() should succeed on divergent branch, got: %v", err)
	}

	// Verify new commits are on remote
	cmd = exec.Command("git", "log", "--oneline", "origin/orc/TASK-DIV")
	cmd.Dir = worktreePath2
	out, _ = cmd.Output()
	if !strings.Contains(string(out), "fileC") || !strings.Contains(string(out), "fileD") {
		t.Errorf("new commits not on remote after force push, log: %s", string(out))
	}

	_ = remoteDir
}

// TestCreatePR_UsesForceFallbackForDivergentBranch tests that createPR uses
// PushWithForceFallback to handle divergent history.
// Covers: SC-1 (integration - push in createPR uses fallback)
func TestCreatePR_UsesForceFallbackForDivergentBranch(t *testing.T) {
	t.Parallel()

	remoteDir, localDir, cleanup := setupRemoteAndLocalReposForExecutor(t)
	defer cleanup()

	gitCfg2 := git.DefaultConfig()
	gitCfg2.WorktreeDir = filepath.Join(localDir, ".orc", "worktrees")
	gitOps, _ := git.New(localDir, gitCfg2)
	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-PR", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-PR") }()

	wtGit := gitOps.InWorktree(worktreePath)

	// Setup WorkflowExecutor
	cfg := &config.Config{
		Completion: config.CompletionConfig{
			TargetBranch: "main",
			Action:       "pr",
		},
	}

	we := &WorkflowExecutor{
		worktreeGit:  wtGit,
		orcConfig:    cfg,
		logger:       slog.Default(),
		worktreePath: worktreePath,
	}

	// Create initial commits and push
	testFile := filepath.Join(worktreePath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = worktreePath
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Initial feature")
	cmd.Dir = worktreePath
	_ = cmd.Run()

	// Push to establish remote branch
	cmd = exec.Command("git", "push", "-u", "origin", "orc/TASK-PR")
	cmd.Dir = worktreePath
	_ = cmd.Run()

	// Create divergent history
	cmd = exec.Command("git", "reset", "--hard", "HEAD~1")
	cmd.Dir = worktreePath
	_ = cmd.Run()

	if err := os.WriteFile(testFile, []byte("divergent"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = worktreePath
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Divergent feature")
	cmd.Dir = worktreePath
	_ = cmd.Run()

	// Create task
	task := &orcv1.Task{
		Id:     "TASK-PR",
		Title:  "Test PR creation with divergent branch",
		Branch: "orc/TASK-PR",
	}

	// Call createPR - should use PushWithForceFallback internally
	ctx := context.Background()
	err = we.createPR(ctx, task, wtGit, "main")

	// The PR creation will fail (no gh CLI or remote GitHub), but the PUSH should succeed
	// Check if the error is about PR creation, not push
	if err != nil {
		if strings.Contains(err.Error(), "non-fast-forward") ||
			strings.Contains(err.Error(), "push failed") {
			t.Errorf("push should have used force fallback, got error: %v", err)
		}
		// Expected: "create PR: ..." (gh CLI error)
		t.Logf("expected error (no gh CLI): %v", err)
	}

	_ = remoteDir
}

// TestDirectMerge_UsesForceFallbackForDivergentBranch tests that directMerge
// uses PushWithForceFallback to handle divergent history.
// Covers: SC-1 (integration - push in directMerge uses fallback)
func TestDirectMerge_UsesForceFallbackForDivergentBranch(t *testing.T) {
	t.Parallel()

	remoteDir, localDir, cleanup := setupRemoteAndLocalReposForExecutor(t)
	defer cleanup()

	gitCfg3 := git.DefaultConfig()
	gitCfg3.WorktreeDir = filepath.Join(localDir, ".orc", "worktrees")
	gitOps, _ := git.New(localDir, gitCfg3)
	baseBranch, _ := gitOps.GetCurrentBranch()

	// Create worktree
	worktreePath, err := gitOps.CreateWorktree("TASK-MERGE", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-MERGE") }()

	wtGit := gitOps.InWorktree(worktreePath)

	// Create initial commits and push
	testFile := filepath.Join(worktreePath, "feature.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = worktreePath
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Initial feature")
	cmd.Dir = worktreePath
	_ = cmd.Run()

	// Push to establish remote branch
	cmd = exec.Command("git", "push", "-u", "origin", "orc/TASK-MERGE")
	cmd.Dir = worktreePath
	_ = cmd.Run()

	// Create divergent history
	cmd = exec.Command("git", "reset", "--hard", "HEAD~1")
	cmd.Dir = worktreePath
	_ = cmd.Run()

	if err := os.WriteFile(testFile, []byte("divergent content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = worktreePath
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Divergent feature")
	cmd.Dir = worktreePath
	_ = cmd.Run()

	// Setup WorkflowExecutor (minimal setup for directMerge test)
	cfg := &config.Config{
		Completion: config.CompletionConfig{
			TargetBranch: "main",
			Action:       "merge",
		},
	}

	we := &WorkflowExecutor{
		worktreeGit:  wtGit,
		orcConfig:    cfg,
		logger:       slog.Default(),
		worktreePath: worktreePath,
	}

	// Create task
	task := &orcv1.Task{
		Id:     "TASK-MERGE",
		Title:  "Test direct merge with divergent branch",
		Branch: "orc/TASK-MERGE",
	}

	// Call directMerge - the first push (task branch) should use force fallback
	ctx := context.Background()
	err = we.directMerge(ctx, task, wtGit, "main")

	// Check if failure is about force push or something else
	if err != nil {
		if strings.Contains(err.Error(), "non-fast-forward") {
			t.Errorf("first push should have used force fallback, got error: %v", err)
		}
		// Expected errors: "checkout target" (no main in worktree), etc.
		t.Logf("expected error (incomplete setup): %v", err)
	}

	_ = remoteDir
}

// TestResume_InterruptedTaskWithRemoteCommits tests that resuming a task
// after interruption properly handles existing remote commits.
// Covers: SC-4 (orc resume on interrupted task with prior remote commits)
func TestResume_InterruptedTaskWithRemoteCommits(t *testing.T) {
	t.Parallel()

	remoteDir, localDir, cleanup := setupRemoteAndLocalReposForExecutor(t)
	defer cleanup()

	gitCfg4 := git.DefaultConfig()
	gitCfg4.WorktreeDir = filepath.Join(localDir, ".orc", "worktrees")
	gitOps, _ := git.New(localDir, gitCfg4)
	baseBranch, _ := gitOps.GetCurrentBranch()

	// === First execution: Make commits and push ===
	worktreePath1, err := gitOps.CreateWorktree("TASK-RESUME", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() failed: %v", err)
	}

	wtGit1 := gitOps.InWorktree(worktreePath1)

	// Make commits from first execution
	for i, name := range []string{"impl1.go", "impl2.go"} {
		testFile := filepath.Join(worktreePath1, name)
		content := "package main\n// implementation " + string(rune('A'+i))
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		cmd := exec.Command("git", "add", name)
		cmd.Dir = worktreePath1
		_ = cmd.Run()
		cmd = exec.Command("git", "commit", "-m", "Add "+name)
		cmd.Dir = worktreePath1
		_ = cmd.Run()
	}

	// Push to remote
	if err := wtGit1.Push("origin", "orc/TASK-RESUME", true); err != nil {
		t.Fatalf("initial push failed: %v", err)
	}

	// === Simulate interruption: Cleanup worktree ===
	if err := gitOps.CleanupWorktree("TASK-RESUME"); err != nil {
		t.Fatalf("cleanup worktree failed: %v", err)
	}

	// === Resume: Create new worktree ===
	worktreePath2, err := gitOps.CreateWorktree("TASK-RESUME", baseBranch)
	if err != nil {
		t.Fatalf("CreateWorktree() for resume failed: %v", err)
	}
	defer func() { _ = gitOps.CleanupWorktree("TASK-RESUME") }()

	wtGit2 := gitOps.InWorktree(worktreePath2)

	// Checkout the task branch (should exist from first run)
	cmd := exec.Command("git", "checkout", "orc/TASK-RESUME")
	cmd.Dir = worktreePath2
	if err := cmd.Run(); err != nil {
		// Branch might not exist locally, create from remote
		cmd = exec.Command("git", "checkout", "-b", "orc/TASK-RESUME", "origin/orc/TASK-RESUME")
		cmd.Dir = worktreePath2
		if err := cmd.Run(); err != nil {
			t.Fatalf("checkout task branch failed: %v", err)
		}
	}

	// Verify prior commits are present
	cmd = exec.Command("git", "log", "--oneline")
	cmd.Dir = worktreePath2
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "impl1") || !strings.Contains(string(out), "impl2") {
		t.Errorf("prior commits should be present after resume, got: %s", string(out))
	}

	// Verify prior files are present
	if _, err := os.Stat(filepath.Join(worktreePath2, "impl1.go")); os.IsNotExist(err) {
		t.Error("impl1.go should exist after resume")
	}
	if _, err := os.Stat(filepath.Join(worktreePath2, "impl2.go")); os.IsNotExist(err) {
		t.Error("impl2.go should exist after resume")
	}

	// Make new commits (continued execution)
	testFile := filepath.Join(worktreePath2, "impl3.go")
	if err := os.WriteFile(testFile, []byte("package main\n// continued work"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = worktreePath2
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Add impl3 (continued)")
	cmd.Dir = worktreePath2
	_ = cmd.Run()

	// Push should succeed (no divergence in this case - just ahead)
	if err := wtGit2.Push("origin", "orc/TASK-RESUME", false); err != nil {
		t.Fatalf("push after resume should succeed: %v", err)
	}

	// Verify all commits are on remote
	cmd = exec.Command("git", "log", "--oneline", "origin/orc/TASK-RESUME")
	cmd.Dir = worktreePath2
	out, _ = cmd.Output()
	if !strings.Contains(string(out), "impl1") ||
		!strings.Contains(string(out), "impl2") ||
		!strings.Contains(string(out), "impl3") {
		t.Errorf("all commits should be on remote, got: %s", string(out))
	}

	_ = remoteDir
}

// setupRemoteAndLocalReposForExecutor creates a bare "remote" repo and local clone.
// Similar to git package helper but in executor package for test isolation.
func setupRemoteAndLocalReposForExecutor(t *testing.T) (remoteDir, localDir string, cleanup func()) {
	t.Helper()

	remoteDir = t.TempDir()
	localDir = t.TempDir()

	// Initialize bare "remote" repository
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = remoteDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}

	// Initialize local repository
	cmd = exec.Command("git", "init")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init local repo: %v", err)
	}

	// Configure git user
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = localDir
	_ = cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Create initial commit on main
	testFile := filepath.Join(localDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create initial commit: %v", err)
	}

	// Add remote pointing to bare repo
	cmd = exec.Command("git", "remote", "add", "origin", "file://"+remoteDir)
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	// Push main to remote
	cmd = exec.Command("git", "push", "-u", "origin", "main")
	cmd.Dir = localDir
	_ = cmd.Run() // May fail if default branch is master

	cleanup = func() {
		// Directories cleaned up by t.TempDir()
	}

	return remoteDir, localDir, cleanup
}
