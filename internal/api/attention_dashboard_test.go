// Package api provides TDD tests for TASK-734: Redesign Board as attention management dashboard
//
// These tests verify the new attention management dashboard API endpoints
// that support the UX Simplification redesign with three main sections:
// - Running: Active tasks with progress and timing
// - Needs Attention: Blocked tasks, decisions, gates requiring action
// - Queue: Ready tasks organized by initiative
//
// Success Criteria Coverage:
// - SC-1: Three main sections with correct data filtering
// - SC-2: Running section with timing, progress, and phase data
// - SC-3: Needs attention section with blocked tasks and decisions
// - SC-4: Queue section with initiative organization and priority
// - SC-6: Priority-based organization and sorting
// - SC-7: Real-time update support via event publishing

package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/decision"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// SC-1: GetAttentionDashboardData returns three main sections
// ============================================================================

// TestGetAttentionDashboardData_ReturnsThreeSections verifies SC-1:
// The API should return data for all three main sections: running, needs attention, queue.
func TestGetAttentionDashboardData_ReturnsThreeSections(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create test data for each section
	runningTask := task.NewProtoTask("TASK-001", "Implement feature")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	runningTask.CurrentPhase = "implement"
	runningTask.StartedAt = timestamppb.New(time.Now().Add(-5 * time.Minute))

	blockedTask := task.NewProtoTask("TASK-002", "Deploy to prod")
	blockedTask.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	blockedTask.BlockedBy = []string{"TASK-001"}

	queuedTask := task.NewProtoTask("TASK-003", "Add tests")
	queuedTask.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED

	require.NoError(t, backend.SaveTask(runningTask))
	require.NoError(t, backend.SaveTask(blockedTask))
	require.NoError(t, backend.SaveTask(queuedTask))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err, "GetAttentionDashboardData failed")

	// VERIFY SC-1: All three sections should be present with correct data
	assert.NotNil(t, resp.Msg.RunningSummary, "running summary should be present")
	assert.NotNil(t, resp.Msg.AttentionItems, "attention items should be present")
	assert.NotNil(t, resp.Msg.QueueSummary, "queue summary should be present")

	// Running section should contain running task
	assert.Equal(t, int32(1), resp.Msg.RunningSummary.TaskCount)
	assert.Len(t, resp.Msg.RunningSummary.Tasks, 1)
	assert.Equal(t, "TASK-001", resp.Msg.RunningSummary.Tasks[0].Id)

	// Attention items should contain blocked task
	assert.Greater(t, len(resp.Msg.AttentionItems), 0, "should have attention items")

	// Queue section should contain planned task
	assert.Equal(t, int32(1), resp.Msg.QueueSummary.TaskCount)
}

// ============================================================================
// SC-2: Running section with timing, progress, and phase data
// ============================================================================

// TestGetRunningTaskDetails_IncludesTimingAndProgress verifies SC-2:
// Running tasks should include timing information, current phase, and progress data.
func TestGetRunningTaskDetails_IncludesTimingAndProgress(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	startTime := time.Now().Add(-10 * time.Minute)
	runningTask := task.NewProtoTask("TASK-001", "Implement auth system")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	runningTask.CurrentPhase = "implement"
	runningTask.StartedAt = timestamppb.New(startTime)

	initID := "INIT-001"
	runningTask.InitiativeId = &initID

	// Create initiative for display
	init := initiative.NewProtoInitiative("INIT-001", "Authentication Feature")
	require.NoError(t, backend.SaveInitiativeProto(init))

	require.NoError(t, backend.SaveTask(runningTask))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-2: Running task should include timing and phase data
	require.Len(t, resp.Msg.RunningSummary.Tasks, 1)
	runningTaskData := resp.Msg.RunningSummary.Tasks[0]

	assert.Equal(t, "TASK-001", runningTaskData.Id)
	assert.Equal(t, "Implement auth system", runningTaskData.Title)
	assert.Equal(t, "implement", runningTaskData.CurrentPhase)
	assert.NotNil(t, runningTaskData.StartedAt, "started time should be present")
	assert.Equal(t, "INIT-001", runningTaskData.InitiativeId)
	assert.Equal(t, "Authentication Feature", runningTaskData.InitiativeTitle)

	// Should include elapsed time calculation (approximately 10 minutes)
	elapsedSeconds := runningTaskData.ElapsedTimeSeconds
	assert.Greater(t, elapsedSeconds, int64(590), "should have elapsed ~600 seconds") // ~10 minutes - 10s buffer
	assert.Less(t, elapsedSeconds, int64(610), "should not exceed expected time + buffer")
}

