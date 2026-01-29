package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// newTestBackend creates a test backend using in-memory database for speed.
func newTestBackend(t *testing.T) storage.Backend {
	t.Helper()
	return storage.NewTestBackend(t)
}

// newTestExecutor creates an executor configured for test isolation.
func newTestExecutor(t *testing.T) *Executor {
	t.Helper()
	cfg := DefaultConfig()
	cfg.WorkDir = t.TempDir()
	cfg.Backend = newTestBackend(t)
	return New(cfg)
}

func TestResolveClaudePath(t *testing.T) {
	t.Parallel()
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
			result := ResolveClaudePath(tc.input)

			if tc.wantSame && result != tc.input {
				t.Errorf("ResolveClaudePath(%q) = %q, want %q", tc.input, result, tc.input)
			}

			if tc.wantAbs && result != "" && !filepath.IsAbs(result) {
				// Only check for absolute if we expect it AND claude is actually in PATH
				// If claude isn't installed, it should fall back to the original
				if tc.input != "claude" {
					t.Errorf("ResolveClaudePath(%q) = %q, want absolute path", tc.input, result)
				}
			}
		})
	}
}

func TestFindClaudeInCommonLocations(t *testing.T) {
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
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
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
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
	defer func() { _ = os.RemoveAll(testDir) }()

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
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
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
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
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
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
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
	// Note: Cannot use t.Parallel() - modifies global commonClaudeLocations
	// Test that ResolveClaudePath falls back to common locations
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
	_ = os.Setenv("PATH", t.TempDir())
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	result := ResolveClaudePath("claude")
	if result != fakeClaude {
		t.Errorf("ResolveClaudePath(\"claude\") = %q, want %q (from common locations)", result, fakeClaude)
	}
}

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

	// Verify claude path is set (required for ClaudeExecutor-based execution)
	if e.claudePath == "" {
		t.Error("executor claudePath is empty - ClaudeExecutor won't work")
	}
}

// TestNew_ClaudePathResolution verifies that the claude path is resolved
// correctly. This is critical: without the Claude path, ClaudeExecutor-based
// execution fails to spawn Claude processes.
func TestNew_ClaudePathResolution(t *testing.T) {
	t.Parallel()
	// Create a fake claude binary to test with
	tmpDir := t.TempDir()
	fakeClaude := filepath.Join(tmpDir, "claude")
	if err := os.WriteFile(fakeClaude, []byte("#!/bin/sh\necho fake"), 0755); err != nil {
		t.Fatalf("failed to create fake claude: %v", err)
	}

	cfg := DefaultConfig()
	cfg.ClaudePath = fakeClaude
	cfg.WorkDir = t.TempDir()
	e := New(cfg)

	if e == nil {
		t.Fatal("New() returned nil")
	}

	// The claude path must be set for ClaudeExecutor-based execution to work
	if e.claudePath == "" {
		t.Fatal("claudePath is empty - ClaudeExecutor will fail")
	}

	// Verify the path was resolved to an absolute path
	if !filepath.IsAbs(e.claudePath) {
		t.Errorf("claudePath should be absolute, got %q", e.claudePath)
	}
}

func TestNewWithNilConfig(t *testing.T) {
	t.Parallel()
	e := New(nil)

	if e == nil {
		t.Fatal("New(nil) returned nil")
	}

	// Should use defaults
	if e.config.MaxIterations != 30 {
		t.Errorf("MaxIterations = %d, want 30", e.config.MaxIterations)
	}
}

// Note: TestRenderTemplate* tests removed - template rendering is tested in template_test.go

func TestPhaseState(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

func TestPhaseStateWithAllFields(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
				Model:                      "sonnet",
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	e := newTestExecutor(t)

	mockClient := claude.NewMockClient(`{"status": "complete", "summary": "Done"}`)
	e.SetClient(mockClient)

	// Verify client was set (by making a request)
	if e.client == nil {
		t.Error("client should not be nil after SetClient")
	}
}

func TestPublishHelpers(t *testing.T) {
	t.Parallel()
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
		if data.Status != "running" {
			t.Errorf("expected status running, got %s", data.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestPublishPhaseComplete(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	ch := pub.Subscribe("TASK-003")
	defer pub.Unsubscribe("TASK-003", ch)

	testErr := fmt.Errorf("something went wrong")
	e.publishPhaseFailed("TASK-003", "review", testErr)

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
	t.Parallel()
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
	t.Parallel()
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	ch := pub.Subscribe("TASK-005")
	defer pub.Unsubscribe("TASK-005", ch)

	e.publishTokens("TASK-005", "spec", 1000, 500, 0, 0, 1500)

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
	t.Parallel()
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
	t.Parallel()
	e := newTestExecutor(t)
	// No publisher set - should not panic
	e.publishPhaseStart("TASK-001", "test")
	e.publishPhaseComplete("TASK-001", "test", "sha")
	e.publishPhaseFailed("TASK-001", "test", nil)
	e.publishTranscript("TASK-001", "test", 1, "type", "content")
	e.publishTokens("TASK-001", "test", 0, 0, 0, 0, 0)
	e.publishError("TASK-001", "test", "msg", false)
}

func TestPublishState(t *testing.T) {
	t.Parallel()
	e := newTestExecutor(t)
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)

	ch := pub.Subscribe("TASK-007")
	defer pub.Unsubscribe("TASK-007", ch)

	testExec := task.InitProtoExecutionState()
	task.StartPhaseProto(testExec, "implement")

	e.publishState("TASK-007", testExec)

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

// Note: TestBuildPromptNode*, TestExecutePhase*, TestEvaluateGate*, TestExecuteWithRetry*,
// TestExecuteTask*, TestResumeFromPhase*, TestFailSetup*, TestHandlePhaseFailure* tests removed -
// those methods were deleted during executor consolidation.
// See internal/executor/workflow_executor_test.go for tests of the new WorkflowExecutor.
