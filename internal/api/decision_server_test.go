package api

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/task"
)

func TestDecisionServerProjectScopedIsolation(t *testing.T) {
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
	taskOne.CurrentPhase = stringPtr("review")
	require.NoError(t, backendOne.SaveTask(taskOne))

	taskTwo := task.NewProtoTask("TASK-001", "Beta blocked task")
	taskTwo.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	taskTwo.CurrentPhase = stringPtr("review")
	require.NoError(t, backendTwo.SaveTask(taskTwo))

	store := gate.NewPendingDecisionStore()
	require.NoError(t, store.Add(&gate.PendingDecision{
		ProjectID:   projectOne.ID,
		DecisionID:  "gate-review",
		TaskID:      taskOne.Id,
		TaskTitle:   taskOne.Title,
		Phase:       "review",
		GateType:    "human",
		Question:    "Ship alpha?",
		RequestedAt: time.Now(),
	}))
	require.NoError(t, store.Add(&gate.PendingDecision{
		ProjectID:   projectTwo.ID,
		DecisionID:  "gate-review",
		TaskID:      taskTwo.Id,
		TaskTitle:   taskTwo.Title,
		Phase:       "review",
		GateType:    "human",
		Question:    "Ship beta?",
		RequestedAt: time.Now(),
	}))

	publisher := events.NewMemoryPublisher()
	globalCh := publisher.Subscribe(events.GlobalTaskID)
	defer publisher.Unsubscribe(events.GlobalTaskID, globalCh)
	server := NewDecisionServer(nil, store, publisher, nil).(*decisionServer)
	server.SetProjectCache(cache)

	listResp, err := server.ListPendingDecisions(context.Background(), connect.NewRequest(&orcv1.ListPendingDecisionsRequest{
		ProjectId: projectOne.ID,
	}))
	require.NoError(t, err)
	require.Len(t, listResp.Msg.Decisions, 1)
	require.Equal(t, "Ship alpha?", listResp.Msg.Decisions[0].Question)

	_, err = server.GetPendingDecision(context.Background(), connect.NewRequest(&orcv1.GetPendingDecisionRequest{
		ProjectId: projectOne.ID,
		Id:        "gate-review",
	}))
	require.NoError(t, err)

	_, err = server.GetPendingDecision(context.Background(), connect.NewRequest(&orcv1.GetPendingDecisionRequest{
		ProjectId: projectOne.ID,
		Id:        "missing",
	}))
	require.Error(t, err)

	resolveResp, err := server.ResolveDecision(context.Background(), connect.NewRequest(&orcv1.ResolveDecisionRequest{
		ProjectId: projectOne.ID,
		Id:        "gate-review",
		Approved:  true,
	}))
	require.NoError(t, err)
	require.True(t, resolveResp.Msg.Decision.Approved)

	var resolvedEvent events.Event
	for {
		event := <-globalCh
		if event.Type != events.EventDecisionResolved {
			continue
		}
		resolvedEvent = event
		break
	}
	require.Equal(t, projectOne.ID, resolvedEvent.ProjectID)

	reloadedOne, err := backendOne.LoadTask(taskOne.Id)
	require.NoError(t, err)
	require.Equal(t, orcv1.TaskStatus_TASK_STATUS_PLANNED, reloadedOne.GetStatus())

	reloadedTwo, err := backendTwo.LoadTask(taskTwo.Id)
	require.NoError(t, err)
	require.Equal(t, orcv1.TaskStatus_TASK_STATUS_BLOCKED, reloadedTwo.GetStatus())

	_, ok := store.Get(projectOne.ID, "gate-review")
	require.False(t, ok)
	_, ok = store.Get(projectTwo.ID, "gate-review")
	require.True(t, ok)
}

func TestDecisionServerRequiresProjectScope(t *testing.T) {
	t.Parallel()

	store := gate.NewPendingDecisionStore()
	server := NewDecisionServer(nil, store, nil, nil).(*decisionServer)

	_, err := server.ListPendingDecisions(context.Background(), connect.NewRequest(&orcv1.ListPendingDecisionsRequest{}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = server.GetPendingDecision(context.Background(), connect.NewRequest(&orcv1.GetPendingDecisionRequest{
		Id: "gate-review",
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	_, err = server.ResolveDecision(context.Background(), connect.NewRequest(&orcv1.ResolveDecisionRequest{
		Id:       "gate-review",
		Approved: true,
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestDecisionServerResolveDecisionRejectsUnknownSelectedOption(t *testing.T) {
	tmpDir := setupTestHome(t)
	project := setupTestProject(t, tmpDir, "alpha")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	backend, err := cache.GetBackend(project.ID)
	require.NoError(t, err)

	blockedTask := task.NewProtoTask("TASK-010", "Blocked task")
	blockedTask.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	blockedTask.CurrentPhase = stringPtr("review")
	require.NoError(t, backend.SaveTask(blockedTask))

	store := gate.NewPendingDecisionStore()
	require.NoError(t, store.Add(&gate.PendingDecision{
		ProjectID:  project.ID,
		DecisionID: "gate-review",
		TaskID:     blockedTask.Id,
		TaskTitle:  blockedTask.Title,
		Phase:      "review",
		GateType:   "human",
		Question:   "Choose a path",
		Options: []gate.PendingDecisionOption{
			{ID: "ship-now", Label: "Ship now"},
		},
		RequestedAt: time.Now(),
	}))

	server := NewDecisionServer(nil, store, nil, nil).(*decisionServer)
	server.SetProjectCache(cache)

	_, err = server.ResolveDecision(context.Background(), connect.NewRequest(&orcv1.ResolveDecisionRequest{
		ProjectId:      project.ID,
		Id:             "gate-review",
		Approved:       true,
		SelectedOption: stringPtr("not-a-real-option"),
	}))
	require.Error(t, err)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	storedTask, loadErr := backend.LoadTask(blockedTask.Id)
	require.NoError(t, loadErr)
	require.Equal(t, orcv1.TaskStatus_TASK_STATUS_BLOCKED, storedTask.GetStatus())

	decision, ok := store.Get(project.ID, "gate-review")
	require.True(t, ok)
	require.Equal(t, "gate-review", decision.DecisionID)
}
