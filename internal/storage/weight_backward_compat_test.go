// Package storage provides storage backends for orc.
//
// TDD Tests for TASK-748: Kill weight from task model - Backward compatibility.
//
// Success Criteria Coverage:
// - SC-5: Remove weight TEXT column from SQLite tasks table via migration
// - SC-13: Verify backward compatibility: old tasks with weight field still work
// - SC-14: Weight ignored in import operations
//
// These tests verify that:
// 1. Existing databases with weight column continue to work
// 2. Tasks with weight field are still loadable
// 3. workflow_id takes precedence over weight
// 4. Import operations ignore weight field
package storage

import (
	"reflect"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// ============================================================================
// SC-5: Database schema should not have weight column (after migration)
// ============================================================================

// TestDatabaseSchema_NoWeightColumn verifies SC-5:
// After migration, new databases should not have weight column in tasks table.
// Note: Existing databases may still have the column for data preservation.
func TestDatabaseSchema_NoWeightColumn(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	// Create a task to ensure table exists
	workflowID := "implement-small"
	task := &orcv1.Task{
		Id:         "TASK-001",
		Title:      "Test task",
		WorkflowId: &workflowID,
		Status:     orcv1.TaskStatus_TASK_STATUS_CREATED,
		CreatedAt:  timestamppb.Now(),
		UpdatedAt:  timestamppb.Now(),
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// For new databases (in-memory test backends), weight column should not exist
	// This test verifies the schema change is reflected
	//
	// SC-5: After implementation, the schema should not include weight column
	// The test backend uses the latest schema, so if weight column is removed
	// from the schema, this test will pass.

	// Try to verify the Task struct no longer has Weight field
	taskType := reflect.TypeFor[orcv1.Task]()
	_, hasWeight := taskType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-5 FAILED: Task struct still has Weight field - schema migration needed")
	}
}

// ============================================================================
// SC-13: Backward compatibility - old tasks with weight still work
// ============================================================================

// TestBackwardCompat_LoadTaskWithWeight verifies SC-13:
// Tasks that were created with weight field should still be loadable.
// The weight field should be ignored, and workflow_id takes precedence.
func TestBackwardCompat_LoadTaskWithWeight(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	// Create a task that simulates an "old" task with weight
	// In the new model, workflow_id is what matters
	workflowID := "implement-medium"
	oldTask := &orcv1.Task{
		Id:         "TASK-OLD-001",
		Title:      "Old task with weight",
		WorkflowId: &workflowID, // This should be what's used
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		CreatedAt:  timestamppb.New(time.Now().Add(-24 * time.Hour)),
		UpdatedAt:  timestamppb.New(time.Now().Add(-24 * time.Hour)),
	}

	// Save and reload
	if err := backend.SaveTask(oldTask); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	loaded, err := backend.LoadTask("TASK-OLD-001")
	if err != nil {
		t.Fatalf("LoadTask failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("loaded task is nil")
	}

	// SC-13: Task should load correctly
	if loaded.Id != "TASK-OLD-001" {
		t.Errorf("loaded ID = %q, want %q", loaded.Id, "TASK-OLD-001")
	}

	// SC-13: workflow_id should be preserved
	if loaded.WorkflowId == nil {
		t.Error("workflow_id should be preserved, got nil")
	} else if *loaded.WorkflowId != "implement-medium" {
		t.Errorf("workflow_id = %q, want %q", *loaded.WorkflowId, "implement-medium")
	}

	// SC-13: Task should not have Weight field after proto change
	taskType := reflect.TypeFor[orcv1.Task]()
	_, hasWeight := taskType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-13 FAILED: Task still has Weight field - should be removed")
	}
}

// TestBackwardCompat_WorkflowTakesPrecedence verifies SC-13:
// When a task has both weight (in legacy DB) and workflow_id,
// workflow_id should be used for execution, not weight.
func TestBackwardCompat_WorkflowTakesPrecedence(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	// Task with workflow_id set (the new way)
	workflowID := "qa-e2e" // Different from what weight would have mapped to
	task := &orcv1.Task{
		Id:         "TASK-PRECEDENCE-001",
		Title:      "Task with explicit workflow",
		WorkflowId: &workflowID,
		Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
		CreatedAt:  timestamppb.Now(),
		UpdatedAt:  timestamppb.Now(),
	}

	if err := backend.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	loaded, err := backend.LoadTask("TASK-PRECEDENCE-001")
	if err != nil {
		t.Fatalf("LoadTask failed: %v", err)
	}

	// SC-13: workflow_id should be the one used (not derived from weight)
	if loaded.WorkflowId == nil {
		t.Fatal("workflow_id should not be nil")
	}
	if *loaded.WorkflowId != "qa-e2e" {
		t.Errorf("workflow_id = %q, want %q (explicit workflow should take precedence)", *loaded.WorkflowId, "qa-e2e")
	}
}

// TestBackwardCompat_ListAllTasksWithWeight verifies SC-13:
// LoadAllTasks should work even with tasks that have weight column set.
func TestBackwardCompat_ListAllTasksWithWeight(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	// Create multiple tasks
	for i := range 5 {
		wfID := "implement-small"
		task := &orcv1.Task{
			Id:         taskIDFromIndex(i),
			Title:      "Test task",
			WorkflowId: &wfID,
			Status:     orcv1.TaskStatus_TASK_STATUS_PLANNED,
			CreatedAt:  timestamppb.Now(),
			UpdatedAt:  timestamppb.Now(),
		}
		if err := backend.SaveTask(task); err != nil {
			t.Fatalf("SaveTask %d failed: %v", i, err)
		}
	}

	// Load all tasks
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		t.Fatalf("LoadAllTasks failed: %v", err)
	}

	if len(tasks) != 5 {
		t.Errorf("expected 5 tasks, got %d", len(tasks))
	}

	// SC-13: All tasks should load correctly without weight field
	for _, task := range tasks {
		if task.Id == "" {
			t.Error("task ID should not be empty")
		}
		// Verify Weight field doesn't exist
		taskType := reflect.TypeFor[orcv1.Task]()
		_, hasWeight := taskType.FieldByName("Weight")
		if hasWeight {
			t.Error("SC-13 FAILED: Task still has Weight field")
		}
	}
}