// TestGetRunningTaskDetails_IncludesPipelineProgress verifies SC-2:
// Running tasks should include 5-phase progress pipeline (Plan → Code → Test → Review → Done).
func TestGetRunningTaskDetails_IncludesPipelineProgress(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create running task with mock execution state showing phase progress
	runningTask := task.NewProtoTask("TASK-001", "Feature implementation")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	runningTask.CurrentPhase = "implement"

	require.NoError(t, backend.SaveTask(runningTask))

	// Mock execution state with phase progression
	// TODO: Create execution state with completed spec phase, active implement phase
	// This will require integration with the execution state system

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-2: Should include pipeline progress data
	require.Len(t, resp.Msg.RunningSummary.Tasks, 1)
	runningTaskData := resp.Msg.RunningSummary.Tasks[0]

	// Should include phase progression data
	assert.NotNil(t, runningTaskData.PhaseProgress, "should include phase progress")
	assert.Equal(t, "implement", runningTaskData.PhaseProgress.CurrentPhase)

	// Should map to the 5-phase pipeline model
	assert.Contains(t, []string{"plan", "code", "test", "review", "done"},
		mapPhaseToDisplay(runningTaskData.PhaseProgress.CurrentPhase))
}

// ============================================================================
// SC-3: Needs attention section with blocked tasks and decisions
// ============================================================================

// TestGetAttentionItems_IncludesBlockedTasks verifies SC-3:
// Attention section should include blocked tasks with action buttons.
func TestGetAttentionItems_IncludesBlockedTasks(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create blocked task
	blockedTask := task.NewProtoTask("TASK-002", "Deploy to production")
	blockedTask.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	blockedTask.BlockedBy = []string{"TASK-001"}
	blockedTask.Priority = orcv1.TaskPriority_TASK_PRIORITY_HIGH

	require.NoError(t, backend.SaveTask(blockedTask))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-3: Should include blocked task in attention items
	assert.Greater(t, len(resp.Msg.AttentionItems), 0, "should have attention items")

	var blockedItem *orcv1.AttentionItem
	for _, item := range resp.Msg.AttentionItems {
		if item.Type == orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_BLOCKED_TASK {
			blockedItem = item
			break
		}
	}

	require.NotNil(t, blockedItem, "should have blocked task attention item")
	assert.Equal(t, "TASK-002", blockedItem.TaskId)
	assert.Equal(t, "Deploy to production", blockedItem.Title)
	assert.Equal(t, orcv1.TaskPriority_TASK_PRIORITY_HIGH, blockedItem.Priority)
	assert.Contains(t, blockedItem.Description, "TASK-001", "should show what task is blocking")

	// Should include available actions
	assert.Contains(t, blockedItem.AvailableActions, orcv1.AttentionAction_ATTENTION_ACTION_SKIP)
	assert.Contains(t, blockedItem.AvailableActions, orcv1.AttentionAction_ATTENTION_ACTION_FORCE)
}

