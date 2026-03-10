// Tests for TASK-021: Project brief auto-generated context from task history.
//
// These tests verify the wiring between the brief generator and the executor's
// enrichContextForPhase function, ensuring that rctx.ProjectBrief is populated
// from seeded backend data before each phase.
//
// SC-1: enrichContextForPhase populates rctx.ProjectBrief
// SC-2: Brief generation skipped gracefully for non-DatabaseBackend
// SC-3: Brief cache prevents redundant regeneration across phases
package executor

import (
	"fmt"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// SC-1: enrichContextForPhase populates rctx.ProjectBrief with formatted brief
//
// Given a backend with seeded decisions and findings, enrichContextForPhase
// should populate rctx.ProjectBrief with content containing section headers.
// =============================================================================

func TestEnrichContext_PopulatesProjectBrief(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Seed initiative with decisions
	init := initiative.New("INIT-001", "Auth System")
	init.Status = initiative.StatusActive
	init.Decisions = []initiative.Decision{
		{ID: "DEC-001", Decision: "Use JWT tokens for auth", Rationale: "Stateless", Date: time.Now()},
		{ID: "DEC-002", Decision: "Use bcrypt for passwords", Rationale: "Industry standard", Date: time.Now()},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	// Seed completed tasks with high-severity findings
	for i := 0; i < 5; i++ {
		tsk := task.NewProtoTask(taskIDForBriefTest(i), "Task "+taskIDForBriefTest(i))
		tsk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		initID := "INIT-001"
		tsk.InitiativeId = &initID
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task: %v", err)
		}
	}

	// Add high-severity findings
	file := "internal/auth/login.go"
	line := int32(42)
	findings := &orcv1.ReviewRoundFindings{
		TaskId:  "TASK-B001",
		Round:   1,
		Summary: "Review findings",
		Issues: []*orcv1.ReviewFinding{
			{Severity: "high", Description: "SQL injection in login handler", File: &file, Line: &line},
		},
	}
	if err := backend.SaveReviewFindings(findings); err != nil {
		t.Fatalf("save findings: %v", err)
	}

	// Create config with brief settings
	cfg := config.Default()

	// Create executor with real backend
	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), cfg, t.TempDir())

	// Create a running task for enrichment
	tsk := task.NewProtoTask("TASK-BRIEF-TEST", "Test brief enrichment")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	rctx := &variable.ResolutionContext{
		TaskID:    tsk.Id,
		TaskTitle: tsk.Title,
	}

	// Act: call enrichContextForPhase
	we.enrichContextForPhase(rctx, "implement", tsk)

	// Assert: ProjectBrief should be populated
	if rctx.ProjectBrief == "" {
		t.Fatal("enrichContextForPhase should populate rctx.ProjectBrief, got empty string")
	}

	// Should contain section headers from the brief
	if !strings.Contains(rctx.ProjectBrief, "### Decisions") {
		t.Errorf("ProjectBrief should contain '### Decisions' section header, got:\n%s", rctx.ProjectBrief)
	}

	// Should contain decision content
	if !strings.Contains(rctx.ProjectBrief, "JWT") {
		t.Errorf("ProjectBrief should contain decision content about JWT, got:\n%s", rctx.ProjectBrief)
	}
}

// =============================================================================
// SC-2: Brief generation is skipped gracefully for non-DatabaseBackend
//
// When backend type is not *storage.DatabaseBackend (e.g., a mock),
// enrichContextForPhase should leave rctx.ProjectBrief empty without error.
// =============================================================================

func TestEnrichContext_SkipsForNonDatabaseBackend(t *testing.T) {
	t.Parallel()

	// Use a wrapper that delegates to a real backend but is NOT *storage.DatabaseBackend
	wrapper := newNonDatabaseBackend(t)

	we := NewWorkflowExecutor(wrapper, nil, nil, config.Default(), t.TempDir())

	tsk := task.NewProtoTask("TASK-MOCK-TEST", "Test mock backend")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := wrapper.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	rctx := &variable.ResolutionContext{
		TaskID:    tsk.Id,
		TaskTitle: tsk.Title,
	}

	// Should not panic or error — just skip brief generation
	we.enrichContextForPhase(rctx, "implement", tsk)

	// ProjectBrief should remain empty
	if rctx.ProjectBrief != "" {
		t.Errorf("expected empty ProjectBrief for non-DatabaseBackend, got %q", rctx.ProjectBrief)
	}
}

