package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
	"github.com/randalmurphal/orc/internal/workflow"
)

func TestCompletionRecommendation(t *testing.T) {
	backend, taskItem, workflowID := setupCompletionRecommendationRunFixture(t)
	publisher := &testPublishHelper{}
	mockTurn := NewMockTurnExecutor(reviewFindingsJSON(false, true))

	var we *WorkflowExecutor
	we = NewWorkflowExecutor(
		backend,
		backend.DB(),
		testGlobalDBFrom(backend),
		&config.Config{},
		t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowPublisher(publisher),
		WithWorkflowCompletionRecommendationGenerator(func(ctx context.Context, run *db.WorkflowRun, generatedTask *orcv1.Task) (*CompletionRecommendationResult, error) {
			if generatedTask.GetStatus() != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
				t.Fatalf("task status at generator call = %s, want completed", generatedTask.GetStatus())
			}

			candidate := finalizeRecommendationCandidate(generatedTask.GetId(), controlplane.RecommendationCandidate{
				Kind:           db.RecommendationKindFollowUp,
				Title:          "Capture post-completion follow-up",
				Summary:        "Completion generated a deterministic follow-up recommendation.",
				ProposedAction: "Keep the recommendation in the inbox until an operator decides what to do.",
				Evidence:       buildRecommendationEvidence(run, "review", "Generator override executed from completion path.", parseChangedFilesForRecommendations(generatedTask)),
				Confidence:     "high",
			})

			persisted, dedupeSuppressed, err := we.persistRecommendationCandidates(run, generatedTask, []controlplane.RecommendationCandidate{candidate})
			return &CompletionRecommendationResult{
				Generated:        1,
				DedupeSuppressed: dedupeSuppressed,
				Persisted:        persisted,
			}, err
		}),
	)

	result, err := we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      taskItem.GetId(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("Run() success = false, want true")
	}

	recommendations, err := backend.LoadAllRecommendations()
	if err != nil {
		t.Fatalf("LoadAllRecommendations() error = %v", err)
	}
	if len(recommendations) != 1 {
		t.Fatalf("recommendation count = %d, want 1", len(recommendations))
	}

	recommendation := recommendations[0]
	if recommendation.GetSourceTaskId() != taskItem.GetId() {
		t.Fatalf("SourceTaskId = %q, want %q", recommendation.GetSourceTaskId(), taskItem.GetId())
	}
	if recommendation.GetSourceRunId() != result.RunID {
		t.Fatalf("SourceRunId = %q, want %q", recommendation.GetSourceRunId(), result.RunID)
	}
	if recommendation.GetKind() != orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP {
		t.Fatalf("Kind = %s, want follow_up", recommendation.GetKind())
	}
	if !strings.HasPrefix(recommendation.GetDedupeKey(), "task:"+taskItem.GetId()+":follow_up:") {
		t.Fatalf("DedupeKey = %q, want task-scoped follow_up key", recommendation.GetDedupeKey())
	}

	events := filterEventsByType(publisher.events, "recommendation_created")
	if len(events) != 1 {
		t.Fatalf("recommendation_created event count = %d, want 1", len(events))
	}
}