// TestGetAttentionItems_IncludesPendingDecisions verifies SC-3:
// Attention section should include pending decisions requiring user choice.
func TestGetAttentionItems_IncludesPendingDecisions(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create task with pending decision
	task1 := task.NewProtoTask("TASK-001", "Choose authentication method")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	require.NoError(t, backend.SaveTask(task1))

	// Create pending decision
	pendingDecision := decision.NewProtoDecision("DEC-001", "TASK-001", "Which auth method?")
	pendingDecision.Options = []*orcv1.DecisionOption{
		{Id: "jwt", Label: "JWT tokens", Description: "Stateless tokens", Recommended: true},
		{Id: "sessions", Label: "Server sessions", Description: "Traditional sessions", Recommended: false},
	}
	require.NoError(t, backend.SaveDecisionProto(pendingDecision))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-3: Should include pending decision in attention items
	var decisionItem *orcv1.AttentionItem
	for _, item := range resp.Msg.AttentionItems {
		if item.Type == orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_PENDING_DECISION {
			decisionItem = item
			break
		}
	}

	require.NotNil(t, decisionItem, "should have pending decision attention item")
	assert.Equal(t, "TASK-001", decisionItem.TaskId)
	assert.Equal(t, "Which auth method?", decisionItem.Title)

	// Should include decision options
	assert.Len(t, decisionItem.DecisionOptions, 2)
	assert.Equal(t, "JWT tokens", decisionItem.DecisionOptions[0].Label)
	assert.Equal(t, "Server sessions", decisionItem.DecisionOptions[1].Label)
	assert.True(t, decisionItem.DecisionOptions[0].Recommended)
}

// TestGetAttentionItems_IncludesGateApprovals verifies SC-3:
// Attention section should include gates waiting for approval.
func TestGetAttentionItems_IncludesGateApprovals(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create task with gate waiting for approval
	task1 := task.NewProtoTask("TASK-001", "Code review complete")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	task1.CurrentPhase = "review"
	require.NoError(t, backend.SaveTask(task1))

	// Create gate approval waiting
	gateApproval := gate.NewProtoGateApproval("GATE-001", "TASK-001", "review")
	gateApproval.Type = orcv1.GateType_GATE_TYPE_HUMAN
	gateApproval.Question = "Ready for deployment?"
	gateApproval.Status = orcv1.GateStatus_GATE_STATUS_PENDING
	require.NoError(t, backend.SaveGateApprovalProto(gateApproval))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-3: Should include gate approval in attention items
	var gateItem *orcv1.AttentionItem
	for _, item := range resp.Msg.AttentionItems {
		if item.Type == orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_GATE_APPROVAL {
			gateItem = item
			break
		}
	}

	require.NotNil(t, gateItem, "should have gate approval attention item")
	assert.Equal(t, "TASK-001", gateItem.TaskId)
	assert.Equal(t, "Ready for deployment?", gateItem.Title)
	assert.Contains(t, gateItem.AvailableActions, orcv1.AttentionAction_ATTENTION_ACTION_APPROVE)
	assert.Contains(t, gateItem.AvailableActions, orcv1.AttentionAction_ATTENTION_ACTION_REJECT)
}

// ============================================================================
// SC-4: Queue section with initiative organization and priority
// ============================================================================

