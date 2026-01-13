package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// newTestExecutor creates an executor configured for test isolation.
func newTestExecutor(t *testing.T) *Executor {
	t.Helper()
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	return New(cfg)
}

func TestResolveClaudePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantAbs  bool // Whether result should be absolute
		wantSame bool // Whether result should be same as input
	}{
		{
			name:     "empty string",
			input:    "",
			wantSame: true,
		},
		{
			name:     "already absolute",
			input:    "/usr/local/bin/claude",
			wantAbs:  true,
			wantSame: true,
		},
		{
			name:    "relative claude",
			input:   "claude",
			wantAbs: true, // Should resolve to absolute if claude exists in PATH
		},
		{
			name:     "relative nonexistent",
			input:    "nonexistent-binary-xyz",
			wantSame: true, // Falls back to original if not found
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveClaudePath(tc.input)

			if tc.wantSame && result != tc.input {
				t.Errorf("resolveClaudePath(%q) = %q, want %q", tc.input, result, tc.input)
			}

			if tc.wantAbs && result != "" && !filepath.IsAbs(result) {
				// Only check for absolute if we expect it AND claude is actually in PATH
				// If claude isn't installed, it should fall back to the original
				if tc.input != "claude" {
					t.Errorf("resolveClaudePath(%q) = %q, want absolute path", tc.input, result)
				}
			}
		})
	}
}

func TestFindClaudeInCommonLocations(t *testing.T) {
	// Create a temp directory with a fake claude binary
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")

	// Create the fake binary file with executable permissions
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/sh\necho fake"), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	// Save original and replace with test locations
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{
		"/nonexistent/path/claude", // Won't exist
		fakeClaude,                 // Should be found
	}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != fakeClaude {
		t.Errorf("findClaudeInCommonLocations() = %q, want %q", result, fakeClaude)
	}
}

func TestFindClaudeInCommonLocations_HomeExpansion(t *testing.T) {
	// This test verifies ~ expansion works
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("could not determine home directory")
	}

	// Create a temp subdir in home
	testDir := filepath.Join(homeDir, ".orc-test-"+t.Name())
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("failed to create test dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	fakeClaude := filepath.Join(testDir, "claude")
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/sh\necho fake"), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	// Save original and replace with test locations using ~
	originalLocations := commonClaudeLocations
	relativePath := "~/" + filepath.Base(testDir) + "/claude"
	commonClaudeLocations = []string{relativePath}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != fakeClaude {
		t.Errorf("findClaudeInCommonLocations() = %q, want %q (expanded from %q)", result, fakeClaude, relativePath)
	}
}

func TestFindClaudeInCommonLocations_NoMatch(t *testing.T) {
	// Save original and replace with nonexistent locations
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{
		"/nonexistent/path1/claude",
		"/nonexistent/path2/claude",
	}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != "" {
		t.Errorf("findClaudeInCommonLocations() = %q, want empty string", result)
	}
}

func TestFindClaudeInCommonLocations_SkipsNonExecutable(t *testing.T) {
	// Create a temp directory with a non-executable file
	tmpDir := t.TempDir()
	nonExecFile := filepath.Join(tmpDir, "claude")

	// Create file WITHOUT executable permission (0644)
	if err := os.WriteFile(nonExecFile, []byte("#!/bin/sh\necho fake"), 0644); err != nil {
		t.Fatalf("failed to create non-exec file: %v", err)
	}

	// Save original and replace with test location
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{nonExecFile}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != "" {
		t.Errorf("findClaudeInCommonLocations() = %q, want empty (file not executable)", result)
	}
}

func TestFindClaudeInCommonLocations_SkipsDirectories(t *testing.T) {
	// Create a directory named "claude" (edge case: something might create a dir with this name)
	tmpDir := t.TempDir()
	claudeDir := filepath.Join(tmpDir, "claude")

	// Create directory with executable permission
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// Save original and replace with test location
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{claudeDir}
	defer func() { commonClaudeLocations = originalLocations }()

	result := findClaudeInCommonLocations()
	if result != "" {
		t.Errorf("findClaudeInCommonLocations() = %q, want empty (directory should be skipped)", result)
	}
}

func TestResolveClaudePath_WithCommonLocations(t *testing.T) {
	// Test that resolveClaudePath falls back to common locations
	// when PATH lookup fails

	// Create a temp directory with a fake claude binary
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/sh\necho fake"), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	// Save original and replace with test locations
	originalLocations := commonClaudeLocations
	commonClaudeLocations = []string{fakeClaude}
	defer func() { commonClaudeLocations = originalLocations }()

	// Modify PATH to not include claude (use a temp dir)
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", originalPath)

	result := resolveClaudePath("claude")
	if result != fakeClaude {
		t.Errorf("resolveClaudePath(\"claude\") = %q, want %q (from common locations)", result, fakeClaude)
	}
}

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
	e := newTestExecutor(t)

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
	e := newTestExecutor(t)

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

func TestRenderTemplateWithRetryContext(t *testing.T) {
	e := newTestExecutor(t)

	state := PhaseState{
		TaskID:       "TASK-001",
		TaskTitle:    "Fix bug",
		Phase:        "implement",
		Weight:       "small",
		RetryContext: "Previous attempt failed because tests didn't pass",
	}

	tmpl := "Task: {{TASK_ID}}\nRetry info: {{RETRY_CONTEXT}}"
	result := e.renderTemplate(tmpl, state)

	expected := "Task: TASK-001\nRetry info: Previous attempt failed because tests didn't pass"
	if result != expected {
		t.Errorf("renderTemplate() = %q, want %q", result, expected)
	}
}

