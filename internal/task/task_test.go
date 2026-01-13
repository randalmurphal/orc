package task

import (
	"os"
	"path/filepath"
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
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, "TASK-001")

	// Create task directory
	err := os.MkdirAll(taskDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create and save task
	task := New("TASK-001", "Test task")
	task.Weight = WeightMedium
	task.Description = "Test description"

	err = task.SaveTo(taskDir)
	if err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	// Load task
	loaded, err := LoadFrom(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
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
}

func TestNextID(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)

	// First ID should be TASK-001 (no tasks directory yet)
	id, err := NextIDIn(tasksDir)
	if err != nil {
		t.Fatalf("NextIDIn() failed: %v", err)
	}
	if id != "TASK-001" {
		t.Errorf("NextIDIn() = %s, want TASK-001", id)
	}

	// Create tasks directory and first task
	err = os.MkdirAll(filepath.Join(tasksDir, "TASK-001"), 0755)
	if err != nil {
		t.Fatalf("failed to create task directory: %v", err)
	}

	// Second ID should be TASK-002
	id, err = NextIDIn(tasksDir)
	if err != nil {
		t.Fatalf("NextIDIn() failed: %v", err)
	}
	if id != "TASK-002" {
		t.Errorf("NextIDIn() = %s, want TASK-002", id)
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
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)

	err := os.MkdirAll(tasksDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create two tasks
	task1 := New("TASK-001", "First task")
	task1.CreatedAt = time.Now().Add(-time.Hour)
	task1.SaveTo(filepath.Join(tasksDir, "TASK-001"))

	task2 := New("TASK-002", "Second task")
	task2.CreatedAt = time.Now()
	task2.SaveTo(filepath.Join(tasksDir, "TASK-002"))

	// Load all
	tasks, err := LoadAllFrom(tasksDir)
	if err != nil {
		t.Fatalf("LoadAllFrom() failed: %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("LoadAllFrom() returned %d tasks, want 2", len(tasks))
	}

	// Should be sorted by creation time (newest first)
	if tasks[0].ID != "TASK-002" {
		t.Errorf("tasks not sorted correctly: first task is %s, want TASK-002", tasks[0].ID)
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)

	err := os.MkdirAll(tasksDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Non-existent task
	if ExistsIn(tmpDir, "TASK-999") {
		t.Error("ExistsIn() = true for non-existent task")
	}

	// Create task
	task := New("TASK-001", "Test task")
	task.SaveTo(filepath.Join(tasksDir, "TASK-001"))

	// Existing task
	if !ExistsIn(tmpDir, "TASK-001") {
		t.Error("ExistsIn() = false for existing task")
	}
}

func TestLoadNonExistentTask(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)

	err := os.MkdirAll(tasksDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	_, err = LoadFrom(tmpDir, "TASK-999")
	if err == nil {
		t.Error("LoadFrom() should return error for non-existent task")
	}
}

func TestLoadAllEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	// No tasks directory at all
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)
	tasks, err := LoadAllFrom(tasksDir)
	if err != nil {
		t.Fatalf("LoadAllFrom() on empty dir should not error: %v", err)
	}
	if len(tasks) != 0 {
		t.Error("LoadAllFrom() on empty dir should return nil/empty")
	}
}

func TestLoadAllSkipsNonDirs(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)

	err := os.MkdirAll(tasksDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create a regular file in tasks directory (should be skipped)
	err = os.WriteFile(filepath.Join(tasksDir, ".gitkeep"), []byte(""), 0644)
	if err != nil {
		t.Fatalf("failed to create .gitkeep: %v", err)
	}

	// Create a valid task
	task := New("TASK-001", "Test task")
	task.SaveTo(filepath.Join(tasksDir, "TASK-001"))

	tasks, err := LoadAllFrom(tasksDir)
	if err != nil {
		t.Fatalf("LoadAllFrom() failed: %v", err)
	}

	// Should only have the valid task, not the .gitkeep file
	if len(tasks) != 1 {
		t.Errorf("LoadAllFrom() returned %d tasks, want 1", len(tasks))
	}
}

func TestNextIDWithGaps(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)

	err := os.MkdirAll(tasksDir, 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	// Create TASK-001 and TASK-003 (gap at 002)
	os.MkdirAll(filepath.Join(tasksDir, "TASK-001"), 0755)
	os.MkdirAll(filepath.Join(tasksDir, "TASK-003"), 0755)

	// NextIDIn should give TASK-004 (highest + 1)
	id, err := NextIDIn(tasksDir)
	if err != nil {
		t.Fatalf("NextIDIn() failed: %v", err)
	}
	if id != "TASK-004" {
		t.Errorf("NextIDIn() = %s, want TASK-004", id)
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)
	os.MkdirAll(tasksDir, 0755)

	task := New("TASK-001", "Test task")
	task.Status = StatusCompleted
	task.SaveTo(filepath.Join(tasksDir, "TASK-001"))

	if !ExistsIn(tmpDir, "TASK-001") {
		t.Error("Task should exist before delete")
	}

	err := DeleteIn(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("DeleteIn() failed: %v", err)
	}

	if ExistsIn(tmpDir, "TASK-001") {
		t.Error("Task should not exist after delete")
	}
}

func TestDelete_RunningTask(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)
	os.MkdirAll(tasksDir, 0755)

	task := New("TASK-001", "Running task")
	task.Status = StatusRunning
	task.SaveTo(filepath.Join(tasksDir, "TASK-001"))

	err := DeleteIn(tmpDir, "TASK-001")
	if err == nil {
		t.Error("DeleteIn() should fail for running task")
	}

	if !ExistsIn(tmpDir, "TASK-001") {
		t.Error("Running task should still exist after failed delete")
	}
}

func TestDelete_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	tasksDir := filepath.Join(tmpDir, OrcDir, TasksDir)
	os.MkdirAll(tasksDir, 0755)

	err := DeleteIn(tmpDir, "TASK-999")
	if err == nil {
		t.Error("DeleteIn() should fail for non-existent task")
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

func TestIsValidWeight(t *testing.T) {
	tests := []struct {
		weight Weight
		valid  bool
	}{
		{WeightTrivial, true},
		{WeightSmall, true},
		{WeightMedium, true},
		{WeightLarge, true},
		{WeightGreenfield, true},
		{Weight("invalid"), false},
		{Weight(""), false},
		{Weight("huge"), false},
		{Weight("LARGE"), false}, // case-sensitive
	}

	for _, tt := range tests {
		if got := IsValidWeight(tt.weight); got != tt.valid {
			t.Errorf("IsValidWeight(%q) = %v, want %v", tt.weight, got, tt.valid)
		}
	}
}

func TestValidWeights(t *testing.T) {
	weights := ValidWeights()

	if len(weights) != 5 {
		t.Errorf("ValidWeights() returned %d weights, want 5", len(weights))
	}

	expected := []Weight{WeightTrivial, WeightSmall, WeightMedium, WeightLarge, WeightGreenfield}
	for i, w := range expected {
		if weights[i] != w {
			t.Errorf("ValidWeights()[%d] = %s, want %s", i, weights[i], w)
		}
	}
}
