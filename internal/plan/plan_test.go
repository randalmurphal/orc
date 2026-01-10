package plan

import (
	"os"
	"testing"

	"github.com/randalmurphal/orc/internal/task"
)

func TestPlanCurrentPhase(t *testing.T) {
	p := &Plan{
		Phases: []Phase{
			{ID: "spec", Status: PhaseCompleted},
			{ID: "implement", Status: PhaseRunning},
			{ID: "test", Status: PhasePending},
		},
	}

	current := p.CurrentPhase()
	if current == nil {
		t.Fatal("CurrentPhase() returned nil")
	}
	if current.ID != "implement" {
		t.Errorf("CurrentPhase() = %s, want implement", current.ID)
	}
}

func TestPlanCurrentPhaseAllComplete(t *testing.T) {
	p := &Plan{
		Phases: []Phase{
			{ID: "spec", Status: PhaseCompleted},
			{ID: "implement", Status: PhaseCompleted},
		},
	}

	current := p.CurrentPhase()
	if current != nil {
		t.Errorf("CurrentPhase() = %s, want nil", current.ID)
	}
}

func TestPlanGetPhase(t *testing.T) {
	p := &Plan{
		Phases: []Phase{
			{ID: "spec", Name: "Specification"},
			{ID: "implement", Name: "Implementation"},
		},
	}

	// Found
	phase := p.GetPhase("spec")
	if phase == nil {
		t.Fatal("GetPhase(spec) returned nil")
	}
	if phase.Name != "Specification" {
		t.Errorf("GetPhase(spec).Name = %s, want Specification", phase.Name)
	}

	// Not found
	phase = p.GetPhase("nonexistent")
	if phase != nil {
		t.Errorf("GetPhase(nonexistent) = %v, want nil", phase)
	}
}

func TestPlanIsComplete(t *testing.T) {
	tests := []struct {
		name     string
		phases   []Phase
		complete bool
	}{
		{
			name: "all completed",
			phases: []Phase{
				{ID: "a", Status: PhaseCompleted},
				{ID: "b", Status: PhaseCompleted},
			},
			complete: true,
		},
		{
			name: "some pending",
			phases: []Phase{
				{ID: "a", Status: PhaseCompleted},
				{ID: "b", Status: PhasePending},
			},
			complete: false,
		},
		{
			name: "skipped counts as complete",
			phases: []Phase{
				{ID: "a", Status: PhaseCompleted},
				{ID: "b", Status: PhaseSkipped},
			},
			complete: true,
		},
		{
			name:     "empty is complete",
			phases:   []Phase{},
			complete: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Plan{Phases: tt.phases}
			if p.IsComplete() != tt.complete {
				t.Errorf("IsComplete() = %v, want %v", p.IsComplete(), tt.complete)
			}
		})
	}
}

func TestGeneratorGenerate(t *testing.T) {
	gen := NewGenerator()

	// Load template
	tmpl := &PlanTemplate{
		Version:     1,
		Weight:      task.WeightSmall,
		Description: "Small task template",
		Phases: []Phase{
			{ID: "implement", Name: "implement"},
			{ID: "test", Name: "test"},
		},
	}
	gen.LoadTemplate(task.WeightSmall, tmpl)

	// Generate plan
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}
	plan, err := gen.Generate(tsk)
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	if plan.TaskID != "TASK-001" {
		t.Errorf("plan.TaskID = %s, want TASK-001", plan.TaskID)
	}

	if plan.Weight != task.WeightSmall {
		t.Errorf("plan.Weight = %s, want small", plan.Weight)
	}

	if len(plan.Phases) != 2 {
		t.Errorf("len(plan.Phases) = %d, want 2", len(plan.Phases))
	}

	// Check phases have pending status
	for _, phase := range plan.Phases {
		if phase.Status != PhasePending {
			t.Errorf("phase %s status = %s, want pending", phase.ID, phase.Status)
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()

	err := os.MkdirAll(tmpDir+"/.orc/tasks/TASK-001", 0755)
	if err != nil {
		t.Fatalf("failed to create test directory: %v", err)
	}

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	// Create and save plan
	p := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightMedium,
		Description: "Test plan",
		Phases: []Phase{
			{ID: "spec", Name: "spec", Status: PhasePending},
			{ID: "implement", Name: "implement", Status: PhasePending},
		},
	}

	err = p.Save("TASK-001")
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load plan
	loaded, err := Load("TASK-001")
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if loaded.TaskID != p.TaskID {
		t.Errorf("loaded TaskID = %s, want %s", loaded.TaskID, p.TaskID)
	}

	if loaded.Weight != p.Weight {
		t.Errorf("loaded Weight = %s, want %s", loaded.Weight, p.Weight)
	}

	if len(loaded.Phases) != len(p.Phases) {
		t.Errorf("loaded phases = %d, want %d", len(loaded.Phases), len(p.Phases))
	}
}
