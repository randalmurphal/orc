package gate

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/db"
)

// mockLLMClient implements claude.Client for testing AI gate evaluation.
type mockLLMClient struct {
	response string
	usage    claude.TokenUsage
	model    string
	err      error
	// capturedPrompt records the prompt sent to Complete for inspection.
	capturedPrompt string
}

func (m *mockLLMClient) Complete(_ context.Context, req claude.CompletionRequest) (*claude.CompletionResponse, error) {
	if len(req.Messages) > 0 {
		m.capturedPrompt = req.Messages[0].Content
	}
	if m.err != nil {
		return nil, m.err
	}
	return &claude.CompletionResponse{
		Content: m.response,
		Usage:   m.usage,
		Model:   m.model,
	}, nil
}

func (m *mockLLMClient) StreamJSON(_ context.Context, _ claude.CompletionRequest) (<-chan claude.StreamEvent, *claude.StreamResult, error) {
	return nil, nil, errors.New("not implemented")
}

// mockClientCreator implements LLMClientCreator for testing.
type mockClientCreator struct {
	client       *mockLLMClient
	createdModel string
}

func (m *mockClientCreator) NewSchemaClient(model string) claude.Client {
	m.createdModel = model
	return m.client
}

// mockAgentLookup implements AgentLookup for testing.
type mockAgentLookup struct {
	agents map[string]*db.Agent
	err    error
}

func (m *mockAgentLookup) GetAgent(id string) (*db.Agent, error) {
	if m.err != nil {
		return nil, m.err
	}
	agent, ok := m.agents[id]
	if !ok {
		return nil, nil // matches ProjectDB behavior: not found returns nil, nil
	}
	return agent, nil
}

// mockCostRecorder implements CostRecorder for testing.
type mockCostRecorder struct {
	entries []db.CostEntry
}

func (m *mockCostRecorder) RecordCost(entry db.CostEntry) {
	m.entries = append(m.entries, entry)
}

// --- Helper functions ---

func testAgent(id, prompt, model string) *db.Agent {
	return &db.Agent{
		ID:          id,
		Name:        "Test Agent " + id,
		Description: "Test agent for " + id,
		Prompt:      prompt,
		Model:       model,
	}
}

func approvedResponse() string {
	data, _ := json.Marshal(GateAgentResponse{
		Status: "approved",
		Reason: "All checks passed",
	})
	return string(data)
}

func rejectedResponse(reason, retryFrom string, outputData map[string]any) string {
	data, _ := json.Marshal(GateAgentResponse{
		Status:    "rejected",
		Reason:    reason,
		RetryFrom: retryFrom,
		Data:      outputData,
	})
	return string(data)
}

func blockedResponse(reason string) string {
	data, _ := json.Marshal(GateAgentResponse{
		Status: "blocked",
		Reason: reason,
	})
	return string(data)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nil, nil))
}

// =============================================================================
// SC-1: AI gate evaluation - agent lookup and LLM call
// =============================================================================

func TestEvaluateAI_Approved(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{
		response: approvedResponse(),
		usage:    claude.TokenUsage{InputTokens: 100, OutputTokens: 50},
	}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"dep-checker": testAgent("dep-checker", "Check dependencies", "sonnet"),
		},
	}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID: "TASK-001",
		Phase:  "implement",
		AgentID: "dep-checker",
	}

	decision, err := eval.EvaluateWithOptions(context.Background(), gate, "phase output", opts)
	if err != nil {
		t.Fatalf("EvaluateWithOptions() error = %v", err)
	}
	if !decision.Approved {
		t.Error("expected decision to be approved")
	}
	if decision.Reason != "All checks passed" {
		t.Errorf("Reason = %q, want %q", decision.Reason, "All checks passed")
	}
}

func TestEvaluateAI_Rejected(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{
		response: rejectedResponse("Missing error handling", "implement", nil),
	}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"reviewer": testAgent("reviewer", "Review code", ""),
		},
	}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-001",
		Phase:   "implement",
		AgentID: "reviewer",
	}

	decision, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err != nil {
		t.Fatalf("EvaluateWithOptions() error = %v", err)
	}
	if decision.Approved {
		t.Error("expected decision to be rejected")
	}
	if decision.Reason != "Missing error handling" {
		t.Errorf("Reason = %q, want %q", decision.Reason, "Missing error handling")
	}
}

func TestEvaluateAI_Blocked(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{
		response: blockedResponse("Cannot evaluate without tests"),
	}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check things", ""),
		},
	}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-001",
		Phase:   "review",
		AgentID: "checker",
	}

	decision, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err != nil {
		t.Fatalf("EvaluateWithOptions() error = %v", err)
	}
	if decision.Approved {
		t.Error("expected decision to be not approved for 'blocked' status")
	}
}

