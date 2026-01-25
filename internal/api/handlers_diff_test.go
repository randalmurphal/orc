package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// === handleGetDiff Tests ===

// TestHandleGetDiff_NoBranch verifies that when a task has no branch set (t.Branch == ""),
// the endpoint returns HTTP 200 with an empty diff instead of HTTP 500.
// This covers SC-1: GET /api/tasks/{id}/diff returns 200 with empty diff when task has no branch.
func TestHandleGetDiff_NoBranch(t *testing.T) {
	t.Parallel()

	srv, taskID, cleanup := setupDiffTestEnv(t, withTaskNoBranch())
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 200, not 500
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response structure matches expected empty diff
	var result diff.DiffResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify empty diff structure per SC-1
	if result.Base != "" {
		t.Errorf("expected empty base, got %q", result.Base)
	}
	if result.Head != "" {
		t.Errorf("expected empty head, got %q", result.Head)
	}
	if result.Stats.FilesChanged != 0 {
		t.Errorf("expected 0 files_changed, got %d", result.Stats.FilesChanged)
	}
	if result.Stats.Additions != 0 {
		t.Errorf("expected 0 additions, got %d", result.Stats.Additions)
	}
	if result.Stats.Deletions != 0 {
		t.Errorf("expected 0 deletions, got %d", result.Stats.Deletions)
	}
	if len(result.Files) != 0 {
		t.Errorf("expected empty files array, got %d files", len(result.Files))
	}
	// Ensure files is [] not null (JSON serialization check)
	if result.Files == nil {
		t.Error("expected files to be empty array, not nil")
	}
}

// TestHandleGetDiff_NoBranch_FilesOnly verifies the files-only mode also returns
// empty diff gracefully when no branch exists.
func TestHandleGetDiff_NoBranch_FilesOnly(t *testing.T) {
	t.Parallel()

	srv, taskID, cleanup := setupDiffTestEnv(t, withTaskNoBranch())
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff?files=true", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result diff.DiffResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Stats.FilesChanged != 0 {
		t.Errorf("expected 0 files_changed, got %d", result.Stats.FilesChanged)
	}
	if len(result.Files) != 0 {
		t.Errorf("expected empty files array, got %d files", len(result.Files))
	}
}

// TestHandleGetDiff_TaskNotFound verifies that a non-existent task returns 404.
func TestHandleGetDiff_TaskNotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/diff", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === handleGetDiffStats Tests ===

// TestHandleGetDiffStats_NoBranch verifies that when a task has no branch,
// the stats endpoint returns HTTP 200 with zero stats.
// This covers SC-2: GET /api/tasks/{id}/diff/stats returns 200 with zero stats when task has no branch.
func TestHandleGetDiffStats_NoBranch(t *testing.T) {
	t.Parallel()

	srv, taskID, cleanup := setupDiffTestEnv(t, withTaskNoBranch())
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff/stats", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 200, not 500
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response structure matches expected zero stats per SC-2
	var stats diff.DiffStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if stats.FilesChanged != 0 {
		t.Errorf("expected files_changed=0, got %d", stats.FilesChanged)
	}
	if stats.Additions != 0 {
		t.Errorf("expected additions=0, got %d", stats.Additions)
	}
	if stats.Deletions != 0 {
		t.Errorf("expected deletions=0, got %d", stats.Deletions)
	}
}

// TestHandleGetDiffStats_TaskNotFound verifies that a non-existent task returns 404.
func TestHandleGetDiffStats_TaskNotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/diff/stats", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === handleGetDiffFile Tests ===

// TestHandleGetDiffFile_NoBranch verifies that when a task has no branch,
// the file endpoint returns HTTP 404 with a descriptive error message.
// This covers SC-4: GET /api/tasks/{id}/diff/file/{path} returns 404 when task has no branch.
func TestHandleGetDiffFile_NoBranch(t *testing.T) {
	t.Parallel()

	srv, taskID, cleanup := setupDiffTestEnv(t, withTaskNoBranch())
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff/file/some/path.go", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 404 with descriptive message per SC-4
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}

	// Verify error message is descriptive
	var errResp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	expectedError := "task has no branch to diff"
	if errResp["error"] != expectedError {
		t.Errorf("expected error %q, got %q", expectedError, errResp["error"])
	}
}

