package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// setupTranscriptAPITest creates a backend with a task and sample transcripts.
func setupTranscriptAPITest(t *testing.T) (*storage.DatabaseBackend, string) {
	t.Helper()

	tmpDir := t.TempDir()
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, storageCfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Create a task
	tsk := task.New("TASK-TEST", "Pagination Test")
	tsk.Status = task.StatusRunning
	tsk.Weight = "medium"
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Add sample transcripts
	pdb := backend.DB()
	baseTime := time.Now().Add(-1 * time.Hour)

	transcripts := []db.Transcript{
		{TaskID: "TASK-TEST", Phase: "spec", SessionID: "sess1", MessageUUID: "msg1", Type: "assistant", Role: "assistant", Content: "Spec message 1", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime},
		{TaskID: "TASK-TEST", Phase: "spec", SessionID: "sess1", MessageUUID: "msg2", Type: "assistant", Role: "assistant", Content: "Spec message 2", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime.Add(1 * time.Minute)},
		{TaskID: "TASK-TEST", Phase: "implement", SessionID: "sess1", MessageUUID: "msg3", Type: "assistant", Role: "assistant", Content: "Implement message 1", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime.Add(2 * time.Minute)},
		{TaskID: "TASK-TEST", Phase: "implement", SessionID: "sess1", MessageUUID: "msg4", Type: "assistant", Role: "assistant", Content: "Implement message 2", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime.Add(3 * time.Minute)},
		{TaskID: "TASK-TEST", Phase: "implement", SessionID: "sess1", MessageUUID: "msg5", Type: "assistant", Role: "assistant", Content: "Implement message 3", Model: "claude-3-5-sonnet-20241022", Timestamp: baseTime.Add(4 * time.Minute)},
	}

	for i := range transcripts {
		if err := pdb.AddTranscript(&transcripts[i]); err != nil {
			t.Fatalf("AddTranscript failed: %v", err)
		}
	}

	return backend, tmpDir
}

func TestHandleGetTranscripts_DefaultPagination(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTranscriptAPITest(t)
	defer func() { _ = backend.Close() }()

	srv := New(&Config{WorkDir: tmpDir})

	// Request without pagination params should return paginated format with defaults
	req := httptest.NewRequest("GET", "/api/tasks/TASK-TEST/transcripts", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Should be a paginated object (not array)
	var response struct {
		Transcripts []map[string]any `json:"transcripts"`
		Pagination  struct {
			TotalCount int  `json:"total_count"`
			HasMore    bool `json:"has_more"`
		} `json:"pagination"`
		Phases []map[string]any `json:"phases"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Transcripts) != 5 {
		t.Errorf("expected 5 transcripts, got %d", len(response.Transcripts))
	}
	if response.Pagination.TotalCount != 5 {
		t.Errorf("expected total_count 5, got %d", response.Pagination.TotalCount)
	}
}

func TestHandleGetTranscripts_Paginated(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTranscriptAPITest(t)
	defer func() { _ = backend.Close() }()

	srv := New(&Config{WorkDir: tmpDir})

	// Request with limit param should return paginated object
	req := httptest.NewRequest("GET", "/api/tasks/TASK-TEST/transcripts?limit=2", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Should be an object with transcripts, pagination, phases
	var response struct {
		Transcripts []map[string]any `json:"transcripts"`
		Pagination  struct {
			NextCursor *int64 `json:"next_cursor"`
			PrevCursor *int64 `json:"prev_cursor"`
			HasMore    bool   `json:"has_more"`
			TotalCount int    `json:"total_count"`
		} `json:"pagination"`
		Phases []map[string]any `json:"phases"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Transcripts) != 2 {
		t.Errorf("expected 2 transcripts, got %d", len(response.Transcripts))
	}

	if !response.Pagination.HasMore {
		t.Error("expected HasMore to be true")
	}

	if response.Pagination.TotalCount != 5 {
		t.Errorf("expected TotalCount to be 5, got %d", response.Pagination.TotalCount)
	}

	if response.Pagination.NextCursor == nil {
		t.Error("expected NextCursor to be set")
	}

	if len(response.Phases) != 2 {
		t.Errorf("expected 2 phases, got %d", len(response.Phases))
	}
}

func TestHandleGetTranscripts_InvalidLimit(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTranscriptAPITest(t)
	defer func() { _ = backend.Close() }()

	srv := New(&Config{WorkDir: tmpDir})

	tests := []struct {
		name   string
		limit  string
		status int
	}{
		{"limit too low", "0", http.StatusBadRequest},
		{"limit too high", "201", http.StatusBadRequest},
		{"limit invalid", "abc", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/tasks/TASK-TEST/transcripts?limit="+tt.limit, nil)
			w := httptest.NewRecorder()

			srv.mux.ServeHTTP(w, req)

			if w.Code != tt.status {
				t.Errorf("expected status %d, got %d: %s", tt.status, w.Code, w.Body.String())
			}
		})
	}
}

