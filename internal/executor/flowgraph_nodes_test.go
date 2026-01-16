package executor

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// Tests for flowgraph node builders and phase execution methods.
// These methods are currently in executor.go and will be extracted to
// flowgraph_nodes.go during the refactoring process.

func TestBuildPromptNode_LoadsTemplate(t *testing.T) {
	// Create a temp dir with a template
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates", "prompts")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Write a test template
	templateContent := `# Implementation Phase

Task: {{TASK_ID}} - {{TASK_TITLE}}
Weight: {{WEIGHT}}
Description: {{TASK_DESCRIPTION}}

Please implement the requested feature.
`
	if err := os.WriteFile(filepath.Join(templatesDir, "implement.md"), []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	// Create executor with custom templates dir
	cfg := DefaultConfig()
	cfg.TemplatesDir = filepath.Join(tmpDir, "templates")
	e := New(cfg)

	// Create phase that matches template name
	phase := &plan.Phase{
		ID:   "implement",
		Name: "implement",
	}

	nodeFunc := e.buildPromptNode(phase)

	initialState := PhaseState{
		TaskID:          "TASK-001",
		TaskTitle:       "Add new feature",
		TaskDescription: "Implement a button that does something",
		Phase:           "implement",
		Weight:          "medium",
	}

	// Execute the node
	result, err := nodeFunc(nil, initialState)
	if err != nil {
		t.Fatalf("buildPromptNode failed: %v", err)
	}

	// Verify template was loaded and rendered
	if !strings.Contains(result.Prompt, "TASK-001") {
		t.Errorf("prompt should contain task ID, got: %s", result.Prompt)
	}
	if !strings.Contains(result.Prompt, "Add new feature") {
		t.Errorf("prompt should contain task title, got: %s", result.Prompt)
	}
	if !strings.Contains(result.Prompt, "medium") {
		t.Errorf("prompt should contain weight, got: %s", result.Prompt)
	}
	if !strings.Contains(result.Prompt, "Implementation Phase") {
		t.Errorf("prompt should contain template header, got: %s", result.Prompt)
	}

	// Verify iteration was incremented
	if result.Iteration != 1 {
		t.Errorf("iteration = %d, want 1", result.Iteration)
	}
}

func TestBuildPromptNode_FallsBackToInlinePrompt(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TemplatesDir = "/nonexistent/path"
	e := New(cfg)

	// Create phase with inline prompt (no template file will exist)
	phase := &plan.Phase{
		ID:     "custom",
		Name:   "Custom Phase",
		Prompt: "Do work for {{TASK_TITLE}} with {{WEIGHT}} priority",
	}

	nodeFunc := e.buildPromptNode(phase)

	initialState := PhaseState{
		TaskID:    "TASK-002",
		TaskTitle: "Custom Task",
		Phase:     "custom",
		Weight:    "small",
	}

	result, err := nodeFunc(nil, initialState)
	if err != nil {
		t.Fatalf("buildPromptNode failed: %v", err)
	}

	expected := "Do work for Custom Task with small priority"
	if result.Prompt != expected {
		t.Errorf("prompt = %q, want %q", result.Prompt, expected)
	}
}

func TestBuildPromptNode_ErrorsWithNoPrompt(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TemplatesDir = "/nonexistent/path"
	e := New(cfg)

	// Create phase with no inline prompt and no template
	phase := &plan.Phase{
		ID:   "missing",
		Name: "Missing Phase",
		// No Prompt field
	}

	nodeFunc := e.buildPromptNode(phase)

	initialState := PhaseState{
		TaskID: "TASK-003",
		Phase:  "missing",
	}

	_, err := nodeFunc(nil, initialState)
	if err == nil {
		t.Error("buildPromptNode should fail when no prompt is available")
	}
	if !strings.Contains(err.Error(), "no prompt template found") {
		t.Errorf("error should mention 'no prompt template found', got: %s", err.Error())
	}
}

func TestBuildPromptNode_TriesIDAfterName(t *testing.T) {
	// Create a temp dir with a template named by ID
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates", "prompts")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("failed to create templates dir: %v", err)
	}

	// Write a template with ID name (not Name)
	if err := os.WriteFile(filepath.Join(templatesDir, "spec.md"), []byte("Spec template for {{TASK_TITLE}}"), 0644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}

	cfg := DefaultConfig()
	cfg.TemplatesDir = filepath.Join(tmpDir, "templates")
	e := New(cfg)

	// Phase has different Name and ID
	phase := &plan.Phase{
		ID:   "spec",
		Name: "Specification",
	}

	nodeFunc := e.buildPromptNode(phase)

	initialState := PhaseState{
		TaskID:    "TASK-004",
		TaskTitle: "New Feature",
		Phase:     "spec",
	}

	result, err := nodeFunc(nil, initialState)
	if err != nil {
		t.Fatalf("buildPromptNode failed: %v", err)
	}

	if !strings.Contains(result.Prompt, "New Feature") {
		t.Errorf("prompt should contain task title from ID-based template, got: %s", result.Prompt)
	}
}

