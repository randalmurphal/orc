package api

import (
	"testing"

	"github.com/randalmurphal/orc/internal/db"
)

func TestDbWorkflowPhaseToProto_DependsOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		dependsOn string
		wantDeps  []string
	}{
		{
			name:      "valid JSON array",
			dependsOn: `["spec","tdd_write"]`,
			wantDeps:  []string{"spec", "tdd_write"},
		},
		{
			name:      "empty string means no deps",
			dependsOn: "",
			wantDeps:  nil,
		},
		{
			name:      "invalid JSON treated as no deps",
			dependsOn: "not-json",
			wantDeps:  nil,
		},
		{
			name:      "empty JSON array",
			dependsOn: `[]`,
			wantDeps:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			phase := &db.WorkflowPhase{
				ID:              1,
				WorkflowID:      "wf-1",
				PhaseTemplateID: "tmpl-1",
				Sequence:        1,
				DependsOn:       tt.dependsOn,
			}

			proto := dbWorkflowPhaseToProto(phase)

			if tt.wantDeps == nil {
				if len(proto.DependsOn) != 0 {
					t.Errorf("expected no deps, got %v", proto.DependsOn)
				}
			} else {
				if len(proto.DependsOn) != len(tt.wantDeps) {
					t.Fatalf("expected %d deps, got %d: %v", len(tt.wantDeps), len(proto.DependsOn), proto.DependsOn)
				}
				for i, want := range tt.wantDeps {
					if proto.DependsOn[i] != want {
						t.Errorf("dep[%d]: expected %q, got %q", i, want, proto.DependsOn[i])
					}
				}
			}
		})
	}
}

func TestDbWorkflowPhaseToProto_PositionFields(t *testing.T) {
	t.Parallel()

	t.Run("nil positions", func(t *testing.T) {
		t.Parallel()
		phase := &db.WorkflowPhase{
			ID:              1,
			WorkflowID:      "wf-1",
			PhaseTemplateID: "tmpl-1",
			Sequence:        1,
		}
		proto := dbWorkflowPhaseToProto(phase)
		if proto.PositionX != nil {
			t.Errorf("expected nil PositionX, got %v", *proto.PositionX)
		}
		if proto.PositionY != nil {
			t.Errorf("expected nil PositionY, got %v", *proto.PositionY)
		}
	})

	t.Run("set positions", func(t *testing.T) {
		t.Parallel()
		x, y := 100.5, 200.75
		phase := &db.WorkflowPhase{
			ID:              2,
			WorkflowID:      "wf-1",
			PhaseTemplateID: "tmpl-1",
			Sequence:        1,
			PositionX:       &x,
			PositionY:       &y,
		}
		proto := dbWorkflowPhaseToProto(phase)
		if proto.PositionX == nil || *proto.PositionX != 100.5 {
			t.Errorf("expected PositionX=100.5, got %v", proto.PositionX)
		}
		if proto.PositionY == nil || *proto.PositionY != 200.75 {
			t.Errorf("expected PositionY=200.75, got %v", proto.PositionY)
		}
	})
}

func TestDbWorkflowPhaseToProto_LoopConfig(t *testing.T) {
	t.Parallel()

	t.Run("empty loop config", func(t *testing.T) {
		t.Parallel()
		phase := &db.WorkflowPhase{
			ID:              1,
			WorkflowID:      "wf-1",
			PhaseTemplateID: "tmpl-1",
			Sequence:        1,
		}
		proto := dbWorkflowPhaseToProto(phase)
		if proto.LoopConfig != nil {
			t.Errorf("expected nil LoopConfig, got %v", *proto.LoopConfig)
		}
	})

	t.Run("set loop config", func(t *testing.T) {
		t.Parallel()
		phase := &db.WorkflowPhase{
			ID:              2,
			WorkflowID:      "wf-1",
			PhaseTemplateID: "tmpl-1",
			Sequence:        1,
			LoopConfig:      `{"max_iterations": 3}`,
		}
		proto := dbWorkflowPhaseToProto(phase)
		if proto.LoopConfig == nil || *proto.LoopConfig != `{"max_iterations": 3}` {
			t.Errorf("expected LoopConfig=%q, got %v", `{"max_iterations": 3}`, proto.LoopConfig)
		}
	})
}

func TestDbWorkflowPhasesToProto_IncludesDependsOn(t *testing.T) {
	t.Parallel()

	phases := []*db.WorkflowPhase{
		{
			ID:              1,
			WorkflowID:      "wf-1",
			PhaseTemplateID: "spec",
			Sequence:        1,
			DependsOn:       "",
		},
		{
			ID:              2,
			WorkflowID:      "wf-1",
			PhaseTemplateID: "implement",
			Sequence:        2,
			DependsOn:       `["spec"]`,
		},
	}

	result := dbWorkflowPhasesToProto(phases)

	if len(result) != 2 {
		t.Fatalf("expected 2 phases, got %d", len(result))
	}
	if len(result[0].DependsOn) != 0 {
		t.Errorf("phase 0: expected no deps, got %v", result[0].DependsOn)
	}
	if len(result[1].DependsOn) != 1 || result[1].DependsOn[0] != "spec" {
		t.Errorf("phase 1: expected [spec], got %v", result[1].DependsOn)
	}
}

func TestDbWorkflowPhasesToProto_IncludesNewFields(t *testing.T) {
	t.Parallel()

	x, y := 50.0, 75.0
	phases := []*db.WorkflowPhase{
		{
			ID:              1,
			WorkflowID:      "wf-1",
			PhaseTemplateID: "spec",
			Sequence:        1,
			PositionX:       &x,
			PositionY:       &y,
			LoopConfig:      `{"count": 2}`,
		},
	}

	result := dbWorkflowPhasesToProto(phases)

	if len(result) != 1 {
		t.Fatalf("expected 1 phase, got %d", len(result))
	}
	p := result[0]
	if p.PositionX == nil || *p.PositionX != 50.0 {
		t.Errorf("expected PositionX=50.0, got %v", p.PositionX)
	}
	if p.PositionY == nil || *p.PositionY != 75.0 {
		t.Errorf("expected PositionY=75.0, got %v", p.PositionY)
	}
	if p.LoopConfig == nil || *p.LoopConfig != `{"count": 2}` {
		t.Errorf("expected LoopConfig=%q, got %v", `{"count": 2}`, p.LoopConfig)
	}
}
