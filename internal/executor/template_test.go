package executor

import (
	"testing"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

func TestRenderTemplateFunc(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     TemplateVars
		want     string
	}{
		{
			name:     "all variables substituted",
			template: "Task: {{TASK_ID}} - {{TASK_TITLE}}, Phase: {{PHASE}}, Weight: {{WEIGHT}}, Iteration: {{ITERATION}}",
			vars: TemplateVars{
				TaskID:    "TASK-001",
				TaskTitle: "Add feature X",
				Phase:     "implement",
				Weight:    "medium",
				Iteration: 3,
			},
			want: "Task: TASK-001 - Add feature X, Phase: implement, Weight: medium, Iteration: 3",
		},
		{
			name:     "missing variable replaced with empty",
			template: "Task: {{TASK_ID}} - Retry: {{RETRY_CONTEXT}}",
			vars: TemplateVars{
				TaskID:       "TASK-002",
				RetryContext: "", // Empty
			},
			want: "Task: TASK-002 - Retry: ",
		},
		{
			name:     "empty template returns empty",
			template: "",
			vars: TemplateVars{
				TaskID:    "TASK-003",
				TaskTitle: "Test",
			},
			want: "",
		},
		{
			name:     "no variables returns original",
			template: "This is a plain text template with no variables.",
			vars: TemplateVars{
				TaskID: "TASK-004",
			},
			want: "This is a plain text template with no variables.",
		},
		{
			name:     "prior content variables",
			template: "Research: {{RESEARCH_CONTENT}}\nSpec: {{SPEC_CONTENT}}\nDesign: {{DESIGN_CONTENT}}",
			vars: TemplateVars{
				ResearchContent: "Research findings here",
				SpecContent:     "Spec document here",
				DesignContent:   "Design document here",
			},
			want: "Research: Research findings here\nSpec: Spec document here\nDesign: Design document here",
		},
		{
			name:     "task description",
			template: "Title: {{TASK_TITLE}}\nDescription: {{TASK_DESCRIPTION}}",
			vars: TemplateVars{
				TaskTitle:       "Add feature",
				TaskDescription: "Add a new button to the UI",
			},
			want: "Title: Add feature\nDescription: Add a new button to the UI",
		},
		{
			name:     "retry context",
			template: "Task: {{TASK_ID}}\nRetry info: {{RETRY_CONTEXT}}",
			vars: TemplateVars{
				TaskID:       "TASK-005",
				RetryContext: "Previous attempt failed because tests didn't pass",
			},
			want: "Task: TASK-005\nRetry info: Previous attempt failed because tests didn't pass",
		},
		{
			name:     "iteration zero",
			template: "Iteration: {{ITERATION}}",
			vars: TemplateVars{
				Iteration: 0,
			},
			want: "Iteration: 0",
		},
		{
			name:     "multiple occurrences of same variable",
			template: "{{TASK_ID}} is the task. The task is {{TASK_ID}}.",
			vars: TemplateVars{
				TaskID: "TASK-006",
			},
			want: "TASK-006 is the task. The task is TASK-006.",
		},
		{
			name:     "unknown variable left as is",
			template: "Task: {{TASK_ID}}, Unknown: {{UNKNOWN_VAR}}",
			vars: TemplateVars{
				TaskID: "TASK-007",
			},
			want: "Task: TASK-007, Unknown: {{UNKNOWN_VAR}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderTemplate(tt.template, tt.vars)
			if got != tt.want {
				t.Errorf("RenderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildTemplateVars(t *testing.T) {
	tests := []struct {
		name         string
		task         *task.Task
		phase        *plan.Phase
		state        *state.State
		iteration    int
		retryContext string
		wantID       string
		wantTitle    string
		wantPhase    string
		wantWeight   string
		wantIter     int
		wantRetry    string
	}{
		{
			name: "basic task context",
			task: &task.Task{
				ID:          "TASK-001",
				Title:       "Test Task",
				Description: "Test description",
				Weight:      task.WeightSmall,
			},
			phase: &plan.Phase{
				ID:   "implement",
				Name: "Implementation",
			},
			state:        nil,
			iteration:    1,
			retryContext: "",
			wantID:       "TASK-001",
			wantTitle:    "Test Task",
			wantPhase:    "implement",
			wantWeight:   "small",
			wantIter:     1,
			wantRetry:    "",
		},
		{
			name: "with retry context",
			task: &task.Task{
				ID:     "TASK-002",
				Title:  "Retry Task",
				Weight: task.WeightMedium,
			},
			phase: &plan.Phase{
				ID: "test",
			},
			state:        state.New("TASK-002"),
			iteration:    2,
			retryContext: "Test failed: assertion error",
			wantID:       "TASK-002",
			wantTitle:    "Retry Task",
			wantPhase:    "test",
			wantWeight:   "medium",
			wantIter:     2,
			wantRetry:    "Test failed: assertion error",
		},
		{
			name: "nil state - empty prior content",
			task: &task.Task{
				ID:     "TASK-003",
				Title:  "No State Task",
				Weight: task.WeightLarge,
			},
			phase: &plan.Phase{
				ID: "spec",
			},
			state:        nil,
			iteration:    0,
			retryContext: "",
			wantID:       "TASK-003",
			wantTitle:    "No State Task",
			wantPhase:    "spec",
			wantWeight:   "large",
			wantIter:     0,
			wantRetry:    "",
		},
		{
			name: "with state - populates prior content fields",
			task: &task.Task{
				ID:     "TASK-004",
				Title:  "Stateful Task",
				Weight: task.WeightGreenfield,
			},
			phase: &plan.Phase{
				ID: "implement",
			},
			state:        createStateWithCompletedPhases("TASK-004"),
			iteration:    5,
			retryContext: "",
			wantID:       "TASK-004",
			wantTitle:    "Stateful Task",
			wantPhase:    "implement",
			wantWeight:   "greenfield",
			wantIter:     5,
			wantRetry:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTemplateVars(tt.task, tt.phase, tt.state, tt.iteration, tt.retryContext)

			if got.TaskID != tt.wantID {
				t.Errorf("TaskID = %q, want %q", got.TaskID, tt.wantID)
			}
			if got.TaskTitle != tt.wantTitle {
				t.Errorf("TaskTitle = %q, want %q", got.TaskTitle, tt.wantTitle)
			}
			if got.Phase != tt.wantPhase {
				t.Errorf("Phase = %q, want %q", got.Phase, tt.wantPhase)
			}
			if got.Weight != tt.wantWeight {
				t.Errorf("Weight = %q, want %q", got.Weight, tt.wantWeight)
			}
			if got.Iteration != tt.wantIter {
				t.Errorf("Iteration = %d, want %d", got.Iteration, tt.wantIter)
			}
			if got.RetryContext != tt.wantRetry {
				t.Errorf("RetryContext = %q, want %q", got.RetryContext, tt.wantRetry)
			}
		})
	}
}

func TestLoadPromptTemplate(t *testing.T) {
	tests := []struct {
		name      string
		phase     *plan.Phase
		wantErr   bool
		wantInErr string
		checkFunc func(string) bool
	}{
		{
			name: "inline prompt returns inline",
			phase: &plan.Phase{
				ID:     "custom",
				Prompt: "This is an inline prompt for {{TASK_TITLE}}",
			},
			wantErr: false,
			checkFunc: func(s string) bool {
				return s == "This is an inline prompt for {{TASK_TITLE}}"
			},
		},
		{
			name: "empty inline loads from template",
			phase: &plan.Phase{
				ID:     "implement",
				Prompt: "", // Will try to load from templates
			},
			wantErr: false,
			checkFunc: func(s string) bool {
				// Should load from embedded template
				return len(s) > 0
			},
		},
		{
			name:      "nil phase returns error",
			phase:     nil,
			wantErr:   true,
			wantInErr: "nil",
		},
		{
			name: "missing phase template returns error",
			phase: &plan.Phase{
				ID:     "nonexistent-phase-that-does-not-exist",
				Prompt: "",
			},
			wantErr:   true,
			wantInErr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadPromptTemplate(tt.phase)

			if tt.wantErr {
				if err == nil {
					t.Errorf("LoadPromptTemplate() error = nil, want error containing %q", tt.wantInErr)
					return
				}
				if tt.wantInErr != "" && !contains(err.Error(), tt.wantInErr) {
					t.Errorf("LoadPromptTemplate() error = %v, want error containing %q", err, tt.wantInErr)
				}
				return
			}

			if err != nil {
				t.Errorf("LoadPromptTemplate() unexpected error = %v", err)
				return
			}

			if tt.checkFunc != nil && !tt.checkFunc(got) {
				t.Errorf("LoadPromptTemplate() = %q, check failed", got)
			}
		})
	}
}

func TestTemplateVars_ZeroValue(t *testing.T) {
	var vars TemplateVars

	// Zero value should be safe to use
	result := RenderTemplate("{{TASK_ID}}:{{ITERATION}}", vars)
	if result != ":0" {
		t.Errorf("Zero value render = %q, want %q", result, ":0")
	}
}

// Helper functions

func createStateWithCompletedPhases(taskID string) *state.State {
	s := state.New(taskID)
	s.StartPhase("research")
	s.CompletePhase("research", "sha1")
	s.StartPhase("spec")
	s.CompletePhase("spec", "sha2")
	s.StartPhase("design")
	s.CompletePhase("design", "sha3")
	return s
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRenderTemplate_WorktreeVariables(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     TemplateVars
		want     string
	}{
		{
			name:     "worktree path substitution",
			template: "Working in: {{WORKTREE_PATH}}",
			vars: TemplateVars{
				WorktreePath: "/home/user/.orc/worktrees/orc-TASK-001",
			},
			want: "Working in: /home/user/.orc/worktrees/orc-TASK-001",
		},
		{
			name:     "task branch substitution",
			template: "Branch: {{TASK_BRANCH}}",
			vars: TemplateVars{
				TaskBranch: "orc/TASK-001",
			},
			want: "Branch: orc/TASK-001",
		},
		{
			name:     "target branch substitution",
			template: "Merge to: {{TARGET_BRANCH}}",
			vars: TemplateVars{
				TargetBranch: "main",
			},
			want: "Merge to: main",
		},
		{
			name:     "all worktree variables together",
			template: "Path: {{WORKTREE_PATH}}, Branch: {{TASK_BRANCH}}, Target: {{TARGET_BRANCH}}",
			vars: TemplateVars{
				WorktreePath: "/tmp/worktree",
				TaskBranch:   "orc/TASK-002",
				TargetBranch: "develop",
			},
			want: "Path: /tmp/worktree, Branch: orc/TASK-002, Target: develop",
		},
		{
			name:     "empty worktree variables replaced with empty",
			template: "{{WORKTREE_PATH}}|{{TASK_BRANCH}}|{{TARGET_BRANCH}}",
			vars:     TemplateVars{},
			want:     "||",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderTemplate(tt.template, tt.vars)
			if got != tt.want {
				t.Errorf("RenderTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildTemplateVarsWithWorktree(t *testing.T) {
	taskObj := &task.Task{
		ID:          "TASK-001",
		Title:       "Test Task",
		Description: "Test description",
		Weight:      task.WeightMedium,
	}
	phaseObj := &plan.Phase{
		ID:   "implement",
		Name: "Implementation",
	}
	wctx := WorktreeContext{
		WorktreePath: "/home/user/.orc/worktrees/orc-TASK-001",
		TaskBranch:   "orc/TASK-001",
		TargetBranch: "main",
	}

	vars := BuildTemplateVarsWithWorktree(taskObj, phaseObj, nil, 1, "", wctx)

	// Check worktree context fields
	if vars.WorktreePath != wctx.WorktreePath {
		t.Errorf("WorktreePath = %q, want %q", vars.WorktreePath, wctx.WorktreePath)
	}
	if vars.TaskBranch != wctx.TaskBranch {
		t.Errorf("TaskBranch = %q, want %q", vars.TaskBranch, wctx.TaskBranch)
	}
	if vars.TargetBranch != wctx.TargetBranch {
		t.Errorf("TargetBranch = %q, want %q", vars.TargetBranch, wctx.TargetBranch)
	}

	// Check that regular fields are also populated
	if vars.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want TASK-001", vars.TaskID)
	}
	if vars.Phase != "implement" {
		t.Errorf("Phase = %q, want implement", vars.Phase)
	}
}

func TestTemplateVars_WithWorktreeContext(t *testing.T) {
	vars := TemplateVars{
		TaskID:    "TASK-001",
		TaskTitle: "Test",
		Phase:     "spec",
		Weight:    "medium",
	}

	wctx := WorktreeContext{
		WorktreePath: "/tmp/worktree",
		TaskBranch:   "orc/TASK-001",
		TargetBranch: "develop",
	}

	result := vars.WithWorktreeContext(wctx)

	// Worktree fields should be populated
	if result.WorktreePath != "/tmp/worktree" {
		t.Errorf("WorktreePath = %q, want /tmp/worktree", result.WorktreePath)
	}
	if result.TaskBranch != "orc/TASK-001" {
		t.Errorf("TaskBranch = %q, want orc/TASK-001", result.TaskBranch)
	}
	if result.TargetBranch != "develop" {
		t.Errorf("TargetBranch = %q, want develop", result.TargetBranch)
	}

	// Original fields should still be there
	if result.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want TASK-001", result.TaskID)
	}

	// Original vars should be unmodified (value receiver)
	if vars.WorktreePath != "" {
		t.Errorf("original WorktreePath modified to %q, should be empty", vars.WorktreePath)
	}
}

func TestExecutorConfig_GetTargetBranch(t *testing.T) {
	tests := []struct {
		name   string
		config ExecutorConfig
		want   string
	}{
		{
			name:   "empty target branch defaults to main",
			config: ExecutorConfig{},
			want:   "main",
		},
		{
			name: "explicit target branch returned",
			config: ExecutorConfig{
				TargetBranch: "develop",
			},
			want: "develop",
		},
		{
			name: "custom target branch",
			config: ExecutorConfig{
				TargetBranch: "production",
			},
			want: "production",
		},
		{
			name: "main is explicit (not default)",
			config: ExecutorConfig{
				TargetBranch: "main",
			},
			want: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetTargetBranch()
			if got != tt.want {
				t.Errorf("GetTargetBranch() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultConfigForWeight_TargetBranch(t *testing.T) {
	weights := []task.Weight{
		task.WeightTrivial,
		task.WeightSmall,
		task.WeightMedium,
		task.WeightLarge,
		task.WeightGreenfield,
	}

	for _, w := range weights {
		t.Run(string(w), func(t *testing.T) {
			cfg := DefaultConfigForWeight(w)
			// All default configs should have empty TargetBranch (defaults to "main")
			if cfg.TargetBranch != "" {
				t.Errorf("DefaultConfigForWeight(%s).TargetBranch = %q, want empty", w, cfg.TargetBranch)
			}
			// GetTargetBranch should return "main" for all defaults
			if cfg.GetTargetBranch() != "main" {
				t.Errorf("DefaultConfigForWeight(%s).GetTargetBranch() = %q, want main", w, cfg.GetTargetBranch())
			}
		})
	}
}
