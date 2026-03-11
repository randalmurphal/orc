package storage

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

func TestRecommendationConcurrentDecision(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)
	createRecommendationFixtures(t, backend)

	rec := testProtoRecommendation()
	require.NoError(t, backend.SaveRecommendation(rec))

	var wg sync.WaitGroup
	errs := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := backend.UpdateRecommendationStatus(
				rec.Id,
				orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED,
				"randy",
				"take action",
			)
			errs <- err
		}()
	}

	wg.Wait()
	close(errs)

	var successCount int
	var conflictCount int
	for err := range errs {
		if err == nil {
			successCount++
			continue
		}
		require.ErrorIs(t, err, db.ErrRecommendationConflict)
		conflictCount++
	}

	require.Equal(t, 1, successCount)
	require.Equal(t, 1, conflictCount)

	loaded, err := backend.LoadRecommendation(rec.Id)
	require.NoError(t, err)
	require.Equal(t, orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED, loaded.Status)
	require.Equal(t, "randy", loaded.GetDecidedBy())
}

func TestRecommendationBackendRoundTrip(t *testing.T) {
	t.Parallel()

	backend := NewTestBackend(t)
	createRecommendationFixtures(t, backend)

	rec := testProtoRecommendation()
	require.NoError(t, backend.SaveRecommendation(rec))

	loaded, err := backend.LoadRecommendation(rec.Id)
	require.NoError(t, err)
	require.Equal(t, rec.DedupeKey, loaded.DedupeKey)
	require.Equal(t, rec.SourceThreadId, loaded.SourceThreadId)
	require.Empty(t, loaded.PromotedToType)
	require.Empty(t, loaded.PromotedToId)
	require.Empty(t, loaded.PromotedBy)
	require.Nil(t, loaded.PromotedAt)

	count, err := backend.CountRecommendationsByStatus(orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func createRecommendationFixtures(t *testing.T, backend *DatabaseBackend) {
	t.Helper()

	require.NoError(t, backend.DB().SaveWorkflow(&db.Workflow{
		ID:   "wf-recommendation",
		Name: "Recommendation Workflow",
	}))
	require.NoError(t, backend.DB().SaveTask(&db.Task{
		ID:         "TASK-001",
		Title:      "Recommendation Source Task",
		WorkflowID: "wf-recommendation",
		Status:     "running",
	}))
	taskID := "TASK-001"
	require.NoError(t, backend.DB().SaveWorkflowRun(&db.WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "wf-recommendation",
		ContextType: "task",
		TaskID:      &taskID,
		Status:      "running",
	}))
	thread := &db.Thread{
		Title:  "Recommendation discussion",
		TaskID: taskID,
	}
	require.NoError(t, backend.DB().CreateThread(thread))
}

func testProtoRecommendation() *orcv1.Recommendation {
	return &orcv1.Recommendation{
		Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP,
		Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
		Title:          "Clean up duplicate polling",
		Summary:        "Two polling loops are doing the same work.",
		ProposedAction: "Remove the legacy loop after validating the new path.",
		Evidence:       "Both loops hit the same endpoint every 5 seconds.",
		SourceTaskId:   "TASK-001",
		SourceRunId:    "RUN-001",
		SourceThreadId: "THR-001",
		DedupeKey:      "cleanup:task-001:duplicate-polling",
	}
}
