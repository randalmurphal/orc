package api

import (
	"context"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/workflow"
)

// Tests for TASK-752 SC-3: API lists built-in workflows in /api/workflows
//
// These tests verify:
// - ListWorkflows returns all 8 required built-in workflows
// - Built-in workflows have IsBuiltin=true in API response
// - Built-in workflows cannot be modified via UpdateWorkflow (PermissionDenied)
// - Built-in workflows can be cloned

// requiredBuiltinWorkflowIDs defines the exact 8 workflows per TASK-752 spec.
var requiredBuiltinWorkflowIDs = []string{
	"implement-large",
	"implement-medium",
	"implement-small",
	"implement-trivial",
	"review",
	"qa-e2e",
	"spec",
	"docs",
}

// TestListWorkflows_ReturnsAllBuiltinWorkflows verifies SC-3:
// API lists all 8 built-in workflows.
func TestListWorkflows_ReturnsAllBuiltinWorkflows(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed built-in workflows
	_, err := workflow.SeedBuiltins(globalDB)
	require.NoError(t, err, "SeedBuiltins should not error")

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.ListWorkflowsRequest{})
	resp, err := server.ListWorkflows(context.Background(), req)
	require.NoError(t, err, "ListWorkflows should not error")

	// Extract workflow IDs from response
	workflowIDs := make(map[string]bool)
	for _, wf := range resp.Msg.Workflows {
		workflowIDs[wf.Id] = true
	}

	// Verify all 8 required workflows are present
	for _, expectedID := range requiredBuiltinWorkflowIDs {
		assert.True(t, workflowIDs[expectedID],
			"ListWorkflows should include built-in workflow %s", expectedID)
	}
}

// TestListWorkflows_BuiltinWorkflowsHaveIsBuiltinTrue verifies SC-3:
// Built-in workflows have IsBuiltin=true in API response.
func TestListWorkflows_BuiltinWorkflowsHaveIsBuiltinTrue(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed built-in workflows
	_, err := workflow.SeedBuiltins(globalDB)
	require.NoError(t, err)

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.ListWorkflowsRequest{})
	resp, err := server.ListWorkflows(context.Background(), req)
	require.NoError(t, err)

	// Verify each required workflow has IsBuiltin=true
	workflowMap := make(map[string]*orcv1.Workflow)
	for _, wf := range resp.Msg.Workflows {
		workflowMap[wf.Id] = wf
	}

	for _, expectedID := range requiredBuiltinWorkflowIDs {
		wf, exists := workflowMap[expectedID]
		require.True(t, exists, "workflow %s should exist in response", expectedID)
		assert.True(t, wf.IsBuiltin,
			"workflow %s should have IsBuiltin=true in API response", expectedID)
	}
}

// TestListWorkflows_IncludesPhaseCounts verifies API response includes phase counts.
func TestListWorkflows_IncludesPhaseCounts(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed built-in workflows
	_, err := workflow.SeedBuiltins(globalDB)
	require.NoError(t, err)

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.ListWorkflowsRequest{})
	resp, err := server.ListWorkflows(context.Background(), req)
	require.NoError(t, err)

	expectedMinPhases := map[string]int32{
		"implement-large":   6,
		"implement-medium":  5,
		"implement-small":   3,
		"implement-trivial": 1,
		"review":            1,
		"qa-e2e":            1,
		"spec":              1,
		"docs":              1,
	}

	for workflowID, minPhases := range expectedMinPhases {
		count, exists := resp.Msg.PhaseCounts[workflowID]
		if !exists {
			t.Errorf("PhaseCounts should include workflow %s", workflowID)
			continue
		}
		assert.GreaterOrEqual(t, count, minPhases,
			"workflow %s should have at least %d phases, got %d",
			workflowID, minPhases, count)
	}
}

// TestUpdateWorkflow_BuiltinWorkflow_ReturnsPermissionDenied verifies SC-2:
// Built-in workflows cannot be modified.
func TestUpdateWorkflow_BuiltinWorkflow_ReturnsPermissionDenied(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed built-in workflows
	_, err := workflow.SeedBuiltins(globalDB)
	require.NoError(t, err)

	// Create resolver that returns embedded source for built-ins
	resolver := workflow.NewResolver(workflow.WithEmbedded(true))
	server := NewWorkflowServer(backend, globalDB, resolver, nil, nil, slog.Default())

	// Try to update each built-in workflow
	for _, workflowID := range requiredBuiltinWorkflowIDs {
		t.Run(workflowID, func(t *testing.T) {
			newName := "Modified Name"
			req := connect.NewRequest(&orcv1.UpdateWorkflowRequest{
				Id:   workflowID,
				Name: &newName,
			})

			_, err := server.UpdateWorkflow(context.Background(), req)
			require.Error(t, err, "UpdateWorkflow should error for built-in workflow %s", workflowID)

			// Verify error is PermissionDenied
			connectErr, ok := err.(*connect.Error)
			require.True(t, ok, "error should be a connect.Error")
			assert.Equal(t, connect.CodePermissionDenied, connectErr.Code(),
				"error code should be PermissionDenied for built-in workflow %s", workflowID)
		})
	}
}

