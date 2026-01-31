package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/hosting"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// =============================================================================
// Mock provider for createPR tests
// =============================================================================

// prTestProvider implements hosting.Provider for createPR tests.
// Separate from mockProvider in ci_merge_test.go to avoid coupling.
type prTestProvider struct {
	// Configurable method behavior
	findPRByBranchFunc func(ctx context.Context, branch string) (*hosting.PR, error)
	createPRFunc       func(ctx context.Context, opts hosting.PRCreateOptions) (*hosting.PR, error)
	updatePRFunc       func(ctx context.Context, number int, opts hosting.PRUpdateOptions) error
	enableAutoMergeErr error
	approvePRErr       error

	// Call tracking for assertions
	findPRByBranchCalls []string
	createPRCalls       []hosting.PRCreateOptions
	updatePRCalls       []struct {
		Number int
		Opts   hosting.PRUpdateOptions
	}
	enableAutoMergeCalls []struct {
		Number int
		Method string
	}
	approvePRCalls []struct {
		Number int
		Body   string
	}
}

func (p *prTestProvider) FindPRByBranch(ctx context.Context, branch string) (*hosting.PR, error) {
	p.findPRByBranchCalls = append(p.findPRByBranchCalls, branch)
	if p.findPRByBranchFunc != nil {
		return p.findPRByBranchFunc(ctx, branch)
	}
	return nil, hosting.ErrNoPRFound
}

func (p *prTestProvider) CreatePR(ctx context.Context, opts hosting.PRCreateOptions) (*hosting.PR, error) {
	p.createPRCalls = append(p.createPRCalls, opts)
	if p.createPRFunc != nil {
		return p.createPRFunc(ctx, opts)
	}
	return &hosting.PR{
		Number:  99,
		HTMLURL: "https://github.com/owner/repo/pull/99",
		Title:   opts.Title,
		State:   "open",
	}, nil
}

func (p *prTestProvider) UpdatePR(ctx context.Context, number int, opts hosting.PRUpdateOptions) error {
	p.updatePRCalls = append(p.updatePRCalls, struct {
		Number int
		Opts   hosting.PRUpdateOptions
	}{number, opts})
	if p.updatePRFunc != nil {
		return p.updatePRFunc(ctx, number, opts)
	}
	return nil
}

func (p *prTestProvider) EnableAutoMerge(_ context.Context, number int, method string) error {
	p.enableAutoMergeCalls = append(p.enableAutoMergeCalls, struct {
		Number int
		Method string
	}{number, method})
	return p.enableAutoMergeErr
}

func (p *prTestProvider) ApprovePR(_ context.Context, number int, body string) error {
	p.approvePRCalls = append(p.approvePRCalls, struct {
		Number int
		Body   string
	}{number, body})
	return p.approvePRErr
}

// Stub implementations for interface satisfaction (unused by createPR tests).
func (p *prTestProvider) GetPR(context.Context, int) (*hosting.PR, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *prTestProvider) MergePR(context.Context, int, hosting.PRMergeOptions) error {
	return fmt.Errorf("not implemented")
}
func (p *prTestProvider) ListPRComments(context.Context, int) ([]hosting.PRComment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *prTestProvider) CreatePRComment(context.Context, int, hosting.PRCommentCreate) (*hosting.PRComment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *prTestProvider) ReplyToComment(context.Context, int, int64, string) (*hosting.PRComment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *prTestProvider) GetPRComment(context.Context, int, int64) (*hosting.PRComment, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *prTestProvider) GetCheckRuns(context.Context, string) ([]hosting.CheckRun, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *prTestProvider) GetPRReviews(context.Context, int) ([]hosting.PRReview, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *prTestProvider) GetPRStatusSummary(context.Context, *hosting.PR) (*hosting.PRStatusSummary, error) {
	return nil, fmt.Errorf("not implemented")
}
func (p *prTestProvider) UpdatePRBranch(context.Context, int) error {
	return fmt.Errorf("not implemented")
}
func (p *prTestProvider) DeleteBranch(context.Context, string) error {
	return fmt.Errorf("not implemented")
}
func (p *prTestProvider) CheckAuth(context.Context) error { return nil }
func (p *prTestProvider) Name() hosting.ProviderType      { return "mock" }
func (p *prTestProvider) OwnerRepo() (string, string)     { return "owner", "repo" }

