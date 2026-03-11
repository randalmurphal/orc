// Package api provides TDD tests for TASK-772: Complete TASK-734 backend integration (action handlers)
//
// These tests verify the backend integration gaps in the attention dashboard:
// - Action handlers (PerformAttentionAction, UpdateQueueOrganization)
// - Missing integrations (pending decisions, gate approvals, real data)
// - Real output lines from transcripts
// - Calculated initiative completion percentages
//
// Success Criteria Coverage:
// - SC-1: PerformAttentionAction processes all attention actions correctly
// - SC-2: buildAttentionItems loads real pending decisions from decision store
// - SC-3: Running tasks show real output lines and initiatives show calculated completion

package api

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-1: PerformAttentionAction processes all attention actions correctly
// ============================================================================

// TestPerformAttentionAction_RetryAction verifies SC-1:
// RETRY actions should call TaskService.RetryTask to resume failed tasks
func TestPerformAttentionAction_RetryAction(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a failed task
	failedTask := task.NewProtoTask("TASK-001", "Failed task")
	failedTask.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	require.NoError(t, backend.SaveTask(failedTask))

	server := NewAttentionDashboardServer(backend, nil, nil, nil)

	req := connect.NewRequest(&orcv1.PerformAttentionActionRequest{
		AttentionItemId: "failed-TASK-001",
		Action:          orcv1.AttentionAction_ATTENTION_ACTION_RETRY,
	})

	resp, err := server.PerformAttentionAction(context.Background(), req)
	require.NoError(t, err)

	// Should successfully retry the task (not return "not yet implemented")
	assert.True(t, resp.Msg.Success, "RETRY action should succeed")
	assert.Empty(t, resp.Msg.ErrorMessage, "RETRY action should not return error message")

	// Task should be resumed to running status
	reloadedTask, err := backend.LoadTask("TASK-001")
	require.NoError(t, err)
	assert.Equal(t, orcv1.TaskStatus_TASK_STATUS_RUNNING, reloadedTask.Status, "Task should be resumed to running")
}

// TestPerformAttentionAction_ApproveAction verifies SC-1:
// APPROVE actions should call DecisionService.ResolveDecision with approved=true
func TestPerformAttentionAction_ApproveAction(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "alpha")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	backend, err := cache.GetBackend(proj.ID)
	require.NoError(t, err)

	// Create a blocked task with pending decision
	blockedTask := task.NewProtoTask("TASK-002", "Blocked task")
	blockedTask.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	blockedTask.CurrentPhase = stringPtr("implement")
	require.NoError(t, backend.SaveTask(blockedTask))

	// Create a pending decisions store with a test decision
	pendingDecisions := gate.NewPendingDecisionStore()
	testDecision := &gate.PendingDecision{
		ProjectID:   proj.ID,
		DecisionID:  "DEC-001",
		TaskID:      "TASK-002",
		TaskTitle:   "Blocked task",
		Phase:       "implement",
		GateType:    "human",
		Question:    "Should we proceed with this implementation?",
		Context:     "Test decision",
		Options: []gate.PendingDecisionOption{
			{
				ID:          "ship-now",
				Label:       "Ship now",
				Description: "Accept the reviewed path",
				Recommended: true,
			},
		},
		RequestedAt: time.Now(),
	}
	require.NoError(t, pendingDecisions.Add(testDecision))

	server := NewAttentionDashboardServer(nil, nil, pendingDecisions, nil).(*attentionDashboardServer)
	server.SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.PerformAttentionActionRequest{
		ProjectId:       proj.ID,
		AttentionItemId: "decision-DEC-001",
		Action:          orcv1.AttentionAction_ATTENTION_ACTION_APPROVE,
		DecisionOptionId: "ship-now",
	})

	resp, err := server.PerformAttentionAction(context.Background(), req)
	require.NoError(t, err)

	// Should successfully approve the decision (not return "not yet implemented")
	assert.True(t, resp.Msg.Success, "APPROVE action should succeed")
	assert.Empty(t, resp.Msg.ErrorMessage, "APPROVE action should not return error message")
}

// TestPerformAttentionAction_ViewAction verifies SC-1:
// VIEW actions should return success without side effects
func TestPerformAttentionAction_ViewAction(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewAttentionDashboardServer(backend, nil, nil, nil)

	req := connect.NewRequest(&orcv1.PerformAttentionActionRequest{
		AttentionItemId: "blocked-TASK-003",
		Action:          orcv1.AttentionAction_ATTENTION_ACTION_VIEW,
	})

	resp, err := server.PerformAttentionAction(context.Background(), req)
	require.NoError(t, err)

	// VIEW should always succeed
	assert.True(t, resp.Msg.Success, "VIEW action should succeed")
	assert.Empty(t, resp.Msg.ErrorMessage, "VIEW action should not return error message")
}

// ============================================================================
// SC-2: buildAttentionItems loads real pending decisions from decision store
// ============================================================================

