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

	"github.com/randalmurphal/orc/internal/db"
)

// setupTaskCommentsTestEnv creates a test environment with a task and optionally comments.
func setupTaskCommentsTestEnv(t *testing.T, opts ...func(*testing.T, string, string)) (srv *Server, taskID string, cleanup func()) {
	t.Helper()

	tmpDir := t.TempDir()

	// Create .orc directory with config
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	configYAML := `worktree:
  enabled: false
`
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(configYAML), 0644)

	// Create task directory
	taskID = "TASK-COMMENTS-001"
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", taskID)
	_ = os.MkdirAll(taskDir, 0755)

	// Create task.yaml
	taskYAML := fmt.Sprintf(`id: %s
title: Comments Test Task
description: A task for testing comments handlers
status: running
weight: medium
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID)
	_ = os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Initialize database with task
	pdb, err := db.OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	_, err = pdb.Exec(`
		INSERT INTO tasks (id, title, status, weight, created_at)
		VALUES (?, ?, ?, ?, datetime('now'))
	`, taskID, "Comments Test Task", "running", "medium")
	if err != nil {
		t.Fatalf("failed to create task in database: %v", err)
	}
	_ = pdb.Close()

	// Apply optional setup functions
	for _, opt := range opts {
		opt(t, tmpDir, taskID)
	}

	srv = New(&Config{WorkDir: tmpDir})

	cleanup = func() {}

	return srv, taskID, cleanup
}

// withTaskComments adds task comments to the database.
func withTaskComments(comments []db.TaskComment) func(*testing.T, string, string) {
	return func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		pdb, err := db.OpenProject(tmpDir)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer func() { _ = pdb.Close() }()

		for _, c := range comments {
			c.TaskID = taskID
			if err := pdb.CreateTaskComment(&c); err != nil {
				t.Fatalf("failed to create task comment: %v", err)
			}
		}
	}
}

func TestHandleListTaskComments_Empty(t *testing.T) {
	srv, taskID, cleanup := setupTaskCommentsTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/comments", nil)
	req.SetPathValue("id", taskID)
	w := httptest.NewRecorder()

	srv.handleListTaskComments(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var comments []db.TaskComment
	if err := json.NewDecoder(w.Body).Decode(&comments); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(comments) != 0 {
		t.Errorf("len(comments) = %d, want 0", len(comments))
	}
}

func TestHandleListTaskComments_WithComments(t *testing.T) {
	testComments := []db.TaskComment{
		{Author: "user1", AuthorType: db.AuthorTypeHuman, Content: "Comment 1"},
		{Author: "claude", AuthorType: db.AuthorTypeAgent, Content: "Comment 2"},
	}

	srv, taskID, cleanup := setupTaskCommentsTestEnv(t, withTaskComments(testComments))
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/comments", nil)
	req.SetPathValue("id", taskID)
	w := httptest.NewRecorder()

	srv.handleListTaskComments(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var comments []db.TaskComment
	if err := json.NewDecoder(w.Body).Decode(&comments); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(comments) != 2 {
		t.Errorf("len(comments) = %d, want 2", len(comments))
	}
}

func TestHandleListTaskComments_FilterByAuthorType(t *testing.T) {
	testComments := []db.TaskComment{
		{Author: "user1", AuthorType: db.AuthorTypeHuman, Content: "Human comment"},
		{Author: "claude", AuthorType: db.AuthorTypeAgent, Content: "Agent comment"},
	}

	srv, taskID, cleanup := setupTaskCommentsTestEnv(t, withTaskComments(testComments))
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/comments?author_type=agent", nil)
	req.SetPathValue("id", taskID)
	w := httptest.NewRecorder()

	srv.handleListTaskComments(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var comments []db.TaskComment
	if err := json.NewDecoder(w.Body).Decode(&comments); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(comments) != 1 {
		t.Errorf("len(comments) = %d, want 1", len(comments))
	}
	if len(comments) > 0 && comments[0].AuthorType != db.AuthorTypeAgent {
		t.Errorf("AuthorType = %q, want agent", comments[0].AuthorType)
	}
}

func TestHandleCreateTaskComment(t *testing.T) {
	srv, taskID, cleanup := setupTaskCommentsTestEnv(t)
	defer cleanup()

	body := createTaskCommentRequest{
		Author:     "testuser",
		AuthorType: "human",
		Content:    "This is a test comment",
		Phase:      "implement",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/comments", bytes.NewReader(jsonBody))
	req.SetPathValue("id", taskID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleCreateTaskComment(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var comment db.TaskComment
	if err := json.NewDecoder(w.Body).Decode(&comment); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if comment.ID == "" {
		t.Error("comment ID not set")
	}
	if comment.Author != "testuser" {
		t.Errorf("Author = %q, want testuser", comment.Author)
	}
	if comment.AuthorType != db.AuthorTypeHuman {
		t.Errorf("AuthorType = %q, want human", comment.AuthorType)
	}
	if comment.Content != "This is a test comment" {
		t.Errorf("Content = %q, want 'This is a test comment'", comment.Content)
	}
	if comment.Phase != "implement" {
		t.Errorf("Phase = %q, want implement", comment.Phase)
	}
}

func TestHandleCreateTaskComment_EmptyContent(t *testing.T) {
	srv, taskID, cleanup := setupTaskCommentsTestEnv(t)
	defer cleanup()

	body := createTaskCommentRequest{
		Author:  "testuser",
		Content: "", // Empty content
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/comments", bytes.NewReader(jsonBody))
	req.SetPathValue("id", taskID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleCreateTaskComment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleCreateTaskComment_InvalidAuthorType(t *testing.T) {
	srv, taskID, cleanup := setupTaskCommentsTestEnv(t)
	defer cleanup()

	body := createTaskCommentRequest{
		AuthorType: "invalid", // Invalid author type
		Content:    "Test comment",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/tasks/"+taskID+"/comments", bytes.NewReader(jsonBody))
	req.SetPathValue("id", taskID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleCreateTaskComment(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleGetTaskComment(t *testing.T) {
	var commentID string
	testComments := []db.TaskComment{
		{Author: "testuser", AuthorType: db.AuthorTypeHuman, Content: "Test comment"},
	}

	setupWithID := func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		pdb, err := db.OpenProject(tmpDir)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer func() { _ = pdb.Close() }()

		for i := range testComments {
			testComments[i].TaskID = taskID
			if err := pdb.CreateTaskComment(&testComments[i]); err != nil {
				t.Fatalf("failed to create task comment: %v", err)
			}
			commentID = testComments[i].ID
		}
	}

	srv, taskID, cleanup := setupTaskCommentsTestEnv(t, setupWithID)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/comments/"+commentID, nil)
	req.SetPathValue("id", taskID)
	req.SetPathValue("commentId", commentID)
	w := httptest.NewRecorder()

	srv.handleGetTaskComment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var comment db.TaskComment
	if err := json.NewDecoder(w.Body).Decode(&comment); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if comment.ID != commentID {
		t.Errorf("ID = %q, want %q", comment.ID, commentID)
	}
}

func TestHandleGetTaskComment_NotFound(t *testing.T) {
	srv, taskID, cleanup := setupTaskCommentsTestEnv(t)
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/comments/TC-nonexistent", nil)
	req.SetPathValue("id", taskID)
	req.SetPathValue("commentId", "TC-nonexistent")
	w := httptest.NewRecorder()

	srv.handleGetTaskComment(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleUpdateTaskComment(t *testing.T) {
	var commentID string
	setupWithID := func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		pdb, err := db.OpenProject(tmpDir)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer func() { _ = pdb.Close() }()

		comment := &db.TaskComment{
			TaskID:     taskID,
			Author:     "testuser",
			AuthorType: db.AuthorTypeHuman,
			Content:    "Original content",
		}
		if err := pdb.CreateTaskComment(comment); err != nil {
			t.Fatalf("failed to create task comment: %v", err)
		}
		commentID = comment.ID
	}

	srv, taskID, cleanup := setupTaskCommentsTestEnv(t, setupWithID)
	defer cleanup()

	body := updateTaskCommentRequest{
		Content: "Updated content",
		Phase:   "test",
	}
	jsonBody, _ := json.Marshal(body)

	req := httptest.NewRequest("PATCH", "/api/tasks/"+taskID+"/comments/"+commentID, bytes.NewReader(jsonBody))
	req.SetPathValue("id", taskID)
	req.SetPathValue("commentId", commentID)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.handleUpdateTaskComment(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}

	var comment db.TaskComment
	if err := json.NewDecoder(w.Body).Decode(&comment); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if comment.Content != "Updated content" {
		t.Errorf("Content = %q, want 'Updated content'", comment.Content)
	}
}

func TestHandleDeleteTaskComment(t *testing.T) {
	var commentID string
	setupWithID := func(t *testing.T, tmpDir, taskID string) {
		t.Helper()

		pdb, err := db.OpenProject(tmpDir)
		if err != nil {
			t.Fatalf("failed to open database: %v", err)
		}
		defer func() { _ = pdb.Close() }()

		comment := &db.TaskComment{
			TaskID:     taskID,
			Author:     "testuser",
			AuthorType: db.AuthorTypeHuman,
			Content:    "To be deleted",
		}
		if err := pdb.CreateTaskComment(comment); err != nil {
			t.Fatalf("failed to create task comment: %v", err)
		}
		commentID = comment.ID
	}

	srv, taskID, cleanup := setupTaskCommentsTestEnv(t, setupWithID)
	defer cleanup()

	req := httptest.NewRequest("DELETE", "/api/tasks/"+taskID+"/comments/"+commentID, nil)
	req.SetPathValue("id", taskID)
	req.SetPathValue("commentId", commentID)
	w := httptest.NewRecorder()

	srv.handleDeleteTaskComment(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestHandleGetTaskCommentStats(t *testing.T) {
	testComments := []db.TaskComment{
		{Author: "user1", AuthorType: db.AuthorTypeHuman, Content: "Human 1"},
		{Author: "user2", AuthorType: db.AuthorTypeHuman, Content: "Human 2"},
		{Author: "claude", AuthorType: db.AuthorTypeAgent, Content: "Agent"},
		{Author: "orc", AuthorType: db.AuthorTypeSystem, Content: "System"},
	}

	srv, taskID, cleanup := setupTaskCommentsTestEnv(t, withTaskComments(testComments))
	defer cleanup()

	req := httptest.NewRequest("GET", "/api/tasks/"+taskID+"/comments/stats", nil)
	req.SetPathValue("id", taskID)
	w := httptest.NewRecorder()

	srv.handleGetTaskCommentStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var stats map[string]any
	if err := json.NewDecoder(w.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if stats["total_comments"].(float64) != 4 {
		t.Errorf("total_comments = %v, want 4", stats["total_comments"])
	}
	if stats["human_count"].(float64) != 2 {
		t.Errorf("human_count = %v, want 2", stats["human_count"])
	}
	if stats["agent_count"].(float64) != 1 {
		t.Errorf("agent_count = %v, want 1", stats["agent_count"])
	}
	if stats["system_count"].(float64) != 1 {
		t.Errorf("system_count = %v, want 1", stats["system_count"])
	}
}