func TestRecommendationCandidateSchema(t *testing.T) {
	_, taskItem, run := setupCompletionRecommendationContext(t)

	outputs := []*storage.PhaseOutputInfo{
		{
			WorkflowRunID:   run.ID,
			PhaseTemplateID: "review",
			Content:         reviewFindingsJSON(false, true),
		},
		{
			WorkflowRunID:   run.ID,
			PhaseTemplateID: "implement",
			Content: `{
				"status": "complete",
				"summary": "Implementation landed with follow-up notes.",
				"verification": {
					"tests": {"status": "SKIPPED", "command": "go test ./...", "evidence": "Skipped in fixture."},
					"success_criteria": [{"id": "SC-9", "status": "FAIL", "evidence": "Still uncovered in fixture."}],
					"build": {"status": "PASS"},
					"linting": {"status": "PASS"},
					"wiring": {"status": "PASS"}
				},
				"pre_existing_issues": ["Legacy flaky test still exists."]
			}`,
		},
	}

	candidates := buildCompletionRecommendationCandidates(taskItem, run, outputs)
	if len(candidates) == 0 {
		t.Fatal("buildCompletionRecommendationCandidates() returned no candidates")
	}

	validKinds := map[string]bool{
		db.RecommendationKindCleanup:         true,
		db.RecommendationKindRisk:            true,
		db.RecommendationKindFollowUp:        true,
		db.RecommendationKindDecisionRequest: true,
	}
	for _, candidate := range candidates {
		if !validKinds[candidate.Kind] {
			t.Fatalf("candidate kind = %q, want valid recommendation kind", candidate.Kind)
		}
		if strings.TrimSpace(candidate.Title) == "" {
			t.Fatalf("candidate title is empty: %#v", candidate)
		}
		if strings.TrimSpace(candidate.Summary) == "" {
			t.Fatalf("candidate summary is empty: %#v", candidate)
		}
		if strings.TrimSpace(candidate.Evidence) == "" {
			t.Fatalf("candidate evidence is empty: %#v", candidate)
		}
		if strings.TrimSpace(candidate.DedupeKey) == "" {
			t.Fatalf("candidate dedupe key is empty: %#v", candidate)
		}
		if strings.TrimSpace(candidate.ProposedAction) == "" {
			t.Fatalf("candidate proposed action is empty: %#v", candidate)
		}
		if !strings.HasPrefix(candidate.DedupeKey, "task:"+taskItem.GetId()+":"+candidate.Kind+":") {
			t.Fatalf("candidate dedupe key = %q, want task-scoped prefix", candidate.DedupeKey)
		}
	}
}

func TestRecommendationDedupe(t *testing.T) {
	backend, taskItem, run := setupCompletionRecommendationContext(t)
	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir())

	candidate := finalizeRecommendationCandidate(taskItem.GetId(), controlplane.RecommendationCandidate{
		Kind:           db.RecommendationKindCleanup,
		Title:          "Remove duplicate completion work",
		Summary:        "The same cleanup recommendation should not persist twice.",
		ProposedAction: "Suppress the duplicate by dedupe key.",
		Evidence:       buildRecommendationEvidence(run, "implement", "Dedupe fixture.", parseChangedFilesForRecommendations(taskItem)),
		Confidence:     "high",
	})

	firstPersisted, firstSuppressed, err := we.persistRecommendationCandidates(run, taskItem, []controlplane.RecommendationCandidate{candidate})
	if err != nil {
		t.Fatalf("first persist error = %v", err)
	}
	if len(firstPersisted) != 1 {
		t.Fatalf("first persist count = %d, want 1", len(firstPersisted))
	}
	if firstSuppressed != 0 {
		t.Fatalf("first dedupe suppressed = %d, want 0", firstSuppressed)
	}

	secondPersisted, secondSuppressed, err := we.persistRecommendationCandidates(run, taskItem, []controlplane.RecommendationCandidate{candidate})
	if err != nil {
		t.Fatalf("second persist error = %v", err)
	}
	if len(secondPersisted) != 0 {
		t.Fatalf("second persist count = %d, want 0", len(secondPersisted))
	}
	if secondSuppressed != 1 {
		t.Fatalf("second dedupe suppressed = %d, want 1", secondSuppressed)
	}

	recommendations, err := backend.LoadAllRecommendations()
	if err != nil {
		t.Fatalf("LoadAllRecommendations() error = %v", err)
	}
	if len(recommendations) != 1 {
		t.Fatalf("stored recommendation count = %d, want 1", len(recommendations))
	}
}

