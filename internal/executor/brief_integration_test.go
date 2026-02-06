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
	we := NewWorkflowExecutor(backend, backend.DB(), cfg, t.TempDir())

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

	we := NewWorkflowExecutor(wrapper, nil, config.Default(), t.TempDir())

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
	we := NewWorkflowExecutor(backend, backend.DB(), cfg, t.TempDir())

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
	we := NewWorkflowExecutor(backend, backend.DB(), cfg, t.TempDir())

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

// newNonDatabaseBackend creates a wrapper that satisfies storage.Backend
// but whose concrete type is NOT *storage.DatabaseBackend.
func newNonDatabaseBackend(t *testing.T) *nonDatabaseBackendWrapper {
	t.Helper()
	return &nonDatabaseBackendWrapper{Backend: storage.NewTestBackend(t)}
}