// =============================================================================
// Test setup
// =============================================================================

// createPRTestEnv holds all components needed for createPR tests.
type createPRTestEnv struct {
	we      *WorkflowExecutor
	mock    *prTestProvider
	task    *orcv1.Task
	gitOps  *git.Git
	logBuf  *bytes.Buffer
	backend *storage.DatabaseBackend
}

// setupCreatePRTest creates a test environment with a real git repo (with pushable
// bare remote), an in-memory backend, and a mock hosting provider injected into
// the WorkflowExecutor.
func setupCreatePRTest(t *testing.T) *createPRTestEnv {
	t.Helper()

	// Create bare remote repo that accepts pushes
	remoteDir := t.TempDir()
	runGitInDir(t, remoteDir, "init", "--bare")

	// Create local repo
	localDir := t.TempDir()
	runGitInDir(t, localDir, "init", "-b", "main")
	runGitInDir(t, localDir, "config", "user.email", "test@test.com")
	runGitInDir(t, localDir, "config", "user.name", "Test User")
	runGitInDir(t, localDir, "remote", "add", "origin", remoteDir)

	// Initial commit and push to main
	if err := os.WriteFile(filepath.Join(localDir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runGitInDir(t, localDir, "add", ".")
	runGitInDir(t, localDir, "commit", "-m", "Initial commit")
	runGitInDir(t, localDir, "push", "-u", "origin", "main")

	// Create task branch with a change
	runGitInDir(t, localDir, "checkout", "-b", "orc/TASK-001")
	if err := os.WriteFile(filepath.Join(localDir, "feature.go"), []byte("package feature\n"), 0644); err != nil {
		t.Fatalf("write feature.go: %v", err)
	}
	runGitInDir(t, localDir, "add", ".")
	runGitInDir(t, localDir, "commit", "-m", "[orc] TASK-001: implement feature")

	mainGitOps, err := git.New(localDir, git.DefaultConfig())
	if err != nil {
		t.Fatalf("git.New: %v", err)
	}
	// Use InWorktree so push safety checks pass (PushWithForceFallback requires worktree context)
	gitOps := mainGitOps.InWorktree(localDir)

	backend := storage.NewTestBackend(t)
	mock := &prTestProvider{}

	tsk := task.NewProtoTask("TASK-001", "Fix authentication bug")
	tsk.Branch = "orc/TASK-001"
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Capture log output for SC-5 assertions
	var logBuf bytes.Buffer
	handler := slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)

	cfg := &config.Config{
		Completion: config.CompletionConfig{
			Action:       "pr",
			TargetBranch: "main",
		},
	}

	we := &WorkflowExecutor{
		backend:      backend,
		orcConfig:    cfg,
		logger:       logger,
		workingDir:   localDir,
		worktreePath: localDir,
		// hostingProvider is the injected mock — this field does not exist yet.
		// The implementation must add it so getHostingProvider() returns it
		// instead of creating a real provider from config.
		hostingProvider: mock,
	}

	return &createPRTestEnv{
		we:      we,
		mock:    mock,
		task:    tsk,
		gitOps:  gitOps,
		logBuf:  &logBuf,
		backend: backend,
	}
}

// runGitInDir runs a git command in the given directory, failing the test on error.
func runGitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s (in %s) failed: %v\n%s", strings.Join(args, " "), dir, err, out)
	}
}

// =============================================================================
// Tests for SC-1: Reuse existing PR when FindPRByBranch returns one
// =============================================================================