func TestRecommendationLowSignal(t *testing.T) {
	candidates := []controlplane.RecommendationCandidate{
		{
			Kind:           db.RecommendationKindCleanup,
			Title:          "",
			Summary:        "Missing title should be filtered.",
			ProposedAction: "Ignore this fixture.",
			Evidence:       "Fixture.",
			DedupeKey:      "task:TASK-001:cleanup:missing-title",
			Confidence:     "high",
		},
		{
			Kind:           db.RecommendationKindRisk,
			Title:          "Low confidence fixture",
			Summary:        "Low confidence should be filtered.",
			ProposedAction: "Ignore this fixture.",
			Evidence:       "Fixture.",
			DedupeKey:      "task:TASK-001:risk:low-confidence",
			Confidence:     "low",
		},
		{
			Kind:           db.RecommendationKindFollowUp,
			Title:          "Valid fixture",
			Summary:        "This candidate should survive filtering.",
			ProposedAction: "Persist the remaining candidate.",
			Evidence:       "Fixture.",
			DedupeKey:      "task:TASK-001:follow_up:valid",
			Confidence:     "medium",
		},
	}

	filtered, filteredCount := filterLowSignalCandidates(candidates)
	if filteredCount != 2 {
		t.Fatalf("filtered count = %d, want 2", filteredCount)
	}
	if len(filtered) != 1 {
		t.Fatalf("filtered candidates len = %d, want 1", len(filtered))
	}
	if filtered[0].Title != "Valid fixture" {
		t.Fatalf("remaining candidate title = %q, want Valid fixture", filtered[0].Title)
	}
}

func TestRecommendationGenerationFailure(t *testing.T) {
	baseBackend, taskItem, workflowID := setupCompletionRecommendationRunFixture(t)
	backend := &failingRecommendationSaveBackend{
		Backend: baseBackend,
		err:     fmt.Errorf("save exploded"),
	}
	mockTurn := NewMockTurnExecutor(reviewFindingsJSON(false, false))
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	we := NewWorkflowExecutor(
		backend,
		baseBackend.DB(),
		testGlobalDBFrom(baseBackend),
		&config.Config{},
		t.TempDir(),
		WithWorkflowTurnExecutor(mockTurn),
		WithWorkflowLogger(logger),
	)

	result, err := we.Run(context.Background(), workflowID, WorkflowRunOptions{
		ContextType: ContextTask,
		TaskID:      taskItem.GetId(),
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("Run() success = false, want true")
	}

	reloaded, err := baseBackend.LoadTask(taskItem.GetId())
	if err != nil {
		t.Fatalf("LoadTask() error = %v", err)
	}
	if reloaded.GetStatus() != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
		t.Fatalf("task status = %s, want completed", reloaded.GetStatus())
	}
	if !strings.Contains(logBuf.String(), "failed to generate completion recommendations") {
		t.Fatalf("warn log missing completion recommendation failure: %s", logBuf.String())
	}
}