// TestAttentionDashboard_IncludesPendingDecisions verifies SC-2:
// buildAttentionItems() should load pending decisions from DecisionService.ListPendingDecisions
// and include them as attention items with APPROVE/REJECT actions
func TestAttentionDashboard_IncludesPendingDecisions(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "alpha")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	backend, err := cache.GetBackend(proj.ID)
	require.NoError(t, err)

	blockedTask := task.NewProtoTask("TASK-004", "Blocked task needing decision")
	blockedTask.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	require.NoError(t, backend.SaveTask(blockedTask))

	pendingDecisions := gate.NewPendingDecisionStore()
	require.NoError(t, pendingDecisions.Add(&gate.PendingDecision{
		ProjectID:   proj.ID,
		DecisionID:  "DEC-004",
		TaskID:      "TASK-004",
		TaskTitle:   blockedTask.Title,
		Phase:       "review",
		GateType:    "human",
		Question:    "Approve rollout?",
		Context:     "Needs a human decision",
		RequestedAt: time.Now(),
	}))

	server := NewAttentionDashboardServer(nil, nil, pendingDecisions, nil).(*attentionDashboardServer)
	server.SetProjectCache(cache)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: proj.ID,
	})
	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	var decisionItem *orcv1.AttentionItem
	for _, item := range resp.Msg.AttentionItems {
		if item.Type == orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_PENDING_DECISION {
			decisionItem = item
			break
		}
	}

	require.NotNil(t, decisionItem, "should include pending decision attention item")
	assert.Equal(t, proj.ID+"::decision-DEC-004", decisionItem.Id)
	assert.Equal(t, "TASK-004", decisionItem.TaskId)
	assert.Equal(t, "Blocked task needing decision", decisionItem.Title)
	assert.Equal(t, proj.ID, decisionItem.ProjectId)
	assert.Contains(t, decisionItem.AvailableActions, orcv1.AttentionAction_ATTENTION_ACTION_APPROVE)
	assert.Contains(t, decisionItem.AvailableActions, orcv1.AttentionAction_ATTENTION_ACTION_REJECT)
}

func TestAttentionDashboard_PendingDecisionsStayProjectScoped(t *testing.T) {
	tmpDir := setupTestHome(t)
	projectOne := setupTestProject(t, tmpDir, "alpha")
	projectTwo := setupTestProject(t, tmpDir, "beta")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	backendOne, err := cache.GetBackend(projectOne.ID)
	require.NoError(t, err)
	backendTwo, err := cache.GetBackend(projectTwo.ID)
	require.NoError(t, err)

	taskOne := task.NewProtoTask("TASK-001", "Alpha decision")
	taskOne.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	require.NoError(t, backendOne.SaveTask(taskOne))

	taskTwo := task.NewProtoTask("TASK-001", "Beta decision")
	taskTwo.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	require.NoError(t, backendTwo.SaveTask(taskTwo))

	store := gate.NewPendingDecisionStore()
	require.NoError(t, store.Add(&gate.PendingDecision{
		ProjectID:   projectOne.ID,
		DecisionID:  "gate-review",
		TaskID:      taskOne.Id,
		TaskTitle:   taskOne.Title,
		Phase:       "review",
		GateType:    "human",
		Question:    "Approve alpha?",
		RequestedAt: time.Now(),
	}))
	require.NoError(t, store.Add(&gate.PendingDecision{
		ProjectID:   projectTwo.ID,
		DecisionID:  "gate-review",
		TaskID:      taskTwo.Id,
		TaskTitle:   taskTwo.Title,
		Phase:       "review",
		GateType:    "human",
		Question:    "Approve beta?",
		RequestedAt: time.Now(),
	}))

	server := NewAttentionDashboardServer(nil, nil, store, nil).(*attentionDashboardServer)
	server.SetProjectCache(cache)

	resp, err := server.GetAttentionDashboardData(context.Background(), connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: projectOne.ID,
	}))
	require.NoError(t, err)

	var decisionItems []*orcv1.AttentionItem
	for _, item := range resp.Msg.AttentionItems {
		if item.Type == orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_PENDING_DECISION {
			decisionItems = append(decisionItems, item)
		}
	}

	require.Len(t, decisionItems, 1)
	require.Equal(t, projectOne.ID, decisionItems[0].ProjectId)
	require.Equal(t, "Approve alpha?", decisionItems[0].Description)
	require.Equal(t, projectOne.ID+"::decision-gate-review", decisionItems[0].Id)
}

// ============================================================================
// SC-3: Running tasks show real output lines and initiatives show calculated completion
// ============================================================================

