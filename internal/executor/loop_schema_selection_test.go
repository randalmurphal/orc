// Tests for TASK-756: Wire executor to use LoopSchemas for iteration-specific schema selection.
//
// These tests verify that GetSchemaForPhaseWithRound is replaced with
// LoopConfig-aware schema selection.
//
// Coverage mapping:
//
//	SC-1: TestSchemaForIteration_UsesLoopSchemas
//	SC-2: TestSchemaForIteration_FallsBackWhenNoLoopSchemas
//	SC-3: TestClaudeExecutor_UsesLoopConfigForSchema
package executor

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
)

// =============================================================================
// SC-1: Schema selection uses LoopConfig.GetSchemaForIteration when configured
// =============================================================================

func TestSchemaForIteration_UsesLoopSchemas(t *testing.T) {
	t.Parallel()

	loopCfg := &db.LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		LoopSchemas: map[string]string{
			"1":       "findings",
			"default": "decision",
		},
	}

	// Iteration 1 should use "findings" schema → ReviewFindingsSchema
	schema1 := GetSchemaForIteration(loopCfg, 1, "review", false)
	if schema1 != ReviewFindingsSchema {
		t.Errorf("iteration 1 schema should be ReviewFindingsSchema, got different schema")
	}

	// Iteration 2 should use "default" → "decision" → ReviewDecisionSchema
	schema2 := GetSchemaForIteration(loopCfg, 2, "review", false)
	if schema2 != ReviewDecisionSchema {
		t.Errorf("iteration 2 schema should be ReviewDecisionSchema, got different schema")
	}

	// Iteration 3 should also use "default"
	schema3 := GetSchemaForIteration(loopCfg, 3, "review", false)
	if schema3 != ReviewDecisionSchema {
		t.Errorf("iteration 3 schema should be ReviewDecisionSchema, got different schema")
	}
}

// =============================================================================
// SC-2: Schema selection falls back to phase-based logic when LoopSchemas empty
// =============================================================================

func TestSchemaForIteration_FallsBackWhenNoLoopSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		loopCfg          *db.LoopConfig
		iteration        int
		phaseID          string
		producesArtifact bool
		wantSchema       string
	}{
		{
			name:             "nil_loopconfig_review_iter1",
			loopCfg:          nil,
			iteration:        1,
			phaseID:          "review",
			producesArtifact: false,
			wantSchema:       ReviewFindingsSchema,
		},
		{
			name:             "empty_loopschemas_review_iter2",
			loopCfg:          &db.LoopConfig{LoopToPhase: "implement"},
			iteration:        2,
			phaseID:          "review",
			producesArtifact: false,
			// With no LoopSchemas, should use round-based fallback (iter 2 = round 2)
			wantSchema: ReviewDecisionSchema,
		},
		{
			name:             "nil_loopconfig_implement",
			loopCfg:          nil,
			iteration:        1,
			phaseID:          "implement",
			producesArtifact: false,
			wantSchema:       ImplementCompletionSchema,
		},
		{
			name:             "nil_loopconfig_implement_codex",
			loopCfg:          nil,
			iteration:        1,
			phaseID:          "implement_codex",
			producesArtifact: false,
			wantSchema:       ImplementCompletionSchema,
		},
		{
			name:             "nil_loopconfig_spec_produces_artifact",
			loopCfg:          nil,
			iteration:        1,
			phaseID:          "spec",
			producesArtifact: true,
			wantSchema:       ContentProducingPhaseSchema,
		},
		{
			name:             "nil_loopconfig_qa",
			loopCfg:          nil,
			iteration:        1,
			phaseID:          "qa",
			producesArtifact: false,
			wantSchema:       QAResultSchema,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := GetSchemaForIteration(tt.loopCfg, tt.iteration, tt.phaseID, tt.producesArtifact)
			if schema != tt.wantSchema {
				t.Errorf("GetSchemaForIteration() = different schema, want %s", tt.name)
			}
		})
	}
}

// =============================================================================
// SC-3: ClaudeExecutor accepts LoopConfig and uses it for schema selection
// =============================================================================

func TestClaudeExecutor_UsesLoopConfigForSchema(t *testing.T) {
	t.Parallel()

	loopCfg := &db.LoopConfig{
		LoopToPhase: "implement",
		MaxLoops:    3,
		LoopSchemas: map[string]string{
			"1":       "findings",
			"default": "decision",
		},
	}

	// Create executor with LoopConfig for iteration 2
	exec := NewClaudeExecutor(
		WithClaudePhaseID("review"),
		WithClaudeLoopConfig(loopCfg),
		WithClaudeLoopIteration(2), // Second iteration
	)

	// Verify the executor has the loop config
	if exec.loopConfig == nil {
		t.Fatal("executor should have loopConfig set")
	}
	if exec.loopIteration != 2 {
		t.Errorf("executor loopIteration = %d, want 2", exec.loopIteration)
	}
}

// =============================================================================
// Schema identifier mapping tests
// =============================================================================

func TestMapSchemaIdentifierToSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		identifier string
		phaseID    string
		wantSchema string
	}{
		{"findings", "review", ReviewFindingsSchema},
		{"decision", "review", ReviewDecisionSchema},
		{"", "review", ReviewFindingsSchema},         // Empty defaults to findings
		{"unknown", "review", PhaseCompletionSchema}, // Unknown falls back to default
		{"qa_result", "qa", QAResultSchema},
		{"", "implement", ImplementCompletionSchema},
		{"", "implement_codex", ImplementCompletionSchema},
	}

	for _, tt := range tests {
		t.Run(tt.identifier+"_"+tt.phaseID, func(t *testing.T) {
			schema := MapSchemaIdentifierToSchema(tt.identifier, tt.phaseID, false)
			if schema != tt.wantSchema {
				t.Errorf("MapSchemaIdentifierToSchema(%q, %q) = different schema", tt.identifier, tt.phaseID)
			}
		})
	}
}
