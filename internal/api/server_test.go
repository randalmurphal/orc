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

	"github.com/randalmurphal/orc/internal/hooks"
	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/randalmurphal/orc/internal/skills"
	"github.com/randalmurphal/orc/internal/task"
)

func TestHealthEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", resp["status"])
	}
}

func TestCORSHeaders(t *testing.T) {
	srv := New(nil)

	// CORS headers are set on actual requests, not just OPTIONS
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("expected CORS header to be set")
	}
}

func TestListPromptsEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var prompts []prompt.PromptInfo
	if err := json.NewDecoder(w.Body).Decode(&prompts); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should have at least one prompt (embedded)
	if len(prompts) == 0 {
		t.Error("expected at least one prompt")
	}

	// Check for implement phase
	found := false
	for _, p := range prompts {
		if p.Phase == "implement" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'implement' phase")
	}
}

func TestGetPromptVariablesEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/variables", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var vars map[string]string
	if err := json.NewDecoder(w.Body).Decode(&vars); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Check for required variables
	if _, ok := vars["{{TASK_ID}}"]; !ok {
		t.Error("expected TASK_ID variable")
	}
}

func TestGetPromptEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/implement", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var p prompt.Prompt
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if p.Phase != "implement" {
		t.Errorf("expected phase 'implement', got %q", p.Phase)
	}

	if p.Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestGetPromptEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetPromptDefaultEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/implement/default", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var p prompt.Prompt
	if err := json.NewDecoder(w.Body).Decode(&p); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if p.Source != prompt.SourceEmbedded {
		t.Errorf("expected source 'embedded', got %q", p.Source)
	}
}

