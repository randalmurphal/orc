package api

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

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

	require.Len(t, resp.Msg.AttentionItems, 2)
	itemsByTask := make(map[string]*orcv1.AttentionItem, len(resp.Msg.AttentionItems))
	for _, item := range resp.Msg.AttentionItems {
		itemsByTask[item.TaskId] = item
	}

	require.Contains(t, itemsByTask, taskWithSignal.Id)
	require.Contains(t, itemsByTask, taskWithoutSignal.Id)
	require.Equal(t, orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_BLOCKED_TASK, itemsByTask[taskWithSignal.Id].Type)
	require.Equal(t, string(controlplane.AttentionSignalKindBlocker), itemsByTask[taskWithSignal.Id].SignalKind)
	require.Equal(t, controlplane.AttentionSignalReferenceTypeTask, itemsByTask[taskWithSignal.Id].ReferenceType)
	require.Equal(t, taskWithSignal.Id, itemsByTask[taskWithSignal.Id].ReferenceId)
	require.Equal(t, taskWithoutSignal.Id, itemsByTask[taskWithoutSignal.Id].ReferenceId)
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

	rootResp, err := server.GetAttentionDashboardData(context.Background(), connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{}))
	require.NoError(t, err)
	require.Len(t, rootResp.Msg.AttentionItems, 2)
	require.Equal(t, projectOne.ID+"::blocked-TASK-001", rootResp.Msg.AttentionItems[0].Id)
	require.Equal(t, projectOne.ID, rootResp.Msg.AttentionItems[0].ProjectId)
	require.Equal(t, "TASK-001", rootResp.Msg.AttentionItems[0].TaskId)
	require.Equal(t, string(controlplane.AttentionSignalKindBlocker), rootResp.Msg.AttentionItems[0].SignalKind)
	require.Equal(t, controlplane.AttentionSignalReferenceTypeTask, rootResp.Msg.AttentionItems[0].ReferenceType)
	require.Equal(t, taskOne.Id, rootResp.Msg.AttentionItems[0].ReferenceId)
	require.Equal(t, projectTwo.ID+"::blocked-TASK-002", rootResp.Msg.AttentionItems[1].Id)
	require.Equal(t, projectTwo.ID, rootResp.Msg.AttentionItems[1].ProjectId)

	resp, err := server.GetAttentionDashboardData(context.Background(), connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{
		ProjectId: projectOne.ID,
	}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.AttentionItems, 1)
	require.Equal(t, "TASK-001", resp.Msg.AttentionItems[0].TaskId)
	require.Equal(t, projectOne.ID, resp.Msg.AttentionItems[0].ProjectId)
}

func TestCrossProjectRunning(t *testing.T) {
	tmpDir := setupTestHome(t)
	projectOne := setupTestProject(t, tmpDir, "alpha")
	projectTwo := setupTestProject(t, tmpDir, "beta")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	backendOne, err := cache.GetBackend(projectOne.ID)
	require.NoError(t, err)
	backendTwo, err := cache.GetBackend(projectTwo.ID)
	require.NoError(t, err)

	alphaTask := task.NewProtoTask("TASK-001", "Alpha running task")
	alphaTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	alphaTask.StartedAt = timestamppb.New(time.Now().Add(-2 * time.Hour))
	require.NoError(t, backendOne.SaveTask(alphaTask))

	betaTask := task.NewProtoTask("TASK-002", "Beta running task")
	betaTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	betaTask.StartedAt = timestamppb.New(time.Now().Add(-15 * time.Minute))
	require.NoError(t, backendTwo.SaveTask(betaTask))

	server := NewAttentionDashboardServer(nil, nil, nil, nil)
	server.(*attentionDashboardServer).SetProjectCache(cache)

	resp, err := server.GetAttentionDashboardData(context.Background(), connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{}))
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.RunningSummary)
	require.Equal(t, int32(2), resp.Msg.RunningSummary.TaskCount)
	require.Len(t, resp.Msg.RunningSummary.Tasks, 2)

	first := resp.Msg.RunningSummary.Tasks[0]
	second := resp.Msg.RunningSummary.Tasks[1]
	require.Equal(t, projectOne.ID, first.ProjectId)
	require.Equal(t, projectOne.Name, first.ProjectName)
	require.Equal(t, "TASK-001", first.Id)
	require.Equal(t, projectTwo.ID, second.ProjectId)
	require.Equal(t, projectTwo.Name, second.ProjectName)
	require.Equal(t, "TASK-002", second.Id)
	require.GreaterOrEqual(t, first.ElapsedTimeSeconds, second.ElapsedTimeSeconds)
}

