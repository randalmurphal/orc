package workflow

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/stretchr/testify/assert"
)

// TestCacheDefaultMaxIterations tests SC-3: Cache sync conversions include the field.
func TestCacheDefaultMaxIterations(t *testing.T) {
	t.Run("workflowToDBWorkflow preserves default_max_iterations", func(t *testing.T) {
		wf := &Workflow{
			ID:                   "test-workflow",
			Name:                 "Test Workflow",
			DefaultMaxIterations: 50,
		}

		dbWf := workflowToDBWorkflow(wf, SourceProject)

		assert.Equal(t, 50, dbWf.DefaultMaxIterations)
	})

	t.Run("DBWorkflowToWorkflow preserves default_max_iterations", func(t *testing.T) {
		dbWf := &db.Workflow{
			ID:                   "test-workflow",
			Name:                 "Test Workflow",
			DefaultMaxIterations: 50,
		}

		wf := DBWorkflowToWorkflow(dbWf)

		assert.Equal(t, 50, wf.DefaultMaxIterations)
	})
}
