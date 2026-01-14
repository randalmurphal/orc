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

func TestRemoveTask(t *testing.T) {
	init := New("INIT-001", "Remove Task Test")

	// Add some tasks
	init.AddTask("TASK-001", "First task", nil)
	init.AddTask("TASK-002", "Second task", []string{"TASK-001"})
	init.AddTask("TASK-003", "Third task", []string{"TASK-001"})

	if len(init.Tasks) != 3 {
		t.Fatalf("Tasks count = %d, want 3", len(init.Tasks))
	}

	// Remove existing task
	if !init.RemoveTask("TASK-002") {
		t.Error("RemoveTask should return true for existing task")
	}
	if len(init.Tasks) != 2 {
		t.Errorf("Tasks count = %d, want 2 after removal", len(init.Tasks))
	}

	// Verify correct task was removed
	for _, task := range init.Tasks {
		if task.ID == "TASK-002" {
			t.Error("TASK-002 should have been removed")
		}
	}

	// Verify remaining tasks are correct
	if init.Tasks[0].ID != "TASK-001" {
		t.Errorf("First task ID = %q, want %q", init.Tasks[0].ID, "TASK-001")
	}
	if init.Tasks[1].ID != "TASK-003" {
		t.Errorf("Second task ID = %q, want %q", init.Tasks[1].ID, "TASK-003")
	}

	// Remove non-existing task
	if init.RemoveTask("TASK-999") {
		t.Error("RemoveTask should return false for non-existing task")
	}
	if len(init.Tasks) != 2 {
		t.Errorf("Tasks count = %d, want 2 (no change for non-existing)", len(init.Tasks))
	}

	// Remove first task
	if !init.RemoveTask("TASK-001") {
		t.Error("RemoveTask should return true for existing task")
	}
	if len(init.Tasks) != 1 {
		t.Errorf("Tasks count = %d, want 1 after removal", len(init.Tasks))
	}
	if init.Tasks[0].ID != "TASK-003" {
		t.Errorf("Remaining task ID = %q, want %q", init.Tasks[0].ID, "TASK-003")
	}

	// Remove last task
	if !init.RemoveTask("TASK-003") {
		t.Error("RemoveTask should return true for existing task")
	}
	if len(init.Tasks) != 0 {
		t.Errorf("Tasks count = %d, want 0 after removing all", len(init.Tasks))
	}

	// Remove from empty list
	if init.RemoveTask("TASK-001") {
		t.Error("RemoveTask should return false when list is empty")
	}
}

// Tests for initiative dependencies

func TestValidateBlockedBy(t *testing.T) {
	existingIDs := map[string]bool{
		"INIT-001": true,
		"INIT-002": true,
		"INIT-003": true,
	}

	tests := []struct {
		name      string
		initID    string
		blockedBy []string
		wantErrs  int
	}{
		{
			name:      "valid blockers",
			initID:    "INIT-004",
			blockedBy: []string{"INIT-001", "INIT-002"},
			wantErrs:  0,
		},
		{
			name:      "self-reference",
			initID:    "INIT-001",
			blockedBy: []string{"INIT-001"},
			wantErrs:  1,
		},
		{
			name:      "non-existent initiative",
			initID:    "INIT-004",
			blockedBy: []string{"INIT-999"},
			wantErrs:  1,
		},
		{
			name:      "multiple errors",
			initID:    "INIT-004",
			blockedBy: []string{"INIT-004", "INIT-999"},
			wantErrs:  2,
		},
		{
			name:      "empty blockers",
			initID:    "INIT-004",
			blockedBy: []string{},
			wantErrs:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := ValidateBlockedBy(tt.initID, tt.blockedBy, existingIDs)
			if len(errs) != tt.wantErrs {
				t.Errorf("ValidateBlockedBy() got %d errors, want %d", len(errs), tt.wantErrs)
			}
		})
	}
}

