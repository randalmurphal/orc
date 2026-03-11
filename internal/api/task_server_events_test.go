package api

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/task"
)

func TestPauseAllTasksPublishesProjectScopedTaskUpdatedEvents(t *testing.T) {
	tmpDir := setupTestHome(t)
	proj := setupTestProject(t, tmpDir, "alpha")

	cache := NewProjectCache(10)
	defer func() { _ = cache.Close() }()

	backend, err := cache.GetBackend(proj.ID)
	require.NoError(t, err)

	runningTask := task.NewProtoTask("TASK-001", "Running task")
	runningTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	require.NoError(t, backend.SaveTask(runningTask))

	publisher := events.NewMemoryPublisher()
	eventCh := publisher.Subscribe(events.GlobalTaskID)
	defer publisher.Unsubscribe(events.GlobalTaskID, eventCh)

	server := NewTaskServer(backend, nil, nil, publisher, "", nil, nil).(*taskServer)
	server.SetProjectCache(cache)

	_, err = server.PauseAllTasks(context.Background(), connect.NewRequest(&orcv1.PauseAllTasksRequest{
		ProjectId: proj.ID,
	}))
	require.NoError(t, err)

	event := <-eventCh
	require.Equal(t, events.EventTaskUpdated, event.Type)
	require.Equal(t, proj.ID, event.ProjectID)
	require.Equal(t, runningTask.Id, event.TaskID)
}
