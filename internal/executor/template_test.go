package executor

import (
	"testing"

	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

func TestRenderTemplateFunc(t *testing.T) {
	t.Parallel()
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
			template: "Research: {{RESEARCH_CONTENT}}\nSpec: {{SPEC_CONTENT}}",
			vars: TemplateVars{
				ResearchContent: "Research findings here",
				SpecContent:     "Spec document here",
			},
			want: "Research: Research findings here\nSpec: Spec document here",
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
	t.Parallel()
	tests := []struct {
		name         string
		task         *task.Task
		phase        *Phase
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
			phase: &Phase{
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
			phase: &Phase{
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
			phase: &Phase{
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
				Weight: task.WeightLarge,
			},
			phase: &Phase{
				ID: "implement",
			},
			state:        createStateWithCompletedPhases("TASK-004"),
			iteration:    5,
			retryContext: "",
			wantID:       "TASK-004",
			wantTitle:    "Stateful Task",
			wantPhase:    "implement",
			wantWeight:   "large",
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
	t.Parallel()
	tests := []struct {
		name      string
		phase     *Phase
		wantErr   bool
		wantInErr string
		checkFunc func(string) bool
	}{
		{
			name: "inline prompt returns inline",
			phase: &Phase{
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
			phase: &Phase{
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
			phase: &Phase{
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	taskObj := &task.Task{
		ID:          "TASK-001",
		Title:       "Test Task",
		Description: "Test description",
		Weight:      task.WeightMedium,
	}
	phaseObj := &Phase{
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	weights := []task.Weight{
		task.WeightTrivial,
		task.WeightSmall,
		task.WeightMedium,
		task.WeightLarge,
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

func TestRenderTemplate_UITestingVariables(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		template string
		vars     TemplateVars
		want     string
	}{
		{
			name:     "requires ui testing true",
			template: "UI Testing: {{REQUIRES_UI_TESTING}}",
			vars: TemplateVars{
				RequiresUITesting: true,
			},
			want: "UI Testing: true",
		},
		{
			name:     "requires ui testing false",
			template: "UI Testing: {{REQUIRES_UI_TESTING}}",
			vars: TemplateVars{
				RequiresUITesting: false,
			},
			want: "UI Testing: ",
		},
		{
			name:     "screenshot directory",
			template: "Save to: {{SCREENSHOT_DIR}}/image.png",
			vars: TemplateVars{
				ScreenshotDir: "/path/to/screenshots",
			},
			want: "Save to: /path/to/screenshots/image.png",
		},
		{
			name:     "test results",
			template: "Previous results: {{TEST_RESULTS}}",
			vars: TemplateVars{
				TestResults: "All 10 tests passed",
			},
			want: "Previous results: All 10 tests passed",
		},
		{
			name:     "all ui testing variables together",
			template: "UI: {{REQUIRES_UI_TESTING}}, Dir: {{SCREENSHOT_DIR}}, Results: {{TEST_RESULTS}}",
			vars: TemplateVars{
				RequiresUITesting: true,
				ScreenshotDir:     "/screenshots",
				TestResults:       "Passed",
			},
			want: "UI: true, Dir: /screenshots, Results: Passed",
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

func TestTemplateVars_WithUITestingContext(t *testing.T) {
	t.Parallel()
	vars := TemplateVars{
		TaskID:    "TASK-001",
		TaskTitle: "Test",
		Phase:     "test",
		Weight:    "medium",
	}

	uiCtx := UITestingContext{
		RequiresUITesting: true,
		ScreenshotDir:     "/path/to/screenshots",
		TestResults:       "All tests passed",
	}

	result := vars.WithUITestingContext(uiCtx)

	// UI testing fields should be populated
	if !result.RequiresUITesting {
		t.Error("RequiresUITesting should be true")
	}
	if result.ScreenshotDir != "/path/to/screenshots" {
		t.Errorf("ScreenshotDir = %q, want /path/to/screenshots", result.ScreenshotDir)
	}
	if result.TestResults != "All tests passed" {
		t.Errorf("TestResults = %q, want 'All tests passed'", result.TestResults)
	}

	// Original fields should still be there
	if result.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want TASK-001", result.TaskID)
	}
	if result.Phase != "test" {
		t.Errorf("Phase = %q, want test", result.Phase)
	}

	// Original vars should be unmodified (value receiver)
	if vars.RequiresUITesting {
		t.Error("original RequiresUITesting should be false")
	}
	if vars.ScreenshotDir != "" {
		t.Errorf("original ScreenshotDir modified to %q, should be empty", vars.ScreenshotDir)
	}
}

func TestUITestingContext_ZeroValue(t *testing.T) {
	t.Parallel()
	var ctx UITestingContext

	// Zero value should be safe to use
	vars := TemplateVars{TaskID: "TASK-001"}
	result := vars.WithUITestingContext(ctx)

	if result.RequiresUITesting {
		t.Error("zero value RequiresUITesting should be false")
	}
	if result.ScreenshotDir != "" {
		t.Errorf("zero value ScreenshotDir = %q, want empty", result.ScreenshotDir)
	}
	if result.TestResults != "" {
		t.Errorf("zero value TestResults = %q, want empty", result.TestResults)
	}
}

func TestRenderTemplate_InitiativeVariables(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		template string
		vars     TemplateVars
		want     string
	}{
		{
			name:     "initiative id substitution",
			template: "Initiative: {{INITIATIVE_ID}}",
			vars: TemplateVars{
				InitiativeID: "INIT-001",
			},
			want: "Initiative: INIT-001",
		},
		{
			name:     "initiative title substitution",
			template: "Title: {{INITIATIVE_TITLE}}",
			vars: TemplateVars{
				InitiativeTitle: "User Authentication Overhaul",
			},
			want: "Title: User Authentication Overhaul",
		},
		{
			name:     "initiative vision substitution",
			template: "Vision: {{INITIATIVE_VISION}}",
			vars: TemplateVars{
				InitiativeVision: "Modernize auth to support SSO and OAuth2",
			},
			want: "Vision: Modernize auth to support SSO and OAuth2",
		},
		{
			name:     "initiative decisions substitution",
			template: "Decisions:\n{{INITIATIVE_DECISIONS}}",
			vars: TemplateVars{
				InitiativeDecisions: "- **DEC-001**: Use OAuth2 (Industry standard)",
			},
			want: "Decisions:\n- **DEC-001**: Use OAuth2 (Industry standard)",
		},
		{
			name:     "all initiative variables together",
			template: "{{INITIATIVE_ID}}: {{INITIATIVE_TITLE}} - {{INITIATIVE_VISION}}",
			vars: TemplateVars{
				InitiativeID:     "INIT-002",
				InitiativeTitle:  "Refactoring",
				InitiativeVision: "Clean up technical debt",
			},
			want: "INIT-002: Refactoring - Clean up technical debt",
		},
		{
			name:     "empty initiative variables replaced with empty",
			template: "[{{INITIATIVE_ID}}][{{INITIATIVE_TITLE}}][{{INITIATIVE_VISION}}][{{INITIATIVE_DECISIONS}}]",
			vars:     TemplateVars{},
			want:     "[][][][]",
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

func TestTemplateVars_WithInitiativeContext(t *testing.T) {
	t.Parallel()
	vars := TemplateVars{
		TaskID:    "TASK-001",
		TaskTitle: "Test",
		Phase:     "implement",
		Weight:    "medium",
	}

	initCtx := InitiativeContext{
		ID:     "INIT-001",
		Title:  "Auth Modernization",
		Vision: "Implement modern auth patterns",
		Decisions: []InitiativeDecision{
			{ID: "DEC-001", Decision: "Use OAuth2", Rationale: "Industry standard"},
			{ID: "DEC-002", Decision: "JWT tokens", Rationale: "Stateless auth"},
		},
	}

	result := vars.WithInitiativeContext(initCtx)

	// Initiative fields should be populated
	if result.InitiativeID != "INIT-001" {
		t.Errorf("InitiativeID = %q, want INIT-001", result.InitiativeID)
	}
	if result.InitiativeTitle != "Auth Modernization" {
		t.Errorf("InitiativeTitle = %q, want 'Auth Modernization'", result.InitiativeTitle)
	}
	if result.InitiativeVision != "Implement modern auth patterns" {
		t.Errorf("InitiativeVision = %q, want 'Implement modern auth patterns'", result.InitiativeVision)
	}
	if result.InitiativeDecisions == "" {
		t.Error("InitiativeDecisions should not be empty")
	}
	if !contains(result.InitiativeDecisions, "DEC-001") {
		t.Errorf("InitiativeDecisions should contain DEC-001, got %q", result.InitiativeDecisions)
	}
	if !contains(result.InitiativeDecisions, "DEC-002") {
		t.Errorf("InitiativeDecisions should contain DEC-002, got %q", result.InitiativeDecisions)
	}

	// Original fields should still be there
	if result.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want TASK-001", result.TaskID)
	}
	if result.Phase != "implement" {
		t.Errorf("Phase = %q, want implement", result.Phase)
	}

	// Original vars should be unmodified (value receiver)
	if vars.InitiativeID != "" {
		t.Errorf("original InitiativeID modified to %q, should be empty", vars.InitiativeID)
	}
	if vars.InitiativeVision != "" {
		t.Errorf("original InitiativeVision modified to %q, should be empty", vars.InitiativeVision)
	}
}

func TestInitiativeContext_FormatDecisions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		ctx       InitiativeContext
		want      string
		wantEmpty bool
	}{
		{
			name: "no decisions",
			ctx: InitiativeContext{
				ID:        "INIT-001",
				Decisions: nil,
			},
			wantEmpty: true,
		},
		{
			name: "single decision without rationale",
			ctx: InitiativeContext{
				ID: "INIT-001",
				Decisions: []InitiativeDecision{
					{ID: "DEC-001", Decision: "Use microservices"},
				},
			},
			want: "- **DEC-001**: Use microservices",
		},
		{
			name: "single decision with rationale",
			ctx: InitiativeContext{
				ID: "INIT-001",
				Decisions: []InitiativeDecision{
					{ID: "DEC-001", Decision: "Use microservices", Rationale: "Better scalability"},
				},
			},
			want: "- **DEC-001**: Use microservices (Better scalability)",
		},
		{
			name: "multiple decisions",
			ctx: InitiativeContext{
				ID: "INIT-001",
				Decisions: []InitiativeDecision{
					{ID: "DEC-001", Decision: "Use OAuth2", Rationale: "Industry standard"},
					{ID: "DEC-002", Decision: "JWT tokens"},
					{ID: "DEC-003", Decision: "Redis for sessions", Rationale: "Performance"},
				},
			},
			want: "- **DEC-001**: Use OAuth2 (Industry standard)\n- **DEC-002**: JWT tokens\n- **DEC-003**: Redis for sessions (Performance)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.FormatDecisions()
			if tt.wantEmpty {
				if got != "" {
					t.Errorf("FormatDecisions() = %q, want empty", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("FormatDecisions() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInitiativeContext_ZeroValue(t *testing.T) {
	t.Parallel()
	var ctx InitiativeContext

	// Zero value should be safe to use
	vars := TemplateVars{TaskID: "TASK-001"}
	result := vars.WithInitiativeContext(ctx)

	if result.InitiativeID != "" {
		t.Errorf("zero value InitiativeID = %q, want empty", result.InitiativeID)
	}
	if result.InitiativeTitle != "" {
		t.Errorf("zero value InitiativeTitle = %q, want empty", result.InitiativeTitle)
	}
	if result.InitiativeVision != "" {
		t.Errorf("zero value InitiativeVision = %q, want empty", result.InitiativeVision)
	}
	if result.InitiativeDecisions != "" {
		t.Errorf("zero value InitiativeDecisions = %q, want empty", result.InitiativeDecisions)
	}
}

func TestExtractVerificationResults(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name: "standard format with table",
			content: `# Implementation Summary

Some implementation details here.

### Verification Results

| ID | Criterion | Method | Result | Notes |
|----|-----------|--------|--------|-------|
| SC-1 | User can log out | npm test logout | ✅ PASS | 3 tests passed |
| SC-2 | Session invalidated | curl check | ✅ PASS | Cookie cleared |

### Next Steps

More content here.`,
			want: `| ID | Criterion | Method | Result | Notes |
|----|-----------|--------|--------|-------|
| SC-1 | User can log out | npm test logout | ✅ PASS | 3 tests passed |
| SC-2 | Session invalidated | curl check | ✅ PASS | Cookie cleared |`,
		},
		{
			name: "alternate format with ## header",
			content: `# Implementation

## Verification Results

| ID | Criterion | Result |
|----|-----------|--------|
| SC-1 | Feature works | ✅ PASS |

## Summary

Done.`,
			want: `| ID | Criterion | Result |
|----|-----------|--------|
| SC-1 | Feature works | ✅ PASS |`,
		},
		{
			name: "section at end of document",
			content: `# Summary

Implementation complete.

### Verification Results

| ID | Result |
|----|--------|
| SC-1 | ✅ PASS |`,
			want: `| ID | Result |
|----|--------|
| SC-1 | ✅ PASS |`,
		},
		{
			name: "no table characters returns empty",
			content: `### Verification Results

All criteria verified manually.
No issues found.

### Done`,
			want: "",
		},
		{
			name: "missing section returns empty",
			content: `# Implementation

Did some work.

### Summary

All done.`,
			want: "",
		},
		{
			name:    "empty content returns empty",
			content: "",
			want:    "",
		},
		{
			name: "section with mixed content extracts table",
			content: `### Verification Results

All criteria have been verified:

| ID | Criterion | Method | Result |
|----|-----------|--------|--------|
| SC-1 | API works | curl test | ✅ PASS |
| SC-2 | UI renders | browser check | ✅ PASS |

Summary: All passed.

### Commit`,
			want: `All criteria have been verified:

| ID | Criterion | Method | Result |
|----|-----------|--------|--------|
| SC-1 | API works | curl test | ✅ PASS |
| SC-2 | UI renders | browser check | ✅ PASS |

Summary: All passed.`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractVerificationResults(tt.content)
			if got != tt.want {
				t.Errorf("extractVerificationResults() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderTemplate_VerificationResultsVariable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		template string
		vars     TemplateVars
		want     string
	}{
		{
			name:     "verification results substitution",
			template: "Previous results:\n{{VERIFICATION_RESULTS}}",
			vars: TemplateVars{
				VerificationResults: "| SC-1 | ✅ PASS |",
			},
			want: "Previous results:\n| SC-1 | ✅ PASS |",
		},
		{
			name:     "empty verification results",
			template: "[{{VERIFICATION_RESULTS}}]",
			vars:     TemplateVars{},
			want:     "[]",
		},
		{
			name:     "multiline verification results",
			template: "Results:\n{{VERIFICATION_RESULTS}}\nEnd.",
			vars: TemplateVars{
				VerificationResults: "| ID | Result |\n|----|--------|\n| SC-1 | PASS |",
			},
			want: "Results:\n| ID | Result |\n|----|--------|\n| SC-1 | PASS |\nEnd.",
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

func TestRenderTemplate_InitiativeContextSection(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		vars         TemplateVars
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:      "no initiative - empty section",
			vars:      TemplateVars{TaskID: "TASK-001"},
			wantEmpty: true,
		},
		{
			name: "with initiative - has section header",
			vars: TemplateVars{
				TaskID:          "TASK-001",
				InitiativeID:    "INIT-001",
				InitiativeTitle: "Auth Overhaul",
			},
			wantContains: []string{
				"## Initiative Context",
				"INIT-001",
				"Auth Overhaul",
			},
		},
		{
			name: "with vision - includes vision section",
			vars: TemplateVars{
				TaskID:           "TASK-001",
				InitiativeID:     "INIT-001",
				InitiativeTitle:  "Auth Overhaul",
				InitiativeVision: "Modernize authentication",
			},
			wantContains: []string{
				"### Vision",
				"Modernize authentication",
			},
		},
		{
			name: "with decisions - includes decisions section",
			vars: TemplateVars{
				TaskID:              "TASK-001",
				InitiativeID:        "INIT-001",
				InitiativeTitle:     "Auth Overhaul",
				InitiativeDecisions: "- **DEC-001**: Use OAuth2",
			},
			wantContains: []string{
				"### Decisions",
				"DEC-001",
			},
		},
		{
			name: "full context - has alignment note",
			vars: TemplateVars{
				TaskID:              "TASK-001",
				InitiativeID:        "INIT-001",
				InitiativeTitle:     "Auth Overhaul",
				InitiativeVision:    "Modernize authentication",
				InitiativeDecisions: "- **DEC-001**: Use OAuth2",
			},
			wantContains: []string{
				"**Alignment**",
				"aligns with the initiative vision",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			template := "{{INITIATIVE_CONTEXT}}"
			got := RenderTemplate(template, tt.vars)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("RenderTemplate() = %q, want empty", got)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("RenderTemplate() should contain %q, got:\n%s", want, got)
				}
			}
		})
	}
}
