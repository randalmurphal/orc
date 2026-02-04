// Tests for TASK-686: Phase condition evaluator in executor.
//
// These tests define the contract for EvaluateCondition and its integration
// into the executor's phase loop. Tests will NOT compile until the
// implementation is created in condition.go.
//
// Coverage mapping:
//   SC-1:  TestEvaluateCondition_Eq_*
//   SC-2:  TestEvaluateCondition_Operators
//   SC-3:  TestEvaluateCondition_TaskFields_*
//   SC-4:  TestEvaluateCondition_FieldResolution_*
//   SC-5:  TestWorkflowExecutor_ConditionSkip
//   SC-6:  TestEvaluateCondition_All_*
//   SC-7:  TestEvaluateCondition_Any_*
//   SC-9:  TestWorkflowExecutor_ConditionSkip (SkipReason)
//   SC-10: TestWorkflowExecutor_ResumeSkipped
package executor

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// Helper: build a ConditionContext with sensible defaults
// =============================================================================

func newTestConditionContext(t *testing.T) *ConditionContext {
	t.Helper()
	tsk := task.NewProtoTask("TASK-001", "Test task")
	tsk.Category = orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	tsk.Priority = orcv1.TaskPriority_TASK_PRIORITY_NORMAL

	return &ConditionContext{
		Task: tsk,
		Vars: variable.VariableSet{
			"SPEC_CONTENT": "some spec",
			"MY_VAR":       "hello",
		},
		RCtx: &variable.ResolutionContext{
			PriorOutputs: map[string]string{
				"spec": `{"status": "complete", "summary": "spec done"}`,
			},
			Environment: map[string]string{
				"HOME":      "/home/test",
				"SKIP_SPEC": "1",
			},
		},
	}
}

// =============================================================================
// SC-1: EvaluateCondition returns true for eq operator when field value matches
// =============================================================================

func TestEvaluateCondition_Eq_Match(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"field": "task.category", "op": "eq", "value": "feature"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("eq should return true when field value matches")
	}
}

func TestEvaluateCondition_Eq_NoMatch(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"field": "task.category", "op": "eq", "value": "bug"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("eq should return false when field value does not match")
	}
}

// =============================================================================
// SC-2: EvaluateCondition correctly evaluates all 7 operators
// =============================================================================