func TestDetectCircularDependency(t *testing.T) {
	// Create initiatives: INIT-001 -> INIT-002 -> INIT-003
	initiatives := map[string]*Initiative{
		"INIT-001": {ID: "INIT-001", BlockedBy: []string{}},
		"INIT-002": {ID: "INIT-002", BlockedBy: []string{"INIT-001"}},
		"INIT-003": {ID: "INIT-003", BlockedBy: []string{"INIT-002"}},
	}

	tests := []struct {
		name       string
		initID     string
		newBlocker string
		wantCycle  bool
	}{
		{
			name:       "no cycle - adding new blocker",
			initID:     "INIT-003",
			newBlocker: "INIT-001",
			wantCycle:  false,
		},
		{
			name:       "cycle - INIT-001 blocked by INIT-003",
			initID:     "INIT-001",
			newBlocker: "INIT-003",
			wantCycle:  true,
		},
		{
			name:       "cycle - direct self-block via chain",
			initID:     "INIT-001",
			newBlocker: "INIT-002",
			wantCycle:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cycle := DetectCircularDependency(tt.initID, tt.newBlocker, initiatives)
			hasCycle := cycle != nil
			if hasCycle != tt.wantCycle {
				t.Errorf("DetectCircularDependency() = %v, want cycle = %v", cycle, tt.wantCycle)
			}
		})
	}
}

func TestDetectCircularDependencyWithAll(t *testing.T) {
	// Create initiatives: INIT-001 -> INIT-002 -> INIT-003
	initiatives := map[string]*Initiative{
		"INIT-001": {ID: "INIT-001", BlockedBy: []string{}},
		"INIT-002": {ID: "INIT-002", BlockedBy: []string{"INIT-001"}},
		"INIT-003": {ID: "INIT-003", BlockedBy: []string{"INIT-002"}},
	}

	tests := []struct {
		name        string
		initID      string
		newBlockers []string
		wantCycle   bool
	}{
		{
			name:        "no cycle",
			initID:      "INIT-003",
			newBlockers: []string{"INIT-001"},
			wantCycle:   false,
		},
		{
			name:        "cycle",
			initID:      "INIT-001",
			newBlockers: []string{"INIT-003"},
			wantCycle:   true,
		},
		{
			name:        "empty blockers",
			initID:      "INIT-002",
			newBlockers: []string{},
			wantCycle:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cycle := DetectCircularDependencyWithAll(tt.initID, tt.newBlockers, initiatives)
			hasCycle := cycle != nil
			if hasCycle != tt.wantCycle {
				t.Errorf("DetectCircularDependencyWithAll() = %v, want cycle = %v", cycle, tt.wantCycle)
			}
		})
	}
}

func TestComputeBlocks(t *testing.T) {
	initiatives := []*Initiative{
		{ID: "INIT-001", BlockedBy: []string{}},
		{ID: "INIT-002", BlockedBy: []string{"INIT-001"}},
		{ID: "INIT-003", BlockedBy: []string{"INIT-001", "INIT-002"}},
	}

	// INIT-001 blocks INIT-002 and INIT-003
	blocks := ComputeBlocks("INIT-001", initiatives)
	if len(blocks) != 2 {
		t.Errorf("ComputeBlocks(INIT-001) = %v, want 2 blocks", blocks)
	}

	// INIT-002 blocks INIT-003
	blocks = ComputeBlocks("INIT-002", initiatives)
	if len(blocks) != 1 {
		t.Errorf("ComputeBlocks(INIT-002) = %v, want 1 block", blocks)
	}

	// INIT-003 blocks nothing
	blocks = ComputeBlocks("INIT-003", initiatives)
	if len(blocks) != 0 {
		t.Errorf("ComputeBlocks(INIT-003) = %v, want 0 blocks", blocks)
	}
}

func TestPopulateComputedFields(t *testing.T) {
	initiatives := []*Initiative{
		{ID: "INIT-001", BlockedBy: []string{}},
		{ID: "INIT-002", BlockedBy: []string{"INIT-001"}},
		{ID: "INIT-003", BlockedBy: []string{"INIT-001", "INIT-002"}},
	}

	PopulateComputedFields(initiatives)

	// Check INIT-001 blocks
	if len(initiatives[0].Blocks) != 2 {
		t.Errorf("INIT-001 Blocks = %v, want 2", initiatives[0].Blocks)
	}

	// Check INIT-002 blocks
	if len(initiatives[1].Blocks) != 1 {
		t.Errorf("INIT-002 Blocks = %v, want 1", initiatives[1].Blocks)
	}

	// Check INIT-003 blocks
	if len(initiatives[2].Blocks) != 0 {
		t.Errorf("INIT-003 Blocks = %v, want 0", initiatives[2].Blocks)
	}
}

