package db

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecommendationCRUD(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)

	rec := newTestRecommendation()
	require.NoError(t, pdb.CreateRecommendation(rec))
	require.NotEmpty(t, rec.ID)
	require.False(t, rec.CreatedAt.IsZero())
	require.False(t, rec.UpdatedAt.IsZero())

	loaded, err := pdb.GetRecommendation(rec.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Equal(t, rec.DedupeKey, loaded.DedupeKey)
	require.Equal(t, RecommendationStatusPending, loaded.Status)
	require.Equal(t, rec.SourceThreadID, loaded.SourceThreadID)
	require.Empty(t, loaded.PromotedToType)
	require.Empty(t, loaded.PromotedToID)
	require.Empty(t, loaded.PromotedBy)
	require.Nil(t, loaded.PromotedAt)

	list, err := pdb.ListRecommendations(RecommendationListOpts{Status: RecommendationStatusPending})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, rec.ID, list[0].ID)

	require.NoError(t, pdb.DeleteRecommendation(rec.ID))

	deleted, err := pdb.GetRecommendation(rec.ID)
	require.NoError(t, err)
	require.Nil(t, deleted)
}

func TestRecommendationTransition(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)

	rec := newTestRecommendation()
	require.NoError(t, pdb.CreateRecommendation(rec))

	discussed, err := pdb.DiscussRecommendation(rec.ID, "randy", "needs operator review")
	require.NoError(t, err)
	require.Equal(t, RecommendationStatusDiscussed, discussed.Status)
	require.NotNil(t, discussed.DecidedAt)
	require.Equal(t, "randy", discussed.DecidedBy)

	accepted, err := pdb.AcceptRecommendation(rec.ID, "randy", "ship it")
	require.NoError(t, err)
	require.Equal(t, RecommendationStatusAccepted, accepted.Status)
	require.NotNil(t, accepted.DecidedAt)
	require.Equal(t, "randy", accepted.DecidedBy)
	require.Equal(t, "ship it", accepted.DecisionReason)

	history, err := pdb.ListRecommendationHistory(rec.ID)
	require.NoError(t, err)
	require.Len(t, history, 3)
	require.Equal(t, RecommendationStatusAccepted, history[0].ToStatus)
	require.Equal(t, RecommendationStatusDiscussed, history[0].FromStatus)

	invalid, err := pdb.AcceptRecommendation(rec.ID, "randy", "again")
	require.ErrorIs(t, err, ErrRecommendationConflict)
	require.Nil(t, invalid)

	rejectedRec := newTestRecommendation()
	rejectedRec.DedupeKey = "cleanup:task-001:rejected"
	require.NoError(t, pdb.CreateRecommendation(rejectedRec))
	_, err = pdb.RejectRecommendation(rejectedRec.ID, "randy", "not worth it")
	require.NoError(t, err)

	invalid, err = pdb.AcceptRecommendation(rejectedRec.ID, "randy", "too late")
	require.ErrorIs(t, err, ErrInvalidRecommendationTransition)
	require.Nil(t, invalid)
}

func TestRecommendationAcceptPromotionToTask(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)

	rec := newTestRecommendation()
	require.NoError(t, pdb.CreateRecommendation(rec))

	now := time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)
	taskItem := &Task{
		ID:           "TASK-002",
		Title:        rec.Title,
		Description:  "Accepted from recommendation REC-001.",
		WorkflowID:   "wf-recommendation",
		Status:       "created",
		StateStatus:  "pending",
		Branch:       "orc/TASK-002",
		Queue:        "backlog",
		Priority:     "normal",
		Category:     "feature",
		InitiativeID: "INIT-001",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	accepted, err := pdb.AcceptRecommendationWithTask(rec.ID, "randy", "worth doing", taskItem)
	require.NoError(t, err)
	require.Equal(t, RecommendationStatusAccepted, accepted.Status)
	require.Equal(t, RecommendationPromotionTypeTask, accepted.PromotedToType)
	require.Equal(t, "TASK-002", accepted.PromotedToID)

	savedTask, err := pdb.GetTask("TASK-002")
	require.NoError(t, err)
	require.NotNil(t, savedTask)
	require.Equal(t, "backlog", savedTask.Queue)

	history, err := pdb.ListRecommendationHistory(rec.ID)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.Equal(t, RecommendationStatusAccepted, history[0].ToStatus)
	require.Equal(t, "worth doing", history[0].DecisionReason)
}