func TestCheckCompletionNode_DetectsComplete(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	phase := &plan.Phase{ID: "implement"}
	testState := state.New("TASK-COMP-001")

	nodeFunc := e.checkCompletionNode(phase, testState)

	// State with completion marker
	ps := PhaseState{
		TaskID:   "TASK-COMP-001",
		Phase:    "implement",
		Response: "Work done! <phase_complete>true</phase_complete> All finished.",
	}

	result, err := nodeFunc(nil, ps)
	if err != nil {
		t.Fatalf("checkCompletionNode failed: %v", err)
	}

	if !result.Complete {
		t.Error("expected Complete to be true")
	}
	if result.Blocked {
		t.Error("expected Blocked to be false")
	}
}

func TestCheckCompletionNode_DetectsPhaseSpecificComplete(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	phase := &plan.Phase{ID: "test"}
	testState := state.New("TASK-COMP-002")

	nodeFunc := e.checkCompletionNode(phase, testState)

	// State with phase-specific completion marker
	ps := PhaseState{
		TaskID:   "TASK-COMP-002",
		Phase:    "test",
		Response: "Tests passed! <test_complete>true</test_complete>",
	}

	result, err := nodeFunc(nil, ps)
	if err != nil {
		t.Fatalf("checkCompletionNode failed: %v", err)
	}

	if !result.Complete {
		t.Error("expected Complete to be true for phase-specific marker")
	}
}

func TestCheckCompletionNode_DetectsBlocked(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	phase := &plan.Phase{ID: "spec"}
	testState := state.New("TASK-BLOCK-001")

	nodeFunc := e.checkCompletionNode(phase, testState)

	// State with blocked marker
	ps := PhaseState{
		TaskID:   "TASK-BLOCK-001",
		Phase:    "spec",
		Response: "I need more information. <phase_blocked>Missing API specification</phase_blocked>",
	}

	result, err := nodeFunc(nil, ps)
	if err != nil {
		t.Fatalf("checkCompletionNode failed: %v", err)
	}

	if !result.Blocked {
		t.Error("expected Blocked to be true")
	}
	if result.Complete {
		t.Error("expected Complete to be false when blocked")
	}
}

func TestCheckCompletionNode_NoMarkers(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	phase := &plan.Phase{ID: "implement"}
	testState := state.New("TASK-CONT-001")

	nodeFunc := e.checkCompletionNode(phase, testState)

	// State without any markers - should continue
	ps := PhaseState{
		TaskID:   "TASK-CONT-001",
		Phase:    "implement",
		Response: "Still working on it...",
	}

	result, err := nodeFunc(nil, ps)
	if err != nil {
		t.Fatalf("checkCompletionNode failed: %v", err)
	}

	if result.Complete {
		t.Error("expected Complete to be false")
	}
	if result.Blocked {
		t.Error("expected Blocked to be false")
	}
}

