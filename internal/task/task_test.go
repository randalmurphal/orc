package task

import (
	"testing"
)

// skipPersistenceTest skips tests that require file persistence.
// File I/O was removed from the task package as part of the database-only migration.
// Persistence is now tested via the storage package.
func skipPersistenceTest(t *testing.T) {
	t.Helper()
	t.Skip("skipping persistence test: file I/O removed from task package, tested via storage backend")
}

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
		{StatusFinalizing, false},
		{StatusCompleted, true},
		{StatusFailed, true},
		{StatusResolved, true},
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
		{StatusFinalizing, false},
		{StatusCompleted, false},
		{StatusFailed, false},
		{StatusResolved, false},
	}

	for _, tt := range tests {
		task := &Task{Status: tt.status}
		if task.CanRun() != tt.canRun {
			t.Errorf("CanRun() for %s = %v, want %v", tt.status, task.CanRun(), tt.canRun)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	skipPersistenceTest(t)
}

func TestNextID(t *testing.T) {
	skipPersistenceTest(t)
}

func TestTaskDir(t *testing.T) {
	dir := TaskDir("TASK-001")
	expected := ".orc/tasks/TASK-001"
	if dir != expected {
		t.Errorf("TaskDir() = %s, want %s", dir, expected)
	}
}

func TestLoadAll(t *testing.T) {
	skipPersistenceTest(t)
}

func TestExists(t *testing.T) {
	skipPersistenceTest(t)
}

func TestLoadNonExistentTask(t *testing.T) {
	skipPersistenceTest(t)
}

func TestLoadAllEmpty(t *testing.T) {
	skipPersistenceTest(t)
}

func TestLoadAllSkipsNonDirs(t *testing.T) {
	skipPersistenceTest(t)
}

func TestNextIDWithGaps(t *testing.T) {
	skipPersistenceTest(t)
}

func TestDelete(t *testing.T) {
	skipPersistenceTest(t)
}

func TestDelete_RunningTask(t *testing.T) {
	skipPersistenceTest(t)
}

func TestDelete_NonExistent(t *testing.T) {
	skipPersistenceTest(t)
}

func TestSaveTo(t *testing.T) {
	skipPersistenceTest(t)
}

func TestLoadAllFrom(t *testing.T) {
	skipPersistenceTest(t)
}

func TestLoadAllFrom_Empty(t *testing.T) {
	skipPersistenceTest(t)
}

func TestLoadAllFrom_SkipsInvalid(t *testing.T) {
	skipPersistenceTest(t)
}

func TestNextIDIn(t *testing.T) {
	skipPersistenceTest(t)
}

func TestNextIDIn_SkipsNonMatching(t *testing.T) {
	skipPersistenceTest(t)
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

	if len(weights) != 4 {
		t.Errorf("ValidWeights() returned %d weights, want 4", len(weights))
	}

	expected := []Weight{WeightTrivial, WeightSmall, WeightMedium, WeightLarge}
	for i, w := range expected {
		if weights[i] != w {
			t.Errorf("ValidWeights()[%d] = %s, want %s", i, weights[i], w)
		}
	}
}

func TestIsValidStatus(t *testing.T) {
	tests := []struct {
		status Status
		valid  bool
	}{
		{StatusCreated, true},
		{StatusClassifying, true},
		{StatusPlanned, true},
		{StatusRunning, true},
		{StatusPaused, true},
		{StatusBlocked, true},
		{StatusFinalizing, true},
		{StatusCompleted, true},
		{StatusFailed, true},
		{StatusResolved, true},
		{Status("invalid"), false},
		{Status(""), false},
		{Status("COMPLETED"), false}, // case-sensitive
	}

	for _, tt := range tests {
		if got := IsValidStatus(tt.status); got != tt.valid {
			t.Errorf("IsValidStatus(%q) = %v, want %v", tt.status, got, tt.valid)
		}
	}
}

func TestValidStatuses(t *testing.T) {
	statuses := ValidStatuses()

	if len(statuses) != 10 {
		t.Errorf("ValidStatuses() returned %d statuses, want 10", len(statuses))
	}

	expected := []Status{
		StatusCreated, StatusClassifying, StatusPlanned, StatusRunning,
		StatusPaused, StatusBlocked, StatusFinalizing, StatusCompleted,
		StatusFailed, StatusResolved,
	}
	for i, s := range expected {
		if statuses[i] != s {
			t.Errorf("ValidStatuses()[%d] = %s, want %s", i, statuses[i], s)
		}
	}
}

func TestDetectUITesting(t *testing.T) {
	tests := []struct {
		title       string
		description string
		expected    bool
	}{
		// Should detect UI testing
		{"Add login button", "", true},
		{"Fix form validation", "", true},
		{"Create user dashboard page", "", true},
		{"Add modal dialog", "", true},
		{"Update sidebar navigation", "", true},
		{"Implement dark mode", "", true},
		{"Fix CSS styling issue", "", true},
		{"Add responsive layout", "", true},
		{"Fix dropdown select", "", true},
		{"Update tooltip behavior", "", true},
		{"Add click handler for submit", "", true},
		{"Fix the form input field", "", true},
		{"Backend task", "update the component registry", true},
		{"", "add aria labels for accessibility", true},

		// Should NOT detect UI testing (false positives we're avoiding)
		{"Fix database connection", "", false},
		{"Update API endpoint", "", false},
		{"Refactor auth service", "", false},
		{"Add logging", "", false},
		{"Fix memory leak in worker", "", false},
		{"Give users quick visibility into changes", "", false}, // "quick" should not match "click"
		{"Transform data before saving", "", false},             // "transform" should not match "form"
		{"Perform data transformation", "", false},              // "perform" should not match "form"
		{"Display output information", "", false},               // generic words
		{"Built-in feature for users", "", false},               // "built" should not match "ui"
		{"Required functionality", "", false},                   // "required" should not match "ui"
		{"Add scrolling behavior to list", "", false},           // "scrolling" is not a keyword
		{"Page load optimization", "", false},                   // "page" removed (too generic)
		{"", "clicking saves the configuration", false},         // "clicking" != "click"
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
	skipPersistenceTest(t)
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
	skipPersistenceTest(t)
}

func TestQueueAndPriority_DefaultsAfterLoad(t *testing.T) {
	skipPersistenceTest(t)
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
	skipPersistenceTest(t)
}

func TestInitiativeID_EmptySerialization(t *testing.T) {
	skipPersistenceTest(t)
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
		{ID: "TASK-001", Title: "Base task", Status: StatusPlanned},
		{ID: "TASK-002", Title: "Depends on TASK-001", BlockedBy: []string{"TASK-001"}, Status: StatusPlanned},
		{ID: "TASK-003", Title: "References TASK-001", Description: "See TASK-001", Status: StatusPlanned},
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

	// TASK-001 should not be blocked (no BlockedBy)
	if tasks[0].IsBlocked {
		t.Errorf("TASK-001 IsBlocked = true, want false")
	}
	if len(tasks[0].UnmetBlockers) != 0 {
		t.Errorf("TASK-001 UnmetBlockers = %v, want []", tasks[0].UnmetBlockers)
	}

	// TASK-002 should be blocked (TASK-001 is not completed)
	if !tasks[1].IsBlocked {
		t.Errorf("TASK-002 IsBlocked = false, want true")
	}
	if len(tasks[1].UnmetBlockers) != 1 || tasks[1].UnmetBlockers[0] != "TASK-001" {
		t.Errorf("TASK-002 UnmetBlockers = %v, want [TASK-001]", tasks[1].UnmetBlockers)
	}
}

func TestPopulateComputedFields_BlockedByCompleted(t *testing.T) {
	// Test that IsBlocked is false when all blockers are completed
	tasks := []*Task{
		{ID: "TASK-001", Title: "Completed task", Status: StatusCompleted},
		{ID: "TASK-002", Title: "Depends on completed task", BlockedBy: []string{"TASK-001"}, Status: StatusPlanned},
	}

	PopulateComputedFields(tasks)

	// TASK-002 should NOT be blocked (TASK-001 is completed)
	if tasks[1].IsBlocked {
		t.Errorf("TASK-002 IsBlocked = true, want false (blocker is completed)")
	}
	if len(tasks[1].UnmetBlockers) != 0 {
		t.Errorf("TASK-002 UnmetBlockers = %v, want []", tasks[1].UnmetBlockers)
	}
}

func TestPopulateComputedFields_MixedBlockers(t *testing.T) {
	// Test with mix of completed and incomplete blockers
	tasks := []*Task{
		{ID: "TASK-001", Title: "Completed task", Status: StatusCompleted},
		{ID: "TASK-002", Title: "Running task", Status: StatusRunning},
		{ID: "TASK-003", Title: "Depends on both", BlockedBy: []string{"TASK-001", "TASK-002"}, Status: StatusPlanned},
	}

	PopulateComputedFields(tasks)

	// TASK-003 should be blocked (TASK-002 is not completed)
	if !tasks[2].IsBlocked {
		t.Errorf("TASK-003 IsBlocked = false, want true (has one incomplete blocker)")
	}
	if len(tasks[2].UnmetBlockers) != 1 || tasks[2].UnmetBlockers[0] != "TASK-002" {
		t.Errorf("TASK-003 UnmetBlockers = %v, want [TASK-002]", tasks[2].UnmetBlockers)
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
	// TASK-001 (completed) is met
	if len(unmet) != 3 {
		t.Errorf("GetUnmetDependencies() = %v, want 3 unmet dependencies", unmet)
	}
}

func TestDependency_YAMLSerialization(t *testing.T) {
	skipPersistenceTest(t)
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

// Tests for PR Status functionality

func TestIsValidPRStatus(t *testing.T) {
	tests := []struct {
		status PRStatus
		valid  bool
	}{
		{PRStatusNone, true},
		{PRStatusDraft, true},
		{PRStatusPendingReview, true},
		{PRStatusChangesRequested, true},
		{PRStatusApproved, true},
		{PRStatusMerged, true},
		{PRStatusClosed, true},
		{"invalid", false},
		{"random", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := IsValidPRStatus(tt.status); got != tt.valid {
				t.Errorf("IsValidPRStatus(%s) = %v, want %v", tt.status, got, tt.valid)
			}
		})
	}
}

func TestValidPRStatuses(t *testing.T) {
	statuses := ValidPRStatuses()
	if len(statuses) != 7 {
		t.Errorf("ValidPRStatuses() returned %d statuses, want 7", len(statuses))
	}

	// Verify all statuses are valid
	for _, s := range statuses {
		if !IsValidPRStatus(s) {
			t.Errorf("ValidPRStatuses() includes invalid status: %s", s)
		}
	}
}

func TestHasPR(t *testing.T) {
	// Task without PR
	task1 := New("TASK-001", "Test task")
	if task1.HasPR() {
		t.Error("HasPR() should return false for task without PR")
	}

	// Task with empty PR
	task2 := New("TASK-002", "Test task")
	task2.PR = &PRInfo{}
	if task2.HasPR() {
		t.Error("HasPR() should return false for task with empty PR")
	}

	// Task with valid PR
	task3 := New("TASK-003", "Test task")
	task3.PR = &PRInfo{URL: "https://github.com/owner/repo/pull/123"}
	if !task3.HasPR() {
		t.Error("HasPR() should return true for task with PR URL")
	}
}

func TestGetPRStatus(t *testing.T) {
	// Task without PR
	task1 := New("TASK-001", "Test task")
	if task1.GetPRStatus() != PRStatusNone {
		t.Errorf("GetPRStatus() should return PRStatusNone for task without PR, got %s", task1.GetPRStatus())
	}

	// Task with PR status
	task2 := New("TASK-002", "Test task")
	task2.PR = &PRInfo{Status: PRStatusApproved}
	if task2.GetPRStatus() != PRStatusApproved {
		t.Errorf("GetPRStatus() = %s, want %s", task2.GetPRStatus(), PRStatusApproved)
	}
}

func TestSetPRInfo(t *testing.T) {
	task := New("TASK-001", "Test task")

	// Set PR info
	task.SetPRInfo("https://github.com/owner/repo/pull/123", 123)

	if task.PR == nil {
		t.Fatal("SetPRInfo() should create PR struct")
	}
	if task.PR.URL != "https://github.com/owner/repo/pull/123" {
		t.Errorf("PR.URL = %s, want https://github.com/owner/repo/pull/123", task.PR.URL)
	}
	if task.PR.Number != 123 {
		t.Errorf("PR.Number = %d, want 123", task.PR.Number)
	}
	// Should default to pending_review
	if task.PR.Status != PRStatusPendingReview {
		t.Errorf("PR.Status = %s, want %s", task.PR.Status, PRStatusPendingReview)
	}

	// Update existing PR info should preserve status
	task.PR.Status = PRStatusApproved
	task.SetPRInfo("https://github.com/owner/repo/pull/124", 124)
	if task.PR.Status != PRStatusApproved {
		t.Errorf("SetPRInfo() should preserve existing status, got %s", task.PR.Status)
	}
}

func TestUpdatePRStatus(t *testing.T) {
	task := New("TASK-001", "Test task")

	// Update PR status creates PR struct if needed
	task.UpdatePRStatus(PRStatusApproved, "success", true, 2, 2)

	if task.PR == nil {
		t.Fatal("UpdatePRStatus() should create PR struct")
	}
	if task.PR.Status != PRStatusApproved {
		t.Errorf("PR.Status = %s, want %s", task.PR.Status, PRStatusApproved)
	}
	if task.PR.ChecksStatus != "success" {
		t.Errorf("PR.ChecksStatus = %s, want success", task.PR.ChecksStatus)
	}
	if !task.PR.Mergeable {
		t.Error("PR.Mergeable should be true")
	}
	if task.PR.ReviewCount != 2 {
		t.Errorf("PR.ReviewCount = %d, want 2", task.PR.ReviewCount)
	}
	if task.PR.ApprovalCount != 2 {
		t.Errorf("PR.ApprovalCount = %d, want 2", task.PR.ApprovalCount)
	}
	if task.PR.LastCheckedAt == nil {
		t.Error("PR.LastCheckedAt should be set")
	}
}

func TestPRInfo_YAMLSerialization(t *testing.T) {
	skipPersistenceTest(t)
}

func TestPRInfo_EmptyPreserved(t *testing.T) {
	skipPersistenceTest(t)
}

// Tests for DependencyStatus functionality

func TestValidDependencyStatuses(t *testing.T) {
	statuses := ValidDependencyStatuses()

	if len(statuses) != 3 {
		t.Errorf("ValidDependencyStatuses() returned %d statuses, want 3", len(statuses))
	}

	expected := []DependencyStatus{DependencyStatusBlocked, DependencyStatusReady, DependencyStatusNone}
	for i, s := range expected {
		if statuses[i] != s {
			t.Errorf("ValidDependencyStatuses()[%d] = %s, want %s", i, statuses[i], s)
		}
	}
}

func TestIsValidDependencyStatus(t *testing.T) {
	tests := []struct {
		status DependencyStatus
		valid  bool
	}{
		{DependencyStatusBlocked, true},
		{DependencyStatusReady, true},
		{DependencyStatusNone, true},
		{DependencyStatus("invalid"), false},
		{DependencyStatus(""), false},
		{DependencyStatus("BLOCKED"), false}, // case-sensitive
	}

	for _, tt := range tests {
		if got := IsValidDependencyStatus(tt.status); got != tt.valid {
			t.Errorf("IsValidDependencyStatus(%q) = %v, want %v", tt.status, got, tt.valid)
		}
	}
}

func TestComputeDependencyStatus(t *testing.T) {
	tests := []struct {
		name          string
		blockedBy     []string
		unmetBlockers []string
		expected      DependencyStatus
	}{
		{
			name:          "no dependencies",
			blockedBy:     nil,
			unmetBlockers: nil,
			expected:      DependencyStatusNone,
		},
		{
			name:          "empty blocked_by",
			blockedBy:     []string{},
			unmetBlockers: nil,
			expected:      DependencyStatusNone,
		},
		{
			name:          "blocked - has unmet dependencies",
			blockedBy:     []string{"TASK-001"},
			unmetBlockers: []string{"TASK-001"},
			expected:      DependencyStatusBlocked,
		},
		{
			name:          "ready - all dependencies met",
			blockedBy:     []string{"TASK-001"},
			unmetBlockers: nil,
			expected:      DependencyStatusReady,
		},
		{
			name:          "ready - multiple deps all satisfied",
			blockedBy:     []string{"TASK-001", "TASK-002"},
			unmetBlockers: []string{},
			expected:      DependencyStatusReady,
		},
		{
			name:          "blocked - some deps unmet",
			blockedBy:     []string{"TASK-001", "TASK-002"},
			unmetBlockers: []string{"TASK-002"},
			expected:      DependencyStatusBlocked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				ID:            "TASK-TEST",
				BlockedBy:     tt.blockedBy,
				UnmetBlockers: tt.unmetBlockers,
			}
			got := task.ComputeDependencyStatus()
			if got != tt.expected {
				t.Errorf("ComputeDependencyStatus() = %s, want %s", got, tt.expected)
			}
		})
	}
}

func TestPopulateComputedFields_DependencyStatus(t *testing.T) {
	// Test that PopulateComputedFields correctly sets DependencyStatus
	tasks := []*Task{
		{ID: "TASK-001", Title: "Completed blocker", Status: StatusCompleted},
		{ID: "TASK-002", Title: "Running blocker", Status: StatusRunning},
		{ID: "TASK-003", Title: "No deps", Status: StatusPlanned},
		{ID: "TASK-004", Title: "All deps satisfied", BlockedBy: []string{"TASK-001"}, Status: StatusPlanned},
		{ID: "TASK-005", Title: "Has unmet deps", BlockedBy: []string{"TASK-001", "TASK-002"}, Status: StatusPlanned},
	}

	PopulateComputedFields(tasks)

	// TASK-003: no dependencies -> "none"
	if tasks[2].DependencyStatus != DependencyStatusNone {
		t.Errorf("TASK-003 DependencyStatus = %s, want %s", tasks[2].DependencyStatus, DependencyStatusNone)
	}

	// TASK-004: all deps satisfied (TASK-001 is completed) -> "ready"
	if tasks[3].DependencyStatus != DependencyStatusReady {
		t.Errorf("TASK-004 DependencyStatus = %s, want %s", tasks[3].DependencyStatus, DependencyStatusReady)
	}

	// TASK-005: has unmet deps (TASK-002 is running) -> "blocked"
	if tasks[4].DependencyStatus != DependencyStatusBlocked {
		t.Errorf("TASK-005 DependencyStatus = %s, want %s", tasks[4].DependencyStatus, DependencyStatusBlocked)
	}
}

// ============================================================================
// Quality Metrics Tests
// ============================================================================

func TestRecordPhaseRetry(t *testing.T) {
	task := &Task{ID: "TASK-001", Title: "Test Task", Weight: WeightMedium}

	// Initially no quality metrics
	if task.Quality != nil {
		t.Error("expected Quality to be nil initially")
	}

	// Record first retry
	task.RecordPhaseRetry("implement")
	if task.Quality == nil {
		t.Fatal("expected Quality to be initialized after recording retry")
	}
	if task.Quality.PhaseRetries["implement"] != 1 {
		t.Errorf("expected implement retries=1, got %d", task.Quality.PhaseRetries["implement"])
	}
	if task.Quality.TotalRetries != 1 {
		t.Errorf("expected TotalRetries=1, got %d", task.Quality.TotalRetries)
	}

	// Record more retries
	task.RecordPhaseRetry("implement")
	task.RecordPhaseRetry("review")
	if task.Quality.PhaseRetries["implement"] != 2 {
		t.Errorf("expected implement retries=2, got %d", task.Quality.PhaseRetries["implement"])
	}
	if task.Quality.PhaseRetries["review"] != 1 {
		t.Errorf("expected review retries=1, got %d", task.Quality.PhaseRetries["review"])
	}
	if task.Quality.TotalRetries != 3 {
		t.Errorf("expected TotalRetries=3, got %d", task.Quality.TotalRetries)
	}
}

func TestRecordReviewRejection(t *testing.T) {
	task := &Task{ID: "TASK-001", Title: "Test Task", Weight: WeightMedium}

	task.RecordReviewRejection()
	if task.Quality == nil {
		t.Fatal("expected Quality to be initialized")
	}
	if task.Quality.ReviewRejections != 1 {
		t.Errorf("expected ReviewRejections=1, got %d", task.Quality.ReviewRejections)
	}

	task.RecordReviewRejection()
	task.RecordReviewRejection()
	if task.Quality.ReviewRejections != 3 {
		t.Errorf("expected ReviewRejections=3, got %d", task.Quality.ReviewRejections)
	}
}

func TestRecordManualIntervention(t *testing.T) {
	task := &Task{ID: "TASK-001", Title: "Test Task", Weight: WeightMedium}

	task.RecordManualIntervention("Fixed via orc resolve")
	if task.Quality == nil {
		t.Fatal("expected Quality to be initialized")
	}
	if !task.Quality.ManualIntervention {
		t.Error("expected ManualIntervention=true")
	}
	if task.Quality.ManualInterventionReason != "Fixed via orc resolve" {
		t.Errorf("expected reason to match, got %q", task.Quality.ManualInterventionReason)
	}

	// Recording again should update the reason
	task.RecordManualIntervention("Updated reason")
	if task.Quality.ManualInterventionReason != "Updated reason" {
		t.Errorf("expected updated reason, got %q", task.Quality.ManualInterventionReason)
	}
}

func TestGetPhaseRetries(t *testing.T) {
	task := &Task{ID: "TASK-001", Title: "Test Task", Weight: WeightMedium}

	// No quality metrics yet
	if task.GetPhaseRetries("implement") != 0 {
		t.Error("expected 0 retries for nil Quality")
	}

	task.RecordPhaseRetry("implement")
	task.RecordPhaseRetry("implement")
	if task.GetPhaseRetries("implement") != 2 {
		t.Errorf("expected 2 retries, got %d", task.GetPhaseRetries("implement"))
	}
	if task.GetPhaseRetries("review") != 0 {
		t.Errorf("expected 0 retries for unrecorded phase, got %d", task.GetPhaseRetries("review"))
	}
}

func TestGetTotalRetries(t *testing.T) {
	task := &Task{ID: "TASK-001", Title: "Test Task", Weight: WeightMedium}

	if task.GetTotalRetries() != 0 {
		t.Error("expected 0 total retries for nil Quality")
	}

	task.RecordPhaseRetry("implement")
	task.RecordPhaseRetry("review")
	if task.GetTotalRetries() != 2 {
		t.Errorf("expected 2 total retries, got %d", task.GetTotalRetries())
	}
}

func TestGetReviewRejections(t *testing.T) {
	task := &Task{ID: "TASK-001", Title: "Test Task", Weight: WeightMedium}

	if task.GetReviewRejections() != 0 {
		t.Error("expected 0 rejections for nil Quality")
	}

	task.RecordReviewRejection()
	if task.GetReviewRejections() != 1 {
		t.Errorf("expected 1 rejection, got %d", task.GetReviewRejections())
	}
}

func TestHadManualIntervention(t *testing.T) {
	task := &Task{ID: "TASK-001", Title: "Test Task", Weight: WeightMedium}

	if task.HadManualIntervention() {
		t.Error("expected false for nil Quality")
	}

	task.RecordManualIntervention("test")
	if !task.HadManualIntervention() {
		t.Error("expected true after recording intervention")
	}
}

func TestEnsureQualityMetrics(t *testing.T) {
	task := &Task{ID: "TASK-001", Title: "Test Task", Weight: WeightMedium}

	if task.Quality != nil {
		t.Error("expected Quality to be nil initially")
	}

	task.EnsureQualityMetrics()
	if task.Quality == nil {
		t.Fatal("expected Quality to be initialized")
	}
	if task.Quality.PhaseRetries == nil {
		t.Error("expected PhaseRetries map to be initialized")
	}

	// Calling again should not reset
	task.Quality.TotalRetries = 5
	task.EnsureQualityMetrics()
	if task.Quality.TotalRetries != 5 {
		t.Error("EnsureQualityMetrics should not reset existing values")
	}
}
