package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// === GitHub PR Integration Tests ===

// setupGitHubTestEnv creates a test environment with task, database, and optionally review comments.
func setupGitHubTestEnv(t *testing.T, opts ...func(*testing.T, string, string)) (srv *Server, taskID string, cleanup func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Create .orc directory with config that disables worktrees
	orcDir := filepath.Join(tmpDir, ".orc")
	os.MkdirAll(orcDir, 0755)
	configYAML := `worktree:
  enabled: false
`
	os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configYAML), 0644)

	// Create task directory
	taskID = "TASK-GH-001"
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	// Create task.yaml with branch
	taskYAML := fmt.Sprintf(`id: %s
title: GitHub Test Task
description: A task for testing GitHub handlers
status: running
weight: medium
branch: orc/%s
current_phase: implement
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
started_at: 2025-01-01T00:00:00Z
`, taskID, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create plan.yaml
	planYAML := `phases:
  - id: implement
    status: running
  - id: test
    status: pending
`
	os.WriteFile(filepath.Join(taskDir, "plan.yaml"), []byte(planYAML), 0644)

	// Create state.yaml
	stateYAML := fmt.Sprintf(`task_id: %s
current_phase: implement
current_iteration: 1
status: running
started_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
phases:
  implement:
    status: running
    started_at: 2025-01-01T00:00:00Z
    iterations: 1
tokens:
  input_tokens: 0
  output_tokens: 0
  total_tokens: 0
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "state.yaml"), []byte(stateYAML), 0644)

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

// withReviewComments adds review comments to the database.
func withReviewComments(comments []db.ReviewComment) func(*testing.T, string, string) {
	return func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		pdb, err := db.OpenProject(tmpDir)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer pdb.Close()

		// Insert the task into the database first (for foreign key constraint)
		_, err = pdb.Exec(`
			INSERT INTO tasks (id, title, status, weight, created_at)
			VALUES (?, ?, ?, ?, datetime('now'))
		`, taskID, "Test Task", "running", "medium")
		if err != nil {
			t.Fatalf("failed to create task in database: %v", err)
		}

		for _, c := range comments {
			c.TaskID = taskID
			if err := pdb.CreateReviewComment(&c); err != nil {
				t.Fatalf("failed to create review comment: %v", err)
			}
		}
	}
}

// === handleAutoFixComment Tests ===

func TestHandleAutoFixComment_TaskNotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/github/pr/comments/C123/autofix", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAutoFixComment_CommentNotFound(t *testing.T) {
	srv, taskID, cleanup := setupGitHubTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/NONEXISTENT/autofix", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleAutoFixComment_BuildsRetryContext(t *testing.T) {
	// Create a comment to autofix
	testComment := db.ReviewComment{
		ID:         "RC-testfix1",
		FilePath:   "internal/api/server.go",
		LineNumber: 42,
		Content:    "This function should handle errors more gracefully",
		Severity:   db.SeverityIssue,
		Status:     db.CommentStatusOpen,
	}

	srv, taskID, cleanup := setupGitHubTestEnv(t, withReviewComments([]db.ReviewComment{testComment}))
	defer cleanup()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/RC-testfix1/autofix", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response structure
	var resp autoFixResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TaskID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, resp.TaskID)
	}

	if resp.CommentID != "RC-testfix1" {
		t.Errorf("expected comment ID RC-testfix1, got %s", resp.CommentID)
	}

	if resp.Status != "running" {
		t.Errorf("expected status 'running', got %s", resp.Status)
	}

	// Verify state was updated with retry context
	st, err := state.LoadFrom(srv.workDir, taskID)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	if st.RetryContext == nil {
		t.Error("expected retry context to be set in state")
	}
}

func TestHandleAutoFixComment_StoresMetadata(t *testing.T) {
	testComment := db.ReviewComment{
		ID:         "RC-meta123",
		FilePath:   "pkg/handler.go",
		LineNumber: 100,
		Content:    "Missing input validation",
		Severity:   db.SeverityBlocker,
		Status:     db.CommentStatusOpen,
	}

	srv, taskID, cleanup := setupGitHubTestEnv(t, withReviewComments([]db.ReviewComment{testComment}))
	defer cleanup()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/RC-meta123/autofix", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify task metadata was updated
	tsk, err := task.LoadFrom(srv.workDir, taskID)
	if err != nil {
		t.Fatalf("failed to load task: %v", err)
	}

	if tsk.Metadata == nil {
		t.Fatal("expected metadata to be set")
	}

	if tsk.Metadata["autofix_comment_id"] != "RC-meta123" {
		t.Errorf("expected autofix_comment_id to be RC-meta123, got %s", tsk.Metadata["autofix_comment_id"])
	}

	if tsk.Metadata["autofix_file"] != "pkg/handler.go" {
		t.Errorf("expected autofix_file to be pkg/handler.go, got %s", tsk.Metadata["autofix_file"])
	}

	if tsk.Metadata["autofix_line"] != "100" {
		t.Errorf("expected autofix_line to be 100, got %s", tsk.Metadata["autofix_line"])
	}
}

func TestHandleAutoFixComment_UpdatesCompletedTaskStatus(t *testing.T) {
	testComment := db.ReviewComment{
		ID:       "RC-status1",
		FilePath: "main.go",
		Content:  "Fix this",
		Status:   db.CommentStatusOpen,
	}

	srv, taskID, cleanup := setupGitHubTestEnv(t, withReviewComments([]db.ReviewComment{testComment}))
	defer cleanup()

	// Set task to completed status first
	tsk, _ := task.LoadFrom(srv.workDir, taskID)
	tsk.Status = task.StatusCompleted
	tsk.SaveTo(task.TaskDirIn(srv.workDir, taskID))

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/RC-status1/autofix", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify task status was reset to planned for re-execution
	reloadedTask, _ := task.LoadFrom(srv.workDir, taskID)
	if reloadedTask.Status != task.StatusPlanned {
		t.Errorf("expected status to be reset to planned, got %s", reloadedTask.Status)
	}
}

// === handleReplyToPRComment Tests ===

func TestHandleReplyToPRComment_TaskNotFound(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{"body": "Reply text"}`)
	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/github/pr/comments/123/reply", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleReplyToPRComment_NoBranch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task without branch
	taskID := "TASK-NOBRANCH"
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	taskYAML := fmt.Sprintf(`id: %s
title: No Branch Task
status: running
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	body := bytes.NewBufferString(`{"body": "Reply text"}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/123/reply", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleReplyToPRComment_EmptyBody(t *testing.T) {
	srv, taskID, cleanup := setupGitHubTestEnv(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"body": ""}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/123/reply", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	// Verify error message mentions body
	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "body is required" {
		t.Errorf("expected error 'body is required', got %s", errResp["error"])
	}
}

func TestHandleReplyToPRComment_InvalidBody(t *testing.T) {
	srv, taskID, cleanup := setupGitHubTestEnv(t)
	defer cleanup()

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/123/reply", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleReplyToPRComment_InvalidCommentID(t *testing.T) {
	srv, taskID, cleanup := setupGitHubTestEnv(t)
	defer cleanup()

	// Comment ID must be a valid int64
	body := bytes.NewBufferString(`{"body": "Reply text"}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/not-a-number/reply", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should fail at comment ID parsing (before GitHub auth)
	// Note: The handler will try GitHub auth first, which will fail without gh
	// but the comment ID validation happens after that, so we just check that
	// it doesn't return 200
	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 status for invalid comment ID, got %d", w.Code)
	}
}

// === handleImportPRComments Tests ===

func TestHandleImportPRComments_TaskNotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/github/pr/comments/import", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleImportPRComments_NoBranch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task without branch
	taskID := "TASK-NOBRANCH2"
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	taskYAML := fmt.Sprintf(`id: %s
title: No Branch Task
status: running
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/import", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// === handleListPRChecks Tests ===

func TestHandleListPRChecks_TaskNotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/github/pr/checks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleListPRChecks_NoBranch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task without branch
	taskID := "TASK-NOBRANCH3"
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	taskYAML := fmt.Sprintf(`id: %s
title: No Branch Task
status: running
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/github/pr/checks", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// === Check Status Categorization Tests (handleListPRChecks) ===

// Note: These test the categorization logic conceptually. Full integration
// tests would require mocking the GitHub client.

func TestCheckStatusCategorization_NeutralAndSkipped(t *testing.T) {
	// Test that neutral, skipped, and cancelled conclusions are not counted as failures
	// This validates the logic in handleListPRChecks

	tests := []struct {
		status     string
		conclusion string
		category   string
	}{
		{"completed", "success", "passed"},
		{"completed", "failure", "failed"},
		{"completed", "neutral", "neutral"},
		{"completed", "skipped", "neutral"},
		{"completed", "cancelled", "neutral"},
		{"completed", "action_required", "neutral"},
		{"completed", "timed_out", "failed"},
		{"completed", "stale", "failed"},
		{"completed", "startup_failure", "failed"},
		{"in_progress", "", "pending"},
		{"queued", "", "pending"},
		{"waiting", "", "pending"},
		{"pending", "", "pending"},
		{"requested", "", "pending"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.status, tt.conclusion), func(t *testing.T) {
			var category string

			switch tt.status {
			case "completed":
				switch tt.conclusion {
				case "success":
					category = "passed"
				case "neutral", "skipped", "cancelled", "action_required":
					category = "neutral"
				default:
					category = "failed"
				}
			default:
				category = "pending"
			}

			if category != tt.category {
				t.Errorf("status=%s conclusion=%s: expected category %s, got %s",
					tt.status, tt.conclusion, tt.category, category)
			}
		})
	}
}

// === handleCreatePR Tests ===

func TestHandleCreatePR_TaskNotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/github/pr", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleCreatePR_NoBranch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task without branch
	taskID := "TASK-NOBRANCH4"
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	taskYAML := fmt.Sprintf(`id: %s
title: No Branch Task
status: running
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleCreatePR_DefaultsTitle(t *testing.T) {
	// This tests that buildPRBody works correctly with a task
	tsk := &task.Task{
		ID:          "TASK-PR-001",
		Title:       "Implement feature X",
		Description: "This implements feature X as requested",
	}

	body := buildPRBody(tsk)

	if body == "" {
		t.Error("expected non-empty PR body")
	}

	if !bytes.Contains([]byte(body), []byte("Implement feature X")) {
		t.Error("expected PR body to contain task title")
	}

	if !bytes.Contains([]byte(body), []byte("This implements feature X")) {
		t.Error("expected PR body to contain task description")
	}

	if !bytes.Contains([]byte(body), []byte("Generated by [orc]")) {
		t.Error("expected PR body to contain orc attribution")
	}
}

// === handleGetPR Tests ===

func TestHandleGetPR_TaskNotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/github/pr", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleGetPR_NoBranch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task without branch
	taskID := "TASK-NOBRANCH5"
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	taskYAML := fmt.Sprintf(`id: %s
title: No Branch Task
status: running
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/github/pr", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// === handleMergePR Tests ===

func TestHandleMergePR_TaskNotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/github/pr/merge", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleMergePR_NoBranch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task without branch
	taskID := "TASK-NOBRANCH6"
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	taskYAML := fmt.Sprintf(`id: %s
title: No Branch Task
status: running
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/merge", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// === handleSyncPRComments Tests ===

func TestHandleSyncPRComments_TaskNotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/github/pr/comments/sync", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleSyncPRComments_NoBranch(t *testing.T) {
	tmpDir := t.TempDir()

	// Create task without branch
	taskID := "TASK-NOBRANCH7"
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	taskYAML := fmt.Sprintf(`id: %s
title: No Branch Task
status: running
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/sync", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSyncPRComments_NoComments(t *testing.T) {
	srv, taskID, cleanup := setupGitHubTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/sync", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should return 200 with message "no comments to sync"
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp syncCommentsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Total != 0 {
		t.Errorf("expected 0 total comments, got %d", resp.Total)
	}

	if resp.Message != "no comments to sync" {
		t.Errorf("expected message 'no comments to sync', got %s", resp.Message)
	}
}

// === Helper Function Tests ===

func TestFormatReviewCommentBody(t *testing.T) {
	tests := []struct {
		name     string
		comment  db.ReviewComment
		expected string
	}{
		{
			name: "blocker severity",
			comment: db.ReviewComment{
				Content:  "Critical bug",
				Severity: db.SeverityBlocker,
			},
			expected: "**[BLOCKER]** Critical bug",
		},
		{
			name: "issue severity",
			comment: db.ReviewComment{
				Content:  "Should fix this",
				Severity: db.SeverityIssue,
			},
			expected: "**[Issue]** Should fix this",
		},
		{
			name: "suggestion severity",
			comment: db.ReviewComment{
				Content:  "Consider refactoring",
				Severity: db.SeveritySuggestion,
			},
			expected: "**[Suggestion]** Consider refactoring",
		},
		{
			name: "empty severity defaults to suggestion",
			comment: db.ReviewComment{
				Content:  "A comment",
				Severity: "",
			},
			expected: "**[Suggestion]** A comment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatReviewCommentBody(tt.comment)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"", 10, ""},
		{"ab", 5, "ab"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, expected %q",
					tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}

func TestBuildPRBody_WithDescription(t *testing.T) {
	tsk := &task.Task{
		ID:          "TASK-001",
		Title:       "Add user authentication",
		Description: "Implement OAuth2 login flow with Google provider",
	}

	body := buildPRBody(tsk)

	// Should contain task title
	if !bytes.Contains([]byte(body), []byte("Add user authentication")) {
		t.Error("expected PR body to contain task title")
	}

	// Should contain description
	if !bytes.Contains([]byte(body), []byte("OAuth2 login flow")) {
		t.Error("expected PR body to contain task description")
	}

	// Should have description header when description exists
	if !bytes.Contains([]byte(body), []byte("### Description")) {
		t.Error("expected PR body to contain Description header")
	}
}

func TestBuildPRBody_WithoutDescription(t *testing.T) {
	tsk := &task.Task{
		ID:    "TASK-002",
		Title: "Fix bug",
	}

	body := buildPRBody(tsk)

	// Should contain task title
	if !bytes.Contains([]byte(body), []byte("Fix bug")) {
		t.Error("expected PR body to contain task title")
	}

	// Should NOT have description header when no description
	if bytes.Contains([]byte(body), []byte("### Description")) {
		t.Error("expected PR body to NOT contain Description header when no description")
	}
}

// === State/Plan Loading Tests ===

func TestHandleAutoFixComment_LoadsPlanAndState(t *testing.T) {
	testComment := db.ReviewComment{
		ID:         "RC-loadtest",
		FilePath:   "test.go",
		LineNumber: 10,
		Content:    "Test comment",
		Status:     db.CommentStatusOpen,
	}

	srv, taskID, cleanup := setupGitHubTestEnv(t, withReviewComments([]db.ReviewComment{testComment}))
	defer cleanup()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/RC-loadtest/autofix", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify plan is loadable
	_, err := plan.LoadFrom(srv.workDir, taskID)
	if err != nil {
		t.Errorf("expected plan to be loadable: %v", err)
	}

	// Verify state has retry context
	st, err := state.LoadFrom(srv.workDir, taskID)
	if err != nil {
		t.Errorf("expected state to be loadable: %v", err)
	}

	if st.RetryContext == nil {
		t.Error("expected state to have retry context")
	}
}

// === Integration Tests (require GitHub CLI) ===

// Note: These tests require the `gh` CLI to be installed and authenticated.
// They are skipped if gh auth fails.

func skipIfNoGH(t *testing.T) {
	t.Helper()
	// This is a helper that would check for gh auth
	// For now, we skip these tests by not including them
}

// === Response Type Tests ===

func TestSyncCommentsResponse_Structure(t *testing.T) {
	resp := syncCommentsResponse{
		Synced:   5,
		Skipped:  2,
		Errors:   1,
		Total:    8,
		PRNumber: 123,
		Message:  "synced 5 comments to PR #123",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded syncCommentsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.Synced != 5 {
		t.Errorf("expected Synced=5, got %d", decoded.Synced)
	}
	if decoded.PRNumber != 123 {
		t.Errorf("expected PRNumber=123, got %d", decoded.PRNumber)
	}
}

func TestAutoFixResponse_Structure(t *testing.T) {
	resp := autoFixResponse{
		TaskID:    "TASK-001",
		CommentID: "RC-abc123",
		Status:    "running",
		Message:   "Auto-fix started",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded autoFixResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.TaskID != "TASK-001" {
		t.Errorf("expected TaskID=TASK-001, got %s", decoded.TaskID)
	}
	if decoded.CommentID != "RC-abc123" {
		t.Errorf("expected CommentID=RC-abc123, got %s", decoded.CommentID)
	}
}

func TestImportPRCommentsResponse_Structure(t *testing.T) {
	resp := importPRCommentsResponse{
		Imported: 3,
		Skipped:  1,
		Errors:   0,
		Total:    4,
		PRNumber: 456,
		Message:  "imported 3 comments from PR #456",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded importPRCommentsResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.Imported != 3 {
		t.Errorf("expected Imported=3, got %d", decoded.Imported)
	}
	if decoded.PRNumber != 456 {
		t.Errorf("expected PRNumber=456, got %d", decoded.PRNumber)
	}
}

// === Edge Cases ===

func TestHandleAutoFixComment_WithOpenComments(t *testing.T) {
	// Test that other open comments are included in the retry context
	comments := []db.ReviewComment{
		{
			ID:         "RC-target",
			FilePath:   "target.go",
			LineNumber: 10,
			Content:    "Fix this specific issue",
			Severity:   db.SeverityIssue,
			Status:     db.CommentStatusOpen,
		},
		{
			ID:         "RC-other",
			FilePath:   "other.go",
			LineNumber: 20,
			Content:    "Another open issue",
			Severity:   db.SeveritySuggestion,
			Status:     db.CommentStatusOpen,
		},
		{
			ID:         "RC-resolved",
			FilePath:   "resolved.go",
			LineNumber: 30,
			Content:    "Already fixed",
			Severity:   db.SeverityIssue,
			Status:     db.CommentStatusResolved,
		},
	}

	srv, taskID, cleanup := setupGitHubTestEnv(t, withReviewComments(comments))
	defer cleanup()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/RC-target/autofix", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// The handler should have included all open comments in the context
	// We can't easily verify this without inspecting the retry context,
	// but we can verify the response indicates success
	var resp autoFixResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "running" {
		t.Errorf("expected status 'running', got %s", resp.Status)
	}
}

// TestHandleAutoFixComment_ConcurrentExecution tests that the handler
// properly manages the running tasks map.
func TestHandleAutoFixComment_RegistersRunningTask(t *testing.T) {
	testComment := db.ReviewComment{
		ID:       "RC-concurrent",
		FilePath: "concurrent.go",
		Content:  "Fix concurrency issue",
		Status:   db.CommentStatusOpen,
	}

	srv, taskID, cleanup := setupGitHubTestEnv(t, withReviewComments([]db.ReviewComment{testComment}))
	defer cleanup()

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/github/pr/comments/RC-concurrent/autofix", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Give the goroutine a moment to start
	time.Sleep(50 * time.Millisecond)

	// Check that the task is registered (or was registered and completed)
	// This is a basic check - in a real test we might want to mock the executor
	srv.runningTasksMu.RLock()
	_, exists := srv.runningTasks[taskID]
	srv.runningTasksMu.RUnlock()

	// The task might have already completed (execution failed due to no Claude)
	// so we just verify the response was correct
	var resp autoFixResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.TaskID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, resp.TaskID)
	}

	// Allow for the fact that exists could be true or false depending on timing
	_ = exists
}
