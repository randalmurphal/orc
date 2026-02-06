// Tests for TASK-004: KnowledgePhaseExecutor that queries the knowledge graph
// and stores results as workflow variables.
//
// Coverage mapping:
//   SC-5:  TestKnowledgeExecutor_QueryAndStore
//   SC-6:  TestKnowledgeExecutor_TokenBudget
//   SC-7:  TestKnowledgeExecutor_ResultShape
//   SC-8:  TestKnowledgeExecutor_FallbackSkip
//   SC-9:  TestKnowledgeExecutor_FallbackError
//
// Edge cases:
//   TestKnowledgeExecutor_EmptyQueryUsesTaskDescription
//   TestKnowledgeExecutor_InvalidPresetErrors
//   TestKnowledgeExecutor_EmptyResultCompletesSuccessfully
//   TestKnowledgeExecutor_QueryErrorPropagates
//
// Failure modes:
//   TestKnowledgeExecutor_FallbackError - unavailable + no skip = error
//   TestKnowledgeExecutor_QueryErrorPropagates - query failure = phase error
//   TestKnowledgeExecutor_InvalidPresetErrors - unknown preset = error
package executor

import (
	"context"
	"errors"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/knowledge"
	"github.com/randalmurphal/orc/internal/knowledge/retrieve"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// SC-5: KnowledgePhaseExecutor calls knowledge.Service.Query() with configured
// query/preset and stores result as configured output_var.
// =============================================================================

func TestKnowledgeExecutor_QueryAndStore(t *testing.T) {
	t.Parallel()

	mockSvc := &mockKnowledgeService{
		available: true,
		queryResult: &retrieve.PipelineResult{
			Documents: []retrieve.ScoredDocument{
				{Document: retrieve.Document{
					ID:      "doc-1",
					Content: "Pattern: always wrap errors with context",
				}},
				{Document: retrieve.Document{
					ID:      "doc-2",
					Content: "Decision: use bcrypt for passwords",
				}},
			},
			TokensUsed: 150,
		},
	}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-001", "Implement login endpoint")
	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: vars,
		RCtx: rctx,
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:     "login endpoint patterns and decisions",
			Preset:    "standard",
			OutputVar: "KNOWLEDGE_CONTEXT",
		},
	}

	result, err := executor.ExecutePhase(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify query was called with correct parameters
	if mockSvc.lastQuery != "login endpoint patterns and decisions" {
		t.Errorf("query = %q, want %q", mockSvc.lastQuery, "login endpoint patterns and decisions")
	}
	if mockSvc.lastOpts.Preset != "standard" {
		t.Errorf("preset = %q, want %q", mockSvc.lastOpts.Preset, "standard")
	}

	// Verify result has completed status
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// Verify content contains rendered knowledge
	if result.Content == "" {
		t.Error("expected non-empty content with rendered knowledge")
	}

	// Verify output variable was stored in vars
	if vars["KNOWLEDGE_CONTEXT"] == "" {
		t.Error("expected KNOWLEDGE_CONTEXT variable to be set")
	}

	// Verify output variable was stored in rctx for persistence across ResolveAll()
	if rctx.PhaseOutputVars["KNOWLEDGE_CONTEXT"] == "" {
		t.Error("expected KNOWLEDGE_CONTEXT in rctx.PhaseOutputVars for persistence")
	}
}

// =============================================================================
// SC-6: KnowledgePhaseExecutor respects MaxTokens from phase configuration
// =============================================================================

func TestKnowledgeExecutor_TokenBudget(t *testing.T) {
	t.Parallel()

	mockSvc := &mockKnowledgeService{
		available:   true,
		queryResult: &retrieve.PipelineResult{},
	}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-002", "Test token budget")
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:     "test query",
			Preset:    "fast",
			OutputVar: "KNOWLEDGE_CONTEXT",
			MaxTokens: 2000,
		},
	}

	_, err := executor.ExecutePhase(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify token budget was passed to Query()
	if mockSvc.lastOpts.MaxTokens != 2000 {
		t.Errorf("MaxTokens = %d, want 2000", mockSvc.lastOpts.MaxTokens)
	}
}