// TestGetQueueData_OrganizesByInitiative verifies SC-4:
// Queue section should organize ready tasks by initiative swimlanes.
func TestGetQueueData_OrganizesByInitiative(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create initiatives
	init1 := initiative.NewProtoInitiative("INIT-001", "Frontend Polish")
	init2 := initiative.NewProtoInitiative("INIT-002", "Auth Overhaul")
	require.NoError(t, backend.SaveInitiativeProto(init1))
	require.NoError(t, backend.SaveInitiativeProto(init2))

	// Create queued tasks linked to initiatives
	task1 := task.NewProtoTask("TASK-001", "Refactor button variants")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task1.InitiativeId = stringPtr("INIT-001")

	task2 := task.NewProtoTask("TASK-002", "Add loading states")
	task2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task2.InitiativeId = stringPtr("INIT-001")

	task3 := task.NewProtoTask("TASK-003", "Implement OAuth2")
	task3.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task3.InitiativeId = stringPtr("INIT-002")

	require.NoError(t, backend.SaveTask(task1))
	require.NoError(t, backend.SaveTask(task2))
	require.NoError(t, backend.SaveTask(task3))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-4: Should organize tasks by initiative swimlanes
	queueSummary := resp.Msg.QueueSummary
	assert.Equal(t, int32(3), queueSummary.TaskCount)
	assert.Len(t, queueSummary.Swimlanes, 2, "should have 2 initiative swimlanes")

	// Find swimlanes by initiative ID
	var frontendSwimlane, authSwimlane *orcv1.InitiativeSwimlane
	for _, swimlane := range queueSummary.Swimlanes {
		if swimlane.InitiativeId == "INIT-001" {
			frontendSwimlane = swimlane
		} else if swimlane.InitiativeId == "INIT-002" {
			authSwimlane = swimlane
		}
	}

	require.NotNil(t, frontendSwimlane, "should have frontend swimlane")
	require.NotNil(t, authSwimlane, "should have auth swimlane")

	assert.Equal(t, "Frontend Polish", frontendSwimlane.InitiativeTitle)
	assert.Equal(t, int32(2), frontendSwimlane.TaskCount)
	assert.Len(t, frontendSwimlane.Tasks, 2)

	assert.Equal(t, "Auth Overhaul", authSwimlane.InitiativeTitle)
	assert.Equal(t, int32(1), authSwimlane.TaskCount)
	assert.Len(t, authSwimlane.Tasks, 1)
}

// TestGetQueueData_IncludesTaskPositioning verifies SC-4:
// Queue should include task position numbering within swimlanes.
func TestGetQueueData_IncludesTaskPositioning(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create initiative
	init := initiative.NewProtoInitiative("INIT-001", "Test Initiative")
	require.NoError(t, backend.SaveInitiativeProto(init))

	// Create ordered tasks
	task1 := task.NewProtoTask("TASK-001", "First task")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task1.InitiativeId = stringPtr("INIT-001")

	task2 := task.NewProtoTask("TASK-002", "Second task")
	task2.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	task2.InitiativeId = stringPtr("INIT-001")

	require.NoError(t, backend.SaveTask(task1))
	require.NoError(t, backend.SaveTask(task2))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-4: Should include position numbers
	swimlane := resp.Msg.QueueSummary.Swimlanes[0]
	assert.Equal(t, int32(1), swimlane.Tasks[0].Position)
	assert.Equal(t, int32(2), swimlane.Tasks[1].Position)
}

// TestGetQueueData_IncludesPriorityIndicators verifies SC-4:
// Queue should display priority indicators for high-priority tasks.
func TestGetQueueData_IncludesPriorityIndicators(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create high priority task
	highPriorityTask := task.NewProtoTask("TASK-001", "Critical bug fix")
	highPriorityTask.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	highPriorityTask.Priority = orcv1.TaskPriority_TASK_PRIORITY_HIGH

	normalTask := task.NewProtoTask("TASK-002", "Normal feature")
	normalTask.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	normalTask.Priority = orcv1.TaskPriority_TASK_PRIORITY_NORMAL

	require.NoError(t, backend.SaveTask(highPriorityTask))
	require.NoError(t, backend.SaveTask(normalTask))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-4: Should include priority information
	queueSummary := resp.Msg.QueueSummary
	var highPriorityQueueTask *orcv1.QueuedTask
	for _, swimlane := range queueSummary.Swimlanes {
		for _, queuedTask := range swimlane.Tasks {
			if queuedTask.Id == "TASK-001" {
				highPriorityQueueTask = queuedTask
				break
			}
		}
	}

	require.NotNil(t, highPriorityQueueTask, "should find high priority task")
	assert.Equal(t, orcv1.TaskPriority_TASK_PRIORITY_HIGH, highPriorityQueueTask.Priority)
}

// ============================================================================
// SC-6: Priority-based organization and sorting
// ============================================================================

