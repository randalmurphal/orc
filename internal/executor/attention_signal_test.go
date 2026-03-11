package executor

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

func TestAttentionSignalFailRunCreatesSignalAndEvent(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := &testPublishHelper{}
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		testGlobalDBFrom(backend),
		config.Default(),
		t.TempDir(),
		WithWorkflowPublisher(publisher),
	)

	tsk := task.NewProtoTask("TASK-001", "Failing task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	require.NoError(t, backend.SaveTask(tsk))
	require.NoError(t, backend.SaveWorkflow(&db.Workflow{ID: "wf-attention", Name: "Attention Workflow"}))

	runTaskID := tsk.Id
	run := &db.WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "wf-attention",
		ContextType: "task",
		TaskID:      &runTaskID,
		Status:      string(workflow.RunStatusRunning),
	}

	we.failRun(run, tsk, fmt.Errorf("phase exploded"))

	signals, err := backend.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Len(t, signals, 1)
	require.Equal(t, controlplane.AttentionSignalKindBlocker, signals[0].Kind)
	require.Equal(t, controlplane.AttentionSignalStatusFailed, signals[0].Status)
	require.Equal(t, controlplane.AttentionSignalReferenceTypeTask, signals[0].ReferenceType)
	require.Equal(t, tsk.Id, signals[0].ReferenceID)

	require.Equal(t, orcv1.TaskStatus_TASK_STATUS_FAILED, tsk.Status)
	require.True(t, hasPublishedEvent(publisher.events, events.EventAttentionSignalCreated))
}

func TestAttentionSignalResolveForTaskPublishesResolvedEvent(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := &testPublishHelper{}
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		testGlobalDBFrom(backend),
		config.Default(),
		t.TempDir(),
		WithWorkflowPublisher(publisher),
	)

	require.NoError(t, backend.SaveTask(task.NewProtoTask("TASK-001", "Resolvable task")))
	require.NoError(t, backend.SaveAttentionSignal(&controlplane.PersistedAttentionSignal{
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        controlplane.AttentionSignalStatusBlocked,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   "TASK-001",
		Title:         "Blocked task",
		Summary:       "Waiting on a decision.",
	}))

	require.NoError(t, we.resolveAttentionSignalsForTask("TASK-001", "resume"))

	signals, err := backend.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Empty(t, signals)
	require.True(t, hasPublishedEvent(publisher.events, events.EventAttentionSignalResolved))
}

func TestAttentionSignalFailRunDoesNotPersistWhenSignalSaveFails(t *testing.T) {
	t.Parallel()

	realBackend := storage.NewTestBackend(t)
	backend := &failingAttentionSignalBackend{Backend: realBackend}
	we := NewWorkflowExecutor(
		backend,
		realBackend.DB(),
		testGlobalDBFrom(realBackend),
		config.Default(),
		t.TempDir(),
	)

	tsk := task.NewProtoTask("TASK-001", "Failing task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	require.NoError(t, realBackend.SaveTask(tsk))
	require.NoError(t, realBackend.SaveWorkflow(&db.Workflow{ID: "wf-attention", Name: "Attention Workflow"}))

	runTaskID := tsk.Id
	run := &db.WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "wf-attention",
		ContextType: "task",
		TaskID:      &runTaskID,
		Status:      string(workflow.RunStatusRunning),
	}

	we.failRun(run, tsk, fmt.Errorf("phase exploded"))

	signals, err := realBackend.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Empty(t, signals)
}

func TestAttentionSignalCompletionFailureCreatesSignalAndEvent(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	publisher := &testPublishHelper{}
	we := NewWorkflowExecutor(
		backend,
		backend.DB(),
		testGlobalDBFrom(backend),
		config.Default(),
		t.TempDir(),
		WithWorkflowPublisher(publisher),
	)

	tsk := task.NewProtoTask("TASK-001", "Completion failure")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	require.NoError(t, backend.SaveTask(tsk))

	we.failTaskAfterCompletionError(tsk, fmt.Errorf("pr creation failed"))

	loadedTask, err := backend.LoadTask(tsk.Id)
	require.NoError(t, err)
	require.Equal(t, orcv1.TaskStatus_TASK_STATUS_FAILED, loadedTask.GetStatus())
	require.Equal(t, "completion_failed", loadedTask.GetMetadata()["failed_reason"])

	signals, err := backend.LoadActiveAttentionSignals()
	require.NoError(t, err)
	require.Len(t, signals, 1)
	require.Equal(t, controlplane.AttentionSignalStatusFailed, signals[0].Status)
	require.True(t, hasPublishedEvent(publisher.events, events.EventAttentionSignalCreated))
	require.True(t, hasPublishedEvent(publisher.events, events.EventTaskUpdated))
}

type failingAttentionSignalBackend struct {
	storage.Backend
}

func (f *failingAttentionSignalBackend) SaveAttentionSignal(*controlplane.PersistedAttentionSignal) error {
	return fmt.Errorf("attention signals unavailable")
}

func hasPublishedEvent(published []events.Event, eventType events.EventType) bool {
	for _, event := range published {
		if event.Type == eventType {
			return true
		}
	}
	return false
}
