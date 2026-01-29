package api

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

// ensurePhaseTemplates creates phase templates required by FK constraints.
func ensurePhaseTemplates(t *testing.T, backend *storage.DatabaseBackend, ids ...string) {
	t.Helper()
	for _, id := range ids {
		if err := backend.SavePhaseTemplate(&db.PhaseTemplate{
			ID:           id,
			Name:         id,
			PromptSource: "db",
		}); err != nil {
			t.Fatalf("save phase template %s: %v", id, err)
		}
	}
}

func TestSaveWorkflowLayout_Success(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	ensurePhaseTemplates(t, backend, "spec", "implement")

	// Create a non-builtin workflow with phases
	err := backend.SaveWorkflow(&db.Workflow{
		ID:           "wf-custom",
		Name:         "Custom Workflow",
		WorkflowType: "task",
		IsBuiltin:    false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("save workflow: %v", err)
	}
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-custom",
		PhaseTemplateID: "spec",
		Sequence:        1,
	})
	if err != nil {
		t.Fatalf("save phase spec: %v", err)
	}
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-custom",
		PhaseTemplateID: "implement",
		Sequence:        2,
	})
	if err != nil {
		t.Fatalf("save phase implement: %v", err)
	}

	server := NewWorkflowServer(backend, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.SaveWorkflowLayoutRequest{
		WorkflowId: "wf-custom",
		Positions: []*orcv1.PhasePosition{
			{PhaseTemplateId: "spec", PositionX: 100.0, PositionY: 200.0},
			{PhaseTemplateId: "implement", PositionX: 300.0, PositionY: 400.0},
		},
	})

	resp, err := server.SaveWorkflowLayout(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Msg.Success {
		t.Error("expected success=true")
	}

	// Verify positions persisted
	phases, err := backend.GetWorkflowPhases("wf-custom")
	if err != nil {
		t.Fatalf("get phases: %v", err)
	}
	for _, p := range phases {
		switch p.PhaseTemplateID {
		case "spec":
			if p.PositionX == nil || *p.PositionX != 100.0 {
				t.Errorf("spec PositionX: want 100.0, got %v", p.PositionX)
			}
			if p.PositionY == nil || *p.PositionY != 200.0 {
				t.Errorf("spec PositionY: want 200.0, got %v", p.PositionY)
			}
		case "implement":
			if p.PositionX == nil || *p.PositionX != 300.0 {
				t.Errorf("implement PositionX: want 300.0, got %v", p.PositionX)
			}
			if p.PositionY == nil || *p.PositionY != 400.0 {
				t.Errorf("implement PositionY: want 400.0, got %v", p.PositionY)
			}
		}
	}
}

func TestSaveWorkflowLayout_BuiltinWorkflow_Rejected(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)

	err := backend.SaveWorkflow(&db.Workflow{
		ID:           "wf-builtin",
		Name:         "Built-in Workflow",
		WorkflowType: "task",
		IsBuiltin:    true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	server := NewWorkflowServer(backend, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.SaveWorkflowLayoutRequest{
		WorkflowId: "wf-builtin",
		Positions: []*orcv1.PhasePosition{
			{PhaseTemplateId: "spec", PositionX: 10.0, PositionY: 20.0},
		},
	})

	_, err = server.SaveWorkflowLayout(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for builtin workflow")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodePermissionDenied {
		t.Errorf("expected CodePermissionDenied, got %v", connectErr.Code())
	}
}

func TestSaveWorkflowLayout_EmptyWorkflowID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewWorkflowServer(backend, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.SaveWorkflowLayoutRequest{
		WorkflowId: "",
		Positions: []*orcv1.PhasePosition{
			{PhaseTemplateId: "spec", PositionX: 10.0, PositionY: 20.0},
		},
	})

	_, err := server.SaveWorkflowLayout(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty workflow_id")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
	}
}

func TestSaveWorkflowLayout_WorkflowNotFound(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewWorkflowServer(backend, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.SaveWorkflowLayoutRequest{
		WorkflowId: "wf-nonexistent",
		Positions: []*orcv1.PhasePosition{
			{PhaseTemplateId: "spec", PositionX: 10.0, PositionY: 20.0},
		},
	})

	_, err := server.SaveWorkflowLayout(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for non-existent workflow")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected *connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected CodeNotFound, got %v", connectErr.Code())
	}
}
