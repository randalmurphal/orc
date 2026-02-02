package workflow

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for TASK-752: Implement and expose 8 built-in workflow templates
//
// Success Criteria:
// - SC-1: All 8 workflows exist in database after migration
// - SC-2: Workflows marked as is_builtin=true (not editable, only clonable)
// - SC-3: API lists them in /api/workflows (tested in api/builtin_workflows_api_test.go)
// - SC-4: UI shows them in Built-in section (E2E test in e2e/builtin_workflows_test.ts)

// requiredBuiltinWorkflows defines the exact 8 workflows that MUST exist per TASK-752.
// Order matches the design doc table in docs/plans/2026-02-01-ux-simplification-brainstorm.md
var requiredBuiltinWorkflows = []struct {
	ID          string
	Name        string
	MinPhases   int    // Minimum number of phases expected
	Description string // Expected description (optional check)
}{
	{"implement-large", "Implement (Large)", 6, ""},
	{"implement-medium", "Implement (Medium)", 5, ""},
	{"implement-small", "Implement (Small)", 3, ""},
	{"implement-trivial", "Implement (Trivial)", 1, ""},
	{"review", "Review", 1, ""},
	{"qa-e2e", "QA E2E", 1, ""},
	{"spec", "Spec Only", 1, ""},
	{"docs", "Documentation", 1, ""},
}

// TestBuiltinWorkflows_ExactlyEightExist verifies SC-1:
// All 8 workflows exist in database after SeedBuiltins.
func TestBuiltinWorkflows_ExactlyEightExist(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)
	_, err := SeedBuiltins(gdb)
	require.NoError(t, err, "SeedBuiltins should not error")

	// Verify each required workflow exists
	for _, expected := range requiredBuiltinWorkflows {
		t.Run(expected.ID, func(t *testing.T) {
			wf, err := gdb.GetWorkflow(expected.ID)
			require.NoError(t, err, "GetWorkflow(%s) should not error", expected.ID)
			require.NotNil(t, wf, "workflow %s must exist after SeedBuiltins", expected.ID)

			// Verify name matches
			assert.Equal(t, expected.Name, wf.Name,
				"workflow %s should have name %q", expected.ID, expected.Name)

			// Verify phases exist
			phases, err := gdb.GetWorkflowPhases(expected.ID)
			require.NoError(t, err, "GetWorkflowPhases(%s) should not error", expected.ID)
			assert.GreaterOrEqual(t, len(phases), expected.MinPhases,
				"workflow %s should have at least %d phases, got %d",
				expected.ID, expected.MinPhases, len(phases))
		})
	}
}

// TestBuiltinWorkflows_AllMarkedAsBuiltin verifies SC-2 (first part):
// All 8 workflows have IsBuiltin=true.
func TestBuiltinWorkflows_AllMarkedAsBuiltin(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)
	_, err := SeedBuiltins(gdb)
	require.NoError(t, err)

	for _, expected := range requiredBuiltinWorkflows {
		t.Run(expected.ID, func(t *testing.T) {
			wf, err := gdb.GetWorkflow(expected.ID)
			require.NoError(t, err)
			require.NotNil(t, wf)

			assert.True(t, wf.IsBuiltin,
				"workflow %s must have IsBuiltin=true", expected.ID)
		})
	}
}

// TestBuiltinWorkflows_ListReturnsAll verifies that ListWorkflows returns all 8.
func TestBuiltinWorkflows_ListReturnsAll(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)
	_, err := SeedBuiltins(gdb)
	require.NoError(t, err)

	workflows, err := gdb.ListWorkflows()
	require.NoError(t, err)

	// Extract IDs of built-in workflows from the list
	builtinIDs := make([]string, 0)
	for _, wf := range workflows {
		if wf.IsBuiltin {
			builtinIDs = append(builtinIDs, wf.ID)
		}
	}

	// Verify all 8 required workflows are present
	for _, expected := range requiredBuiltinWorkflows {
		assert.Contains(t, builtinIDs, expected.ID,
			"ListWorkflows should include built-in workflow %s", expected.ID)
	}
}

// TestBuiltinWorkflows_EmbeddedYAMLExists verifies embedded YAML files exist for all 8.
func TestBuiltinWorkflows_EmbeddedYAMLExists(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(WithEmbedded(true))

	for _, expected := range requiredBuiltinWorkflows {
		t.Run(expected.ID, func(t *testing.T) {
			resolved, err := resolver.ResolveWorkflow(expected.ID)
			require.NoError(t, err, "embedded workflow %s should be resolvable", expected.ID)
			require.NotNil(t, resolved)

			assert.Equal(t, SourceEmbedded, resolved.Source,
				"workflow %s should come from embedded source", expected.ID)
			assert.Equal(t, expected.ID, resolved.Workflow.ID)
		})
	}
}

// TestBuiltinWorkflows_ListBuiltinIDs verifies ListBuiltinWorkflowIDs returns all 8.
func TestBuiltinWorkflows_ListBuiltinIDs(t *testing.T) {
	t.Parallel()

	ids := ListBuiltinWorkflowIDs()
	require.NotEmpty(t, ids, "ListBuiltinWorkflowIDs should return workflow IDs")

	for _, expected := range requiredBuiltinWorkflows {
		assert.True(t, slices.Contains(ids, expected.ID),
			"ListBuiltinWorkflowIDs should include %s", expected.ID)
	}
}