func TestHandleGetTranscripts_InvalidCursor(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTranscriptAPITest(t)
	defer func() { _ = backend.Close() }()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TEST/transcripts?cursor=invalid", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleGetTranscripts_InvalidDirection(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTranscriptAPITest(t)
	defer func() { _ = backend.Close() }()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TEST/transcripts?direction=sideways", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleGetTranscripts_PhaseFilter(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTranscriptAPITest(t)
	defer func() { _ = backend.Close() }()

	srv := New(&Config{WorkDir: tmpDir})

	req := httptest.NewRequest("GET", "/api/tasks/TASK-TEST/transcripts?phase=implement&limit=100", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response struct {
		Transcripts []map[string]any `json:"transcripts"`
		Pagination  struct {
			TotalCount int `json:"total_count"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Transcripts) != 3 {
		t.Errorf("expected 3 implement transcripts, got %d", len(response.Transcripts))
	}

	// Verify all are implement phase
	for i, tr := range response.Transcripts {
		phase, ok := tr["phase"].(string)
		if !ok || phase != "implement" {
			t.Errorf("transcript %d: expected phase 'implement', got %v", i, tr["phase"])
		}
	}

	if response.Pagination.TotalCount != 3 {
		t.Errorf("expected TotalCount to be 3, got %d", response.Pagination.TotalCount)
	}
}

func TestHandleGetTranscripts_CursorNavigation(t *testing.T) {
	t.Parallel()
	backend, tmpDir := setupTranscriptAPITest(t)
	defer func() { _ = backend.Close() }()

	srv := New(&Config{WorkDir: tmpDir})

	// Get first page
	req1 := httptest.NewRequest("GET", "/api/tasks/TASK-TEST/transcripts?limit=2", nil)
	w1 := httptest.NewRecorder()
	srv.mux.ServeHTTP(w1, req1)

	var page1 struct {
		Transcripts []map[string]any `json:"transcripts"`
		Pagination  struct {
			NextCursor *int64 `json:"next_cursor"`
		} `json:"pagination"`
	}

	if err := json.NewDecoder(w1.Body).Decode(&page1); err != nil {
		t.Fatalf("failed to decode page 1: %v", err)
	}

	if page1.Pagination.NextCursor == nil {
		t.Fatal("expected NextCursor on page 1")
	}

	// Get second page using cursor
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/api/tasks/TASK-TEST/transcripts?limit=2&cursor=%d", *page1.Pagination.NextCursor), nil)
	w2 := httptest.NewRecorder()
	srv.mux.ServeHTTP(w2, req2)

	var page2 struct {
		Transcripts []map[string]any `json:"transcripts"`
	}

	if err := json.NewDecoder(w2.Body).Decode(&page2); err != nil {
		t.Fatalf("failed to decode page 2: %v", err)
	}

	// Verify no overlap
	if len(page1.Transcripts) > 0 && len(page2.Transcripts) > 0 {
		id1 := page1.Transcripts[0]["id"]
		id2 := page2.Transcripts[0]["id"]
		if id1 == id2 {
			t.Error("pages should not have overlapping transcripts")
		}
	}
}