func TestCheckCompletionNode_UpdatesState(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	phase := &plan.Phase{ID: "implement"}
	testState := state.New("TASK-STATE-001")
	testState.StartPhase("implement")

	nodeFunc := e.checkCompletionNode(phase, testState)

	ps := PhaseState{
		TaskID:       "TASK-STATE-001",
		Phase:        "implement",
		Response:     "<phase_complete>true</phase_complete>",
		InputTokens:  100,
		OutputTokens: 50,
	}

	_, err := nodeFunc(nil, ps)
	if err != nil {
		t.Fatalf("checkCompletionNode failed: %v", err)
	}

	// Verify state was updated
	implPhase := testState.Phases["implement"]
	if implPhase.Iterations != 1 {
		t.Errorf("iterations = %d, want 1", implPhase.Iterations)
	}
	if testState.Tokens.InputTokens != 100 {
		t.Errorf("input tokens = %d, want 100", testState.Tokens.InputTokens)
	}
	if testState.Tokens.OutputTokens != 50 {
		t.Errorf("output tokens = %d, want 50", testState.Tokens.OutputTokens)
	}
}

func TestCheckCompletionNode_NilState(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	phase := &plan.Phase{ID: "implement"}

	// Pass nil state - should not panic
	nodeFunc := e.checkCompletionNode(phase, nil)

	ps := PhaseState{
		TaskID:   "TASK-NIL-001",
		Phase:    "implement",
		Response: "<phase_complete>true</phase_complete>",
	}

	result, err := nodeFunc(nil, ps)
	if err != nil {
		t.Fatalf("checkCompletionNode failed with nil state: %v", err)
	}

	if !result.Complete {
		t.Error("expected Complete to be true even with nil state")
	}
}

func TestSaveTranscript_WritesToFile(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)

	ps := PhaseState{
		TaskID:       "TASK-TRANS-001",
		Phase:        "implement",
		Iteration:    1,
		Prompt:       "Implement the feature",
		Response:     "Here is the implementation...",
		InputTokens:  100,
		OutputTokens: 250,
		Complete:     false,
		Blocked:      false,
	}

	err := e.saveTranscript(ps)
	if err != nil {
		t.Fatalf("saveTranscript failed: %v", err)
	}

	// Verify file was created
	expectedPath := filepath.Join(tmpDir, ".orc", "tasks", "TASK-TRANS-001", "transcripts", "implement-001.md")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("transcript file not created at %s", expectedPath)
	}

	// Verify file contents
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("failed to read transcript: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "implement - Iteration 1") {
		t.Error("transcript should contain phase and iteration")
	}
	if !strings.Contains(contentStr, "Implement the feature") {
		t.Error("transcript should contain prompt")
	}
	if !strings.Contains(contentStr, "Here is the implementation") {
		t.Error("transcript should contain response")
	}
	if !strings.Contains(contentStr, "Tokens: 100 input, 250 output") {
		t.Error("transcript should contain token counts")
	}
	if !strings.Contains(contentStr, "Complete: false") {
		t.Error("transcript should contain completion status")
	}
}

func TestSaveTranscript_MultipleIterations(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)

	// Save multiple iterations
	for i := 1; i <= 3; i++ {
		ps := PhaseState{
			TaskID:    "TASK-MULTI-001",
			Phase:     "implement",
			Iteration: i,
			Prompt:    "Continue...",
			Response:  "More work...",
		}
		if err := e.saveTranscript(ps); err != nil {
			t.Fatalf("saveTranscript iteration %d failed: %v", i, err)
		}
	}

	// Verify all files exist
	transcriptsDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-MULTI-001", "transcripts")
	files, err := os.ReadDir(transcriptsDir)
	if err != nil {
		t.Fatalf("failed to read transcripts dir: %v", err)
	}

	if len(files) != 3 {
		t.Errorf("expected 3 transcript files, got %d", len(files))
	}

	// Check file names
	expectedNames := []string{"implement-001.md", "implement-002.md", "implement-003.md"}
	for i, expected := range expectedNames {
		if files[i].Name() != expected {
			t.Errorf("file %d name = %s, want %s", i, files[i].Name(), expected)
		}
	}
}

func TestSaveTranscript_CreatesDirIfMissing(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)

	ps := PhaseState{
		TaskID:    "TASK-NEWDIR-001",
		Phase:     "spec",
		Iteration: 1,
		Prompt:    "Write spec",
		Response:  "Spec content",
	}

	// Directory doesn't exist yet
	err := e.saveTranscript(ps)
	if err != nil {
		t.Fatalf("saveTranscript failed to create directory: %v", err)
	}

	// Verify directory was created
	transcriptsDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-NEWDIR-001", "transcripts")
	if _, err := os.Stat(transcriptsDir); os.IsNotExist(err) {
		t.Error("transcripts directory should have been created")
	}
}

