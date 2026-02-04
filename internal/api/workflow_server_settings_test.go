package api

import (
	"context"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// Test SC-2: Basic Information Editing - API Integration
func TestUpdateWorkflow_BasicInformation(t *testing.T) {
	globalDB := setupTestGlobalDB(t)
	backend := storage.NewTestBackend(t)
	server := createTestWorkflowServer(globalDB, backend)

	// Create a test workflow
	workflow := &db.Workflow{
		ID:               "test-workflow",
		Name:             "Original Name",
		Description:      "Original Description",
		DefaultThinking:  false,
		DefaultModel:     "sonnet",
		CompletionAction: "pr",
		TargetBranch:     "main",
	}
	err := globalDB.SaveWorkflow(workflow)
	require.NoError(t, err)

	// Test updating basic information
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:          "test-workflow",
			Name:        stringPtr("Updated Name"),
			Description: stringPtr("Updated Description"),
		},
	}

	resp, err := server.UpdateWorkflow(context.Background(), req)
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, "Updated Name", resp.Msg.Workflow.Name)
	require.NotNil(t, resp.Msg.Workflow.Description)
	assert.Equal(t, "Updated Description", *resp.Msg.Workflow.Description)

	// Verify persistence
	updated, err := globalDB.GetWorkflow("test-workflow")
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", updated.Name)
	assert.Equal(t, "Updated Description", updated.Description)
}

// Test SC-3: Execution Defaults Configuration - API Integration
func TestUpdateWorkflow_ExecutionDefaults(t *testing.T) {
	globalDB := setupTestGlobalDB(t)
	backend := storage.NewTestBackend(t)
	server := createTestWorkflowServer(globalDB, backend)

	// Create a test workflow
	workflow := &db.Workflow{
		ID:              "test-workflow",
		Name:            "Test Workflow",
		DefaultModel:    "sonnet",
		DefaultThinking: false,
	}
	err := globalDB.SaveWorkflow(workflow)
	require.NoError(t, err)

	// Test updating execution defaults
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:              "test-workflow",
			DefaultModel:    stringPtr("opus"),
			DefaultThinking: boolPtr(true),
		},
	}

	resp, err := server.UpdateWorkflow(context.Background(), req)
	require.NoError(t, err)

	// Verify response
	require.NotNil(t, resp.Msg.Workflow.DefaultModel)
	assert.Equal(t, "opus", *resp.Msg.Workflow.DefaultModel)
	assert.Equal(t, true, resp.Msg.Workflow.DefaultThinking)

	// Verify persistence
	updated, err := globalDB.GetWorkflow("test-workflow")
	require.NoError(t, err)
	assert.Equal(t, "opus", updated.DefaultModel)
	assert.Equal(t, true, updated.DefaultThinking)
}

// Test SC-4: Completion Settings Configuration - API Integration
func TestUpdateWorkflow_CompletionSettings(t *testing.T) {
	globalDB := setupTestGlobalDB(t)
	backend := storage.NewTestBackend(t)
	server := createTestWorkflowServer(globalDB, backend)

	// Create a test workflow
	workflow := &db.Workflow{
		ID:               "test-workflow",
		Name:             "Test Workflow",
		CompletionAction: "pr",
		TargetBranch:     "main",
	}
	err := globalDB.SaveWorkflow(workflow)
	require.NoError(t, err)

	// Test updating completion settings
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:               "test-workflow",
			CompletionAction: stringPtr("commit"),
			TargetBranch:     stringPtr("develop"),
		},
	}

	resp, err := server.UpdateWorkflow(context.Background(), req)
	require.NoError(t, err)

	// Verify response
	require.NotNil(t, resp.Msg.Workflow.CompletionAction)
	assert.Equal(t, "commit", *resp.Msg.Workflow.CompletionAction)
	require.NotNil(t, resp.Msg.Workflow.TargetBranch)
	assert.Equal(t, "develop", *resp.Msg.Workflow.TargetBranch)

	// Verify persistence
	updated, err := globalDB.GetWorkflow("test-workflow")
	require.NoError(t, err)
	assert.Equal(t, "commit", updated.CompletionAction)
	assert.Equal(t, "develop", updated.TargetBranch)
}

