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

func TestDetectUITesting(t *testing.T) {
	tests := []struct {
		title       string
		description string
		expected    bool
	}{
		{"Add login button", "", true},
		{"Fix form validation", "", true},
		{"Create user dashboard page", "", true},
		{"Add modal dialog", "", true},
		{"Update navigation menu", "", true},
		{"Implement dark mode", "", true},
		{"Fix CSS styling issue", "", true},
		{"Add responsive layout", "", true},
		{"Fix dropdown select", "", true},
		{"Update tooltip behavior", "", true},
		{"Fix database connection", "", false},
		{"Update API endpoint", "", false},
		{"Refactor auth service", "", false},
		{"Add logging", "", false},
		{"Fix memory leak in worker", "", false},
		{"", "clicking the save button should save", true},
		{"", "scroll to bottom on load", true},
		{"Backend task", "update the component registry", true},
	}

	for _, tt := range tests {
		t.Run(tt.title+tt.description, func(t *testing.T) {
			got := DetectUITesting(tt.title, tt.description)
			if got != tt.expected {
				t.Errorf("DetectUITesting(%q, %q) = %v, want %v", tt.title, tt.description, got, tt.expected)
			}
		})
	}
}

func TestSetTestingRequirements_UnitTests(t *testing.T) {
	// Trivial weight should not require unit tests
	task1 := New("TASK-001", "Fix typo")
	task1.Weight = WeightTrivial
	task1.SetTestingRequirements(false)

	if task1.TestingRequirements == nil {
		t.Fatal("expected TestingRequirements to be initialized")
	}
	if task1.TestingRequirements.Unit {
		t.Error("trivial tasks should not require unit tests")
	}

	// Non-trivial weight should require unit tests
	task2 := New("TASK-002", "Add feature")
	task2.Weight = WeightMedium
	task2.SetTestingRequirements(false)

	if !task2.TestingRequirements.Unit {
		t.Error("medium weight tasks should require unit tests")
	}
}

func TestSetTestingRequirements_E2ETests(t *testing.T) {
	// UI task in frontend project should require E2E
	task1 := New("TASK-001", "Add login button")
	task1.Weight = WeightMedium
	task1.SetTestingRequirements(true) // hasFrontend = true

	if !task1.RequiresUITesting {
		t.Error("expected RequiresUITesting=true for UI task")
	}
	if !task1.TestingRequirements.E2E {
		t.Error("UI task in frontend project should require E2E tests")
	}

	// UI task in non-frontend project should not require E2E
	task2 := New("TASK-002", "Add login button")
	task2.Weight = WeightMedium
	task2.SetTestingRequirements(false) // hasFrontend = false

	if !task2.RequiresUITesting {
		t.Error("expected RequiresUITesting=true for UI task")
	}
	if task2.TestingRequirements.E2E {
		t.Error("UI task in non-frontend project should not require E2E tests")
	}

	// Non-UI task in frontend project should not require E2E
	task3 := New("TASK-003", "Fix database query")
	task3.Weight = WeightMedium
	task3.SetTestingRequirements(true) // hasFrontend = true

	if task3.RequiresUITesting {
		t.Error("expected RequiresUITesting=false for non-UI task")
	}
	if task3.TestingRequirements.E2E {
		t.Error("non-UI task should not require E2E tests")
	}
}

func TestSetTestingRequirements_VisualTests(t *testing.T) {
	tests := []struct {
		title    string
		expected bool
	}{
		{"Update visual design", true},
		{"Fix CSS styling", true},
		{"Implement new theme", true},
		{"Update layout", true},
		{"Make responsive", true},
		{"Fix database bug", false},
		{"Add API endpoint", false},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			task := New("TASK-001", tt.title)
			task.Weight = WeightMedium
			task.SetTestingRequirements(true)

			if task.TestingRequirements.Visual != tt.expected {
				t.Errorf("Visual = %v, want %v for %q", task.TestingRequirements.Visual, tt.expected, tt.title)
			}
		})
	}
}

func TestTestingRequirements_YAMLSerialization(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, "TASK-001")
	os.MkdirAll(taskDir, 0755)

	// Create task with testing requirements
	task := New("TASK-001", "Add login button")
	task.Weight = WeightMedium
	task.RequiresUITesting = true
	task.TestingRequirements = &TestingRequirements{
		Unit:   true,
		E2E:    true,
		Visual: false,
	}

	if err := task.SaveTo(taskDir); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	// Load and verify
	loaded, err := LoadFrom(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	if !loaded.RequiresUITesting {
		t.Error("RequiresUITesting not preserved")
	}
	if loaded.TestingRequirements == nil {
		t.Fatal("TestingRequirements not preserved")
	}
	if !loaded.TestingRequirements.Unit {
		t.Error("TestingRequirements.Unit not preserved")
	}
	if !loaded.TestingRequirements.E2E {
		t.Error("TestingRequirements.E2E not preserved")
	}
	if loaded.TestingRequirements.Visual {
		t.Error("TestingRequirements.Visual incorrectly set")
	}
}

