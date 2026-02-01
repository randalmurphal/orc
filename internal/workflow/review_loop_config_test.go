package workflow

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for TASK-759: Update built-in review phase template to use loop_config
//
// Coverage mapping:
//   SC-1: TestReviewPhaseLoopConfig_Seeded - loop_config populated in DB after seeding
//   SC-2: TestReviewPhaseLoopConfig_Values - all required fields configured correctly
//   SC-3: TestReviewPhaseLoopConfig_AllWeightWorkflows - all implement-* workflows have it

// SC-1: Review phase in built-in workflows has loop_config populated in DB after seeding
func TestReviewPhaseLoopConfig_Seeded(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gdb.Close() })

	// Seed built-ins
	_, err = SeedBuiltins(gdb)
	require.NoError(t, err)

	// Get workflow phases for implement-medium
	phases, err := gdb.GetWorkflowPhases("implement-medium")
	require.NoError(t, err)
	require.NotEmpty(t, phases, "implement-medium should have phases")

	// Find the review phase
	var reviewPhase *db.WorkflowPhase
	for _, p := range phases {
		if p.PhaseTemplateID == "review" {
			reviewPhase = p
			break
		}
	}
	require.NotNil(t, reviewPhase, "implement-medium should have a review phase")

	// Verify loop_config is populated
	assert.NotEmpty(t, reviewPhase.LoopConfig, "review phase should have loop_config")
}

// SC-2: loop_config has correct configuration:
//   - loop_to_phase: implement
//   - condition: {field: "phase_output.review.status", op: "eq", value: "needs_changes"}
//   - max_loops: 3
//   - loop_templates: {"1": "review.md", "default": "review_round2.md"}
//   - loop_schemas: {"1": "ReviewFindingsSchema", "default": "ReviewDecisionSchema"}
func TestReviewPhaseLoopConfig_Values(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gdb.Close() })

	// Seed built-ins
	_, err = SeedBuiltins(gdb)
	require.NoError(t, err)

	// Get workflow phases for implement-medium
	phases, err := gdb.GetWorkflowPhases("implement-medium")
	require.NoError(t, err)

	// Find the review phase
	var reviewPhase *db.WorkflowPhase
	for _, p := range phases {
		if p.PhaseTemplateID == "review" {
			reviewPhase = p
			break
		}
	}
	require.NotNil(t, reviewPhase)
	require.NotEmpty(t, reviewPhase.LoopConfig, "loop_config must be set before parsing")

	// Parse loop_config
	cfg, err := db.ParseLoopConfig(reviewPhase.LoopConfig)
	require.NoError(t, err, "loop_config should be valid JSON")
	require.NotNil(t, cfg, "loop_config should parse to non-nil")

	// Verify loop_to_phase
	assert.Equal(t, "implement", cfg.LoopToPhase, "loop_to_phase should be 'implement'")

	// Verify max_loops
	assert.Equal(t, 3, cfg.EffectiveMaxLoops(), "max_loops should be 3")

	// Verify condition is JSON object (not legacy string)
	assert.False(t, cfg.IsLegacyCondition(), "condition should be JSON object format, not legacy string")

	// Parse condition and verify structure
	var condition struct {
		Field string `json:"field"`
		Op    string `json:"op"`
		Value string `json:"value"`
	}
	err = json.Unmarshal(cfg.Condition, &condition)
	require.NoError(t, err, "condition should be valid JSON object")
	assert.Equal(t, "phase_output.review.status", condition.Field)
	assert.Equal(t, "eq", condition.Op)
	assert.Equal(t, "needs_changes", condition.Value)

	// Verify loop_templates
	require.NotNil(t, cfg.LoopTemplates, "loop_templates should be configured")
	assert.Equal(t, "review.md", cfg.LoopTemplates["1"], "first iteration should use review.md")
	assert.Equal(t, "review_round2.md", cfg.LoopTemplates["default"], "subsequent iterations should use review_round2.md")

	// Verify loop_schemas
	require.NotNil(t, cfg.LoopSchemas, "loop_schemas should be configured")
	assert.Equal(t, "ReviewFindingsSchema", cfg.LoopSchemas["1"], "first iteration should use ReviewFindingsSchema")
	assert.Equal(t, "ReviewDecisionSchema", cfg.LoopSchemas["default"], "subsequent iterations should use ReviewDecisionSchema")
}

// SC-3: All weight-based workflows (implement-small, implement-medium, implement-large)
// have the review phase with loop_config
func TestReviewPhaseLoopConfig_AllWeightWorkflows(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = gdb.Close() })

	// Seed built-ins
	_, err = SeedBuiltins(gdb)
	require.NoError(t, err)

	// Check all workflows that have a review phase
	workflowIDs := []string{"implement-small", "implement-medium", "implement-large"}
	for _, wfID := range workflowIDs {
		t.Run(wfID, func(t *testing.T) {
			phases, err := gdb.GetWorkflowPhases(wfID)
			require.NoError(t, err)

			// Find the review phase
			var reviewPhase *db.WorkflowPhase
			for _, p := range phases {
				if p.PhaseTemplateID == "review" {
					reviewPhase = p
					break
				}
			}
			require.NotNil(t, reviewPhase, "%s should have a review phase", wfID)
			require.NotEmpty(t, reviewPhase.LoopConfig, "%s review phase should have loop_config", wfID)

			// Verify it parses correctly
			cfg, err := db.ParseLoopConfig(reviewPhase.LoopConfig)
			require.NoError(t, err)
			require.NotNil(t, cfg)
			assert.Equal(t, "implement", cfg.LoopToPhase, "%s should loop to implement", wfID)
		})
	}
}