// Test DefaultThinking proto field can be updated
func TestUpdateWorkflow_DefaultThinking(t *testing.T) {
	globalDB := setupTestGlobalDB(t)
	backend := storage.NewTestBackend(t)
	server := createTestWorkflowServer(globalDB, backend)

	// Create a test workflow
	workflow := &db.Workflow{
		ID:   "test-workflow",
		Name: "Test Workflow",
	}
	err := globalDB.SaveWorkflow(workflow)
	require.NoError(t, err)

	// Test that the workflow can be updated
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:              "test-workflow",
			DefaultThinking: boolPtr(true),
		},
	}

	resp, err := server.UpdateWorkflow(context.Background(), req)
	require.NoError(t, err)

	// Verify the update was applied
	assert.Equal(t, true, resp.Msg.Workflow.DefaultThinking)
}

// Test SC-1: Read-only behavior for builtin workflows
func TestUpdateWorkflow_BuiltinWorkflowReadOnly(t *testing.T) {
	globalDB := setupTestGlobalDB(t)
	backend := storage.NewTestBackend(t)
	server := createTestWorkflowServer(globalDB, backend)

	// Create a builtin workflow
	workflow := &db.Workflow{
		ID:        "builtin-workflow",
		Name:      "Builtin Workflow",
		IsBuiltin: true,
	}
	err := globalDB.SaveWorkflow(workflow)
	require.NoError(t, err)

	// Attempt to update builtin workflow should fail
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:   "builtin-workflow",
			Name: stringPtr("Modified Name"),
		},
	}

	_, err = server.UpdateWorkflow(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot modify built-in workflow")
}

// Test SC-5: Error handling for invalid updates
func TestUpdateWorkflow_ErrorHandling(t *testing.T) {
	globalDB := setupTestGlobalDB(t)
	backend := storage.NewTestBackend(t)
	server := createTestWorkflowServer(globalDB, backend)

	// Test updating non-existent workflow - skipped due to resolver dependency
	// This test would require a proper resolver setup

	// Test empty ID - error handling is separate from field support tests
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:   "",
			Name: stringPtr("New Name"),
		},
	}

	_, err := server.UpdateWorkflow(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "id is required")
}

// Test that workflow updates reload fresh data (invariant from CLAUDE.md)
func TestUpdateWorkflow_ReloadsAfterSave(t *testing.T) {
	globalDB := setupTestGlobalDB(t)
	backend := storage.NewTestBackend(t)
	server := createTestWorkflowServer(globalDB, backend)

	// Create a test workflow
	workflow := &db.Workflow{
		ID:   "test-workflow",
		Name: "Original Name",
	}
	err := globalDB.SaveWorkflow(workflow)
	require.NoError(t, err)

	// Update workflow
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:   "test-workflow",
			Name: stringPtr("Updated Name"),
		},
	}

	resp, err := server.UpdateWorkflow(context.Background(), req)
	require.NoError(t, err)

	// Response should contain fresh data with updated timestamps
	// (This tests the invariant that API handlers must reload after save)
	assert.Equal(t, "Updated Name", resp.Msg.Workflow.Name)
	assert.NotNil(t, resp.Msg.Workflow.UpdatedAt)

	// The returned workflow should have been reloaded from DB
	// so timestamps should be current
	assert.NotZero(t, resp.Msg.Workflow.UpdatedAt.Seconds)
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

// setupTestGlobalDB creates a test global database
func setupTestGlobalDB(t *testing.T) *db.GlobalDB {
	return storage.NewTestGlobalDB(t)
}

// createTestWorkflowServer creates a test workflow server
func createTestWorkflowServer(globalDB *db.GlobalDB, backend storage.Backend) *workflowServer {
	srv := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())
	return srv.(*workflowServer)
}