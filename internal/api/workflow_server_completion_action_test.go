package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkflowCompletionAction_CreateWorkflow tests SC-4:
// CreateWorkflow API request accepts completion_action.
func TestWorkflowCompletionAction_CreateWorkflow(t *testing.T) {
	t.Run("creates workflow with completion_action=pr", func(t *testing.T) {
		// Setup test database
		globalDB := storage.NewTestGlobalDB(t)

		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		// Create workflow with completion_action
		req := &orcv1.CreateWorkflowRequest{
			Id:               "test-workflow",
			Name:             "Test Workflow",
			CompletionAction: completionActionStrPtr("pr"),
		}

		resp, err := server.CreateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Verify response has completion_action
		assert.Equal(t, "pr", *resp.Msg.Workflow.CompletionAction)

		// Verify it was persisted
		dbWf, err := globalDB.GetWorkflow("test-workflow")
		require.NoError(t, err)
		assert.Equal(t, "pr", dbWf.CompletionAction)
	})

	t.Run("creates workflow with completion_action=commit", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)
		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		req := &orcv1.CreateWorkflowRequest{
			Id:               "commit-workflow",
			Name:             "Commit Only",
			CompletionAction: completionActionStrPtr("commit"),
		}

		resp, err := server.CreateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)

		assert.Equal(t, "commit", *resp.Msg.Workflow.CompletionAction)
	})

	t.Run("creates workflow with completion_action=none", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)
		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		req := &orcv1.CreateWorkflowRequest{
			Id:               "no-action-workflow",
			Name:             "No Action",
			CompletionAction: completionActionStrPtr("none"),
		}

		resp, err := server.CreateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)

		assert.Equal(t, "none", *resp.Msg.Workflow.CompletionAction)
	})

	t.Run("creates workflow with empty completion_action (inherit)", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)
		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		req := &orcv1.CreateWorkflowRequest{
			Id:   "inherit-workflow",
			Name: "Inherit Action",
			// No CompletionAction set - should default to empty (inherit)
		}

		resp, err := server.CreateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)

		// Empty or nil means inherit from config
		if resp.Msg.Workflow.CompletionAction != nil {
			assert.Equal(t, "", *resp.Msg.Workflow.CompletionAction)
		}

		// Verify persisted as empty
		dbWf, err := globalDB.GetWorkflow("inherit-workflow")
		require.NoError(t, err)
		assert.Equal(t, "", dbWf.CompletionAction)
	})
}