// TestKnowledgeExecutor_TokenBudgetDefault verifies a reasonable default is used
// when no token budget is configured.
func TestKnowledgeExecutor_TokenBudgetDefault(t *testing.T) {
	t.Parallel()

	mockSvc := &mockKnowledgeService{
		available:   true,
		queryResult: &retrieve.PipelineResult{},
	}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-003", "Test default token budget")
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:     "test query",
			OutputVar: "KNOWLEDGE_CONTEXT",
			// MaxTokens not set → should use reasonable default
		},
	}

	_, err := executor.ExecutePhase(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify a reasonable default was applied (not unlimited/zero)
	if mockSvc.lastOpts.MaxTokens == 0 {
		t.Error("expected non-zero default MaxTokens when not configured")
	}
}

// =============================================================================
// SC-7: KnowledgePhaseExecutor produces PhaseResult with zero cost/tokens
// =============================================================================

func TestKnowledgeExecutor_ResultShape(t *testing.T) {
	t.Parallel()

	mockSvc := &mockKnowledgeService{
		available: true,
		queryResult: &retrieve.PipelineResult{
			Documents: []retrieve.ScoredDocument{
				{Document: retrieve.Document{Content: "some knowledge"}},
			},
		},
	}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-004", "Test result shape")
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:     "test query",
			OutputVar: "KNOWLEDGE_CONTEXT",
		},
	}

	result, err := executor.ExecutePhase(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Zero cost - no LLM call
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0 (no LLM call)", result.CostUSD)
	}
	if result.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0 (no LLM call)", result.InputTokens)
	}
	if result.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0 (no LLM call)", result.OutputTokens)
	}

	// Completed status
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// Content contains rendered markdown
	if result.Content == "" {
		t.Error("expected non-empty content with rendered knowledge context")
	}
}

// TestKnowledgeExecutor_EmptyResultCompletesSuccessfully verifies that empty
// query results produce completed status with empty content (not error).
func TestKnowledgeExecutor_EmptyResultCompletesSuccessfully(t *testing.T) {
	t.Parallel()

	mockSvc := &mockKnowledgeService{
		available:   true,
		queryResult: &retrieve.PipelineResult{}, // Empty results
	}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-005", "Test empty result")
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:     "query with no results",
			OutputVar: "KNOWLEDGE_CONTEXT",
		},
	}

	result, err := executor.ExecutePhase(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should complete successfully even with empty results
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED (empty result is OK)", result.Status)
	}
}

// =============================================================================
// SC-8: fallback="skip" → SKIPPED status, no error
// =============================================================================

func TestKnowledgeExecutor_FallbackSkip(t *testing.T) {
	t.Parallel()

	// Service is unavailable
	mockSvc := &mockKnowledgeService{available: false}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-006", "Test fallback skip")
	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: vars,
		RCtx: rctx,
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:    "test query",
			OutputVar: "KNOWLEDGE_CONTEXT",
			Fallback: "skip",
		},
	}

	result, err := executor.ExecutePhase(context.Background(), params)
	if err != nil {
		t.Fatalf("fallback=skip should not return error, got: %v", err)
	}

	// Should be SKIPPED, not COMPLETED or FAILED
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String() {
		t.Errorf("status = %q, want SKIPPED", result.Status)
	}

	// Content should be empty
	if result.Content != "" {
		t.Errorf("content should be empty when skipped, got: %q", result.Content)
	}

	// Output variable should be set to empty string
	if _, exists := vars["KNOWLEDGE_CONTEXT"]; !exists {
		t.Error("expected KNOWLEDGE_CONTEXT variable to be set (empty string)")
	}
}

// TestKnowledgeExecutor_FallbackSkipOnQueryError verifies that fallback=skip
// also handles query execution errors gracefully.
func TestKnowledgeExecutor_FallbackSkipOnQueryError(t *testing.T) {
	t.Parallel()

	mockSvc := &mockKnowledgeService{
		available: true,
		queryErr:  errors.New("neo4j connection reset"),
	}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-007", "Test fallback skip on error")
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:    "test query",
			OutputVar: "KNOWLEDGE_CONTEXT",
			Fallback: "skip",
		},
	}

	result, err := executor.ExecutePhase(context.Background(), params)
	if err != nil {
		t.Fatalf("fallback=skip should not return error even on query failure, got: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String() {
		t.Errorf("status = %q, want SKIPPED", result.Status)
	}
}

// =============================================================================
// SC-9: No fallback (or fallback != "skip") + unavailable → error
// =============================================================================