func TestRenderTemplate_AllVariables(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	ps := PhaseState{
		TaskID:          "TASK-RENDER-001",
		TaskTitle:       "Add button",
		TaskDescription: "Add a submit button to the form",
		Phase:           "implement",
		Weight:          "small",
		Iteration:       2,
		ResearchContent: "Prior research findings",
		SpecContent:     "Spec document",
		DesignContent:   "Design mockup",
		RetryContext:    "Previous attempt failed",
	}

	tmpl := `Task: {{TASK_ID}} - {{TASK_TITLE}}
Description: {{TASK_DESCRIPTION}}
Phase: {{PHASE}}
Weight: {{WEIGHT}}
Iteration: {{ITERATION}}
Research: {{RESEARCH_CONTENT}}
Spec: {{SPEC_CONTENT}}
Design: {{DESIGN_CONTENT}}
Retry: {{RETRY_CONTEXT}}`

	result := e.renderTemplate(tmpl, ps)

	expected := `Task: TASK-RENDER-001 - Add button
Description: Add a submit button to the form
Phase: implement
Weight: small
Iteration: 2
Research: Prior research findings
Spec: Spec document
Design: Design mockup
Retry: Previous attempt failed`

	if result != expected {
		t.Errorf("renderTemplate:\ngot:\n%s\n\nwant:\n%s", result, expected)
	}
}

func TestRenderTemplate_EmptyValues(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	ps := PhaseState{
		TaskID:    "TASK-EMPTY-001",
		TaskTitle: "Test",
		Phase:     "implement",
		Weight:    "trivial",
		// All other fields empty
	}

	tmpl := "{{RESEARCH_CONTENT}}{{SPEC_CONTENT}}{{DESIGN_CONTENT}}{{RETRY_CONTEXT}}"
	result := e.renderTemplate(tmpl, ps)

	// Empty strings should result in empty output
	if result != "" {
		t.Errorf("expected empty result for empty values, got: %s", result)
	}
}

func TestRenderTemplate_NewVariables(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	ps := PhaseState{
		TaskID:            "TASK-NEW-001",
		TaskTitle:         "New feature",
		TaskDescription:   "A new feature description",
		TaskCategory:      "feature",
		Phase:             "spec",
		Weight:            "medium",
		Iteration:         1,
		WorktreePath:      "/tmp/worktree",
		TaskBranch:        "orc/TASK-NEW-001",
		TargetBranch:      "main",
		InitiativeContext: "## Initiative Context\n\nThis is part of INIT-001.",
		RequiresUITesting: "true",
		ScreenshotDir:     "/path/to/screenshots",
		TestResults:       "All tests passed",
		CoverageThreshold: 90,
		ReviewFindings:    "Code review findings here",
	}

	// Test all new template variables
	tests := []struct {
		name     string
		tmpl     string
		expected string
	}{
		{
			name:     "task category",
			tmpl:     "Category: {{TASK_CATEGORY}}",
			expected: "Category: feature",
		},
		{
			name:     "initiative context",
			tmpl:     "{{INITIATIVE_CONTEXT}}",
			expected: "## Initiative Context\n\nThis is part of INIT-001.",
		},
		{
			name:     "requires UI testing",
			tmpl:     "UI Testing: {{REQUIRES_UI_TESTING}}",
			expected: "UI Testing: true",
		},
		{
			name:     "screenshot dir",
			tmpl:     "Screenshots: {{SCREENSHOT_DIR}}",
			expected: "Screenshots: /path/to/screenshots",
		},
		{
			name:     "test results",
			tmpl:     "Tests: {{TEST_RESULTS}}",
			expected: "Tests: All tests passed",
		},
		{
			name:     "coverage threshold",
			tmpl:     "Coverage: {{COVERAGE_THRESHOLD}}%",
			expected: "Coverage: 90%",
		},
		{
			name:     "review findings",
			tmpl:     "Review: {{REVIEW_FINDINGS}}",
			expected: "Review: Code review findings here",
		},
		{
			name:     "worktree path",
			tmpl:     "Worktree: {{WORKTREE_PATH}}",
			expected: "Worktree: /tmp/worktree",
		},
		{
			name:     "task branch",
			tmpl:     "Branch: {{TASK_BRANCH}}",
			expected: "Branch: orc/TASK-NEW-001",
		},
		{
			name:     "target branch",
			tmpl:     "Target: {{TARGET_BRANCH}}",
			expected: "Target: main",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := e.renderTemplate(tc.tmpl, ps)
			if result != tc.expected {
				t.Errorf("renderTemplate(%q):\ngot:  %q\nwant: %q", tc.tmpl, result, tc.expected)
			}
		})
	}
}

