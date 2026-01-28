package gate

import (
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

func TestResolver_TaskOverrideTakesPrecedence(t *testing.T) {
	cfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType: "auto",
			PhaseOverrides: map[string]string{
				"spec": "ai",
			},
		},
	}

	taskOverrides := map[string]*db.TaskGateOverride{
		"spec": {TaskID: "TASK-001", PhaseID: "spec", GateType: "human"},
	}

	r := NewResolver(cfg, WithTaskOverrides(taskOverrides))
	result := r.Resolve("spec", "medium")

	if result.GateType != GateHuman {
		t.Errorf("expected human gate from task override, got %s", result.GateType)
	}
	if result.Source != "task_override" {
		t.Errorf("expected source 'task_override', got %s", result.Source)
	}
}

func TestResolver_WeightOverrideTakesPrecedence(t *testing.T) {
	cfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType: "auto",
			PhaseOverrides: map[string]string{
				"spec": "ai",
			},
			WeightOverrides: map[string]map[string]string{
				"large": {"spec": "human"},
			},
		},
	}

	r := NewResolver(cfg)

	// Large weight should get human gate
	result := r.Resolve("spec", "large")
	if result.GateType != GateHuman {
		t.Errorf("expected human gate for large weight, got %s", result.GateType)
	}
	if result.Source != "weight_override" {
		t.Errorf("expected source 'weight_override', got %s", result.Source)
	}

	// Medium weight should get ai gate from phase override
	result = r.Resolve("spec", "medium")
	if result.GateType != GateAI {
		t.Errorf("expected ai gate for medium weight, got %s", result.GateType)
	}
	if result.Source != "phase_override" {
		t.Errorf("expected source 'phase_override', got %s", result.Source)
	}
}

func TestResolver_PhaseGateFromDatabase(t *testing.T) {
	cfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType: "auto",
		},
	}

	phaseGates := map[string]*db.PhaseGate{
		"review": {PhaseID: "review", GateType: "human", Enabled: true},
	}

	r := NewResolver(cfg, WithPhaseGates(phaseGates))
	result := r.Resolve("review", "medium")

	if result.GateType != GateHuman {
		t.Errorf("expected human gate from database, got %s", result.GateType)
	}
	if result.Source != "phase_gate" {
		t.Errorf("expected source 'phase_gate', got %s", result.Source)
	}
}

func TestResolver_DisabledPhaseGate(t *testing.T) {
	cfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType: "auto",
		},
	}

	phaseGates := map[string]*db.PhaseGate{
		"review": {PhaseID: "review", GateType: "human", Enabled: false},
	}

	r := NewResolver(cfg, WithPhaseGates(phaseGates))
	result := r.Resolve("review", "medium")

	// Should fall through to default since phase gate is disabled
	if result.GateType != GateAuto {
		t.Errorf("expected auto gate (default), got %s", result.GateType)
	}
	if result.Source != "default" {
		t.Errorf("expected source 'default', got %s", result.Source)
	}
}

func TestResolver_EnabledPhases(t *testing.T) {
	cfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType:   "human",
			EnabledPhases: []string{"spec", "review"},
		},
	}

	r := NewResolver(cfg)

	// Enabled phase should get gate
	result := r.Resolve("spec", "medium")
	if result.GateType != GateHuman {
		t.Errorf("expected human gate for enabled phase, got %s", result.GateType)
	}
	if !result.Enabled {
		t.Error("expected enabled to be true for enabled phase")
	}

	// Non-enabled phase should be skipped
	result = r.Resolve("implement", "medium")
	if result.GateType != GateSkip {
		t.Errorf("expected skip gate for non-enabled phase, got %s", result.GateType)
	}
	if result.Enabled {
		t.Error("expected enabled to be false for non-enabled phase")
	}
}

func TestResolver_DisabledPhasesTakesPrecedence(t *testing.T) {
	cfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType:    "human",
			AllPhases:      true,
			DisabledPhases: []string{"breakdown", "docs"},
		},
	}

	r := NewResolver(cfg)

	// Enabled phase should get gate (AllPhases = true)
	result := r.Resolve("spec", "medium")
	if result.GateType != GateHuman {
		t.Errorf("expected human gate, got %s", result.GateType)
	}
	if !result.Enabled {
		t.Error("expected enabled to be true")
	}

	// Disabled phase should be skipped even with AllPhases = true
	result = r.Resolve("breakdown", "medium")
	if result.GateType != GateSkip {
		t.Errorf("expected skip gate for disabled phase, got %s", result.GateType)
	}
	if result.Enabled {
		t.Error("expected enabled to be false for disabled phase")
	}
}

func TestResolver_DefaultFallback(t *testing.T) {
	cfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType: "ai",
		},
	}

	r := NewResolver(cfg)
	result := r.Resolve("implement", "medium")

	if result.GateType != GateAI {
		t.Errorf("expected ai gate (default), got %s", result.GateType)
	}
	if result.Source != "default" {
		t.Errorf("expected source 'default', got %s", result.Source)
	}
}

func TestResolver_NilConfig(t *testing.T) {
	r := NewResolver(nil)
	result := r.Resolve("spec", "medium")

	// Should default to auto
	if result.GateType != GateAuto {
		t.Errorf("expected auto gate with nil config, got %s", result.GateType)
	}
	if result.Source != "default" {
		t.Errorf("expected source 'default', got %s", result.Source)
	}
}

func TestResolver_IsPhaseGated(t *testing.T) {
	cfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType:    "human",
			DisabledPhases: []string{"docs"},
		},
	}

	r := NewResolver(cfg)

	if !r.IsPhaseGated("spec", "medium") {
		t.Error("expected spec to be gated")
	}

	if r.IsPhaseGated("docs", "medium") {
		t.Error("expected docs to NOT be gated")
	}
}

func TestResolver_SkipGateType(t *testing.T) {
	cfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType: "auto",
			PhaseOverrides: map[string]string{
				"breakdown": "skip",
			},
		},
	}

	r := NewResolver(cfg)

	if r.IsPhaseGated("breakdown", "medium") {
		t.Error("expected breakdown to NOT be gated when gate type is skip")
	}
}