func TestCompletionRecommendationsVariable(t *testing.T) {
	backend, taskItem, run := setupCompletionRecommendationContext(t)
	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir())
	otherWorkflowID := "completion-recommendation-wf"
	otherTask := task.NewProtoTask("TASK-OTHER", "Other task")
	otherTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	otherTask.WorkflowId = &otherWorkflowID
	if err := backend.SaveTask(otherTask); err != nil {
		t.Fatalf("SaveTask(other) error = %v", err)
	}
	otherTaskID := otherTask.GetId()
	if err := backend.SaveWorkflowRun(&db.WorkflowRun{
		ID:          "RUN-OTHER",
		WorkflowID:  otherWorkflowID,
		ContextType: string(ContextTask),
		TaskID:      &otherTaskID,
		Status:      string(workflow.RunStatusRunning),
	}); err != nil {
		t.Fatalf("SaveWorkflowRun(other) error = %v", err)
	}
	taskID := taskItem.GetId()
	if err := backend.SaveWorkflowRun(&db.WorkflowRun{
		ID:          "RUN-STALE",
		WorkflowID:  otherWorkflowID,
		ContextType: string(ContextTask),
		TaskID:      &taskID,
		Status:      string(workflow.RunStatusCompleted),
	}); err != nil {
		t.Fatalf("SaveWorkflowRun(stale) error = %v", err)
	}

	recommendations := []*orcv1.Recommendation{
		{
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP,
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Title:          "Cleanup the completion path",
			Summary:        "Current task generated a cleanup follow-up.",
			ProposedAction: "Track the cleanup as explicit follow-up work.",
			Evidence:       buildRecommendationEvidence(run, "review", "Current task fixture.", parseChangedFilesForRecommendations(taskItem)),
			SourceTaskId:   taskItem.GetId(),
			SourceRunId:    run.ID,
			DedupeKey:      "task:TASK-001:cleanup:current",
		},
		{
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK,
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Title:          "Ignore other task risk",
			Summary:        "Other tasks must not leak into COMPLETION_RECOMMENDATIONS.",
			ProposedAction: "Ignore this fixture.",
			Evidence:       "Other task fixture.",
			SourceTaskId:   otherTask.GetId(),
			SourceRunId:    "RUN-OTHER",
			DedupeKey:      "task:TASK-OTHER:risk:other",
		},
		{
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP,
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Title:          "Ignore prior run follow-up",
			Summary:        "Older runs for the same task must not leak into COMPLETION_RECOMMENDATIONS.",
			ProposedAction: "Ignore this stale fixture.",
			Evidence:       "Prior run fixture.",
			SourceTaskId:   taskItem.GetId(),
			SourceRunId:    "RUN-STALE",
			DedupeKey:      "task:TASK-001:follow_up:stale",
		},
	}
	for _, recommendation := range recommendations {
		if err := backend.SaveRecommendation(recommendation); err != nil {
			t.Fatalf("SaveRecommendation(%q) error = %v", recommendation.GetTitle(), err)
		}
	}

	rctx := &variable.ResolutionContext{
		WorkflowRunID: run.ID,
		TaskID:        taskItem.GetId(),
		TaskTitle:     taskItem.GetTitle(),
	}
	if err := we.populateControlPlaneContext(rctx, "docs", taskItem, controlPlaneVariableUsage{
		CompletionRecommendations: true,
	}); err != nil {
		t.Fatalf("populateControlPlaneContext() error = %v", err)
	}

	resolver := variable.NewResolver(t.TempDir())
	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	expected := controlplane.FormatRecommendationSummary([]controlplane.RecommendationCandidate{
		{
			Kind:           "cleanup",
			Title:          "Cleanup the completion path",
			Summary:        "Current task generated a cleanup follow-up.",
			ProposedAction: "Track the cleanup as explicit follow-up work.",
			Evidence:       buildRecommendationEvidence(run, "review", "Current task fixture.", parseChangedFilesForRecommendations(taskItem)),
			DedupeKey:      "task:TASK-001:cleanup:current",
		},
	})
	if vars["COMPLETION_RECOMMENDATIONS"] != expected {
		t.Fatalf("COMPLETION_RECOMMENDATIONS = %q, want %q", vars["COMPLETION_RECOMMENDATIONS"], expected)
	}

	usage := detectControlPlaneVariableUsage("{{COMPLETION_RECOMMENDATIONS}}")
	if !usage.CompletionRecommendations {
		t.Fatal("CompletionRecommendations usage = false, want true")
	}
}