// TestCreatePR_ReusesExistingPR verifies that when an open PR exists on the
// task branch, the executor reuses it by calling UpdatePR and saving the PR
// info to task metadata, WITHOUT calling CreatePR.
// Covers: SC-1, SC-5
func TestCreatePR_ReusesExistingPR(t *testing.T) {
	t.Parallel()
	env := setupCreatePRTest(t)

	existingPR := &hosting.PR{
		Number:     42,
		HTMLURL:    "https://github.com/owner/repo/pull/42",
		Title:      "[orc] TASK-001: old title",
		State:      "open",
		HeadBranch: "orc/TASK-001",
	}
	env.mock.findPRByBranchFunc = func(_ context.Context, branch string) (*hosting.PR, error) {
		if branch == "orc/TASK-001" {
			return existingPR, nil
		}
		return nil, hosting.ErrNoPRFound
	}

	err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
	if err != nil {
		t.Fatalf("createPR() error: %v", err)
	}

	// FindPRByBranch must have been called with the task branch
	if len(env.mock.findPRByBranchCalls) == 0 {
		t.Fatal("FindPRByBranch was not called")
	}
	if env.mock.findPRByBranchCalls[0] != "orc/TASK-001" {
		t.Errorf("FindPRByBranch branch = %q, want %q",
			env.mock.findPRByBranchCalls[0], "orc/TASK-001")
	}

	// CreatePR must NOT have been called
	if len(env.mock.createPRCalls) > 0 {
		t.Error("CreatePR was called when existing PR should have been reused")
	}

	// UpdatePR must have been called on PR #42
	if len(env.mock.updatePRCalls) == 0 {
		t.Fatal("UpdatePR was not called for existing PR")
	}
	if env.mock.updatePRCalls[0].Number != 42 {
		t.Errorf("UpdatePR number = %d, want 42", env.mock.updatePRCalls[0].Number)
	}

	// Task must have PR info saved
	if !task.HasPRProto(env.task) {
		t.Fatal("task does not have PR info after reuse")
	}
	if got := task.GetPRURLProto(env.task); got != "https://github.com/owner/repo/pull/42" {
		t.Errorf("PR URL = %q, want %q", got, "https://github.com/owner/repo/pull/42")
	}

	// SC-5: Log should mention reusing
	logOutput := env.logBuf.String()
	if !strings.Contains(strings.ToLower(logOutput), "reus") {
		t.Errorf("log should mention reusing PR, got:\n%s", logOutput)
	}
}

// =============================================================================
// Tests for SC-2: Create new PR when no existing PR found
// =============================================================================

// TestCreatePR_CreatesNewWhenNoneExists verifies that when FindPRByBranch
// returns ErrNoPRFound, the executor creates a new PR as before.
// Covers: SC-2, SC-5
func TestCreatePR_CreatesNewWhenNoneExists(t *testing.T) {
	t.Parallel()
	env := setupCreatePRTest(t)

	// Default mock returns ErrNoPRFound for FindPRByBranch (no func override needed)

	err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
	if err != nil {
		t.Fatalf("createPR() error: %v", err)
	}

	// FindPRByBranch should have been called
	if len(env.mock.findPRByBranchCalls) == 0 {
		t.Fatal("FindPRByBranch was not called")
	}

	// CreatePR must have been called (since no existing PR)
	if len(env.mock.createPRCalls) == 0 {
		t.Fatal("CreatePR was not called when no existing PR exists")
	}

	// Verify CreatePR was called with correct options
	opts := env.mock.createPRCalls[0]
	expectedTitle := "[orc] TASK-001: Fix authentication bug"
	if opts.Title != expectedTitle {
		t.Errorf("CreatePR title = %q, want %q", opts.Title, expectedTitle)
	}
	if opts.Head != "orc/TASK-001" {
		t.Errorf("CreatePR head = %q, want %q", opts.Head, "orc/TASK-001")
	}
	if opts.Base != "main" {
		t.Errorf("CreatePR base = %q, want %q", opts.Base, "main")
	}

	// Task must have PR info saved
	if !task.HasPRProto(env.task) {
		t.Fatal("task does not have PR info after creation")
	}

	// SC-5: Log should mention creating (not reusing)
	logOutput := env.logBuf.String()
	if !strings.Contains(strings.ToLower(logOutput), "creat") {
		t.Errorf("log should mention creating PR, got:\n%s", logOutput)
	}
}

// =============================================================================
// Tests for SC-3: Skip when task already has PR metadata
// =============================================================================

// TestCreatePR_SkipsWhenAlreadyHasPR verifies that when HasPRProto returns
// true, the executor returns immediately without calling any provider methods.
// Covers: SC-3, SC-5
func TestCreatePR_SkipsWhenAlreadyHasPR(t *testing.T) {
	t.Parallel()
	env := setupCreatePRTest(t)

	// Set PR info on task (simulates previous successful PR creation)
	task.SetPRInfoProto(env.task, "https://github.com/owner/repo/pull/50", 50)

	err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
	if err != nil {
		t.Fatalf("createPR() error: %v", err)
	}

	// No provider methods should have been called
	if len(env.mock.findPRByBranchCalls) > 0 {
		t.Error("FindPRByBranch was called when task already has PR metadata")
	}
	if len(env.mock.createPRCalls) > 0 {
		t.Error("CreatePR was called when task already has PR metadata")
	}
	if len(env.mock.updatePRCalls) > 0 {
		t.Error("UpdatePR was called when task already has PR metadata")
	}

	// SC-5: Log should mention PR already exists
	logOutput := env.logBuf.String()
	if !strings.Contains(strings.ToLower(logOutput), "already") {
		t.Errorf("log should mention PR already exists, got:\n%s", logOutput)
	}
}