func TestKnowledgeExecutor_FallbackError(t *testing.T) {
	t.Parallel()

	// Service is unavailable
	mockSvc := &mockKnowledgeService{available: false}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-008", "Test fallback error")
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:    "test query",
			OutputVar: "KNOWLEDGE_CONTEXT",
			// No fallback set → should error
		},
	}

	_, err := executor.ExecutePhase(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when knowledge service is unavailable and no fallback=skip")
	}

	// Error message should mention "knowledge" and reason
	errStr := err.Error()
	if !containsSubstring(errStr, "knowledge") {
		t.Errorf("error should mention 'knowledge', got: %q", errStr)
	}
}

func TestKnowledgeExecutor_FallbackErrorExplicit(t *testing.T) {
	t.Parallel()

	// Service is unavailable with explicit fallback="error"
	mockSvc := &mockKnowledgeService{available: false}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-009", "Test explicit error fallback")
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:    "test query",
			OutputVar: "KNOWLEDGE_CONTEXT",
			Fallback: "error",
		},
	}

	_, err := executor.ExecutePhase(context.Background(), params)
	if err == nil {
		t.Fatal("expected error when fallback=error and service unavailable")
	}
}

// =============================================================================
// Edge case: Empty query uses task description as fallback
// =============================================================================

func TestKnowledgeExecutor_EmptyQueryUsesTaskDescription(t *testing.T) {
	t.Parallel()

	mockSvc := &mockKnowledgeService{
		available:   true,
		queryResult: &retrieve.PipelineResult{},
	}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-010", "Implement OAuth2 login")
	task.SetDescriptionProto(tsk, "Implement OAuth2 login with Google provider")

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:    "", // Empty → should use task description
			OutputVar: "KNOWLEDGE_CONTEXT",
		},
	}

	_, err := executor.ExecutePhase(context.Background(), params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify it used the task description as the query
	if mockSvc.lastQuery == "" {
		t.Error("expected non-empty query (should have used task description)")
	}
}

// =============================================================================
// Edge case: Unknown preset → error (not fallback to default)
// =============================================================================

func TestKnowledgeExecutor_InvalidPresetErrors(t *testing.T) {
	t.Parallel()

	mockSvc := &mockKnowledgeService{
		available: true,
		queryErr:  errors.New("unknown preset: \"nonexistent_preset\""),
	}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-011", "Test invalid preset")
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:    "test query",
			Preset:   "nonexistent_preset",
			OutputVar: "KNOWLEDGE_CONTEXT",
		},
	}

	_, err := executor.ExecutePhase(context.Background(), params)
	if err == nil {
		t.Fatal("expected error for unknown preset (not silent fallback)")
	}
}

// =============================================================================
// Error path: Query error propagates as phase error (not swallowed)
// =============================================================================

func TestKnowledgeExecutor_QueryErrorPropagates(t *testing.T) {
	t.Parallel()

	mockSvc := &mockKnowledgeService{
		available: true,
		queryErr:  errors.New("connection refused"),
	}

	executor := NewKnowledgePhaseExecutor(mockSvc)

	tsk := task.NewProtoTask("TASK-012", "Test query error")
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{
			ID:            "gather-context",
			OutputVarName: "KNOWLEDGE_CONTEXT",
		},
		Task: tsk,
		Vars: variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
		KnowledgeConfig: &KnowledgePhaseConfig{
			Query:    "test query",
			OutputVar: "KNOWLEDGE_CONTEXT",
			// No fallback → error should propagate
		},
	}

	_, err := executor.ExecutePhase(context.Background(), params)
	if err == nil {
		t.Fatal("expected error to propagate from Query() failure")
	}
}

// =============================================================================
// Helper: mock knowledge service for testing
// =============================================================================

// mockKnowledgeService implements the interface that KnowledgePhaseExecutor
// needs to call Query() and IsAvailable().
type mockKnowledgeService struct {
	available   bool
	queryResult *retrieve.PipelineResult
	queryErr    error
	lastQuery   string
	lastOpts    retrieve.QueryOpts
}

func (m *mockKnowledgeService) IsAvailable() bool {
	return m.available
}

func (m *mockKnowledgeService) Query(ctx context.Context, query string, opts retrieve.QueryOpts) (*retrieve.PipelineResult, error) {
	m.lastQuery = query
	m.lastOpts = opts
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return m.queryResult, nil
}

// Suppress unused import warnings — knowledge package is needed for type references.
var _ knowledge.ServiceConfig
