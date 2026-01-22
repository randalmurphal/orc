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
	"github.com/randalmurphal/orc/internal/task"
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

func TestHandleGetEvents_CombinedFilters(t *testing.T) {
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

	// Create tasks
	task1 := task.New("TASK-001", "Task 1")
	task1.Status = task.StatusCompleted
	task2 := task.New("TASK-002", "Task 2")
	task2.Status = task.StatusRunning
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}
	if err := backend.SaveTask(task2); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create events with different tasks, types, and times
	baseTime := time.Now().UTC()
	testEvents := []*db.EventLog{
		{TaskID: "TASK-001", EventType: "phase", Source: "executor", CreatedAt: baseTime.Add(-5 * time.Hour)},
		{TaskID: "TASK-001", EventType: "transcript", Source: "executor", CreatedAt: baseTime.Add(-4 * time.Hour)},
		{TaskID: "TASK-001", EventType: "phase", Source: "executor", CreatedAt: baseTime.Add(-3 * time.Hour)},
		{TaskID: "TASK-002", EventType: "phase", Source: "executor", CreatedAt: baseTime.Add(-2 * time.Hour)},
		{TaskID: "TASK-001", EventType: "activity", Source: "executor", CreatedAt: baseTime.Add(-1 * time.Hour)},
	}

	for _, ev := range testEvents {
		if err := backend.SaveEvent(ev); err != nil {
			t.Fatalf("failed to save event: %v", err)
		}
	}

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	// Test combining task_id + event types + since
	sinceTime := baseTime.Add(-4 * time.Hour).Format(time.RFC3339)
	req := httptest.NewRequest("GET", "/api/events?task_id=TASK-001&types=phase,activity&since="+sinceTime, nil)
	w := httptest.NewRecorder()

	server.handleGetEvents(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response EventsListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Should match: TASK-001 + (phase OR activity) + after sinceTime
	// That's the phase at -3h and activity at -1h (2 events)
	if len(response.Events) != 2 {
		t.Errorf("expected 2 events with combined filters, got %d", len(response.Events))
	}

	// Verify all returned events match our filters
	for _, ev := range response.Events {
		if ev.TaskID != "TASK-001" {
			t.Errorf("expected task_id TASK-001, got %s", ev.TaskID)
		}
		if ev.EventType != "phase" && ev.EventType != "activity" {
			t.Errorf("expected event type 'phase' or 'activity', got %s", ev.EventType)
		}
	}
}

func TestHandleGetEvents_UnknownEventTypes(t *testing.T) {
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

	// Create task
	task1 := task.New("TASK-001", "Task 1")
	task1.Status = task.StatusRunning
	if err := backend.SaveTask(task1); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create events with known types
	testEvents := []*db.EventLog{
		{TaskID: "TASK-001", EventType: "phase", Source: "executor", CreatedAt: time.Now()},
		{TaskID: "TASK-001", EventType: "transcript", Source: "executor", CreatedAt: time.Now()},
	}
	for _, ev := range testEvents {
		if err := backend.SaveEvent(ev); err != nil {
			t.Fatalf("failed to save event: %v", err)
		}
	}

	server := &Server{
		backend: backend,
		logger:  testLogger(),
	}

	// Query with unknown event types - should return empty results (not error)
	req := httptest.NewRequest("GET", "/api/events?types=nonexistent_type,another_fake_type", nil)
	w := httptest.NewRecorder()

	server.handleGetEvents(w, req)

	// Should succeed with 200 OK but return no results
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response EventsListResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// No events should match unknown types
	if len(response.Events) != 0 {
		t.Errorf("expected 0 events for unknown types, got %d", len(response.Events))
	}
	if response.Total != 0 {
		t.Errorf("expected total=0, got %d", response.Total)
	}
}