// =============================================================================
// Tests for SC-4: Reused PR gets correct title/body and auto-merge/approve
// =============================================================================

// TestCreatePR_UpdatesReusedPR verifies that when an existing PR is reused,
// its title and body are updated to match the current task, and auto-merge/approve
// settings are applied if configured.
// Covers: SC-4
func TestCreatePR_UpdatesReusedPR(t *testing.T) {
	t.Parallel()
	env := setupCreatePRTest(t)

	// Enable auto-merge and auto-approve
	env.we.orcConfig.Completion.PR = config.PRConfig{
		AutoMerge:  true,
		AutoApprove: true,
	}
	env.we.orcConfig.Completion.CI.MergeMethod = "squash"

	existingPR := &hosting.PR{
		Number:     42,
		HTMLURL:    "https://github.com/owner/repo/pull/42",
		Title:      "[orc] TASK-001: old stale title",
		State:      "open",
		HeadBranch: "orc/TASK-001",
	}
	env.mock.findPRByBranchFunc = func(_ context.Context, _ string) (*hosting.PR, error) {
		return existingPR, nil
	}

	err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
	if err != nil {
		t.Fatalf("createPR() error: %v", err)
	}

	// Verify UpdatePR was called with correct title/body format
	if len(env.mock.updatePRCalls) == 0 {
		t.Fatal("UpdatePR was not called")
	}
	update := env.mock.updatePRCalls[0]

	expectedTitle := "[orc] TASK-001: Fix authentication bug"
	if update.Opts.Title != expectedTitle {
		t.Errorf("UpdatePR title = %q, want %q", update.Opts.Title, expectedTitle)
	}

	// Body should contain task info
	if !strings.Contains(update.Opts.Body, "Fix authentication bug") {
		t.Errorf("UpdatePR body should contain task title, got: %q", update.Opts.Body)
	}

	// Auto-merge should have been attempted on PR #42
	if len(env.mock.enableAutoMergeCalls) == 0 {
		t.Error("EnableAutoMerge was not called for reused PR")
	} else {
		if env.mock.enableAutoMergeCalls[0].Number != 42 {
			t.Errorf("EnableAutoMerge PR number = %d, want 42",
				env.mock.enableAutoMergeCalls[0].Number)
		}
		if env.mock.enableAutoMergeCalls[0].Method != "squash" {
			t.Errorf("EnableAutoMerge method = %q, want %q",
				env.mock.enableAutoMergeCalls[0].Method, "squash")
		}
	}

	// Auto-approve should have been attempted on PR #42
	if len(env.mock.approvePRCalls) == 0 {
		t.Error("ApprovePR was not called for reused PR")
	} else if env.mock.approvePRCalls[0].Number != 42 {
		t.Errorf("ApprovePR PR number = %d, want 42",
			env.mock.approvePRCalls[0].Number)
	}
}

// TestCreatePR_UpdatesReusedPR_NoAutoMergeWhenDisabled verifies that auto-merge
// and auto-approve are NOT applied to reused PRs when not configured.
// Covers: SC-4 (preservation of config-driven behavior)
func TestCreatePR_UpdatesReusedPR_NoAutoMergeWhenDisabled(t *testing.T) {
	t.Parallel()
	env := setupCreatePRTest(t)

	// Auto-merge and auto-approve are disabled by default (zero values)
	existingPR := &hosting.PR{
		Number:     42,
		HTMLURL:    "https://github.com/owner/repo/pull/42",
		State:      "open",
		HeadBranch: "orc/TASK-001",
	}
	env.mock.findPRByBranchFunc = func(_ context.Context, _ string) (*hosting.PR, error) {
		return existingPR, nil
	}

	err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
	if err != nil {
		t.Fatalf("createPR() error: %v", err)
	}

	// Auto-merge should NOT have been called
	if len(env.mock.enableAutoMergeCalls) > 0 {
		t.Error("EnableAutoMerge was called when auto-merge is disabled")
	}

	// Auto-approve should NOT have been called
	if len(env.mock.approvePRCalls) > 0 {
		t.Error("ApprovePR was called when auto-approve is disabled")
	}
}

