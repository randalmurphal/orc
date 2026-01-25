package task

import (
	"testing"
	"time"
)

func TestTokenUsage_EffectiveInputTokens(t *testing.T) {
	tests := []struct {
		name     string
		usage    TokenUsage
		expected int
	}{
		{
			name: "no cache tokens",
			usage: TokenUsage{
				InputTokens:              1000,
				OutputTokens:             500,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     0,
				TotalTokens:              1500,
			},
			expected: 1000,
		},
		{
			name: "with cache read tokens",
			usage: TokenUsage{
				InputTokens:              56,
				OutputTokens:             500,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     27944,
				TotalTokens:              556,
			},
			expected: 28000, // 56 + 27944 = actual context window
		},
		{
			name: "with cache creation tokens",
			usage: TokenUsage{
				InputTokens:              5000,
				OutputTokens:             1000,
				CacheCreationInputTokens: 3000,
				CacheReadInputTokens:     0,
				TotalTokens:              6000,
			},
			expected: 8000, // 5000 + 3000
		},
		{
			name: "with both cache types",
			usage: TokenUsage{
				InputTokens:              100,
				OutputTokens:             200,
				CacheCreationInputTokens: 1000,
				CacheReadInputTokens:     5000,
				TotalTokens:              300,
			},
			expected: 6100, // 100 + 1000 + 5000
		},
		{
			name: "zero values",
			usage: TokenUsage{
				InputTokens:              0,
				OutputTokens:             0,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     0,
				TotalTokens:              0,
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.usage.EffectiveInputTokens()
			if got != tt.expected {
				t.Errorf("EffectiveInputTokens() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestTokenUsage_EffectiveTotalTokens(t *testing.T) {
	tests := []struct {
		name     string
		usage    TokenUsage
		expected int
	}{
		{
			name: "no cache tokens",
			usage: TokenUsage{
				InputTokens:              1000,
				OutputTokens:             500,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     0,
				TotalTokens:              1500,
			},
			expected: 1500, // 1000 + 500
		},
		{
			name: "with cache read tokens",
			usage: TokenUsage{
				InputTokens:              56,
				OutputTokens:             500,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     27944,
				TotalTokens:              556,
			},
			expected: 28500, // 56 + 27944 + 500
		},
		{
			name: "with both cache types",
			usage: TokenUsage{
				InputTokens:              100,
				OutputTokens:             200,
				CacheCreationInputTokens: 1000,
				CacheReadInputTokens:     5000,
				TotalTokens:              300,
			},
			expected: 6300, // 100 + 1000 + 5000 + 200
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.usage.EffectiveTotalTokens()
			if got != tt.expected {
				t.Errorf("EffectiveTotalTokens() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestExecutionState_AddTokens(t *testing.T) {
	state := &ExecutionState{
		Tokens: TokenUsage{},
		Phases: map[string]*PhaseState{
			"implement": {Status: PhaseStatusRunning},
		},
	}

	// Add tokens for implement phase
	state.AddTokens("implement", 1000, 500, 200, 300)

	// Check totals
	if state.Tokens.InputTokens != 1000 {
		t.Errorf("InputTokens = %d, want 1000", state.Tokens.InputTokens)
	}
	if state.Tokens.OutputTokens != 500 {
		t.Errorf("OutputTokens = %d, want 500", state.Tokens.OutputTokens)
	}
	if state.Tokens.CacheCreationInputTokens != 200 {
		t.Errorf("CacheCreationInputTokens = %d, want 200", state.Tokens.CacheCreationInputTokens)
	}
	if state.Tokens.CacheReadInputTokens != 300 {
		t.Errorf("CacheReadInputTokens = %d, want 300", state.Tokens.CacheReadInputTokens)
	}
	if state.Tokens.TotalTokens != 1500 {
		t.Errorf("TotalTokens = %d, want 1500", state.Tokens.TotalTokens)
	}

	// Check phase-level tokens
	phase := state.Phases["implement"]
	if phase.Tokens.InputTokens != 1000 {
		t.Errorf("Phase InputTokens = %d, want 1000", phase.Tokens.InputTokens)
	}
	if phase.Tokens.OutputTokens != 500 {
		t.Errorf("Phase OutputTokens = %d, want 500", phase.Tokens.OutputTokens)
	}

	// Add more tokens
	state.AddTokens("implement", 500, 250, 100, 150)

	// Verify accumulation
	if state.Tokens.InputTokens != 1500 {
		t.Errorf("InputTokens after second add = %d, want 1500", state.Tokens.InputTokens)
	}
	if state.Tokens.TotalTokens != 2250 {
		t.Errorf("TotalTokens after second add = %d, want 2250", state.Tokens.TotalTokens)
	}
}

func TestExecutionState_AddCost(t *testing.T) {
	state := &ExecutionState{
		Cost: CostTracking{},
	}

	// Add cost for first phase
	state.AddCost("implement", 0.50)
	if state.Cost.TotalCostUSD != 0.50 {
		t.Errorf("TotalCostUSD = %f, want 0.50", state.Cost.TotalCostUSD)
	}
	if state.Cost.PhaseCosts["implement"] != 0.50 {
		t.Errorf("implement cost = %f, want 0.50", state.Cost.PhaseCosts["implement"])
	}

	// Add more cost to same phase
	state.AddCost("implement", 0.25)
	if state.Cost.TotalCostUSD != 0.75 {
		t.Errorf("TotalCostUSD = %f, want 0.75", state.Cost.TotalCostUSD)
	}
	if state.Cost.PhaseCosts["implement"] != 0.75 {
		t.Errorf("implement cost = %f, want 0.75", state.Cost.PhaseCosts["implement"])
	}

	// Add cost to different phase
	state.AddCost("review", 0.10)
	if state.Cost.TotalCostUSD != 0.85 {
		t.Errorf("TotalCostUSD = %f, want 0.85", state.Cost.TotalCostUSD)
	}
	if state.Cost.PhaseCosts["review"] != 0.10 {
		t.Errorf("review cost = %f, want 0.10", state.Cost.PhaseCosts["review"])
	}
}

func TestPhaseState_Status(t *testing.T) {
	state := &ExecutionState{
		Phases: make(map[string]*PhaseState),
	}

	// Start a phase
	state.StartPhase("implement")
	if state.Phases["implement"] == nil {
		t.Fatal("StartPhase should create phase state")
	}
	if state.Phases["implement"].Status != PhaseStatusRunning {
		t.Errorf("Status = %s, want %s", state.Phases["implement"].Status, PhaseStatusRunning)
	}
	if state.Phases["implement"].StartedAt.IsZero() {
		t.Error("StartedAt should be set")
	}

	// Complete the phase
	state.CompletePhase("implement", "abc123")
	if state.Phases["implement"].Status != PhaseStatusCompleted {
		t.Errorf("Status = %s, want %s", state.Phases["implement"].Status, PhaseStatusCompleted)
	}
	if state.Phases["implement"].CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %s, want abc123", state.Phases["implement"].CommitSHA)
	}
	if state.Phases["implement"].CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
}

func TestPhaseState_FailAndInterrupt(t *testing.T) {
	state := &ExecutionState{
		Phases: make(map[string]*PhaseState),
	}

	// Fail a phase
	state.StartPhase("implement")
	testErr := &testError{msg: "test failure"}
	state.FailPhase("implement", testErr)

	if state.Phases["implement"].Status != PhaseStatusFailed {
		t.Errorf("Status = %s, want %s", state.Phases["implement"].Status, PhaseStatusFailed)
	}

	// Test interrupt
	state.StartPhase("review")
	state.InterruptPhase("review")

	if state.Phases["review"].Status != PhaseStatusInterrupted {
		t.Errorf("Status = %s, want %s", state.Phases["review"].Status, PhaseStatusInterrupted)
	}
}

// testError is a simple error for testing
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestGateDecision(t *testing.T) {
	decision := GateDecision{
		Phase:     "review",
		GateType:  "auto",
		Approved:  true,
		Reason:    "automated approval",
		Timestamp: time.Now(),
	}

	if decision.Phase != "review" {
		t.Errorf("Phase = %s, want review", decision.Phase)
	}
	if decision.GateType != "auto" {
		t.Errorf("GateType = %s, want auto", decision.GateType)
	}
	if !decision.Approved {
		t.Error("Approved should be true")
	}
}