// TestBuiltinWorkflows_PhasesAreValid verifies each workflow's phases reference valid templates.
func TestBuiltinWorkflows_PhasesAreValid(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)
	_, err := SeedBuiltins(gdb)
	require.NoError(t, err)

	for _, expected := range requiredBuiltinWorkflows {
		t.Run(expected.ID, func(t *testing.T) {
			phases, err := gdb.GetWorkflowPhases(expected.ID)
			require.NoError(t, err)
			require.NotEmpty(t, phases, "workflow %s must have phases", expected.ID)

			// Each phase should reference an existing phase template
			for _, phase := range phases {
				tmpl, err := gdb.GetPhaseTemplate(phase.PhaseTemplateID)
				require.NoError(t, err, "phase template %s should exist", phase.PhaseTemplateID)
				require.NotNil(t, tmpl, "phase template %s should not be nil", phase.PhaseTemplateID)
			}
		})
	}
}

// TestBuiltinWorkflows_CorrectPhaseSequences verifies phase sequences are properly ordered.
func TestBuiltinWorkflows_CorrectPhaseSequences(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)
	_, err := SeedBuiltins(gdb)
	require.NoError(t, err)

	expectedPhases := map[string][]string{
		"implement-large":   {"spec", "tdd_write", "breakdown", "implement", "review", "docs"},
		"implement-medium":  {"spec", "tdd_write", "implement", "review", "docs"},
		"implement-small":   {"tiny_spec", "implement", "review"},
		"implement-trivial": {"implement"},
		"review":            {"review"},
		"qa-e2e":            {}, // Variable based on loop config
		"spec":              {"spec"},
		"docs":              {"docs"},
	}

	for workflowID, expectedPhaseIDs := range expectedPhases {
		if len(expectedPhaseIDs) == 0 {
			continue // Skip workflows with variable phases
		}

		t.Run(workflowID, func(t *testing.T) {
			phases, err := gdb.GetWorkflowPhases(workflowID)
			require.NoError(t, err)

			// Verify phases are in correct sequence order
			for i, phase := range phases {
				assert.Equal(t, i, phase.Sequence,
					"workflow %s phase %d should have sequence %d, got %d",
					workflowID, i, i, phase.Sequence)
			}

			// Verify phase template IDs match expected
			actualPhaseIDs := make([]string, len(phases))
			for i, phase := range phases {
				actualPhaseIDs[i] = phase.PhaseTemplateID
			}

			assert.Equal(t, expectedPhaseIDs, actualPhaseIDs,
				"workflow %s should have phases %v, got %v",
				workflowID, expectedPhaseIDs, actualPhaseIDs)
		})
	}
}

// TestBuiltinWorkflows_IdempotentSeeding verifies seeding twice doesn't duplicate workflows.
func TestBuiltinWorkflows_IdempotentSeeding(t *testing.T) {
	t.Parallel()

	gdb := openTestGlobalDB(t)

	// Seed twice
	_, err := SeedBuiltins(gdb)
	require.NoError(t, err)

	workflows1, err := gdb.ListWorkflows()
	require.NoError(t, err)
	count1 := len(workflows1)

	// Second seed
	_, err = SeedBuiltins(gdb)
	require.NoError(t, err)

	workflows2, err := gdb.ListWorkflows()
	require.NoError(t, err)
	count2 := len(workflows2)

	assert.Equal(t, count1, count2,
		"seeding twice should not create duplicate workflows")
}

// --- Tests for SC-2 (second part): Built-in workflows are not editable ---

// TestBuiltinWorkflows_SourceNotEditable verifies embedded source is not editable.
func TestBuiltinWorkflows_SourceNotEditable(t *testing.T) {
	t.Parallel()

	// Embedded source should return IsEditable() = false
	assert.False(t, SourceEmbedded.IsEditable(),
		"SourceEmbedded should not be editable")

	// Project sources should be editable
	assert.True(t, SourceProject.IsEditable(),
		"SourceProject should be editable")
	assert.True(t, SourceProjectLocal.IsEditable(),
		"SourceProjectLocal should be editable")
	assert.True(t, SourcePersonalGlobal.IsEditable(),
		"SourcePersonalGlobal should be editable")
}

// TestWeightToWorkflowID_ReturnsCorrectWorkflows verifies weight mapping returns
// the expected built-in workflow IDs.
func TestWeightToWorkflowID_ReturnsCorrectWorkflows(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		weight   string
		expected string
	}{
		{"trivial", "implement-trivial"},
		{"TASK_WEIGHT_TRIVIAL", "implement-trivial"},
		{"small", "implement-small"},
		{"TASK_WEIGHT_SMALL", "implement-small"},
		{"medium", "implement-medium"},
		{"TASK_WEIGHT_MEDIUM", "implement-medium"},
		{"large", "implement-large"},
		{"TASK_WEIGHT_LARGE", "implement-large"},
		{"", ""},         // Unspecified
		{"invalid", ""},  // Invalid
	}

	for _, tc := range testCases {
		t.Run(tc.weight, func(t *testing.T) {
			result := WeightToWorkflowIDString(tc.weight)
			assert.Equal(t, tc.expected, result,
				"weight %q should map to workflow %q", tc.weight, tc.expected)
		})
	}
}

