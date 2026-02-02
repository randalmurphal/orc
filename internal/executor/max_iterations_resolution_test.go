package executor

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
	"github.com/stretchr/testify/assert"
)

// TestResolveMaxIterations_WorkflowDefault tests the max iterations resolution chain:
// 1. phase.max_iterations_override
// 2. workflow.default_max_iterations (NEW)
// 3. phase_template.max_iterations
// 4. 20 (hardcoded fallback)
//
// These tests verify SC-1: Resolution chain priority is respected.
func TestResolveMaxIterations_WorkflowDefault(t *testing.T) {
	t.Run("uses workflow default_max_iterations when no phase override", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_max_iterations
		env.executor.wf = &workflow.Workflow{
			ID:                   "test-workflow",
			DefaultMaxIterations: 50,
		}

		// Phase template with max_iterations=30
		tmpl := &db.PhaseTemplate{
			ID:            "implement",
			MaxIterations: 30,
		}
		// No phase override
		phase := &db.WorkflowPhase{}

		maxIter := env.executor.resolveMaxIterations(tmpl, phase)

		// workflow.default_max_iterations=50 should beat template max_iterations=30
		assert.Equal(t, 50, maxIter)
	})

	t.Run("phase override beats workflow default_max_iterations", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_max_iterations
		env.executor.wf = &workflow.Workflow{
			ID:                   "test-workflow",
			DefaultMaxIterations: 50,
		}

		tmpl := &db.PhaseTemplate{
			ID:            "implement",
			MaxIterations: 30,
		}
		phaseOverride := 15
		phase := &db.WorkflowPhase{
			MaxIterationsOverride: &phaseOverride,
		}

		maxIter := env.executor.resolveMaxIterations(tmpl, phase)

		// phase.max_iterations_override=15 should beat workflow.default_max_iterations=50
		assert.Equal(t, 15, maxIter)
	})

	t.Run("falls through to template when workflow default is zero", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_max_iterations=0 (inherit)
		env.executor.wf = &workflow.Workflow{
			ID:                   "test-workflow",
			DefaultMaxIterations: 0, // 0 means inherit
		}

		tmpl := &db.PhaseTemplate{
			ID:            "implement",
			MaxIterations: 30,
		}
		phase := &db.WorkflowPhase{}

		maxIter := env.executor.resolveMaxIterations(tmpl, phase)

		// Should fall through to template max_iterations=30
		assert.Equal(t, 30, maxIter)
	})

	t.Run("falls back to 20 when all sources are zero", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// Set workflow with default_max_iterations=0
		env.executor.wf = &workflow.Workflow{
			ID:                   "test-workflow",
			DefaultMaxIterations: 0,
		}

		// Template with max_iterations=0 (unset)
		tmpl := &db.PhaseTemplate{
			ID:            "implement",
			MaxIterations: 0,
		}
		phase := &db.WorkflowPhase{}

		maxIter := env.executor.resolveMaxIterations(tmpl, phase)

		// Should fall back to hardcoded default of 20
		assert.Equal(t, 20, maxIter)
	})

	t.Run("nil workflow falls through to template", func(t *testing.T) {
		env := setupTestExecutor(t, nil)

		// No workflow set (nil)
		env.executor.wf = nil

		tmpl := &db.PhaseTemplate{
			ID:            "implement",
			MaxIterations: 35,
		}
		phase := &db.WorkflowPhase{}

		maxIter := env.executor.resolveMaxIterations(tmpl, phase)

		// Should use template max_iterations=35
		assert.Equal(t, 35, maxIter)
	})
}
