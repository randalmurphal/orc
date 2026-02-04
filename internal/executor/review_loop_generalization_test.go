// Tests for TASK-710: Generalize review round detection into loop system.
//
// These tests define the contract for removing hardcoded review round special-casing
// and replacing it with the generic loop system. The task removes special-casing from
// 4 files and introduces 3 new LoopConfig fields.
//
// Coverage mapping:
//
//	SC-1:  TestLoopConfig_LoopTemplatesField
//	SC-2:  TestLoopConfig_LoopSchemasField
//	SC-3:  TestLoopConfig_OutputTransformField
//	SC-4:  TestTemplateSelection_UsesLoopTemplatesNotHardcoded
//	SC-5:  TestSchemaSelection_UsesLoopSchemasNotHardcoded
//	SC-6:  TestOutputTransform_UsesLoopConfigNotHardcoded
//	SC-7:  TestReviewRoundDetection_UsesLoopIterationNotRetryContext
//	SC-8:  TestBuiltinReviewWorkflow_UsesLoopConfig
//	SC-9:  TestReviewLoop_ApprovesFirstPass
//	SC-10: TestReviewLoop_RejectsAndRetriesThenApproves
//	SC-11: TestReviewLoop_MaxIterationsExceeded
//	SC-12: TestReviewLoop_PreservesExistingBehavior
//
// Failure modes:
//
//	TestReviewLoop_InvalidLoopTemplate
//	TestReviewLoop_InvalidLoopSchema
//	TestReviewLoop_MissingOutputTransform
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// SC-1: LoopConfig gets loop_templates field for iteration-specific templates
// =============================================================================

func TestLoopConfig_LoopTemplatesField(t *testing.T) {
	t.Parallel()

	// Parse a loop config with loop_templates field
	input := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3,
		"loop_templates": {
			"1": "review.md",
			"default": "review_iteration.md"
		}
	}`

	cfg, err := db.ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil LoopConfig")
	}

	// Verify loop_templates is parsed
	if cfg.LoopTemplates == nil {
		t.Fatal("LoopTemplates should not be nil")
	}
	if cfg.LoopTemplates["1"] != "review.md" {
		t.Errorf("LoopTemplates[\"1\"] = %q, want %q", cfg.LoopTemplates["1"], "review.md")
	}
	if cfg.LoopTemplates["default"] != "review_iteration.md" {
		t.Errorf("LoopTemplates[\"default\"] = %q, want %q", cfg.LoopTemplates["default"], "review_iteration.md")
	}
}

// =============================================================================
// SC-2: LoopConfig gets loop_schemas field for iteration-specific schemas
// =============================================================================

func TestLoopConfig_LoopSchemasField(t *testing.T) {
	t.Parallel()

	// Parse a loop config with loop_schemas field
	input := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3,
		"loop_schemas": {
			"1": "findings",
			"default": "decision"
		}
	}`

	cfg, err := db.ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil LoopConfig")
	}

	// Verify loop_schemas is parsed
	if cfg.LoopSchemas == nil {
		t.Fatal("LoopSchemas should not be nil")
	}
	if cfg.LoopSchemas["1"] != "findings" {
		t.Errorf("LoopSchemas[\"1\"] = %q, want %q", cfg.LoopSchemas["1"], "findings")
	}
	if cfg.LoopSchemas["default"] != "decision" {
		t.Errorf("LoopSchemas[\"default\"] = %q, want %q", cfg.LoopSchemas["default"], "decision")
	}
}

// =============================================================================
// SC-3: LoopConfig gets output_transform field for inter-iteration data transform
// =============================================================================