func TestEvaluateAI_AgentNotFound(t *testing.T) {
	t.Parallel()

	creator := &mockClientCreator{client: &mockLLMClient{}}
	lookup := &mockAgentLookup{agents: map[string]*db.Agent{}} // empty registry

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-001",
		Phase:   "implement",
		AgentID: "nonexistent-agent",
	}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err == nil {
		t.Fatal("expected error for agent not found")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want to contain 'not found'", err.Error())
	}
}

func TestEvaluateAI_NoAgentConfigured(t *testing.T) {
	t.Parallel()

	creator := &mockClientCreator{client: &mockLLMClient{}}
	lookup := &mockAgentLookup{agents: map[string]*db.Agent{}}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-001",
		Phase:   "implement",
		AgentID: "", // empty agent ID
	}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err == nil {
		t.Fatal("expected error for empty agent ID")
	}
	if !strings.Contains(err.Error(), "no agent configured") {
		t.Errorf("error = %q, want to contain 'no agent configured'", err.Error())
	}
}

func TestEvaluateAI_NoDB(t *testing.T) {
	t.Parallel()

	// No AgentLookup dependency provided
	eval := New(
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-001",
		Phase:   "implement",
		AgentID: "some-agent",
	}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err == nil {
		t.Fatal("expected error when agent lookup is nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error = %q, want to contain 'required'", err.Error())
	}
}

// =============================================================================
// SC-2: Prompt building from GateInputConfig
// =============================================================================

func TestAIGatePromptBuilding_IncludesPhaseOutputs(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check stuff", ""),
		},
	}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-001",
		Phase:   "review",
		AgentID: "checker",
		InputConfig: &db.GateInputConfig{
			IncludePhaseOutput: []string{"spec", "tdd_write"},
		},
		PhaseOutputs: map[string]string{
			"spec":      "The spec content here",
			"tdd_write": "The test content here",
		},
	}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "current phase output", opts)
	if err != nil {
		t.Fatalf("EvaluateWithOptions() error = %v", err)
	}

	prompt := client.capturedPrompt
	if !strings.Contains(prompt, "The spec content here") {
		t.Error("prompt should contain spec phase output")
	}
	if !strings.Contains(prompt, "The test content here") {
		t.Error("prompt should contain tdd_write phase output")
	}
}

func TestAIGatePromptBuilding_IncludesTaskContext(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check stuff", ""),
		},
	}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:       "TASK-001",
		TaskTitle:    "Add user auth",
		Phase:        "review",
		AgentID:      "checker",
		TaskDesc:     "Implement JWT authentication",
		TaskCategory: "feature",
		TaskWeight:   "medium",
		InputConfig: &db.GateInputConfig{
			IncludeTask: true,
		},
	}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err != nil {
		t.Fatalf("EvaluateWithOptions() error = %v", err)
	}

	prompt := client.capturedPrompt
	if !strings.Contains(prompt, "Add user auth") {
		t.Error("prompt should contain task title")
	}
	if !strings.Contains(prompt, "Implement JWT authentication") {
		t.Error("prompt should contain task description")
	}
	if !strings.Contains(prompt, "feature") {
		t.Error("prompt should contain task category")
	}
	if !strings.Contains(prompt, "medium") {
		t.Error("prompt should contain task weight")
	}
}

func TestAIGatePromptBuilding_IncludesExtraVars(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check stuff", ""),
		},
	}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-001",
		Phase:   "review",
		AgentID: "checker",
		InputConfig: &db.GateInputConfig{
			ExtraVars: []string{"CUSTOM_CONTEXT=some value", "ANOTHER=data"},
		},
	}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err != nil {
		t.Fatalf("EvaluateWithOptions() error = %v", err)
	}

	prompt := client.capturedPrompt
	if !strings.Contains(prompt, "CUSTOM_CONTEXT") {
		t.Error("prompt should contain extra var key")
	}
	if !strings.Contains(prompt, "some value") {
		t.Error("prompt should contain extra var value")
	}
}

