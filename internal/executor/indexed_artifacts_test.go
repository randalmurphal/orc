package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

func TestIndexedArtifactsVariable(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	require.NoError(t, backend.DB().SaveInitiative(&db.Initiative{
		ID:     "INIT-001",
		Title:  "Indexed Artifacts",
		Status: "active",
	}))
	require.NoError(t, backend.SaveWorkflow(&db.Workflow{
		ID:          "wf-indexed",
		Name:        "indexed",
		Description: "indexed",
	}))
	seedTask := task.NewProtoTask("TASK-001", "Seed task")
	seedTask.InitiativeId = stringPtr("INIT-001")
	require.NoError(t, backend.SaveTask(seedTask))
	require.NoError(t, backend.SaveArtifactIndexEntry(&db.ArtifactIndexEntry{
		Kind:         db.ArtifactKindAcceptedRecommendation,
		Title:        "Accepted cleanup",
		Content:      "Summary: Remove duplicate polling.\nEvidence: This was already accepted.",
		DedupeKey:    "cleanup:duplicate-polling",
		InitiativeID: "INIT-001",
		SourceTaskID: "TASK-001",
	}))

	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), config.Default(), t.TempDir())
	taskItem := task.NewProtoTask("TASK-001", "Current task")
	taskItem.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	taskItem.InitiativeId = stringPtr("INIT-001")

	rctx := &variable.ResolutionContext{TaskID: taskItem.Id}
	err := we.populateControlPlaneContext(rctx, "implement", taskItem, controlPlaneVariableUsage{
		IndexedArtifacts: true,
	})
	require.NoError(t, err)
	require.Contains(t, rctx.IndexedArtifacts, "## Indexed Artifacts")
	require.Contains(t, rctx.IndexedArtifacts, "Accepted cleanup")
}

func TestIndexedArtifactsVariable_LoadFailureFailsWhenReferenced(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	failingBackend := &failingControlPlaneBackend{Backend: backend}
	we := NewWorkflowExecutor(failingBackend, backend.DB(), testGlobalDBFrom(backend), config.Default(), t.TempDir())

	rctx := &variable.ResolutionContext{
		IndexedArtifacts: "stale indexed artifacts",
	}
	err := we.populateControlPlaneContext(rctx, "implement", nil, controlPlaneVariableUsage{
		IndexedArtifacts: true,
	})
	require.Error(t, err)
	require.Empty(t, rctx.IndexedArtifacts)
}

func TestIndexedArtifactsVariable_NotLoadedWhenUnused(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	failingBackend := &failingControlPlaneBackend{Backend: backend}
	we := NewWorkflowExecutor(failingBackend, backend.DB(), testGlobalDBFrom(backend), config.Default(), t.TempDir())

	rctx := &variable.ResolutionContext{
		IndexedArtifacts: "stale indexed artifacts",
	}
	err := we.populateControlPlaneContext(rctx, "implement", nil, controlPlaneVariableUsage{})
	require.NoError(t, err)
	require.Empty(t, rctx.IndexedArtifacts)
}

func TestArtifactIndex_TaskOutcome(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	require.NoError(t, backend.DB().SaveInitiative(&db.Initiative{
		ID:     "INIT-001",
		Title:  "Task Outcomes",
		Status: "active",
	}))
	require.NoError(t, backend.SaveWorkflow(&db.Workflow{
		ID:          "wf-artifact",
		Name:        "artifact",
		Description: "artifact",
	}))

	taskItem := task.NewProtoTask("TASK-001", "Outcome task")
	taskItem.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	taskItem.InitiativeId = stringPtr("INIT-001")
	require.NoError(t, backend.SaveTask(taskItem))

	file := "internal/executor/workflow_executor.go"
	require.NoError(t, backend.SaveReviewFindings(&orcv1.ReviewRoundFindings{
		TaskId:  taskItem.Id,
		Round:   1,
		Summary: "Review round 1",
		Issues: []*orcv1.ReviewFinding{
			{Severity: "high", Description: "Missing nil guard", File: &file},
		},
	}))
	require.NoError(t, backend.SaveInitiativeNote(&db.InitiativeNote{
		ID:           "NOTE-001",
		InitiativeID: "INIT-001",
		Author:       "codex",
		AuthorType:   db.NoteAuthorAgent,
		SourceTask:   taskItem.Id,
		SourcePhase:  "docs",
		NoteType:     db.NoteTypeHandoff,
		Content:      "Keep the migration ordering stable.",
		Graduated:    true,
		CreatedAt:    time.Now(),
	}))

	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), config.Default(), t.TempDir())
	runTaskID := taskItem.Id
	run := &db.WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "wf-artifact",
		ContextType: "task",
		TaskID:      &runTaskID,
		Prompt:      "prompt",
		Status:      "completed",
	}
	require.NoError(t, backend.SaveWorkflowRun(run))

	require.NoError(t, we.indexTaskOutcomes(taskItem, run))

	results, err := backend.QueryArtifactIndex(db.ArtifactIndexQueryOpts{
		Kind:         db.ArtifactKindTaskOutcome,
		SourceTaskID: taskItem.Id,
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, results, 2)
}

func (f *failingControlPlaneBackend) GetRecentArtifacts(db.RecentArtifactOpts) ([]db.ArtifactIndexEntry, error) {
	return nil, errControlPlaneIndexedArtifactsUnavailable
}

var errControlPlaneIndexedArtifactsUnavailable = &controlPlaneTestError{message: "indexed artifacts unavailable"}

type controlPlaneTestError struct {
	message string
}

func (e *controlPlaneTestError) Error() string { return e.message }