func TestRenderTemplate_CoverageThresholdDefault(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	ps := PhaseState{
		TaskID:            "TASK-COV-001",
		TaskTitle:         "Test",
		Phase:             "test",
		Weight:            "small",
		CoverageThreshold: 0, // Not set, should default to 85
	}

	tmpl := "Threshold: {{COVERAGE_THRESHOLD}}%"
	result := e.renderTemplate(tmpl, ps)

	expected := "Threshold: 85%"
	if result != expected {
		t.Errorf("renderTemplate should default coverage threshold to 85:\ngot:  %q\nwant: %q", result, expected)
	}
}

func TestRenderTemplate_ImplementationSummaryAlias(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	ps := PhaseState{
		TaskID:           "TASK-ALIAS-001",
		TaskTitle:        "Test",
		Phase:            "finalize",
		Weight:           "small",
		ImplementContent: "Implementation summary content here",
	}

	// Both {{IMPLEMENT_CONTENT}} and {{IMPLEMENTATION_SUMMARY}} should work
	result1 := e.renderTemplate("{{IMPLEMENT_CONTENT}}", ps)
	result2 := e.renderTemplate("{{IMPLEMENTATION_SUMMARY}}", ps)

	expected := "Implementation summary content here"
	if result1 != expected {
		t.Errorf("IMPLEMENT_CONTENT: got %q, want %q", result1, expected)
	}
	if result2 != expected {
		t.Errorf("IMPLEMENTATION_SUMMARY: got %q, want %q", result2, expected)
	}
}

func TestCommitCheckpointNode_NoGitOps(t *testing.T) {
	// Create executor with nil gitOps
	cfg := DefaultConfig()
	e := New(cfg)
	e.gitOps = nil // Explicitly nil

	nodeFunc := e.commitCheckpointNode()

	ps := PhaseState{
		TaskID:    "TASK-NOGIT-001",
		TaskTitle: "Test Task",
		Phase:     "implement",
		Complete:  true,
	}

	result, err := nodeFunc(nil, ps)
	if err != nil {
		t.Fatalf("commitCheckpointNode should not fail with nil gitOps: %v", err)
	}

	// State should pass through unchanged
	if result.CommitSHA != "" {
		t.Errorf("CommitSHA should be empty without git, got: %s", result.CommitSHA)
	}
	if !result.Complete {
		t.Error("Complete flag should be preserved")
	}
}

func TestExecuteClaudeNode_NoClient(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	nodeFunc := e.executeClaudeNode()

	ps := PhaseState{
		TaskID: "TASK-NOCLIENT-001",
		Phase:  "implement",
		Prompt: "Do something",
	}

	// Create context without LLM client
	ctx := context.Background()
	fgCtx := &mockFlowgraphContext{ctx: ctx}

	_, err := nodeFunc(fgCtx, ps)
	if err == nil {
		t.Error("executeClaudeNode should fail without LLM client")
	}
	if !strings.Contains(err.Error(), "no LLM client available") {
		t.Errorf("error should mention 'no LLM client available', got: %s", err.Error())
	}
}

func TestExecuteClaudeNode_Success(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	// Create mock client
	mockClient := claude.NewMockClient("Implementation complete!")

	nodeFunc := e.executeClaudeNode()

	ps := PhaseState{
		TaskID: "TASK-SUCCESS-001",
		Phase:  "implement",
		Prompt: "Implement the feature",
	}

	// Create context with LLM client
	ctx := WithLLM(context.Background(), mockClient)
	fgCtx := &mockFlowgraphContext{ctx: ctx}

	result, err := nodeFunc(fgCtx, ps)
	if err != nil {
		t.Fatalf("executeClaudeNode failed: %v", err)
	}

	if result.Response != "Implementation complete!" {
		t.Errorf("response = %q, want 'Implementation complete!'", result.Response)
	}
}

