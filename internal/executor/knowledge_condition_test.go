// Tests for TASK-004: knowledge.available condition field resolution.
//
// Coverage mapping:
//   SC-10: TestKnowledgeConditionField_Available
//   SC-10: TestKnowledgeConditionField_Unavailable
//   SC-10: TestKnowledgeConditionField_MissingService
//
// These tests verify that the condition evaluator's resolveField() function
// supports the "knowledge" prefix, specifically "knowledge.available".
package executor

import (
	"testing"

	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// SC-10: knowledge.available resolves to "true" when service is available
// =============================================================================

func TestKnowledgeConditionField_Available(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-001", "Test knowledge condition")

	ctx := &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{},
		// KnowledgeAvailable is set by the executor when knowledge service
		// is injected. This field must be true for the condition to pass.
		KnowledgeAvailable: true,
	}

	result, err := EvaluateCondition(
		`{"field": "knowledge.available", "op": "eq", "value": "true"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("knowledge.available should be 'true' when service is available")
	}
}

// =============================================================================
// SC-10: knowledge.available resolves to "false" when service is unavailable
// =============================================================================

func TestKnowledgeConditionField_Unavailable(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-002", "Test knowledge unavailable")

	ctx := &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{},
		// KnowledgeAvailable not set (false by default)
		KnowledgeAvailable: false,
	}

	// knowledge.available == "true" should evaluate to false
	result, err := EvaluateCondition(
		`{"field": "knowledge.available", "op": "eq", "value": "true"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("knowledge.available should be 'false' when service is unavailable")
	}
}

// =============================================================================
// SC-10: Missing knowledge service reference → evaluates to "false" (safe default)
// =============================================================================

func TestKnowledgeConditionField_MissingService(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-003", "Test missing knowledge service")

	// ConditionContext without KnowledgeAvailable set at all
	// (zero value of bool is false → safe default)
	ctx := &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{},
	}

	result, err := EvaluateCondition(
		`{"field": "knowledge.available", "op": "eq", "value": "true"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("knowledge.available should be 'false' when no service reference exists")
	}
}

// =============================================================================
// Edge case: knowledge.available with neq operator
// =============================================================================

func TestKnowledgeConditionField_NeqOperator(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-004", "Test knowledge neq")

	ctx := &ConditionContext{
		Task:               tsk,
		Vars:               variable.VariableSet{},
		RCtx:               &variable.ResolutionContext{},
		KnowledgeAvailable: true,
	}

	// knowledge.available != "false" → true (it's "true", not "false")
	result, err := EvaluateCondition(
		`{"field": "knowledge.available", "op": "neq", "value": "false"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("knowledge.available neq 'false' should be true when available")
	}
}

// =============================================================================
// Edge case: knowledge.unknown_field → empty string (safe default)
// =============================================================================

func TestKnowledgeConditionField_UnknownSubfield(t *testing.T) {
	t.Parallel()

	tsk := task.NewProtoTask("TASK-005", "Test unknown knowledge subfield")

	ctx := &ConditionContext{
		Task:               tsk,
		Vars:               variable.VariableSet{},
		RCtx:               &variable.ResolutionContext{},
		KnowledgeAvailable: true,
	}

	// knowledge.nonexistent → empty → exists returns false
	result, err := EvaluateCondition(
		`{"field": "knowledge.nonexistent", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("unknown knowledge subfield should resolve to empty (exists=false)")
	}
}

// =============================================================================
// BDD-4: Condition in workflow evaluates false when knowledge disabled →
// phase skipped by condition system (before executor invoked)
// =============================================================================

func TestKnowledgeConditionField_WorkflowSkipPattern(t *testing.T) {
	t.Parallel()

	// Simulate the condition that would be on a gather-context phase:
	// {"field": "knowledge.available", "op": "eq", "value": "true"}
	// When knowledge is disabled, this evaluates to false → phase skips.

	tsk := task.NewProtoTask("TASK-006", "Test workflow skip pattern")

	ctx := &ConditionContext{
		Task:               tsk,
		Vars:               variable.VariableSet{},
		RCtx:               &variable.ResolutionContext{},
		KnowledgeAvailable: false, // Knowledge disabled
	}

	// This is the exact condition from the workflow YAML
	conditionJSON := `{"field": "knowledge.available", "op": "eq", "value": "true"}`

	result, err := EvaluateCondition(conditionJSON, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("condition should evaluate to false when knowledge is disabled — phase should be skipped")
	}
}