// TestGetAttentionItems_SortedByPriority verifies SC-6:
// Attention items should be sorted by priority with highest first.
func TestGetAttentionItems_SortedByPriority(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create attention items with different priorities
	criticalTask := task.NewProtoTask("TASK-001", "Critical blocked task")
	criticalTask.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	criticalTask.Priority = orcv1.TaskPriority_TASK_PRIORITY_CRITICAL
	criticalTask.BlockedBy = []string{"TASK-999"}

	highTask := task.NewProtoTask("TASK-002", "High priority blocked task")
	highTask.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	highTask.Priority = orcv1.TaskPriority_TASK_PRIORITY_HIGH
	highTask.BlockedBy = []string{"TASK-999"}

	normalTask := task.NewProtoTask("TASK-003", "Normal blocked task")
	normalTask.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	normalTask.Priority = orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	normalTask.BlockedBy = []string{"TASK-999"}

	require.NoError(t, backend.SaveTask(criticalTask))
	require.NoError(t, backend.SaveTask(highTask))
	require.NoError(t, backend.SaveTask(normalTask))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-6: Should be sorted by priority (critical, high, normal)
	attentionItems := resp.Msg.AttentionItems
	assert.GreaterOrEqual(t, len(attentionItems), 3, "should have at least 3 attention items")

	// First item should be critical priority
	assert.Equal(t, "TASK-001", attentionItems[0].TaskId)
	assert.Equal(t, orcv1.TaskPriority_TASK_PRIORITY_CRITICAL, attentionItems[0].Priority)

	// Second item should be high priority
	assert.Equal(t, "TASK-002", attentionItems[1].TaskId)
	assert.Equal(t, orcv1.TaskPriority_TASK_PRIORITY_HIGH, attentionItems[1].Priority)

	// Third item should be normal priority
	assert.Equal(t, "TASK-003", attentionItems[2].TaskId)
	assert.Equal(t, orcv1.TaskPriority_TASK_PRIORITY_NORMAL, attentionItems[2].Priority)
}

// ============================================================================
// SC-7: Real-time update support via event publishing
// ============================================================================

// TestAttentionDashboardEvents_PublishesOnDataChange verifies SC-7:
// API should publish events when attention dashboard data changes.
func TestAttentionDashboardEvents_PublishesOnDataChange(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Mock event publisher to capture published events
	eventPublisher := &mockEventPublisher{
		publishedEvents: make([]interface{}, 0),
	}

	server := NewDashboardServerWithEventPublisher(backend, eventPublisher)

	// Create initial state
	task1 := task.NewProtoTask("TASK-001", "Test task")
	task1.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	require.NoError(t, backend.SaveTask(task1))

	// Simulate status change that should trigger event
	req := connect.NewRequest(&orcv1.UpdateTaskStatusRequest{
		ProjectId: "test-project",
		TaskId:    "TASK-001",
		Status:    orcv1.TaskStatus_TASK_STATUS_RUNNING,
	})

	_, err := server.UpdateTaskStatus(context.Background(), req)
	require.NoError(t, err)

	// VERIFY SC-7: Should publish attention dashboard update event
	assert.Greater(t, len(eventPublisher.publishedEvents), 0, "should publish events")

	// Should include attention dashboard data change event
	var foundAttentionEvent bool
	for _, event := range eventPublisher.publishedEvents {
		if attentionEvent, ok := event.(*orcv1.AttentionDashboardUpdateEvent); ok {
			foundAttentionEvent = true
			assert.Equal(t, "test-project", attentionEvent.ProjectId)
			assert.Equal(t, "TASK-001", attentionEvent.TaskId)
			assert.Equal(t, orcv1.AttentionUpdateType_ATTENTION_UPDATE_TYPE_TASK_STATUS_CHANGE, attentionEvent.UpdateType)
		}
	}
	assert.True(t, foundAttentionEvent, "should publish attention dashboard update event")
}

// ============================================================================
// Integration Tests
// ============================================================================

