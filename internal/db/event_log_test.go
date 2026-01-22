package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMigrate_EventLog(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// First migration
	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	// Verify event_log table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='event_log'").Scan(&tableName)
	if err != nil {
		t.Errorf("event_log table not created: %v", err)
	}

	// Verify indexes exist
	indexes := []string{
		"idx_event_log_task",
		"idx_event_log_task_created",
		"idx_event_log_created",
		"idx_event_log_event_type",
		"idx_event_log_timeline",
	}
	for _, idx := range indexes {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&name)
		if err != nil {
			t.Errorf("index %s not created: %v", idx, err)
		}
	}
}

func TestMigrate_EventLog_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// First migration
	if err := db.Migrate("project"); err != nil {
		t.Fatalf("First Migrate failed: %v", err)
	}

	// Second migration should not fail
	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Second Migrate (idempotent) failed: %v", err)
	}
}

func TestProjectDB_SaveEvent(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task first (for foreign key)
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Save event with all fields
	phase := "implement"
	iteration := 1
	durationMs := int64(1500)
	event := &EventLog{
		TaskID:     "TASK-001",
		Phase:      &phase,
		Iteration:  &iteration,
		EventType:  "phase",
		Data:       map[string]any{"status": "started", "phase": "implement"},
		Source:     "executor",
		CreatedAt:  time.Now().UTC(),
		DurationMs: &durationMs,
	}

	if err := pdb.SaveEvent(event); err != nil {
		t.Fatalf("SaveEvent failed: %v", err)
	}

	if event.ID == 0 {
		t.Error("event ID not set after save")
	}

	// Query back and verify nullable fields are read correctly
	results, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 event, got %d", len(results))
	}
	e := results[0]
	if e.Phase == nil || *e.Phase != "implement" {
		t.Errorf("expected phase='implement', got %v", e.Phase)
	}
	if e.Iteration == nil || *e.Iteration != 1 {
		t.Errorf("expected iteration=1, got %v", e.Iteration)
	}
	if e.DurationMs == nil || *e.DurationMs != 1500 {
		t.Errorf("expected durationMs=1500, got %v", e.DurationMs)
	}
}

func TestProjectDB_SaveEvent_NullPhaseIteration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "created", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Save task-level event (no phase/iteration)
	event := &EventLog{
		TaskID:    "TASK-001",
		Phase:     nil,
		Iteration: nil,
		EventType: "task_created",
		Data:      map[string]any{"title": "Test task"},
		Source:    "api",
		CreatedAt: time.Now().UTC(),
	}

	if err := pdb.SaveEvent(event); err != nil {
		t.Fatalf("SaveEvent with NULL phase/iteration failed: %v", err)
	}

	if event.ID == 0 {
		t.Error("event ID not set")
	}

	// Query and verify NULL fields
	events, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Phase != nil {
		t.Errorf("expected NULL phase, got %v", events[0].Phase)
	}
	if events[0].Iteration != nil {
		t.Errorf("expected NULL iteration, got %v", events[0].Iteration)
	}
}

func TestProjectDB_SaveEvent_JSONData(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Test various JSON data types
	testCases := []struct {
		name string
		data any
	}{
		{"map", map[string]any{"key": "value", "number": 42}},
		{"slice", []string{"a", "b", "c"}},
		{"nested", map[string]any{"outer": map[string]any{"inner": "value"}}},
		{"nil", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := &EventLog{
				TaskID:    "TASK-001",
				EventType: "test",
				Data:      tc.data,
				Source:    "test",
				CreatedAt: time.Now().UTC(),
			}

			if err := pdb.SaveEvent(event); err != nil {
				t.Fatalf("SaveEvent failed for %s: %v", tc.name, err)
			}
		})
	}
}