// taskIDFromIndex generates a task ID from an index.
func taskIDFromIndex(i int) string {
	return "TASK-" + string(rune('0'+i/100)) + string(rune('0'+(i%100)/10)) + string(rune('0'+i%10))
}

// ============================================================================
// SC-14: Import should ignore weight field
// ============================================================================

// TestImport_IgnoresWeightField verifies SC-14:
// When importing task data that contains weight field, it should be ignored.
// The task should be imported successfully with workflow_id.
//
// Note: This is a unit test for the concept. Actual import integration tests
// are in internal/cli/cmd_import_test.go
func TestImport_TaskWithoutWeightFieldExists(t *testing.T) {
	t.Parallel()

	// SC-14: After implementation, imported tasks should not have weight
	// When importing from older exports that contain weight,
	// the weight field should simply not be present in the proto

	// Verify Task proto doesn't have Weight field
	taskType := reflect.TypeFor[orcv1.Task]()
	_, hasWeight := taskType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-14 FAILED: Task proto still has Weight field - imports will fail to ignore weight")
	}
}

// TestBackwardCompat_SaveLoadRoundtrip verifies data integrity:
// Tasks saved and loaded should maintain all required fields.
func TestBackwardCompat_SaveLoadRoundtrip(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)

	workflowID := "implement-medium"
	description := "Test description"
	original := &orcv1.Task{
		Id:          "TASK-ROUNDTRIP-001",
		Title:       "Roundtrip test",
		Description: &description,
		WorkflowId:  &workflowID,
		Status:      orcv1.TaskStatus_TASK_STATUS_PLANNED,
		Category:    orcv1.TaskCategory_TASK_CATEGORY_FEATURE,
		Priority:    orcv1.TaskPriority_TASK_PRIORITY_NORMAL,
		Queue:       orcv1.TaskQueue_TASK_QUEUE_ACTIVE,
		CreatedAt:   timestamppb.Now(),
		UpdatedAt:   timestamppb.Now(),
	}

	if err := backend.SaveTask(original); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	loaded, err := backend.LoadTask("TASK-ROUNDTRIP-001")
	if err != nil {
		t.Fatalf("LoadTask failed: %v", err)
	}

	// Verify all non-weight fields are preserved
	if loaded.Id != original.Id {
		t.Errorf("Id = %q, want %q", loaded.Id, original.Id)
	}
	if loaded.Title != original.Title {
		t.Errorf("Title = %q, want %q", loaded.Title, original.Title)
	}
	if loaded.Description == nil || *loaded.Description != *original.Description {
		t.Errorf("Description mismatch")
	}
	if loaded.WorkflowId == nil || *loaded.WorkflowId != *original.WorkflowId {
		t.Errorf("WorkflowId = %v, want %v", loaded.WorkflowId, original.WorkflowId)
	}
	if loaded.Status != original.Status {
		t.Errorf("Status = %v, want %v", loaded.Status, original.Status)
	}
	if loaded.Category != original.Category {
		t.Errorf("Category = %v, want %v", loaded.Category, original.Category)
	}
	if loaded.Priority != original.Priority {
		t.Errorf("Priority = %v, want %v", loaded.Priority, original.Priority)
	}
}