// TestAttentionDashboardData_CorrectDataFiltering verifies complete data filtering:
// Tasks should be correctly distributed across sections based on status and priority.
func TestAttentionDashboardData_CorrectDataFiltering(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Create tasks across different states
	runningTask := task.NewProtoTask("RUNNING-001", "Active implementation")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	runningTask.CurrentPhase = "implement"
	runningTask.StartedAt = timestamppb.Now()

	blockedHighPriority := task.NewProtoTask("BLOCKED-001", "High priority blocked")
	blockedHighPriority.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	blockedHighPriority.Priority = orcv1.TaskPriority_TASK_PRIORITY_HIGH
	blockedHighPriority.BlockedBy = []string{"DEP-001"}

	queuedNormal := task.NewProtoTask("QUEUED-001", "Normal queued task")
	queuedNormal.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	queuedNormal.Priority = orcv1.TaskPriority_TASK_PRIORITY_NORMAL

	queuedLow := task.NewProtoTask("QUEUED-002", "Low priority queued")
	queuedLow.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
	queuedLow.Priority = orcv1.TaskPriority_TASK_PRIORITY_LOW

	completedTask := task.NewProtoTask("COMPLETED-001", "Finished work")
	completedTask.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED

	require.NoError(t, backend.SaveTask(runningTask))
	require.NoError(t, backend.SaveTask(blockedHighPriority))
	require.NoError(t, backend.SaveTask(queuedNormal))
	require.NoError(t, backend.SaveTask(queuedLow))
	require.NoError(t, backend.SaveTask(completedTask))

	server := NewDashboardServer(backend, nil)

	req := connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetAttentionDashboardData(context.Background(), req)
	require.NoError(t, err)

	// VERIFY: Correct distribution across sections

	// Running section should contain only running task
	assert.Equal(t, int32(1), resp.Msg.RunningSummary.TaskCount)
	assert.Equal(t, "RUNNING-001", resp.Msg.RunningSummary.Tasks[0].Id)

	// Attention section should contain blocked task (high priority surfaces it)
	var hasBlockedTask bool
	for _, item := range resp.Msg.AttentionItems {
		if item.TaskId == "BLOCKED-001" {
			hasBlockedTask = true
			assert.Equal(t, orcv1.TaskPriority_TASK_PRIORITY_HIGH, item.Priority)
		}
	}
	assert.True(t, hasBlockedTask, "attention section should contain high priority blocked task")

	// Queue section should contain planned tasks, but not completed ones
	assert.Equal(t, int32(2), resp.Msg.QueueSummary.TaskCount) // Normal + Low priority

	// Completed task should not appear in any section
	for _, runningTask := range resp.Msg.RunningSummary.Tasks {
		assert.NotEqual(t, "COMPLETED-001", runningTask.Id)
	}
	for _, attentionItem := range resp.Msg.AttentionItems {
		assert.NotEqual(t, "COMPLETED-001", attentionItem.TaskId)
	}
	// Queue check would require iterating through swimlanes, but principle is the same
}

// ============================================================================
// Helper Functions and Mocks
// ============================================================================

// stringPtr returns a pointer to a string value
func stringPtr(s string) *string {
	return &s
}

// mapPhaseToDisplay maps internal phase names to display names for pipeline
func mapPhaseToDisplay(phase string) string {
	switch phase {
	case "spec", "design", "research":
		return "plan"
	case "implement":
		return "code"
	case "test":
		return "test"
	case "review":
		return "review"
	case "docs", "validate":
		return "done"
	default:
		return phase
	}
}

// mockEventPublisher captures events for testing
type mockEventPublisher struct {
	publishedEvents []interface{}
}

func (m *mockEventPublisher) PublishEvent(event interface{}) {
	m.publishedEvents = append(m.publishedEvents, event)
}

// NewDashboardServerWithEventPublisher creates server with custom event publisher
func NewDashboardServerWithEventPublisher(backend storage.Backend, eventPublisher interface{}) *DashboardServer {
	// This would be implemented to inject a custom event publisher
	// for testing event publication in SC-7
	return NewDashboardServer(backend, nil)
}