func TestCrossProjectAttentionSignalsIncludeLegacyBlockedTasks(t *testing.T) {
	tmpDir := setupTestHome(t)
	projectOne := setupTestProject(t, tmpDir, "alpha")
	projectTwo := setupTestProject(t, tmpDir, "beta")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	backendOne, err := cache.GetBackend(projectOne.ID)
	require.NoError(t, err)
	backendTwo, err := cache.GetBackend(projectTwo.ID)
	require.NoError(t, err)

	taskOne := task.NewProtoTask("TASK-001", "Alpha blocked task")
	taskOne.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	require.NoError(t, backendOne.SaveTask(taskOne))

	taskTwo := task.NewProtoTask("TASK-002", "Beta failed task")
	taskTwo.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	require.NoError(t, backendTwo.SaveTask(taskTwo))

	server := NewAttentionDashboardServer(nil, nil, nil, nil)
	server.(*attentionDashboardServer).SetProjectCache(cache)

	resp, err := server.GetAttentionDashboardData(context.Background(), connect.NewRequest(&orcv1.GetAttentionDashboardDataRequest{}))
	require.NoError(t, err)
	require.Len(t, resp.Msg.AttentionItems, 2)

	itemsByProjectTask := make(map[string]*orcv1.AttentionItem, len(resp.Msg.AttentionItems))
	for _, item := range resp.Msg.AttentionItems {
		itemsByProjectTask[item.ProjectId+"::"+item.TaskId] = item
	}

	require.Contains(t, itemsByProjectTask, projectOne.ID+"::TASK-001")
	require.Contains(t, itemsByProjectTask, projectTwo.ID+"::TASK-002")
}

func TestCrossProjectAttentionRetryUsesItemProjectID(t *testing.T) {
	tmpDir := setupTestHome(t)
	projectOne := setupTestProject(t, tmpDir, "alpha")
	projectTwo := setupTestProject(t, tmpDir, "beta")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	backendOne, err := cache.GetBackend(projectOne.ID)
	require.NoError(t, err)
	backendTwo, err := cache.GetBackend(projectTwo.ID)
	require.NoError(t, err)

	taskOne := task.NewProtoTask("TASK-001", "Alpha failed task")
	taskOne.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	require.NoError(t, backendOne.SaveTask(taskOne))
	require.NoError(t, backendOne.SaveAttentionSignal(&controlplane.PersistedAttentionSignal{
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        controlplane.AttentionSignalStatusFailed,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   taskOne.Id,
		Title:         taskOne.Title,
		Summary:       "Needs retry.",
	}))

	taskTwo := task.NewProtoTask("TASK-002", "Beta failed task")
	taskTwo.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	require.NoError(t, backendTwo.SaveTask(taskTwo))
	require.NoError(t, backendTwo.SaveAttentionSignal(&controlplane.PersistedAttentionSignal{
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        controlplane.AttentionSignalStatusFailed,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   taskTwo.Id,
		Title:         taskTwo.Title,
		Summary:       "Needs retry.",
	}))

	server := NewAttentionDashboardServer(nil, nil, nil, nil)
	server.(*attentionDashboardServer).SetProjectCache(cache)

	resp, err := server.PerformAttentionAction(context.Background(), connect.NewRequest(&orcv1.PerformAttentionActionRequest{
		AttentionItemId: projectTwo.ID + "::failed-TASK-002",
		Action:          orcv1.AttentionAction_ATTENTION_ACTION_RETRY,
	}))
	require.NoError(t, err)
	require.True(t, resp.Msg.Success)

	alphaSignals, err := backendOne.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Len(t, alphaSignals, 1)

	betaSignals, err := backendTwo.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Empty(t, betaSignals)
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

func TestAttentionDashboardRetryActionRollsBackOnSignalFailure(t *testing.T) {
	t.Parallel()

	realBackend := storage.NewTestBackend(t)
	backend := &failingResolveAttentionBackend{Backend: realBackend}
	taskItem := task.NewProtoTask("TASK-001", "Failed task")
	taskItem.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	require.NoError(t, realBackend.SaveTask(taskItem))
	require.NoError(t, realBackend.SaveAttentionSignal(&controlplane.PersistedAttentionSignal{
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
	require.False(t, resp.Msg.Success)

	reloadedTask, err := realBackend.LoadTask(taskItem.Id)
	require.NoError(t, err)
	require.Equal(t, orcv1.TaskStatus_TASK_STATUS_FAILED, reloadedTask.GetStatus())

	signals, err := realBackend.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Len(t, signals, 1)
	require.Equal(t, controlplane.AttentionSignalStatusFailed, signals[0].Status)
}

type failingResolveAttentionBackend struct {
	storage.Backend
}

func (f *failingResolveAttentionBackend) ResolveAttentionSignal(id string, resolvedBy string) (*controlplane.PersistedAttentionSignal, error) {
	return nil, context.DeadlineExceeded
}
