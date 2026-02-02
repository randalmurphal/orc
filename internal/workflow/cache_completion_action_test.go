package workflow

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/stretchr/testify/assert"
)

// TestCacheCompletionAction tests SC-3:
// Cache sync (DB conversion) preserves the completion_action field.
func TestCacheCompletionAction(t *testing.T) {
	t.Run("workflowToDBWorkflow preserves completion_action", func(t *testing.T) {
		wf := &Workflow{
			ID:               "test-workflow",
			Name:             "Test Workflow",
			CompletionAction: "pr",
		}

		dbWf := workflowToDBWorkflow(wf, SourceProject)

		assert.Equal(t, "pr", dbWf.CompletionAction)
	})

	t.Run("workflowToDBWorkflow preserves empty completion_action", func(t *testing.T) {
		wf := &Workflow{
			ID:               "test-workflow",
			Name:             "Test Workflow",
			CompletionAction: "", // Inherit
		}

		dbWf := workflowToDBWorkflow(wf, SourceProject)

		assert.Equal(t, "", dbWf.CompletionAction)
	})

	t.Run("workflowToDBWorkflow preserves commit action", func(t *testing.T) {
		wf := &Workflow{
			ID:               "test-workflow",
			Name:             "Test Workflow",
			CompletionAction: "commit",
		}

		dbWf := workflowToDBWorkflow(wf, SourceProject)

		assert.Equal(t, "commit", dbWf.CompletionAction)
	})

	t.Run("workflowToDBWorkflow preserves none action", func(t *testing.T) {
		wf := &Workflow{
			ID:               "test-workflow",
			Name:             "Test Workflow",
			CompletionAction: "none",
		}

		dbWf := workflowToDBWorkflow(wf, SourceProject)

		assert.Equal(t, "none", dbWf.CompletionAction)
	})

	t.Run("DBWorkflowToWorkflow preserves completion_action", func(t *testing.T) {
		dbWf := &db.Workflow{
			ID:               "test-workflow",
			Name:             "Test Workflow",
			CompletionAction: "pr",
		}

		wf := DBWorkflowToWorkflow(dbWf)

		assert.Equal(t, "pr", wf.CompletionAction)
	})

	t.Run("DBWorkflowToWorkflow preserves empty completion_action", func(t *testing.T) {
		dbWf := &db.Workflow{
			ID:               "test-workflow",
			Name:             "Test Workflow",
			CompletionAction: "",
		}

		wf := DBWorkflowToWorkflow(dbWf)

		assert.Equal(t, "", wf.CompletionAction)
	})

	t.Run("roundtrip preserves completion_action", func(t *testing.T) {
		original := &Workflow{
			ID:               "test-workflow",
			Name:             "Test Workflow",
			CompletionAction: "commit",
		}

		// Convert to DB, then back
		dbWf := workflowToDBWorkflow(original, SourceProject)
		restored := DBWorkflowToWorkflow(dbWf)

		assert.Equal(t, original.CompletionAction, restored.CompletionAction)
	})
}