func TestProjectDB_QueryEvents_FilterByTaskID(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create tasks
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Task 1", Status: "running", CreatedAt: time.Now()})
	_ = pdb.SaveTask(&Task{ID: "TASK-002", Title: "Task 2", Status: "running", CreatedAt: time.Now()})

	// Add events for different tasks
	now := time.Now().UTC()
	events := []EventLog{
		{TaskID: "TASK-001", EventType: "state", Source: "executor", CreatedAt: now},
		{TaskID: "TASK-001", EventType: "phase", Source: "executor", CreatedAt: now.Add(time.Second)},
		{TaskID: "TASK-002", EventType: "state", Source: "executor", CreatedAt: now.Add(2 * time.Second)},
		{TaskID: "TASK-001", EventType: "complete", Source: "executor", CreatedAt: now.Add(3 * time.Second)},
	}

	for i := range events {
		if err := pdb.SaveEvent(&events[i]); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Query for TASK-001 only
	results, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 events for TASK-001, got %d", len(results))
	}
	for _, e := range results {
		if e.TaskID != "TASK-001" {
			t.Errorf("expected TaskID=TASK-001, got %s", e.TaskID)
		}
	}

	// Query for TASK-002 only
	results2, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-002"})
	if err != nil {
		t.Fatalf("QueryEvents for TASK-002 failed: %v", err)
	}
	if len(results2) != 1 {
		t.Errorf("expected 1 event for TASK-002, got %d", len(results2))
	}
}

