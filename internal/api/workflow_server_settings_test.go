package api

import (
	"context"
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
		DefaultModel:     "claude-sonnet-3-5",
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
	assert.Equal(t, "Updated Description", resp.Msg.Workflow.Description)

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
		ID:                   "test-workflow",
		Name:                 "Test Workflow",
		DefaultModel:         "claude-sonnet-3-5",
		DefaultThinking:      false,
		DefaultMaxIterations: 20,
	}
	err := globalDB.SaveWorkflow(workflow)
	require.NoError(t, err)

	// Test updating execution defaults
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:                   "test-workflow",
			DefaultModel:         stringPtr("claude-opus-3"),
			DefaultThinking:      boolPtr(true),
			DefaultMaxIterations: int32Ptr(30),
		},
	}

	resp, err := server.UpdateWorkflow(context.Background(), req)
	require.NoError(t, err)

	// Verify response
	assert.Equal(t, "claude-opus-3", resp.Msg.Workflow.DefaultModel)
	assert.Equal(t, true, resp.Msg.Workflow.DefaultThinking)
	assert.Equal(t, int32(30), resp.Msg.Workflow.DefaultMaxIterations)

	// Verify persistence
	updated, err := globalDB.GetWorkflow("test-workflow")
	require.NoError(t, err)
	assert.Equal(t, "claude-opus-3", updated.DefaultModel)
	assert.Equal(t, true, updated.DefaultThinking)
	assert.Equal(t, 30, updated.DefaultMaxIterations)
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
	assert.Equal(t, "commit", resp.Msg.Workflow.CompletionAction)
	assert.Equal(t, "develop", resp.Msg.Workflow.TargetBranch)

	// Verify persistence
	updated, err := globalDB.GetWorkflow("test-workflow")
	require.NoError(t, err)
	assert.Equal(t, "commit", updated.CompletionAction)
	assert.Equal(t, "develop", updated.TargetBranch)
}

// Test default_max_iterations proto field exists and is properly handled
// This tests that the missing field from workflow.proto is added
func TestUpdateWorkflow_DefaultMaxIterations_ProtoFieldExists(t *testing.T) {
	globalDB := setupTestGlobalDB(t)
	backend := storage.NewTestBackend(t)
	server := createTestWorkflowServer(globalDB, backend)

	// Create a test workflow
	workflow := &db.Workflow{
		ID:                   "test-workflow",
		Name:                 "Test Workflow",
		DefaultMaxIterations: 0, // Default value
	}
	err := globalDB.SaveWorkflow(workflow)
	require.NoError(t, err)

	// Test that the proto field DefaultMaxIterations exists and can be set
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:                   "test-workflow",
			DefaultMaxIterations: int32Ptr(25),
		},
	}

	resp, err := server.UpdateWorkflow(context.Background(), req)
	require.NoError(t, err)

	// This test will fail if default_max_iterations field is missing from the proto
	// or if the UpdateWorkflow method doesn't handle it
	assert.Equal(t, int32(25), resp.Msg.Workflow.DefaultMaxIterations)
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

	// Test updating non-existent workflow
	req := &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:   "non-existent",
			Name: stringPtr("New Name"),
		},
	}

	_, err := server.UpdateWorkflow(context.Background(), req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Test empty ID
	req = &connect.Request[orcv1.UpdateWorkflowRequest]{
		Msg: &orcv1.UpdateWorkflowRequest{
			Id:   "",
			Name: stringPtr("New Name"),
		},
	}

	_, err = server.UpdateWorkflow(context.Background(), req)
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
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}

func int32Ptr(i int32) *int32 {
	return &i
}

// setupTestGlobalDB creates a test global database
func setupTestGlobalDB(t *testing.T) *db.GlobalDB {
	// This function should exist in the test setup - if not, it needs to be implemented
	// based on the existing test patterns in the codebase
	globalDB, err := db.NewGlobalDB(":memory:", nil)
	require.NoError(t, err)
	return globalDB
}

// createTestWorkflowServer creates a test workflow server
func createTestWorkflowServer(globalDB *db.GlobalDB, backend storage.Backend) *workflowServer {
	return &workflowServer{
		backend:  backend,
		globalDB: globalDB,
		// Other dependencies can be nil for these specific tests
		resolver: nil,
		cloner:   nil,
		cache:    nil,
		logger:   nil,
	}
}