func TestRecommendationAcceptPromotionRollsBackOnDecisionFailure(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)

	rec := newTestRecommendation()
	require.NoError(t, pdb.CreateRecommendation(rec))

	_, err := pdb.AcceptRecommendationWithDecision(rec.ID, "randy", "missing initiative", &InitiativeDecision{
		ID:           "DEC-ROLLBACK",
		InitiativeID: "INIT-404",
		Decision:     "Do not create this decision",
		DecidedBy:    "randy",
		DecidedAt:    time.Now(),
	})
	require.Error(t, err)

	reloaded, err := pdb.GetRecommendation(rec.ID)
	require.NoError(t, err)
	require.Equal(t, RecommendationStatusPending, reloaded.Status)
	require.Empty(t, reloaded.PromotedToType)
	require.Empty(t, reloaded.PromotedToID)
}

func TestRecommendationDedupeKey(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)

	first := newTestRecommendation()
	second := newTestRecommendation()
	second.DedupeKey = first.DedupeKey

	require.NoError(t, pdb.CreateRecommendation(first))
	err := pdb.CreateRecommendation(second)
	require.Error(t, err)
	require.True(t, strings.Contains(strings.ToLower(err.Error()), "unique"))
}

func TestRecommendationListFilters(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)

	first := newTestRecommendation()
	second := newTestRecommendation()
	second.Kind = RecommendationKindRisk
	second.DedupeKey = "risk:task-001:1"
	second.Title = "Risk recommendation"
	second.Summary = "This is risky"
	second.ProposedAction = "Reduce risk"
	second.Evidence = "Load spikes"
	require.NoError(t, pdb.CreateRecommendation(first))
	require.NoError(t, pdb.CreateRecommendation(second))
	_, err := pdb.RejectRecommendation(second.ID, "randy", "accepted risk")
	require.NoError(t, err)

	pending, err := pdb.ListRecommendations(RecommendationListOpts{Status: RecommendationStatusPending})
	require.NoError(t, err)
	require.Len(t, pending, 1)
	require.Equal(t, first.ID, pending[0].ID)

	risks, err := pdb.ListRecommendations(RecommendationListOpts{Kind: RecommendationKindRisk})
	require.NoError(t, err)
	require.Len(t, risks, 1)
	require.Equal(t, second.ID, risks[0].ID)

	byTask, err := pdb.ListRecommendations(RecommendationListOpts{SourceTaskID: first.SourceTaskID})
	require.NoError(t, err)
	require.Len(t, byTask, 2)
}

func TestRecommendationCreateRejectsPrePromotedData(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)

	rec := newTestRecommendation()
	rec.PromotedToType = RecommendationPromotionTypeTask
	rec.PromotedToID = "TASK-002"
	rec.PromotedBy = "operator"
	now := time.Now()
	rec.PromotedAt = &now

	err := pdb.CreateRecommendation(rec)
	require.Error(t, err)
	require.Contains(t, err.Error(), "promotion fields must be empty")
}

func newRecommendationTestDB(t *testing.T) *ProjectDB {
	t.Helper()

	pdb, err := OpenProjectInMemory()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pdb.Close()
	})

	workflow := &Workflow{ID: "wf-recommendation", Name: "Recommendation Workflow"}
	require.NoError(t, pdb.SaveWorkflow(workflow))

	task := &Task{
		ID:           "TASK-001",
		Title:        "Recommendation Source Task",
		WorkflowID:   workflow.ID,
		Status:       "running",
		InitiativeID: "INIT-001",
	}
	require.NoError(t, pdb.SaveTask(task))

	require.NoError(t, pdb.SaveInitiative(&Initiative{
		ID:        "INIT-001",
		Title:     "Recommendation Initiative",
		Status:    "active",
		CreatedAt: time.Date(2026, time.March, 9, 17, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, time.March, 9, 17, 0, 0, 0, time.UTC),
	}))

	taskID := task.ID
	run := &WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  workflow.ID,
		ContextType: "task",
		TaskID:      &taskID,
		Status:      "running",
	}
	require.NoError(t, pdb.SaveWorkflowRun(run))

	thread := &Thread{
		Title:  "Recommendation discussion",
		TaskID: task.ID,
	}
	require.NoError(t, pdb.CreateThread(thread))

	return pdb
}

func newTestRecommendation() *Recommendation {
	return &Recommendation{
		Kind:           RecommendationKindCleanup,
		Status:         RecommendationStatusPending,
		Title:          "Clean up duplicate polling",
		Summary:        "Two polling loops are doing the same work.",
		ProposedAction: "Remove the legacy loop after validating the new path.",
		Evidence:       "Both loops hit the same endpoint every 5 seconds.",
		SourceTaskID:   "TASK-001",
		SourceRunID:    "RUN-001",
		SourceThreadID: "THR-001",
		DedupeKey:      "cleanup:task-001:duplicate-polling",
	}
}