// Tests for Queue functionality

func TestNew_DefaultQueue(t *testing.T) {
	task := New("TASK-001", "Test task")

	if task.Queue != QueueActive {
		t.Errorf("expected Queue %s, got %s", QueueActive, task.Queue)
	}
}

func TestNew_DefaultPriority(t *testing.T) {
	task := New("TASK-001", "Test task")

	if task.Priority != PriorityNormal {
		t.Errorf("expected Priority %s, got %s", PriorityNormal, task.Priority)
	}
}

func TestIsValidQueue(t *testing.T) {
	tests := []struct {
		queue Queue
		valid bool
	}{
		{QueueActive, true},
		{QueueBacklog, true},
		{Queue("invalid"), false},
		{Queue(""), false},
		{Queue("ACTIVE"), false}, // case-sensitive
	}

	for _, tt := range tests {
		if got := IsValidQueue(tt.queue); got != tt.valid {
			t.Errorf("IsValidQueue(%q) = %v, want %v", tt.queue, got, tt.valid)
		}
	}
}

func TestValidQueues(t *testing.T) {
	queues := ValidQueues()

	if len(queues) != 2 {
		t.Errorf("ValidQueues() returned %d queues, want 2", len(queues))
	}

	expected := []Queue{QueueActive, QueueBacklog}
	for i, q := range expected {
		if queues[i] != q {
			t.Errorf("ValidQueues()[%d] = %s, want %s", i, queues[i], q)
		}
	}
}

func TestIsValidPriority(t *testing.T) {
	tests := []struct {
		priority Priority
		valid    bool
	}{
		{PriorityCritical, true},
		{PriorityHigh, true},
		{PriorityNormal, true},
		{PriorityLow, true},
		{Priority("invalid"), false},
		{Priority(""), false},
		{Priority("HIGH"), false}, // case-sensitive
	}

	for _, tt := range tests {
		if got := IsValidPriority(tt.priority); got != tt.valid {
			t.Errorf("IsValidPriority(%q) = %v, want %v", tt.priority, got, tt.valid)
		}
	}
}

func TestValidPriorities(t *testing.T) {
	priorities := ValidPriorities()

	if len(priorities) != 4 {
		t.Errorf("ValidPriorities() returned %d priorities, want 4", len(priorities))
	}

	expected := []Priority{PriorityCritical, PriorityHigh, PriorityNormal, PriorityLow}
	for i, p := range expected {
		if priorities[i] != p {
			t.Errorf("ValidPriorities()[%d] = %s, want %s", i, priorities[i], p)
		}
	}
}

func TestPriorityOrder(t *testing.T) {
	tests := []struct {
		priority      Priority
		expectedOrder int
	}{
		{PriorityCritical, 0},
		{PriorityHigh, 1},
		{PriorityNormal, 2},
		{PriorityLow, 3},
		{Priority("unknown"), 2}, // Defaults to normal
	}

	for _, tt := range tests {
		if got := PriorityOrder(tt.priority); got != tt.expectedOrder {
			t.Errorf("PriorityOrder(%s) = %d, want %d", tt.priority, got, tt.expectedOrder)
		}
	}

	// Test ordering: critical < high < normal < low
	if PriorityOrder(PriorityCritical) >= PriorityOrder(PriorityHigh) {
		t.Error("Critical should have lower order than High")
	}
	if PriorityOrder(PriorityHigh) >= PriorityOrder(PriorityNormal) {
		t.Error("High should have lower order than Normal")
	}
	if PriorityOrder(PriorityNormal) >= PriorityOrder(PriorityLow) {
		t.Error("Normal should have lower order than Low")
	}
}

func TestGetQueue(t *testing.T) {
	// Task with no queue set should default to active
	task1 := &Task{ID: "TASK-001"}
	if task1.GetQueue() != QueueActive {
		t.Errorf("GetQueue() for empty queue = %s, want %s", task1.GetQueue(), QueueActive)
	}

	// Task with queue set should return that queue
	task2 := &Task{ID: "TASK-002", Queue: QueueBacklog}
	if task2.GetQueue() != QueueBacklog {
		t.Errorf("GetQueue() = %s, want %s", task2.GetQueue(), QueueBacklog)
	}
}

func TestGetPriority(t *testing.T) {
	// Task with no priority set should default to normal
	task1 := &Task{ID: "TASK-001"}
	if task1.GetPriority() != PriorityNormal {
		t.Errorf("GetPriority() for empty priority = %s, want %s", task1.GetPriority(), PriorityNormal)
	}

	// Task with priority set should return that priority
	task2 := &Task{ID: "TASK-002", Priority: PriorityHigh}
	if task2.GetPriority() != PriorityHigh {
		t.Errorf("GetPriority() = %s, want %s", task2.GetPriority(), PriorityHigh)
	}
}