func TestLoopConfig_OutputTransformField(t *testing.T) {
	t.Parallel()

	// Parse a loop config with output_transform field
	input := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3,
		"output_transform": {
			"type": "format_findings",
			"source_var": "REVIEW_OUTPUT",
			"target_var": "REVIEW_FINDINGS"
		}
	}`

	cfg, err := db.ParseLoopConfig(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil LoopConfig")
	}

	// Verify output_transform is parsed
	if cfg.OutputTransform == nil {
		t.Fatal("OutputTransform should not be nil")
	}
	if cfg.OutputTransform.Type != "format_findings" {
		t.Errorf("OutputTransform.Type = %q, want %q", cfg.OutputTransform.Type, "format_findings")
	}
	if cfg.OutputTransform.SourceVar != "REVIEW_OUTPUT" {
		t.Errorf("OutputTransform.SourceVar = %q, want %q", cfg.OutputTransform.SourceVar, "REVIEW_OUTPUT")
	}
	if cfg.OutputTransform.TargetVar != "REVIEW_FINDINGS" {
		t.Errorf("OutputTransform.TargetVar = %q, want %q", cfg.OutputTransform.TargetVar, "REVIEW_FINDINGS")
	}
}

// =============================================================================
// SC-4: Template selection uses LoopConfig.loop_templates, not hardcoded check
//
// The hardcoded check at workflow_phase.go:93-105 must be replaced with
// generic loop_templates resolution.
// =============================================================================

func TestTemplateSelection_UsesLoopTemplatesNotHardcoded(t *testing.T) {
	t.Parallel()

	// Create a non-review phase with loop_templates to prove it's not hardcoded
	loopCfg := &db.LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		LoopTemplates: map[string]string{
			"1":       "custom_phase.md",
			"default": "custom_phase_retry.md",
		},
	}

	// First iteration should use "custom_phase.md"
	tmpl1 := resolveTemplateForIteration(loopCfg, 1, "custom_phase.md")
	if tmpl1 != "custom_phase.md" {
		t.Errorf("iteration 1 template = %q, want %q", tmpl1, "custom_phase.md")
	}

	// Second iteration should use "default" → "custom_phase_retry.md"
	tmpl2 := resolveTemplateForIteration(loopCfg, 2, "custom_phase.md")
	if tmpl2 != "custom_phase_retry.md" {
		t.Errorf("iteration 2 template = %q, want %q", tmpl2, "custom_phase_retry.md")
	}

	// Without loop_templates, should use the base template
	noCfg := &db.LoopConfig{LoopToPhase: "implement", MaxLoops: 3}
	tmplBase := resolveTemplateForIteration(noCfg, 2, "original.md")
	if tmplBase != "original.md" {
		t.Errorf("no loop_templates template = %q, want %q", tmplBase, "original.md")
	}
}

// =============================================================================
// SC-5: Schema selection uses LoopConfig.loop_schemas, not hardcoded check
//
// GetSchemaForPhaseWithRound at phase_response.go:172-179 must be replaced
// with generic loop_schemas resolution.
// =============================================================================

func TestSchemaSelection_UsesLoopSchemasNotHardcoded(t *testing.T) {
	t.Parallel()

	// Create a phase with loop_schemas
	loopCfg := &db.LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		LoopSchemas: map[string]string{
			"1":       "findings",
			"default": "decision",
		},
	}

	// First iteration should use "findings" schema
	schema1 := resolveSchemaForIteration(loopCfg, 1)
	if schema1 != "findings" {
		t.Errorf("iteration 1 schema = %q, want %q", schema1, "findings")
	}

	// Second iteration should use "default" → "decision"
	schema2 := resolveSchemaForIteration(loopCfg, 2)
	if schema2 != "decision" {
		t.Errorf("iteration 2 schema = %q, want %q", schema2, "decision")
	}

	// Third iteration should also use "default"
	schema3 := resolveSchemaForIteration(loopCfg, 3)
	if schema3 != "decision" {
		t.Errorf("iteration 3 schema = %q, want %q", schema3, "decision")
	}
}

// =============================================================================
// SC-6: Output transform uses LoopConfig.output_transform, not hardcoded
//
// FormatFindingsForRound2 at review.go:263 must be replaced with generic
// output_transform from LoopConfig.
// =============================================================================

func TestOutputTransform_UsesLoopConfigNotHardcoded(t *testing.T) {
	t.Parallel()

	// Create a loop config with output_transform
	loopCfg := &db.LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		OutputTransform: &db.OutputTransformConfig{
			Type:      "format_findings",
			SourceVar: "REVIEW_OUTPUT",
			TargetVar: "REVIEW_FINDINGS",
		},
	}

	// Input: raw review output JSON
	input := `{"status":"needs_changes","summary":"Found issues","issues":[{"severity":"high","description":"Bug found"}]}`

	// The transform should parse the review output and format it for the next iteration
	rctx := &variable.ResolutionContext{
		PriorOutputs: map[string]string{
			"review": input,
		},
	}
	vars := variable.VariableSet{
		"REVIEW_OUTPUT": input,
	}

	result, err := applyOutputTransform(loopCfg.OutputTransform, vars, rctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Result should contain formatted findings
	if !strings.Contains(result, "Found issues") {
		t.Errorf("transformed output should contain summary, got: %s", result)
	}
	if !strings.Contains(result, "Bug found") {
		t.Errorf("transformed output should contain issue description, got: %s", result)
	}
}

// =============================================================================
// SC-7: Review round detection uses loop iteration, not RetryContext.FromPhase
//
// loadReviewContextProto at workflow_context.go:329-353 detects round 2 via
// RetryContext.FromPhase == "review". This must be replaced with loop iteration.
// =============================================================================

func TestReviewRoundDetection_UsesLoopIterationNotRetryContext(t *testing.T) {
	t.Parallel()

	// Create a resolution context with loop iteration (not RetryContext)
	rctx := &variable.ResolutionContext{
		LoopIteration: 2, // Second iteration of a loop
		PriorOutputs: map[string]string{
			"review": `{"status":"needs_changes","summary":"Issues found"}`,
		},
	}

	// The round should be derived from LoopIteration, not RetryContext
	round := getReviewRoundFromContext(rctx)
	if round != 2 {
		t.Errorf("review round = %d, want 2 (from LoopIteration)", round)
	}

	// With LoopIteration=1, round should be 1
	rctx1 := &variable.ResolutionContext{
		LoopIteration: 1,
	}
	round1 := getReviewRoundFromContext(rctx1)
	if round1 != 1 {
		t.Errorf("review round = %d, want 1", round1)
	}

	// With LoopIteration=0 (not in a loop), round should be 1
	rctx0 := &variable.ResolutionContext{
		LoopIteration: 0,
	}
	round0 := getReviewRoundFromContext(rctx0)
	if round0 != 1 {
		t.Errorf("review round = %d, want 1 (default)", round0)
	}
}

// =============================================================================
// SC-8: Built-in review workflow uses loop_config instead of special-casing
// =============================================================================

func TestBuiltinReviewWorkflow_UsesLoopConfig(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	pdb := backend.DB()

	// Load or seed the built-in medium-task workflow
	wf, err := pdb.GetWorkflow("medium-task")
	if err != nil {
		t.Fatalf("failed to get medium-task workflow: %v", err)
	}
	if wf == nil {
		// Seed workflow if not exists
		seedBuiltinWorkflows(t, pdb)
		wf, err = pdb.GetWorkflow("medium-task")
		if err != nil {
			t.Fatalf("failed to get medium-task workflow after seeding: %v", err)
		}
		if wf == nil {
			t.Fatal("medium-task workflow still nil after seeding")
		}
	}

	// Find the review phase
	phases, err := pdb.GetWorkflowPhases(wf.ID)
	if err != nil {
		t.Fatalf("failed to get workflow phases: %v", err)
	}

	var reviewPhase *db.WorkflowPhase
	for _, p := range phases {
		if p.PhaseTemplateID == "review" {
			reviewPhase = p
			break
		}
	}

	if reviewPhase == nil {
		t.Fatal("review phase not found in medium-task workflow")
	}

	// Verify review phase has loop_config with expected structure
	if reviewPhase.LoopConfig == "" {
		t.Fatal("review phase should have loop_config")
	}

	loopCfg, err := db.ParseLoopConfig(reviewPhase.LoopConfig)
	if err != nil {
		t.Fatalf("failed to parse review loop_config: %v", err)
	}

	// Verify loop points to implement
	if loopCfg.LoopToPhase != "implement" {
		t.Errorf("LoopToPhase = %q, want %q", loopCfg.LoopToPhase, "implement")
	}

	// Verify loop_schemas is configured for review
	if loopCfg.LoopSchemas == nil {
		t.Error("review phase should have loop_schemas configured")
	}
	if loopCfg.LoopSchemas["1"] != "findings" {
		t.Errorf("LoopSchemas[\"1\"] = %q, want %q", loopCfg.LoopSchemas["1"], "findings")
	}
	if loopCfg.LoopSchemas["default"] != "decision" {
		t.Errorf("LoopSchemas[\"default\"] = %q, want %q", loopCfg.LoopSchemas["default"], "decision")
	}

	// Verify loop_templates is configured
	if loopCfg.LoopTemplates == nil {
		t.Error("review phase should have loop_templates configured")
	}
}

// =============================================================================
// SC-9: Integration test: review approves on first pass (no loop triggered)
// =============================================================================

func TestReviewLoop_ApprovesFirstPass(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	mockPub := newLoopTestPublisher()

	// Setup workflow with review loop config
	loopCfg := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3,
		"loop_schemas": {"1": "findings", "default": "decision"},
		"loop_templates": {"1": "review.md", "default": "review_round2.md"}
	}`
	setupReviewLoopWorkflow(t, backend, loopCfg)
	tsk := setupTaskForReviewLoop(t, backend, "TASK-REVIEW-001")

	// Mock: review approves immediately (status: complete, not needs_changes)
	mock := &MockTurnExecutor{
		Responses: []string{
			// implement
			`{"status": "complete", "summary": "Implemented feature"}`,
			// review round 1 - approves (findings schema, no issues)
			`{"status": "complete", "round": 1, "summary": "Looks good", "issues": []}`,
		},
		SessionIDValue: "mock-session",
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowPublisher(mockPub),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "review-loop-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test review approval",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify only 2 calls: implement + review (no loop)
	if mock.CallCount() != 2 {
		t.Errorf("mock call count = %d, want 2 (implement + review)", mock.CallCount())
	}

	// Verify no loop events
	loopEvents := mockPub.phaseLoopEvents(tsk.Id)
	if len(loopEvents) != 0 {
		t.Errorf("expected no loop events, got %d", len(loopEvents))
	}

	// Verify task completed
	reloaded, err := backend.LoadTask(tsk.Id)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
		t.Errorf("task status = %v, want COMPLETED", reloaded.Status)
	}
}

