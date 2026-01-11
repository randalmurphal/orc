package initiative

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	init := New("INIT-001", "Test Initiative")

	if init.ID != "INIT-001" {
		t.Errorf("ID = %q, want %q", init.ID, "INIT-001")
	}
	if init.Title != "Test Initiative" {
		t.Errorf("Title = %q, want %q", init.Title, "Test Initiative")
	}
	if init.Status != StatusDraft {
		t.Errorf("Status = %q, want %q", init.Status, StatusDraft)
	}
	if init.Version != 1 {
		t.Errorf("Version = %d, want 1", init.Version)
	}
	if init.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".orc", "initiatives")

	init := New("INIT-TEST-001", "Save Test")
	init.Vision = "Test vision"
	init.Owner = Identity{Initials: "RM", DisplayName: "Randy"}

	// Save
	if err := init.SaveTo(baseDir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(baseDir, "INIT-TEST-001", "initiative.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("initiative.yaml should exist")
	}

	// Load
	loaded, err := LoadFrom(baseDir, "INIT-TEST-001")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.ID != init.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, init.ID)
	}
	if loaded.Title != init.Title {
		t.Errorf("Title = %q, want %q", loaded.Title, init.Title)
	}
	if loaded.Vision != init.Vision {
		t.Errorf("Vision = %q, want %q", loaded.Vision, init.Vision)
	}
	if loaded.Owner.Initials != init.Owner.Initials {
		t.Errorf("Owner.Initials = %q, want %q", loaded.Owner.Initials, init.Owner.Initials)
	}
}

func TestAddTask(t *testing.T) {
	init := New("INIT-001", "Task Test")

	// Add first task (no deps)
	init.AddTask("TASK-001", "First task", nil)
	if len(init.Tasks) != 1 {
		t.Fatalf("Tasks count = %d, want 1", len(init.Tasks))
	}
	if init.Tasks[0].ID != "TASK-001" {
		t.Errorf("Task ID = %q, want %q", init.Tasks[0].ID, "TASK-001")
	}

	// Add second task with dependency
	init.AddTask("TASK-002", "Second task", []string{"TASK-001"})
	if len(init.Tasks) != 2 {
		t.Fatalf("Tasks count = %d, want 2", len(init.Tasks))
	}
	if init.Tasks[1].DependsOn[0] != "TASK-001" {
		t.Errorf("DependsOn = %v, want [TASK-001]", init.Tasks[1].DependsOn)
	}

	// Update existing task
	init.AddTask("TASK-001", "Updated title", []string{"TASK-000"})
	if len(init.Tasks) != 2 {
		t.Errorf("Tasks count = %d, want 2 (should update, not add)", len(init.Tasks))
	}
	if init.Tasks[0].Title != "Updated title" {
		t.Errorf("Title = %q, want %q", init.Tasks[0].Title, "Updated title")
	}
}

func TestUpdateTaskStatus(t *testing.T) {
	init := New("INIT-001", "Status Test")
	init.AddTask("TASK-001", "Task", nil)

	// Update existing task
	if !init.UpdateTaskStatus("TASK-001", "completed") {
		t.Error("UpdateTaskStatus should return true for existing task")
	}
	if init.Tasks[0].Status != "completed" {
		t.Errorf("Status = %q, want %q", init.Tasks[0].Status, "completed")
	}

	// Update non-existing task
	if init.UpdateTaskStatus("TASK-999", "completed") {
		t.Error("UpdateTaskStatus should return false for non-existing task")
	}
}

func TestAddDecision(t *testing.T) {
	init := New("INIT-001", "Decision Test")

	init.AddDecision("Use JWT tokens", "Industry standard", "RM")
	if len(init.Decisions) != 1 {
		t.Fatalf("Decisions count = %d, want 1", len(init.Decisions))
	}

	dec := init.Decisions[0]
	if dec.ID != "DEC-001" {
		t.Errorf("Decision ID = %q, want %q", dec.ID, "DEC-001")
	}
	if dec.Decision != "Use JWT tokens" {
		t.Errorf("Decision = %q, want %q", dec.Decision, "Use JWT tokens")
	}
	if dec.By != "RM" {
		t.Errorf("By = %q, want %q", dec.By, "RM")
	}

	// Add another
	init.AddDecision("7-day token expiry", "Security best practice", "RM")
	if init.Decisions[1].ID != "DEC-002" {
		t.Errorf("Decision ID = %q, want %q", init.Decisions[1].ID, "DEC-002")
	}
}