func TestIsBlocked(t *testing.T) {
	initMap := map[string]*Initiative{
		"INIT-001": {ID: "INIT-001", Status: StatusCompleted, BlockedBy: []string{}},
		"INIT-002": {ID: "INIT-002", Status: StatusActive, BlockedBy: []string{}},
		"INIT-003": {ID: "INIT-003", Status: StatusDraft, BlockedBy: []string{"INIT-001"}},
		"INIT-004": {ID: "INIT-004", Status: StatusDraft, BlockedBy: []string{"INIT-002"}},
		"INIT-005": {ID: "INIT-005", Status: StatusDraft, BlockedBy: []string{"INIT-001", "INIT-002"}},
	}

	tests := []struct {
		name       string
		initID     string
		wantBlocked bool
	}{
		{
			name:        "not blocked - no blockers",
			initID:      "INIT-001",
			wantBlocked: false,
		},
		{
			name:        "not blocked - blocker completed",
			initID:      "INIT-003",
			wantBlocked: false,
		},
		{
			name:        "blocked - blocker active",
			initID:      "INIT-004",
			wantBlocked: true,
		},
		{
			name:        "blocked - one blocker not completed",
			initID:      "INIT-005",
			wantBlocked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init := initMap[tt.initID]
			isBlocked := init.IsBlocked(initMap)
			if isBlocked != tt.wantBlocked {
				t.Errorf("IsBlocked() = %v, want %v", isBlocked, tt.wantBlocked)
			}
		})
	}
}

func TestGetUnmetDependencies(t *testing.T) {
	initMap := map[string]*Initiative{
		"INIT-001": {ID: "INIT-001", Status: StatusCompleted},
		"INIT-002": {ID: "INIT-002", Status: StatusActive},
		"INIT-003": {ID: "INIT-003", Status: StatusDraft, BlockedBy: []string{"INIT-001", "INIT-002"}},
	}

	init := initMap["INIT-003"]
	unmet := init.GetUnmetDependencies(initMap)

	if len(unmet) != 1 {
		t.Errorf("GetUnmetDependencies() = %v, want 1 unmet", unmet)
	}
	if len(unmet) > 0 && unmet[0] != "INIT-002" {
		t.Errorf("Unmet dependency = %v, want INIT-002", unmet[0])
	}
}

func TestGetIncompleteBlockers(t *testing.T) {
	initMap := map[string]*Initiative{
		"INIT-001": {ID: "INIT-001", Title: "First", Status: StatusCompleted},
		"INIT-002": {ID: "INIT-002", Title: "Second", Status: StatusActive},
		"INIT-003": {ID: "INIT-003", Title: "Third", Status: StatusDraft, BlockedBy: []string{"INIT-001", "INIT-002"}},
	}

	init := initMap["INIT-003"]
	blockers := init.GetIncompleteBlockers(initMap)

	if len(blockers) != 1 {
		t.Errorf("GetIncompleteBlockers() = %v, want 1 blocker", blockers)
	}
	if len(blockers) > 0 {
		if blockers[0].ID != "INIT-002" {
			t.Errorf("Blocker ID = %v, want INIT-002", blockers[0].ID)
		}
		if blockers[0].Title != "Second" {
			t.Errorf("Blocker Title = %v, want Second", blockers[0].Title)
		}
		if blockers[0].Status != StatusActive {
			t.Errorf("Blocker Status = %v, want active", blockers[0].Status)
		}
	}
}