// TestRunningSummary_RealOutputLines verifies SC-3:
// Running tasks should display real output lines from recent transcript messages
func TestRunningSummary_RealOutputLines(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create a running task
	runningTask := task.NewProtoTask("TASK-005", "Running task with output")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	runningTask.StartedAt = timestamppb.New(time.Now().Add(-30 * time.Second))
	currentPhase := "implement"
	runningTask.CurrentPhase = &currentPhase
	require.NoError(t, backend.SaveTask(runningTask))

	// Add some transcript messages for this task
	transcript1 := &storage.Transcript{
		TaskID:      "TASK-005",
		Phase:       "implement",
		SessionID:   "session-1",
		MessageUUID: "msg-1",
		Role:        "assistant",
		Content:     "Starting implementation...",
		Timestamp:   time.Now().Add(-20 * time.Second).UnixMilli(),
	}
	transcript2 := &storage.Transcript{
		TaskID:      "TASK-005",
		Phase:       "implement",
		SessionID:   "session-1",
		MessageUUID: "msg-2",
		Role:        "assistant",
		Content:     "Writing tests for the new feature...",
		Timestamp:   time.Now().Add(-10 * time.Second).UnixMilli(),
	}
	require.NoError(t, backend.AddTranscript(transcript1))
	require.NoError(t, backend.AddTranscript(transcript2))

	server := NewAttentionDashboardServer(backend, nil, nil, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{})
	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// Should have running task with real output lines
	require.Len(t, resp.Msg.RunningSummary.Tasks, 1, "Should have one running task")
	runningTaskResp := resp.Msg.RunningSummary.Tasks[0]

	// Output lines should be populated from transcripts (not empty)
	assert.NotEmpty(t, runningTaskResp.OutputLines, "OutputLines should be populated from transcripts, not hardcoded empty")
	assert.Contains(t, runningTaskResp.OutputLines, "Starting implementation...", "Should include recent transcript content")
}

// TestQueueSummary_CalculatedCompletionPercentage verifies SC-3:
// Initiative completion percentages should be calculated based on actual task completion
func TestQueueSummary_CalculatedCompletionPercentage(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create an initiative with mixed task statuses
	init1 := initiative.NewProtoInitiative("INIT-010", "Test Initiative")
	require.NoError(t, backend.SaveInitiativeProto(init1))

	// Create tasks: 2 completed, 1 running, 1 planned = 2/4 = 50% completion
	completedTask1 := task.NewProtoTask("TASK-010", "Completed task 1")
	completedTask1.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	completedTask1.InitiativeId = stringPtrHelper("INIT-010")

	completedTask2 := task.NewProtoTask("TASK-011", "Completed task 2")
	completedTask2.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	completedTask2.InitiativeId = stringPtrHelper("INIT-010")

	runningTask := task.NewProtoTask("TASK-012", "Running task")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	runningTask.InitiativeId = stringPtrHelper("INIT-010")

	plannedTask := task.NewProtoTask("TASK-013", "Planned task")
	plannedTask.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	plannedTask.InitiativeId = stringPtrHelper("INIT-010")

	require.NoError(t, backend.SaveTask(completedTask1))
	require.NoError(t, backend.SaveTask(completedTask2))
	require.NoError(t, backend.SaveTask(runningTask))
	require.NoError(t, backend.SaveTask(plannedTask))

	server := NewAttentionDashboardServer(backend, nil, nil, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{})
	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// Find the initiative swimlane
	var initSwimlane *orcv1.InitiativeSwimlane
	for _, swimlane := range resp.Msg.QueueSummary.Swimlanes {
		if swimlane.InitiativeId == "INIT-010" {
			initSwimlane = swimlane
			break
		}
	}
	require.NotNil(t, initSwimlane, "Should have initiative swimlane")

	// Completion percentage should be calculated (50%), not hardcoded 0
	assert.NotEqual(t, float32(0), initSwimlane.CompletionPercentage, "CompletionPercentage should be calculated, not hardcoded 0")
	assert.Equal(t, float32(50), initSwimlane.CompletionPercentage, "Should calculate 50% completion (2 completed out of 4 total)")
}

// TestUpdateQueueOrganization_Implementation verifies that UpdateQueueOrganization
// is implemented (not returning "not yet implemented")
func TestUpdateQueueOrganization_Implementation(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewAttentionDashboardServer(backend, nil, nil, nil)

	req := connect.NewRequest(&orcv1.UpdateQueueOrganizationRequest{
		Update: &orcv1.UpdateQueueOrganizationRequest_SwimlaneState{
			SwimlaneState: &orcv1.SwimlaneStateUpdate{
				InitiativeId: "INIT-001",
				Collapsed:    true,
			},
		},
	})

	resp, err := server.UpdateQueueOrganization(context.Background(), req)
	require.NoError(t, err)

	// Should not return "not yet implemented"
	assert.True(t, resp.Msg.Success, "UpdateQueueOrganization should succeed")
	assert.Empty(t, resp.Msg.ErrorMessage, "UpdateQueueOrganization should not return error message")
}

// stringPtrHelper returns a pointer to a string value (helper for this test file)
// Using stringPtrHelper to avoid conflict with existing stringPtr in main test file
func stringPtrHelper(s string) *string {
	return &s
}
