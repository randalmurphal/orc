package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

func TestHandleGetEvents_NoFilters(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	pdb := backend.DB()

	// Create tasks
	_ = pdb.SaveTask(&db.Task{ID: "TASK-001", Title: "First Task", Status: "running", CreatedAt: time.Now()})
	_ = pdb.SaveTask(&db.Task{ID: "TASK-002", Title: "Second Task", Status: "running", CreatedAt: time.Now()})

	// Add events
	now := time.Now().UTC()
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: now})
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-002", EventType: "phase", Source: "test", CreatedAt: now.Add(time.Second)})
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "complete", Source: "test", CreatedAt: now.Add(2 * time.Second)})

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	req := httptest.NewRequest("GET", "/api/events", nil)
	w := httptest.NewRecorder()

	server.handleGetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response EventsListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Events) != 3 {
		t.Errorf("expected 3 events, got %d", len(response.Events))
	}
	if response.Total != 3 {
		t.Errorf("expected total=3, got %d", response.Total)
	}
	if response.HasMore {
		t.Error("expected has_more=false")
	}

	// Verify task titles are populated
	for _, e := range response.Events {
		if e.TaskID == "TASK-001" && e.TaskTitle != "First Task" {
			t.Errorf("expected title 'First Task', got '%s'", e.TaskTitle)
		}
		if e.TaskID == "TASK-002" && e.TaskTitle != "Second Task" {
			t.Errorf("expected title 'Second Task', got '%s'", e.TaskTitle)
		}
	}
}

func TestHandleGetEvents_TaskIDFilter(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	pdb := backend.DB()

	// Create tasks
	_ = pdb.SaveTask(&db.Task{ID: "TASK-001", Title: "Task 1", Status: "running", CreatedAt: time.Now()})
	_ = pdb.SaveTask(&db.Task{ID: "TASK-002", Title: "Task 2", Status: "running", CreatedAt: time.Now()})

	// Add events
	now := time.Now().UTC()
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: now})
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-002", EventType: "phase", Source: "test", CreatedAt: now.Add(time.Second)})
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "complete", Source: "test", CreatedAt: now.Add(2 * time.Second)})

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	req := httptest.NewRequest("GET", "/api/events?task_id=TASK-001", nil)
	w := httptest.NewRecorder()

	server.handleGetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response EventsListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Events) != 2 {
		t.Errorf("expected 2 events for TASK-001, got %d", len(response.Events))
	}
	if response.Total != 2 {
		t.Errorf("expected total=2, got %d", response.Total)
	}

	// Verify only TASK-001 events
	for _, e := range response.Events {
		if e.TaskID != "TASK-001" {
			t.Errorf("expected TaskID=TASK-001, got %s", e.TaskID)
		}
	}
}

func TestHandleGetEvents_InitiativeIDFilter(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	pdb := backend.DB()

	// Create tasks with initiatives
	task1 := &db.Task{ID: "TASK-001", Title: "Task 1", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-001"}
	_ = pdb.SaveTask(task1)

	task2 := &db.Task{ID: "TASK-002", Title: "Task 2", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-002"}
	_ = pdb.SaveTask(task2)

	// Add events
	now := time.Now().UTC()
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: now})
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-002", EventType: "phase", Source: "test", CreatedAt: now.Add(time.Second)})

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	req := httptest.NewRequest("GET", "/api/events?initiative_id=INIT-001", nil)
	w := httptest.NewRecorder()

	server.handleGetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response EventsListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Events) != 1 {
		t.Errorf("expected 1 event for INIT-001, got %d", len(response.Events))
	}
	if response.Total != 1 {
		t.Errorf("expected total=1, got %d", response.Total)
	}

	if response.Events[0].TaskID != "TASK-001" {
		t.Errorf("expected TASK-001, got %s", response.Events[0].TaskID)
	}
}