func TestRecommendationGolden(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		_, taskItem, run := setupCompletionRecommendationContext(t)
		outputs := []*storage.PhaseOutputInfo{{
			WorkflowRunID:   run.ID,
			PhaseTemplateID: "review",
			Content:         reviewFindingsWithoutSignalsJSON(),
		}}

		candidates := buildCompletionRecommendationCandidates(taskItem, run, outputs)
		filtered, filteredCount := filterLowSignalCandidates(candidates)
		if len(candidates) != 0 {
			t.Fatalf("generated candidates = %d, want 0", len(candidates))
		}
		if len(filtered) != 0 {
			t.Fatalf("filtered candidates len = %d, want 0", len(filtered))
		}
		if filteredCount != 0 {
			t.Fatalf("filtered count = %d, want 0", filteredCount)
		}
	})

	t.Run("accepted", func(t *testing.T) {
		backend, taskItem, run := setupCompletionRecommendationContext(t)
		we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir())
		outputs := []*storage.PhaseOutputInfo{{
			WorkflowRunID:   run.ID,
			PhaseTemplateID: "review",
			Content:         reviewFindingsJSON(false, false),
		}}

		candidates := buildCompletionRecommendationCandidates(taskItem, run, outputs)
		filtered, filteredCount := filterLowSignalCandidates(candidates)
		persisted, dedupeSuppressed, err := we.persistRecommendationCandidates(run, taskItem, filtered)
		if err != nil {
			t.Fatalf("persistRecommendationCandidates() error = %v", err)
		}
		if len(candidates) != 1 || len(persisted) != 1 || filteredCount != 0 || dedupeSuppressed != 0 {
			t.Fatalf("accepted case = generated:%d persisted:%d filtered:%d dedupe:%d, want 1/1/0/0", len(candidates), len(persisted), filteredCount, dedupeSuppressed)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		backend, taskItem, run := setupCompletionRecommendationContext(t)
		we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir())
		outputs := []*storage.PhaseOutputInfo{{
			WorkflowRunID:   run.ID,
			PhaseTemplateID: "review",
			Content:         reviewFindingsJSON(false, false),
		}}

		candidates := buildCompletionRecommendationCandidates(taskItem, run, outputs)
		filtered, _ := filterLowSignalCandidates(candidates)
		if _, _, err := we.persistRecommendationCandidates(run, taskItem, filtered); err != nil {
			t.Fatalf("first persist error = %v", err)
		}
		persisted, dedupeSuppressed, err := we.persistRecommendationCandidates(run, taskItem, filtered)
		if err != nil {
			t.Fatalf("second persist error = %v", err)
		}
		if len(persisted) != 0 || dedupeSuppressed != 1 {
			t.Fatalf("duplicate case = persisted:%d dedupe:%d, want 0/1", len(persisted), dedupeSuppressed)
		}
	})

	t.Run("low-signal", func(t *testing.T) {
		backend, taskItem, run := setupCompletionRecommendationContext(t)
		we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), &config.Config{}, t.TempDir())
		candidates := []controlplane.RecommendationCandidate{
			finalizeRecommendationCandidate(taskItem.GetId(), controlplane.RecommendationCandidate{
				Kind:           db.RecommendationKindCleanup,
				Title:          "",
				Summary:        "Missing title",
				ProposedAction: "Ignore",
				Evidence:       "Fixture.",
				Confidence:     "high",
			}),
			finalizeRecommendationCandidate(taskItem.GetId(), controlplane.RecommendationCandidate{
				Kind:           db.RecommendationKindRisk,
				Title:          "Low confidence",
				Summary:        "Should be filtered",
				ProposedAction: "Ignore",
				Evidence:       "Fixture.",
				Confidence:     "low",
			}),
			finalizeRecommendationCandidate(taskItem.GetId(), controlplane.RecommendationCandidate{
				Kind:           db.RecommendationKindFollowUp,
				Title:          "Valid completion recommendation",
				Summary:        "This candidate survives filtering.",
				ProposedAction: "Persist the valid recommendation.",
				Evidence:       buildRecommendationEvidence(run, "implement", "Low-signal fixture.", parseChangedFilesForRecommendations(taskItem)),
				Confidence:     "medium",
			}),
		}

		filtered, filteredCount := filterLowSignalCandidates(candidates)
		persisted, dedupeSuppressed, err := we.persistRecommendationCandidates(run, taskItem, filtered)
		if err != nil {
			t.Fatalf("persistRecommendationCandidates() error = %v", err)
		}
		if filteredCount != 2 || len(persisted) != 1 || dedupeSuppressed != 0 {
			t.Fatalf("low-signal case = filtered:%d persisted:%d dedupe:%d, want 2/1/0", filteredCount, len(persisted), dedupeSuppressed)
		}
	})
}

