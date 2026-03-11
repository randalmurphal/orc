package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

func TestAttentionDashboardUsesPersistedSignals(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	taskWithSignal := task.NewProtoTask("TASK-001", "Blocked task with signal")
	taskWithSignal.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	taskWithSignal.Priority = orcv1.TaskPriority_TASK_PRIORITY_HIGH
	require.NoError(t, backend.SaveTask(taskWithSignal))
	require.NoError(t, backend.SaveAttentionSignal(&controlplane.PersistedAttentionSignal{
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        controlplane.AttentionSignalStatusBlocked,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   taskWithSignal.Id,
		Title:         taskWithSignal.Title,
		Summary:       "Waiting on review.",
	}))

	taskWithoutSignal := task.NewProtoTask("TASK-002", "Blocked task without signal")
	taskWithoutSignal.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	require.NoError(t, backend.SaveTask(taskWithoutSignal))

	server := NewAttentionDashboardServer(backend, nil, nil, nil)
	resp, err := server.GetAttentionDashboardData(context.Background(), connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{}))
	require.NoError(t, err)

	require.Len(t, resp.Msg.AttentionItems, 1)
	require.Equal(t, "TASK-001", resp.Msg.AttentionItems[0].TaskId)
	require.Equal(t, orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_BLOCKED_TASK, resp.Msg.AttentionItems[0].Type)
}

func TestCrossProjectAttentionSignalsIncludeProjectIDAndStayIsolated(t *testing.T) {
	tmpDir := setupTestHome(t)
	projectOne := setupTestProject(t, tmpDir, "alpha")
	projectTwo := setupTestProject(t, tmpDir, "beta")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	backendOne, err := cache.GetBackend(projectOne.ID)
	require.NoError(t, err)
	backendTwo, err := cache.GetBackend(projectTwo.ID)
	require.NoError(t, err)

	taskOne := task.NewProtoTask("TASK-001", "Critical blocker")
	taskOne.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	taskOne.Priority = orcv1.TaskPriority_TASK_PRIORITY_CRITICAL
	require.NoError(t, backendOne.SaveTask(taskOne))
	require.NoError(t, backendOne.SaveAttentionSignal(&controlplane.PersistedAttentionSignal{
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        controlplane.AttentionSignalStatusBlocked,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   taskOne.Id,
		Title:         taskOne.Title,
		Summary:       "Needs operator attention now.",
	}))

	taskTwo := task.NewProtoTask("TASK-002", "Normal blocker")
	taskTwo.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	taskTwo.Priority = orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	require.NoError(t, backendTwo.SaveTask(taskTwo))
	require.NoError(t, backendTwo.SaveAttentionSignal(&controlplane.PersistedAttentionSignal{
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        controlplane.AttentionSignalStatusBlocked,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   taskTwo.Id,
		Title:         taskTwo.Title,
		Summary:       "Needs attention soon.",
	}))

	server := NewAttentionDashboardServer(nil, nil, nil, nil)
	server.(*attentionDashboardServer).SetProjectCache(cache)

	signals, err := server.(*attentionDashboardServer).loadCrossProjectAttentionSignals()
	require.NoError(t, err)
	require.Len(t, signals, 2)
	require.Equal(t, projectOne.ID, signals[0].ProjectID)
	require.Equal(t, projectTwo.ID, signals[1].ProjectID)

	resp, err := server.GetAttentionDashboardData(context.Background(), connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: projectOne.ID,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.AttentionItems, 1)
	require.Equal(t, "TASK-001", resp.Msg.AttentionItems[0].TaskId)
}

func TestAttentionDashboardRetryActionResolvesSignal(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	taskItem := task.NewProtoTask("TASK-001", "Failed task")
	taskItem.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	require.NoError(t, backend.SaveTask(taskItem))
	require.NoError(t, backend.SaveAttentionSignal(&controlplane.PersistedAttentionSignal{
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        controlplane.AttentionSignalStatusFailed,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   taskItem.Id,
		Title:         taskItem.Title,
		Summary:       "Implement phase failed.",
	}))

	server := NewAttentionDashboardServer(backend, nil, nil, nil)
	resp, err := server.PerformAttentionAction(context.Background(), connect.NewRequest(&orcv1.PerformAttentionActionRequest{
		AttentionItemId: "failed-TASK-001",
		Action:          orcv1.AttentionAction_ATTENTION_ACTION_RETRY,
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Success)

	signals, err := backend.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Empty(t, signals)
}
