package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestArtifactIndex_AcceptedRecommendation(t *testing.T) {
	t.Parallel()

	pdb := NewTestProjectDB(t)
	seedArtifactIndexContext(t, pdb)
	entry := &ArtifactIndexEntry{
		Kind:           ArtifactKindAcceptedRecommendation,
		Title:          "Accept rate-limit cleanup",
		Content:        "Summary: Consolidate rate-limit guards.\nEvidence: Two code paths diverged.",
		DedupeKey:      "cleanup:rate-limit:guards",
		SourceTaskID:   "TASK-001",
		SourceRunID:    "RUN-001",
		SourceThreadID: "THR-001",
	}

	require.NoError(t, pdb.SaveArtifactIndexEntry(entry))
	require.NotZero(t, entry.ID)

	results, err := pdb.QueryArtifactIndex(ArtifactIndexQueryOpts{
		Kind:   ArtifactKindAcceptedRecommendation,
		Search: "rate-limit guards",
		Limit:  10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, entry.DedupeKey, results[0].DedupeKey)
	require.Equal(t, "TASK-001", results[0].SourceTaskID)
	require.Equal(t, "RUN-001", results[0].SourceRunID)
	require.Equal(t, "THR-001", results[0].SourceThreadID)
	require.Contains(t, results[0].Content, "Evidence")
}

func TestArtifactIndex_InitiativeDecision(t *testing.T) {
	t.Parallel()

	pdb := NewTestProjectDB(t)
	seedArtifactIndexContext(t, pdb)
	entry := &ArtifactIndexEntry{
		Kind:         ArtifactKindInitiativeDecision,
		Title:        "Gate rollout behind feature flag",
		Content:      "Decision: Keep the rollout behind a feature flag.\nRationale: Canary latency regressed.",
		DedupeKey:    "initiative_decision:INIT-001:DEC-001",
		InitiativeID: "INIT-001",
	}

	require.NoError(t, pdb.SaveArtifactIndexEntry(entry))

	results, err := pdb.QueryArtifactIndex(ArtifactIndexQueryOpts{
		Kind:         ArtifactKindInitiativeDecision,
		InitiativeID: "INIT-001",
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "INIT-001", results[0].InitiativeID)
	require.Contains(t, results[0].Content, "Rationale")
}

func TestArtifactIndex_PromotedDraft(t *testing.T) {
	t.Parallel()

	pdb := NewTestProjectDB(t)
	seedArtifactIndexContext(t, pdb)
	entry := &ArtifactIndexEntry{
		Kind:           ArtifactKindPromotedDraft,
		Title:          "Promote workspace draft",
		Content:        "Summary: Promote the workspace draft.\nEvidence: Operator accepted the proposal.",
		DedupeKey:      "thread:thr-001:follow_up:promote-workspace-draft",
		SourceThreadID: "THR-001",
	}

	require.NoError(t, pdb.SaveArtifactIndexEntry(entry))

	results, err := pdb.QueryArtifactIndex(ArtifactIndexQueryOpts{
		Kind:           ArtifactKindPromotedDraft,
		SourceThreadID: "THR-001",
		Limit:          10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, "THR-001", results[0].SourceThreadID)
}

func TestArtifactIndex_TaskOutcome(t *testing.T) {
	t.Parallel()

	pdb := NewTestProjectDB(t)
	seedArtifactIndexContext(t, pdb)
	first := &ArtifactIndexEntry{
		Kind:         ArtifactKindTaskOutcome,
		Title:        "High-severity review finding",
		Content:      "Finding: Missing nil guard.\nSeverity: high",
		DedupeKey:    "task_outcome:TASK-001:review:1:0",
		InitiativeID: "INIT-001",
		SourceTaskID: "TASK-001",
		SourceRunID:  "RUN-001",
	}
	second := &ArtifactIndexEntry{
		Kind:         ArtifactKindTaskOutcome,
		Title:        "Initiative note",
		Content:      "Note type: handoff\nContent: Keep the migration ordering stable.",
		DedupeKey:    "task_outcome:TASK-001:note:NOTE-001",
		InitiativeID: "INIT-001",
		SourceTaskID: "TASK-001",
		SourceRunID:  "RUN-001",
	}

	require.NoError(t, pdb.SaveArtifactIndexEntry(first))
	require.NoError(t, pdb.SaveArtifactIndexEntry(second))

	results, err := pdb.GetRecentArtifacts(RecentArtifactOpts{
		InitiativeID: "INIT-001",
		SourceTaskID: "TASK-001",
		Limit:        10,
	})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.Equal(t, ArtifactKindTaskOutcome, results[0].Kind)
	require.Equal(t, "TASK-001", results[0].SourceTaskID)
}

func TestArtifactIndex_DedupeQuery(t *testing.T) {
	t.Parallel()

	pdb := NewTestProjectDB(t)
	seedArtifactIndexContext(t, pdb)
	entry := &ArtifactIndexEntry{
		Kind:         ArtifactKindAcceptedRecommendation,
		Title:        "Accepted cleanup",
		Content:      "Summary: Remove duplicate polling.\nEvidence: This was already accepted.",
		DedupeKey:    "cleanup:duplicate-polling",
		InitiativeID: "INIT-001",
		SourceTaskID: "TASK-001",
	}

	require.NoError(t, pdb.SaveArtifactIndexEntry(entry))

	results, err := pdb.QueryArtifactIndexByDedupeKey("cleanup:duplicate-polling")
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, ArtifactKindAcceptedRecommendation, results[0].Kind)

	err = pdb.CreateRecommendation(&Recommendation{
		Kind:           RecommendationKindCleanup,
		Status:         RecommendationStatusPending,
		Title:          "Recreated cleanup",
		Summary:        "This recommendation should be suppressed by the artifact index.",
		ProposedAction: "Do not recreate already accepted work.",
		Evidence:       "The artifact index already contains the accepted recommendation.",
		SourceTaskID:   "TASK-001",
		SourceRunID:    "RUN-001",
		SourceThreadID: "THR-001",
		DedupeKey:      "cleanup:duplicate-polling",
	})
	require.Error(t, err)
	require.ErrorIs(t, err, ErrRecommendationConflict)
	require.Contains(t, err.Error(), "duplicate")
}

func seedArtifactIndexContext(t *testing.T, pdb *ProjectDB) {
	t.Helper()

	require.NoError(t, pdb.SaveInitiative(&Initiative{
		ID:     "INIT-001",
		Title:  "Artifact Index",
		Status: "active",
	}))
	require.NoError(t, pdb.SaveWorkflow(&Workflow{ID: "wf-artifact", Name: "artifact"}))
	require.NoError(t, pdb.SaveTask(&Task{
		ID:           "TASK-001",
		Title:        "artifact task",
		WorkflowID:   "wf-artifact",
		Status:       "completed",
		StateStatus:  "completed",
		InitiativeID: "INIT-001",
	}))

	taskID := "TASK-001"
	require.NoError(t, pdb.SaveWorkflowRun(&WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "wf-artifact",
		ContextType: "task",
		TaskID:      &taskID,
		Prompt:      "prompt",
		Status:      "completed",
		CreatedAt:   time.Now(),
	}))

	thread := &Thread{
		Title:        "artifact thread",
		TaskID:       "TASK-001",
		InitiativeID: "INIT-001",
	}
	require.NoError(t, pdb.CreateThread(thread))
	require.Equal(t, "THR-001", thread.ID)
}