func TestProjectDB_QueryEvents_FilterByTimeRange(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Add events at different times
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	events := []EventLog{
		{TaskID: "TASK-001", EventType: "event1", Source: "test", CreatedAt: baseTime},
		{TaskID: "TASK-001", EventType: "event2", Source: "test", CreatedAt: baseTime.Add(1 * time.Hour)},
		{TaskID: "TASK-001", EventType: "event3", Source: "test", CreatedAt: baseTime.Add(2 * time.Hour)},
		{TaskID: "TASK-001", EventType: "event4", Source: "test", CreatedAt: baseTime.Add(3 * time.Hour)},
	}

	for i := range events {
		if err := pdb.SaveEvent(&events[i]); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Query with since filter
	since := baseTime.Add(90 * time.Minute)
	results, err := pdb.QueryEvents(QueryEventsOptions{
		TaskID: "TASK-001",
		Since:  &since,
	})
	if err != nil {
		t.Fatalf("QueryEvents with since failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 events since 90min, got %d", len(results))
	}

	// Query with until filter
	until := baseTime.Add(90 * time.Minute)
	results2, err := pdb.QueryEvents(QueryEventsOptions{
		TaskID: "TASK-001",
		Until:  &until,
	})
	if err != nil {
		t.Fatalf("QueryEvents with until failed: %v", err)
	}
	if len(results2) != 2 {
		t.Errorf("expected 2 events until 90min, got %d", len(results2))
	}

	// Query with both since and until
	since2 := baseTime.Add(30 * time.Minute)
	until2 := baseTime.Add(150 * time.Minute)
	results3, err := pdb.QueryEvents(QueryEventsOptions{
		TaskID: "TASK-001",
		Since:  &since2,
		Until:  &until2,
	})
	if err != nil {
		t.Fatalf("QueryEvents with range failed: %v", err)
	}
	if len(results3) != 2 {
		t.Errorf("expected 2 events in range, got %d", len(results3))
	}
}

func TestProjectDB_QueryEvents_FilterByEventType(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Add events with different types
	now := time.Now().UTC()
	events := []EventLog{
		{TaskID: "TASK-001", EventType: "state", Source: "executor", CreatedAt: now},
		{TaskID: "TASK-001", EventType: "phase", Source: "executor", CreatedAt: now.Add(time.Second)},
		{TaskID: "TASK-001", EventType: "error", Source: "executor", CreatedAt: now.Add(2 * time.Second)},
		{TaskID: "TASK-001", EventType: "phase", Source: "executor", CreatedAt: now.Add(3 * time.Second)},
		{TaskID: "TASK-001", EventType: "complete", Source: "executor", CreatedAt: now.Add(4 * time.Second)},
	}

	for i := range events {
		if err := pdb.SaveEvent(&events[i]); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Filter by single event type
	results, err := pdb.QueryEvents(QueryEventsOptions{
		TaskID:     "TASK-001",
		EventTypes: []string{"phase"},
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 phase events, got %d", len(results))
	}

	// Filter by multiple event types
	results2, err := pdb.QueryEvents(QueryEventsOptions{
		TaskID:     "TASK-001",
		EventTypes: []string{"state", "complete"},
	})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(results2) != 2 {
		t.Errorf("expected 2 state/complete events, got %d", len(results2))
	}
}

func TestProjectDB_QueryEvents_OrderByCreatedAtDesc(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Add events in order
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	events := []EventLog{
		{TaskID: "TASK-001", EventType: "first", Source: "test", CreatedAt: baseTime},
		{TaskID: "TASK-001", EventType: "second", Source: "test", CreatedAt: baseTime.Add(1 * time.Minute)},
		{TaskID: "TASK-001", EventType: "third", Source: "test", CreatedAt: baseTime.Add(2 * time.Minute)},
	}

	for i := range events {
		if err := pdb.SaveEvent(&events[i]); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Query all events
	results, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	// Verify descending order (newest first)
	if len(results) != 3 {
		t.Fatalf("expected 3 events, got %d", len(results))
	}
	if results[0].EventType != "third" {
		t.Errorf("first result should be 'third' (newest), got %s", results[0].EventType)
	}
	if results[1].EventType != "second" {
		t.Errorf("second result should be 'second', got %s", results[1].EventType)
	}
	if results[2].EventType != "first" {
		t.Errorf("third result should be 'first' (oldest), got %s", results[2].EventType)
	}
}

func TestProjectDB_QueryEvents_Pagination(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Add 10 events
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 10; i++ {
		event := EventLog{
			TaskID:    "TASK-001",
			EventType: "event",
			Source:    "test",
			CreatedAt: baseTime.Add(time.Duration(i) * time.Minute),
			Data:      map[string]any{"index": i},
		}
		if err := pdb.SaveEvent(&event); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Test limit only
	results, err := pdb.QueryEvents(QueryEventsOptions{
		TaskID: "TASK-001",
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("QueryEvents with limit failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 events with limit, got %d", len(results))
	}

	// Test limit and offset
	results2, err := pdb.QueryEvents(QueryEventsOptions{
		TaskID: "TASK-001",
		Limit:  3,
		Offset: 3,
	})
	if err != nil {
		t.Fatalf("QueryEvents with limit+offset failed: %v", err)
	}
	if len(results2) != 3 {
		t.Errorf("expected 3 events with limit+offset, got %d", len(results2))
	}

	// Verify no overlap between pages
	for _, r1 := range results {
		for _, r2 := range results2 {
			if r1.ID == r2.ID {
				t.Error("pagination pages should not overlap")
			}
		}
	}

	// Test offset beyond available results
	results3, err := pdb.QueryEvents(QueryEventsOptions{
		TaskID: "TASK-001",
		Limit:  5,
		Offset: 100,
	})
	if err != nil {
		t.Fatalf("QueryEvents with large offset failed: %v", err)
	}
	if len(results3) != 0 {
		t.Errorf("expected 0 events with large offset, got %d", len(results3))
	}
}

func TestProjectDB_CascadeDelete_EventLog(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Add events
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		event := EventLog{
			TaskID:    "TASK-001",
			EventType: "test",
			Source:    "test",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}
		if err := pdb.SaveEvent(&event); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Verify events exist
	events, _ := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if len(events) != 5 {
		t.Fatalf("expected 5 events before delete, got %d", len(events))
	}

	// Delete task
	if err := pdb.DeleteTask("TASK-001"); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Verify events are cascade deleted
	eventsAfter, _ := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if len(eventsAfter) != 0 {
		t.Errorf("expected 0 events after cascade delete, got %d", len(eventsAfter))
	}
}

func TestProjectDB_QueryEvents_AllFilters(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create tasks
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Task 1", Status: "running", CreatedAt: time.Now()})
	_ = pdb.SaveTask(&Task{ID: "TASK-002", Title: "Task 2", Status: "running", CreatedAt: time.Now()})

	// Add varied events
	baseTime := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	allEvents := []EventLog{
		{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: baseTime},
		{TaskID: "TASK-001", EventType: "phase", Source: "test", CreatedAt: baseTime.Add(1 * time.Hour)},
		{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: baseTime.Add(2 * time.Hour)},
		{TaskID: "TASK-001", EventType: "error", Source: "test", CreatedAt: baseTime.Add(3 * time.Hour)},
		{TaskID: "TASK-002", EventType: "state", Source: "test", CreatedAt: baseTime.Add(4 * time.Hour)},
	}

	for i := range allEvents {
		if err := pdb.SaveEvent(&allEvents[i]); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Query with all filters
	// Since: 30 mins, Until: 90 mins - should only include the "phase" at 60 mins
	since := baseTime.Add(30 * time.Minute)
	until := baseTime.Add(90 * time.Minute)
	results, err := pdb.QueryEvents(QueryEventsOptions{
		TaskID:     "TASK-001",
		Since:      &since,
		Until:      &until,
		EventTypes: []string{"state", "phase"},
		Limit:      10,
		Offset:     0,
	})
	if err != nil {
		t.Fatalf("QueryEvents with all filters failed: %v", err)
	}

	// Should only return the "phase" event at baseTime+1h (60 mins)
	// Events: state@0, phase@60, state@120, error@180 (TASK-001) and state@240 (TASK-002)
	// Filter: task=TASK-001, since=30min, until=90min, types=state|phase
	// Match: only phase@60min is within [30, 90] range
	if len(results) != 1 {
		t.Errorf("expected 1 event matching all filters, got %d", len(results))
	}
	if len(results) > 0 && results[0].EventType != "phase" {
		t.Errorf("expected phase event, got %s", results[0].EventType)
	}
}

func TestProjectDB_QueryEvents_EmptyResults(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Query non-existent task
	results, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "NONEXISTENT"})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 events for nonexistent task, got %d", len(results))
	}
}

func TestProjectDB_SaveEvent_UTCTimestamps(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Save event with specific time in a non-UTC timezone
	loc, _ := time.LoadLocation("America/New_York")
	localTime := time.Date(2025, 1, 15, 10, 30, 0, 0, loc)

	event := EventLog{
		TaskID:    "TASK-001",
		EventType: "test",
		Source:    "test",
		CreatedAt: localTime,
	}

	if err := pdb.SaveEvent(&event); err != nil {
		t.Fatalf("SaveEvent failed: %v", err)
	}

	// Query and verify UTC storage
	results, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 event, got %d", len(results))
	}

	// The retrieved time should be in UTC
	retrieved := results[0].CreatedAt
	if retrieved.Location() != time.UTC {
		t.Errorf("expected UTC timezone, got %v", retrieved.Location())
	}

	// Verify the time is equivalent (same instant)
	if !retrieved.Equal(localTime.UTC()) {
		t.Errorf("times not equal: got %v, want %v", retrieved, localTime.UTC())
	}
}

func TestProjectDB_QueryEvents_NoFilters(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create tasks
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Task 1", Status: "running", CreatedAt: time.Now()})
	_ = pdb.SaveTask(&Task{ID: "TASK-002", Title: "Task 2", Status: "running", CreatedAt: time.Now()})

	// Add events for multiple tasks
	now := time.Now().UTC()
	events := []EventLog{
		{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: now},
		{TaskID: "TASK-002", EventType: "phase", Source: "test", CreatedAt: now.Add(time.Second)},
		{TaskID: "TASK-001", EventType: "complete", Source: "test", CreatedAt: now.Add(2 * time.Second)},
	}

	for i := range events {
		if err := pdb.SaveEvent(&events[i]); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Query with no filters (global timeline)
	results, err := pdb.QueryEvents(QueryEventsOptions{})
	if err != nil {
		t.Fatalf("QueryEvents with no filters failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 events total, got %d", len(results))
	}
}

func TestProjectDB_QueryEvents_InitiativeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create tasks with different initiatives
	task1 := &Task{ID: "TASK-001", Title: "Task 1", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-001"}
	_ = pdb.SaveTask(task1)

	task2 := &Task{ID: "TASK-002", Title: "Task 2", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-002"}
	_ = pdb.SaveTask(task2)

	task3 := &Task{ID: "TASK-003", Title: "Task 3", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-001"}
	_ = pdb.SaveTask(task3)

	// Add events
	now := time.Now().UTC()
	events := []EventLog{
		{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: now},
		{TaskID: "TASK-002", EventType: "phase", Source: "test", CreatedAt: now.Add(time.Second)},
		{TaskID: "TASK-003", EventType: "complete", Source: "test", CreatedAt: now.Add(2 * time.Second)},
		{TaskID: "TASK-001", EventType: "error", Source: "test", CreatedAt: now.Add(3 * time.Second)},
	}

	for i := range events {
		if err := pdb.SaveEvent(&events[i]); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Query events for INIT-001 only (should get TASK-001 and TASK-003 events)
	results, err := pdb.QueryEvents(QueryEventsOptions{InitiativeID: "INIT-001"})
	if err != nil {
		t.Fatalf("QueryEvents with InitiativeID failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 events for INIT-001, got %d", len(results))
	}

	// Verify only INIT-001 tasks are returned
	for _, e := range results {
		if e.TaskID != "TASK-001" && e.TaskID != "TASK-003" {
			t.Errorf("unexpected task %s in INIT-001 results", e.TaskID)
		}
	}

	// Query events for INIT-002 only
	results2, err := pdb.QueryEvents(QueryEventsOptions{InitiativeID: "INIT-002"})
	if err != nil {
		t.Fatalf("QueryEvents with InitiativeID failed: %v", err)
	}
	if len(results2) != 1 {
		t.Errorf("expected 1 event for INIT-002, got %d", len(results2))
	}
	if len(results2) > 0 && results2[0].TaskID != "TASK-002" {
		t.Errorf("expected TASK-002 for INIT-002, got %s", results2[0].TaskID)
	}

	// Query events for non-existent initiative
	results3, err := pdb.QueryEvents(QueryEventsOptions{InitiativeID: "INIT-999"})
	if err != nil {
		t.Fatalf("QueryEvents with non-existent InitiativeID failed: %v", err)
	}
	if len(results3) != 0 {
		t.Errorf("expected 0 events for INIT-999, got %d", len(results3))
	}

	// Query with both InitiativeID and TaskID filter
	results4, err := pdb.QueryEvents(QueryEventsOptions{
		InitiativeID: "INIT-001",
		TaskID:       "TASK-001",
	})
	if err != nil {
		t.Fatalf("QueryEvents with InitiativeID and TaskID failed: %v", err)
	}
	if len(results4) != 2 {
		t.Errorf("expected 2 events for INIT-001/TASK-001, got %d", len(results4))
	}

	// Query with InitiativeID and EventTypes filter
	results5, err := pdb.QueryEvents(QueryEventsOptions{
		InitiativeID: "INIT-001",
		EventTypes:   []string{"state", "complete"},
	})
	if err != nil {
		t.Fatalf("QueryEvents with InitiativeID and EventTypes failed: %v", err)
	}
	if len(results5) != 2 {
		t.Errorf("expected 2 events for INIT-001 with state/complete types, got %d", len(results5))
	}
}

func TestProjectDB_QueryEventsWithTitles(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create tasks
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "First Task", Status: "running", CreatedAt: time.Now()})
	_ = pdb.SaveTask(&Task{ID: "TASK-002", Title: "Second Task", Status: "running", CreatedAt: time.Now()})

	// Add events
	now := time.Now().UTC()
	events := []EventLog{
		{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: now},
		{TaskID: "TASK-002", EventType: "phase", Source: "test", CreatedAt: now.Add(time.Second)},
		{TaskID: "TASK-001", EventType: "complete", Source: "test", CreatedAt: now.Add(2 * time.Second)},
	}

	for i := range events {
		if err := pdb.SaveEvent(&events[i]); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Query events with titles
	results, err := pdb.QueryEventsWithTitles(QueryEventsOptions{})
	if err != nil {
		t.Fatalf("QueryEventsWithTitles failed: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 events, got %d", len(results))
	}

	// Verify task titles are populated
	for _, e := range results {
		if e.TaskID == "TASK-001" && e.TaskTitle != "First Task" {
			t.Errorf("expected title 'First Task', got '%s'", e.TaskTitle)
		}
		if e.TaskID == "TASK-002" && e.TaskTitle != "Second Task" {
			t.Errorf("expected title 'Second Task', got '%s'", e.TaskTitle)
		}
	}
}

func TestProjectDB_QueryEventsWithTitles_InitiativeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create tasks with different initiatives
	task1 := &Task{ID: "TASK-001", Title: "Task 1", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-001"}
	_ = pdb.SaveTask(task1)

	task2 := &Task{ID: "TASK-002", Title: "Task 2", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-002"}
	_ = pdb.SaveTask(task2)

	task3 := &Task{ID: "TASK-003", Title: "Task 3", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-001"}
	_ = pdb.SaveTask(task3)

	// Add events
	now := time.Now().UTC()
	events := []EventLog{
		{TaskID: "TASK-001", EventType: "state", Source: "test", CreatedAt: now},
		{TaskID: "TASK-002", EventType: "phase", Source: "test", CreatedAt: now.Add(time.Second)},
		{TaskID: "TASK-003", EventType: "complete", Source: "test", CreatedAt: now.Add(2 * time.Second)},
	}

	for i := range events {
		if err := pdb.SaveEvent(&events[i]); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Query events for INIT-001 only
	results, err := pdb.QueryEventsWithTitles(QueryEventsOptions{InitiativeID: "INIT-001"})
	if err != nil {
		t.Fatalf("QueryEventsWithTitles failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 events for INIT-001, got %d", len(results))
	}

	// Verify only INIT-001 tasks are returned
	for _, e := range results {
		if e.TaskID != "TASK-001" && e.TaskID != "TASK-003" {
			t.Errorf("unexpected task %s in INIT-001 results", e.TaskID)
		}
	}
}

func TestProjectDB_CountEvents(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	_ = pdb.SaveTask(&Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()})

	// Add 10 events
	now := time.Now().UTC()
	for i := 0; i < 10; i++ {
		event := EventLog{
			TaskID:    "TASK-001",
			EventType: "test",
			Source:    "test",
			CreatedAt: now.Add(time.Duration(i) * time.Minute),
		}
		if err := pdb.SaveEvent(&event); err != nil {
			t.Fatalf("SaveEvent failed: %v", err)
		}
	}

	// Count all events
	count, err := pdb.CountEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("CountEvents failed: %v", err)
	}
	if count != 10 {
		t.Errorf("expected count=10, got %d", count)
	}

	// Count with time filter
	since := now.Add(5 * time.Minute)
	count2, err := pdb.CountEvents(QueryEventsOptions{
		TaskID: "TASK-001",
		Since:  &since,
	})
	if err != nil {
		t.Fatalf("CountEvents with since failed: %v", err)
	}
	if count2 != 5 {
		t.Errorf("expected count=5 with since filter, got %d", count2)
	}
}

func TestProjectDB_CountEvents_InitiativeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create tasks with initiatives
	task1 := &Task{ID: "TASK-001", Title: "Task 1", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-001"}
	_ = pdb.SaveTask(task1)

	task2 := &Task{ID: "TASK-002", Title: "Task 2", Status: "running", CreatedAt: time.Now(), InitiativeID: "INIT-002"}
	_ = pdb.SaveTask(task2)

	// Add events
	now := time.Now().UTC()
	for i := 0; i < 3; i++ {
		_ = pdb.SaveEvent(&EventLog{TaskID: "TASK-001", EventType: "test", Source: "test", CreatedAt: now.Add(time.Duration(i) * time.Second)})
	}
	for i := 0; i < 2; i++ {
		_ = pdb.SaveEvent(&EventLog{TaskID: "TASK-002", EventType: "test", Source: "test", CreatedAt: now.Add(time.Duration(i) * time.Second)})
	}

	// Count events for INIT-001
	count, err := pdb.CountEvents(QueryEventsOptions{InitiativeID: "INIT-001"})
	if err != nil {
		t.Fatalf("CountEvents failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3 for INIT-001, got %d", count)
	}
}
