package task

import (
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	task := New("TASK-001", "Test task")

	if task.ID != "TASK-001" {
		t.Errorf("expected ID TASK-001, got %s", task.ID)
	}

	if task.Title != "Test task" {
		t.Errorf("expected Title 'Test task', got %s", task.Title)
	}

	if task.Status != StatusCreated {
		t.Errorf("expected Status %s, got %s", StatusCreated, task.Status)
	}

	if task.Branch != "orc/TASK-001" {
		t.Errorf("expected Branch 'orc/TASK-001', got %s", task.Branch)
	}

	if task.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		status   Status
		terminal bool
	}{
		{StatusCreated, false},
		{StatusClassifying, false},
		{StatusPlanned, false},
		{StatusRunning, false},
		{StatusPaused, false},
		{StatusBlocked, false},
		{StatusCompleted, true},
		{StatusFailed, true},
	}

	for _, tt := range tests {
		task := &Task{Status: tt.status}
		if task.IsTerminal() != tt.terminal {
			t.Errorf("IsTerminal() for %s = %v, want %v", tt.status, task.IsTerminal(), tt.terminal)
		}
	}
}

func TestCanRun(t *testing.T) {
	tests := []struct {
		status Status
		canRun bool
	}{
		{StatusCreated, true},
		{StatusClassifying, false},
		{StatusPlanned, true},
		{StatusRunning, false},
		{StatusPaused, true},
		{StatusBlocked, true},
		{StatusCompleted, false},
		{StatusFailed, false},
	}

	for _, tt := range tests {
		task := &Task{Status: tt.status}
		if task.CanRun() != tt.canRun {
			t.Errorf("CanRun() for %s = %v, want %v", tt.status, task.CanRun(), tt.canRun)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	oldOrcDir := OrcDir

	// Override OrcDir for testing
	defer func() {
		// Can't change const, so we test Save/Load with actual directory
	}()

	// Create .orc directory in temp
	err := os.MkdirAll(tmpDir+"/.orc/tasks", 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Change to temp directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create and save task
	task := New("TASK-001", "Test task")
	task.Weight = WeightMedium
	task.Description = "Test description"

	err = task.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load task
	loaded, err := Load("TASK-001")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.ID != task.ID {
		t.Errorf("loaded ID = %s, want %s", loaded.ID, task.ID)
	}

	if loaded.Title != task.Title {
		t.Errorf("loaded Title = %s, want %s", loaded.Title, task.Title)
	}

	if loaded.Weight != task.Weight {
		t.Errorf("loaded Weight = %s, want %s", loaded.Weight, task.Weight)
	}

	if loaded.Description != task.Description {
		t.Errorf("loaded Description = %s, want %s", loaded.Description, task.Description)
	}

	_ = oldOrcDir
}

func TestNextID(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .orc directory
	err := os.MkdirAll(tmpDir+"/.orc/tasks", 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// First ID should be TASK-001
	id, err := NextID()
	if err != nil {
		t.Fatalf("NextID() failed: %v", err)
	}
	if id != "TASK-001" {
		t.Errorf("NextID() = %s, want TASK-001", id)
	}

	// Create task directory
	os.MkdirAll(tmpDir+"/.orc/tasks/TASK-001", 0755)

	// Second ID should be TASK-002
	id, err = NextID()
	if err != nil {
		t.Fatalf("NextID() failed: %v", err)
	}
	if id != "TASK-002" {
		t.Errorf("NextID() = %s, want TASK-002", id)
	}
}

func TestTaskDir(t *testing.T) {
	dir := TaskDir("TASK-001")
	expected := ".orc/tasks/TASK-001"
	if dir != expected {
		t.Errorf("TaskDir() = %s, want %s", dir, expected)
	}
}

func TestLoadAll(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.MkdirAll(tmpDir+"/.orc/tasks", 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create two tasks
	task1 := New("TASK-001", "First task")
	task1.CreatedAt = time.Now().Add(-time.Hour)
	task1.Save()

	task2 := New("TASK-002", "Second task")
	task2.CreatedAt = time.Now()
	task2.Save()

	// Load all
	tasks, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("LoadAll() returned %d tasks, want 2", len(tasks))
	}

	// Should be sorted by creation time (newest first)
	if tasks[0].ID != "TASK-002" {
		t.Errorf("tasks not sorted correctly: first task is %s, want TASK-002", tasks[0].ID)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.MkdirAll(tmpDir+"/.orc/tasks", 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Non-existent task
	if Exists("TASK-999") {
		t.Error("Exists() = true for non-existent task")
	}

	// Create task
	task := New("TASK-001", "Test task")
	task.Save()

	// Existing task
	if !Exists("TASK-001") {
		t.Error("Exists() = false for existing task")
	}
}

func TestLoadNonExistentTask(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.MkdirAll(tmpDir+"/.orc/tasks", 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	_, err = Load("TASK-999")
	if err == nil {
		t.Error("Load() should return error for non-existent task")
	}
}

func TestLoadAllEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// No .orc directory at all
	tasks, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() on empty dir should not error: %v", err)
	}
	if len(tasks) != 0 {
		t.Error("LoadAll() on empty dir should return nil/empty")
	}
}

func TestLoadAllSkipsNonDirs(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.MkdirAll(tmpDir+"/.orc/tasks", 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create a regular file in tasks directory (should be skipped)
	os.WriteFile(tmpDir+"/.orc/tasks/.gitkeep", []byte(""), 0644)

	// Create a valid task
	task := New("TASK-001", "Test task")
	task.Save()

	tasks, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll() failed: %v", err)
	}

	// Should only have the valid task, not the .gitkeep file
	if len(tasks) != 1 {
		t.Errorf("LoadAll() returned %d tasks, want 1", len(tasks))
	}
}

func TestNextIDWithGaps(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.MkdirAll(tmpDir+"/.orc/tasks", 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create TASK-001 and TASK-003 (gap at 002)
	os.MkdirAll(tmpDir+"/.orc/tasks/TASK-001", 0755)
	os.MkdirAll(tmpDir+"/.orc/tasks/TASK-003", 0755)

	// NextID should give TASK-004 (highest + 1)
	id, err := NextID()
	if err != nil {
		t.Fatalf("NextID() failed: %v", err)
	}
	if id != "TASK-004" {
		t.Errorf("NextID() = %s, want TASK-004", id)
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(tmpDir+"/.orc/tasks", 0755)

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	task := New("TASK-001", "Test task")
	task.Status = StatusCompleted
	task.Save()

	if !Exists("TASK-001") {
		t.Error("Task should exist before delete")
	}

	err := Delete("TASK-001")
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	if Exists("TASK-001") {
		t.Error("Task should not exist after delete")
	}
}

func TestDelete_RunningTask(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(tmpDir+"/.orc/tasks", 0755)

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	task := New("TASK-001", "Running task")
	task.Status = StatusRunning
	task.Save()

	err := Delete("TASK-001")
	if err == nil {
		t.Error("Delete() should fail for running task")
	}

	if !Exists("TASK-001") {
		t.Error("Running task should still exist after failed delete")
	}
}

func TestDelete_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	os.MkdirAll(tmpDir+"/.orc/tasks", 0755)

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	err := Delete("TASK-999")
	if err == nil {
		t.Error("Delete() should fail for non-existent task")
	}
}

func TestSaveTo(t *testing.T) {
	tmpDir := t.TempDir()
	customDir := tmpDir + "/custom-project/.orc/tasks/TASK-001"

	task := New("TASK-001", "Custom save test")
	task.Weight = WeightLarge
	task.Description = "Test description"

	err := task.SaveTo(customDir)
	if err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	if _, err := os.Stat(customDir + "/task.yaml"); os.IsNotExist(err) {
		t.Error("SaveTo() did not create task.yaml")
	}
}

func TestLoadAllFrom(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := tmpDir + "/project/.orc/tasks"
	os.MkdirAll(tasksDir, 0755)

	task1 := New("TASK-001", "First task")
	task1.CreatedAt = time.Now().Add(-time.Hour)
	task1.SaveTo(tasksDir + "/TASK-001")

	task2 := New("TASK-002", "Second task")
	task2.CreatedAt = time.Now()
	task2.SaveTo(tasksDir + "/TASK-002")

	tasks, err := LoadAllFrom(tasksDir)
	if err != nil {
		t.Fatalf("LoadAllFrom() failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("LoadAllFrom() returned %d tasks, want 2", len(tasks))
	}

	if tasks[0].ID != "TASK-002" {
		t.Errorf("tasks not sorted correctly: first task is %s, want TASK-002", tasks[0].ID)
	}
}

func TestLoadAllFrom_Empty(t *testing.T) {
	tmpDir := t.TempDir()

	tasks, err := LoadAllFrom(tmpDir + "/nonexistent")
	if err != nil {
		t.Fatalf("LoadAllFrom() should not error for non-existent: %v", err)
	}
	if len(tasks) != 0 {
		t.Error("LoadAllFrom() should return nil/empty for non-existent directory")
	}
}

func TestLoadAllFrom_SkipsInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := tmpDir + "/project/.orc/tasks"
	os.MkdirAll(tasksDir, 0755)

	task := New("TASK-001", "Valid task")
	task.SaveTo(tasksDir + "/TASK-001")

	os.MkdirAll(tasksDir+"/TASK-002", 0755)
	os.WriteFile(tasksDir+"/TASK-002/task.yaml", []byte("invalid: yaml: [broken"), 0644)

	os.MkdirAll(tasksDir+"/TASK-003", 0755)

	tasks, err := LoadAllFrom(tasksDir)
	if err != nil {
		t.Fatalf("LoadAllFrom() failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Errorf("LoadAllFrom() returned %d tasks, want 1", len(tasks))
	}
}

func TestNextIDIn(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := tmpDir + "/project/.orc/tasks"

	id, err := NextIDIn(tmpDir + "/nonexistent")
	if err != nil {
		t.Fatalf("NextIDIn() failed: %v", err)
	}
	if id != "TASK-001" {
		t.Errorf("NextIDIn() = %s, want TASK-001", id)
	}

	os.MkdirAll(tasksDir, 0755)
	os.MkdirAll(tasksDir+"/TASK-001", 0755)
	os.MkdirAll(tasksDir+"/TASK-005", 0755)

	id, err = NextIDIn(tasksDir)
	if err != nil {
		t.Fatalf("NextIDIn() failed: %v", err)
	}
	if id != "TASK-006" {
		t.Errorf("NextIDIn() = %s, want TASK-006", id)
	}
}

func TestNextIDIn_SkipsNonMatching(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := tmpDir + "/project/.orc/tasks"

	os.MkdirAll(tasksDir, 0755)
	os.MkdirAll(tasksDir+"/TASK-001", 0755)
	os.MkdirAll(tasksDir+"/invalid-task", 0755)
	os.MkdirAll(tasksDir+"/FEATURE-001", 0755)

	id, err := NextIDIn(tasksDir)
	if err != nil {
		t.Fatalf("NextIDIn() failed: %v", err)
	}
	if id != "TASK-002" {
		t.Errorf("NextIDIn() = %s, want TASK-002", id)
	}
}