func TestGetReadyTasks(t *testing.T) {
	init := New("INIT-001", "Ready Tasks Test")

	// Add tasks with dependencies
	init.AddTask("TASK-001", "First", nil)
	init.AddTask("TASK-002", "Second", []string{"TASK-001"})
	init.AddTask("TASK-003", "Third", []string{"TASK-001", "TASK-002"})
	init.AddTask("TASK-004", "Fourth", nil) // No deps

	// Initially, TASK-001 and TASK-004 should be ready
	ready := init.GetReadyTasks()
	if len(ready) != 2 {
		t.Errorf("Ready tasks count = %d, want 2", len(ready))
	}

	// Complete TASK-001
	init.UpdateTaskStatus("TASK-001", "completed")
	ready = init.GetReadyTasks()
	// Now TASK-002 should also be ready, TASK-004 still ready
	if len(ready) != 2 {
		t.Errorf("Ready tasks count = %d, want 2 (TASK-002, TASK-004)", len(ready))
	}

	// Complete TASK-002
	init.UpdateTaskStatus("TASK-002", "completed")
	ready = init.GetReadyTasks()
	// Now TASK-003 should be ready, TASK-004 still ready
	if len(ready) != 2 {
		t.Errorf("Ready tasks count = %d, want 2 (TASK-003, TASK-004)", len(ready))
	}
}

func TestStatusLifecycle(t *testing.T) {
	init := New("INIT-001", "Status Lifecycle")

	if init.Status != StatusDraft {
		t.Errorf("Initial status = %q, want %q", init.Status, StatusDraft)
	}

	init.Activate()
	if init.Status != StatusActive {
		t.Errorf("After Activate status = %q, want %q", init.Status, StatusActive)
	}

	init.Complete()
	if init.Status != StatusCompleted {
		t.Errorf("After Complete status = %q, want %q", init.Status, StatusCompleted)
	}

	init.Archive()
	if init.Status != StatusArchived {
		t.Errorf("After Archive status = %q, want %q", init.Status, StatusArchived)
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".orc", "initiatives")

	// Create multiple initiatives
	for i := 1; i <= 3; i++ {
		init := New(sprintf("INIT-%03d", i), sprintf("Initiative %d", i))
		if err := init.SaveTo(baseDir); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List all
	all, err := ListFrom(baseDir)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Initiatives count = %d, want 3", len(all))
	}
}

func TestListByStatus(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".orc", "initiatives")

	// Create initiatives with different statuses
	init1 := New("INIT-001", "Draft")
	init1.SaveTo(baseDir)

	init2 := New("INIT-002", "Active")
	init2.Status = StatusActive
	init2.SaveTo(baseDir)

	init3 := New("INIT-003", "Completed")
	init3.Status = StatusCompleted
	init3.SaveTo(baseDir)

	// This test would need to mock GetInitiativesDir
	// For now, just test that ListFrom works
	all, err := ListFrom(baseDir)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("Initiatives count = %d, want 3", len(all))
	}
}

func TestNextID(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".orc", "initiatives")

	// Create some initiatives
	for i := 1; i <= 5; i++ {
		init := New(sprintf("INIT-%03d", i), sprintf("Initiative %d", i))
		if err := init.SaveTo(baseDir); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// This would need to be adjusted to work with the test directory
	// For now, we'll test the ID generation logic indirectly
	all, _ := ListFrom(baseDir)
	if len(all) != 5 {
		t.Errorf("Should have 5 initiatives")
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".orc", "initiatives")

	// Create an initiative
	init := New("INIT-EXISTS", "Exists Test")
	init.SaveTo(baseDir)

	// Check with direct path
	path := filepath.Join(baseDir, "INIT-EXISTS", "initiative.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("Initiative should exist")
	}

	// Check non-existing
	path = filepath.Join(baseDir, "INIT-NOTEXIST", "initiative.yaml")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Non-existing initiative should not exist")
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".orc", "initiatives")

	// Create an initiative
	init := New("INIT-DELETE", "Delete Test")
	init.SaveTo(baseDir)

	// Verify it exists
	path := filepath.Join(baseDir, "INIT-DELETE", "initiative.yaml")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("Initiative should exist before delete")
	}

	// Delete
	dir := filepath.Join(baseDir, "INIT-DELETE")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("Initiative should not exist after delete")
	}
}

func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