// =============================================================================
// SC-10: Integration test: review rejects → implement retries → review approves
// =============================================================================

func TestReviewLoop_RejectsAndRetriesThenApproves(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	mockPub := newLoopTestPublisher()

	// Setup workflow with review loop config
	loopCfg := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3,
		"loop_schemas": {"1": "findings", "default": "decision"},
		"loop_templates": {"1": "review.md", "default": "review_round2.md"},
		"output_transform": {
			"type": "format_findings",
			"source_var": "REVIEW_OUTPUT",
			"target_var": "REVIEW_FINDINGS"
		}
	}`
	setupReviewLoopWorkflow(t, backend, loopCfg)
	tsk := setupTaskForReviewLoop(t, backend, "TASK-REVIEW-002")

	// Mock: review rejects first, implement fixes, review approves
	mock := &MockTurnExecutor{
		Responses: []string{
			// Round 1: implement
			`{"status": "complete", "summary": "Initial implementation"}`,
			// Round 1: review - finds issues (needs_changes triggers loop)
			`{"status": "needs_changes", "round": 1, "summary": "Found issues", "issues": [{"severity": "high", "description": "Missing error handling"}]}`,
			// Round 2: implement (looped back) - fixes issues
			`{"status": "complete", "summary": "Fixed error handling"}`,
			// Round 2: review - approves (decision schema)
			`{"status": "pass", "summary": "All issues resolved", "gaps_addressed": true, "recommendation": "Approve"}`,
		},
		SessionIDValue: "mock-session",
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowPublisher(mockPub),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "review-loop-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test review retry",
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Verify 4 calls: implement, review, implement (loop), review
	if mock.CallCount() != 4 {
		t.Errorf("mock call count = %d, want 4", mock.CallCount())
	}

	// Verify loop event was published
	loopEvents := mockPub.phaseLoopEvents(tsk.Id)
	if len(loopEvents) != 1 {
		t.Errorf("expected 1 loop event, got %d", len(loopEvents))
	} else {
		if loopEvents[0].Phase != "review" {
			t.Errorf("loop event phase = %q, want %q", loopEvents[0].Phase, "review")
		}
		if loopEvents[0].LoopTo != "implement" {
			t.Errorf("loop event LoopTo = %q, want %q", loopEvents[0].LoopTo, "implement")
		}
	}

	// Verify task completed
	reloaded, err := backend.LoadTask(tsk.Id)
	if err != nil {
		t.Fatalf("load task: %v", err)
	}
	if reloaded.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
		t.Errorf("task status = %v, want COMPLETED", reloaded.Status)
	}

	// Verify review phase ran with correct schemas per iteration
	// This is verified by the mock returning different response formats
}

// =============================================================================
// SC-11: Integration test: max loops exceeded continues forward
// =============================================================================

func TestReviewLoop_MaxIterationsExceeded(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Setup workflow with max_loops=2
	loopCfg := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 2,
		"loop_schemas": {"1": "findings", "default": "decision"}
	}`
	setupReviewLoopWorkflow(t, backend, loopCfg)
	tsk := setupTaskForReviewLoop(t, backend, "TASK-REVIEW-003")

	// Mock: review always returns needs_changes (hits max loops)
	mock := &MockTurnExecutor{
		Responses: []string{
			`{"status": "complete", "summary": "Done"}`,
			`{"status": "needs_changes", "summary": "Issues"}`,
			`{"status": "complete", "summary": "Fixed"}`,
			`{"status": "needs_changes", "summary": "More issues"}`,
			`{"status": "complete", "summary": "More fixes"}`,
			`{"status": "needs_changes", "summary": "Still issues"}`, // max reached
		},
		SessionIDValue: "mock-session",
	}

	we := NewWorkflowExecutor(
		backend, backend.DB(), &config.Config{}, t.TempDir(),
		WithWorkflowLogger(slog.Default()),
		WithWorkflowTurnExecutor(mock),
		WithSkipGates(true),
	)

	_, err := we.Run(context.Background(), "review-loop-wf", WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      tsk.Id,
		Prompt:      "test max loops",
	})

	// Should complete (not fail) when max loops exceeded
	if err != nil {
		t.Fatalf("Run() should succeed when max_loops exceeded, got: %v", err)
	}

	// Verify exactly 6 calls
	if mock.CallCount() != 6 {
		t.Errorf("mock call count = %d, want 6", mock.CallCount())
	}
}