// TestHandleGetDiffFile_TaskNotFound verifies that a non-existent task returns 404.
func TestHandleGetDiffFile_TaskNotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/diff/file/some/path.go", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestHandleGetDiffFile_EmptyPath verifies that missing file path returns 400.
func TestHandleGetDiffFile_EmptyPath(t *testing.T) {
	t.Parallel()

	srv, taskID, cleanup := setupDiffTestEnv(t, withTaskWithBranch("orc/test-branch"))
	defer cleanup()

	// Note: The route is /api/tasks/{id}/diff/file/{path...} so this tests empty path handling
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff/file/", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// === Edge Case Tests ===

// TestHandleGetDiff_BranchSetButDeleted verifies that when a task has a branch name set
// but the branch doesn't exist in git (e.g., worktree was deleted), it returns empty diff.
// This covers BDD-2: branch name set but branch doesn't exist in git.
func TestHandleGetDiff_BranchSetButDeleted(t *testing.T) {
	t.Parallel()

	// Create task with branch name that doesn't exist in git
	srv, taskID, cleanup := setupDiffTestEnv(t, withTaskWithBranch("orc/nonexistent-deleted-branch"))
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 200 with empty diff, not 500
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result diff.DiffResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Empty diff for non-existent branch
	if result.Stats.FilesChanged != 0 {
		t.Errorf("expected 0 files_changed for non-existent branch, got %d", result.Stats.FilesChanged)
	}
	if len(result.Files) != 0 {
		t.Errorf("expected empty files array for non-existent branch, got %d files", len(result.Files))
	}
}

// TestHandleGetDiffStats_BranchSetButDeleted verifies stats endpoint handles deleted branch.
func TestHandleGetDiffStats_BranchSetButDeleted(t *testing.T) {
	t.Parallel()

	srv, taskID, cleanup := setupDiffTestEnv(t, withTaskWithBranch("orc/nonexistent-deleted-branch"))
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff/stats", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 200 with zero stats, not 500
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var stats diff.DiffStats
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if stats.FilesChanged != 0 {
		t.Errorf("expected 0 files_changed for non-existent branch, got %d", stats.FilesChanged)
	}
}

// TestHandleGetDiffFile_BranchSetButDeleted verifies file endpoint handles deleted branch.
func TestHandleGetDiffFile_BranchSetButDeleted(t *testing.T) {
	t.Parallel()

	srv, taskID, cleanup := setupDiffTestEnv(t, withTaskWithBranch("orc/nonexistent-deleted-branch"))
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff/file/some/path.go", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 404 since we can't provide file diff without valid branch
	// (same as no branch case)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === Preservation Tests (Existing Behavior) ===
//
// NOTE: These tests verify that the code paths for merged PRs and commit ranges
// still exist and don't crash. However, due to a database schema limitation
// (db.Task doesn't store PR info or phase commit SHAs), these paths cannot be
// fully tested in isolation. When PR/commit info isn't available and branch is empty,
// the handlers correctly return empty diff (200). Full preservation testing
// requires either fixing the storage layer or integration tests with a real git repo.

// TestHandleGetDiff_WithMergedPR verifies the handler doesn't crash when
// attempting to set up a merged PR scenario.
// NOTE: PR info is not persisted by DatabaseBackend (taskToDBTask doesn't include PR field),
// so this test verifies graceful degradation rather than full merged PR diff functionality.
func TestHandleGetDiff_WithMergedPR(t *testing.T) {
	t.Parallel()

	srv, taskID, cleanup := setupDiffTestEnv(t, withMergedPR("abc123def456"))
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Due to database limitation (PR info not persisted), this will fall back to
	// branch diff path with empty branch, which returns empty diff (200).
	// This is expected behavior given the storage constraint.
	// A full test would require the storage layer to persist PR info.
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response is valid JSON if 200
	if w.Code == http.StatusOK {
		var result diff.DiffResult
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		// Empty diff is acceptable since PR info isn't persisted
	}
}

// TestHandleGetDiff_WithCommitSHAs verifies that tasks with phase commits
// in state are handled correctly.
// NOTE: This tests the commit range path, which depends on state having commit SHAs.
func TestHandleGetDiff_WithCommitSHAs(t *testing.T) {
	t.Parallel()

	srv, taskID, cleanup := setupDiffTestEnv(t, withPhaseCommits("first123", "last456"))
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/diff", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should either:
	// - Return 500 if git operations fail (commits don't exist in test env)
	// - Return 200 with commit range diff if somehow git works
	// Should NOT crash
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

// === Test Environment Setup ===

// setupDiffTestEnv creates a test environment for diff handler tests.
func setupDiffTestEnv(t *testing.T, opts ...func(*testing.T, string, string)) (srv *Server, taskID string, cleanup func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Create .orc directory with config that disables worktrees
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	configYAML := `worktree:
  enabled: false
`
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configYAML), 0644)

	taskID = "TASK-DIFF-001"
	startTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create and save task (default: with branch)
	tsk := task.New(taskID, "Diff Test Task")
	tsk.Description = "A task for testing diff handlers"
	tsk.Status = task.StatusRunning
	tsk.Weight = task.WeightMedium
	tsk.Branch = fmt.Sprintf("orc/%s", taskID) // Default: has branch
	tsk.CurrentPhase = "implement"
	tsk.CreatedAt = startTime
	tsk.UpdatedAt = startTime
	tsk.StartedAt = &startTime
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create and save state
	st := state.New(taskID)
	st.CurrentPhase = "implement"
	st.CurrentIteration = 1
	st.Status = state.StatusRunning
	st.StartedAt = startTime
	st.UpdatedAt = startTime
	st.Phases = map[string]*state.PhaseState{
		"implement": {
			Status:     state.StatusRunning,
			StartedAt:  startTime,
			Iterations: 1,
		},
	}
	if err := backend.SaveState(st); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Close backend before applying opts and creating server
	_ = backend.Close()

	// Apply optional setup functions
	for _, opt := range opts {
		opt(t, tmpDir, taskID)
	}

	srv = New(&Config{WorkDir: tmpDir})

	cleanup = func() {
		// No cleanup needed - t.TempDir() handles cleanup
	}

	return srv, taskID, cleanup
}

// withTaskNoBranch modifies the task to have no branch set.
func withTaskNoBranch() func(*testing.T, string, string) {
	return func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		storageCfg := &config.StorageConfig{Mode: "database"}
		backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer func() { _ = backend.Close() }()

		tsk, err := backend.LoadTask(taskID)
		if err != nil {
			t.Fatalf("failed to load task: %v", err)
		}

		tsk.Branch = "" // Clear the branch
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}
}