func TestIsBacklog(t *testing.T) {
	task1 := &Task{ID: "TASK-001", Queue: QueueActive}
	if task1.IsBacklog() {
		t.Error("IsBacklog() should return false for active queue")
	}

	task2 := &Task{ID: "TASK-002", Queue: QueueBacklog}
	if !task2.IsBacklog() {
		t.Error("IsBacklog() should return true for backlog queue")
	}

	task3 := &Task{ID: "TASK-003"} // Empty queue
	if task3.IsBacklog() {
		t.Error("IsBacklog() should return false when queue is empty (defaults to active)")
	}
}

func TestMoveToBacklog(t *testing.T) {
	task := &Task{ID: "TASK-001", Queue: QueueActive}
	task.MoveToBacklog()

	if task.Queue != QueueBacklog {
		t.Errorf("MoveToBacklog() should set Queue to %s, got %s", QueueBacklog, task.Queue)
	}
}

func TestMoveToActive(t *testing.T) {
	task := &Task{ID: "TASK-001", Queue: QueueBacklog}
	task.MoveToActive()

	if task.Queue != QueueActive {
		t.Errorf("MoveToActive() should set Queue to %s, got %s", QueueActive, task.Queue)
	}
}

func TestQueueAndPriority_YAMLSerialization(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, "TASK-001")
	os.MkdirAll(taskDir, 0755)

	// Create task with queue and priority
	task := New("TASK-001", "Test task")
	task.Queue = QueueBacklog
	task.Priority = PriorityHigh

	if err := task.SaveTo(taskDir); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	// Load and verify
	loaded, err := LoadFrom(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	if loaded.Queue != QueueBacklog {
		t.Errorf("Queue not preserved: got %s, want %s", loaded.Queue, QueueBacklog)
	}
	if loaded.Priority != PriorityHigh {
		t.Errorf("Priority not preserved: got %s, want %s", loaded.Priority, PriorityHigh)
	}
}

func TestQueueAndPriority_DefaultsAfterLoad(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, "TASK-001")
	os.MkdirAll(taskDir, 0755)

	// Create task without explicit queue/priority (simulating old tasks)
	task := &Task{
		ID:        "TASK-001",
		Title:     "Old task",
		Status:    StatusCreated,
		Branch:    "orc/TASK-001",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := task.SaveTo(taskDir); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	// Load and verify defaults work correctly
	loaded, err := LoadFrom(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	// GetQueue and GetPriority should return defaults
	if loaded.GetQueue() != QueueActive {
		t.Errorf("GetQueue() should default to active, got %s", loaded.GetQueue())
	}
	if loaded.GetPriority() != PriorityNormal {
		t.Errorf("GetPriority() should default to normal, got %s", loaded.GetPriority())
	}
}

// Tests for InitiativeID functionality

func TestNew_NoInitiative(t *testing.T) {
	task := New("TASK-001", "Test task")

	if task.InitiativeID != "" {
		t.Errorf("expected InitiativeID to be empty, got %s", task.InitiativeID)
	}
	if task.HasInitiative() {
		t.Error("HasInitiative() should return false for new task")
	}
}

func TestSetInitiative(t *testing.T) {
	task := New("TASK-001", "Test task")

	// Set initiative
	task.SetInitiative("INIT-001")
	if task.InitiativeID != "INIT-001" {
		t.Errorf("expected InitiativeID 'INIT-001', got %s", task.InitiativeID)
	}
	if !task.HasInitiative() {
		t.Error("HasInitiative() should return true after setting initiative")
	}
	if task.GetInitiativeID() != "INIT-001" {
		t.Errorf("GetInitiativeID() should return 'INIT-001', got %s", task.GetInitiativeID())
	}

	// Unlink initiative
	task.SetInitiative("")
	if task.InitiativeID != "" {
		t.Errorf("expected InitiativeID to be empty after unlinking, got %s", task.InitiativeID)
	}
	if task.HasInitiative() {
		t.Error("HasInitiative() should return false after unlinking")
	}
}

func TestGetInitiativeID(t *testing.T) {
	task := New("TASK-001", "Test task")

	// Empty by default
	if task.GetInitiativeID() != "" {
		t.Errorf("GetInitiativeID() should return empty string for new task, got %s", task.GetInitiativeID())
	}

	// Returns value when set
	task.InitiativeID = "INIT-002"
	if task.GetInitiativeID() != "INIT-002" {
		t.Errorf("GetInitiativeID() should return 'INIT-002', got %s", task.GetInitiativeID())
	}
}

func TestHasInitiative(t *testing.T) {
	tests := []struct {
		initiativeID string
		expected     bool
	}{
		{"", false},
		{"INIT-001", true},
		{"INIT-123", true},
	}

	for _, tt := range tests {
		task := &Task{ID: "TASK-001", InitiativeID: tt.initiativeID}
		if task.HasInitiative() != tt.expected {
			t.Errorf("HasInitiative() for %q = %v, want %v", tt.initiativeID, task.HasInitiative(), tt.expected)
		}
	}
}

func TestInitiativeID_YAMLSerialization(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, "TASK-001")
	os.MkdirAll(taskDir, 0755)

	// Create task with initiative
	task := New("TASK-001", "Test task")
	task.SetInitiative("INIT-001")

	if err := task.SaveTo(taskDir); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	// Load and verify
	loaded, err := LoadFrom(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	if loaded.InitiativeID != "INIT-001" {
		t.Errorf("InitiativeID not preserved: got %s, want INIT-001", loaded.InitiativeID)
	}
	if !loaded.HasInitiative() {
		t.Error("HasInitiative() should return true after load")
	}
}

func TestInitiativeID_EmptySerialization(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, "TASK-001")
	os.MkdirAll(taskDir, 0755)

	// Create task without initiative
	task := New("TASK-001", "Test task")

	if err := task.SaveTo(taskDir); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	// Load and verify
	loaded, err := LoadFrom(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	if loaded.InitiativeID != "" {
		t.Errorf("InitiativeID should be empty, got %s", loaded.InitiativeID)
	}
	if loaded.HasInitiative() {
		t.Error("HasInitiative() should return false for task without initiative")
	}
}

// Tests for dependency functionality

func TestValidateBlockedBy(t *testing.T) {
	existingIDs := map[string]bool{
		"TASK-001": true,
		"TASK-002": true,
		"TASK-003": true,
	}

	tests := []struct {
		name      string
		taskID    string
		blockedBy []string
		wantErrs  int
	}{
		{"valid references", "TASK-004", []string{"TASK-001", "TASK-002"}, 0},
		{"non-existent task", "TASK-004", []string{"TASK-999"}, 1},
		{"self-reference", "TASK-001", []string{"TASK-001"}, 1},
		{"mixed valid and invalid", "TASK-004", []string{"TASK-001", "TASK-999"}, 1},
		{"empty list", "TASK-004", []string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateBlockedBy(tt.taskID, tt.blockedBy, existingIDs)
			if len(errs) != tt.wantErrs {
				t.Errorf("ValidateBlockedBy() returned %d errors, want %d", len(errs), tt.wantErrs)
			}
		})
	}
}