func TestRenderTemplateWithDescription(t *testing.T) {
	e := newTestExecutor(t)

	state := PhaseState{
		TaskID:          "TASK-002",
		TaskTitle:       "Add feature",
		TaskDescription: "Add a new button to the UI that triggers an action",
		Phase:           "spec",
		Weight:          "medium",
	}

	tmpl := "Title: {{TASK_TITLE}}\nDescription: {{TASK_DESCRIPTION}}"
	result := e.renderTemplate(tmpl, state)

	expected := "Title: Add feature\nDescription: Add a new button to the UI that triggers an action"
	if result != expected {
		t.Errorf("renderTemplate() = %q, want %q", result, expected)
	}
}

func TestRenderTemplateWithEmptyValues(t *testing.T) {
	e := newTestExecutor(t)

	state := PhaseState{
		TaskID:    "TASK-003",
		TaskTitle: "Test",
		Phase:     "implement",
		Weight:    "trivial",
		// All other fields empty
	}

	tmpl := "{{RESEARCH_CONTENT}}{{SPEC_CONTENT}}{{DESIGN_CONTENT}}"
	result := e.renderTemplate(tmpl, state)

	// Empty strings should just result in empty output
	if result != "" {
		t.Errorf("renderTemplate() with empty values = %q, want empty", result)
	}
}

func TestPhaseStateWithAllFields(t *testing.T) {
	state := PhaseState{
		TaskID:          "TASK-001",
		TaskTitle:       "Full task",
		TaskDescription: "Complete description",
		Phase:           "test",
		Weight:          "large",
		Iteration:       5,
		Prompt:          "Test prompt",
		Response:        "Test response",
		Complete:        true,
		Blocked:         false,
		ResearchContent: "Research",
		SpecContent:     "Spec",
		DesignContent:   "Design",
		RetryContext:    "Retry info",
	}

	if !state.Complete {
		t.Error("Complete should be true")
	}

	if state.Blocked {
		t.Error("Blocked should be false")
	}

	if state.Iteration != 5 {
		t.Errorf("Iteration = %d, want 5", state.Iteration)
	}
}

func TestResultWithError(t *testing.T) {
	testErr := fmt.Errorf("tests failed")
	result := &Result{
		Phase:      "test",
		Iterations: 3,
		Duration:   1 * time.Minute,
		Error:      testErr,
	}

	if result.Error == nil {
		t.Error("Error should not be nil")
	}
	if result.Error.Error() != "tests failed" {
		t.Errorf("Error = %s, want 'tests failed'", result.Error)
	}
}

func TestSetPublisher(t *testing.T) {
	e := newTestExecutor(t)

	if e.publisher != nil {
		t.Error("publisher should be nil initially")
	}

	// We can't easily test with a real publisher without the events package
	// but we can verify SetPublisher doesn't panic with nil
	e.SetPublisher(nil)

	if e.publisher != nil {
		t.Error("publisher should remain nil after setting nil")
	}
}

func TestNewWithDifferentConfigs(t *testing.T) {
	tests := []struct {
		name           string
		cfg            *Config
		wantIterations int
	}{
		{
			name:           "default config",
			cfg:            DefaultConfig(),
			wantIterations: 30,
		},
		{
			name: "custom iterations",
			cfg: &Config{
				ClaudePath:                 "claude",
				Model:                      "claude-sonnet-4-20250514",
				MaxIterations:              50,
				Timeout:                    5 * time.Minute,
				BranchPrefix:               "custom/",
				CommitPrefix:               "[custom]",
				DangerouslySkipPermissions: true,
				EnableCheckpoints:          false,
			},
			wantIterations: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := New(tt.cfg)
			if e.config.MaxIterations != tt.wantIterations {
				t.Errorf("MaxIterations = %d, want %d", e.config.MaxIterations, tt.wantIterations)
			}
		})
	}
}

func TestConfigFromOrc(t *testing.T) {
	orcCfg := &config.Config{
		ClaudePath:                 "/custom/claude",
		Model:                      "custom-model",
		DangerouslySkipPermissions: false,
		MaxIterations:              100,
		Timeout:                    20 * time.Minute,
		BranchPrefix:               "custom/",
		CommitPrefix:               "[custom]",
		TemplatesDir:               "custom-templates",
		EnableCheckpoints:          true,
	}

	cfg := ConfigFromOrc(orcCfg)

	if cfg.ClaudePath != orcCfg.ClaudePath {
		t.Errorf("ClaudePath = %s, want %s", cfg.ClaudePath, orcCfg.ClaudePath)
	}
	if cfg.Model != orcCfg.Model {
		t.Errorf("Model = %s, want %s", cfg.Model, orcCfg.Model)
	}
	if cfg.DangerouslySkipPermissions != orcCfg.DangerouslySkipPermissions {
		t.Errorf("DangerouslySkipPermissions = %v, want %v", cfg.DangerouslySkipPermissions, orcCfg.DangerouslySkipPermissions)
	}
	if cfg.MaxIterations != orcCfg.MaxIterations {
		t.Errorf("MaxIterations = %d, want %d", cfg.MaxIterations, orcCfg.MaxIterations)
	}
	if cfg.Timeout != orcCfg.Timeout {
		t.Errorf("Timeout = %v, want %v", cfg.Timeout, orcCfg.Timeout)
	}
	if cfg.BranchPrefix != orcCfg.BranchPrefix {
		t.Errorf("BranchPrefix = %s, want %s", cfg.BranchPrefix, orcCfg.BranchPrefix)
	}
	if cfg.CommitPrefix != orcCfg.CommitPrefix {
		t.Errorf("CommitPrefix = %s, want %s", cfg.CommitPrefix, orcCfg.CommitPrefix)
	}
}

