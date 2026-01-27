package db

import (
	"path/filepath"
	"testing"
	"time"
)

// TestProjectDB_SaveEvent_IgnoresDuplicates verifies that saving the same event
// twice (same task_id, phase, event_type, created_at) doesn't create duplicates.
func TestProjectDB_SaveEvent_IgnoresDuplicates(t *testing.T) {
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

	// Create task for foreign key
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Fixed timestamp for exact duplicate detection
	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	phase := "implement"

	// Create identical events
	event1 := &EventLog{
		TaskID:    "TASK-001",
		Phase:     &phase,
		EventType: "phase",
		Data:      map[string]any{"status": "running"},
		Source:    "executor",
		CreatedAt: fixedTime,
	}

	event2 := &EventLog{
		TaskID:    "TASK-001",
		Phase:     &phase,
		EventType: "phase",
		Data:      map[string]any{"status": "running"},
		Source:    "executor",
		CreatedAt: fixedTime,
	}

	// Save first event
	if err := pdb.SaveEvent(event1); err != nil {
		t.Fatalf("SaveEvent (first) failed: %v", err)
	}

	// Save duplicate event - should succeed but not create another row
	if err := pdb.SaveEvent(event2); err != nil {
		t.Fatalf("SaveEvent (duplicate) failed: %v", err)
	}

	// Query events - should only have 1
	results, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 event (deduped), got %d", len(results))
	}
}

// TestProjectDB_SaveEvents_IgnoresDuplicates verifies batch insert handles duplicates.
func TestProjectDB_SaveEvents_IgnoresDuplicates(t *testing.T) {
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

	// Create task for foreign key
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	fixedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	phase := "implement"

	// Batch of events with duplicates
	events := []*EventLog{
		{
			TaskID:    "TASK-001",
			Phase:     &phase,
			EventType: "phase",
			Data:      map[string]any{"status": "running"},
			Source:    "executor",
			CreatedAt: fixedTime,
		},
		{
			TaskID:    "TASK-001",
			Phase:     &phase,
			EventType: "phase",
			Data:      map[string]any{"status": "running"},
			Source:    "executor",
			CreatedAt: fixedTime, // Duplicate of first
		},
		{
			TaskID:    "TASK-001",
			Phase:     &phase,
			EventType: "phase",
			Data:      map[string]any{"status": "completed"},
			Source:    "executor",
			CreatedAt: fixedTime.Add(time.Second), // Different event (different timestamp)
		},
	}

	// Save batch
	if err := pdb.SaveEvents(events); err != nil {
		t.Fatalf("SaveEvents failed: %v", err)
	}

	// Query events - should have 2 (one duplicate removed)
	results, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("expected 2 events (one duplicate removed), got %d", len(results))
	}
}

// TestProjectDB_SaveEvent_AllowsDifferentTimestamps verifies events with
// different timestamps are not considered duplicates.
func TestProjectDB_SaveEvent_AllowsDifferentTimestamps(t *testing.T) {
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

	// Create task for foreign key
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	baseTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	phase := "implement"

	// Save events with different timestamps
	for i := 0; i < 3; i++ {
		event := &EventLog{
			TaskID:    "TASK-001",
			Phase:     &phase,
			EventType: "phase",
			Data:      map[string]any{"status": "running"},
			Source:    "executor",
			CreatedAt: baseTime.Add(time.Duration(i) * time.Second),
		}
		if err := pdb.SaveEvent(event); err != nil {
			t.Fatalf("SaveEvent %d failed: %v", i, err)
		}
	}

	// Should have all 3 events (different timestamps = different events)
	results, err := pdb.QueryEvents(QueryEventsOptions{TaskID: "TASK-001"})
	if err != nil {
		t.Fatalf("QueryEvents failed: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("expected 3 events (different timestamps), got %d", len(results))
	}
}