func TestValidateRelatedTo(t *testing.T) {
	existingIDs := map[string]bool{
		"TASK-001": true,
		"TASK-002": true,
	}

	tests := []struct {
		name      string
		taskID    string
		relatedTo []string
		wantErrs  int
	}{
		{"valid references", "TASK-003", []string{"TASK-001", "TASK-002"}, 0},
		{"non-existent task", "TASK-003", []string{"TASK-999"}, 1},
		{"self-reference", "TASK-001", []string{"TASK-001"}, 1},
		{"empty list", "TASK-003", []string{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateRelatedTo(tt.taskID, tt.relatedTo, existingIDs)
			if len(errs) != tt.wantErrs {
				t.Errorf("ValidateRelatedTo() returned %d errors, want %d", len(errs), tt.wantErrs)
			}
		})
	}
}

func TestDetectCircularDependency(t *testing.T) {
	// Create a set of tasks with dependencies
	// TASK-001 <- TASK-002 <- TASK-003
	tasks := map[string]*Task{
		"TASK-001": {ID: "TASK-001", BlockedBy: nil},
		"TASK-002": {ID: "TASK-002", BlockedBy: []string{"TASK-001"}},
		"TASK-003": {ID: "TASK-003", BlockedBy: []string{"TASK-002"}},
	}

	tests := []struct {
		name       string
		taskID     string
		newBlocker string
		wantCycle  bool
	}{
		{"no cycle - valid dependency", "TASK-003", "TASK-001", false},
		{"cycle - TASK-001 blocked by TASK-003", "TASK-001", "TASK-003", true},
		{"cycle - TASK-001 blocked by TASK-002", "TASK-001", "TASK-002", true},
		{"no cycle - new task blocking existing", "TASK-004", "TASK-003", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add the task being tested if it doesn't exist
			if _, exists := tasks[tt.taskID]; !exists {
				tasks[tt.taskID] = &Task{ID: tt.taskID, BlockedBy: nil}
			}

			cycle := DetectCircularDependency(tt.taskID, tt.newBlocker, tasks)
			hasCycle := cycle != nil

			if hasCycle != tt.wantCycle {
				t.Errorf("DetectCircularDependency() hasCycle = %v, want %v (cycle: %v)", hasCycle, tt.wantCycle, cycle)
			}
		})
	}
}