// =============================================================================
// SC-3: Brief cache prevents redundant regeneration across phases
//
// Two consecutive enrichContextForPhase calls without new task completions
// should return the same brief content (same GeneratedAt timestamp).
// =============================================================================

func TestBrief_CachedAcrossPhases(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Seed data so brief is non-empty
	init := initiative.New("INIT-002", "Caching Test")
	init.Status = initiative.StatusActive
	init.Decisions = []initiative.Decision{
		{ID: "DEC-010", Decision: "Use Redis for caching", Rationale: "Performance", Date: time.Now()},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	cfg := config.Default()
	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), cfg, t.TempDir())

	tsk := task.NewProtoTask("TASK-CACHE-TEST", "Test caching")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	// First call — generates fresh brief
	rctx1 := &variable.ResolutionContext{
		TaskID:    tsk.Id,
		TaskTitle: tsk.Title,
	}
	we.enrichContextForPhase(rctx1, "implement", tsk)

	if rctx1.ProjectBrief == "" {
		t.Fatal("first enrichContextForPhase should populate ProjectBrief")
	}

	// Second call — should use cached brief (same content)
	rctx2 := &variable.ResolutionContext{
		TaskID:    tsk.Id,
		TaskTitle: tsk.Title,
	}
	we.enrichContextForPhase(rctx2, "review", tsk)

	if rctx2.ProjectBrief == "" {
		t.Fatal("second enrichContextForPhase should populate ProjectBrief from cache")
	}

	// Content should be identical (from cache)
	if rctx1.ProjectBrief != rctx2.ProjectBrief {
		t.Errorf("cached brief should return same content across phases\nfirst: %q\nsecond: %q",
			rctx1.ProjectBrief, rctx2.ProjectBrief)
	}
}

// =============================================================================
// BDD-1: Project with initiatives+decisions and completed tasks with findings
// =============================================================================

func TestEnrichContext_BDD1_FullProjectContext(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Seed 2 active initiatives with 3 decisions total
	for i, initData := range []struct {
		id        string
		title     string
		decisions []initiative.Decision
	}{
		{
			id: "INIT-010", title: "Auth System",
			decisions: []initiative.Decision{
				{ID: "DEC-A1", Decision: "Use OAuth2", Rationale: "Standard", Date: time.Now()},
				{ID: "DEC-A2", Decision: "Token rotation every 24h", Rationale: "Security", Date: time.Now()},
			},
		},
		{
			id: "INIT-011", title: "Data Layer",
			decisions: []initiative.Decision{
				{ID: "DEC-B1", Decision: "Use PostgreSQL", Rationale: "ACID compliance", Date: time.Now()},
			},
		},
	} {
		_ = i
		init := initiative.New(initData.id, initData.title)
		init.Status = initiative.StatusActive
		init.Decisions = initData.decisions
		if err := backend.SaveInitiative(init); err != nil {
			t.Fatalf("save initiative %s: %v", initData.id, err)
		}
	}

	// Seed 5 completed tasks with high-severity findings
	for i := 0; i < 5; i++ {
		id := taskIDForBriefTest(i + 10)
		tsk := task.NewProtoTask(id, "Completed task "+id)
		tsk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
		if err := backend.SaveTask(tsk); err != nil {
			t.Fatalf("save task: %v", id)
		}

		file := "internal/handler.go"
		findings := &orcv1.ReviewRoundFindings{
			TaskId:  id,
			Round:   1,
			Summary: "Findings for " + id,
			Issues: []*orcv1.ReviewFinding{
				{Severity: "high", Description: "Finding from " + id, File: &file},
			},
		}
		if err := backend.SaveReviewFindings(findings); err != nil {
			t.Fatalf("save findings for %s: %v", id, err)
		}
	}

	cfg := config.Default()
	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), cfg, t.TempDir())

	tsk := task.NewProtoTask("TASK-BDD1", "BDD test task")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	rctx := &variable.ResolutionContext{
		TaskID:    tsk.Id,
		TaskTitle: tsk.Title,
	}

	we.enrichContextForPhase(rctx, "implement", tsk)

	// Should contain decisions section
	if !strings.Contains(rctx.ProjectBrief, "### Decisions") {
		t.Error("BDD-1: ProjectBrief should contain '### Decisions' section")
	}

	// Should contain findings section
	if !strings.Contains(rctx.ProjectBrief, "### Recent Findings") {
		t.Error("BDD-1: ProjectBrief should contain '### Recent Findings' section")
	}
}

