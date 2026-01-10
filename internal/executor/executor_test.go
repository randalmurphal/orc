package executor

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.ClaudePath != "claude" {
		t.Errorf("ClaudePath = %s, want claude", cfg.ClaudePath)
	}

	if cfg.Model == "" {
		t.Error("Model is empty")
	}

	if cfg.MaxIterations != 30 {
		t.Errorf("MaxIterations = %d, want 30", cfg.MaxIterations)
	}

	if cfg.Timeout != 10*time.Minute {
		t.Errorf("Timeout = %v, want 10m", cfg.Timeout)
	}

	if cfg.BranchPrefix != "orc/" {
		t.Errorf("BranchPrefix = %s, want orc/", cfg.BranchPrefix)
	}

	if cfg.CommitPrefix != "[orc]" {
		t.Errorf("CommitPrefix = %s, want [orc]", cfg.CommitPrefix)
	}

	if !cfg.DangerouslySkipPermissions {
		t.Error("DangerouslySkipPermissions should be true by default")
	}

	if !cfg.EnableCheckpoints {
		t.Error("EnableCheckpoints should be true by default")
	}
}

func TestNew(t *testing.T) {
	cfg := DefaultConfig()
	e := New(cfg)

	if e == nil {
		t.Fatal("New() returned nil")
	}

	if e.config == nil {
		t.Error("executor config is nil")
	}

	if e.client == nil {
		t.Error("executor client is nil")
	}

	if e.gitOps == nil {
		t.Error("executor gitOps is nil")
	}

	if e.checkpointStore == nil {
		t.Error("executor checkpointStore is nil when EnableCheckpoints=true")
	}
}

func TestNewWithNilConfig(t *testing.T) {
	e := New(nil)

	if e == nil {
		t.Fatal("New(nil) returned nil")
	}

	// Should use defaults
	if e.config.MaxIterations != 30 {
		t.Errorf("MaxIterations = %d, want 30", e.config.MaxIterations)
	}
}

func TestNewWithoutCheckpoints(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EnableCheckpoints = false
	e := New(cfg)

	if e.checkpointStore != nil {
		t.Error("checkpointStore should be nil when EnableCheckpoints=false")
	}
}

func TestRenderTemplate(t *testing.T) {
	e := New(DefaultConfig())

	state := PhaseState{
		TaskID:    "TASK-001",
		TaskTitle: "Add feature X",
		Phase:     "implement",
		Weight:    "medium",
		Iteration: 3,
	}

	tmpl := "Task: {{TASK_ID}} - {{TASK_TITLE}}, Phase: {{PHASE}}, Weight: {{WEIGHT}}, Iteration: {{ITERATION}}"
	result := e.renderTemplate(tmpl, state)

	expected := "Task: TASK-001 - Add feature X, Phase: implement, Weight: medium, Iteration: 3"
	if result != expected {
		t.Errorf("renderTemplate() = %q, want %q", result, expected)
	}
}

func TestRenderTemplateWithPriorContent(t *testing.T) {
	e := New(DefaultConfig())

	state := PhaseState{
		TaskID:          "TASK-001",
		TaskTitle:       "Build system",
		Phase:           "implement",
		Weight:          "large",
		ResearchContent: "Research findings here",
		SpecContent:     "Spec document here",
		DesignContent:   "Design document here",
	}

	tmpl := `Research: {{RESEARCH_CONTENT}}
Spec: {{SPEC_CONTENT}}
Design: {{DESIGN_CONTENT}}`

	result := e.renderTemplate(tmpl, state)

	if result != `Research: Research findings here
Spec: Spec document here
Design: Design document here` {
		t.Errorf("renderTemplate() with prior content failed: %s", result)
	}
}

func TestPhaseState(t *testing.T) {
	state := PhaseState{
		TaskID:    "TASK-001",
		TaskTitle: "Test task",
		Phase:     "implement",
		Weight:    "small",
	}

	if state.TaskID != "TASK-001" {
		t.Errorf("TaskID = %s, want TASK-001", state.TaskID)
	}

	if state.Complete {
		t.Error("Complete should be false by default")
	}

	if state.Blocked {
		t.Error("Blocked should be false by default")
	}

	if state.Iteration != 0 {
		t.Errorf("Iteration = %d, want 0", state.Iteration)
	}
}

func TestResult(t *testing.T) {
	result := &Result{
		Phase:        "implement",
		Iterations:   5,
		Duration:     30 * time.Second,
		Output:       "Implementation complete",
		CommitSHA:    "abc123",
		InputTokens:  1000,
		OutputTokens: 500,
	}

	if result.Phase != "implement" {
		t.Errorf("Phase = %s, want implement", result.Phase)
	}

	if result.Iterations != 5 {
		t.Errorf("Iterations = %d, want 5", result.Iterations)
	}

	if result.Duration != 30*time.Second {
		t.Errorf("Duration = %v, want 30s", result.Duration)
	}

	if result.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %s, want abc123", result.CommitSHA)
	}
}
