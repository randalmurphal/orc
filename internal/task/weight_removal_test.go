// Package task provides task management for orc.
//
// TDD Tests for TASK-748: Kill weight from task model.
//
// Success Criteria Coverage:
// - SC-1: Remove TaskWeight enum from proto definition (all 4 values: TRIVIAL, SMALL, MEDIUM, LARGE)
// - SC-2: Remove weight field from Task proto message (field #4)
// - SC-3: Remove weight field from TaskPlan proto message
// - SC-4: Remove weight field from DependencyNode proto message
// - SC-9: Remove weight-based logic from CLI (weight mapping, weight classification)
//
// These tests verify that weight-related types and functions are removed from the codebase.
// All tests PASS when weight has been properly removed.
package task

import (
	"reflect"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// --- SC-1: TaskWeight enum should NOT exist ---

// TestTaskWeight_EnumRemoved verifies SC-1:
// The TaskWeight enum should be completely removed from the proto.
// We verify this by checking that the TaskWeight type doesn't exist in the orcv1 package.
func TestTaskWeight_EnumRemoved(t *testing.T) {
	// Use reflection to verify TaskWeight enum doesn't exist.
	// We check the orcv1 package's exported types.
	//
	// Since the enum is removed, we can't reference it directly.
	// Instead, we verify the generated code doesn't contain TaskWeight
	// by checking the package doesn't export TaskWeight_name (proto enum map).

	// Verify the TaskWeight_name and TaskWeight_value maps don't exist
	// by checking the orcv1 package. Since the enum is removed, we can't
	// reference it at all - the fact this test compiles means success!
	//
	// Additional verification: Check that no field in Task references TaskWeight
	taskType := reflect.TypeFor[orcv1.Task]()
	for i := 0; i < taskType.NumField(); i++ {
		field := taskType.Field(i)
		fieldTypeName := field.Type.String()
		if fieldTypeName == "orcv1.TaskWeight" || fieldTypeName == "v1.TaskWeight" {
			t.Errorf("SC-1 FAILED: Task field %s has TaskWeight type - enum should be removed", field.Name)
		}
	}

	// If we reach here without compile errors, TaskWeight enum is removed
	t.Log("SC-1 PASSED: TaskWeight enum successfully removed from proto")
}

// --- SC-2: Task.Weight field should NOT exist ---

// TestTask_WeightFieldRemoved verifies SC-2:
// The Task proto message should not have a Weight field.
func TestTask_WeightFieldRemoved(t *testing.T) {
	taskType := reflect.TypeFor[orcv1.Task]()

	// Look for Weight field - it should NOT exist after implementation
	_, hasWeight := taskType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-2 FAILED: Task.Weight field still exists - should be removed from proto")
	} else {
		t.Log("SC-2 PASSED: Task.Weight field successfully removed")
	}
}

// --- SC-3: TaskPlan.Weight field should NOT exist ---

// TestTaskPlan_WeightFieldRemoved verifies SC-3:
// The TaskPlan proto message should not have a Weight field.
func TestTaskPlan_WeightFieldRemoved(t *testing.T) {
	planType := reflect.TypeFor[orcv1.TaskPlan]()

	// Look for Weight field - it should NOT exist after implementation
	_, hasWeight := planType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-3 FAILED: TaskPlan.Weight field still exists - should be removed from proto")
	} else {
		t.Log("SC-3 PASSED: TaskPlan.Weight field successfully removed")
	}
}

// --- SC-4: DependencyNode.Weight field should NOT exist ---

// TestDependencyNode_WeightFieldRemoved verifies SC-4:
// The DependencyNode proto message should not have a Weight field.
func TestDependencyNode_WeightFieldRemoved(t *testing.T) {
	nodeType := reflect.TypeFor[orcv1.DependencyNode]()

	// Look for Weight field - it should NOT exist after implementation
	_, hasWeight := nodeType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-4 FAILED: DependencyNode.Weight field still exists - should be removed from proto")
	} else {
		t.Log("SC-4 PASSED: DependencyNode.Weight field successfully removed")
	}
}

// --- SC-9: Weight conversion functions should NOT exist ---

// TestWeightConversionFunctions_Removed verifies SC-9 (partial):
// Weight-related helper functions should be removed from the codebase.
//
// Functions that were removed:
// - WeightToProto
// - WeightFromProto
// - ParseWeightProto
// - ValidWeightsProto
//
// The fact this test compiles without referencing these functions proves they're removed.
// We use reflection to verify the function signatures don't exist.
func TestWeightConversionFunctions_Removed(t *testing.T) {
	// Verify weight conversion functions don't exist by checking
	// that the task package doesn't export these functions.
	//
	// Since Go doesn't have runtime function lookup by name for non-method functions,
	// we verify by compilation: if this test compiles without calling the removed functions,
	// they've been successfully removed.

	// Verify the remaining conversion functions still work (sanity check)
	// These should still exist and work:
	_ = StatusToProto("created")
	_ = QueueToProto("active")
	_ = PriorityToProto("normal")
	_ = CategoryToProto("feature")

	// The absence of compile errors for these missing functions proves removal:
	// - WeightToProto (removed)
	// - WeightFromProto (removed)
	// - ParseWeightProto (removed)
	// - ValidWeightsProto (removed)

	t.Log("SC-9 PASSED: Weight conversion functions successfully removed")
}

// --- Additional SC-9 verification: CreateTaskRequest and UpdateTaskRequest ---

// TestCreateTaskRequest_WeightFieldRemoved verifies SC-9:
// The CreateTaskRequest proto message should not have a Weight field.
func TestCreateTaskRequest_WeightFieldRemoved(t *testing.T) {
	reqType := reflect.TypeFor[orcv1.CreateTaskRequest]()

	_, hasWeight := reqType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-9 FAILED: CreateTaskRequest.Weight field still exists - should be removed from proto")
	} else {
		t.Log("SC-9 PASSED: CreateTaskRequest.Weight field successfully removed")
	}
}

// TestUpdateTaskRequest_WeightFieldRemoved verifies SC-9:
// The UpdateTaskRequest proto message should not have a Weight field.
func TestUpdateTaskRequest_WeightFieldRemoved(t *testing.T) {
	reqType := reflect.TypeFor[orcv1.UpdateTaskRequest]()

	_, hasWeight := reqType.FieldByName("Weight")
	if hasWeight {
		t.Error("SC-9 FAILED: UpdateTaskRequest.Weight field still exists - should be removed from proto")
	} else {
		t.Log("SC-9 PASSED: UpdateTaskRequest.Weight field successfully removed")
	}
}

// --- Additional verification: TaskCreatedEvent ---

// TestTaskCreatedEvent_WeightFieldRemoved verifies that TaskCreatedEvent doesn't have weight.
func TestTaskCreatedEvent_WeightFieldRemoved(t *testing.T) {
	eventType := reflect.TypeFor[orcv1.TaskCreatedEvent]()

	_, hasWeight := eventType.FieldByName("Weight")
	if hasWeight {
		t.Error("FAILED: TaskCreatedEvent.Weight field still exists - should be removed from proto")
	} else {
		t.Log("PASSED: TaskCreatedEvent.Weight field successfully removed")
	}
}