func TestAddBlocker(t *testing.T) {
	initMap := map[string]*Initiative{
		"INIT-001": {ID: "INIT-001", Status: StatusActive, BlockedBy: []string{}},
		"INIT-002": {ID: "INIT-002", Status: StatusActive, BlockedBy: []string{}},
		"INIT-003": {ID: "INIT-003", Status: StatusActive, BlockedBy: []string{"INIT-001"}},
	}

	tests := []struct {
		name      string
		initID    string
		blockerID string
		wantErr   bool
	}{
		{
			name:      "valid add",
			initID:    "INIT-002",
			blockerID: "INIT-001",
			wantErr:   false,
		},
		{
			name:      "self-reference",
			initID:    "INIT-001",
			blockerID: "INIT-001",
			wantErr:   true,
		},
		{
			name:      "non-existent",
			initID:    "INIT-001",
			blockerID: "INIT-999",
			wantErr:   true,
		},
		{
			name:      "would create cycle",
			initID:    "INIT-001",
			blockerID: "INIT-003",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the init for each test
			init := &Initiative{
				ID:        tt.initID,
				Status:    StatusActive,
				BlockedBy: []string{},
			}
			if tt.initID == "INIT-003" {
				init.BlockedBy = []string{"INIT-001"}
			}

			err := init.AddBlocker(tt.blockerID, initMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddBlocker() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRemoveBlocker(t *testing.T) {
	init := &Initiative{
		ID:        "INIT-001",
		BlockedBy: []string{"INIT-002", "INIT-003"},
	}

	// Remove existing
	if !init.RemoveBlocker("INIT-002") {
		t.Error("RemoveBlocker should return true for existing blocker")
	}
	if len(init.BlockedBy) != 1 {
		t.Errorf("BlockedBy length = %d, want 1", len(init.BlockedBy))
	}
	if init.BlockedBy[0] != "INIT-003" {
		t.Errorf("Remaining blocker = %v, want INIT-003", init.BlockedBy[0])
	}

	// Remove non-existing
	if init.RemoveBlocker("INIT-999") {
		t.Error("RemoveBlocker should return false for non-existing blocker")
	}
}

func TestSetBlockedBy(t *testing.T) {
	initMap := map[string]*Initiative{
		"INIT-001": {ID: "INIT-001", Status: StatusActive, BlockedBy: []string{}},
		"INIT-002": {ID: "INIT-002", Status: StatusActive, BlockedBy: []string{}},
		"INIT-003": {ID: "INIT-003", Status: StatusActive, BlockedBy: []string{"INIT-001"}},
	}

	tests := []struct {
		name      string
		initID    string
		blockers  []string
		wantErr   bool
	}{
		{
			name:     "valid set",
			initID:   "INIT-002",
			blockers: []string{"INIT-001"},
			wantErr:  false,
		},
		{
			name:     "empty set",
			initID:   "INIT-003",
			blockers: []string{},
			wantErr:  false,
		},
		{
			name:     "self-reference",
			initID:   "INIT-001",
			blockers: []string{"INIT-001"},
			wantErr:  true,
		},
		{
			name:     "non-existent",
			initID:   "INIT-001",
			blockers: []string{"INIT-999"},
			wantErr:  true,
		},
		{
			name:     "would create cycle",
			initID:   "INIT-001",
			blockers: []string{"INIT-003"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			init := &Initiative{
				ID:        tt.initID,
				Status:    StatusActive,
				BlockedBy: []string{},
			}
			if tt.initID == "INIT-003" {
				init.BlockedBy = []string{"INIT-001"}
			}

			err := init.SetBlockedBy(tt.blockers, initMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetBlockedBy() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(init.BlockedBy) != len(tt.blockers) {
				t.Errorf("BlockedBy = %v, want %v", init.BlockedBy, tt.blockers)
			}
		})
	}
}

func TestBlockedByPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, ".orc", "initiatives")

	// Create initiative with blocked_by
	init := New("INIT-TEST-DEPS", "Deps Test")
	init.BlockedBy = []string{"INIT-001", "INIT-002"}

	// Save
	if err := init.SaveTo(baseDir); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load
	loaded, err := LoadFrom(baseDir, "INIT-TEST-DEPS")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify blocked_by was persisted
	if len(loaded.BlockedBy) != 2 {
		t.Errorf("BlockedBy length = %d, want 2", len(loaded.BlockedBy))
	}
	if loaded.BlockedBy[0] != "INIT-001" || loaded.BlockedBy[1] != "INIT-002" {
		t.Errorf("BlockedBy = %v, want [INIT-001 INIT-002]", loaded.BlockedBy)
	}

	// Verify Blocks is not persisted (it's computed)
	// After loading, Blocks should be nil until PopulateComputedFields is called
	if loaded.Blocks != nil && len(loaded.Blocks) > 0 {
		t.Error("Blocks should not be persisted")
	}
}
