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
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// IntegrationTestCosts tracks token usage and costs for integration tests
type IntegrationTestCosts struct {
	mu           sync.Mutex
	TotalInputs  int              `json:"total_inputs"`
	TotalOutputs int              `json:"total_outputs"`
	TotalCost    float64          `json:"total_cost"`
	Tests        []TestCostEntry  `json:"tests"`
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
	skipIfNoRealClaude(t)

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks/INT-001", 0755)

	cfg := DefaultConfig()
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
		Prompt: "Just respond with <phase_complete>true</phase_complete> and nothing else.",
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
	skipIfNoRealClaude(t)

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks/INT-002", 0755)

	cfg := DefaultConfig()
	e := New(cfg)

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
	testTask.Save()

	testPlan := &plan.Plan{
		Version:     1,
		Weight:      "trivial",
		Description: "Integration test",
		Phases: []plan.Phase{
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: "Respond only with: <phase_complete>true</phase_complete>",
			},
		},
	}
	testPlan.Save("INT-002")

	testState := state.New("INT-002")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	start := time.Now()
	err := e.ExecuteTask(ctx, testTask, testPlan, testState)
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
	reloadedState, _ := state.Load("INT-002")
	costs.Add(t.Name(),
		reloadedState.Tokens.InputTokens,
		reloadedState.Tokens.OutputTokens,
		duration)

	reloadedTask, _ := task.Load("INT-002")
	if reloadedTask.Status != task.StatusCompleted {
		t.Errorf("task status = %s, want completed", reloadedTask.Status)
	}

	t.Logf("Task completed in %v, events received: %d", duration, len(receivedEvents))
}

func TestIntegration_ExecuteTask_MultiPhase(t *testing.T) {
	skipIfNoRealClaude(t)

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks/INT-003", 0755)

	cfg := DefaultConfig()
	cfg.MaxIterations = 5
	e := New(cfg)

	client := claude.NewClaudeCLI(claude.WithDangerouslySkipPermissions())
	e.SetClient(client)

	testTask := task.New("INT-003", "Multi-phase integration test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	testTask.Save()

	testPlan := &plan.Plan{
		Version: 1,
		Weight:  "small",
		Phases: []plan.Phase{
			{
				ID:     "spec",
				Name:   "Specification",
				Prompt: "Respond only with: <phase_complete>true</phase_complete>",
			},
			{
				ID:     "implement",
				Name:   "Implementation",
				Prompt: "Respond only with: <phase_complete>true</phase_complete>",
			},
		},
	}
	testPlan.Save("INT-003")

	testState := state.New("INT-003")

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	start := time.Now()
	err := e.ExecuteTask(ctx, testTask, testPlan, testState)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ExecuteTask failed: %v", err)
	}

	reloadedState, _ := state.Load("INT-003")
	costs.Add(t.Name(),
		reloadedState.Tokens.InputTokens,
		reloadedState.Tokens.OutputTokens,
		duration)

	reloadedTask, _ := task.Load("INT-003")
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
	skipIfNoRealClaude(t)

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks/INT-004", 0755)

	cfg := DefaultConfig()
	e := New(cfg)

	client := claude.NewClaudeCLI(claude.WithDangerouslySkipPermissions())
	e.SetClient(client)

	testTask := task.New("INT-004", "Pause/Resume test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	testTask.Save()

	testPlan := &plan.Plan{
		Version: 1,
		Weight:  "small",
		Phases: []plan.Phase{
			{
				ID:     "phase1",
				Prompt: "Respond only with: <phase_complete>true</phase_complete>",
			},
			{
				ID:     "phase2",
				Prompt: "Respond only with: <phase_complete>true</phase_complete>",
			},
		},
	}
	testPlan.Save("INT-004")

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
	testState.Save()
	testTask.Save()

	// Verify paused state
	reloadedTask, _ := task.Load("INT-004")
	if reloadedTask.Status != task.StatusPaused {
		t.Error("Task should be paused")
	}

	// Resume and execute phase2
	testTask.Status = task.StatusRunning
	testTask.Save()

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
	if useRealClaude() {
		t.Skip("Skipping mock test when ORC_REAL_CLAUDE=1")
	}

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks/MOCK-001", 0755)

	e := New(DefaultConfig())
	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>Done!")
	e.SetClient(mockClient)

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
	if useRealClaude() {
		t.Skip("Skipping mock test when ORC_REAL_CLAUDE=1")
	}

	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	os.MkdirAll(".orc/tasks/MOCK-002", 0755)

	e := New(DefaultConfig())
	mockClient := claude.NewMockClient("<phase_complete>true</phase_complete>")
	e.SetClient(mockClient)

	pub := events.NewMemoryPublisher()
	e.SetPublisher(pub)
	eventCh := pub.Subscribe("MOCK-002")
	defer pub.Unsubscribe("MOCK-002", eventCh)

	testTask := task.New("MOCK-002", "Event test")
	testTask.Weight = task.WeightSmall
	testTask.Status = task.StatusPlanned
	testTask.Save()

	testPlan := &plan.Plan{
		Version: 1,
		Phases: []plan.Phase{
			{ID: "implement", Prompt: "Test"},
		},
	}
	testPlan.Save("MOCK-002")

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
	err := e.ExecuteTask(ctx, testTask, testPlan, testState)

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
