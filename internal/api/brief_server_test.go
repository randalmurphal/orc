// Tests for TASK-021: Brief API server.
//
// SC-8: GetProjectBrief returns the current brief as a proto message
// SC-9: RegenerateProjectBrief forces regeneration and returns new brief
// SC-10: Brief API endpoints route correctly via project_id
package api

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// =============================================================================
// SC-8: GetProjectBrief returns the current brief
// =============================================================================

func TestBriefServer_GetProjectBrief(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Seed data for a populated brief
	init := initiative.New("INIT-001", "Test Initiative")
	init.Status = initiative.StatusActive
	init.Decisions = []initiative.Decision{
		{ID: "DEC-001", Decision: "Use JWT tokens", Rationale: "Stateless auth", Date: time.Now()},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	tsk := task.NewProtoTask("TASK-001", "Login feature")
	tsk.Status = orcv1.TaskStatus_TASK_STATUS_COMPLETED
	if err := backend.SaveTask(tsk); err != nil {
		t.Fatalf("save task: %v", err)
	}

	server := NewBriefServer(backend, nil)

	resp, err := server.GetProjectBrief(
		context.Background(),
		connect.NewRequest(&orcv1.GetProjectBriefRequest{}),
	)
	if err != nil {
		t.Fatalf("GetProjectBrief() error: %v", err)
	}

	brief := resp.Msg

	// Should have sections
	if len(brief.Sections) == 0 {
		t.Error("expected at least one section in brief")
	}

	// Should have a generated_at timestamp
	if brief.GeneratedAt == nil {
		t.Error("expected GeneratedAt timestamp")
	}

	// Should have token count
	if brief.TokenCount <= 0 {
		t.Error("expected positive token count")
	}
}

func TestBriefServer_GetProjectBrief_EmptyProject(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	server := NewBriefServer(backend, nil)

	resp, err := server.GetProjectBrief(
		context.Background(),
		connect.NewRequest(&orcv1.GetProjectBriefRequest{}),
	)

	// Should NOT return error — just an empty brief
	if err != nil {
		t.Fatalf("GetProjectBrief() should not error for empty project, got: %v", err)
	}

	brief := resp.Msg
	if len(brief.Sections) != 0 {
		t.Errorf("expected 0 sections for empty project, got %d", len(brief.Sections))
	}
}

// =============================================================================
// SC-9: RegenerateProjectBrief forces regeneration
// =============================================================================

func TestBriefServer_Regenerate(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Seed data
	init := initiative.New("INIT-001", "Test")
	init.Status = initiative.StatusActive
	init.Decisions = []initiative.Decision{
		{ID: "DEC-001", Decision: "Use Redis", Rationale: "Speed", Date: time.Now()},
	}
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	server := NewBriefServer(backend, nil)

	resp, err := server.RegenerateProjectBrief(
		context.Background(),
		connect.NewRequest(&orcv1.RegenerateProjectBriefRequest{}),
	)
	if err != nil {
		t.Fatalf("RegenerateProjectBrief() error: %v", err)
	}

	brief := resp.Msg

	// Should have a fresh timestamp
	if brief.GeneratedAt == nil {
		t.Error("regenerated brief should have GeneratedAt")
	}

	// Should contain sections from seeded data
	if len(brief.Sections) == 0 {
		t.Error("regenerated brief should have sections from seeded data")
	}
}

// =============================================================================
// SC-10: Brief API endpoints route via project_id
// =============================================================================

func TestBriefServer_MultiProject(t *testing.T) {
	t.Parallel()

	// Create two separate backends for two projects
	backend1 := storage.NewTestBackend(t)
	backend2 := storage.NewTestBackend(t)

	// Seed different data in each
	init1 := initiative.New("INIT-P1", "Project One Initiative")
	init1.Status = initiative.StatusActive
	init1.Decisions = []initiative.Decision{
		{ID: "DEC-P1", Decision: "Project One Decision", Date: time.Now()},
	}
	if err := backend1.SaveInitiative(init1); err != nil {
		t.Fatalf("save initiative for project 1: %v", err)
	}

	init2 := initiative.New("INIT-P2", "Project Two Initiative")
	init2.Status = initiative.StatusActive
	init2.Decisions = []initiative.Decision{
		{ID: "DEC-P2", Decision: "Project Two Decision", Date: time.Now()},
	}
	if err := backend2.SaveInitiative(init2); err != nil {
		t.Fatalf("save initiative for project 2: %v", err)
	}

	// The brief server should use getBackend(projectID) pattern for routing.
	// For this test, we create a server with the default backend (project 1)
	// and verify that requests without project_id use the default backend.
	server := NewBriefServer(backend1, nil)

	resp, err := server.GetProjectBrief(
		context.Background(),
		connect.NewRequest(&orcv1.GetProjectBriefRequest{}),
	)
	if err != nil {
		t.Fatalf("GetProjectBrief() error: %v", err)
	}

	// Verify we got project 1's data
	found := false
	for _, sec := range resp.Msg.Sections {
		for _, entry := range sec.Entries {
			if entry.Content == "Project One Decision" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected to find 'Project One Decision' in brief from default backend")
	}
}

// =============================================================================
// Failure mode: backend unavailable
// =============================================================================

func TestBriefServer_InvalidProject(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	// Close the backend to simulate unavailability
	_ = backend.Close()

	server := NewBriefServer(backend, nil)

	_, err := server.GetProjectBrief(
		context.Background(),
		connect.NewRequest(&orcv1.GetProjectBriefRequest{}),
	)

	// Should return an error (database is closed)
	if err == nil {
		t.Error("expected error when backend is closed")
	}
}