type failingRecommendationSaveBackend struct {
	storage.Backend
	err error
}

func (f *failingRecommendationSaveBackend) SaveRecommendation(*orcv1.Recommendation) error {
	return f.err
}

func setupCompletionRecommendationContext(t *testing.T) (*storage.DatabaseBackend, *orcv1.Task, *db.WorkflowRun) {
	t.Helper()

	backend := storage.NewTestBackend(t)
	workflowID := "completion-recommendation-wf"
	createTestWorkflow(t, backend, workflowID)

	taskItem := task.NewProtoTask("TASK-001", "Completion recommendation fixture")
	taskItem.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	taskItem.Metadata = map[string]string{
		"changed_files": "internal/executor/workflow_executor.go, internal/executor/workflow_context.go",
	}
	taskItem.WorkflowId = &workflowID
	if err := backend.SaveTask(taskItem); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	taskID := taskItem.GetId()
	run := &db.WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  workflowID,
		ContextType: string(ContextTask),
		TaskID:      &taskID,
		Status:      string(workflow.RunStatusRunning),
	}
	if err := backend.SaveWorkflowRun(run); err != nil {
		t.Fatalf("SaveWorkflowRun() error = %v", err)
	}

	return backend, taskItem, run
}

func setupCompletionRecommendationRunFixture(t *testing.T) (*storage.DatabaseBackend, *orcv1.Task, string) {
	t.Helper()

	backend, taskItem, _ := setupCompletionRecommendationContext(t)
	gdb := testGlobalDBFrom(backend)
	workflowID := "completion-recommendation-wf"

	tmpl := &db.PhaseTemplate{
		ID:            "review",
		Name:          "review",
		PromptSource:  "db",
		PromptContent: "Review completion recommendations",
		OutputVarName: "REVIEW_OUTPUT",
	}
	if err := gdb.SavePhaseTemplate(tmpl); err != nil {
		t.Fatalf("SavePhaseTemplate() error = %v", err)
	}
	if err := gdb.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      workflowID,
		PhaseTemplateID: "review",
		Sequence:        1,
	}); err != nil {
		t.Fatalf("SaveWorkflowPhase() error = %v", err)
	}

	return backend, taskItem, workflowID
}

func reviewFindingsJSON(includeNeedsChanges bool, includeQuestion bool) string {
	questionSection := ""
	if includeQuestion {
		questionSection = `,"questions":["Does this skipped verification need an operator decision?"]`
	}
	return fmt.Sprintf(`{
		"needs_changes": %t,
		"round": 1,
		"summary": "Review found follow-up work worth tracking.",
		"issues": [{
			"severity": "high",
			"file": "internal/executor/workflow_executor.go",
			"line": 1567,
			"description": "Completion should publish a follow-up recommendation.",
			"suggestion": "Persist a task-scoped recommendation after completion."
		}]%s
	}`, includeNeedsChanges, questionSection)
}

func reviewFindingsWithoutSignalsJSON() string {
	return `{
		"needs_changes": false,
		"round": 1,
		"summary": "Review found no actionable follow-up work.",
		"issues": []
	}`
}

func filterEventsByType(items []events.Event, eventType string) []events.Event {
	filtered := make([]events.Event, 0)
	for _, event := range items {
		if string(event.Type) == eventType {
			filtered = append(filtered, event)
		}
	}
	return filtered
}
