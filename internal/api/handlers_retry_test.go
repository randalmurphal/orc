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

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// === Retry Handler Tests ===

// setupRetryTestEnv creates a test environment with task, state, and optionally review comments.
func setupRetryTestEnv(t *testing.T, opts ...func(*testing.T, string, string)) (srv *Server, taskID string, cleanup func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Create .orc directory with config that disables worktrees
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	configYAML := `worktree:
  enabled: false
`
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configYAML), 0644)

	taskID = "TASK-RETRY-001"
	startTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	completedTime := time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC)

	// Create backend and save test data
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create and save task
	tsk := task.New(taskID, "Retry Test Task")
	tsk.Description = "A task for testing retry handlers"
	tsk.Status = task.StatusFailed
	tsk.Weight = task.WeightMedium
	tsk.CurrentPhase = "test"
	tsk.CreatedAt = startTime
	tsk.UpdatedAt = startTime
	tsk.StartedAt = &startTime
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Set execution state on task
	tsk.Execution.CurrentIteration = 3
	tsk.Execution.Phases = map[string]*task.PhaseState{
		"implement": {
			Status:      task.PhaseStatusCompleted,
			StartedAt:   startTime,
			CompletedAt: &completedTime,
			Iterations:  5,
		},
		"test": {
			Status:     task.PhaseStatusFailed,
			StartedAt:  completedTime,
			Iterations: 3,
		},
	}
	tsk.Execution.Tokens = task.TokenUsage{
		InputTokens:  5000,
		OutputTokens: 2500,
		TotalTokens:  7500,
	}
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task with execution state: %v", err)
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

// withRetryReviewComments adds review comments to the database for retry tests.
func withRetryReviewComments(comments []db.ReviewComment) func(*testing.T, string, string) {
	return func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		pdb, err := db.OpenProject(tmpDir)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer func() { _ = pdb.Close() }()

		// Task is already created by setupRetryTestEnv, just add the review comments
		for _, c := range comments {
			c.TaskID = taskID
			if err := pdb.CreateReviewComment(&c); err != nil {
				t.Fatalf("failed to create review comment: %v", err)
			}
		}
	}
}

// withRetryContext sets up a retry context in task execution state.
func withRetryContext(attempt int) func(*testing.T, string, string) {
	return func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		// Create backend to load/save task
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

		tsk.Execution.RetryContext = &task.RetryContext{
			FromPhase:   "test",
			ToPhase:     "implement",
			Reason:      "Tests failed",
			Attempt:     attempt,
			ContextFile: "",
		}

		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("failed to save task with retry context: %v", err)
		}
	}
}

// === handleRetryTask Tests ===

func TestHandleRetryTask_TaskNotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/retry", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleRetryTask_DefaultOptions(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	// Send empty body - should use defaults
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TaskID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, resp.TaskID)
	}

	if resp.Status != "queued" {
		t.Errorf("expected status 'queued', got %s", resp.Status)
	}

	// Default from_phase should be determined by retry map
	// test -> implement
	if resp.FromPhase != "implement" {
		t.Errorf("expected from_phase 'implement' (from retry map), got %s", resp.FromPhase)
	}
}

