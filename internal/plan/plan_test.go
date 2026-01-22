package plan

import (
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

func TestPlanError(t *testing.T) {
	// Test the planError type - verify they are non-empty strings
	err := ErrNoTemplate
	if err.Error() == "" {
		t.Error("ErrNoTemplate.Error() should not be empty")
	}

	err = ErrNotFound
	if err.Error() == "" {
		t.Error("ErrNotFound.Error() should not be empty")
	}
}

func TestLoadTemplateAndCreateFromTemplate(t *testing.T) {
	// Test LoadTemplate for various weights
	weights := []task.Weight{task.WeightTrivial, task.WeightSmall, task.WeightMedium, task.WeightLarge}

	for _, w := range weights {
		t.Run(string(w), func(t *testing.T) {
			tmpl, err := LoadTemplate(w)
			if err != nil {
				t.Fatalf("LoadTemplate(%s) failed: %v", w, err)
			}

			if tmpl.Weight != w {
				t.Errorf("template weight = %s, want %s", tmpl.Weight, w)
			}

			if len(tmpl.Phases) == 0 {
				t.Error("template has no phases")
			}
		})
	}

	// Test CreateFromTemplate
	tsk := &task.Task{ID: "TASK-TEST", Weight: task.WeightSmall}
	plan, err := CreateFromTemplate(tsk)
	if err != nil {
		t.Fatalf("CreateFromTemplate() failed: %v", err)
	}

	if plan.TaskID != "TASK-TEST" {
		t.Errorf("plan.TaskID = %s, want TASK-TEST", plan.TaskID)
	}

	if plan.Weight != task.WeightSmall {
		t.Errorf("plan.Weight = %s, want small", plan.Weight)
	}
}

func TestLoadTemplateInvalidWeight(t *testing.T) {
	_, err := LoadTemplate("nonexistent")
	if err == nil {
		t.Error("LoadTemplate should return error for invalid weight")
	}
}

func TestReset(t *testing.T) {
	p := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightMedium,
		Description: "Test plan",
		Phases: []Phase{
			{ID: "spec", Name: "Specification", Status: PhaseCompleted, CommitSHA: "abc123"},
			{ID: "implement", Name: "Implementation", Status: PhaseFailed, CommitSHA: "def456"},
			{ID: "test", Name: "Testing", Status: PhasePending, CommitSHA: ""},
		},
	}

	// Reset the plan
	p.Reset()

	// Verify all phases are reset
	for _, phase := range p.Phases {
		if phase.Status != PhasePending {
			t.Errorf("Phase %s status = %s, want %s", phase.ID, phase.Status, PhasePending)
		}
		if phase.CommitSHA != "" {
			t.Errorf("Phase %s CommitSHA = %s, want empty", phase.ID, phase.CommitSHA)
		}
	}

	// Verify other fields are preserved
	if p.TaskID != "TASK-001" {
		t.Errorf("TaskID = %s, want TASK-001", p.TaskID)
	}
	if p.Weight != task.WeightMedium {
		t.Errorf("Weight = %s, want medium", p.Weight)
	}
}

func TestRegeneratePlan_NoOldPlan(t *testing.T) {
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := RegeneratePlan(tsk, nil)
	if err != nil {
		t.Fatalf("RegeneratePlan() failed: %v", err)
	}

	if result.NewPlan == nil {
		t.Fatal("NewPlan is nil")
	}

	if result.NewPlan.Weight != task.WeightSmall {
		t.Errorf("NewPlan.Weight = %s, want small", result.NewPlan.Weight)
	}

	// All phases should be in ResetPhases since there's no old plan
	if len(result.PreservedPhases) != 0 {
		t.Errorf("PreservedPhases = %v, want empty", result.PreservedPhases)
	}

	if len(result.ResetPhases) != len(result.NewPlan.Phases) {
		t.Errorf("ResetPhases len = %d, want %d", len(result.ResetPhases), len(result.NewPlan.Phases))
	}
}

func TestRegeneratePlan_PreservesCompletedPhases(t *testing.T) {
	// Old plan: small weight (tiny_spec, implement, review) with implement completed
	oldPlan := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightSmall,
		Description: "Small task",
		Phases: []Phase{
			{ID: "tiny_spec", Name: "tiny_spec", Status: PhaseCompleted, CommitSHA: "spec123"},
			{ID: "implement", Name: "implement", Status: PhaseCompleted, CommitSHA: "abc123"},
			{ID: "review", Name: "review", Status: PhasePending},
		},
	}

	// Change to medium weight (spec, tdd_write, implement, review, docs) - implement and review exist in both
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightMedium}

	result, err := RegeneratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("RegeneratePlan() failed: %v", err)
	}

	// Check implement phase status was preserved
	implementPhase := result.NewPlan.GetPhase("implement")
	if implementPhase == nil {
		t.Fatal("implement phase not found")
	}
	if implementPhase.Status != PhaseCompleted {
		t.Errorf("implement status = %s, want completed", implementPhase.Status)
	}
	if implementPhase.CommitSHA != "abc123" {
		t.Errorf("implement CommitSHA = %s, want abc123", implementPhase.CommitSHA)
	}

	// Check review phase was reset (was pending, stays pending)
	reviewPhase := result.NewPlan.GetPhase("review")
	if reviewPhase == nil {
		t.Fatal("review phase not found")
	}
	if reviewPhase.Status != PhasePending {
		t.Errorf("review status = %s, want pending", reviewPhase.Status)
	}

	// Check docs phase exists (new in medium) and is pending
	docsPhase := result.NewPlan.GetPhase("docs")
	if docsPhase == nil {
		t.Fatal("docs phase not found")
	}
	if docsPhase.Status != PhasePending {
		t.Errorf("docs status = %s, want pending", docsPhase.Status)
	}

	// Verify PreservedPhases contains implement
	found := false
	for _, p := range result.PreservedPhases {
		if p == "implement" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("PreservedPhases = %v, want to contain 'implement'", result.PreservedPhases)
	}
}