func TestHandleGetEvents_TimeRangeFilter(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	pdb := backend.DB()

	// Create task
	_ = pdb.SaveTask(&db.Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Add events at different times
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "event1", Source: "test", CreatedAt: baseTime})
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "event2", Source: "test", CreatedAt: baseTime.Add(1 * time.Hour)})
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "event3", Source: "test", CreatedAt: baseTime.Add(2 * time.Hour)})

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	// Query with since filter
	since := baseTime.Add(30 * time.Minute).Format(time.RFC3339)
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/events?task_id=TASK-001&since=%s", since), nil)
	w := httptest.NewRecorder()

	server.handleGetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response EventsListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Events) != 2 {
		t.Errorf("expected 2 events since 30min, got %d", len(response.Events))
	}
}

func TestHandleGetEvents_EventTypesFilter(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	pdb := backend.DB()

	// Create task
	_ = pdb.SaveTask(&db.Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Add events with different types
	now := time.Now().UTC()
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: now})
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "phase", Source: "test", CreatedAt: now.Add(time.Second)})
	_ = pdb.SaveEvent(&db.EventLog{TaskID: "TASK-001", EventType: "error", Source: "test", CreatedAt: now.Add(2 * time.Second)})

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	req := httptest.NewRequest("GET", "/api/events?task_id=TASK-001&types=state,phase", nil)
	w := httptest.NewRecorder()

	server.handleGetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response EventsListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Events) != 2 {
		t.Errorf("expected 2 events (state, phase), got %d", len(response.Events))
	}

	// Verify only state and phase types
	for _, e := range response.Events {
		if e.EventType != "state" && e.EventType != "phase" {
			t.Errorf("unexpected event type: %s", e.EventType)
		}
	}
}

func TestHandleGetEvents_Pagination(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	pdb := backend.DB()

	// Create task
	_ = pdb.SaveTask(&db.Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Add 25 events
	now := time.Now().UTC()
	for i := 0; i < 25; i++ {
		_ = pdb.SaveEvent(&db.EventLog{
			TaskID:    "TASK-001",
			EventType: "test",
			Source:    "test",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		})
	}

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	// First page
	req := httptest.NewRequest("GET", "/api/events?task_id=TASK-001&limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	server.handleGetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response EventsListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response.Events) != 10 {
		t.Errorf("expected 10 events, got %d", len(response.Events))
	}
	if response.Total != 25 {
		t.Errorf("expected total=25, got %d", response.Total)
	}
	if response.Limit != 10 {
		t.Errorf("expected limit=10, got %d", response.Limit)
	}
	if response.Offset != 0 {
		t.Errorf("expected offset=0, got %d", response.Offset)
	}
	if !response.HasMore {
		t.Error("expected has_more=true")
	}

	// Last page
	req2 := httptest.NewRequest("GET", "/api/events?task_id=TASK-001&limit=10&offset=20", nil)
	w2 := httptest.NewRecorder()
	server.handleGetEvents(w2, req2)

	var response2 EventsListResponse
	if err := json.NewDecoder(w2.Body).Decode(&response2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(response2.Events) != 5 {
		t.Errorf("expected 5 events on last page, got %d", len(response2.Events))
	}
	if response2.HasMore {
		t.Error("expected has_more=false on last page")
	}
}

func TestHandleGetEvents_InvalidParams(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	tests := []struct {
		name       string
		url        string
		wantStatus int
	}{
		{"invalid limit", "/api/events?limit=2000", http.StatusBadRequest},
		{"invalid limit negative", "/api/events?limit=-1", http.StatusBadRequest},
		{"invalid offset", "/api/events?offset=-1", http.StatusBadRequest},
		{"invalid since", "/api/events?since=not-a-date", http.StatusBadRequest},
		{"invalid until", "/api/events?until=not-a-date", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)
			w := httptest.NewRecorder()
			server.handleGetEvents(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestHandleGetEvents_EmptyResults(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	if err := os.MkdirAll(tmpDir+"/.orc", 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	cfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(tmpDir, cfg)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	req := httptest.NewRequest("GET", "/api/events?task_id=NONEXISTENT", nil)
	w := httptest.NewRecorder()

	server.handleGetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var response EventsListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response.Events == nil {
		t.Error("expected empty array, got nil")
	}
	if len(response.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(response.Events))
	}
	if response.Total != 0 {
		t.Errorf("expected total=0, got %d", response.Total)
	}
	if response.HasMore {
		t.Error("expected has_more=false")
	}
}