func TestEvaluateCondition_Operators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition string
		want      bool
	}{
		// eq
		{
			name:      "eq match",
			condition: `{"field": "task.category", "op": "eq", "value": "feature"}`,
			want:      true,
		},
		{
			name:      "eq no match",
			condition: `{"field": "task.category", "op": "eq", "value": "bug"}`,
			want:      false,
		},
		// neq
		{
			name:      "neq match (different values)",
			condition: `{"field": "task.category", "op": "neq", "value": "bug"}`,
			want:      true,
		},
		{
			name:      "neq no match (same values)",
			condition: `{"field": "task.category", "op": "neq", "value": "feature"}`,
			want:      false,
		},
		// in
		{
			name:      "in match",
			condition: `{"field": "task.category", "op": "in", "value": ["feature", "bug"]}`,
			want:      true,
		},
		{
			name:      "in no match",
			condition: `{"field": "task.category", "op": "in", "value": ["docs", "chore"]}`,
			want:      false,
		},
		// contains
		{
			name:      "contains match",
			condition: `{"field": "var.SPEC_CONTENT", "op": "contains", "value": "spec"}`,
			want:      true,
		},
		{
			name:      "contains no match",
			condition: `{"field": "var.SPEC_CONTENT", "op": "contains", "value": "missing"}`,
			want:      false,
		},
		// exists
		{
			name:      "exists on present field",
			condition: `{"field": "env.HOME", "op": "exists"}`,
			want:      true,
		},
		{
			name:      "exists on missing field",
			condition: `{"field": "env.NONEXISTENT_VAR_XYZ", "op": "exists"}`,
			want:      false,
		},
		// gt (numeric)
		{
			name:      "gt numeric true",
			condition: `{"field": "var.MY_NUM", "op": "gt", "value": "5"}`,
			want:      true,
		},
		{
			name:      "gt numeric false",
			condition: `{"field": "var.MY_NUM", "op": "gt", "value": "20"}`,
			want:      false,
		},
		// lt (numeric)
		{
			name:      "lt numeric true",
			condition: `{"field": "var.MY_NUM", "op": "lt", "value": "20"}`,
			want:      true,
		},
		{
			name:      "lt numeric false",
			condition: `{"field": "var.MY_NUM", "op": "lt", "value": "5"}`,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := newTestConditionContext(t)
			// Add a numeric var for gt/lt tests
			ctx.Vars["MY_NUM"] = "10"

			result, err := EvaluateCondition(tt.condition, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.want {
				t.Errorf("EvaluateCondition() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_UnknownOperator(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	_, err := EvaluateCondition(
		`{"field": "task.category", "op": "invalid_op", "value": "feature"}`,
		ctx,
	)
	if err == nil {
		t.Fatal("expected error for unknown operator, got nil")
	}
}

// =============================================================================
// SC-3: Field resolver resolves task fields to lowercase short forms
// =============================================================================

func TestEvaluateCondition_TaskFields_Category(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		category orcv1.TaskCategory
		value    string
		want     bool
	}{
		{"feature", orcv1.TaskCategory_TASK_CATEGORY_FEATURE, "feature", true},
		{"bug", orcv1.TaskCategory_TASK_CATEGORY_BUG, "bug", true},
		{"refactor", orcv1.TaskCategory_TASK_CATEGORY_REFACTOR, "refactor", true},
		{"docs", orcv1.TaskCategory_TASK_CATEGORY_DOCS, "docs", true},
		{"mismatch", orcv1.TaskCategory_TASK_CATEGORY_FEATURE, "bug", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := newTestConditionContext(t)
			ctx.Task.Category = tt.category

			cond := `{"field": "task.category", "op": "eq", "value": "` + tt.value + `"}`
			result, err := EvaluateCondition(cond, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.want {
				t.Errorf("task.category=%s, eq %q → %v, want %v",
					tt.category.String(), tt.value, result, tt.want)
			}
		})
	}
}

func TestEvaluateCondition_TaskFields_Priority(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		priority orcv1.TaskPriority
		value    string
		want     bool
	}{
		{"critical", orcv1.TaskPriority_TASK_PRIORITY_CRITICAL, "critical", true},
		{"high", orcv1.TaskPriority_TASK_PRIORITY_HIGH, "high", true},
		{"normal", orcv1.TaskPriority_TASK_PRIORITY_NORMAL, "normal", true},
		{"low", orcv1.TaskPriority_TASK_PRIORITY_LOW, "low", true},
		{"mismatch", orcv1.TaskPriority_TASK_PRIORITY_HIGH, "low", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := newTestConditionContext(t)
			ctx.Task.Priority = tt.priority

			cond := `{"field": "task.priority", "op": "eq", "value": "` + tt.value + `"}`
			result, err := EvaluateCondition(cond, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.want {
				t.Errorf("task.priority=%s, eq %q → %v, want %v",
					tt.priority.String(), tt.value, result, tt.want)
			}
		})
	}
}

// =============================================================================
// SC-4: Field resolver resolves var.*, env.*, phase_output.* fields
// =============================================================================

func TestEvaluateCondition_FieldResolution_Var(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"field": "var.MY_VAR", "op": "eq", "value": "hello"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("var.MY_VAR should resolve to 'hello'")
	}
}