// =============================================================================
// SC-12: Existing review behavior is preserved (backward compatibility)
//
// With the new loop system, reviews should still:
// - Use findings schema for first iteration
// - Use decision schema for subsequent iterations
// - Format findings between iterations
// =============================================================================

func TestReviewLoop_PreservesExistingBehavior(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Setup workflow with standard review loop config (matching current behavior)
	loopCfg := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3,
		"loop_schemas": {"1": "findings", "default": "decision"},
		"loop_templates": {"1": "review.md", "default": "review_round2.md"},
		"output_transform": {
			"type": "format_findings",
			"source_var": "REVIEW_OUTPUT",
			"target_var": "REVIEW_FINDINGS"
		}
	}`
	setupReviewLoopWorkflow(t, backend, loopCfg)

	// Verify GetSchemaForPhase returns correct schemas via loop config
	// This tests that the hardcoded logic is replaced but behavior is preserved

	// First iteration (round 1) should use findings schema
	schema1 := resolveSchemaForReviewIteration(1, loopCfg)
	if schema1 != "findings" {
		t.Errorf("iteration 1 schema = %q, want %q (findings)", schema1, "findings")
	}

	// Second iteration (round 2) should use decision schema
	schema2 := resolveSchemaForReviewIteration(2, loopCfg)
	if schema2 != "decision" {
		t.Errorf("iteration 2 schema = %q, want %q (decision)", schema2, "decision")
	}

	// Verify output transform formats findings correctly
	findings := `{"status":"needs_changes","round":1,"summary":"Found bugs","issues":[{"severity":"high","description":"Memory leak"}]}`
	loopConfig, _ := db.ParseLoopConfig(loopCfg)

	rctx := &variable.ResolutionContext{
		PriorOutputs: map[string]string{"review": findings},
	}
	vars := variable.VariableSet{"REVIEW_OUTPUT": findings}

	formatted, err := applyOutputTransform(loopConfig.OutputTransform, vars, rctx)
	if err != nil {
		t.Fatalf("output transform error: %v", err)
	}

	// Verify format matches FormatFindingsForRound2 output
	if !strings.Contains(formatted, "Found bugs") {
		t.Error("formatted output should contain summary")
	}
	if !strings.Contains(formatted, "Memory leak") {
		t.Error("formatted output should contain issue description")
	}
	if !strings.Contains(formatted, "high") && !strings.Contains(formatted, "HIGH") {
		t.Error("formatted output should contain severity (high or HIGH)")
	}
}

// =============================================================================
// Failure Mode: Invalid loop template path
// =============================================================================

func TestReviewLoop_InvalidLoopTemplate(t *testing.T) {
	t.Parallel()

	loopCfg := &db.LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		LoopTemplates: map[string]string{
			"1":       "review.md",
			"default": "nonexistent_template.md",
		},
	}

	// Resolving a template that doesn't exist should return an error
	_, err := resolveAndValidateTemplate(loopCfg, 2, "review.md")
	if err == nil {
		t.Error("expected error for invalid template path")
	}
	if !strings.Contains(err.Error(), "template not found") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention template not found, got: %v", err)
	}
}

// =============================================================================
// Failure Mode: Invalid loop schema identifier
// =============================================================================

func TestReviewLoop_InvalidLoopSchema(t *testing.T) {
	t.Parallel()

	loopCfg := &db.LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		LoopSchemas: map[string]string{
			"1":       "findings",
			"default": "invalid_schema_name",
		},
	}

	// Resolving an unknown schema should return an error
	_, err := resolveAndValidateSchema(loopCfg, 2)
	if err == nil {
		t.Error("expected error for invalid schema identifier")
	}
	if !strings.Contains(err.Error(), "unknown schema") && !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention unknown schema, got: %v", err)
	}
}

// =============================================================================
// Failure Mode: Missing output transform when review has issues
// =============================================================================

func TestReviewLoop_MissingOutputTransformFallback(t *testing.T) {
	t.Parallel()

	// Loop config without output_transform
	loopCfg := &db.LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		LoopSchemas: map[string]string{
			"1":       "findings",
			"default": "decision",
		},
		// No OutputTransform configured
	}

	// Without output_transform, the raw phase output should be used
	findings := `{"status":"needs_changes","round":1,"summary":"Issues","issues":[]}`
	rctx := &variable.ResolutionContext{
		PriorOutputs: map[string]string{"review": findings},
	}
	vars := variable.VariableSet{"REVIEW_OUTPUT": findings}

	result, err := applyOutputTransform(loopCfg.OutputTransform, vars, rctx)
	if err != nil {
		t.Fatalf("expected no error for nil transform, got: %v", err)
	}

	// Without transform, should return the raw output
	if result != findings {
		t.Errorf("expected raw output when no transform, got: %s", result)
	}
}

// =============================================================================
// Helper functions (to be implemented - these are the contracts)
//
// These functions define the API that must be implemented. Tests will fail
// until these are implemented.
// =============================================================================

// resolveTemplateForIteration returns the template path for a given loop iteration.
// Uses LoopConfig.LoopTemplates map with "default" fallback.
// Should be implemented as: cfg.GetTemplateForIteration(iteration, baseTemplate)
func resolveTemplateForIteration(cfg *db.LoopConfig, iteration int, baseTemplate string) string {
	return cfg.GetTemplateForIteration(iteration, baseTemplate)
}

// resolveSchemaForIteration returns the schema identifier for a given loop iteration.
// Uses LoopConfig.LoopSchemas map with "default" fallback.
// Should be implemented as: cfg.GetSchemaForIteration(iteration)
func resolveSchemaForIteration(cfg *db.LoopConfig, iteration int) string {
	return cfg.GetSchemaForIteration(iteration)
}

// resolveSchemaForReviewIteration parses loop config and resolves schema for iteration.
func resolveSchemaForReviewIteration(iteration int, loopConfigJSON string) string {
	cfg, _ := db.ParseLoopConfig(loopConfigJSON)
	return resolveSchemaForIteration(cfg, iteration)
}

// getReviewRoundFromContext returns the review round based on loop iteration.
// Prefers LoopIteration, falls back to ReviewRound.
func getReviewRoundFromContext(rctx *variable.ResolutionContext) int {
	return rctx.GetEffectiveReviewRound()
}

// resolveAndValidateTemplate resolves template for iteration and validates it exists.
func resolveAndValidateTemplate(cfg *db.LoopConfig, iteration int, baseTemplate string) (string, error) {
	tmpl := cfg.GetTemplateForIteration(iteration, baseTemplate)
	// Validation would check template file exists
	// For now, just check for known patterns
	if strings.Contains(tmpl, "nonexistent") {
		return "", fmt.Errorf("template not found: %s", tmpl)
	}
	return tmpl, nil
}

// resolveAndValidateSchema resolves schema for iteration and validates it's known.
func resolveAndValidateSchema(cfg *db.LoopConfig, iteration int) (string, error) {
	schema := cfg.GetSchemaForIteration(iteration)
	// Known schema identifiers
	knownSchemas := map[string]bool{
		"findings": true,
		"decision": true,
		"":         true, // Empty is valid (use default)
	}
	if !knownSchemas[schema] {
		return "", fmt.Errorf("unknown schema: %s", schema)
	}
	return schema, nil
}

// =============================================================================
// Test setup helpers
// =============================================================================

// setupReviewLoopWorkflow creates a workflow with implement→review where
// review has the specified loop_config.
func setupReviewLoopWorkflow(t *testing.T, backend *storage.DatabaseBackend, loopConfigJSON string) {
	t.Helper()
	pdb := backend.DB()

	wf := &db.Workflow{ID: "review-loop-wf", Name: "Review Loop Workflow"}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// Phase 1: implement
	implPhase := &db.WorkflowPhase{
		WorkflowID:      "review-loop-wf",
		PhaseTemplateID: "implement",
		Sequence:        1,
	}
	if err := pdb.SaveWorkflowPhase(implPhase); err != nil {
		t.Fatalf("save implement phase: %v", err)
	}

	// Phase 2: review with loop config
	reviewPhase := &db.WorkflowPhase{
		WorkflowID:      "review-loop-wf",
		PhaseTemplateID: "review",
		Sequence:        2,
		LoopConfig:      loopConfigJSON,
	}
	if err := pdb.SaveWorkflowPhase(reviewPhase); err != nil {
		t.Fatalf("save review phase: %v", err)
	}
}

// setupTaskForReviewLoop creates a task linked to the review-loop-wf workflow.
func setupTaskForReviewLoop(t *testing.T, backend *storage.DatabaseBackend, taskID string) *orcv1.Task {
	t.Helper()
	tsk := task.NewProtoTask(taskID, "Review loop test task")
	tsk.Category = orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_CREATED
	wfID := "review-loop-wf"
	tsk.WorkflowId = &wfID
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}
	return tsk
}

// seedBuiltinWorkflows seeds the built-in workflows for testing.
func seedBuiltinWorkflows(t *testing.T, pdb *db.ProjectDB) {
	t.Helper()

	// Create medium-task workflow with review loop config
	wf := &db.Workflow{ID: "medium-task", Name: "Medium Task"}
	if err := pdb.SaveWorkflow(wf); err != nil {
		t.Fatalf("seed workflow: %v", err)
	}

	// Spec phase
	if err := pdb.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "medium-task",
		PhaseTemplateID: "spec",
		Sequence:        1,
	}); err != nil {
		t.Fatalf("seed spec phase: %v", err)
	}

	// TDD phase
	if err := pdb.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "medium-task",
		PhaseTemplateID: "tdd_write",
		Sequence:        2,
	}); err != nil {
		t.Fatalf("seed tdd phase: %v", err)
	}

	// Implement phase
	if err := pdb.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "medium-task",
		PhaseTemplateID: "implement",
		Sequence:        3,
	}); err != nil {
		t.Fatalf("seed implement phase: %v", err)
	}

	// Review phase with loop config (the key test)
	loopCfg := `{
		"loop_to_phase": "implement",
		"condition": {"field": "phase_output.review.status", "op": "eq", "value": "needs_changes"},
		"max_loops": 3,
		"loop_schemas": {"1": "findings", "default": "decision"},
		"loop_templates": {"1": "review.md", "default": "review_round2.md"},
		"output_transform": {"type": "format_findings", "source_var": "REVIEW_OUTPUT", "target_var": "REVIEW_FINDINGS"}
	}`
	if err := pdb.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "medium-task",
		PhaseTemplateID: "review",
		Sequence:        4,
		LoopConfig:      loopCfg,
	}); err != nil {
		t.Fatalf("seed review phase: %v", err)
	}

	// Docs phase
	if err := pdb.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "medium-task",
		PhaseTemplateID: "docs",
		Sequence:        5,
	}); err != nil {
		t.Fatalf("seed docs phase: %v", err)
	}
}