func TestAIGatePromptBuilding_MissingOutput(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check stuff", ""),
		},
	}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-001",
		Phase:   "review",
		AgentID: "checker",
		InputConfig: &db.GateInputConfig{
			IncludePhaseOutput: []string{"spec", "tdd_write"},
		},
		PhaseOutputs: map[string]string{
			"spec": "The spec content",
			// tdd_write missing
		},
	}

	// Should not error - missing outputs are noted but not fatal
	decision, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err != nil {
		t.Fatalf("expected no error when phase output is missing, got: %v", err)
	}
	if !decision.Approved {
		t.Error("decision should still be based on LLM response, not missing output")
	}

	prompt := client.capturedPrompt
	if !strings.Contains(prompt, "spec") {
		t.Error("prompt should still contain available spec output")
	}
	// Prompt should note that tdd_write is unavailable
	if !strings.Contains(prompt, "unavailable") && !strings.Contains(prompt, "not available") && !strings.Contains(prompt, "missing") {
		t.Error("prompt should note that tdd_write output is unavailable")
	}
}

func TestAIGatePromptBuilding_NilConfig(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check stuff", ""),
		},
	}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:      "TASK-001",
		Phase:       "review",
		AgentID:     "checker",
		InputConfig: nil, // no input config
	}

	// Should work with nil config - just no extra context in prompt
	decision, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err != nil {
		t.Fatalf("expected no error with nil InputConfig, got: %v", err)
	}
	if !decision.Approved {
		t.Error("decision should be approved based on LLM response")
	}
}

func TestAIGatePromptBuilding_EmptyOutputs(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check stuff", ""),
		},
	}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-001",
		Phase:   "review",
		AgentID: "checker",
		InputConfig: &db.GateInputConfig{
			IncludePhaseOutput: []string{"spec"},
		},
		PhaseOutputs: map[string]string{}, // empty map
	}

	// Should succeed - prompt built without output sections
	_, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err != nil {
		t.Fatalf("expected no error with empty phase outputs, got: %v", err)
	}
}

// =============================================================================
// SC-3: Response parsing - status mapping
// =============================================================================

func TestAIGateResponseParsing_StatusApproved(t *testing.T) {
	t.Parallel()

	resp := GateAgentResponse{
		Status: "approved",
		Reason: "Looks good",
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"a": testAgent("a", "p", ""),
		},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "a"}

	d, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !d.Approved {
		t.Error("status 'approved' should map to Approved=true")
	}
	if d.Reason != "Looks good" {
		t.Errorf("Reason = %q, want %q", d.Reason, "Looks good")
	}
}

func TestAIGateResponseParsing_StatusRejected(t *testing.T) {
	t.Parallel()

	resp := GateAgentResponse{
		Status:    "rejected",
		Reason:    "Bad code",
		RetryFrom: "implement",
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "a"}

	d, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Approved {
		t.Error("status 'rejected' should map to Approved=false")
	}
	if d.RetryPhase != "implement" {
		t.Errorf("RetryPhase = %q, want %q", d.RetryPhase, "implement")
	}
}

func TestAIGateResponseParsing_StatusBlocked(t *testing.T) {
	t.Parallel()

	resp := GateAgentResponse{
		Status: "blocked",
		Reason: "Needs prerequisite",
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "a"}

	d, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Approved {
		t.Error("status 'blocked' should map to Approved=false")
	}
}

func TestAIGateResponseParsing_UnknownStatus(t *testing.T) {
	t.Parallel()

	resp := GateAgentResponse{
		Status: "maybe",
		Reason: "Not sure",
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "a"}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err == nil {
		t.Fatal("expected error for unknown status value")
	}
	if !strings.Contains(err.Error(), "unknown status") {
		t.Errorf("error = %q, want to contain 'unknown status'", err.Error())
	}
}

func TestAIGateResponseParsing_LLMError(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{err: errors.New("API rate limit exceeded")}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "a"}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err == nil {
		t.Fatal("expected error when LLM call fails")
	}
	if !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("error should wrap original LLM error, got: %v", err)
	}
}

// =============================================================================
// SC-4: Model selection from agent config
// =============================================================================

func TestAIGateModelSelection_AgentModel(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"fast-checker": testAgent("fast-checker", "Quick check", "haiku"),
		},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "fast-checker"}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creator.createdModel != "haiku" {
		t.Errorf("created model = %q, want %q", creator.createdModel, "haiku")
	}
}

func TestAIGateModelSelection_EmptyModel(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check", ""), // empty model
		},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "checker"}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Empty model should use factory default (empty string passed to NewSchemaClient)
	if creator.createdModel != "" {
		t.Errorf("created model = %q, want empty (factory default)", creator.createdModel)
	}
}

// =============================================================================
// SC-5: Cost tracking
// =============================================================================