func TestEvaluateCondition_FieldResolution_Var_Missing(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// Missing variable: exists should return false
	result, err := EvaluateCondition(
		`{"field": "var.NONEXISTENT", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("exists on missing variable should return false")
	}
}

func TestEvaluateCondition_FieldResolution_Env(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"field": "env.HOME", "op": "eq", "value": "/home/test"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("env.HOME should resolve from ResolutionContext.Environment")
	}
}

func TestEvaluateCondition_FieldResolution_Env_Missing(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"field": "env.NONEXISTENT_XYZ_123", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("exists on missing env var should return false")
	}
}

func TestEvaluateCondition_FieldResolution_PhaseOutput_Status(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"field": "phase_output.spec.status", "op": "eq", "value": "complete"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("phase_output.spec.status should resolve to 'complete' from JSON output")
	}
}

func TestEvaluateCondition_FieldResolution_PhaseOutput_MissingPhase(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"field": "phase_output.nonexistent.status", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("exists on missing phase output should return false")
	}
}

func TestEvaluateCondition_FieldResolution_PhaseOutput_Unparseable(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)
	// Set phase output to non-JSON
	ctx.RCtx.PriorOutputs["bad_phase"] = "not json at all"

	result, err := EvaluateCondition(
		`{"field": "phase_output.bad_phase.status", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("exists on unparseable phase output nested field should return false")
	}
}

// =============================================================================
// SC-6: {"all": [...]} compound condition
// =============================================================================

func TestEvaluateCondition_All_AllTrue(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// task.category=feature, task.priority=normal — both true
	result, err := EvaluateCondition(
		`{"all": [
			{"field": "task.category", "op": "eq", "value": "feature"},
			{"field": "task.priority", "op": "eq", "value": "normal"}
		]}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("all should return true when all sub-conditions are true")
	}
}

func TestEvaluateCondition_All_OneFalse(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// task.category=feature (true), task.category=bug (false)
	result, err := EvaluateCondition(
		`{"all": [
			{"field": "task.category", "op": "eq", "value": "feature"},
			{"field": "task.category", "op": "eq", "value": "bug"}
		]}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("all should return false when any sub-condition is false")
	}
}

func TestEvaluateCondition_All_Empty(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// Vacuous truth: empty all array → true
	result, err := EvaluateCondition(`{"all": []}`, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("all with empty array should return true (vacuous truth)")
	}
}

// =============================================================================
// SC-7: {"any": [...]} compound condition
// =============================================================================

func TestEvaluateCondition_Any_OneTrue(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"any": [
			{"field": "task.category", "op": "eq", "value": "bug"},
			{"field": "task.category", "op": "eq", "value": "feature"}
		]}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("any should return true when any sub-condition is true")
	}
}

func TestEvaluateCondition_Any_AllFalse(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"any": [
			{"field": "task.category", "op": "eq", "value": "docs"},
			{"field": "task.category", "op": "eq", "value": "chore"}
		]}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("any should return false when all sub-conditions are false")
	}
}

func TestEvaluateCondition_Any_Empty(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(`{"any": []}`, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("any with empty array should return false")
	}
}

// =============================================================================
// Edge cases from spec
// =============================================================================

func TestEvaluateCondition_EmptyString(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// Empty condition string → phase should execute (no condition = always run)
	result, err := EvaluateCondition("", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("empty condition string should return true (no condition = always run)")
	}
}

func TestEvaluateCondition_NullString(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition("null", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("null condition should return true (no condition = always run)")
	}
}

func TestEvaluateCondition_DeepNesting(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// all containing any containing simple conditions
	result, err := EvaluateCondition(
		`{"all": [
			{"any": [
				{"field": "task.category", "op": "eq", "value": "feature"},
				{"field": "task.category", "op": "eq", "value": "bug"}
			]},
			{"all": [
				{"field": "task.priority", "op": "eq", "value": "normal"},
				{"field": "env.HOME", "op": "exists"}
			]}
		]}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("deeply nested compound condition should evaluate recursively")
	}
}

func TestEvaluateCondition_InSingleElement(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	result, err := EvaluateCondition(
		`{"field": "task.category", "op": "in", "value": ["feature"]}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("in with single-element array should work like eq")
	}
}

