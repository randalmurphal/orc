// Tests for TASK-007: ScriptPhaseExecutor that runs shell commands with
// timeout and success pattern detection.
//
// Coverage mapping:
//   SC-1:  TestScript_ExitZeroCompleted
//   SC-2:  TestScript_NonZeroExitError
//   SC-3:  TestScript_TimeoutError
//   SC-4:  TestScript_SuccessPatternMatch, TestScript_SuccessPatternMismatch, TestScript_InvalidRegex
//   SC-9:  TestScript_VariableInterpolation, TestScript_MultipleVars, TestScript_UnknownVar
//   SC-10: TestScript_OutputVar, TestScript_OutputVarEmptyNoError, TestScript_NilRCtx, TestScript_NilVars
//   SC-12: TestScript_ZeroCostAndDuration
//
// Edge cases:
//   TestScript_StderrOnly, TestScript_NoOutput, TestScript_ShellMetachars
//
// Failure modes:
//   TestScript_CommandNotFound, TestScript_EmptyCommand
package executor

import (
	"context"
	"strings"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/variable"
)

// =============================================================================
// SC-1: ScriptPhaseExecutor runs command and returns completed with stdout
// =============================================================================

func TestScript_ExitZeroCompleted(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command: `echo "hello"`,
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// stdout should be captured as Content (trimmed)
	if strings.TrimSpace(result.Content) != "hello" {
		t.Errorf("content = %q, want %q", result.Content, "hello")
	}
}

// =============================================================================
// SC-2: Non-zero exit code returns error
// =============================================================================

func TestScript_NonZeroExitError(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "failing-build"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command: "exit 1",
	}

	_, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
}

// =============================================================================
// SC-3: Timeout enforcement — command exceeding duration is killed
// =============================================================================

func TestScript_TimeoutError(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "slow-task"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command: "sleep 60",
		Timeout: 100 * time.Millisecond,
	}

	_, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error for timeout")
	}

	if !containsSubstring(err.Error(), "timeout") &&
		!containsSubstring(err.Error(), "context deadline exceeded") &&
		!containsSubstring(err.Error(), "killed") {
		t.Errorf("error should mention timeout, got: %q", err.Error())
	}
}

// =============================================================================
// SC-4: success_pattern regex matching
// =============================================================================

func TestScript_SuccessPatternMatch(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "migration"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command:        `echo "3 migrations applied"`,
		SuccessPattern: `migrations applied`,
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED when pattern matches", result.Status)
	}
}

func TestScript_SuccessPatternMismatch(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "migration"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command:        `echo "no changes"`,
		SuccessPattern: `migrations applied`,
	}

	_, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error when success_pattern doesn't match stdout")
	}

	if !containsSubstring(err.Error(), "pattern") {
		t.Errorf("error should mention pattern mismatch, got: %q", err.Error())
	}
}

func TestScript_InvalidRegex(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "bad-regex"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command:        `echo "hello"`,
		SuccessPattern: `[invalid(regex`,
	}

	_, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error for invalid regex in success_pattern")
	}
}

// =============================================================================
// SC-9: Variable interpolation in config fields
// =============================================================================

func TestScript_VariableInterpolation(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	vars := variable.VariableSet{
		"WORKTREE_PATH": "/tmp/test-work",
	}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          vars,
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	// Command uses a variable — after interpolation, echo should output
	// the resolved path.
	cfg := ScriptPhaseConfig{
		Command: `echo "{{WORKTREE_PATH}}"`,
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsSubstring(result.Content, "/tmp/test-work") {
		t.Errorf("content = %q, expected resolved variable /tmp/test-work", result.Content)
	}
}

func TestScript_MultipleVars(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	vars := variable.VariableSet{
		"TASK_ID":    "TASK-007",
		"TASK_TITLE": "Script executor",
	}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          vars,
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command: `echo "{{TASK_ID}} {{TASK_TITLE}}"`,
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsSubstring(result.Content, "TASK-007") ||
		!containsSubstring(result.Content, "Script executor") {
		t.Errorf("content = %q, expected both vars resolved", result.Content)
	}
}

func TestScript_UnknownVar(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	// Unknown variable should resolve to empty string per RenderTemplate behavior
	cfg := ScriptPhaseConfig{
		Command: `echo "prefix-{{UNKNOWN_VAR}}-suffix"`,
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should see "prefix--suffix" (empty string for unknown var)
	if !containsSubstring(result.Content, "prefix--suffix") {
		t.Errorf("content = %q, expected unknown var resolved to empty", result.Content)
	}
}

// =============================================================================
// SC-10: Output variable storage
// =============================================================================

func TestScript_OutputVar(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "deploy"},
		Vars:          vars,
		RCtx:          rctx,
	}

	cfg := ScriptPhaseConfig{
		Command:   `echo "v2.1.0 deployed to staging"`,
		OutputVar: "DEPLOY_OUTPUT",
	}

	_, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify stored in params.Vars
	if vars["DEPLOY_OUTPUT"] == "" {
		t.Error("expected DEPLOY_OUTPUT in params.Vars")
	}
	if !containsSubstring(vars["DEPLOY_OUTPUT"], "deployed to staging") {
		t.Errorf("DEPLOY_OUTPUT in Vars = %q, expected deploy output", vars["DEPLOY_OUTPUT"])
	}

	// Verify stored in rctx.PhaseOutputVars for persistence
	if rctx.PhaseOutputVars["DEPLOY_OUTPUT"] == "" {
		t.Error("expected DEPLOY_OUTPUT in rctx.PhaseOutputVars")
	}
}

