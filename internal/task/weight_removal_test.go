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
// Tests WILL FAIL until implementation removes weight from the proto and Go code.
package task

import (
	"reflect"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// --- SC-1: TaskWeight enum should NOT exist ---

// TestTaskWeight_EnumRemoved verifies SC-1:
// The TaskWeight enum should be completely removed from the proto.
// This test will fail to compile once the enum is removed (which is expected).
// After removal, this test file should be updated to verify the enum doesn't exist.
func TestTaskWeight_EnumRemoved(t *testing.T) {
	// This test verifies that TaskWeight enum values no longer exist.
	// Currently, these compile - after weight removal, they should cause compile errors.
	//
	// Once weight is removed, change this test to use reflection to verify
	// the type doesn't exist.

	// For now, test that we want these to NOT exist after implementation:
	// The presence of these values means the enum still exists (implementation needed)

	_ = orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED
	_ = orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL
	_ = orcv1.TaskWeight_TASK_WEIGHT_SMALL
	_ = orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	_ = orcv1.TaskWeight_TASK_WEIGHT_LARGE

	// FAIL: If this test compiles and runs, TaskWeight enum still exists.
	// After implementation: This test should be removed or changed to verify
	// the enum doesn't exist via reflection.
	t.Error("SC-1 FAILED: TaskWeight enum still exists in proto - should be removed")
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
	}
}

// --- SC-9: Weight conversion functions should NOT exist ---

// TestWeightConversionFunctions_Removed verifies SC-9 (partial):
// Weight-related helper functions should be removed from the codebase.
//
// Functions to remove:
// - WeightToProto
// - WeightFromProto
// - ParseWeightProto
// - ValidWeightsProto
//
// This test verifies these functions still exist (and should fail after implementation).
func TestWeightConversionFunctions_Removed(t *testing.T) {
	// These function calls will fail to compile once removed.
	// For now, we verify they still exist and the test fails.

	// WeightToProto should not exist
	_ = WeightToProto("small")
	t.Error("SC-9 FAILED: WeightToProto function still exists - should be removed")
}

func TestWeightFromProto_Removed(t *testing.T) {
	// WeightFromProto should not exist
	_ = WeightFromProto(orcv1.TaskWeight_TASK_WEIGHT_SMALL)
	t.Error("SC-9 FAILED: WeightFromProto function still exists - should be removed")
}

func TestParseWeightProto_Removed(t *testing.T) {
	// ParseWeightProto should not exist
	_, _ = ParseWeightProto("small")
	t.Error("SC-9 FAILED: ParseWeightProto function still exists - should be removed")
}

func TestValidWeightsProto_Removed(t *testing.T) {
	// ValidWeightsProto should not exist
	_ = ValidWeightsProto()
	t.Error("SC-9 FAILED: ValidWeightsProto function still exists - should be removed")
}
