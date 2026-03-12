package db

import (
	"context"
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
	require.Equal(t, rec.PromotedToType, loaded.PromotedToType)
	require.Equal(t, rec.PromotedToID, loaded.PromotedToID)
	require.Equal(t, rec.PromotedBy, loaded.PromotedBy)
	require.NotNil(t, loaded.PromotedAt)

	list, err := pdb.ListRecommendations(RecommendationListOpts{Status: RecommendationStatusPending})
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, rec.ID, list[0].ID)

	require.NoError(t, pdb.DeleteRecommendation(rec.ID))

	deleted, err := pdb.GetRecommendation(rec.ID)
	require.NoError(t, err)
	require.Nil(t, deleted)
}

func TestRecommendationDelete_RemovesSyntheticThreadLink(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)
	require.NoError(t, pdb.SaveWorkflow(&Workflow{ID: "wf", Name: "wf"}))
	require.NoError(t, pdb.SaveTask(&Task{ID: "TASK-001", Title: "task", WorkflowID: "wf", Status: "planned"}))

	thread := &Thread{
		Title:  "Recommendation thread",
		TaskID: "TASK-001",
	}
	require.NoError(t, pdb.CreateThread(thread))

	draft := &ThreadRecommendationDraft{
		ThreadID:       thread.ID,
		Kind:           RecommendationKindFollowUp,
		Title:          "Follow up",
		Summary:        "Need a recommendation link in the thread view.",
		ProposedAction: "Promote the draft.",
		Evidence:       "Thread links should mirror recommendations through one source of truth.",
	}
	require.NoError(t, pdb.CreateThreadRecommendationDraft(draft))

	_, rec, err := pdb.PromoteThreadRecommendationDraft(context.Background(), thread.ID, draft.ID, "operator")
	require.NoError(t, err)

	gotThread, err := pdb.GetThread(thread.ID)
	require.NoError(t, err)
	require.Len(t, gotThread.Links, 2)
	require.Equal(t, ThreadLinkTypeRecommendation, gotThread.Links[1].LinkType)

	require.NoError(t, pdb.DeleteRecommendation(rec.ID))

	gotThread, err = pdb.GetThread(thread.ID)
	require.NoError(t, err)
	require.Len(t, gotThread.Links, 1)
	require.Equal(t, ThreadLinkTypeTask, gotThread.Links[0].LinkType)
}

func TestRecommendationCreate_AllowsThreadOnlyProvenance(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)

	thread := &Thread{Title: "Generic discussion"}
	require.NoError(t, pdb.CreateThread(thread))

	rec := &Recommendation{
		Kind:           RecommendationKindFollowUp,
		Status:         RecommendationStatusPending,
		Title:          "Follow up on generic thread",
		Summary:        "Project-scoped threads still need promotable recommendations.",
		ProposedAction: "Keep thread-only provenance when no task/run exists.",
		Evidence:       "Sidebar-created threads do not carry execution provenance.",
		SourceThreadID: thread.ID,
		DedupeKey:      "follow-up:generic-thread:promotion",
	}

	require.NoError(t, pdb.CreateRecommendation(rec))
	require.NotEmpty(t, rec.ID)
	require.Empty(t, rec.SourceTaskID)
	require.Empty(t, rec.SourceRunID)
	require.Equal(t, thread.ID, rec.SourceThreadID)

	loaded, err := pdb.GetRecommendation(rec.ID)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.Empty(t, loaded.SourceTaskID)
	require.Empty(t, loaded.SourceRunID)
	require.Equal(t, thread.ID, loaded.SourceThreadID)
}

func TestRecommendationCreate_AllowsTaskWithoutRunProvenance(t *testing.T) {
	t.Parallel()

	pdb := newRecommendationTestDB(t)

	require.NoError(t, pdb.SaveWorkflow(&Workflow{ID: "wf", Name: "wf"}))
	require.NoError(t, pdb.SaveTask(&Task{ID: "TASK-001", Title: "task", WorkflowID: "wf", Status: "planned"}))

	rec := &Recommendation{
		Kind:           RecommendationKindFollowUp,
		Status:         RecommendationStatusPending,
		Title:          "Follow up on task thread",
		Summary:        "Task-linked threads without runs still need valid provenance.",
		ProposedAction: "Allow task provenance without requiring a workflow run.",
		Evidence:       "A manual thread linked to a task has no source run yet.",
		SourceTaskID:   "TASK-001",
		SourceThreadID: "THR-001",
		DedupeKey:      "follow-up:task-thread:no-run",
	}

	require.NoError(t, pdb.CreateRecommendation(rec))
	require.Equal(t, "TASK-001", rec.SourceTaskID)
	require.Empty(t, rec.SourceRunID)
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
		ID:         "TASK-001",
		Title:      "Recommendation Source Task",
		WorkflowID: workflow.ID,
		Status:     "running",
	}
	require.NoError(t, pdb.SaveTask(task))

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
	promotedAt := time.Date(2026, time.March, 9, 18, 0, 0, 0, time.UTC)
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
		PromotedToType: "task",
		PromotedToID:   "TASK-002",
		PromotedBy:     "operator",
		PromotedAt:     &promotedAt,
	}
}