func TestAIGateCostTracking(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{
		response: approvedResponse(),
		usage: claude.TokenUsage{
			InputTokens:  500,
			OutputTokens: 100,
			TotalTokens:  600,
		},
		model: "claude-sonnet-4-20250514",
	}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check", "sonnet"),
		},
	}
	costRecorder := &mockCostRecorder{}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithCostRecorder(costRecorder),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "TASK-042",
		Phase:   "implement",
		AgentID: "checker",
	}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(costRecorder.entries) != 1 {
		t.Fatalf("expected 1 cost entry, got %d", len(costRecorder.entries))
	}

	entry := costRecorder.entries[0]
	if entry.TaskID != "TASK-042" {
		t.Errorf("TaskID = %q, want %q", entry.TaskID, "TASK-042")
	}
	if entry.Phase != "gate:implement" {
		t.Errorf("Phase = %q, want %q", entry.Phase, "gate:implement")
	}
	if entry.InputTokens != 500 {
		t.Errorf("InputTokens = %d, want %d", entry.InputTokens, 500)
	}
	if entry.OutputTokens != 100 {
		t.Errorf("OutputTokens = %d, want %d", entry.OutputTokens, 100)
	}
}

func TestAIGateCostTracking_NilCostRecorder(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check", ""),
		},
	}

	// No cost recorder provided - should not panic
	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "checker"}

	decision, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Approved {
		t.Error("should still return valid decision without cost recorder")
	}
}

// =============================================================================
// SC-6: Timeout and context cancellation
// =============================================================================

func TestAIGateTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := &mockLLMClient{err: context.Canceled}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{
			"checker": testAgent("checker", "Check", ""),
		},
	}
	costRecorder := &mockCostRecorder{}

	eval := New(
		WithAgentLookup(lookup),
		WithClientCreator(creator),
		WithCostRecorder(costRecorder),
		WithLogger(testLogger()),
	)

	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "checker"}

	_, err := eval.EvaluateWithOptions(ctx, gate, "out", opts)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should wrap context.Canceled, got: %v", err)
	}

	// No cost should be recorded on cancellation
	if len(costRecorder.entries) != 0 {
		t.Errorf("expected 0 cost entries on cancellation, got %d", len(costRecorder.entries))
	}
}

// =============================================================================
// SC-7: Output data for variable pipeline
// =============================================================================

func TestAIGateOutputData(t *testing.T) {
	t.Parallel()

	resp := GateAgentResponse{
		Status: "approved",
		Reason: "OK",
		Data: map[string]any{
			"issues": []any{"minor style"},
			"score":  float64(95),
		},
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "T",
		Phase:   "p",
		AgentID: "a",
		OutputConfig: &db.GateOutputConfig{
			VariableName: "gate_result",
		},
	}

	d, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if d.OutputData == nil {
		t.Fatal("OutputData should not be nil when agent returns data")
	}
	issues, ok := d.OutputData["issues"]
	if !ok {
		t.Error("OutputData should contain 'issues' key")
	}
	issueList, ok := issues.([]any)
	if !ok || len(issueList) != 1 {
		t.Errorf("issues = %v, want single-element list", issues)
	}

	if d.OutputVar != "gate_result" {
		t.Errorf("OutputVar = %q, want %q", d.OutputVar, "gate_result")
	}
}

func TestAIGateOutputData_NullData(t *testing.T) {
	t.Parallel()

	resp := GateAgentResponse{
		Status: "approved",
		Reason: "OK",
		Data:   nil, // null data
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "T",
		Phase:   "p",
		AgentID: "a",
		OutputConfig: &db.GateOutputConfig{
			VariableName: "gate_result",
		},
	}

	d, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.OutputData != nil {
		t.Errorf("OutputData should be nil when agent data is null, got %v", d.OutputData)
	}
}

func TestAIGateOutputData_NilConfig(t *testing.T) {
	t.Parallel()

	resp := GateAgentResponse{
		Status: "approved",
		Reason: "OK",
		Data:   map[string]any{"key": "val"},
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:       "T",
		Phase:        "p",
		AgentID:      "a",
		OutputConfig: nil, // no output config
	}

	d, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Data still available even without config - just no variable name
	if d.OutputData == nil {
		t.Error("OutputData should still carry agent data even without OutputConfig")
	}
	if d.OutputVar != "" {
		t.Errorf("OutputVar = %q, want empty when OutputConfig is nil", d.OutputVar)
	}
}

// =============================================================================
// SC-8: Retry phase resolution
// =============================================================================

