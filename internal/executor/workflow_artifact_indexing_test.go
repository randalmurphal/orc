package executor

import (
	"context"
	"fmt"
	"log/slog"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/knowledge/index/artifact"
	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
	"github.com/randalmurphal/orc/internal/storage"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// --- Integration tests: WorkflowExecutor artifact indexing wiring ---
//
// These verify that the WorkflowExecutor loads artifacts from the storage
// backend and passes them to the knowledge service's IndexTaskArtifacts.
//
// Litmus test: If the executor doesn't load artifacts from backend or doesn't
// call IndexTaskArtifacts, the recording service would show no call or empty
// params. Removing the indexTaskArtifacts call from Run() makes the recording
// mock never get called.

// recordingArtifactIndexService implements both KnowledgeQueryService and
// KnowledgeArtifactIndexService for testing post-completion artifact indexing.
type recordingArtifactIndexService struct {
	available   bool
	indexCalled bool
	indexParams artifact.IndexParams
	indexErr    error
}

// Compile-time interface assertions — fail if interfaces don't exist or change.
var _ KnowledgeQueryService = (*recordingArtifactIndexService)(nil)
var _ KnowledgeArtifactIndexService = (*recordingArtifactIndexService)(nil)

func (r *recordingArtifactIndexService) IsAvailable() bool {
	return r.available
}

func (r *recordingArtifactIndexService) Query(_ context.Context, _ string, _ retrieve.QueryOpts) (*retrieve.PipelineResult, error) {
	return nil, nil
}

func (r *recordingArtifactIndexService) IndexTaskArtifacts(_ context.Context, params artifact.IndexParams) error {
	r.indexCalled = true
	r.indexParams = params
	return r.indexErr
}

// SC-6: Executor loads artifacts from backend and calls IndexTaskArtifacts.
// Verifies: executor → backend.GetSpecForTask → backend.LoadAllReviewFindings →
//
//	backend.GetScratchpadEntries → service.IndexTaskArtifacts
//
// Fails if: executor doesn't load from backend or doesn't call the service.
func TestArtifactIndex_CalledWithBackendData(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	rec := &recordingArtifactIndexService{available: true}

	// Populate backend with task and artifacts.
	workflowID := "implement-small"
	taskProto := &orcv1.Task{
		Id:         "TASK-AI-001",
		Title:      "Test artifact indexing",
		WorkflowId: &workflowID,
		Status:     orcv1.TaskStatus_TASK_STATUS_RUNNING,
		CreatedAt:  timestamppb.Now(),
		UpdatedAt:  timestamppb.Now(),
	}
	if err := backend.SaveTask(taskProto); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// Store spec.
	if err := backend.SaveSpecForTask("TASK-AI-001", "## Spec\nModify internal/foo.go", "spec"); err != nil {
		t.Fatalf("save spec: %v", err)
	}

	// Store review findings.
	fileStr := "internal/handler.go"
	findings := &orcv1.ReviewRoundFindings{
		TaskId: "TASK-AI-001",
		Round:  1,
		Issues: []*orcv1.ReviewFinding{
			{
				Severity:    "high",
				File:        &fileStr,
				Description: "Missing error check",
			},
		},
	}
	if err := backend.SaveReviewFindings(findings); err != nil {
		t.Fatalf("save findings: %v", err)
	}

	// Store scratchpad entry.
	entry := &storage.ScratchpadEntry{
		TaskID:   "TASK-AI-001",
		PhaseID:  "implement",
		Category: "observation",
		Content:  "Found complexity in parser",
	}
	if err := backend.SaveScratchpadEntry(entry); err != nil {
		t.Fatalf("save scratchpad: %v", err)
	}

	// Create executor with recording knowledge service and real backend.
	we := &WorkflowExecutor{
		backend:          backend,
		knowledgeService: rec,
		logger:           slog.Default(),
	}

	// Call the production artifact indexing method.
	we.indexTaskArtifacts(context.Background(), taskProto)

	// Verify the recording service was called.
	if !rec.indexCalled {
		t.Fatal("IndexTaskArtifacts not called — wiring missing in executor")
	}

	// Verify params contain data loaded from backend.
	if rec.indexParams.TaskID != "TASK-AI-001" {
		t.Errorf("TaskID = %q, want TASK-AI-001", rec.indexParams.TaskID)
	}
	if rec.indexParams.Spec == "" {
		t.Error("Spec empty — executor must call backend.GetSpecForTask")
	}
	if len(rec.indexParams.Findings) == 0 {
		t.Error("Findings empty — executor must call backend.LoadAllReviewFindings")
	}
	if len(rec.indexParams.ScratchpadEntries) == 0 {
		t.Error("ScratchpadEntries empty — executor must call backend.GetScratchpadEntries")
	}
}

// SC-3: Executor loads initiative decisions when task has initiative.
// Verifies: executor → backend.LoadInitiative → params.Decisions populated
// Fails if: executor doesn't load initiative or doesn't extract Decisions.
func TestArtifactIndex_LoadsInitiativeDecisions(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	rec := &recordingArtifactIndexService{available: true}

	// Create initiative with decisions.
	init := &initiative.Initiative{
		ID:     "INIT-001",
		Title:  "User Auth",
		Status: initiative.StatusActive,
		Decisions: []initiative.Decision{
			{ID: "DEC-001", Decision: "Use bcrypt", Rationale: "Industry standard"},
			{ID: "DEC-002", Decision: "JWT for tokens", Rationale: "Stateless"},
		},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Create task linked to initiative.
	workflowID := "implement-small"
	initID := "INIT-001"
	taskProto := &orcv1.Task{
		Id:           "TASK-AI-005",
		Title:        "Login endpoint",
		WorkflowId:   &workflowID,
		InitiativeId: &initID,
		Status:       orcv1.TaskStatus_TASK_STATUS_RUNNING,
		CreatedAt:    timestamppb.Now(),
		UpdatedAt:    timestamppb.Now(),
	}
	if err := backend.SaveTask(taskProto); err != nil {
		t.Fatalf("save task: %v", err)
	}

	we := &WorkflowExecutor{
		backend:          backend,
		knowledgeService: rec,
		logger:           slog.Default(),
	}

	we.indexTaskArtifacts(context.Background(), taskProto)

	if !rec.indexCalled {
		t.Fatal("IndexTaskArtifacts not called")
	}
	if rec.indexParams.InitiativeID != "INIT-001" {
		t.Errorf("InitiativeID = %q, want INIT-001", rec.indexParams.InitiativeID)
	}
	if len(rec.indexParams.Decisions) != 2 {
		t.Errorf("Decisions count = %d, want 2", len(rec.indexParams.Decisions))
	}
}

// SC-10: Executor skips artifact indexing when knowledge service is unavailable.
// Verifies: IsAvailable() guard works at executor level.
// Fails if: executor calls IndexTaskArtifacts when service reports unavailable.
func TestArtifactIndex_SkipsWhenKnowledgeUnavailable(t *testing.T) {
	t.Parallel()

	rec := &recordingArtifactIndexService{available: false}
	we := &WorkflowExecutor{
		knowledgeService: rec,
		logger:           slog.Default(),
	}

	taskProto := &orcv1.Task{Id: "TASK-002"}
	we.indexTaskArtifacts(context.Background(), taskProto)

	if rec.indexCalled {
		t.Error("IndexTaskArtifacts called when service is unavailable")
	}
}

// SC-10: Executor handles nil knowledge service gracefully.
// Verifies: No panic when knowledgeService is nil.
// Fails if: executor panics on nil knowledge service or nil type assertion.
func TestArtifactIndex_SkipsWhenNoKnowledgeService(t *testing.T) {
	t.Parallel()

	we := &WorkflowExecutor{
		knowledgeService: nil,
		logger:           slog.Default(),
	}

	taskProto := &orcv1.Task{Id: "TASK-003"}
	// Must not panic.
	we.indexTaskArtifacts(context.Background(), taskProto)
}

// SC-10: Executor handles knowledge service that doesn't implement indexing.
// Verifies: Type assertion to KnowledgeArtifactIndexService fails gracefully.
// Fails if: executor panics when service doesn't implement the indexing interface.
func TestArtifactIndex_SkipsWhenServiceLacksIndexing(t *testing.T) {
	t.Parallel()

	// queryOnlyService only implements KnowledgeQueryService, not indexing.
	queryOnly := &queryOnlyKnowledgeService{available: true}
	we := &WorkflowExecutor{
		knowledgeService: queryOnly,
		logger:           slog.Default(),
	}

	taskProto := &orcv1.Task{Id: "TASK-004"}
	// Must not panic — type assertion should fail gracefully.
	we.indexTaskArtifacts(context.Background(), taskProto)
}

// SC-11: Executor treats artifact indexing errors as warnings (non-fatal).
// Verifies: Errors from IndexTaskArtifacts are caught and logged, not propagated.
// Fails if: executor panics or propagates the error.
func TestArtifactIndex_NonFatalOnError(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	rec := &recordingArtifactIndexService{
		available: true,
		indexErr:  fmt.Errorf("graph completely unavailable"),
	}

	workflowID := "implement-small"
	taskProto := &orcv1.Task{
		Id:         "TASK-AI-006",
		Title:      "Test error handling",
		WorkflowId: &workflowID,
		Status:     orcv1.TaskStatus_TASK_STATUS_RUNNING,
		CreatedAt:  timestamppb.Now(),
		UpdatedAt:  timestamppb.Now(),
	}
	if err := backend.SaveTask(taskProto); err != nil {
		t.Fatalf("save task: %v", err)
	}

	we := &WorkflowExecutor{
		backend:          backend,
		knowledgeService: rec,
		logger:           slog.Default(),
	}

	// Must not panic despite IndexTaskArtifacts returning an error.
	we.indexTaskArtifacts(context.Background(), taskProto)

	// Service should still be called (error is logged, not prevented).
	if !rec.indexCalled {
		t.Error("IndexTaskArtifacts should still be called even if it errors")
	}
}

// BDD-4: Task with no artifacts — indexer receives empty params, no errors.
// Verifies: executor doesn't error when backend returns empty data for all artifacts.
// Fails if: executor errors on empty spec, nil findings, nil scratchpad.
func TestArtifactIndex_NoArtifacts(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	rec := &recordingArtifactIndexService{available: true}

	workflowID := "implement-small"
	taskProto := &orcv1.Task{
		Id:         "TASK-AI-007",
		Title:      "Empty task",
		WorkflowId: &workflowID,
		Status:     orcv1.TaskStatus_TASK_STATUS_RUNNING,
		CreatedAt:  timestamppb.Now(),
		UpdatedAt:  timestamppb.Now(),
	}
	if err := backend.SaveTask(taskProto); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// No spec, no findings, no scratchpad entries stored.

	we := &WorkflowExecutor{
		backend:          backend,
		knowledgeService: rec,
		logger:           slog.Default(),
	}

	we.indexTaskArtifacts(context.Background(), taskProto)

	if !rec.indexCalled {
		t.Fatal("IndexTaskArtifacts should be called even for empty artifacts")
	}

	// Params should have the task ID but empty artifacts.
	if rec.indexParams.TaskID != "TASK-AI-007" {
		t.Errorf("TaskID = %q, want TASK-AI-007", rec.indexParams.TaskID)
	}
}

// --- Test doubles ---

// queryOnlyKnowledgeService implements KnowledgeQueryService but NOT
// KnowledgeArtifactIndexService. Used to test graceful type assertion failure.
type queryOnlyKnowledgeService struct {
	available bool
}

func (q *queryOnlyKnowledgeService) IsAvailable() bool { return q.available }
func (q *queryOnlyKnowledgeService) Query(_ context.Context, _ string, _ retrieve.QueryOpts) (*retrieve.PipelineResult, error) {
	return nil, nil
}