func TestDetectCircularDependencyWithAll(t *testing.T) {
	// Create a set of tasks with dependencies
	// TASK-001 <- TASK-002 <- TASK-003
	tasks := map[string]*Task{
		"TASK-001": {ID: "TASK-001", BlockedBy: nil},
		"TASK-002": {ID: "TASK-002", BlockedBy: []string{"TASK-001"}},
		"TASK-003": {ID: "TASK-003", BlockedBy: []string{"TASK-002"}},
	}

	tests := []struct {
		name        string
		taskID      string
		newBlockers []string
		wantCycle   bool
	}{
		{"no cycle - valid single dependency", "TASK-003", []string{"TASK-001"}, false},
		{"no cycle - empty list", "TASK-001", []string{}, false},
		{"cycle - direct self via chain", "TASK-001", []string{"TASK-003"}, true},
		{"no cycle - new task with valid blockers", "TASK-004", []string{"TASK-001", "TASK-002"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Add the task being tested if it doesn't exist
			if _, exists := tasks[tt.taskID]; !exists {
				tasks[tt.taskID] = &Task{ID: tt.taskID, BlockedBy: nil}
			}

			cycle := DetectCircularDependencyWithAll(tt.taskID, tt.newBlockers, tasks)
			hasCycle := cycle != nil

			if hasCycle != tt.wantCycle {
				t.Errorf("DetectCircularDependencyWithAll() hasCycle = %v, want %v (cycle: %v)", hasCycle, tt.wantCycle, cycle)
			}
		})
	}
}

func TestComputeBlocks(t *testing.T) {
	tasks := []*Task{
		{ID: "TASK-001", BlockedBy: nil},
		{ID: "TASK-002", BlockedBy: []string{"TASK-001"}},
		{ID: "TASK-003", BlockedBy: []string{"TASK-001", "TASK-002"}},
		{ID: "TASK-004", BlockedBy: []string{"TASK-002"}},
	}

	// TASK-001 blocks TASK-002 and TASK-003
	blocks := ComputeBlocks("TASK-001", tasks)
	if len(blocks) != 2 {
		t.Errorf("ComputeBlocks(TASK-001) = %d tasks, want 2", len(blocks))
	}

	// TASK-002 blocks TASK-003 and TASK-004
	blocks = ComputeBlocks("TASK-002", tasks)
	if len(blocks) != 2 {
		t.Errorf("ComputeBlocks(TASK-002) = %d tasks, want 2", len(blocks))
	}

	// TASK-004 doesn't block anything
	blocks = ComputeBlocks("TASK-004", tasks)
	if len(blocks) != 0 {
		t.Errorf("ComputeBlocks(TASK-004) = %d tasks, want 0", len(blocks))
	}
}