func TestExecutePhase_Flowgraph_Complete(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)
	e.SetUseSessionExecution(false) // Force flowgraph path

	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Done!")
	e.SetClient(mockClient)

	testTask := &task.Task{
		ID:     "TEST-FG-001",
		Title:  "Flowgraph Test",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Name:   "Implementation",
		Prompt: "Implement: {{TASK_TITLE}}",
	}

	testState := state.New("TEST-FG-001")

	ctx := context.Background()
	result, err := e.ExecutePhase(ctx, testTask, testPhase, testState)

	if err != nil {
		t.Fatalf("ExecutePhase failed: %v", err)
	}

	if result.Status != plan.PhaseCompleted {
		t.Errorf("expected status Completed, got %v", result.Status)
	}

	if result.Iterations < 1 {
		t.Errorf("expected at least 1 iteration, got %d", result.Iterations)
	}
}

func TestExecutePhase_Flowgraph_MaxIterations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIterations = 2 // Low for testing
	cfg.WorkDir = t.TempDir()
	e := New(cfg)
	e.SetUseSessionExecution(false)

	// Mock that never completes
	mockClient := claude.NewMockClient("Still working...")
	e.SetClient(mockClient)

	testTask := &task.Task{
		ID:     "TEST-MAX-001",
		Title:  "Max Iter Test",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Prompt: "Do work",
	}

	testState := state.New("TEST-MAX-001")

	ctx := context.Background()
	result, _ := e.ExecutePhase(ctx, testTask, testPhase, testState)

	// Should stop at max iterations
	if result.Iterations > 2 {
		t.Errorf("expected max 2 iterations, got %d", result.Iterations)
	}

	if result.Status != plan.PhaseFailed {
		t.Errorf("expected status Failed when max iterations reached, got %v", result.Status)
	}
}

func TestExecutePhase_Flowgraph_Blocked(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	e := New(cfg)
	e.SetUseSessionExecution(false)

	mockClient := claude.NewMockClient("<phase_blocked>Need clarification</phase_blocked>")
	e.SetClient(mockClient)

	testTask := &task.Task{
		ID:     "TEST-BLOCK-001",
		Title:  "Blocked Test",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "spec",
		Prompt: "Write spec",
	}

	testState := state.New("TEST-BLOCK-001")

	ctx := context.Background()
	result, _ := e.ExecutePhase(ctx, testTask, testPhase, testState)

	if result.Status != plan.PhaseFailed {
		t.Errorf("expected status Failed when blocked, got %v", result.Status)
	}

	if result.Error == nil || !strings.Contains(result.Error.Error(), "blocked") {
		t.Errorf("expected blocked error, got: %v", result.Error)
	}
}

func TestExecutePhase_UsesSessionWhenEnabled(t *testing.T) {
	// Session execution requires a real Claude CLI with valid sessions.
	// This test verifies the code path is taken but can't run without CLI.
	t.Skip("Session execution requires real Claude CLI - tested via integration tests")
}

// mockFlowgraphContext implements flowgraph.Context for testing
type mockFlowgraphContext struct {
	ctx context.Context
}

func (m *mockFlowgraphContext) Context() context.Context {
	return m.ctx
}

func (m *mockFlowgraphContext) Deadline() (time.Time, bool) {
	return m.ctx.Deadline()
}

func (m *mockFlowgraphContext) Done() <-chan struct{} {
	return m.ctx.Done()
}

func (m *mockFlowgraphContext) Err() error {
	return m.ctx.Err()
}

func (m *mockFlowgraphContext) Value(key any) any {
	return m.ctx.Value(key)
}

func (m *mockFlowgraphContext) Logger() *slog.Logger {
	return slog.Default()
}

func (m *mockFlowgraphContext) Checkpointer() checkpoint.Store {
	return nil
}

func (m *mockFlowgraphContext) RunID() string {
	return "test-run"
}

func (m *mockFlowgraphContext) NodeID() string {
	return "test-node"
}

func (m *mockFlowgraphContext) Attempt() int {
	return 1
}