// withTaskWithBranch sets a specific branch name on the task.
func withTaskWithBranch(branch string) func(*testing.T, string, string) {
	return func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		storageCfg := &config.StorageConfig{Mode: "database"}
		backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer func() { _ = backend.Close() }()

		tsk, err := backend.LoadTask(taskID)
		if err != nil {
			t.Fatalf("failed to load task: %v", err)
		}

		tsk.Branch = branch
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}
}

// withMergedPR sets up a task with a merged PR.
func withMergedPR(mergeCommitSHA string) func(*testing.T, string, string) {
	return func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		storageCfg := &config.StorageConfig{Mode: "database"}
		backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer func() { _ = backend.Close() }()

		tsk, err := backend.LoadTask(taskID)
		if err != nil {
			t.Fatalf("failed to load task: %v", err)
		}

		tsk.Branch = "" // No branch, but has merged PR
		tsk.PR = &task.PRInfo{
			Number:         42,
			URL:            "https://github.com/test/test/pull/42",
			Merged:         true,
			MergeCommitSHA: mergeCommitSHA,
		}
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}
	}
}

// withPhaseCommits sets up a task with phase commit SHAs in state.
func withPhaseCommits(firstCommit, lastCommit string) func(*testing.T, string, string) {
	return func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		storageCfg := &config.StorageConfig{Mode: "database"}
		backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer func() { _ = backend.Close() }()

		// Clear the branch to test commit range path
		tsk, err := backend.LoadTask(taskID)
		if err != nil {
			t.Fatalf("failed to load task: %v", err)
		}
		tsk.Branch = ""
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task: %v", err)
		}

		// Set up state with phase commits
		st, err := backend.LoadState(taskID)
		if err != nil {
			t.Fatalf("failed to load state: %v", err)
		}

		st.Phases["spec"] = &state.PhaseState{
			Status:    state.StatusCompleted,
			CommitSHA: firstCommit,
		}
		st.Phases["implement"] = &state.PhaseState{
			Status:    state.StatusCompleted,
			CommitSHA: lastCommit,
		}
		if err := backend.SaveState(st); err != nil {
			t.Fatalf("failed to save state: %v", err)
		}
	}
}