func TestHandleRetryTask_WithReviewComments(t *testing.T) {
	t.Parallel()
	comments := []db.ReviewComment{
		{
			ID:         "RC-retry1",
			FilePath:   "main.go",
			LineNumber: 10,
			Content:    "Missing error handling",
			Severity:   db.SeverityIssue,
			Status:     db.CommentStatusOpen,
		},
		{
			ID:         "RC-retry2",
			FilePath:   "util.go",
			LineNumber: 25,
			Content:    "Consider using constants",
			Severity:   db.SeveritySuggestion,
			Status:     db.CommentStatusOpen,
		},
	}

	srv, taskID, cleanup := setupRetryTestEnv(t, withRetryReviewComments(comments))
	defer cleanup()

	body := bytes.NewBufferString(`{"include_review_comments": true}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should include the open comments
	if resp.CommentCount != 2 {
		t.Errorf("expected 2 comments, got %d", resp.CommentCount)
	}

	// Context should contain the comments
	if resp.Context == "" {
		t.Error("expected non-empty context")
	}
}

func TestHandleRetryTask_WithCustomFromPhase(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"from_phase": "spec"}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should use the custom from_phase
	if resp.FromPhase != "spec" {
		t.Errorf("expected from_phase 'spec', got %s", resp.FromPhase)
	}
}

func TestHandleRetryTask_WithInstructions(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	body := bytes.NewBufferString(`{"instructions": "Focus on fixing the null pointer exception"}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Context should contain the instructions
	if resp.Context == "" {
		t.Error("expected non-empty context")
	}

	if !bytes.Contains([]byte(resp.Context), []byte("null pointer exception")) {
		t.Error("expected context to contain instructions")
	}
}

func TestHandleRetryTask_AttemptNumberFromState(t *testing.T) {
	t.Parallel()
	// Set up state with existing retry context (attempt 2)
	srv, taskID, cleanup := setupRetryTestEnv(t, withRetryContext(2))
	defer cleanup()

	body := bytes.NewBufferString(`{}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Context should indicate attempt 3 (previous was 2, so next is 3)
	if !bytes.Contains([]byte(resp.Context), []byte("attempt 3")) {
		t.Error("expected context to indicate attempt 3")
	}
}

// === handleGetRetryPreview Tests ===

func TestHandleGetRetryPreview_TaskNotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/retry/preview", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleGetRetryPreview_ReturnsPreview(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/retry/preview", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryPreviewResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TaskID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, resp.TaskID)
	}

	if resp.CurrentPhase != "test" {
		t.Errorf("expected current phase 'test', got %s", resp.CurrentPhase)
	}

	if resp.EstimatedTokens <= 0 {
		t.Error("expected positive estimated tokens")
	}
}

func TestHandleGetRetryPreview_IncludesOpenComments(t *testing.T) {
	t.Parallel()
	comments := []db.ReviewComment{
		{
			ID:       "RC-preview1",
			FilePath: "preview.go",
			Content:  "Open comment for preview",
			Status:   db.CommentStatusOpen,
		},
		{
			ID:       "RC-preview2",
			FilePath: "preview.go",
			Content:  "Resolved comment",
			Status:   db.CommentStatusResolved,
		},
	}

	srv, taskID, cleanup := setupRetryTestEnv(t, withRetryReviewComments(comments))
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/retry/preview", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryPreviewResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should only count open comments
	if resp.OpenComments != 1 {
		t.Errorf("expected 1 open comment, got %d", resp.OpenComments)
	}
}

func TestHandleGetRetryPreview_AttemptNumberFromState(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t, withRetryContext(3))
	defer cleanup()

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/%s/retry/preview", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryPreviewResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Context preview should indicate attempt 4 (previous was 3)
	if !bytes.Contains([]byte(resp.ContextPreview), []byte("attempt 4")) {
		t.Error("expected context preview to indicate attempt 4")
	}
}

// === handleRetryWithFeedback Tests ===

func TestHandleRetryWithFeedback_TaskNotFound(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t)

	body := bytes.NewBufferString(`{"failure_reason": "Tests failed"}`)
	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/retry/feedback", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleRetryWithFeedback_InvalidBody(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry/feedback", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleRetryWithFeedback_WithFailureReason(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	body := bytes.NewBufferString(`{
		"failure_reason": "Test assertions failed in user_test.go",
		"failure_output": "FAIL: TestUserCreate expected 200 got 500"
	}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry/feedback", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Context should contain the failure reason
	if !bytes.Contains([]byte(resp.Context), []byte("Test assertions failed")) {
		t.Error("expected context to contain failure reason")
	}

	// Context should contain failure output
	if !bytes.Contains([]byte(resp.Context), []byte("FAIL: TestUserCreate")) {
		t.Error("expected context to contain failure output")
	}
}

func TestHandleRetryWithFeedback_WithPRComments(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	body := bytes.NewBufferString(`{
		"failure_reason": "Review feedback",
		"pr_comments": [
			{
				"author": "reviewer1",
				"body": "Please add error handling here",
				"file_path": "handler.go",
				"line": 42
			},
			{
				"author": "reviewer2",
				"body": "Missing tests for edge case",
				"file_path": "handler_test.go",
				"line": 100
			}
		]
	}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry/feedback", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Comment count should include PR comments
	if resp.CommentCount < 2 {
		t.Errorf("expected at least 2 comments (from PR), got %d", resp.CommentCount)
	}

	// Context should contain PR feedback
	if !bytes.Contains([]byte(resp.Context), []byte("error handling")) {
		t.Error("expected context to contain PR comment content")
	}
}

func TestHandleRetryWithFeedback_CombinesReviewAndPRComments(t *testing.T) {
	t.Parallel()
	// Set up with review comments
	reviewComments := []db.ReviewComment{
		{
			ID:       "RC-combined",
			FilePath: "api.go",
			Content:  "Review comment about security",
			Status:   db.CommentStatusOpen,
		},
	}

	srv, taskID, cleanup := setupRetryTestEnv(t, withRetryReviewComments(reviewComments))
	defer cleanup()

	// Also include PR comments
	body := bytes.NewBufferString(`{
		"failure_reason": "Combined feedback",
		"pr_comments": [
			{
				"author": "github_reviewer",
				"body": "PR comment about performance"
			}
		]
	}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry/feedback", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should count both review (1) and PR comments (1)
	if resp.CommentCount != 2 {
		t.Errorf("expected 2 comments total, got %d", resp.CommentCount)
	}
}

func TestHandleRetryWithFeedback_CustomFromPhase(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	body := bytes.NewBufferString(`{
		"failure_reason": "Need to rework spec",
		"from_phase": "spec"
	}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry/feedback", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.FromPhase != "spec" {
		t.Errorf("expected from_phase 'spec', got %s", resp.FromPhase)
	}
}

func TestHandleRetryWithFeedback_AttemptTracking(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t, withRetryContext(1))
	defer cleanup()

	body := bytes.NewBufferString(`{"failure_reason": "Second retry"}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry/feedback", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should indicate attempt 2
	if !bytes.Contains([]byte(resp.Context), []byte("attempt 2")) {
		t.Error("expected context to indicate attempt 2")
	}
}

// === Retry Map Tests ===

func TestDefaultRetryMap(t *testing.T) {
	t.Parallel()
	retryMap := executor.DefaultRetryMap()

	tests := []struct {
		failedPhase       string
		expectedRetry     string
		shouldHaveMapping bool
	}{
		{"test", "implement", true},
		{"test_unit", "implement", true},
		{"test_e2e", "implement", true},
		{"implement", "", false}, // No mapping - retry from same phase
		{"spec", "", false},      // No mapping
	}

	for _, tt := range tests {
		t.Run(tt.failedPhase, func(t *testing.T) {
			mapped, ok := retryMap[tt.failedPhase]

			if tt.shouldHaveMapping {
				if !ok {
					t.Errorf("expected mapping for %s", tt.failedPhase)
				}
				if mapped != tt.expectedRetry {
					t.Errorf("expected retry phase %s, got %s", tt.expectedRetry, mapped)
				}
			} else {
				if ok {
					t.Errorf("did not expect mapping for %s, but got %s", tt.failedPhase, mapped)
				}
			}
		})
	}
}

// === Warning Log Tests ===

func TestHandleRetryTask_LogsWarningOnCommentFetchError(t *testing.T) {
	t.Parallel()
	// This test verifies the handler gracefully handles errors when fetching comments
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	// Remove the database to simulate an error condition
	_ = os.RemoveAll(filepath.Join(srv.workDir, ".orc", "orc.db"))

	body := bytes.NewBufferString(`{"include_review_comments": true}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should still return success (with warning logged)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Comment count should be 0 since fetch failed
	if resp.CommentCount != 0 {
		t.Errorf("expected 0 comments due to error, got %d", resp.CommentCount)
	}

	// Response should still be valid
	if resp.TaskID != taskID {
		t.Errorf("expected task ID %s, got %s", taskID, resp.TaskID)
	}
}

// === Response Type Tests ===

func TestRetryResponse_Structure(t *testing.T) {
	t.Parallel()
	resp := retryResponse{
		TaskID:       "TASK-001",
		FromPhase:    "implement",
		Context:      "# Retry Context\n...",
		Status:       "queued",
		CommentCount: 5,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded retryResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.TaskID != "TASK-001" {
		t.Errorf("expected TaskID=TASK-001, got %s", decoded.TaskID)
	}
	if decoded.FromPhase != "implement" {
		t.Errorf("expected FromPhase=implement, got %s", decoded.FromPhase)
	}
	if decoded.CommentCount != 5 {
		t.Errorf("expected CommentCount=5, got %d", decoded.CommentCount)
	}
}

func TestRetryPreviewResponse_Structure(t *testing.T) {
	t.Parallel()
	resp := retryPreviewResponse{
		TaskID:          "TASK-002",
		CurrentPhase:    "test",
		OpenComments:    3,
		ContextPreview:  "# Retry Context\nThis is attempt 1...",
		EstimatedTokens: 500,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded retryPreviewResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if decoded.TaskID != "TASK-002" {
		t.Errorf("expected TaskID=TASK-002, got %s", decoded.TaskID)
	}
	if decoded.CurrentPhase != "test" {
		t.Errorf("expected CurrentPhase=test, got %s", decoded.CurrentPhase)
	}
	if decoded.OpenComments != 3 {
		t.Errorf("expected OpenComments=3, got %d", decoded.OpenComments)
	}
	if decoded.EstimatedTokens != 500 {
		t.Errorf("expected EstimatedTokens=500, got %d", decoded.EstimatedTokens)
	}
}

// === Edge Cases ===

func TestHandleRetryTask_EmptyTaskID(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srv := New(&Config{WorkDir: tmpDir})

	// Empty task ID in URL - Go mux may redirect or return 404
	req := httptest.NewRequest("POST", "/api/tasks//retry", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should not return 200 for invalid request
	// Go's ServeMux may return 301 (redirect) or 404, both are acceptable
	if w.Code == http.StatusOK {
		t.Errorf("expected non-200 status for empty task ID, got %d", w.Code)
	}
}

func TestHandleRetryTask_NoState(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create .orc directory
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)

	// Create task without state via backend
	taskID := "TASK-NOSTATE"
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	tsk := task.New(taskID, "No State Task")
	tsk.Status = task.StatusFailed
	tsk.CurrentPhase = "test"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	// Note: Not saving state - that's the point of this test
	_ = backend.Close()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry", taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should still succeed (state is optional for determining attempt number)
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should default to attempt 1
	if !bytes.Contains([]byte(resp.Context), []byte("attempt 1")) {
		t.Error("expected context to indicate attempt 1 when no state")
	}
}

func TestHandleRetryWithFeedback_WithInstructions(t *testing.T) {
	t.Parallel()
	srv, taskID, cleanup := setupRetryTestEnv(t)
	defer cleanup()

	body := bytes.NewBufferString(`{
		"failure_reason": "Build failed",
		"instructions": "Make sure to run go mod tidy before building"
	}`)
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/tasks/%s/retry/feedback", taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp retryResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Context should contain instructions
	if !bytes.Contains([]byte(resp.Context), []byte("go mod tidy")) {
		t.Error("expected context to contain instructions")
	}
}

// === Request Type Tests ===

func TestRetryRequest_Structure(t *testing.T) {
	t.Parallel()
	req := retryRequest{
		IncludeReviewComments: true,
		IncludePRComments:     true,
		Instructions:          "Focus on fixing tests",
		FromPhase:             "implement",
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var decoded retryRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	if !decoded.IncludeReviewComments {
		t.Error("expected IncludeReviewComments=true")
	}
	if decoded.FromPhase != "implement" {
		t.Errorf("expected FromPhase=implement, got %s", decoded.FromPhase)
	}
}

// === Integration with Executor Package ===

func TestBuildRetryContextForFreshSession(t *testing.T) {
	t.Parallel()
	// Test that the executor function is callable and produces valid output
	opts := executor.RetryOptions{
		FailedPhase:   "test",
		FailureReason: "Tests failed with assertion errors",
		FailureOutput: "FAIL: TestUserAuth\nExpected: true\nGot: false",
		AttemptNumber: 1,
		MaxAttempts:   3,
	}

	context := executor.BuildRetryContextForFreshSession(opts)

	if context == "" {
		t.Error("expected non-empty context")
	}

	if !bytes.Contains([]byte(context), []byte("Retry Context")) {
		t.Error("expected context to contain 'Retry Context' header")
	}

	if !bytes.Contains([]byte(context), []byte("attempt 1 of 3")) {
		t.Error("expected context to indicate attempt number")
	}

	if !bytes.Contains([]byte(context), []byte("Tests failed with assertion errors")) {
		t.Error("expected context to contain failure reason")
	}
}

func TestBuildRetryContextForFreshSession_WithReviewComments(t *testing.T) {
	t.Parallel()
	comments := []db.ReviewComment{
		{
			FilePath:   "auth.go",
			LineNumber: 50,
			Content:    "Missing null check",
			Severity:   db.SeverityBlocker,
		},
		{
			FilePath:   "auth.go",
			LineNumber: 75,
			Content:    "Consider using constants",
			Severity:   db.SeveritySuggestion,
		},
	}

	opts := executor.RetryOptions{
		FailedPhase:    "test",
		FailureReason:  "Review issues",
		ReviewComments: comments,
		AttemptNumber:  2,
		MaxAttempts:    3,
	}

	context := executor.BuildRetryContextForFreshSession(opts)

	// Should contain comments grouped by file
	if !bytes.Contains([]byte(context), []byte("auth.go")) {
		t.Error("expected context to contain file path")
	}

	if !bytes.Contains([]byte(context), []byte("BLOCKER")) {
		t.Error("expected context to contain severity")
	}

	if !bytes.Contains([]byte(context), []byte("Missing null check")) {
		t.Error("expected context to contain comment content")
	}
}

func TestBuildRetryContextForFreshSession_WithPRComments(t *testing.T) {
	t.Parallel()
	prComments := []executor.PRCommentFeedback{
		{
			Author:   "reviewer1",
			Body:     "Please add documentation",
			FilePath: "api.go",
			Line:     100,
		},
	}

	opts := executor.RetryOptions{
		FailedPhase:   "test",
		PRComments:    prComments,
		AttemptNumber: 1,
		MaxAttempts:   3,
	}

	context := executor.BuildRetryContextForFreshSession(opts)

	// Should contain PR feedback section
	if !bytes.Contains([]byte(context), []byte("PR Feedback")) {
		t.Error("expected context to contain PR Feedback section")
	}

	if !bytes.Contains([]byte(context), []byte("@reviewer1")) {
		t.Error("expected context to contain reviewer")
	}

	if !bytes.Contains([]byte(context), []byte("add documentation")) {
		t.Error("expected context to contain comment body")
	}
}