func TestEnrichControlPlaneContext(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	we := NewWorkflowExecutor(backend, backend.DB(), testGlobalDBFrom(backend), config.Default(), t.TempDir())

	targetTask := task.NewProtoTask("TASK-813", "Control-plane contracts")
	targetTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	targetTask.Description = stringPtr("Wire shared control-plane contracts into the resolver.")
	if err := backend.SaveTask(targetTask); err != nil {
		t.Fatalf("save target task: %v", err)
	}

	if err := backend.SaveWorkflow(&db.Workflow{
		ID:          "wf-controlplane",
		Name:        "Control Plane",
		Description: "test workflow",
	}); err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	runTaskID := targetTask.Id
	if err := backend.SaveWorkflowRun(&db.WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "wf-controlplane",
		ContextType: "task",
		TaskID:      &runTaskID,
		Prompt:      "prompt",
		Status:      "running",
	}); err != nil {
		t.Fatalf("save workflow run: %v", err)
	}

	blockedOne := task.NewProtoTask("TASK-101", "Blocked schema review")
	blockedOne.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	blockedOne.CurrentPhase = stringPtr("review")
	blockedOne.Description = stringPtr("Waiting on schema review")
	blockedOne.Metadata = map[string]string{"blocked_reason": "schema approval pending"}
	if err := backend.SaveTask(blockedOne); err != nil {
		t.Fatalf("save blocked task 1: %v", err)
	}

	blockedTwo := task.NewProtoTask("TASK-102", "Blocked resolver update")
	blockedTwo.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
	blockedTwo.CurrentPhase = stringPtr("implement")
	blockedTwo.Description = stringPtr("Resolver wiring still missing")
	if err := backend.SaveTask(blockedTwo); err != nil {
		t.Fatalf("save blocked task 2: %v", err)
	}

	for _, recommendation := range []*orcv1.Recommendation{
		{
			Id:             "REC-001",
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP,
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Title:          "Unify schema builder",
			Summary:        "There are duplicate schema builders.",
			ProposedAction: "Use the shared helper.",
			Evidence:       "Review found two call paths.",
			SourceTaskId:   targetTask.Id,
			SourceRunId:    "RUN-001",
			DedupeKey:      "cleanup:schema-builder",
		},
		{
			Id:             "REC-002",
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP,
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Title:          "Template follow-up",
			Summary:        "Templates still need to adopt the new vars.",
			ProposedAction: "Add variables in a later task.",
			Evidence:       "This task is contract-only.",
			SourceTaskId:   targetTask.Id,
			SourceRunId:    "RUN-001",
			DedupeKey:      "follow_up:template-vars",
		},
		{
			Id:             "REC-003",
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK,
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Title:          "Prompt budget regression",
			Summary:        "Control-plane summaries could grow too large.",
			ProposedAction: "Keep formatter limits in place.",
			Evidence:       "Prompt contexts already run hot.",
			SourceTaskId:   targetTask.Id,
			SourceRunId:    "RUN-001",
			DedupeKey:      "risk:prompt-budget",
		},
		{
			Id:             "REC-004",
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP,
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Title:          "Already accepted",
			Summary:        "Should not appear in pending summary.",
			ProposedAction: "Ignore it here.",
			Evidence:       "Not pending anymore.",
			SourceTaskId:   targetTask.Id,
			SourceRunId:    "RUN-001",
			DedupeKey:      "cleanup:accepted",
		},
	} {
		if err := backend.SaveRecommendation(recommendation); err != nil {
			t.Fatalf("save recommendation %s: %v", recommendation.Id, err)
		}
	}
	if _, err := backend.UpdateRecommendationStatus(
		"REC-004",
		orcv1.RecommendationStatus_RECOMMENDATION_STATUS_ACCEPTED,
		"tester",
		"accepted for filtering coverage",
	); err != nil {
		t.Fatalf("accept recommendation: %v", err)
	}

	rctx := &variable.ResolutionContext{
		TaskID:    targetTask.Id,
		TaskTitle: targetTask.Title,
	}

	we.enrichContextForPhase(rctx, "implement", targetTask)

	if got := strings.Count(rctx.PendingRecommendations, "\n- ["); got != 3 {
		t.Fatalf("pending recommendation count = %d, want 3\n%s", got, rctx.PendingRecommendations)
	}
	if strings.Contains(rctx.PendingRecommendations, "Already accepted") {
		t.Fatalf("accepted recommendation leaked into pending summary: %s", rctx.PendingRecommendations)
	}
	for _, taskID := range []string{"TASK-101", "TASK-102"} {
		if !strings.Contains(rctx.AttentionSummary, taskID) {
			t.Fatalf("attention summary missing %s: %s", taskID, rctx.AttentionSummary)
		}
	}
	if !strings.Contains(rctx.HandoffContext, "## Handoff Pack") {
		t.Fatalf("handoff context missing header: %s", rctx.HandoffContext)
	}
	for _, want := range []string{
		"Task: TASK-813 Control-plane contracts",
		"Current phase: implement",
		"Summary: Wire shared control-plane contracts into the resolver.",
		"Next step: Add variables in a later task.",
		"Next step: Use the shared helper.",
		"Risk: Prompt budget regression: Control-plane summaries could grow too large.",
	} {
		if !strings.Contains(rctx.HandoffContext, want) {
			t.Fatalf("handoff context missing %q: %s", want, rctx.HandoffContext)
		}
	}
}