// TestGetWorkflow_ReturnsBuiltinWorkflow verifies GetWorkflow returns built-in workflows.
func TestGetWorkflow_ReturnsBuiltinWorkflow(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed built-in workflows
	_, err := workflow.SeedBuiltins(globalDB)
	require.NoError(t, err)

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	for _, workflowID := range requiredBuiltinWorkflowIDs {
		t.Run(workflowID, func(t *testing.T) {
			req := connect.NewRequest(&orcv1.GetWorkflowRequest{
				Id: workflowID,
			})

			resp, err := server.GetWorkflow(context.Background(), req)
			require.NoError(t, err, "GetWorkflow should not error for %s", workflowID)
			require.NotNil(t, resp.Msg.Workflow, "response should have workflow")

			wf := resp.Msg.Workflow.Workflow
			assert.Equal(t, workflowID, wf.Id)
			assert.True(t, wf.IsBuiltin, "workflow %s should have IsBuiltin=true", workflowID)
			assert.NotEmpty(t, wf.Name, "workflow %s should have a name", workflowID)

			// Verify phases are included
			assert.NotEmpty(t, resp.Msg.Workflow.Phases,
				"workflow %s should have phases", workflowID)
		})
	}
}

// TestAddPhase_BuiltinWorkflow_ReturnsPermissionDenied verifies SC-2:
// Cannot add phases to built-in workflows.
func TestAddPhase_BuiltinWorkflow_ReturnsPermissionDenied(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed built-in workflows
	_, err := workflow.SeedBuiltins(globalDB)
	require.NoError(t, err)

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	// Try to add a phase to a built-in workflow
	req := connect.NewRequest(&orcv1.AddPhaseRequest{
		WorkflowId:      "implement-medium",
		PhaseTemplateId: "spec",
		Sequence:        99,
	})

	_, err = server.AddPhase(context.Background(), req)
	require.Error(t, err, "AddPhase should error for built-in workflow")

	connectErr, ok := err.(*connect.Error)
	require.True(t, ok, "error should be a connect.Error")
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code(),
		"error code should be PermissionDenied")
}

// TestRemovePhase_BuiltinWorkflow_ReturnsPermissionDenied verifies SC-2:
// Cannot remove phases from built-in workflows.
func TestRemovePhase_BuiltinWorkflow_ReturnsPermissionDenied(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed built-in workflows
	_, err := workflow.SeedBuiltins(globalDB)
	require.NoError(t, err)

	server := NewWorkflowServer(backend, globalDB, nil, nil, nil, slog.Default())

	// Get a phase ID from the workflow first
	phases, err := globalDB.GetWorkflowPhases("implement-medium")
	require.NoError(t, err)
	require.NotEmpty(t, phases)

	// Try to remove a phase from a built-in workflow
	req := connect.NewRequest(&orcv1.RemovePhaseRequest{
		WorkflowId: "implement-medium",
		PhaseId:    int32(phases[0].ID),
	})

	_, err = server.RemovePhase(context.Background(), req)
	require.Error(t, err, "RemovePhase should error for built-in workflow")

	connectErr, ok := err.(*connect.Error)
	require.True(t, ok, "error should be a connect.Error")
	assert.Equal(t, connect.CodePermissionDenied, connectErr.Code(),
		"error code should be PermissionDenied")
}

// TestListWorkflows_IncludesSources verifies API includes source information.
func TestListWorkflows_IncludesSources(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	globalDB := storage.NewTestGlobalDB(t)

	// Seed built-in workflows
	_, err := workflow.SeedBuiltins(globalDB)
	require.NoError(t, err)

	// Use resolver to provide source information
	resolver := workflow.NewResolver(workflow.WithEmbedded(true))
	server := NewWorkflowServer(backend, globalDB, resolver, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.ListWorkflowsRequest{})
	resp, err := server.ListWorkflows(context.Background(), req)
	require.NoError(t, err)

	// Verify sources are included for built-in workflows
	for _, expectedID := range requiredBuiltinWorkflowIDs {
		source, exists := resp.Msg.Sources[expectedID]
		if !exists {
			t.Errorf("Sources should include workflow %s", expectedID)
			continue
		}

		// Built-in workflows should have embedded source
		assert.Equal(t, orcv1.DefinitionSource_DEFINITION_SOURCE_EMBEDDED, source,
			"workflow %s should have EMBEDDED source", expectedID)
	}
}