func TestRegeneratePlan_PreservesSkippedPhases(t *testing.T) {
	// Old plan with skipped phase
	oldPlan := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightMedium,
		Phases: []Phase{
			{ID: "spec", Name: "spec", Status: PhaseCompleted, CommitSHA: "spec123"},
			{ID: "tdd_write", Name: "tdd_write", Status: PhaseCompleted, CommitSHA: "tdd123"},
			{ID: "implement", Name: "implement", Status: PhaseCompleted, CommitSHA: "abc123"},
			{ID: "review", Name: "review", Status: PhaseSkipped},
			{ID: "docs", Name: "docs", Status: PhasePending},
		},
	}

	// Keep same weight but regenerate
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightMedium}

	result, err := RegeneratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("RegeneratePlan() failed: %v", err)
	}

	// Check review phase status (skipped) was preserved
	reviewPhase := result.NewPlan.GetPhase("review")
	if reviewPhase == nil {
		t.Fatal("review phase not found")
	}
	if reviewPhase.Status != PhaseSkipped {
		t.Errorf("review status = %s, want skipped", reviewPhase.Status)
	}

	// Verify both implement and review are in PreservedPhases
	preservedSet := make(map[string]bool)
	for _, p := range result.PreservedPhases {
		preservedSet[p] = true
	}
	if !preservedSet["implement"] {
		t.Error("PreservedPhases should contain 'implement'")
	}
	if !preservedSet["review"] {
		t.Error("PreservedPhases should contain 'review'")
	}
}

func TestRegeneratePlan_DoesNotPreserveRunningOrFailed(t *testing.T) {
	// Old plan with running and failed phases (no completed phases)
	oldPlan := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightMedium,
		Phases: []Phase{
			{ID: "spec", Name: "spec", Status: PhaseRunning},
			{ID: "tdd_write", Name: "tdd_write", Status: PhaseFailed},
			{ID: "implement", Name: "implement", Status: PhasePending},
			{ID: "review", Name: "review", Status: PhasePending},
			{ID: "docs", Name: "docs", Status: PhasePending},
		},
	}

	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightMedium}

	result, err := RegeneratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("RegeneratePlan() failed: %v", err)
	}

	// All phases should be reset (running/failed not preserved)
	for _, phase := range result.NewPlan.Phases {
		if phase.Status != PhasePending {
			t.Errorf("phase %s status = %s, want pending", phase.ID, phase.Status)
		}
	}

	// PreservedPhases should be empty
	if len(result.PreservedPhases) != 0 {
		t.Errorf("PreservedPhases = %v, want empty", result.PreservedPhases)
	}
}

func TestRegeneratePlan_WeightDowngrade(t *testing.T) {
	// Old plan: large weight with some phases completed
	oldPlan := &Plan{
		Version:     1,
		TaskID:      "TASK-001",
		Weight:      task.WeightLarge,
		Phases: []Phase{
			{ID: "spec", Name: "spec", Status: PhaseCompleted, CommitSHA: "spec123"},
			{ID: "design", Name: "design", Status: PhaseCompleted, CommitSHA: "design123"},
			{ID: "tdd_write", Name: "tdd_write", Status: PhaseCompleted, CommitSHA: "tdd123"},
			{ID: "tasks", Name: "tasks", Status: PhaseCompleted, CommitSHA: "tasks123"},
			{ID: "implement", Name: "implement", Status: PhaseCompleted, CommitSHA: "impl123"},
			{ID: "review", Name: "review", Status: PhasePending},
			{ID: "docs", Name: "docs", Status: PhasePending},
			{ID: "validate", Name: "validate", Status: PhasePending},
		},
	}

	// Downgrade to small weight (tiny_spec, implement, review) - spec, design, tdd_write, tasks, docs, validate don't exist
	tsk := &task.Task{ID: "TASK-001", Weight: task.WeightSmall}

	result, err := RegeneratePlan(tsk, oldPlan)
	if err != nil {
		t.Fatalf("RegeneratePlan() failed: %v", err)
	}

	// Small weight should have 3 phases (tiny_spec, implement, review)
	if len(result.NewPlan.Phases) != 3 {
		t.Errorf("NewPlan phases = %d, want 3", len(result.NewPlan.Phases))
	}

	// implement should be preserved (completed in old, exists in new)
	implementPhase := result.NewPlan.GetPhase("implement")
	if implementPhase == nil {
		t.Fatal("implement phase not found")
	}
	if implementPhase.Status != PhaseCompleted {
		t.Errorf("implement status = %s, want completed", implementPhase.Status)
	}

	// spec should not exist in new plan (small uses tiny_spec instead)
	specPhase := result.NewPlan.GetPhase("spec")
	if specPhase != nil {
		t.Error("spec phase should not exist in small weight plan")
	}
}