func TestEvaluateCondition_ExistsEmpty(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)
	// Set a var to empty string
	ctx.Vars["EMPTY_VAR"] = ""

	result, err := EvaluateCondition(
		`{"field": "var.EMPTY_VAR", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("exists on empty string value should return false")
	}
}

func TestEvaluateCondition_ContainsEmptyField(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)
	ctx.Vars["EMPTY_VAR"] = ""

	result, err := EvaluateCondition(
		`{"field": "var.EMPTY_VAR", "op": "contains", "value": "anything"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("contains on empty field value should return false")
	}
}

func TestEvaluateCondition_GtLtStringFallback(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)
	ctx.Vars["ALPHA"] = "beta"

	// String comparison: "beta" > "alpha" is true
	result, err := EvaluateCondition(
		`{"field": "var.ALPHA", "op": "gt", "value": "alpha"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("gt with non-numeric values should fall back to string comparison")
	}

	// String comparison: "beta" < "gamma" is true
	result, err = EvaluateCondition(
		`{"field": "var.ALPHA", "op": "lt", "value": "gamma"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("lt with non-numeric values should fall back to string comparison")
	}
}

// =============================================================================
// Failure modes from spec
// =============================================================================

func TestEvaluateCondition_InvalidJSON(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	_, err := EvaluateCondition("not valid json {{{", ctx)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestEvaluateCondition_UnknownFieldPrefix(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// Unknown prefix: field resolves to ("", false)
	// exists → false
	result, err := EvaluateCondition(
		`{"field": "unknown.thing", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("exists on unknown field prefix should return false")
	}
}

func TestEvaluateCondition_NilTask(t *testing.T) {
	t.Parallel()

	ctx := &ConditionContext{
		Task: nil,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			Environment: map[string]string{},
		},
	}

	// task.category with nil task: resolves to ("", false)
	// exists → false
	result, err := EvaluateCondition(
		`{"field": "task.category", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("task.category with nil task should resolve to empty → exists returns false")
	}
}

func TestEvaluateCondition_InNonArray(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// "in" operator with string instead of array → parse error
	_, err := EvaluateCondition(
		`{"field": "task.category", "op": "in", "value": "feature"}`,
		ctx,
	)
	if err == nil {
		t.Fatal("expected error when 'in' operator has non-array value, got nil")
	}
}

func TestEvaluateCondition_AmbiguousCondition(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// Has both simple fields AND compound fields → error
	_, err := EvaluateCondition(
		`{"field": "task.category", "op": "eq", "value": "feature", "all": [{"field": "task.priority", "op": "eq", "value": "normal"}]}`,
		ctx,
	)
	if err == nil {
		t.Fatal("expected error for ambiguous condition (both simple and compound fields)")
	}
}

// =============================================================================
// BDD-3: Condition means "run when true" — exists on set env var executes phase
// =============================================================================

func TestEvaluateCondition_ExistsOnSetEnvVar(t *testing.T) {
	t.Parallel()
	ctx := newTestConditionContext(t)

	// env.SKIP_SPEC is set to "1" → exists returns true → phase should execute
	result, err := EvaluateCondition(
		`{"field": "env.SKIP_SPEC", "op": "exists"}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("exists on set env var should return true (phase should execute)")
	}
}

// =============================================================================
// BDD-2: Compound condition with in + neq
// =============================================================================

func TestEvaluateCondition_CompoundInAndNeq(t *testing.T) {
	t.Parallel()

	// Feature category task, normal priority
	ctx := newTestConditionContext(t)
	ctx.Task.Category = orcv1.TaskCategory_TASK_CATEGORY_DOCS

	// Condition: category in [feature, bug] AND priority != low
	// Since category is "docs", the first condition fails → all returns false
	result, err := EvaluateCondition(
		`{"all": [
			{"field": "task.category", "op": "in", "value": ["feature", "bug"]},
			{"field": "task.priority", "op": "neq", "value": "low"}
		]}`,
		ctx,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("all should return false when category is 'docs' and condition requires feature/bug")
	}
}