func TestScript_OutputVarEmptyNoError(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          vars,
		RCtx:          rctx,
	}

	// No output_var configured — should not store anything and not error
	cfg := ScriptPhaseConfig{
		Command: `echo "hello"`,
	}

	_, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No variable should be stored
	if len(vars) > 0 {
		t.Errorf("expected no variables stored when output_var is empty, got %v", vars)
	}
}

func TestScript_NilRCtx(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	vars := variable.VariableSet{}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          vars,
		RCtx:          nil, // nil rctx
	}

	cfg := ScriptPhaseConfig{
		Command:   `echo "output"`,
		OutputVar: "BUILD_OUTPUT",
	}

	// Should not panic — stores to Vars only
	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	if vars["BUILD_OUTPUT"] == "" {
		t.Error("expected BUILD_OUTPUT in params.Vars even with nil RCtx")
	}
}

func TestScript_NilVars(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          nil, // nil vars
		RCtx:          nil,
	}

	cfg := ScriptPhaseConfig{
		Command:   `echo "output"`,
		OutputVar: "BUILD_OUTPUT",
	}

	// Should not panic
	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}
}

// =============================================================================
// SC-12: Zero LLM cost and positive duration
// =============================================================================

func TestScript_ZeroCostAndDuration(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command: `echo "hello"`,
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.InputTokens != 0 {
		t.Errorf("InputTokens = %d, want 0", result.InputTokens)
	}
	if result.OutputTokens != 0 {
		t.Errorf("OutputTokens = %d, want 0", result.OutputTokens)
	}
	if result.CostUSD != 0 {
		t.Errorf("CostUSD = %f, want 0", result.CostUSD)
	}
	if result.DurationMS <= 0 {
		t.Errorf("DurationMS = %d, want > 0", result.DurationMS)
	}
}

// =============================================================================
// Failure modes
// =============================================================================

func TestScript_CommandNotFound(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command: "nonexistent_command_xyz_12345",
	}

	_, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error for command not found")
	}
}

func TestScript_EmptyCommand(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command: "",
	}

	_, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

// =============================================================================
// Edge cases
// =============================================================================

func TestScript_StderrOnly(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	// Writes to stderr only, exits 0
	cfg := ScriptPhaseConfig{
		Command: `echo "error output" >&2`,
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Content should be empty (only stdout is captured)
	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED (exit code 0)", result.Status)
	}
}

func TestScript_NoOutput(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command: "true", // Produces no output, exits 0
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}
}

func TestScript_ShellMetachars(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          variable.VariableSet{},
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	// Uses pipe and shell metacharacters
	cfg := ScriptPhaseConfig{
		Command: `echo "line1" && echo "line2" | tr 'a-z' 'A-Z'`,
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should see output from both commands (via shell)
	if !containsSubstring(result.Content, "line1") {
		t.Errorf("content = %q, expected 'line1' from first command", result.Content)
	}
	if !containsSubstring(result.Content, "LINE2") {
		t.Errorf("content = %q, expected 'LINE2' from piped uppercase", result.Content)
	}
}

// =============================================================================
// SC-4 + SC-10: Pattern match + output_var (BDD-3)
// =============================================================================

func TestScript_SuccessPatternWithOutputVar(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	vars := variable.VariableSet{}
	rctx := &variable.ResolutionContext{
		PhaseOutputVars: make(map[string]string),
	}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "deploy"},
		Vars:          vars,
		RCtx:          rctx,
	}

	cfg := ScriptPhaseConfig{
		Command:        `echo "v2.1.0 deployed to staging"`,
		SuccessPattern: `deployed`,
		OutputVar:      "DEPLOY_OUTPUT",
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
		t.Errorf("status = %q, want COMPLETED", result.Status)
	}

	// Output var should contain full stdout
	if !containsSubstring(vars["DEPLOY_OUTPUT"], "v2.1.0 deployed to staging") {
		t.Errorf("DEPLOY_OUTPUT = %q, expected full stdout", vars["DEPLOY_OUTPUT"])
	}
	if !containsSubstring(rctx.PhaseOutputVars["DEPLOY_OUTPUT"], "v2.1.0 deployed to staging") {
		t.Errorf("rctx DEPLOY_OUTPUT = %q, expected full stdout", rctx.PhaseOutputVars["DEPLOY_OUTPUT"])
	}
}

// =============================================================================
// Name() returns executor type name
// =============================================================================

func TestScript_Name(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()
	if executor.Name() != "script" {
		t.Errorf("Name() = %q, want %q", executor.Name(), "script")
	}
}

// =============================================================================
// SC-9: Variable interpolation in workdir and success_pattern
// =============================================================================

func TestScript_VariableInterpolationInWorkdir(t *testing.T) {
	t.Parallel()

	executor := NewScriptPhaseExecutor()

	tmpDir := t.TempDir()
	vars := variable.VariableSet{
		"WORKTREE_PATH": tmpDir,
	}
	params := PhaseTypeParams{
		PhaseTemplate: &db.PhaseTemplate{ID: "build"},
		Vars:          vars,
		RCtx: &variable.ResolutionContext{
			PhaseOutputVars: make(map[string]string),
		},
	}

	cfg := ScriptPhaseConfig{
		Command: "pwd",
		Workdir: "{{WORKTREE_PATH}}",
	}

	result, err := executor.ExecuteScript(context.Background(), params, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsSubstring(result.Content, tmpDir) {
		t.Errorf("content = %q, expected workdir to be resolved to %s", result.Content, tmpDir)
	}
}
