// Package executor provides integration tests for the executor.
// Run with: go test -v ./internal/executor -run TestIntegration
// To use real Claude: ORC_REAL_CLAUDE=1 go test -v ./internal/executor -run TestIntegration
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// IntegrationTestCosts tracks token usage and costs for integration tests
type IntegrationTestCosts struct {
	mu           sync.Mutex
	TotalInputs  int             `json:"total_inputs"`
	TotalOutputs int             `json:"total_outputs"`
	TotalCost    float64         `json:"total_cost"`
	Tests        []TestCostEntry `json:"tests"`
}

// TestCostEntry records cost for a single test
type TestCostEntry struct {
	Name         string    `json:"name"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	Cost         float64   `json:"cost"`
	Duration     string    `json:"duration"`
	Timestamp    time.Time `json:"timestamp"`
}

// Claude pricing (per 1M tokens) - Sonnet 4 pricing
const (
	SonnetInputPrice  = 3.0  // $3 per 1M input tokens
	SonnetOutputPrice = 15.0 // $15 per 1M output tokens
)

func calculateCost(inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) / 1_000_000 * SonnetInputPrice
	outputCost := float64(outputTokens) / 1_000_000 * SonnetOutputPrice
	return inputCost + outputCost
}

var costs = &IntegrationTestCosts{}

func (c *IntegrationTestCosts) Add(name string, input, output int, duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cost := calculateCost(input, output)
	c.TotalInputs += input
	c.TotalOutputs += output
	c.TotalCost += cost

	c.Tests = append(c.Tests, TestCostEntry{
		Name:         name,
		InputTokens:  input,
		OutputTokens: output,
		Cost:         cost,
		Duration:     duration.String(),
		Timestamp:    time.Now(),
	})
}

func (c *IntegrationTestCosts) Report() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return fmt.Sprintf(`
=== Integration Test Token Usage ===
Total Tests: %d
Total Input Tokens: %d
Total Output Tokens: %d
Total Estimated Cost: $%.4f
====================================
`,
		len(c.Tests),
		c.TotalInputs,
		c.TotalOutputs,
		c.TotalCost,
	)
}

func (c *IntegrationTestCosts) SaveReport(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// useRealClaude returns true if ORC_REAL_CLAUDE=1
func useRealClaude() bool {
	return os.Getenv("ORC_REAL_CLAUDE") == "1"
}

// skipIfNoRealClaude skips the test unless ORC_REAL_CLAUDE=1
func skipIfNoRealClaude(t *testing.T) {
	if !useRealClaude() {
		t.Skip("Skipping integration test (set ORC_REAL_CLAUDE=1 to run)")
	}
}

// TestMain runs after all tests to print cost report
func TestMain(m *testing.M) {
	code := m.Run()

	if useRealClaude() && len(costs.Tests) > 0 {
		fmt.Println(costs.Report())

		// Save detailed report
		reportPath := filepath.Join(os.TempDir(), "orc-integration-test-costs.json")
		if err := costs.SaveReport(reportPath); err == nil {
			fmt.Printf("Detailed report saved to: %s\n", reportPath)
		}
	}

	os.Exit(code)
}

// === Integration Tests (require ORC_REAL_CLAUDE=1) ===

func TestIntegration_ExecutePhase_Complete(t *testing.T) {
	t.Parallel()
	skipIfNoRealClaude(t)

	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/INT-001")

	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)

	// Use real Claude client
	client := claude.NewClaudeCLI(claude.WithDangerouslySkipPermissions())
	e.SetClient(client)

	testTask := &task.Task{
		ID:     "INT-001",
		Title:  "Say hello in 10 words or less",
		Status: task.StatusRunning,
		Weight: task.WeightTrivial,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Name:   "Implementation",
		Prompt: `Just respond with JSON: {"status": "complete", "summary": "Done"}`,
	}

	testState := state.New("INT-001")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	start := time.Now()
	result, err := e.ExecutePhase(ctx, testTask, testPhase, testState)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ExecutePhase failed: %v", err)
	}

	costs.Add(t.Name(), result.InputTokens, result.OutputTokens, duration)

	if result.Status != plan.PhaseCompleted {
		t.Errorf("expected status Completed, got %v", result.Status)
	}

	t.Logf("Phase completed in %v, tokens: %d input / %d output",
		duration, result.InputTokens, result.OutputTokens)
}

func TestIntegration_ExecuteTask_SinglePhase(t *testing.T) {
	t.Parallel()
	skipIfNoRealClaude(t)

	tmpDir := t.TempDir()

	// Create backend for storage
	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	e.SetBackend(backend)

	client := claude.NewClaudeCLI(claude.WithDangerouslySkipPermissions())
	e.SetClient(client)

	// Set up publisher to track events
	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)
	eventCh := pub.Subscribe("INT-002")
	defer pub.Unsubscribe("INT-002", eventCh)

	testTask := task.New("INT-002", "Integration test task")
	testTask.Weight = task.WeightTrivial
	testTask.Status = task.StatusPlanned
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	testPlan := &plan.Plan{
		Version:     1,
		Weight:      "trivial",
		Description: "Integration test",
		Phases: []plan.Phase{
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: `Respond only with JSON: {"status": "complete", "summary": "Done"}`,
			},
		},
	}
	if err := backend.SavePlan(testPlan, "INT-002"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	testState := state.New("INT-002")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	start := time.Now()
	err = e.ExecuteTask(ctx, testTask, testPlan, testState)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	// Collect events
	var receivedEvents []events.Event
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case evt := <-eventCh:
				receivedEvents = append(receivedEvents, evt)
				if evt.Type == events.EventComplete {
					return
				}
			case <-time.After(5 * time.Second):
				return
			}
		}
	}()
	<-done

	// Track costs from state
	reloadedState, _ := backend.LoadState("INT-002")
	costs.Add(t.Name(),
		reloadedState.Tokens.InputTokens,
		reloadedState.Tokens.OutputTokens,
		duration)

	reloadedTask, _ := backend.LoadTask("INT-002")
	if reloadedTask.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want completed", reloadedTask.Status)
	}

	t.Logf("Task completed in %v, events received: %d", duration, len(receivedEvents))
}

func TestIntegration_ExecuteTask_MultiPhase(t *testing.T) {
	t.Parallel()
	skipIfNoRealClaude(t)

	tmpDir := t.TempDir()

	// Create backend for storage
	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	cfg := DefaultConfig()
	cfg.MaxIterations = 5
	cfg.WorkDir = tmpDir
	e := New(cfg)
	e.SetBackend(backend)

	client := claude.NewClaudeCLI(claude.WithDangerouslySkipPermissions())
	e.SetClient(client)

	testTask := task.New("INT-003", "Multi-phase integration test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	testPlan := &plan.Plan{
		Version: 1,
		Weight:  "small",
		Phases: []plan.Phase{
			{
				ID:     "spec",
				Name:   "Specification",
				Prompt: `Respond only with JSON: {"status": "complete", "summary": "Done"}`,
			},
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: `Respond only with JSON: {"status": "complete", "summary": "Done"}`,
			},
		},
	}
	if err := backend.SavePlan(testPlan, "INT-003"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	testState := state.New("INT-003")

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	start := time.Now()
	err = e.ExecuteTask(ctx, testTask, testPlan, testState)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	reloadedState, _ := backend.LoadState("INT-003")
	costs.Add(t.Name(),
		reloadedState.Tokens.InputTokens,
		reloadedState.Tokens.OutputTokens,
		duration)

	reloadedTask, _ := backend.LoadTask("INT-003")
	if reloadedTask.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want completed", reloadedTask.Status)
	}

	// Verify both phases completed
	if reloadedState.Phases["spec"].Status != state.StatusCompleted {
		t.Error("spec phase should be completed")
	}
	if reloadedState.Phases["implement"].Status != state.StatusCompleted {
		t.Error("implement phase should be completed")
	}

	t.Logf("Multi-phase task completed in %v", duration)
}

func TestIntegration_Pause_Resume(t *testing.T) {
	t.Parallel()
	skipIfNoRealClaude(t)

	tmpDir := t.TempDir()

	// Create backend for storage
	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	e.SetBackend(backend)

	client := claude.NewClaudeCLI(claude.WithDangerouslySkipPermissions())
	e.SetClient(client)

	testTask := task.New("INT-004", "Pause/Resume test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	testPlan := &plan.Plan{
		Version: 1,
		Weight:  "small",
		Phases: []plan.Phase{
			{
				ID:     "phase1",
				Prompt: `Respond only with JSON: {"status": "complete", "summary": "Done"}`,
			},
			{
				ID:     "phase2",
				Prompt: `Respond only with JSON: {"status": "complete", "summary": "Done"}`,
			},
		},
	}
	if err := backend.SavePlan(testPlan, "INT-004"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	testState := state.New("INT-004")

	// Execute first phase
	ctx := context.Background()
	start := time.Now()

	phase1 := &testPlan.Phases[0]
	result1, err := e.ExecutePhase(ctx, testTask, phase1, testState)
	if err != nil {
		t.Fatalf("Phase 1 failed: %v", err)
	}

	// Simulate pause
	testState.InterruptPhase("phase1")
	testTask.Status = task.StatusPaused
	if err := backend.SaveState(testState); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	// Verify paused state
	reloadedTask, _ := backend.LoadTask("INT-004")
	if reloadedTask.Status != task.StatusPaused {
		t.Error("Task should be paused")
	}

	// Resume and execute phase2
	testTask.Status = task.StatusRunning
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	phase2 := &testPlan.Phases[1]
	result2, err := e.ExecutePhase(ctx, testTask, phase2, testState)
	if err != nil {
		t.Fatalf("Phase 2 failed: %v", err)
	}

	duration := time.Since(start)
	totalInput := result1.InputTokens + result2.InputTokens
	totalOutput := result1.OutputTokens + result2.OutputTokens

	costs.Add(t.Name(), totalInput, totalOutput, duration)

	t.Logf("Pause/Resume test completed in %v", duration)
}

// === Mock-based tests (always run) ===

func TestMock_ExecutePhase_Complete(t *testing.T) {
	t.Parallel()
	if useRealClaude() {
		t.Skip("Skipping mock test when ORC_REAL_CLAUDE=1")
	}

	tmpDir := t.TempDir()
	taskDir := filepath.Join(tmpDir, ".orc/tasks/MOCK-001")

	if err := os.MkdirAll(taskDir, 0755); err != nil {
		t.Fatalf("failed to create task dir: %v", err)
	}

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	// Use mock TurnExecutor instead of real Claude CLI
	mockExecutor := NewMockTurnExecutor(`{"status": "complete", "summary": "Done!"}`)
	e.SetTurnExecutor(mockExecutor)
	// Disable validation/backpressure for testing
	e.SetOrcConfig(&config.Config{Validation: config.ValidationConfig{Enabled: false}})

	testTask := &task.Task{
		ID:     "MOCK-001",
		Title:  "Mock test",
		Status: task.StatusRunning,
		Weight: task.WeightSmall,
	}

	testPhase := &plan.Phase{
		ID:     "implement",
		Prompt: "Test prompt",
	}

	testState := state.New("MOCK-001")

	ctx := context.Background()
	result, err := e.ExecutePhase(ctx, testTask, testPhase, testState)

	if err != nil {
		t.Fatalf("ExecutePhase failed: %v", err)
	}

	if result.Status != plan.PhaseCompleted {
		t.Errorf("expected status Completed, got %v", result.Status)
	}
}

func TestMock_ExecuteTask_WithEvents(t *testing.T) {
	t.Parallel()
	if useRealClaude() {
		t.Skip("Skipping mock test when ORC_REAL_CLAUDE=1")
	}

	tmpDir := t.TempDir()

	// Create backend for storage
	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	e.SetBackend(backend)
	// Use mock TurnExecutor instead of real Claude CLI
	mockExecutor := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	e.SetTurnExecutor(mockExecutor)
	// Disable validation/backpressure for testing
	e.SetOrcConfig(&config.Config{Validation: config.ValidationConfig{Enabled: false}})

	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)
	eventCh := pub.Subscribe("MOCK-002")
	defer pub.Unsubscribe("MOCK-002", eventCh)

	testTask := task.New("MOCK-002", "Event test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	testPlan := &plan.Plan{
		Version: 1,
		Phases: []plan.Phase{
			{ID: "implement", Prompt: "Test"},
		},
	}
	if err := backend.SavePlan(testPlan, "MOCK-002"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	testState := state.New("MOCK-002")

	// Collect events
	var receivedEvents []events.Event
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case evt := <-eventCh:
				receivedEvents = append(receivedEvents, evt)
				if evt.Type == events.EventComplete {
					return
				}
			case <-time.After(2 * time.Second):
				return
			}
		}
	}()

	ctx := context.Background()
	err = e.ExecuteTask(ctx, testTask, testPlan, testState)

	<-done

	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	// Verify events were published
	hasPhaseEvent := false
	hasCompleteEvent := false
	for _, evt := range receivedEvents {
		if evt.Type == events.EventPhase {
			hasPhaseEvent = true
		}
		if evt.Type == events.EventComplete {
			hasCompleteEvent = true
		}
	}

	if !hasPhaseEvent {
		t.Error("Expected phase events")
	}
	if !hasCompleteEvent {
		t.Error("Expected complete event")
	}
}

// TestMock_ExecuteTask_SetsStartedAt verifies the fix for TASK-284:
// ExecuteTask must set state.StartedAt when execution begins, ensuring
// Elapsed() returns a valid duration instead of 0.
func TestMock_ExecuteTask_SetsStartedAt(t *testing.T) {
	t.Parallel()
	if useRealClaude() {
		t.Skip("Skipping mock test when ORC_REAL_CLAUDE=1")
	}

	tmpDir := t.TempDir()

	// Create backend for storage
	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	e := New(cfg)
	e.SetBackend(backend)
	// Use mock TurnExecutor instead of real Claude CLI
	mockExecutor := NewMockTurnExecutor(`{"status": "complete", "summary": "Done"}`)
	e.SetTurnExecutor(mockExecutor)
	// Disable validation/backpressure for testing
	e.SetOrcConfig(&config.Config{Validation: config.ValidationConfig{Enabled: false}})

	testTask := task.New("MOCK-ELAPSED", "Elapsed time test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

	testPlan := &plan.Plan{
		Version: 1,
		Phases: []plan.Phase{
			{ID: "implement", Prompt: "Test"},
		},
	}
	if err := backend.SavePlan(testPlan, "MOCK-ELAPSED"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	// CRITICAL: Create a state with zero StartedAt to simulate a loaded state
	// from the database. This is the bug scenario - states loaded from DB
	// may have zero StartedAt if the task hasn't started yet.
	testState := &state.State{
		TaskID: "MOCK-ELAPSED",
		Status: state.StatusPending,
		Phases: make(map[string]*state.PhaseState),
		// StartedAt is intentionally zero (default)
	}

	// Verify pre-condition: StartedAt should be zero
	if !testState.StartedAt.IsZero() {
		t.Fatal("Pre-condition failed: StartedAt should be zero before ExecuteTask")
	}

	beforeExec := time.Now()
	ctx := context.Background()
	err = e.ExecuteTask(ctx, testTask, testPlan, testState)
	afterExec := time.Now()

	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	// VERIFY FIX: StartedAt should now be set
	if testState.StartedAt.IsZero() {
		t.Fatal("BUG: StartedAt is still zero after ExecuteTask - Elapsed() would return 0")
	}

	// Verify StartedAt is within expected range
	if testState.StartedAt.Before(beforeExec) || testState.StartedAt.After(afterExec) {
		t.Errorf("StartedAt = %v, want between %v and %v", testState.StartedAt, beforeExec, afterExec)
	}

	// Verify Elapsed() returns a sensible value
	elapsed := testState.Elapsed()
	if elapsed < 0 {
		t.Errorf("Elapsed() = %v, want non-negative", elapsed)
	}
	// Should be a small positive value (test runs in milliseconds)
	if elapsed > 5*time.Second {
		t.Errorf("Elapsed() = %v, unexpectedly large", elapsed)
	}

	t.Logf("ExecuteTask properly set StartedAt; Elapsed() = %v", elapsed)
}

// TestIntegration_PhaseTimeout_EnforcesLimit verifies that PhaseMax timeout is enforced
// and produces a recoverable interrupted state.
func TestIntegration_PhaseTimeout_EnforcesLimit(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backend
	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	cfg.Backend = backend

	// Create orc config with a short PhaseMax timeout
	orcCfg := &config.Config{
		Timeouts: config.TimeoutsConfig{
			PhaseMax: 50 * time.Millisecond, // Short timeout for testing
		},
	}

	e := NewWithConfig(cfg, orcCfg)
	e.SetBackend(backend)

	// Use a mock executor that takes longer than the timeout
	mockExecutor := NewMockTurnExecutor(`{"status": "complete", "summary": "Done!"}`)
	mockExecutor.Delay = 500 * time.Millisecond // Longer than PhaseMax
	e.SetTurnExecutor(mockExecutor)

	// Create task
	testTask := task.New("INT-TIMEOUT", "Phase timeout test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

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
	if err := backend.SavePlan(testPlan, "INT-TIMEOUT"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	testState := state.New("INT-TIMEOUT")

	// Execute - should timeout
	ctx := context.Background()
	err = e.ExecuteTask(ctx, testTask, testPlan, testState)

	// Should get a timeout error
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}

	// Verify it's a phase timeout error
	if !isPhaseTimeoutError(err) {
		t.Fatalf("expected phaseTimeoutError, got %T: %v", err, err)
	}

	// Verify error message includes task ID and resume hint
	errMsg := err.Error()
	if !strings.Contains(errMsg, "INT-TIMEOUT") {
		t.Errorf("error message should contain task ID, got: %s", errMsg)
	}
	if !strings.Contains(errMsg, "orc resume") {
		t.Errorf("error message should contain resume hint, got: %s", errMsg)
	}

	// Verify task is failed (timeout is an error condition, recoverable via orc resume)
	reloadedTask, loadErr := backend.LoadTask("INT-TIMEOUT")
	if loadErr != nil {
		t.Fatalf("failed to reload task: %v", loadErr)
	}

	if reloadedTask.Status != task.StatusFailed {
		t.Errorf("task status = %s, want failed (timeout is an error condition)", reloadedTask.Status)
	}

	// Verify phase is failed
	reloadedState, stateErr := backend.LoadState("INT-TIMEOUT")
	if stateErr != nil {
		t.Fatalf("failed to reload state: %v", stateErr)
	}

	if reloadedState.Phases["implement"].Status != state.StatusFailed {
		t.Errorf("phase status = %s, want failed", reloadedState.Phases["implement"].Status)
	}

	t.Logf("PhaseMax timeout correctly enforced - task can be resumed via 'orc resume'")
}

// TestIntegration_PhaseTimeout_Disabled verifies that PhaseMax=0 disables timeout.
func TestIntegration_PhaseTimeout_Disabled(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	backend, err := storage.NewDatabaseBackend(tmpDir, &config.StorageConfig{})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer func() { _ = backend.Close() }()

	cfg := DefaultConfig()
	cfg.WorkDir = tmpDir
	cfg.Backend = backend

	// PhaseMax=0 means unlimited
	orcCfg := &config.Config{
		Timeouts: config.TimeoutsConfig{
			PhaseMax: 0,
		},
	}

	e := NewWithConfig(cfg, orcCfg)
	e.SetBackend(backend)

	// Use mock TurnExecutor instead of real Claude CLI
	mockExecutor := NewMockTurnExecutor(`{"status": "complete", "summary": "Done!"}`)
	e.SetTurnExecutor(mockExecutor)
	// Disable validation/backpressure for testing
	e.SetOrcConfig(&config.Config{
		Timeouts:   config.TimeoutsConfig{PhaseMax: 0},
		Validation: config.ValidationConfig{Enabled: false},
	})

	testTask := task.New("INT-NO-TIMEOUT", "No timeout test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	if err := backend.SaveTask(testTask); err != nil {
		t.Fatalf("failed to save task: %v", err)
	}

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
	if err := backend.SavePlan(testPlan, "INT-NO-TIMEOUT"); err != nil {
		t.Fatalf("failed to save plan: %v", err)
	}

	testState := state.New("INT-NO-TIMEOUT")

	ctx := context.Background()
	err = e.ExecuteTask(ctx, testTask, testPlan, testState)

	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	reloadedTask, _ := backend.LoadTask("INT-NO-TIMEOUT")
	if reloadedTask.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want completed", reloadedTask.Status)
	}

	t.Logf("PhaseMax=0 correctly disables timeout")
}
