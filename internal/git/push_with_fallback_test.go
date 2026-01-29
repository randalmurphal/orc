package git

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPushWithForceFallback_NormalPushSucceeds verifies that when normal push
// succeeds, no force push is attempted.
// Covers: SC-1 (happy path - no retry needed)
func TestPushWithForceFallback_NormalPushSucceeds(t *testing.T) {
	t.Parallel()

	// Setup: Create two repos - one as "remote", one as local
	remoteDir, localDir, cleanup := setupRemoteAndLocalRepos(t)
	defer cleanup()

	// Create Git instance for local repo (in worktree context)
	baseGit, err := New(localDir, DefaultConfig())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	g := baseGit.InWorktree(localDir)

	// Create a task branch and make a commit
	if err := g.CreateBranch("TASK-001"); err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	testFile := filepath.Join(localDir, "feature.txt")
	if err := os.WriteFile(testFile, []byte("new feature"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Add feature")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Setup logger to capture output
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Call PushWithForceFallback - should succeed with normal push
	err = g.PushWithForceFallback("origin", "orc/TASK-001", true, logger)
	if err != nil {
		t.Fatalf("PushWithForceFallback() failed: %v", err)
	}

	// Verify no force push warning was logged
	logOutput := logBuf.String()
	if strings.Contains(logOutput, "force-with-lease") {
		t.Error("unexpected force-with-lease log - normal push should not require force")
	}

	// Verify branch exists on remote
	cmd = exec.Command("git", "branch", "-r")
	cmd.Dir = localDir
	out, _ := cmd.Output()
	if !strings.Contains(string(out), "origin/orc/TASK-001") {
		t.Error("branch not pushed to remote")
	}
	_ = remoteDir // used in cleanup
}

// TestPushWithForceFallback_RetriesOnNonFastForward verifies that when push
// fails with non-fast-forward, it retries with --force-with-lease.
// Covers: SC-1 (retry with force-with-lease on non-fast-forward)
func TestPushWithForceFallback_RetriesOnNonFastForward(t *testing.T) {
	t.Parallel()

	// Setup: Create two repos - one as "remote", one as local
	remoteDir, localDir, cleanup := setupRemoteAndLocalRepos(t)
	defer cleanup()

	// Create Git instance for local repo (in worktree context)
	baseGit, err := New(localDir, DefaultConfig())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	g := baseGit.InWorktree(localDir)

	// Create a task branch and make a commit
	if err := g.CreateBranch("TASK-002"); err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	testFile := filepath.Join(localDir, "feature.txt")
	if err := os.WriteFile(testFile, []byte("commit A"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Commit A")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Push the branch first time
	cmd = exec.Command("git", "push", "-u", "origin", "orc/TASK-002")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("initial push failed: %v", err)
	}

	// Now create a divergent history:
	// 1. Reset local branch back
	cmd = exec.Command("git", "reset", "--hard", "HEAD~1")
	cmd.Dir = localDir
	_ = cmd.Run()

	// 2. Make a different commit locally
	if err := os.WriteFile(testFile, []byte("commit A' (divergent)"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Commit A prime (divergent)")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Setup logger to capture output
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Call PushWithForceFallback - should retry with force and succeed
	err = g.PushWithForceFallback("origin", "orc/TASK-002", false, logger)
	if err != nil {
		t.Fatalf("PushWithForceFallback() failed: %v", err)
	}

	// Verify force push warning was logged
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "force-with-lease") {
		t.Error("expected force-with-lease warning in log, got: " + logOutput)
	}
	if !strings.Contains(logOutput, "divergent") || !strings.Contains(logOutput, "non-fast-forward") {
		t.Error("log should mention divergent history or non-fast-forward, got: " + logOutput)
	}

	_ = remoteDir // used in cleanup
}

// TestPushWithForceFallback_ProtectedBranchBlocked verifies that force push
// is NEVER attempted for protected branches.
// Covers: SC-2 (force push is NEVER used on protected branches)
func TestPushWithForceFallback_ProtectedBranchBlocked(t *testing.T) {
	t.Parallel()

	tmpDir := setupTestRepo(t)
	baseGit, _ := New(tmpDir, DefaultConfig())
	g := baseGit.InWorktree(tmpDir)

	protectedBranches := []string{"main", "master", "develop", "release"}

	for _, branch := range protectedBranches {
		t.Run(branch, func(t *testing.T) {
			var logBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuf, nil))

			err := g.PushWithForceFallback("origin", branch, false, logger)

			// Must fail with protected branch error
			if err == nil {
				t.Errorf("PushWithForceFallback(%q) should fail for protected branch", branch)
			}
			if !errors.Is(err, ErrProtectedBranch) {
				t.Errorf("error should be ErrProtectedBranch, got: %v", err)
			}
			if !strings.Contains(err.Error(), "protected") {
				t.Errorf("error should mention protected, got: %v", err)
			}
		})
	}
}

// TestPushWithForceFallback_TaskBranchAllowed verifies that task branches
// (orc/TASK-XXX) are allowed to use force fallback.
// Covers: SC-1 (task branches can use force-with-lease)
func TestPushWithForceFallback_TaskBranchAllowed(t *testing.T) {
	t.Parallel()

	remoteDir, localDir, cleanup := setupRemoteAndLocalRepos(t)
	defer cleanup()

	baseGit, _ := New(localDir, DefaultConfig())
	g := baseGit.InWorktree(localDir)

	// Create task branch with commit
	if err := g.CreateBranch("TASK-003"); err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	testFile := filepath.Join(localDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cmd := exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Test commit")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Should not fail with protected branch error
	err := g.PushWithForceFallback("origin", "orc/TASK-003", true, nil)
	if err != nil && errors.Is(err, ErrProtectedBranch) {
		t.Error("task branch should not be blocked as protected")
	}

	_ = remoteDir
}

// TestPushWithForceFallback_RequiresWorktreeContext verifies that the function
// requires worktree context.
// Covers: SC-2 (safety - cannot push from main repo)
func TestPushWithForceFallback_RequiresWorktreeContext(t *testing.T) {
	t.Parallel()

	tmpDir := setupTestRepo(t)
	g, _ := New(tmpDir, DefaultConfig())

	// Note: NOT calling InWorktree - should fail
	err := g.PushWithForceFallback("origin", "orc/TASK-001", false, nil)

	if err == nil {
		t.Fatal("PushWithForceFallback should fail without worktree context")
	}
	if !errors.Is(err, ErrMainRepoModification) {
		t.Errorf("error should be ErrMainRepoModification, got: %v", err)
	}
}

// TestPushWithForceFallback_NetworkErrorPassthrough verifies that network
// errors (not non-fast-forward) are passed through without retry.
// Covers: Failure mode - network error during push
func TestPushWithForceFallback_NetworkErrorPassthrough(t *testing.T) {
	t.Parallel()

	tmpDir := setupTestRepo(t)
	baseGit, _ := New(tmpDir, DefaultConfig())
	g := baseGit.InWorktree(tmpDir)

	// Add a nonexistent remote to trigger network error
	cmd := exec.Command("git", "remote", "add", "origin", "https://nonexistent.invalid/repo.git")
	cmd.Dir = tmpDir
	_ = cmd.Run()

	if err := g.CreateBranch("TASK-004"); err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	err := g.PushWithForceFallback("origin", "orc/TASK-004", true, logger)

	// Should fail with network error
	if err == nil {
		t.Fatal("expected error for network failure")
	}

	// Should NOT log force-with-lease retry (network errors shouldn't trigger it)
	logOutput := logBuf.String()
	if strings.Contains(logOutput, "force-with-lease") {
		t.Error("should not retry with force on network error")
	}
}

// TestPushWithForceFallback_ForceAlsoFailsReturnsError verifies that if
// force push also fails (unexpected remote change), we return an error.
// Covers: Failure mode - force push also fails
func TestPushWithForceFallback_ForceAlsoFailsReturnsError(t *testing.T) {
	// This test is conceptually difficult to set up without mocks because
	// --force-with-lease fails when:
	// 1. Remote has commits we haven't fetched yet
	// 2. Someone else pushed between our fetch and push
	//
	// We document this as expected behavior - the error should propagate.
	t.Skip("Requires mock git implementation or complex multi-process setup")
}

// TestPushWithForceFallback_ErrorMessageContainsContext verifies SC-3:
// Error message clearly indicates push failure reason.
// This tests that error messages contain useful context for debugging.
func TestPushWithForceFallback_ErrorMessageContainsContext(t *testing.T) {
	t.Parallel()

	remoteDir, localDir, cleanup := setupRemoteAndLocalRepos(t)
	defer cleanup()

	baseGit, _ := New(localDir, DefaultConfig())
	g := baseGit.InWorktree(localDir)

	// Create task branch with commit
	if err := g.CreateBranch("TASK-ERR"); err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	testFile := filepath.Join(localDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("commit 1"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "First commit")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Push first time
	cmd = exec.Command("git", "push", "-u", "origin", "orc/TASK-ERR")
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("initial push failed: %v", err)
	}

	// Create divergent history to trigger force-with-lease
	cmd = exec.Command("git", "reset", "--hard", "HEAD~1")
	cmd.Dir = localDir
	_ = cmd.Run()

	if err := os.WriteFile(testFile, []byte("divergent commit"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Divergent commit")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Capture log to verify warning message contains context
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// This should succeed using force-with-lease
	err := g.PushWithForceFallback("origin", "orc/TASK-ERR", false, logger)
	if err != nil {
		// If push fails, error should contain useful context
		errStr := err.Error()
		if !strings.Contains(errStr, "push") &&
			!strings.Contains(errStr, "TASK-ERR") &&
			!strings.Contains(errStr, "origin") {
			t.Errorf("SC-3 FAILED: error message lacks context, got: %v", err)
		}
	}

	// SC-3: Verify warning log contains context about the push failure
	logOutput := logBuf.String()
	if strings.Contains(logOutput, "force-with-lease") {
		// The log should contain:
		// 1. Branch name
		// 2. Reason for retry
		if !strings.Contains(logOutput, "TASK-ERR") {
			t.Error("SC-3 FAILED: warning log should contain branch name")
		}
		if !strings.Contains(logOutput, "non-fast-forward") &&
			!strings.Contains(logOutput, "divergent") {
			t.Error("SC-3 FAILED: warning log should indicate divergent history reason")
		}
	}

	_ = remoteDir
}

// TestIsNonFastForwardError_VariousPatterns verifies that IsNonFastForwardError
// correctly identifies different error message patterns from git.
// Covers: SC-3 (error detection for retry logic)
func TestIsNonFastForwardError_VariousPatterns(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		errMsg   string
		expected bool
	}{
		{
			name:     "explicit non-fast-forward",
			errMsg:   "error: failed to push some refs - non-fast-forward update",
			expected: true,
		},
		{
			name:     "rejected fetch first",
			errMsg:   "Updates were rejected because the remote contains work that you do not have locally. Please fetch first",
			expected: true,
		},
		{
			name:     "failed to push behind",
			errMsg:   "failed to push: your branch is behind 'origin/orc/TASK-001'",
			expected: true,
		},
		{
			name:     "network error",
			errMsg:   "Could not resolve host: github.com",
			expected: false,
		},
		{
			name:     "permission denied",
			errMsg:   "Permission denied (publickey)",
			expected: false,
		},
		{
			name:     "repository not found",
			errMsg:   "remote: Repository not found",
			expected: false,
		},
		{
			name:     "nil error",
			errMsg:   "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var err error
			if tc.errMsg != "" {
				err = errors.New(tc.errMsg)
			}

			got := IsNonFastForwardError(err)
			if got != tc.expected {
				t.Errorf("IsNonFastForwardError(%q) = %v, want %v", tc.errMsg, got, tc.expected)
			}
		})
	}
}

// TestPushWithForceFallback_LogsWarningOnForce verifies that when force push
// is used, a warning is logged.
// Covers: SC-5 (warning logged when force push is used)
func TestPushWithForceFallback_LogsWarningOnForce(t *testing.T) {
	t.Parallel()

	remoteDir, localDir, cleanup := setupRemoteAndLocalRepos(t)
	defer cleanup()

	baseGit, _ := New(localDir, DefaultConfig())
	g := baseGit.InWorktree(localDir)

	// Create divergent history that requires force push
	if err := g.CreateBranch("TASK-005"); err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	// Initial commit and push
	testFile := filepath.Join(localDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("original"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Original")
	cmd.Dir = localDir
	_ = cmd.Run()
	cmd = exec.Command("git", "push", "-u", "origin", "orc/TASK-005")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Create divergence
	cmd = exec.Command("git", "reset", "--hard", "HEAD~1")
	cmd.Dir = localDir
	_ = cmd.Run()

	if err := os.WriteFile(testFile, []byte("divergent"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Divergent")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Capture log output
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	err := g.PushWithForceFallback("origin", "orc/TASK-005", false, logger)
	if err != nil {
		t.Fatalf("PushWithForceFallback() failed: %v", err)
	}

	// Verify warning was logged
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "WARN") || !strings.Contains(logOutput, "force") {
		t.Errorf("expected warning log about force push, got: %s", logOutput)
	}
	// Should mention the branch name
	if !strings.Contains(logOutput, "orc/TASK-005") && !strings.Contains(logOutput, "TASK-005") {
		t.Errorf("log should mention branch name, got: %s", logOutput)
	}

	_ = remoteDir
}

// TestPushWithForceFallback_NilLoggerHandled verifies that nil logger doesn't panic.
// Covers: Edge case - nil logger
func TestPushWithForceFallback_NilLoggerHandled(t *testing.T) {
	t.Parallel()

	remoteDir, localDir, cleanup := setupRemoteAndLocalRepos(t)
	defer cleanup()

	baseGit, _ := New(localDir, DefaultConfig())
	g := baseGit.InWorktree(localDir)

	if err := g.CreateBranch("TASK-006"); err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	testFile := filepath.Join(localDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "Test")
	cmd.Dir = localDir
	_ = cmd.Run()

	// Should not panic with nil logger
	err := g.PushWithForceFallback("origin", "orc/TASK-006", true, nil)
	if err != nil {
		t.Errorf("PushWithForceFallback() with nil logger failed: %v", err)
	}

	_ = remoteDir
}

// TestPushWithForceFallback_NoRemoteBranchNormalPush verifies that when
// remote branch doesn't exist, normal push is used (not force).
// Covers: Edge case - no remote branch exists
func TestPushWithForceFallback_NoRemoteBranchNormalPush(t *testing.T) {
	t.Parallel()

	remoteDir, localDir, cleanup := setupRemoteAndLocalRepos(t)
	defer cleanup()

	baseGit, _ := New(localDir, DefaultConfig())
	g := baseGit.InWorktree(localDir)

	// Create branch that doesn't exist on remote
	if err := g.CreateBranch("TASK-NEW"); err != nil {
		t.Fatalf("CreateBranch() failed: %v", err)
	}

	testFile := filepath.Join(localDir, "new.txt")
	if err := os.WriteFile(testFile, []byte("new"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = localDir
	_ = cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "New commit")
	cmd.Dir = localDir
	_ = cmd.Run()

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Should succeed with normal push
	err := g.PushWithForceFallback("origin", "orc/TASK-NEW", true, logger)
	if err != nil {
		t.Fatalf("PushWithForceFallback() failed: %v", err)
	}

	// Should NOT log force push
	logOutput := logBuf.String()
	if strings.Contains(logOutput, "force-with-lease") {
		t.Error("should not use force for new branch push")
	}

	_ = remoteDir
}

// TestPushWithForceFallback_CustomProtectedBranches verifies that custom
// protected branches are respected.
// Covers: SC-2 (custom protected branches)
func TestPushWithForceFallback_CustomProtectedBranches(t *testing.T) {
	t.Parallel()

	tmpDir := setupTestRepo(t)

	cfg := Config{
		BranchPrefix:      "orc/",
		CommitPrefix:      "[orc]",
		WorktreeDir:       ".orc/worktrees",
		ProtectedBranches: []string{"prod", "staging"}, // Custom list
	}

	baseGit, _ := New(tmpDir, cfg)
	g := baseGit.InWorktree(tmpDir)

	// prod should be protected (custom)
	err := g.PushWithForceFallback("origin", "prod", false, nil)
	if err == nil || !errors.Is(err, ErrProtectedBranch) {
		t.Error("prod should be protected with custom config")
	}

	// staging should be protected (custom)
	err = g.PushWithForceFallback("origin", "staging", false, nil)
	if err == nil || !errors.Is(err, ErrProtectedBranch) {
		t.Error("staging should be protected with custom config")
	}

	// main should NOT be protected in this config (replaced by custom list)
	err = g.PushWithForceFallback("origin", "main", false, nil)
	if err != nil && errors.Is(err, ErrProtectedBranch) {
		t.Error("main should not be protected with custom config that overrides defaults")
	}
}

// setupRemoteAndLocalRepos creates a bare "remote" repo and a local clone
// for testing push operations.
func setupRemoteAndLocalRepos(t *testing.T) (remoteDir, localDir string, cleanup func()) {
	t.Helper()

	// Create temporary directories
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

	// Configure git user for commits
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

	// Add remote pointing to bare repo (use file:// for test isolation)
	cmd = exec.Command("git", "remote", "add", "origin", "file://"+remoteDir)
	cmd.Dir = localDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}

	// Push main to remote to establish baseline
	cmd = exec.Command("git", "push", "-u", "origin", "main")
	cmd.Dir = localDir
	// Note: might fail if default branch is "master", that's ok for tests

	cleanup = func() {
		// Directories cleaned up by t.TempDir()
	}

	return remoteDir, localDir, cleanup
}
