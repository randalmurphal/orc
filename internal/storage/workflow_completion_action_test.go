package storage

import (
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowCompletionAction_DatabasePersistence tests SC-6:
// DB schema includes completion_action column and persists it correctly.
func TestWorkflowCompletionAction_DatabasePersistence(t *testing.T) {
	t.Run("saves and loads workflow with completion_action=pr", func(t *testing.T) {
		globalDB := NewTestGlobalDB(t)

		wf := &db.Workflow{
			ID:               "test-wf",
			Name:             "Test",
			CompletionAction: "pr",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err := globalDB.SaveWorkflow(wf)
		require.NoError(t, err)

		loaded, err := globalDB.GetWorkflow("test-wf")
		require.NoError(t, err)
		require.NotNil(t, loaded)

		assert.Equal(t, "pr", loaded.CompletionAction)
	})

	t.Run("saves and loads workflow with completion_action=commit", func(t *testing.T) {
		globalDB := NewTestGlobalDB(t)

		wf := &db.Workflow{
			ID:               "test-wf",
			Name:             "Test",
			CompletionAction: "commit",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err := globalDB.SaveWorkflow(wf)
		require.NoError(t, err)

		loaded, err := globalDB.GetWorkflow("test-wf")
		require.NoError(t, err)
		assert.Equal(t, "commit", loaded.CompletionAction)
	})

	t.Run("saves and loads workflow with completion_action=none", func(t *testing.T) {
		globalDB := NewTestGlobalDB(t)

		wf := &db.Workflow{
			ID:               "test-wf",
			Name:             "Test",
			CompletionAction: "none",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err := globalDB.SaveWorkflow(wf)
		require.NoError(t, err)

		loaded, err := globalDB.GetWorkflow("test-wf")
		require.NoError(t, err)
		assert.Equal(t, "none", loaded.CompletionAction)
	})

	t.Run("saves and loads workflow with empty completion_action (inherit)", func(t *testing.T) {
		globalDB := NewTestGlobalDB(t)

		wf := &db.Workflow{
			ID:               "test-wf",
			Name:             "Test",
			CompletionAction: "", // Inherit from config
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		err := globalDB.SaveWorkflow(wf)
		require.NoError(t, err)

		loaded, err := globalDB.GetWorkflow("test-wf")
		require.NoError(t, err)
		assert.Equal(t, "", loaded.CompletionAction)
	})

	t.Run("updates completion_action on existing workflow", func(t *testing.T) {
		globalDB := NewTestGlobalDB(t)

		// Create initial workflow
		wf := &db.Workflow{
			ID:               "test-wf",
			Name:             "Test",
			CompletionAction: "pr",
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		err := globalDB.SaveWorkflow(wf)
		require.NoError(t, err)

		// Update completion_action
		wf.CompletionAction = "commit"
		err = globalDB.SaveWorkflow(wf)
		require.NoError(t, err)

		loaded, err := globalDB.GetWorkflow("test-wf")
		require.NoError(t, err)
		assert.Equal(t, "commit", loaded.CompletionAction)
	})

	t.Run("lists workflows with completion_action", func(t *testing.T) {
		globalDB := NewTestGlobalDB(t)

		workflows := []*db.Workflow{
			{ID: "wf-1", Name: "W1", CompletionAction: "pr", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "wf-2", Name: "W2", CompletionAction: "commit", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "wf-3", Name: "W3", CompletionAction: "none", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			{ID: "wf-4", Name: "W4", CompletionAction: "", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		}
		for _, wf := range workflows {
			require.NoError(t, globalDB.SaveWorkflow(wf))
		}

		listed, err := globalDB.ListWorkflows()
		require.NoError(t, err)
		require.Len(t, listed, 4)

		// Build a map for easier checking
		actionMap := make(map[string]string)
		for _, wf := range listed {
			actionMap[wf.ID] = wf.CompletionAction
		}

		assert.Equal(t, "pr", actionMap["wf-1"])
		assert.Equal(t, "commit", actionMap["wf-2"])
		assert.Equal(t, "none", actionMap["wf-3"])
		assert.Equal(t, "", actionMap["wf-4"])
	})
}