// TestWorkflowCompletionAction_UpdateWorkflow tests SC-5:
// UpdateWorkflow API request accepts completion_action.
func TestWorkflowCompletionAction_UpdateWorkflow(t *testing.T) {
	t.Run("updates workflow completion_action from empty to pr", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)

		// Create initial workflow
		err := globalDB.SaveWorkflow(&db.Workflow{
			ID:               "test-workflow",
			Name:             "Test",
			CompletionAction: "", // Initially inherit
		})
		require.NoError(t, err)

		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		// Update to pr
		req := &orcv1.UpdateWorkflowRequest{
			Id:               "test-workflow",
			CompletionAction: completionActionStrPtr("pr"),
		}

		resp, err := server.UpdateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)
		require.NotNil(t, resp)

		assert.Equal(t, "pr", *resp.Msg.Workflow.CompletionAction)

		// Verify it was persisted
		dbWf, err := globalDB.GetWorkflow("test-workflow")
		require.NoError(t, err)
		assert.Equal(t, "pr", dbWf.CompletionAction)
	})

	t.Run("updates workflow completion_action to commit", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)

		err := globalDB.SaveWorkflow(&db.Workflow{
			ID:               "test-workflow",
			Name:             "Test",
			CompletionAction: "pr",
		})
		require.NoError(t, err)

		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		req := &orcv1.UpdateWorkflowRequest{
			Id:               "test-workflow",
			CompletionAction: completionActionStrPtr("commit"),
		}

		resp, err := server.UpdateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)

		assert.Equal(t, "commit", *resp.Msg.Workflow.CompletionAction)
	})

	t.Run("updates workflow completion_action to none", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)

		err := globalDB.SaveWorkflow(&db.Workflow{
			ID:               "test-workflow",
			Name:             "Test",
			CompletionAction: "pr",
		})
		require.NoError(t, err)

		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		req := &orcv1.UpdateWorkflowRequest{
			Id:               "test-workflow",
			CompletionAction: completionActionStrPtr("none"),
		}

		resp, err := server.UpdateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)

		assert.Equal(t, "none", *resp.Msg.Workflow.CompletionAction)
	})

	t.Run("updates workflow completion_action to inherit (empty)", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)

		err := globalDB.SaveWorkflow(&db.Workflow{
			ID:               "test-workflow",
			Name:             "Test",
			CompletionAction: "pr", // Currently set to pr
		})
		require.NoError(t, err)

		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		// Update to empty (inherit)
		req := &orcv1.UpdateWorkflowRequest{
			Id:               "test-workflow",
			CompletionAction: completionActionStrPtr(""), // Empty means inherit
		}

		resp, err := server.UpdateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)

		// Should be empty after update
		if resp.Msg.Workflow.CompletionAction != nil {
			assert.Equal(t, "", *resp.Msg.Workflow.CompletionAction)
		}

		dbWf, err := globalDB.GetWorkflow("test-workflow")
		require.NoError(t, err)
		assert.Equal(t, "", dbWf.CompletionAction)
	})

	t.Run("preserves completion_action when not in update request", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)

		err := globalDB.SaveWorkflow(&db.Workflow{
			ID:               "test-workflow",
			Name:             "Test",
			CompletionAction: "commit",
		})
		require.NoError(t, err)

		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		// Update name only, not completion_action
		newName := "Updated Name"
		req := &orcv1.UpdateWorkflowRequest{
			Id:   "test-workflow",
			Name: &newName,
			// No CompletionAction in request - should preserve existing
		}

		resp, err := server.UpdateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)

		// Should still be "commit"
		assert.Equal(t, "commit", *resp.Msg.Workflow.CompletionAction)
	})
}

// TestWorkflowCompletionAction_GetWorkflow tests that GetWorkflow returns completion_action.
func TestWorkflowCompletionAction_GetWorkflow(t *testing.T) {
	t.Run("returns completion_action in response", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)

		err := globalDB.SaveWorkflow(&db.Workflow{
			ID:               "test-workflow",
			Name:             "Test",
			CompletionAction: "pr",
		})
		require.NoError(t, err)

		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		req := &orcv1.GetWorkflowRequest{
			Id: "test-workflow",
		}

		resp, err := server.GetWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)

		assert.Equal(t, "pr", *resp.Msg.Workflow.Workflow.CompletionAction)
	})
}

// TestWorkflowCompletionAction_ListWorkflows tests that ListWorkflows returns completion_action.
func TestWorkflowCompletionAction_ListWorkflows(t *testing.T) {
	t.Run("returns completion_action for all workflows", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)

		workflows := []*db.Workflow{
			{ID: "wf-1", Name: "Workflow 1", CompletionAction: "pr"},
			{ID: "wf-2", Name: "Workflow 2", CompletionAction: "commit"},
			{ID: "wf-3", Name: "Workflow 3", CompletionAction: "none"},
			{ID: "wf-4", Name: "Workflow 4", CompletionAction: ""},
		}
		for _, wf := range workflows {
			require.NoError(t, globalDB.SaveWorkflow(wf))
		}

		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		resp, err := server.ListWorkflows(context.Background(), connect.NewRequest(&orcv1.ListWorkflowsRequest{}))
		require.NoError(t, err)
		require.Len(t, resp.Msg.Workflows, 4)

		// Build a map for easier checking
		actionMap := make(map[string]string)
		for _, wf := range resp.Msg.Workflows {
			action := ""
			if wf.CompletionAction != nil {
				action = *wf.CompletionAction
			}
			actionMap[wf.Id] = action
		}

		assert.Equal(t, "pr", actionMap["wf-1"])
		assert.Equal(t, "commit", actionMap["wf-2"])
		assert.Equal(t, "none", actionMap["wf-3"])
		assert.Equal(t, "", actionMap["wf-4"])
	})
}

// completionActionStrPtr is a helper for creating string pointers.
// Named differently from strPtr in task_server_workflow_test.go to avoid redeclaration.
func completionActionStrPtr(s string) *string {
	return &s
}
