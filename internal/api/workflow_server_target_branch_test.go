package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateWorkflow_WithTargetBranch tests SC-2:
// CreateWorkflow API request accepts and persists target_branch.
func TestCreateWorkflow_WithTargetBranch(t *testing.T) {
	t.Run("creates workflow with target_branch=develop", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)
		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		req := &orcv1.CreateWorkflowRequest{
			Id:           "test-workflow",
			Name:         "Test Workflow",
			TargetBranch: targetBranchStrPtr("develop"),
		}

		resp, err := server.CreateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Verify response has target_branch
		assert.Equal(t, "develop", *resp.Msg.Workflow.TargetBranch)

		// Verify it was persisted
		dbWf, err := globalDB.GetWorkflow("test-workflow")
		require.NoError(t, err)
		assert.Equal(t, "develop", dbWf.TargetBranch)
	})

	t.Run("creates workflow with feature branch name", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)
		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		req := &orcv1.CreateWorkflowRequest{
			Id:           "feature-workflow",
			Name:         "Feature Workflow",
			TargetBranch: targetBranchStrPtr("feature/auth"),
		}

		resp, err := server.CreateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)
		assert.Equal(t, "feature/auth", *resp.Msg.Workflow.TargetBranch)
	})

	t.Run("creates workflow with empty target_branch (inherit)", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)
		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		req := &orcv1.CreateWorkflowRequest{
			Id:   "inherit-workflow",
			Name: "Inherit Workflow",
			// No target_branch - should inherit from config
		}

		resp, err := server.CreateWorkflow(context.Background(), connect.NewRequest(req))
		require.NoError(t, err)

		// Empty string means inherit
		if resp.Msg.Workflow.TargetBranch != nil {
			assert.Equal(t, "", *resp.Msg.Workflow.TargetBranch)
		}

		// Verify persistence
		dbWf, err := globalDB.GetWorkflow("inherit-workflow")
		require.NoError(t, err)
		assert.Equal(t, "", dbWf.TargetBranch)
	})
}

// TestUpdateWorkflow_TargetBranch tests SC-2:
// UpdateWorkflow API request can modify target_branch.
func TestUpdateWorkflow_TargetBranch(t *testing.T) {
	t.Run("updates target_branch on existing workflow", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)
		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		// Create initial workflow
		createReq := &orcv1.CreateWorkflowRequest{
			Id:           "update-test",
			Name:         "Update Test",
			TargetBranch: targetBranchStrPtr("main"),
		}
		_, err := server.CreateWorkflow(context.Background(), connect.NewRequest(createReq))
		require.NoError(t, err)

		// Update target_branch
		updateReq := &orcv1.UpdateWorkflowRequest{
			Id:           "update-test",
			Name:         targetBranchStrPtr("Update Test"),
			TargetBranch: targetBranchStrPtr("develop"),
		}

		resp, err := server.UpdateWorkflow(context.Background(), connect.NewRequest(updateReq))
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Verify response
		assert.Equal(t, "develop", *resp.Msg.Workflow.TargetBranch)

		// Verify persistence
		dbWf, err := globalDB.GetWorkflow("update-test")
		require.NoError(t, err)
		assert.Equal(t, "develop", dbWf.TargetBranch)
	})

	t.Run("can clear target_branch by setting empty string", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)
		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		// Create with target_branch
		createReq := &orcv1.CreateWorkflowRequest{
			Id:           "clear-test",
			Name:         "Clear Test",
			TargetBranch: targetBranchStrPtr("develop"),
		}
		_, err := server.CreateWorkflow(context.Background(), connect.NewRequest(createReq))
		require.NoError(t, err)

		// Clear target_branch
		updateReq := &orcv1.UpdateWorkflowRequest{
			Id:           "clear-test",
			Name:         targetBranchStrPtr("Clear Test"),
			TargetBranch: targetBranchStrPtr(""),
		}

		resp, err := server.UpdateWorkflow(context.Background(), connect.NewRequest(updateReq))
		require.NoError(t, err)

		// Verify cleared
		if resp.Msg.Workflow.TargetBranch != nil {
			assert.Equal(t, "", *resp.Msg.Workflow.TargetBranch)
		}

		// Verify persistence
		dbWf, err := globalDB.GetWorkflow("clear-test")
		require.NoError(t, err)
		assert.Equal(t, "", dbWf.TargetBranch)
	})
}

// TestGetWorkflow_TargetBranch tests SC-2:
// GetWorkflow API returns target_branch.
func TestGetWorkflow_TargetBranch(t *testing.T) {
	t.Run("returns target_branch in response", func(t *testing.T) {
		globalDB := storage.NewTestGlobalDB(t)
		server := NewWorkflowServer(nil, globalDB, nil, nil, nil, nil)

		// Create workflow
		createReq := &orcv1.CreateWorkflowRequest{
			Id:           "get-test",
			Name:         "Get Test",
			TargetBranch: targetBranchStrPtr("release/v2"),
		}
		_, err := server.CreateWorkflow(context.Background(), connect.NewRequest(createReq))
		require.NoError(t, err)

		// Get workflow
		getReq := &orcv1.GetWorkflowRequest{
			Id: "get-test",
		}

		resp, err := server.GetWorkflow(context.Background(), connect.NewRequest(getReq))
		require.NoError(t, err)
		require.NotNil(t, resp)

		// WorkflowWithDetails has a nested Workflow message
		assert.Equal(t, "release/v2", *resp.Msg.Workflow.Workflow.TargetBranch)
	})
}

func targetBranchStrPtr(s string) *string {
	return &s
}