// =============================================================================
// Tests for failure modes
// =============================================================================

// TestCreatePR_FindPRByBranchError verifies that when FindPRByBranch returns
// a network error (not ErrNoPRFound), the executor falls through to CreatePR
// as a best-effort approach (no regression from current behavior).
// Covers: SC-1 error path, failure mode
func TestCreatePR_FindPRByBranchError(t *testing.T) {
	t.Parallel()
	env := setupCreatePRTest(t)

	env.mock.findPRByBranchFunc = func(_ context.Context, _ string) (*hosting.PR, error) {
		return nil, fmt.Errorf("network timeout")
	}

	err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
	if err != nil {
		t.Fatalf("createPR() error: %v", err)
	}

	// FindPRByBranch was called (and failed)
	if len(env.mock.findPRByBranchCalls) == 0 {
		t.Fatal("FindPRByBranch was not called")
	}

	// CreatePR should have been called as fallthrough
	if len(env.mock.createPRCalls) == 0 {
		t.Fatal("CreatePR was not called after FindPRByBranch error — should fall through")
	}

	// Task should still get PR info from the created PR
	if !task.HasPRProto(env.task) {
		t.Fatal("task does not have PR info after fallthrough creation")
	}

	// Log should warn about the FindPRByBranch failure
	logOutput := env.logBuf.String()
	if !strings.Contains(strings.ToLower(logOutput), "warn") ||
		!strings.Contains(logOutput, "network timeout") {
		t.Errorf("log should warn about FindPRByBranch error, got:\n%s", logOutput)
	}
}

// TestCreatePR_UpdatePRError verifies that when UpdatePR fails after finding
// an existing PR, the PR info is still saved to task (the PR exists, just
// with stale title/body). No error is returned.
// Covers: SC-4 error path, failure mode
func TestCreatePR_UpdatePRError(t *testing.T) {
	t.Parallel()
	env := setupCreatePRTest(t)

	existingPR := &hosting.PR{
		Number:     42,
		HTMLURL:    "https://github.com/owner/repo/pull/42",
		State:      "open",
		HeadBranch: "orc/TASK-001",
	}
	env.mock.findPRByBranchFunc = func(_ context.Context, _ string) (*hosting.PR, error) {
		return existingPR, nil
	}
	env.mock.updatePRFunc = func(_ context.Context, _ int, _ hosting.PRUpdateOptions) error {
		return fmt.Errorf("API rate limited")
	}

	err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
	if err != nil {
		t.Fatalf("createPR() should not return error when UpdatePR fails, got: %v", err)
	}

	// PR info should still be saved despite UpdatePR failure
	if !task.HasPRProto(env.task) {
		t.Fatal("task does not have PR info — should be saved even when UpdatePR fails")
	}
	if got := task.GetPRURLProto(env.task); got != "https://github.com/owner/repo/pull/42" {
		t.Errorf("PR URL = %q, want %q", got, "https://github.com/owner/repo/pull/42")
	}

	// CreatePR should NOT have been called (we found the existing PR)
	if len(env.mock.createPRCalls) > 0 {
		t.Error("CreatePR should not be called when existing PR was found (even if UpdatePR fails)")
	}

	// Log should warn about UpdatePR failure
	logOutput := env.logBuf.String()
	if !strings.Contains(logOutput, "rate limited") {
		t.Errorf("log should warn about UpdatePR error, got:\n%s", logOutput)
	}
}

// =============================================================================
// Tests for edge cases
// =============================================================================

