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

func TestValidateWorkflow_ValidLinearWorkflow(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	ensurePhaseTemplates(t, backend, "spec", "implement", "review")

	err := backend.SaveWorkflow(&db.Workflow{
		ID:           "wf-linear",
		Name:         "Linear Workflow",
		WorkflowType: "task",
		IsBuiltin:    false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// spec -> implement -> review (linear chain via depends_on)
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-linear",
		PhaseTemplateID: "spec",
		Sequence:        1,
		DependsOn:       "",
	})
	if err != nil {
		t.Fatalf("save phase spec: %v", err)
	}
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-linear",
		PhaseTemplateID: "implement",
		Sequence:        2,
		DependsOn:       `["spec"]`,
	})
	if err != nil {
		t.Fatalf("save phase implement: %v", err)
	}
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-linear",
		PhaseTemplateID: "review",
		Sequence:        3,
		DependsOn:       `["implement"]`,
	})
	if err != nil {
		t.Fatalf("save phase review: %v", err)
	}

	server := NewWorkflowServer(backend, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.ValidateWorkflowRequest{
		WorkflowId: "wf-linear",
	})

	resp, err := server.ValidateWorkflow(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Msg.Valid {
		t.Errorf("expected valid=true, got false")
		for _, issue := range resp.Msg.Issues {
			t.Logf("  issue: severity=%s message=%s phases=%v", issue.Severity, issue.Message, issue.PhaseIds)
		}
	}
	if len(resp.Msg.Issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(resp.Msg.Issues))
	}
}

func TestValidateWorkflow_CycleDetected(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	ensurePhaseTemplates(t, backend, "phase_a", "phase_b", "phase_c")

	err := backend.SaveWorkflow(&db.Workflow{
		ID:           "wf-cycle",
		Name:         "Cyclic Workflow",
		WorkflowType: "task",
		IsBuiltin:    false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	// A depends on C, B depends on A, C depends on B -> cycle
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-cycle",
		PhaseTemplateID: "phase_a",
		Sequence:        1,
		DependsOn:       `["phase_c"]`,
	})
	if err != nil {
		t.Fatalf("save phase_a: %v", err)
	}
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-cycle",
		PhaseTemplateID: "phase_b",
		Sequence:        2,
		DependsOn:       `["phase_a"]`,
	})
	if err != nil {
		t.Fatalf("save phase_b: %v", err)
	}
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-cycle",
		PhaseTemplateID: "phase_c",
		Sequence:        3,
		DependsOn:       `["phase_b"]`,
	})
	if err != nil {
		t.Fatalf("save phase_c: %v", err)
	}

	server := NewWorkflowServer(backend, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.ValidateWorkflowRequest{
		WorkflowId: "wf-cycle",
	})

	resp, err := server.ValidateWorkflow(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Msg.Valid {
		t.Error("expected valid=false for cyclic workflow")
	}

	// Should have at least one error-severity issue about cycles
	foundCycleError := false
	for _, issue := range resp.Msg.Issues {
		if issue.Severity == "error" {
			foundCycleError = true
			break
		}
	}
	if !foundCycleError {
		t.Error("expected at least one issue with severity=\"error\" for cycle detection")
	}
}

func TestValidateWorkflow_InvalidDependencyReference(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	ensurePhaseTemplates(t, backend, "spec", "implement")

	err := backend.SaveWorkflow(&db.Workflow{
		ID:           "wf-bad-dep",
		Name:         "Bad Dependency Workflow",
		WorkflowType: "task",
		IsBuiltin:    false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-bad-dep",
		PhaseTemplateID: "spec",
		Sequence:        1,
		DependsOn:       "",
	})
	if err != nil {
		t.Fatalf("save phase spec: %v", err)
	}
	// implement depends on "nonexistent" phase
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-bad-dep",
		PhaseTemplateID: "implement",
		Sequence:        2,
		DependsOn:       `["nonexistent"]`,
	})
	if err != nil {
		t.Fatalf("save phase implement: %v", err)
	}

	server := NewWorkflowServer(backend, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.ValidateWorkflowRequest{
		WorkflowId: "wf-bad-dep",
	})

	resp, err := server.ValidateWorkflow(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Msg.Valid {
		t.Error("expected valid=false for invalid dependency reference")
	}

	foundRefError := false
	for _, issue := range resp.Msg.Issues {
		if issue.Severity == "error" {
			foundRefError = true
			break
		}
	}
	if !foundRefError {
		t.Error("expected at least one issue with severity=\"error\" for invalid dependency reference")
	}
}

func TestValidateWorkflow_InvalidLoopToPhase(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	ensurePhaseTemplates(t, backend, "spec", "review")

	err := backend.SaveWorkflow(&db.Workflow{
		ID:           "wf-bad-loop",
		Name:         "Bad Loop Workflow",
		WorkflowType: "task",
		IsBuiltin:    false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	})
	if err != nil {
		t.Fatalf("save workflow: %v", err)
	}

	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-bad-loop",
		PhaseTemplateID: "spec",
		Sequence:        1,
		DependsOn:       "",
	})
	if err != nil {
		t.Fatalf("save phase spec: %v", err)
	}
	// review has loop_config pointing to a phase that doesn't exist
	err = backend.SaveWorkflowPhase(&db.WorkflowPhase{
		WorkflowID:      "wf-bad-loop",
		PhaseTemplateID: "review",
		Sequence:        2,
		DependsOn:       `["spec"]`,
		LoopConfig:      `{"loop_to_phase": "nonexistent_phase", "max_iterations": 3}`,
	})
	if err != nil {
		t.Fatalf("save phase review: %v", err)
	}

	server := NewWorkflowServer(backend, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.ValidateWorkflowRequest{
		WorkflowId: "wf-bad-loop",
	})

	resp, err := server.ValidateWorkflow(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Msg.Valid {
		t.Error("expected valid=true when only warnings exist (loop_to_phase is a warning)")
	}

	foundLoopWarning := false
	for _, issue := range resp.Msg.Issues {
		if issue.Severity == "warning" {
			foundLoopWarning = true
			break
		}
	}
	if !foundLoopWarning {
		t.Error("expected at least one issue with severity=\"warning\" for invalid loop_to_phase")
	}
}

func TestValidateWorkflow_EmptyWorkflowID(t *testing.T) {
	t.Parallel()

	backend := storage.NewTestBackend(t)
	server := NewWorkflowServer(backend, nil, nil, nil, slog.Default())

	req := connect.NewRequest(&orcv1.ValidateWorkflowRequest{
		WorkflowId: "",
	})

	_, err := server.ValidateWorkflow(context.Background(), req)
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