func TestSavePromptEndpoint_EmptyContent(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{"content":""}`)
	req := httptest.NewRequest("PUT", "/api/prompts/test", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeletePromptEndpoint_NoOverride(t *testing.T) {
	srv := New(nil)

	// Try to delete a prompt that has no override
	req := httptest.NewRequest("DELETE", "/api/prompts/implement", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Hooks API Tests ===

func TestListHooksEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/hooks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var hookList []hooks.HookInfo
	if err := json.NewDecoder(w.Body).Decode(&hookList); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Empty list is OK if no hooks exist
	if hookList == nil {
		t.Error("expected non-nil list")
	}
}

func TestGetHookTypesEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/hooks/types", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var types []hooks.HookType
	if err := json.NewDecoder(w.Body).Decode(&types); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(types) == 0 {
		t.Error("expected at least one hook type")
	}
}

func TestGetHookEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/hooks/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateHookEndpoint_InvalidBody(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", "/api/hooks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateHookEndpoint_MissingName(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{"type":"pre:tool","command":"echo test"}`)
	req := httptest.NewRequest("POST", "/api/hooks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteHookEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/hooks/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Skills API Tests ===

func TestListSkillsEndpoint(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/skills", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var skillList []skills.SkillInfo
	if err := json.NewDecoder(w.Body).Decode(&skillList); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Empty list is OK if no skills exist
	if skillList == nil {
		t.Error("expected non-nil list")
	}
}

func TestGetSkillEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/skills/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateSkillEndpoint_InvalidBody(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", "/api/skills", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateSkillEndpoint_MissingName(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{"prompt":"Do something"}`)
	req := httptest.NewRequest("POST", "/api/skills", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteSkillEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/skills/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Task API Tests ===

func TestListTasksEndpoint_EmptyDir(t *testing.T) {
	// Create temp dir for .orc
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create minimal .orc structure
	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var tasks []task.Task
	if err := json.NewDecoder(w.Body).Decode(&tasks); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}
}

func TestListTasksEndpoint_WithTasks(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task directory and file
	taskDir := filepath.Join(".orc", "tasks", "TASK-001")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-001
title: Test Task
description: A test task
status: pending
weight: small
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var tasks []task.Task
	if err := json.NewDecoder(w.Body).Decode(&tasks); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].ID != "TASK-001" {
		t.Errorf("expected task ID 'TASK-001', got %q", tasks[0].ID)
	}
}

func TestListTasksEndpoint_Pagination(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create multiple tasks for pagination testing
	for i := 1; i <= 25; i++ {
		taskDir := filepath.Join(".orc", "tasks", fmt.Sprintf("TASK-%03d", i))
		os.MkdirAll(taskDir, 0755)

		taskYAML := fmt.Sprintf(`id: TASK-%03d
title: Test Task %d
description: Test task number %d
status: pending
weight: small
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`, i, i, i)
		os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)
	}

	srv := New(nil)

	// Test pagination with page=1, limit=10
	req := httptest.NewRequest("GET", "/api/tasks?page=1&limit=10", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp struct {
		Tasks      []task.Task `json:"tasks"`
		Total      int         `json:"total"`
		Page       int         `json:"page"`
		Limit      int         `json:"limit"`
		TotalPages int         `json:"total_pages"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Tasks) != 10 {
		t.Errorf("expected 10 tasks, got %d", len(resp.Tasks))
	}
	if resp.Total != 25 {
		t.Errorf("expected total 25, got %d", resp.Total)
	}
	if resp.Page != 1 {
		t.Errorf("expected page 1, got %d", resp.Page)
	}
	if resp.TotalPages != 3 {
		t.Errorf("expected 3 total pages, got %d", resp.TotalPages)
	}

	// Test page 3 (should have 5 tasks)
	req = httptest.NewRequest("GET", "/api/tasks?page=3&limit=10", nil)
	w = httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Tasks) != 5 {
		t.Errorf("expected 5 tasks on page 3, got %d", len(resp.Tasks))
	}

	// Test without pagination (backward compatible)
	req = httptest.NewRequest("GET", "/api/tasks", nil)
	w = httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	var allTasks []task.Task
	if err := json.NewDecoder(w.Body).Decode(&allTasks); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(allTasks) != 25 {
		t.Errorf("expected 25 tasks without pagination, got %d", len(allTasks))
	}
}

func TestGetTaskEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task
	taskDir := filepath.Join(".orc", "tasks", "TASK-002")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-002
title: Get Test Task
description: For testing GET endpoint
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/TASK-002", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var tsk task.Task
	if err := json.NewDecoder(w.Body).Decode(&tsk); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if tsk.ID != "TASK-002" {
		t.Errorf("expected task ID 'TASK-002', got %q", tsk.ID)
	}

	if tsk.Title != "Get Test Task" {
		t.Errorf("expected title 'Get Test Task', got %q", tsk.Title)
	}
}

func TestGetTaskEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateTaskEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	body := bytes.NewBufferString(`{"title":"New Task","description":"Create test","weight":"small"}`)
	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Errorf("expected status 200 or 201, got %d: %s", w.Code, w.Body.String())
	}

	var tsk task.Task
	if err := json.NewDecoder(w.Body).Decode(&tsk); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if tsk.Title != "New Task" {
		t.Errorf("expected title 'New Task', got %q", tsk.Title)
	}

	if tsk.ID == "" {
		t.Error("expected non-empty task ID")
	}
}

func TestCreateTaskEndpoint_MissingTitle(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	body := bytes.NewBufferString(`{"description":"No title"}`)
	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateTaskEndpoint_InvalidJSON(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("POST", "/api/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteTaskEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create a task to delete
	os.MkdirAll(".orc/tasks/TASK-DEL-001", 0755)
	testTask := task.New("TASK-DEL-001", "Delete Test")
	testTask.Status = task.StatusCompleted
	testTask.Save()

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-DEL-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify task was deleted
	if task.Exists("TASK-DEL-001") {
		t.Error("task should have been deleted")
	}
}

func TestDeleteTaskEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-NONEXISTENT", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteTaskEndpoint_RunningTask(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create a running task
	os.MkdirAll(".orc/tasks/TASK-RUN-001", 0755)
	testTask := task.New("TASK-RUN-001", "Running Task")
	testTask.Status = task.StatusRunning
	testTask.Save()

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/tasks/TASK-RUN-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d", w.Code)
	}

	// Verify task still exists
	if !task.Exists("TASK-RUN-001") {
		t.Error("running task should not have been deleted")
	}
}

func TestGetStateEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task with state file
	taskDir := filepath.Join(".orc", "tasks", "TASK-003")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-003
title: State Test
status: running
weight: small
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	stateYAML := `task_id: TASK-003
status: running
started_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
phases: {}
tokens:
  input: 0
  output: 0
  total: 0
`
	os.WriteFile(filepath.Join(taskDir, "state.yaml"), []byte(stateYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/TASK-003/state", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetStateEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/state", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetPlanEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/plan", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetTranscriptsEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task with transcripts directory
	taskDir := filepath.Join(".orc", "tasks", "TASK-004")
	transcriptsDir := filepath.Join(taskDir, "transcripts")
	os.MkdirAll(transcriptsDir, 0755)

	taskYAML := `id: TASK-004
title: Transcripts Test
status: completed
weight: small
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create transcript file
	transcript := "This is a test transcript."
	os.WriteFile(filepath.Join(transcriptsDir, "implement-001.md"), []byte(transcript), 0644)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/TASK-004/transcripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var transcripts []TranscriptFile
	if err := json.NewDecoder(w.Body).Decode(&transcripts); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(transcripts) != 1 {
		t.Errorf("expected 1 transcript, got %d", len(transcripts))
	}
}

func TestRunTaskEndpoint_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestPauseTaskEndpoint_NotRunning(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/TASK-999/pause", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestResumeTaskEndpoint_NotPaused(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/TASK-999/resume", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestServerConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Addr != ":8080" {
		t.Errorf("expected addr ':8080', got %q", cfg.Addr)
	}
}

func TestNewServer_WithConfig(t *testing.T) {
	cfg := &Config{
		Addr: ":9090",
	}

	srv := New(cfg)

	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

// TranscriptFile is needed for decoding transcripts response
type TranscriptFile struct {
	Filename  string `json:"filename"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

// === Config API Tests ===

func TestGetConfigEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create config directory and file
	os.MkdirAll(".orc", 0755)
	configYAML := `version: 1
model: claude-sonnet-4-20250514
max_iterations: 30
timeout: 10m
`
	os.WriteFile(".orc/config.yaml", []byte(configYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetConfigEndpoint_NoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// No config file exists
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Should still return OK with default config
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateConfigEndpoint(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create config directory
	os.MkdirAll(".orc", 0755)

	srv := New(nil)

	// Update config with new values
	body := bytes.NewBufferString(`{
		"profile": "safe",
		"automation": {
			"gates_default": "human",
			"retry_enabled": true,
			"retry_max": 5
		},
		"execution": {
			"model": "claude-opus-4-20250514",
			"max_iterations": 50,
			"timeout": "1h"
		},
		"git": {
			"branch_prefix": "test/",
			"commit_prefix": "[test]"
		}
	}`)
	req := httptest.NewRequest("PUT", "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response contains updated values
	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["profile"] != "safe" {
		t.Errorf("expected profile 'safe', got %v", resp["profile"])
	}

	automation := resp["automation"].(map[string]interface{})
	if automation["gates_default"] != "human" {
		t.Errorf("expected gates_default 'human', got %v", automation["gates_default"])
	}

	execution := resp["execution"].(map[string]interface{})
	if execution["model"] != "claude-opus-4-20250514" {
		t.Errorf("expected model 'claude-opus-4-20250514', got %v", execution["model"])
	}

	git := resp["git"].(map[string]interface{})
	if git["branch_prefix"] != "test/" {
		t.Errorf("expected branch_prefix 'test/', got %v", git["branch_prefix"])
	}
}

func TestUpdateConfigEndpoint_InvalidBody(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{invalid json}`)
	req := httptest.NewRequest("PUT", "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateConfigEndpoint_PartialUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create config directory
	os.MkdirAll(".orc", 0755)

	srv := New(nil)

	// Update only profile
	body := bytes.NewBufferString(`{"profile": "strict"}`)
	req := httptest.NewRequest("PUT", "/api/config", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["profile"] != "strict" {
		t.Errorf("expected profile 'strict', got %v", resp["profile"])
	}
}

// === Update Hook/Skill Tests ===

func TestUpdateHookEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{"type":"pre:tool","command":"echo updated"}`)
	req := httptest.NewRequest("PUT", "/api/hooks/nonexistent", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Update returns 400 for errors (not found is reported as error)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateHookEndpoint_InvalidBody(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("PUT", "/api/hooks/test", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateSkillEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`{"prompt":"Updated prompt"}`)
	req := httptest.NewRequest("PUT", "/api/skills/nonexistent", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	// Update returns 400 for errors (not found is reported as error)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateSkillEndpoint_InvalidBody(t *testing.T) {
	srv := New(nil)

	body := bytes.NewBufferString(`invalid json`)
	req := httptest.NewRequest("PUT", "/api/skills/test", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// === Save Prompt Success Test ===

func TestSavePromptEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .orc/prompts directory
	os.MkdirAll(".orc/prompts", 0755)

	srv := New(nil)

	body := bytes.NewBufferString(`{"content":"Custom prompt content for testing"}`)
	req := httptest.NewRequest("PUT", "/api/prompts/implement", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify prompt was saved
	content, err := os.ReadFile(".orc/prompts/implement.md")
	if err != nil {
		t.Errorf("failed to read saved prompt: %v", err)
	}
	if string(content) != "Custom prompt content for testing" {
		t.Errorf("prompt content mismatch: got %q", string(content))
	}
}

func TestDeletePromptEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .orc/prompts directory with a prompt
	os.MkdirAll(".orc/prompts", 0755)
	os.WriteFile(".orc/prompts/test.md", []byte("test content"), 0644)

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/prompts/test", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}

	// Verify file was deleted
	if _, err := os.Stat(".orc/prompts/test.md"); !os.IsNotExist(err) {
		t.Error("expected prompt file to be deleted")
	}
}

// === Publisher Test ===

func TestServerPublisher(t *testing.T) {
	srv := New(nil)

	// Publisher method should return the internal publisher
	pub := srv.Publisher()
	if pub == nil {
		t.Error("expected non-nil publisher")
	}
}

// === Get Plan Success Test ===

func TestGetPlanEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task with plan file
	taskDir := filepath.Join(".orc", "tasks", "TASK-010")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-010
title: Plan Test
status: pending
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	planYAML := `version: 1
weight: medium
description: Test plan
phases:
  - id: implement
    name: Implementation
    prompt: Do the work
`
	os.WriteFile(filepath.Join(taskDir, "plan.yaml"), []byte(planYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/TASK-010/plan", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

// === Publish Tests ===

func TestPublishWithSubscribers(t *testing.T) {
	srv := New(nil)

	// Manually add a subscriber
	ch := make(chan Event, 10)
	srv.subscribersMu.Lock()
	srv.subscribers["TASK-001"] = append(srv.subscribers["TASK-001"], ch)
	srv.subscribersMu.Unlock()

	// Publish an event
	srv.Publish("TASK-001", Event{Type: "test", Data: "hello"})

	// Check if event was received
	select {
	case evt := <-ch:
		if evt.Type != "test" {
			t.Errorf("expected event type 'test', got %q", evt.Type)
		}
	default:
		t.Error("expected to receive event")
	}
}

func TestPublishWithFullChannel(t *testing.T) {
	srv := New(nil)

	// Create a full channel (capacity 0)
	ch := make(chan Event, 0)
	srv.subscribersMu.Lock()
	srv.subscribers["TASK-001"] = append(srv.subscribers["TASK-001"], ch)
	srv.subscribersMu.Unlock()

	// Publish should not block even with full channel
	done := make(chan struct{})
	go func() {
		srv.Publish("TASK-001", Event{Type: "test", Data: "hello"})
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		// Success - did not block
	case <-time.After(100 * time.Millisecond):
		t.Error("Publish blocked on full channel")
	}
}

func TestPublishNoSubscribers(t *testing.T) {
	srv := New(nil)

	// Publish should not panic with no subscribers
	srv.Publish("NONEXISTENT", Event{Type: "test", Data: "hello"})
}

// === Run Task Additional Tests ===

func TestRunTaskEndpoint_TaskCannotRun(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task with running status
	taskDir := filepath.Join(".orc", "tasks", "TASK-011")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-011
title: Running Task
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/TASK-011/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRunTaskEndpoint_NoPlan(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task without plan file (status must allow running)
	taskDir := filepath.Join(".orc", "tasks", "TASK-012")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-012
title: No Plan Task
status: planned
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/TASK-012/run", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === Pause/Resume Tests ===

func TestPauseTaskEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create running task
	taskDir := filepath.Join(".orc", "tasks", "TASK-013")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-013
title: Running Task
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/TASK-013/pause", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "paused" {
		t.Errorf("expected status 'paused', got %q", resp["status"])
	}
}

func TestPauseTaskEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/pause", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestResumeTaskEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create paused task
	taskDir := filepath.Join(".orc", "tasks", "TASK-014")
	os.MkdirAll(taskDir, 0755)

	taskYAML := `id: TASK-014
title: Paused Task
status: paused
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/TASK-014/resume", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify response
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "resumed" {
		t.Errorf("expected status 'resumed', got %q", resp["status"])
	}
}

func TestResumeTaskEndpoint_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("POST", "/api/tasks/NONEXISTENT/resume", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === SSE Stream Tests ===

func TestStreamEndpoint_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/NONEXISTENT/stream", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

// === Additional Transcript Tests ===

func TestGetTranscriptsEndpoint_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task with empty transcripts directory
	taskDir := filepath.Join(".orc", "tasks", "TASK-TRANS-001")
	os.MkdirAll(filepath.Join(taskDir, "transcripts"), 0755)

	taskYAML := `id: TASK-TRANS-001
title: Transcript Test
status: pending
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TRANS-001/transcripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify we get an empty array
	var transcripts []interface{}
	json.NewDecoder(w.Body).Decode(&transcripts)
	if len(transcripts) != 0 {
		t.Errorf("expected empty transcripts, got %d", len(transcripts))
	}
}

func TestGetTranscriptsEndpoint_WithTranscripts(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create task with transcripts
	taskDir := filepath.Join(".orc", "tasks", "TASK-TRANS-002")
	transcriptsDir := filepath.Join(taskDir, "transcripts")
	os.MkdirAll(transcriptsDir, 0755)

	taskYAML := `id: TASK-TRANS-002
title: Transcript Test
status: running
weight: medium
created_at: 2024-01-01T00:00:00Z
updated_at: 2024-01-01T00:00:00Z
`
	os.WriteFile(filepath.Join(taskDir, "task.yaml"), []byte(taskYAML), 0644)

	// Create a transcript file
	transcriptContent := `# Phase: implement
## Iteration 1
Implementation done!
`
	os.WriteFile(filepath.Join(transcriptsDir, "implement-001.md"), []byte(transcriptContent), 0644)

	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TRANS-002/transcripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var transcripts []map[string]interface{}
	json.NewDecoder(w.Body).Decode(&transcripts)
	if len(transcripts) == 0 {
		t.Error("expected at least one transcript")
	}
}

// === Additional Create Task Tests ===

func TestCreateTaskEndpoint_WithWeight(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	body := `{"title": "Test Task", "weight": "large"}`
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["weight"] != "large" {
		t.Errorf("weight = %v, want large", resp["weight"])
	}
}

func TestCreateTaskEndpoint_WithDescription(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks", 0755)

	srv := New(nil)

	body := `{"title": "Test Task", "description": "Detailed description here"}`
	req := httptest.NewRequest("POST", "/api/tasks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["description"] != "Detailed description here" {
		t.Errorf("description = %v, want 'Detailed description here'", resp["description"])
	}
}

// === Hook CRUD Tests ===

func TestCreateHookEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .claude/hooks directory
	os.MkdirAll(".claude/hooks", 0755)

	srv := New(nil)

	// Note: hooks use "type" field not "trigger", must be a valid HookType
	body := `{"name": "test-hook", "type": "post:tool", "pattern": "*", "command": "echo hello"}`
	req := httptest.NewRequest("POST", "/api/hooks", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateHookEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create hook first (JSON format with .json extension)
	hooksDir := ".claude/hooks"
	os.MkdirAll(hooksDir, 0755)

	existingHook := `{"name": "update-hook", "type": "pre:tool", "pattern": "*", "command": "echo before"}`
	os.WriteFile(filepath.Join(hooksDir, "update-hook.json"), []byte(existingHook), 0644)

	srv := New(nil)

	body := `{"name": "update-hook", "type": "post:tool", "pattern": "*", "command": "echo after"}`
	req := httptest.NewRequest("PUT", "/api/hooks/update-hook", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteHookEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create hook to delete (JSON format with .json extension)
	hooksDir := ".claude/hooks"
	os.MkdirAll(hooksDir, 0755)

	hookContent := `{"name": "delete-hook", "type": "post:tool", "command": "echo hello"}`
	os.WriteFile(filepath.Join(hooksDir, "delete-hook.json"), []byte(hookContent), 0644)

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/hooks/delete-hook", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
}

// === Skill CRUD Tests ===

func TestCreateSkillEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create .claude/skills directory
	os.MkdirAll(".claude/skills", 0755)

	srv := New(nil)

	body := `{"name": "test-skill", "description": "A test skill", "prompt": "Do something useful"}`
	req := httptest.NewRequest("POST", "/api/skills", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSkillEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create skill first (YAML format with .yaml extension)
	skillsDir := ".claude/skills"
	os.MkdirAll(skillsDir, 0755)

	existingSkill := `name: update-skill
description: Original description
prompt: Original prompt
`
	os.WriteFile(filepath.Join(skillsDir, "update-skill.yaml"), []byte(existingSkill), 0644)

	srv := New(nil)

	body := `{"name": "update-skill", "description": "Updated description", "prompt": "Updated prompt"}`
	req := httptest.NewRequest("PUT", "/api/skills/update-skill", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteSkillEndpoint_Success(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	// Create skill to delete (YAML format with .yaml extension)
	skillsDir := ".claude/skills"
	os.MkdirAll(skillsDir, 0755)

	skillContent := `name: delete-skill
description: To be deleted
prompt: Some prompt
`
	os.WriteFile(filepath.Join(skillsDir, "delete-skill.yaml"), []byte(skillContent), 0644)

	srv := New(nil)

	req := httptest.NewRequest("DELETE", "/api/skills/delete-skill", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}
}

// === Prompt Default Tests ===

func TestGetPromptDefaultEndpoint_NotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/prompts/nonexistent/default", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

// === Project-scoped Task API Tests ===

// setupProjectTestEnv creates a temporary project with task for testing
func setupProjectTestEnv(t *testing.T) (srv *Server, projectID, taskID, cleanup string) {
	t.Helper()

	// Create temp directory structure
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "test-project")
	os.MkdirAll(filepath.Join(projectDir, ".orc", "tasks"), 0755)

	// Point orc to the temp directory first so registry path resolves correctly
	origHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)

	// Create global .orc directory where project registry lives
	globalOrcDir := filepath.Join(tmpDir, ".orc")
	os.MkdirAll(globalOrcDir, 0755)
	projectID = "test-proj-123"

	// Create projects.yaml in the correct location ($HOME/.orc/projects.yaml)
	projectsYAML := fmt.Sprintf(`projects:
  - id: %s
    name: test-project
    path: %s
    created_at: 2025-01-01T00:00:00Z
`, projectID, projectDir)
	os.WriteFile(filepath.Join(globalOrcDir, "projects.yaml"), []byte(projectsYAML), 0644)

	// Create task directory
	taskID = "TASK-001"
	taskDir := filepath.Join(projectDir, ".orc", "tasks", taskID)
	os.MkdirAll(taskDir, 0755)

	// Create task.yaml
	taskYAML := fmt.Sprintf(`id: %s
title: Test Task
weight: medium
status: running
branch: orc/%s
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
    iterations: 0
tokens:
  input_tokens: 0
  output_tokens: 0
  total_tokens: 0
`, taskID)
	os.WriteFile(filepath.Join(taskDir, "state.yaml"), []byte(stateYAML), 0644)

	srv = New(nil)

	cleanup = origHome
	return srv, projectID, taskID, cleanup
}

func cleanupProjectTestEnv(origHome string) {
	os.Setenv("HOME", origHome)
}

func TestProjectTaskPause_Success(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/pause", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "paused" {
		t.Errorf("expected status 'paused', got %q", resp["status"])
	}
}

func TestProjectTaskPause_NotRunning(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	// Modify task to be completed
	home := os.Getenv("HOME")
	taskPath := filepath.Join(home, "test-project", ".orc", "tasks", taskID, "task.yaml")
	taskYAML := fmt.Sprintf(`id: %s
title: Test Task
weight: medium
status: completed
branch: orc/%s
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
`, taskID, taskID)
	os.WriteFile(taskPath, []byte(taskYAML), 0644)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/pause", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskResume_Success(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	// Modify task to be paused
	home := os.Getenv("HOME")
	taskPath := filepath.Join(home, "test-project", ".orc", "tasks", taskID, "task.yaml")
	taskYAML := fmt.Sprintf(`id: %s
title: Test Task
weight: medium
status: paused
branch: orc/%s
created_at: 2025-01-01T00:00:00Z
updated_at: 2025-01-01T00:00:00Z
started_at: 2025-01-01T00:00:00Z
`, taskID, taskID)
	os.WriteFile(taskPath, []byte(taskYAML), 0644)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/resume", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "resumed" {
		t.Errorf("expected status 'resumed', got %q", resp["status"])
	}
}

func TestProjectTaskResume_NotPaused(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	// Task is running, not paused
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/resume", projectID, taskID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskRewind_Success(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	// Request body
	body := bytes.NewBufferString(`{"phase": "implement"}`)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/rewind", projectID, taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "rewound" {
		t.Errorf("expected status 'rewound', got %q", resp["status"])
	}
	if resp["phase"] != "implement" {
		t.Errorf("expected phase 'implement', got %q", resp["phase"])
	}
}

func TestProjectTaskRewind_InvalidPhase(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	body := bytes.NewBufferString(`{"phase": "nonexistent"}`)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/rewind", projectID, taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskRewind_MissingPhase(t *testing.T) {
	srv, projectID, taskID, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	body := bytes.NewBufferString(`{}`)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/projects/%s/tasks/%s/rewind", projectID, taskID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectTaskNotFound(t *testing.T) {
	srv, projectID, _, cleanup := setupProjectTestEnv(t)
	defer cleanupProjectTestEnv(cleanup)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/projects/%s/tasks/NONEXISTENT", projectID), nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestProjectNotFound(t *testing.T) {
	srv := New(nil)

	req := httptest.NewRequest("GET", "/api/projects/invalid-project/tasks/TASK-001", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