func TestAIGateRetryFromConfig(t *testing.T) {
	t.Parallel()

	// Both config and LLM specify retry_from - config takes precedence
	resp := GateAgentResponse{
		Status:    "rejected",
		Reason:    "Failed",
		RetryFrom: "tdd_write", // LLM suggestion
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "T",
		Phase:   "review",
		AgentID: "a",
		OutputConfig: &db.GateOutputConfig{
			RetryFrom: "spec", // Config override
		},
	}

	d, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.RetryPhase != "spec" {
		t.Errorf("RetryPhase = %q, want %q (config should override LLM)", d.RetryPhase, "spec")
	}
}

func TestAIGateRetryFromLLM(t *testing.T) {
	t.Parallel()

	// Config doesn't specify retry_from - use LLM's suggestion
	resp := GateAgentResponse{
		Status:    "rejected",
		Reason:    "Bad tests",
		RetryFrom: "tdd_write",
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "T",
		Phase:   "review",
		AgentID: "a",
		OutputConfig: &db.GateOutputConfig{
			RetryFrom: "", // Empty config
		},
	}

	d, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.RetryPhase != "tdd_write" {
		t.Errorf("RetryPhase = %q, want %q (LLM value when config is empty)", d.RetryPhase, "tdd_write")
	}
}

func TestAIGateRetryFromConfig_NoneSet(t *testing.T) {
	t.Parallel()

	// Neither config nor LLM specify retry_from
	resp := GateAgentResponse{
		Status: "rejected",
		Reason: "Not good enough",
	}
	data, _ := json.Marshal(resp)
	client := &mockLLMClient{response: string(data)}
	creator := &mockClientCreator{client: client}
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"a": testAgent("a", "p", "")},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{
		TaskID:  "T",
		Phase:   "review",
		AgentID: "a",
	}

	d, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.RetryPhase != "" {
		t.Errorf("RetryPhase = %q, want empty when neither config nor LLM set it", d.RetryPhase)
	}
}

// =============================================================================
// Backward compatibility: auto and human gates still work
// =============================================================================

func TestEvaluateAuto_BackwardCompat(t *testing.T) {
	t.Parallel()

	// New() with zero options should still work for auto gates
	eval := New()

	gate := &Gate{
		Type:     GateAuto,
		Criteria: []string{"has_output"},
	}

	decision, err := eval.EvaluateWithOptions(context.Background(), gate, "some output", nil)
	if err != nil {
		t.Fatalf("auto gate should work without AI deps: %v", err)
	}
	if !decision.Approved {
		t.Error("auto gate should approve with output present")
	}
}

func TestEvaluateAuto_NoCriteria_BackwardCompat(t *testing.T) {
	t.Parallel()

	eval := New()

	gate := &Gate{Type: GateAuto}

	decision, err := eval.EvaluateWithOptions(context.Background(), gate, "", nil)
	if err != nil {
		t.Fatalf("auto gate with no criteria should work: %v", err)
	}
	if !decision.Approved {
		t.Error("auto gate with no criteria should auto-approve")
	}
}

// =============================================================================
// Edge case: empty agent prompt
// =============================================================================

func TestEvaluateAI_EmptyAgentPrompt(t *testing.T) {
	t.Parallel()

	client := &mockLLMClient{response: approvedResponse()}
	creator := &mockClientCreator{client: client}
	agent := testAgent("checker", "", "") // empty prompt
	agent.Description = "A gate checker agent"
	lookup := &mockAgentLookup{
		agents: map[string]*db.Agent{"checker": agent},
	}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "checker"}

	// Should use agent description as context when prompt is empty
	_, err := eval.EvaluateWithOptions(context.Background(), gate, "output", opts)
	if err != nil {
		t.Fatalf("should handle empty agent prompt: %v", err)
	}

	// Verify the description was used somewhere in the prompt
	if !strings.Contains(client.capturedPrompt, "A gate checker agent") {
		t.Error("prompt should include agent description when prompt field is empty")
	}
}

// =============================================================================
// Edge case: agent lookup DB error
// =============================================================================

func TestEvaluateAI_AgentLookupError(t *testing.T) {
	t.Parallel()

	creator := &mockClientCreator{client: &mockLLMClient{}}
	lookup := &mockAgentLookup{err: errors.New("database connection lost")}

	eval := New(WithAgentLookup(lookup), WithClientCreator(creator), WithLogger(testLogger()))
	gate := &Gate{Type: GateAI}
	opts := &EvaluateOptions{TaskID: "T", Phase: "p", AgentID: "checker"}

	_, err := eval.EvaluateWithOptions(context.Background(), gate, "out", opts)
	if err == nil {
		t.Fatal("expected error when agent lookup fails")
	}
	if !strings.Contains(err.Error(), "database connection lost") {
		t.Errorf("error should wrap DB error, got: %v", err)
	}
}