func TestEnrichControlPlaneContextClearsStaleValuesOnLoadFailure(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	failingBackend := &failingControlPlaneBackend{Backend: backend}
	we := NewWorkflowExecutor(failingBackend, backend.DB(), testGlobalDBFrom(backend), config.Default(), t.TempDir())

	targetTask := task.NewProtoTask("TASK-813", "Control-plane contracts")
	targetTask.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING

	rctx := &variable.ResolutionContext{
		PendingRecommendations: "stale pending summary",
		AttentionSummary:       "stale attention summary",
		HandoffContext:         "stale handoff summary",
	}

	we.enrichContextForPhase(rctx, "implement", targetTask)

	if rctx.PendingRecommendations != "" {
		t.Fatalf("PendingRecommendations = %q, want empty string after load failure", rctx.PendingRecommendations)
	}
	if rctx.AttentionSummary != "" {
		t.Fatalf("AttentionSummary = %q, want empty string after load failure", rctx.AttentionSummary)
	}
	if rctx.HandoffContext != "" {
		t.Fatalf("HandoffContext = %q, want empty string after load failure", rctx.HandoffContext)
	}
}

func stringPtr(value string) *string {
	return &value
}

// =============================================================================
// Helpers
// =============================================================================

func taskIDForBriefTest(i int) string {
	return fmt.Sprintf("TASK-B%03d", i+1)
}

// nonDatabaseBackendWrapper embeds a real DatabaseBackend (via the Backend
// interface) so all methods are delegated. The key property: its concrete type
// is *nonDatabaseBackendWrapper, NOT *storage.DatabaseBackend, so type assertions
// like `backend.(*storage.DatabaseBackend)` return false.
type nonDatabaseBackendWrapper struct {
	storage.Backend // embeds the interface — delegates all methods
}

type failingControlPlaneBackend struct {
	storage.Backend
}

func (f *failingControlPlaneBackend) LoadAllRecommendations() ([]*orcv1.Recommendation, error) {
	return nil, fmt.Errorf("recommendations unavailable")
}

func (f *failingControlPlaneBackend) LoadAllTasks() ([]*orcv1.Task, error) {
	return nil, fmt.Errorf("tasks unavailable")
}

// newNonDatabaseBackend creates a wrapper that satisfies storage.Backend
// but whose concrete type is NOT *storage.DatabaseBackend.
func newNonDatabaseBackend(t *testing.T) *nonDatabaseBackendWrapper {
	t.Helper()
	return &nonDatabaseBackendWrapper{Backend: storage.NewTestBackend(t)}
}