// TestCreatePR_TaskHasMetadataAndDifferentPRExists verifies that when task
// already has PR metadata, the fast-path returns immediately — even if a
// different PR exists on the branch (no provider calls).
// Covers: SC-3, edge case
func TestCreatePR_TaskHasMetadataAndDifferentPRExists(t *testing.T) {
	t.Parallel()
	env := setupCreatePRTest(t)

	// Task has metadata for PR #50
	task.SetPRInfoProto(env.task, "https://github.com/owner/repo/pull/50", 50)

	// A different PR #99 exists on the branch (shouldn't matter)
	env.mock.findPRByBranchFunc = func(_ context.Context, _ string) (*hosting.PR, error) {
		return &hosting.PR{Number: 99, HTMLURL: "https://github.com/owner/repo/pull/99"}, nil
	}

	err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
	if err != nil {
		t.Fatalf("createPR() error: %v", err)
	}

	// Fast-path: no provider calls at all
	if len(env.mock.findPRByBranchCalls) > 0 {
		t.Error("FindPRByBranch was called — fast-path (HasPRProto) should prevent this")
	}

	// Task PR info should still be PR #50 (unchanged)
	if got := task.GetPRURLProto(env.task); got != "https://github.com/owner/repo/pull/50" {
		t.Errorf("PR URL changed to %q, should remain %q",
			got, "https://github.com/owner/repo/pull/50")
	}
}

// TestCreatePR_SavesTaskToBackend verifies that after PR creation or reuse,
// the task is saved to the backend with updated PR info.
// Covers: SC-1, SC-2 (persistence)
func TestCreatePR_SavesTaskToBackend(t *testing.T) {
	t.Parallel()

	t.Run("saves after reuse", func(t *testing.T) {
		t.Parallel()
		env := setupCreatePRTest(t)

		env.mock.findPRByBranchFunc = func(_ context.Context, _ string) (*hosting.PR, error) {
			return &hosting.PR{
				Number:  42,
				HTMLURL: "https://github.com/owner/repo/pull/42",
				State:   "open",
			}, nil
		}

		err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
		if err != nil {
			t.Fatalf("createPR() error: %v", err)
		}

		// Reload task from backend to verify persistence
		loaded, err := env.backend.LoadTask(env.task.Id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if !task.HasPRProto(loaded) {
			t.Fatal("reloaded task does not have PR info — SaveTask was not called")
		}
	})

	t.Run("saves after creation", func(t *testing.T) {
		t.Parallel()
		env := setupCreatePRTest(t)
		// Default mock: FindPRByBranch returns ErrNoPRFound, CreatePR returns PR #99

		err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
		if err != nil {
			t.Fatalf("createPR() error: %v", err)
		}

		// Reload task from backend
		loaded, err := env.backend.LoadTask(env.task.Id)
		if err != nil {
			t.Fatalf("GetTask() error: %v", err)
		}
		if !task.HasPRProto(loaded) {
			t.Fatal("reloaded task does not have PR info — SaveTask was not called")
		}
	})
}

// TestCreatePR_PRTitleFormat verifies the exact PR title format used for both
// creation and reuse paths.
// Covers: SC-4, preservation requirement
func TestCreatePR_PRTitleFormat(t *testing.T) {
	t.Parallel()

	t.Run("new PR title matches format", func(t *testing.T) {
		t.Parallel()
		env := setupCreatePRTest(t)
		// Default: no existing PR

		err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
		if err != nil {
			t.Fatalf("createPR() error: %v", err)
		}

		if len(env.mock.createPRCalls) == 0 {
			t.Fatal("CreatePR was not called")
		}
		expectedTitle := fmt.Sprintf("[orc] %s: %s", env.task.Id, env.task.Title)
		if got := env.mock.createPRCalls[0].Title; got != expectedTitle {
			t.Errorf("CreatePR title = %q, want %q", got, expectedTitle)
		}
	})

	t.Run("reused PR title matches format", func(t *testing.T) {
		t.Parallel()
		env := setupCreatePRTest(t)

		env.mock.findPRByBranchFunc = func(_ context.Context, _ string) (*hosting.PR, error) {
			return &hosting.PR{
				Number:  42,
				HTMLURL: "https://github.com/owner/repo/pull/42",
				State:   "open",
			}, nil
		}

		err := env.we.createPR(context.Background(), env.task, env.gitOps, "main")
		if err != nil {
			t.Fatalf("createPR() error: %v", err)
		}

		if len(env.mock.updatePRCalls) == 0 {
			t.Fatal("UpdatePR was not called")
		}
		expectedTitle := fmt.Sprintf("[orc] %s: %s", env.task.Id, env.task.Title)
		if got := env.mock.updatePRCalls[0].Opts.Title; got != expectedTitle {
			t.Errorf("UpdatePR title = %q, want %q", got, expectedTitle)
		}
	})
}