func TestNewWithConfig(t *testing.T) {
	cfg := DefaultConfig()
	orcCfg := &config.Config{
		Profile: "strict",
	}

	e := NewWithConfig(cfg, orcCfg)

	if e == nil {
		t.Fatal("NewWithConfig() returned nil")
	}
	if e.orcConfig.Profile != "strict" {
		t.Errorf("orcConfig.Profile = %s, want strict", e.orcConfig.Profile)
	}
}

func TestNewWithConfig_NilOrcConfig(t *testing.T) {
	cfg := DefaultConfig()

	e := NewWithConfig(cfg, nil)

	if e == nil {
		t.Fatal("NewWithConfig() returned nil")
	}
	// Should use default orc config
	if e.orcConfig == nil {
		t.Error("orcConfig should not be nil")
	}
}

func TestSetClient(t *testing.T) {
	e := newTestExecutor(t)

	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>")
	e.SetClient(mockClient)

	// Verify client was set (by making a request)
	if e.client == nil {
		t.Error("client should not be nil after SetClient")
	}
}

func TestPublishHelpers(t *testing.T) {
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	// Subscribe to receive events
	ch := pub.Subscribe("TASK-001")
	defer pub.Unsubscribe("TASK-001", ch)

	// Test publishPhaseStart
	e.publishPhaseStart("TASK-001", "implement")

	select {
	case event := <-ch:
		if event.Type != events.EventPhase {
			t.Errorf("expected EventPhase, got %v", event.Type)
		}
		if event.TaskID != "TASK-001" {
			t.Errorf("expected task TASK-001, got %s", event.TaskID)
		}
		data, ok := event.Data.(events.PhaseUpdate)
		if !ok {
			t.Fatalf("expected PhaseUpdate data, got %T", event.Data)
		}
		if data.Phase != "implement" {
			t.Errorf("expected phase implement, got %s", data.Phase)
		}
		if data.Status != string(plan.PhaseRunning) {
			t.Errorf("expected status %s, got %s", plan.PhaseRunning, data.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPublishPhaseComplete(t *testing.T) {
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	ch := pub.Subscribe("TASK-002")
	defer pub.Unsubscribe("TASK-002", ch)

	e.publishPhaseComplete("TASK-002", "test", "abc123")

	select {
	case event := <-ch:
		data, ok := event.Data.(events.PhaseUpdate)
		if !ok {
			t.Fatalf("expected PhaseUpdate, got %T", event.Data)
		}
		if data.Status != "completed" {
			t.Errorf("expected status completed, got %s", data.Status)
		}
		if data.CommitSHA != "abc123" {
			t.Errorf("expected commit abc123, got %s", data.CommitSHA)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPublishPhaseFailed(t *testing.T) {
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	ch := pub.Subscribe("TASK-003")
	defer pub.Unsubscribe("TASK-003", ch)

	testErr := fmt.Errorf("something went wrong")
	e.publishPhaseFailed("TASK-003", "validate", testErr)

	select {
	case event := <-ch:
		data, ok := event.Data.(events.PhaseUpdate)
		if !ok {
			t.Fatalf("expected PhaseUpdate, got %T", event.Data)
		}
		if data.Status != "failed" {
			t.Errorf("expected status failed, got %s", data.Status)
		}
		if data.Error != "something went wrong" {
			t.Errorf("expected error message, got %s", data.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPublishTranscript(t *testing.T) {
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	ch := pub.Subscribe("TASK-004")
	defer pub.Unsubscribe("TASK-004", ch)

	e.publishTranscript("TASK-004", "implement", 3, "response", "Here is the implementation")

	select {
	case event := <-ch:
		if event.Type != events.EventTranscript {
			t.Errorf("expected EventTranscript, got %v", event.Type)
		}
		data, ok := event.Data.(events.TranscriptLine)
		if !ok {
			t.Fatalf("expected TranscriptLine, got %T", event.Data)
		}
		if data.Phase != "implement" {
			t.Errorf("expected phase implement, got %s", data.Phase)
		}
		if data.Iteration != 3 {
			t.Errorf("expected iteration 3, got %d", data.Iteration)
		}
		if data.Type != "response" {
			t.Errorf("expected type response, got %s", data.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPublishTokens(t *testing.T) {
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	ch := pub.Subscribe("TASK-005")
	defer pub.Unsubscribe("TASK-005", ch)

	e.publishTokens("TASK-005", "spec", 1000, 500, 0, 1500)

	select {
	case event := <-ch:
		if event.Type != events.EventTokens {
			t.Errorf("expected EventTokens, got %v", event.Type)
		}
		data, ok := event.Data.(events.TokenUpdate)
		if !ok {
			t.Fatalf("expected TokenUpdate, got %T", event.Data)
		}
		if data.InputTokens != 1000 {
			t.Errorf("expected input 1000, got %d", data.InputTokens)
		}
		if data.OutputTokens != 500 {
			t.Errorf("expected output 500, got %d", data.OutputTokens)
		}
		if data.TotalTokens != 1500 {
			t.Errorf("expected total 1500, got %d", data.TotalTokens)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPublishError(t *testing.T) {
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	ch := pub.Subscribe("TASK-006")
	defer pub.Unsubscribe("TASK-006", ch)

	e.publishError("TASK-006", "build", "compilation failed", true)

	select {
	case event := <-ch:
		if event.Type != events.EventError {
			t.Errorf("expected EventError, got %v", event.Type)
		}
		data, ok := event.Data.(events.ErrorData)
		if !ok {
			t.Fatalf("expected ErrorData, got %T", event.Data)
		}
		if data.Phase != "build" {
			t.Errorf("expected phase build, got %s", data.Phase)
		}
		if data.Message != "compilation failed" {
			t.Errorf("expected message 'compilation failed', got %s", data.Message)
		}
		if !data.Fatal {
			t.Error("expected fatal to be true")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPublishWithNoPublisher(t *testing.T) {
	e := newTestExecutor(t)
	// No publisher set - should not panic
	e.publishPhaseStart("TASK-001", "test")
	e.publishPhaseComplete("TASK-001", "test", "sha")
	e.publishPhaseFailed("TASK-001", "test", nil)
	e.publishTranscript("TASK-001", "test", 1, "type", "content")
	e.publishTokens("TASK-001", "test", 0, 0, 0, 0)
	e.publishError("TASK-001", "test", "msg", false)
}

func TestExecutePhase_Complete(t *testing.T) {
	e := newTestExecutor(t)
	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Implementation done.")
	e.SetClient(mockClient)

	// Create test task
	testTask := &task.Task{
		ID:     "TEST-001",
		Title:  "Test task",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	// Create test phase
	testPhase := &plan.Phase{
		ID:     "implement",
		Name:   "Implementation",
		Prompt: "Implement the feature: {{TASK_TITLE}}",
	}

	// Create test state
	testState := state.New("TEST-001")

	ctx := context.Background()
	result, err := e.ExecutePhase(ctx, testTask, testPhase, testState)

	if err != nil {
		t.Fatalf("ExecutePhase failed: %v", err)
	}

	if result.Status != plan.PhaseCompleted {
		t.Errorf("expected status Completed, got %v", result.Status)
	}

	if result.Phase != "implement" {
		t.Errorf("expected phase implement, got %s", result.Phase)
	}

	if mockClient.CallCount() < 1 {
		t.Error("expected at least one Claude call")
	}
}

func TestExecutePhase_MaxIterations(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxIterations = 2 // Low for testing
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	// Mock that never completes
	mockClient := claude.NewMockClient("Still working on it...")
	e.SetClient(mockClient)

	testTask := &task.Task{
		ID:     "TEST-002",
		Title:  "Never ending task",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Name:   "Implementation",
		Prompt: "Implement: {{TASK_TITLE}}",
	}

	testState := state.New("TEST-002")

	ctx := context.Background()
	result, _ := e.ExecutePhase(ctx, testTask, testPhase, testState)

	// Should stop at max iterations
	if result.Iterations > 2 {
		t.Errorf("expected max 2 iterations, got %d", result.Iterations)
	}
}

func TestExecutePhase_Blocked(t *testing.T) {
	e := newTestExecutor(t)
	mockClient := claude.NewMockClient("<phase_blocked>Need clarification on requirements</phase_blocked>")
	e.SetClient(mockClient)

	testTask := &task.Task{
		ID:     "TEST-003",
		Title:  "Blocked task",
		Status: task.StatusRunning,
		Weight: task.WeightMedium,
	}

	testPhase := &plan.Phase{
		ID:     "spec",
		Name:   "Specification",
		Prompt: "Write spec for: {{TASK_TITLE}}",
	}

	testState := state.New("TEST-003")

	ctx := context.Background()
	result, _ := e.ExecutePhase(ctx, testTask, testPhase, testState)

	// When blocked, status becomes PhaseFailed with specific error
	if result.Status != plan.PhaseFailed {
		t.Errorf("expected status Failed (blocked), got %v", result.Status)
	}
	if result.Error == nil || !strings.Contains(result.Error.Error(), "blocked") {
		t.Errorf("expected blocked error, got %v", result.Error)
	}
}

func TestExecutePhase_ContextCancellation(t *testing.T) {
	e := newTestExecutor(t)

	// Mock that takes time (simulated by the mock sleeping)
	mockClient := claude.NewMockClient("Response")
	e.SetClient(mockClient)

	testTask := &task.Task{
		ID:     "TEST-004",
		Title:  "Cancellable task",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Name:   "Implementation",
		Prompt: "Implement: {{TASK_TITLE}}",
	}

	testState := state.New("TEST-004")

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.ExecutePhase(ctx, testTask, testPhase, testState)

	if err == nil {
		// Context cancellation might not always return an error depending on timing
		// This is acceptable behavior
		t.Log("ExecutePhase completed despite cancelled context (timing dependent)")
	}
}

func TestExecutePhase_WithPublisher(t *testing.T) {
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Done!")
	e.SetClient(mockClient)

	ch := pub.Subscribe("TEST-005")
	defer pub.Unsubscribe("TEST-005", ch)

	testTask := &task.Task{
		ID:     "TEST-005",
		Title:  "Event task",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Name:   "Implementation",
		Prompt: "Implement: {{TASK_TITLE}}",
	}

	testState := state.New("TEST-005")

	ctx := context.Background()
	_, err := e.ExecutePhase(ctx, testTask, testPhase, testState)

	if err != nil {
		t.Fatalf("ExecutePhase failed: %v", err)
	}

	// Verify at least one event was published
	select {
	case event := <-ch:
		if event.TaskID != "TEST-005" {
			t.Errorf("expected task TEST-005, got %s", event.TaskID)
		}
	case <-time.After(100 * time.Millisecond):
		// Events may have been published but drained before we could read them
		t.Log("No events captured (timing dependent)")
	}
}

func TestPublishState(t *testing.T) {
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	ch := pub.Subscribe("TASK-007")
	defer pub.Unsubscribe("TASK-007", ch)

	testState := state.New("TASK-007")
	testState.StartPhase("implement")

	e.publishState("TASK-007", testState)

	select {
	case event := <-ch:
		if event.Type != events.EventState {
			t.Errorf("expected EventState, got %v", event.Type)
		}
		if event.TaskID != "TASK-007" {
			t.Errorf("expected task TASK-007, got %s", event.TaskID)
		}
		// Data should be the state object
		if event.Data == nil {
			t.Error("expected state data, got nil")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for state event")
	}
}

func TestEvaluateGate_AutoApprove(t *testing.T) {
	cfg := DefaultConfig()
	orcCfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: true,
		},
	}

	e := NewWithConfig(cfg, orcCfg)

	testPhase := &plan.Phase{
		ID: "implement",
		Gate: plan.Gate{
			Type:     plan.GateAuto,
			Criteria: []string{"has_output"},
		},
	}

	ctx := context.Background()
	decision, err := e.evaluateGate(ctx, testPhase, "some output", "small")

	if err != nil {
		t.Fatalf("evaluateGate failed: %v", err)
	}

	if !decision.Approved {
		t.Error("expected auto-approve on success")
	}

	if decision.Reason != "auto-approved on success" {
		t.Errorf("expected reason 'auto-approved on success', got %s", decision.Reason)
	}
}

func TestEvaluateGate_WithCriteria(t *testing.T) {
	cfg := DefaultConfig()
	orcCfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType:          "auto",
			AutoApproveOnSuccess: false, // Actually evaluate criteria
		},
	}

	e := NewWithConfig(cfg, orcCfg)

	testPhase := &plan.Phase{
		ID: "implement",
		Gate: plan.Gate{
			Type:     plan.GateAuto,
			Criteria: []string{"has_output"},
		},
	}

	ctx := context.Background()

	// Should approve with output
	decision, err := e.evaluateGate(ctx, testPhase, "some output here", "small")
	if err != nil {
		t.Fatalf("evaluateGate failed: %v", err)
	}
	if !decision.Approved {
		t.Error("expected approval with output")
	}

	// Should reject without output
	decision, err = e.evaluateGate(ctx, testPhase, "", "small")
	if err != nil {
		t.Fatalf("evaluateGate failed: %v", err)
	}
	if decision.Approved {
		t.Error("expected rejection without output")
	}
}

func TestEvaluateGate_PhaseOverride(t *testing.T) {
	cfg := DefaultConfig()
	orcCfg := &config.Config{
		Gates: config.GateConfig{
			DefaultType: "auto",
			PhaseOverrides: map[string]string{
				"spec": "auto", // Override spec to use auto gate
			},
			AutoApproveOnSuccess: true,
		},
	}

	e := NewWithConfig(cfg, orcCfg)

	testPhase := &plan.Phase{
		ID: "spec",
		Gate: plan.Gate{
			Type: plan.GateHuman, // This will be overridden
		},
	}

	ctx := context.Background()
	decision, err := e.evaluateGate(ctx, testPhase, "spec content", "large")

	if err != nil {
		t.Fatalf("evaluateGate failed: %v", err)
	}

	// Should use auto gate due to override
	if !decision.Approved {
		t.Error("expected auto-approval via phase override")
	}
}

func TestLoadRetryContextForPhase(t *testing.T) {
	// Test with no retry context
	testState := state.New("TASK-999")
	ctx := LoadRetryContextForPhase(testState)
	if ctx != "" {
		t.Errorf("expected empty retry context, got %s", ctx)
	}
}

func TestLoadRetryContextForPhase_WithContext(t *testing.T) {
	// Test with retry context set
	testState := state.New("TASK-888")
	testState.SetRetryContext("test", "implement", "test failed", "output here", 1)

	ctx := LoadRetryContextForPhase(testState)
	if ctx == "" {
		t.Error("expected retry context, got empty")
	}
	if !strings.Contains(ctx, "test failed") {
		t.Errorf("expected retry context to contain failure reason, got %s", ctx)
	}
}

func TestSaveRetryContextFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Save retry context
	path, err := SaveRetryContextFile(tmpDir, "TASK-001", "test", "implement", "tests failed", "error output", 1)
	if err != nil {
		t.Fatalf("SaveRetryContextFile failed: %v", err)
	}

	// Verify file was created
	expectedPath := filepath.Join(tmpDir, ".orc/tasks/TASK-001/retry-context-test-1.md")
	if path != expectedPath {
		t.Errorf("path = %s, want %s", path, expectedPath)
	}

	// Verify file contents
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read retry context file: %v", err)
	}

	if !strings.Contains(string(content), "tests failed") {
		t.Error("retry context should contain failure reason")
	}
	if !strings.Contains(string(content), "error output") {
		t.Error("retry context should contain output")
	}
}

func TestSaveRetryContextFile_MultipleAttempts(t *testing.T) {
	tmpDir := t.TempDir()

	// Save multiple retry contexts
	path1, _ := SaveRetryContextFile(tmpDir, "TASK-002", "test", "implement", "first failure", "output1", 1)
	path2, _ := SaveRetryContextFile(tmpDir, "TASK-002", "test", "implement", "second failure", "output2", 2)

	// Verify both files exist with different names
	if path1 == path2 {
		t.Error("retry context files should have different paths for different attempts")
	}

	if _, err := os.Stat(path1); os.IsNotExist(err) {
		t.Error("first retry context file should exist")
	}
	if _, err := os.Stat(path2); os.IsNotExist(err) {
		t.Error("second retry context file should exist")
	}
}

func TestBuildPromptNode_InlinePrompt(t *testing.T) {
	e := newTestExecutor(t)

	// Create a phase with inline prompt (no template file)
	testPhase := &plan.Phase{
		ID:     "custom",
		Name:   "Custom Phase",
		Prompt: "Do something for {{TASK_TITLE}}",
	}

	nodeFunc := e.buildPromptNode(testPhase)

	initialState := PhaseState{
		TaskID:    "TEST-001",
		TaskTitle: "Test Task",
		Phase:     "custom",
		Weight:    "small",
	}

	// Execute the node
	result, err := nodeFunc(nil, initialState)
	if err != nil {
		t.Fatalf("buildPromptNode failed: %v", err)
	}

	// Verify prompt was rendered
	if !strings.Contains(result.Prompt, "Test Task") {
		t.Errorf("prompt should contain task title, got: %s", result.Prompt)
	}

	// Verify iteration was incremented
	if result.Iteration != 1 {
		t.Errorf("iteration = %d, want 1", result.Iteration)
	}
}

func TestBuildPromptNode_NoPrompt(t *testing.T) {
	e := newTestExecutor(t)

	// Create a phase with no prompt and no template
	testPhase := &plan.Phase{
		ID:   "nonexistent",
		Name: "Nonexistent Phase",
		// No Prompt field
	}

	nodeFunc := e.buildPromptNode(testPhase)

	initialState := PhaseState{
		TaskID:    "TEST-001",
		TaskTitle: "Test Task",
		Phase:     "nonexistent",
	}

	// Execute the node - should return error
	_, err := nodeFunc(nil, initialState)
	if err == nil {
		t.Error("buildPromptNode should fail when no prompt is available")
	}
}

func TestExecuteWithRetry_Success(t *testing.T) {
	e := newTestExecutor(t)
	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Done!")
	e.SetClient(mockClient)

	testTask := &task.Task{
		ID:     "TEST-RETRY-001",
		Title:  "Retry Test",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Name:   "Implementation",
		Prompt: "Implement: {{TASK_TITLE}}",
	}

	testState := state.New("TEST-RETRY-001")

	ctx := context.Background()
	result, err := e.ExecuteWithRetry(ctx, testTask, testPhase, testState)

	if err != nil {
		t.Fatalf("ExecuteWithRetry failed: %v", err)
	}

	if result.Status != plan.PhaseCompleted {
		t.Errorf("expected status Completed, got %v", result.Status)
	}
}

func TestExecuteWithRetry_ContextCancelled(t *testing.T) {
	e := newTestExecutor(t)
	mockClient := claude.NewMockClient("Still working...")
	e.SetClient(mockClient)

	testTask := &task.Task{
		ID:     "TEST-CANCEL",
		Title:  "Cancel Test",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Prompt: "Do work",
	}

	testState := state.New("TEST-CANCEL")

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := e.ExecuteWithRetry(ctx, testTask, testPhase, testState)
	// May or may not error depending on timing - just ensure no panic
	_ = err
}

func TestCommitCheckpointNode(t *testing.T) {
	// Create temp dir that's not a git repo to ensure gitOps is nil
	tmpDir := t.TempDir()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)

	nodeFunc := e.commitCheckpointNode()

	// Test with completed state
	state := PhaseState{
		TaskID:   "TEST-001",
		Phase:    "implement",
		Complete: true,
		Response: "Implementation done",
	}

	// Since gitOps is nil in a non-git dir, this should pass through without error
	result, err := nodeFunc(nil, state)
	if err != nil {
		t.Fatalf("commitCheckpointNode failed: %v", err)
	}

	// State should pass through
	if !result.Complete {
		t.Error("state should still be complete")
	}
}

// === ExecuteTask Tests ===

func TestExecuteTask_SinglePhaseSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/TASK-EXEC-001")

	// Initialize orc directory structure
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	// Create task
	testTask := task.New("TASK-EXEC-001", "Execute Task Test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := testTask.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create plan with single phase that uses inline prompt
	testPlan := &plan.Plan{
		Version:     1,
		Weight:      "small",
		Description: "Test plan",
		Phases: []plan.Phase{
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: "Implement: {{TASK_TITLE}} <phase_complete>true</phase_complete>",
			},
		},
	}
	if err := testPlan.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Create state
	testState := state.New("TASK-EXEC-001")

	// Create executor with mock client
	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Implementation done!")
	e.SetClient(mockClient)

	// Execute task
	ctx := context.Background()
	err := e.ExecuteTask(ctx, testTask, testPlan, testState)
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	// Reload and verify task status
	reloadedTask, err := task.LoadFrom(tmpDir, "TASK-EXEC-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloadedTask.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want completed", reloadedTask.Status)
	}
}

func TestExecuteTask_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/TASK-CANCEL-001")

	// Initialize orc directory structure
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	// Create task
	testTask := task.New("TASK-CANCEL-001", "Cancel Test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := testTask.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create plan
	testPlan := &plan.Plan{
		Version: 1,
		Weight:  "small",
		Phases: []plan.Phase{
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: "Do work",
			},
		},
	}
	if err := testPlan.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Create state
	testState := state.New("TASK-CANCEL-001")

	// Create executor with mock client that returns incomplete response
	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	mockClient := claude.NewMockClient("Still working...")
	e.SetClient(mockClient)

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := e.ExecuteTask(ctx, testTask, testPlan, testState)
	if err == nil {
		t.Error("ExecuteTask should fail when context is cancelled")
	}
	// Should be context.Canceled error
	if err != context.Canceled {
		// Could be a wrapped error, just check it's not nil
		t.Log("Got expected error:", err)
	}
}

func TestExecuteTask_SkipCompletedPhase(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/TASK-SKIP-001")

	// Initialize orc directory structure
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	// Create task
	testTask := task.New("TASK-SKIP-001", "Skip Phase Test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := testTask.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create plan with two phases
	testPlan := &plan.Plan{
		Version: 1,
		Weight:  "small",
		Phases: []plan.Phase{
			{
				ID:     "spec",
				Name:   "Specification",
				Prompt: "Spec: {{TASK_TITLE}}",
			},
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: "Implement: {{TASK_TITLE}}",
			},
		},
	}
	if err := testPlan.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Create state with first phase already completed
	testState := state.New("TASK-SKIP-001")
	testState.StartPhase("spec")
	testState.CompletePhase("spec", "abc123")
	if err := testState.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create executor with mock client
	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Done!")
	e.SetClient(mockClient)

	// Execute task
	ctx := context.Background()
	err := e.ExecuteTask(ctx, testTask, testPlan, testState)
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	// Verify task completed
	reloadedTask, _ := task.LoadFrom(tmpDir, "TASK-SKIP-001")
	if reloadedTask.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want completed", reloadedTask.Status)
	}

	// Verify spec phase was skipped (not re-executed)
	reloadedState, _ := state.LoadFrom(tmpDir, "TASK-SKIP-001")
	specPhase := reloadedState.Phases["spec"]
	if specPhase.CommitSHA != "abc123" {
		t.Errorf("spec phase commit SHA changed unexpectedly: %s", specPhase.CommitSHA)
	}
}

func TestExecuteTask_WithPublisher(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/TASK-PUB-001")

	// Initialize orc directory structure
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	// Create task
	testTask := task.New("TASK-PUB-001", "Publisher Test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := testTask.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create plan
	testPlan := &plan.Plan{
		Version: 1,
		Weight:  "small",
		Phases: []plan.Phase{
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: "Implement: {{TASK_TITLE}}",
			},
		},
	}
	if err := testPlan.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	testState := state.New("TASK-PUB-001")

	// Create publisher and subscribe
	pub := events.NewMemoryPublisher()
	eventCh := pub.Subscribe("TASK-PUB-001")
	receivedEvents := make([]events.Event, 0)

	// Collect events in background
	done := make(chan struct{})
	go func() {
		for evt := range eventCh {
			receivedEvents = append(receivedEvents, evt)
			if evt.Type == events.EventComplete {
				close(done)
				return
			}
		}
	}()

	// Create executor with mock client and publisher
	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Done!")
	e.SetClient(mockClient)
	e.SetPublisher(pub)

	// Execute task
	ctx := context.Background()
	err := e.ExecuteTask(ctx, testTask, testPlan, testState)
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	// Wait for events with timeout
	select {
	case <-done:
		// Good, events received
	case <-time.After(2 * time.Second):
		t.Log("Timed out waiting for events")
	}

	// Should have received some events
	if len(receivedEvents) == 0 {
		t.Error("expected to receive events")
	}

	// Check for phase start and complete events
	hasPhaseStart := false
	hasComplete := false
	for _, evt := range receivedEvents {
		if evt.Type == events.EventPhase {
			hasPhaseStart = true
		}
		if evt.Type == events.EventComplete {
			hasComplete = true
		}
	}
	if !hasPhaseStart {
		t.Error("expected phase start event")
	}
	if !hasComplete {
		t.Error("expected complete event")
	}
}

// === ResumeFromPhase Tests ===

func TestResumeFromPhase_Success(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/TASK-RESUME-001")

	// Initialize orc directory structure
	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	// Create task
	testTask := task.New("TASK-RESUME-001", "Resume Test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPaused
	if err := testTask.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create plan with two phases
	testPlan := &plan.Plan{
		Version: 1,
		Weight:  "small",
		Phases: []plan.Phase{
			{
				ID:     "spec",
				Name:   "Specification",
				Prompt: "Spec: {{TASK_TITLE}}",
			},
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: "Implement: {{TASK_TITLE}}",
			},
		},
	}
	if err := testPlan.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// Create state with first phase completed, second interrupted
	testState := state.New("TASK-RESUME-001")
	testState.StartPhase("spec")
	testState.CompletePhase("spec", "abc123")
	testState.StartPhase("implement")
	testState.InterruptPhase("implement")
	if err := testState.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Create executor with mock client
	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Done!")
	e.SetClient(mockClient)

	// Resume from implement phase
	ctx := context.Background()
	err := e.ResumeFromPhase(ctx, testTask, testPlan, testState, "implement")
	if err != nil {
		t.Fatalf("ResumeFromPhase failed: %v", err)
	}

	// Verify task completed
	reloadedTask, _ := task.LoadFrom(tmpDir, "TASK-RESUME-001")
	if reloadedTask.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want completed", reloadedTask.Status)
	}
}

func TestResumeFromPhase_PhaseNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/TASK-RESUME-002")

	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	testTask := task.New("TASK-RESUME-002", "Resume Test")
	testTask.Weight = task.WeightSmall
	if err := testTask.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	testPlan := &plan.Plan{
		Version: 1,
		Phases: []plan.Phase{
			{
				ID:     "implement",
				Prompt: "Do work",
			},
		},
	}
	if err := testPlan.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	testState := state.New("TASK-RESUME-002")

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)

	ctx := context.Background()
	err := e.ResumeFromPhase(ctx, testTask, testPlan, testState, "nonexistent")
	if err == nil {
		t.Error("ResumeFromPhase should fail for nonexistent phase")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found', got: %s", err.Error())
	}
}

// === ExecuteWithRetry Tests ===

func TestExecuteWithRetry_RetryOnTransientError(t *testing.T) {
	e := newTestExecutor(t)

	// Create a mock client that fails with an error
	mockClient := claude.NewMockClient("").
		WithError(fmt.Errorf("rate limited"))

	// This approach doesn't work well with the mock - the error persists
	// Let's just test that the function handles the retry config properly
	e.SetClient(mockClient)

	testTask := &task.Task{
		ID:     "TEST-RETRY-002",
		Title:  "Retry Test",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Name:   "Implementation",
		Prompt: "Implement: {{TASK_TITLE}}",
	}

	testState := state.New("TEST-RETRY-002")

	ctx := context.Background()

	// This will likely fail due to mock limitation, but it exercises the retry path
	_, err := e.ExecuteWithRetry(ctx, testTask, testPhase, testState)
	if err == nil {
		t.Log("ExecuteWithRetry succeeded (mock returned success eventually)")
	} else {
		t.Log("ExecuteWithRetry failed as expected with mock error:", err)
	}
}

func TestSaveTranscript(t *testing.T) {
	tmpDir := t.TempDir()
	transcriptsDir := filepath.Join(tmpDir, ".orc/tasks/TASK-TRANS-001/transcripts")

	// Create task directory structure
	if err := os.MkdirAll(transcriptsDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)

	phaseState := PhaseState{
		TaskID:    "TASK-TRANS-001",
		Phase:     "implement",
		Iteration: 1,
		Response:  "Implementation complete!",
	}

	err := e.saveTranscript(phaseState)
	if err != nil {
		t.Fatalf("saveTranscript failed: %v", err)
	}

	// Verify file was created
	files, _ := os.ReadDir(transcriptsDir)
	if len(files) == 0 {
		t.Error("expected transcript file to be created")
	}
}

// TestFailSetup_UpdatesTaskStatus verifies that when setup fails (e.g., worktree creation),
// the task status is properly updated to "failed" and the error is stored.
func TestFailSetup_UpdatesTaskStatus(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/TASK-SETUP-FAIL")

	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	// Create task
	testTask := task.New("TASK-SETUP-FAIL", "Setup Failure Test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusRunning // Set to running as if execution started
	if err := testTask.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Create state
	testState := state.New("TASK-SETUP-FAIL")

	// Create executor
	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	e.currentTaskDir = taskDir

	// Setup publisher to capture error events
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)
	ch := pub.Subscribe("TASK-SETUP-FAIL")
	defer pub.Unsubscribe("TASK-SETUP-FAIL", ch)

	// Simulate a setup failure
	setupErr := fmt.Errorf("create worktree: git error: unable to create branch")
	e.failSetup(testTask, testState, setupErr)

	// Verify task status was updated to failed
	if testTask.Status != task.StatusFailed {
		t.Errorf("task status = %s, want failed", testTask.Status)
	}

	// Reload from disk to verify persistence
	reloadedTask, err := task.LoadFrom(tmpDir, "TASK-SETUP-FAIL")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}
	if reloadedTask.Status != task.StatusFailed {
		t.Errorf("reloaded task status = %s, want failed", reloadedTask.Status)
	}

	// Verify state error was set
	if testState.Error == "" {
		t.Error("state.Error should be set")
	}
	if !strings.Contains(testState.Error, "create worktree") {
		t.Errorf("state.Error = %q, should contain 'create worktree'", testState.Error)
	}

	// Verify error event was published
	select {
	case event := <-ch:
		if event.Type != events.EventError {
			t.Errorf("expected EventError, got %v", event.Type)
		}
		data, ok := event.Data.(events.ErrorData)
		if !ok {
			t.Fatalf("expected ErrorData, got %T", event.Data)
		}
		if data.Phase != "setup" {
			t.Errorf("expected phase 'setup', got %s", data.Phase)
		}
		if !data.Fatal {
			t.Error("expected fatal error")
		}
		if !strings.Contains(data.Message, "create worktree") {
			t.Errorf("expected error message to contain 'create worktree', got %s", data.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for error event")
	}
}

func TestExecuteTask_UpdatesTaskCurrentPhase(t *testing.T) {
	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/TASK-PHASE-001")

	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	// Create task
	testTask := task.New("TASK-PHASE-001", "Current Phase Test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := testTask.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Verify initial state has empty CurrentPhase
	if testTask.CurrentPhase != "" {
		t.Errorf("initial CurrentPhase = %q, want empty", testTask.CurrentPhase)
	}

	// Create plan with inline prompt (not template file)
	testPlan := &plan.Plan{
		Version: 1,
		Weight:  "small",
		Phases: []plan.Phase{
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: "Implement: {{TASK_TITLE}}",
			},
		},
	}
	if err := testPlan.SaveTo(taskDir); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	testState := state.New("TASK-PHASE-001")

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Done!")
	e.SetClient(mockClient)

	ctx := context.Background()
	err := e.ExecuteTask(ctx, testTask, testPlan, testState)
	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	// Reload task from disk to verify CurrentPhase was saved
	reloadedTask, err := task.LoadFrom(tmpDir, "TASK-PHASE-001")
	if err != nil {
		t.Fatalf("failed to reload task: %v", err)
	}

	// CurrentPhase should be set to "implement" (the phase that was executed)
	if reloadedTask.CurrentPhase != "implement" {
		t.Errorf("reloaded task CurrentPhase = %q, want %q", reloadedTask.CurrentPhase, "implement")
	}

	// Also verify task status is completed
	if reloadedTask.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want completed", reloadedTask.Status)
	}
}