func TestDetectTaskReferences(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "no references",
			text:     "This is a plain description",
			expected: nil,
		},
		{
			name:     "single reference",
			text:     "This depends on TASK-001",
			expected: []string{"TASK-001"},
		},
		{
			name:     "multiple references",
			text:     "This depends on TASK-001 and TASK-002",
			expected: []string{"TASK-001", "TASK-002"},
		},
		{
			name:     "duplicate references",
			text:     "See TASK-001 for context. Also TASK-001 is related.",
			expected: []string{"TASK-001"},
		},
		{
			name:     "mixed with text",
			text:     "Before TASK-001, then TASK-002, finally TASK-003 after text",
			expected: []string{"TASK-001", "TASK-002", "TASK-003"},
		},
		{
			name:     "4+ digit task IDs",
			text:     "Large project: TASK-1234 and TASK-99999",
			expected: []string{"TASK-1234", "TASK-99999"},
		},
		{
			name:     "too few digits ignored",
			text:     "Invalid: TASK-01 and TASK-1 should not match",
			expected: nil,
		},
		{
			name:     "word boundaries",
			text:     "MYTASK-001 and TASK-001X should not fully match but TASK-001 should",
			expected: []string{"TASK-001"},
		},
		{
			name:     "sorted output",
			text:     "TASK-003, TASK-001, TASK-002 should be sorted",
			expected: []string{"TASK-001", "TASK-002", "TASK-003"},
		},
		{
			name:     "empty string",
			text:     "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectTaskReferences(tt.text)
			if len(got) != len(tt.expected) {
				t.Errorf("DetectTaskReferences() = %v, want %v", got, tt.expected)
				return
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("DetectTaskReferences()[%d] = %s, want %s", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestComputeReferencedBy(t *testing.T) {
	tasks := []*Task{
		{ID: "TASK-001", Title: "Base task"},
		{ID: "TASK-002", Title: "Depends on TASK-001", Description: "This relates to TASK-001"},
		{ID: "TASK-003", Title: "Mentions TASK-001", Description: "See TASK-001 and TASK-002"},
		{ID: "TASK-004", Title: "No references"},
	}

	// TASK-001 is referenced by TASK-002 and TASK-003
	refs := ComputeReferencedBy("TASK-001", tasks)
	if len(refs) != 2 {
		t.Errorf("ComputeReferencedBy(TASK-001) = %d tasks, want 2, got %v", len(refs), refs)
	}

	// TASK-002 is referenced by TASK-003
	refs = ComputeReferencedBy("TASK-002", tasks)
	if len(refs) != 1 {
		t.Errorf("ComputeReferencedBy(TASK-002) = %d tasks, want 1, got %v", len(refs), refs)
	}

	// TASK-004 is not referenced by anyone
	refs = ComputeReferencedBy("TASK-004", tasks)
	if len(refs) != 0 {
		t.Errorf("ComputeReferencedBy(TASK-004) = %d tasks, want 0", len(refs))
	}
}

func TestComputeReferencedBy_ExcludesBlockedBy(t *testing.T) {
	// TASK-002 mentions TASK-001 in its description but also has it in BlockedBy
	// So TASK-001's referenced_by should NOT include TASK-002
	tasks := []*Task{
		{ID: "TASK-001", Title: "Base task"},
		{ID: "TASK-002", Title: "Blocked by TASK-001", Description: "Depends on TASK-001", BlockedBy: []string{"TASK-001"}},
		{ID: "TASK-003", Title: "Also mentions TASK-001", Description: "See TASK-001"},
	}

	refs := ComputeReferencedBy("TASK-001", tasks)

	// Should only have TASK-003, not TASK-002 (which is in blocked_by)
	if len(refs) != 1 {
		t.Errorf("ComputeReferencedBy(TASK-001) = %v, want [TASK-003]", refs)
	}
	if len(refs) > 0 && refs[0] != "TASK-003" {
		t.Errorf("ComputeReferencedBy(TASK-001)[0] = %s, want TASK-003", refs[0])
	}
}

func TestComputeReferencedBy_ExcludesRelatedTo(t *testing.T) {
	// TASK-002 mentions TASK-001 in its description but also has it in RelatedTo
	// So TASK-001's referenced_by should NOT include TASK-002
	tasks := []*Task{
		{ID: "TASK-001", Title: "Base task"},
		{ID: "TASK-002", Title: "Related to TASK-001", Description: "Relates to TASK-001", RelatedTo: []string{"TASK-001"}},
		{ID: "TASK-003", Title: "Also mentions TASK-001", Description: "See TASK-001"},
	}

	refs := ComputeReferencedBy("TASK-001", tasks)

	// Should only have TASK-003, not TASK-002 (which is in related_to)
	if len(refs) != 1 {
		t.Errorf("ComputeReferencedBy(TASK-001) = %v, want [TASK-003]", refs)
	}
	if len(refs) > 0 && refs[0] != "TASK-003" {
		t.Errorf("ComputeReferencedBy(TASK-001)[0] = %s, want TASK-003", refs[0])
	}
}

func TestComputeReferencedBy_ExcludesSelfReference(t *testing.T) {
	// A task mentioning itself should not appear in its own referenced_by
	tasks := []*Task{
		{ID: "TASK-001", Title: "Self-referencing task", Description: "This task TASK-001 refers to itself"},
		{ID: "TASK-002", Title: "Normal task"},
	}

	refs := ComputeReferencedBy("TASK-001", tasks)

	// Should be empty since the only reference is self-reference
	if len(refs) != 0 {
		t.Errorf("ComputeReferencedBy(TASK-001) = %v, want empty (self-reference excluded)", refs)
	}
}

func TestComputeReferencedBy_ExcludesBlockedByAndRelatedTo(t *testing.T) {
	// Test combining both exclusions
	tasks := []*Task{
		{ID: "TASK-001", Title: "Base task"},
		{ID: "TASK-002", Title: "Blocked", Description: "TASK-001 context", BlockedBy: []string{"TASK-001"}},
		{ID: "TASK-003", Title: "Related", Description: "TASK-001 context", RelatedTo: []string{"TASK-001"}},
		{ID: "TASK-004", Title: "Just mentions", Description: "See TASK-001"},
		{ID: "TASK-005", Title: "Both types", Description: "TASK-001 here", BlockedBy: []string{"TASK-001"}, RelatedTo: []string{"TASK-001"}},
	}

	refs := ComputeReferencedBy("TASK-001", tasks)

	// Should only have TASK-004
	if len(refs) != 1 {
		t.Errorf("ComputeReferencedBy(TASK-001) = %v, want [TASK-004]", refs)
	}
	if len(refs) > 0 && refs[0] != "TASK-004" {
		t.Errorf("ComputeReferencedBy(TASK-001)[0] = %s, want TASK-004", refs[0])
	}
}

func TestPopulateComputedFields(t *testing.T) {
	tasks := []*Task{
		{ID: "TASK-001", Title: "Base task"},
		{ID: "TASK-002", Title: "Depends on TASK-001", BlockedBy: []string{"TASK-001"}},
		{ID: "TASK-003", Title: "References TASK-001", Description: "See TASK-001"},
	}

	PopulateComputedFields(tasks)

	// TASK-001 should have Blocks = [TASK-002]
	// ReferencedBy excludes TASK-002 (it's in blocked_by), so only TASK-003
	if len(tasks[0].Blocks) != 1 || tasks[0].Blocks[0] != "TASK-002" {
		t.Errorf("TASK-001 Blocks = %v, want [TASK-002]", tasks[0].Blocks)
	}
	if len(tasks[0].ReferencedBy) != 1 || tasks[0].ReferencedBy[0] != "TASK-003" {
		t.Errorf("TASK-001 ReferencedBy = %v, want [TASK-003] (TASK-002 excluded because it's in blocked_by)", tasks[0].ReferencedBy)
	}

	// TASK-002 should have Blocks = [] (computed, wasn't populated manually)
	if len(tasks[1].Blocks) != 0 {
		t.Errorf("TASK-002 Blocks = %v, want []", tasks[1].Blocks)
	}
}

func TestHasUnmetDependencies(t *testing.T) {
	taskMap := map[string]*Task{
		"TASK-001": {ID: "TASK-001", Status: StatusCompleted},
		"TASK-002": {ID: "TASK-002", Status: StatusRunning},
		"TASK-003": {ID: "TASK-003", Status: StatusPlanned},
	}

	tests := []struct {
		name      string
		task      *Task
		wantUnmet bool
	}{
		{"no blockers", &Task{ID: "TASK-004", BlockedBy: nil}, false},
		{"completed blocker", &Task{ID: "TASK-004", BlockedBy: []string{"TASK-001"}}, false},
		{"running blocker", &Task{ID: "TASK-004", BlockedBy: []string{"TASK-002"}}, true},
		{"planned blocker", &Task{ID: "TASK-004", BlockedBy: []string{"TASK-003"}}, true},
		{"mixed blockers", &Task{ID: "TASK-004", BlockedBy: []string{"TASK-001", "TASK-002"}}, true},
		{"non-existent blocker", &Task{ID: "TASK-004", BlockedBy: []string{"TASK-999"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasUnmet := tt.task.HasUnmetDependencies(taskMap)
			if hasUnmet != tt.wantUnmet {
				t.Errorf("HasUnmetDependencies() = %v, want %v", hasUnmet, tt.wantUnmet)
			}
		})
	}
}

func TestGetUnmetDependencies(t *testing.T) {
	taskMap := map[string]*Task{
		"TASK-001": {ID: "TASK-001", Status: StatusCompleted},
		"TASK-002": {ID: "TASK-002", Status: StatusRunning},
		"TASK-003": {ID: "TASK-003", Status: StatusPlanned},
	}

	task := &Task{ID: "TASK-004", BlockedBy: []string{"TASK-001", "TASK-002", "TASK-003", "TASK-999"}}
	unmet := task.GetUnmetDependencies(taskMap)

	// Should return TASK-002, TASK-003, and TASK-999 (not completed or non-existent)
	if len(unmet) != 3 {
		t.Errorf("GetUnmetDependencies() = %v, want 3 unmet dependencies", unmet)
	}
}

func TestDependency_YAMLSerialization(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, OrcDir, TasksDir, "TASK-001")
	os.MkdirAll(taskDir, 0755)

	// Create task with dependencies
	task := New("TASK-001", "Test task")
	task.BlockedBy = []string{"TASK-002", "TASK-003"}
	task.RelatedTo = []string{"TASK-004"}
	// Blocks and ReferencedBy are computed, not stored

	if err := task.SaveTo(taskDir); err != nil {
		t.Fatalf("SaveTo() failed: %v", err)
	}

	// Load and verify stored fields
	loaded, err := LoadFrom(tmpDir, "TASK-001")
	if err != nil {
		t.Fatalf("LoadFrom() failed: %v", err)
	}

	if len(loaded.BlockedBy) != 2 {
		t.Errorf("BlockedBy not preserved: got %v, want [TASK-002 TASK-003]", loaded.BlockedBy)
	}
	if len(loaded.RelatedTo) != 1 || loaded.RelatedTo[0] != "TASK-004" {
		t.Errorf("RelatedTo not preserved: got %v, want [TASK-004]", loaded.RelatedTo)
	}

	// Computed fields should be empty after load (not persisted)
	if len(loaded.Blocks) != 0 {
		t.Errorf("Blocks should be empty after load (computed), got %v", loaded.Blocks)
	}
	if len(loaded.ReferencedBy) != 0 {
		t.Errorf("ReferencedBy should be empty after load (computed), got %v", loaded.ReferencedBy)
	}
}

func TestDependencyError(t *testing.T) {
	err := &DependencyError{
		TaskID:  "TASK-001",
		Message: "test error",
	}

	expected := "dependency error for TASK-001: test error"
	if err.Error() != expected {
		t.Errorf("DependencyError.Error() = %q, want %q", err.Error(), expected)
	}
}

func TestGetIncompleteBlockers(t *testing.T) {
	taskMap := map[string]*Task{
		"TASK-001": {ID: "TASK-001", Title: "Completed task", Status: StatusCompleted},
		"TASK-002": {ID: "TASK-002", Title: "Running task", Status: StatusRunning},
		"TASK-003": {ID: "TASK-003", Title: "Planned task", Status: StatusPlanned},
	}

	tests := []struct {
		name         string
		task         *Task
		wantBlockers int
	}{
		{
			name:         "no blockers",
			task:         &Task{ID: "TASK-004", BlockedBy: nil},
			wantBlockers: 0,
		},
		{
			name:         "completed blocker (no blockers returned)",
			task:         &Task{ID: "TASK-004", BlockedBy: []string{"TASK-001"}},
			wantBlockers: 0,
		},
		{
			name:         "running blocker",
			task:         &Task{ID: "TASK-004", BlockedBy: []string{"TASK-002"}},
			wantBlockers: 1,
		},
		{
			name:         "planned blocker",
			task:         &Task{ID: "TASK-004", BlockedBy: []string{"TASK-003"}},
			wantBlockers: 1,
		},
		{
			name:         "mixed blockers (only incomplete returned)",
			task:         &Task{ID: "TASK-004", BlockedBy: []string{"TASK-001", "TASK-002", "TASK-003"}},
			wantBlockers: 2,
		},
		{
			name:         "non-existent blocker",
			task:         &Task{ID: "TASK-004", BlockedBy: []string{"TASK-999"}},
			wantBlockers: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockers := tt.task.GetIncompleteBlockers(taskMap)
			if len(blockers) != tt.wantBlockers {
				t.Errorf("GetIncompleteBlockers() returned %d blockers, want %d", len(blockers), tt.wantBlockers)
			}
		})
	}
}

func TestGetIncompleteBlockers_ReturnsCorrectInfo(t *testing.T) {
	taskMap := map[string]*Task{
		"TASK-001": {ID: "TASK-001", Title: "Running task", Status: StatusRunning},
		"TASK-002": {ID: "TASK-002", Title: "Planned task", Status: StatusPlanned},
	}

	task := &Task{ID: "TASK-003", BlockedBy: []string{"TASK-001", "TASK-002"}}
	blockers := task.GetIncompleteBlockers(taskMap)

	if len(blockers) != 2 {
		t.Fatalf("GetIncompleteBlockers() returned %d blockers, want 2", len(blockers))
	}

	// Check first blocker
	if blockers[0].ID != "TASK-001" {
		t.Errorf("blockers[0].ID = %s, want TASK-001", blockers[0].ID)
	}
	if blockers[0].Title != "Running task" {
		t.Errorf("blockers[0].Title = %s, want 'Running task'", blockers[0].Title)
	}
	if blockers[0].Status != StatusRunning {
		t.Errorf("blockers[0].Status = %s, want %s", blockers[0].Status, StatusRunning)
	}

	// Check second blocker
	if blockers[1].ID != "TASK-002" {
		t.Errorf("blockers[1].ID = %s, want TASK-002", blockers[1].ID)
	}
	if blockers[1].Title != "Planned task" {
		t.Errorf("blockers[1].Title = %s, want 'Planned task'", blockers[1].Title)
	}
	if blockers[1].Status != StatusPlanned {
		t.Errorf("blockers[1].Status = %s, want %s", blockers[1].Status, StatusPlanned)
	}
}

func TestGetIncompleteBlockers_NonExistentTask(t *testing.T) {
	taskMap := map[string]*Task{}

	task := &Task{ID: "TASK-001", BlockedBy: []string{"TASK-999"}}
	blockers := task.GetIncompleteBlockers(taskMap)

	if len(blockers) != 1 {
		t.Fatalf("GetIncompleteBlockers() returned %d blockers, want 1", len(blockers))
	}

	if blockers[0].ID != "TASK-999" {
		t.Errorf("blockers[0].ID = %s, want TASK-999", blockers[0].ID)
	}
	if blockers[0].Title != "(task not found)" {
		t.Errorf("blockers[0].Title = %s, want '(task not found)'", blockers[0].Title)
	}
	if blockers[0].Status != "" {
		t.Errorf("blockers[0].Status = %s, want empty", blockers[0].Status)
	}
}
