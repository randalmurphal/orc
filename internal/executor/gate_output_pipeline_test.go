package executor

import (
	"encoding/json"
	"strings"
	"testing"
)

// =============================================================================
// SC-1: GateEvaluationResult propagates OutputData and OutputVar from Decision
// =============================================================================

func TestGateEvaluationResult_HasOutputFields(t *testing.T) {
	t.Parallel()

	// GateEvaluationResult must carry OutputData and OutputVar from the gate Decision.
	result := &GateEvaluationResult{
		Approved: true,
		Reason:   "approved by AI gate",
		OutputData: map[string]any{
			"issues": []any{"XSS", "CSRF"},
			"score":  float64(85),
		},
		OutputVar: "SECURITY_RESULT",
	}

	if result.OutputData == nil {
		t.Fatal("OutputData must be set on GateEvaluationResult")
	}
	if result.OutputVar != "SECURITY_RESULT" {
		t.Errorf("OutputVar = %q, want %q", result.OutputVar, "SECURITY_RESULT")
	}

	// Verify OutputData is a proper map with expected keys
	issues, ok := result.OutputData["issues"]
	if !ok {
		t.Fatal("OutputData missing 'issues' key")
	}
	issuesList, ok := issues.([]any)
	if !ok || len(issuesList) != 2 {
		t.Errorf("OutputData['issues'] = %v, want 2-element list", issues)
	}
}

func TestGateEvaluationResult_OutputFieldsZeroValue(t *testing.T) {
	t.Parallel()

	// When no output is set, fields should be zero-valued (backward compatible).
	result := &GateEvaluationResult{
		Approved: true,
		Reason:   "auto-approved",
	}

	if result.OutputData != nil {
		t.Errorf("OutputData should be nil by default, got %v", result.OutputData)
	}
	if result.OutputVar != "" {
		t.Errorf("OutputVar should be empty by default, got %q", result.OutputVar)
	}
}

// =============================================================================
// SC-2: After gate approval with output, executor stores to vars
// =============================================================================

func TestGateOutputApplied_ApprovedWithData(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		"EXISTING_VAR": "preserved",
	}

	gateResult := &GateEvaluationResult{
		Approved: true,
		Reason:   "approved",
		OutputData: map[string]any{
			"findings": []any{"item1", "item2"},
		},
		OutputVar: "REVIEW_FINDINGS",
	}

	applyGateOutputToVars(vars, gateResult)

	// Variable should be set with JSON-serialized data
	val, ok := vars["REVIEW_FINDINGS"]
	if !ok {
		t.Fatal("REVIEW_FINDINGS not set in vars after gate approval with output")
	}

	// Verify it's valid JSON containing the expected data
	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("gate output variable is not valid JSON: %v", err)
	}
	findings, ok := parsed["findings"]
	if !ok {
		t.Fatal("parsed output missing 'findings' key")
	}
	findingsList, ok := findings.([]any)
	if !ok || len(findingsList) != 2 {
		t.Errorf("findings = %v, want 2-element list", findings)
	}

	// Existing variable must be preserved
	if vars["EXISTING_VAR"] != "preserved" {
		t.Errorf("EXISTING_VAR = %q, want %q", vars["EXISTING_VAR"], "preserved")
	}
}

func TestGateOutputNoConfig_EmptyOutputVar(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	gateResult := &GateEvaluationResult{
		Approved: true,
		Reason:   "approved",
		OutputData: map[string]any{
			"data": "some data",
		},
		OutputVar: "", // No variable name configured
	}

	applyGateOutputToVars(vars, gateResult)

	// No variable should be set when OutputVar is empty
	if len(vars) != 0 {
		t.Errorf("vars should be empty when OutputVar is empty, got %v", vars)
	}
}

func TestGateOutputNoConfig_NilOutputData(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	gateResult := &GateEvaluationResult{
		Approved:   true,
		Reason:     "approved",
		OutputData: nil, // No output data
		OutputVar:  "SOME_VAR",
	}

	applyGateOutputToVars(vars, gateResult)

	// No variable should be set when OutputData is nil
	if _, ok := vars["SOME_VAR"]; ok {
		t.Error("variable should not be set when OutputData is nil")
	}
}

func TestGateOutputNoConfig_NilGateResult(t *testing.T) {
	t.Parallel()

	vars := map[string]string{"EXISTING": "val"}

	// applyGateOutputToVars must handle nil result safely
	applyGateOutputToVars(vars, nil)

	// Existing vars preserved, no panic
	if vars["EXISTING"] != "val" {
		t.Errorf("existing var modified after nil gate result")
	}
}

// =============================================================================
// SC-3: Stored gate output resolvable in subsequent phase prompts via {{VAR}}
// =============================================================================

func TestGateOutputInNextPhase_VariableResolvable(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	// Simulate gate storing output
	gateResult := &GateEvaluationResult{
		Approved: true,
		OutputData: map[string]any{
			"security_score": float64(92),
			"issues":         []any{},
		},
		OutputVar: "SECURITY_RESULT",
	}

	applyGateOutputToVars(vars, gateResult)

	// The variable should now be in the vars map, ready for template rendering
	val, ok := vars["SECURITY_RESULT"]
	if !ok {
		t.Fatal("SECURITY_RESULT not found in vars after gate output applied")
	}

	// Verify the value is non-empty and contains expected content
	if val == "" {
		t.Fatal("SECURITY_RESULT is empty")
	}
	if !strings.Contains(val, "security_score") {
		t.Errorf("SECURITY_RESULT = %q, expected to contain 'security_score'", val)
	}
}

// =============================================================================
// SC-4: Gate rejection context appended to RETRY_CONTEXT
// =============================================================================

func TestGateRetryContext_IncludesGateContext(t *testing.T) {
	t.Parallel()

	gateContext := "Security issues found: XSS vulnerability in login form, CSRF missing on /api/update"

	retryCtx := BuildRetryContext("review", "gate rejected", "phase output here", 1, "")

	// The existing BuildRetryContext should work as before
	if !strings.Contains(retryCtx, "review") {
		t.Error("retry context missing phase name")
	}

	// Now test the extended version with gate context
	retryCtxWithGate := BuildRetryContextWithGateAnalysis(
		"review", "gate rejected", "phase output here", 1, "", gateContext,
	)

	// Must include the gate analysis section
	if !strings.Contains(retryCtxWithGate, gateContext) {
		t.Error("retry context missing gate analysis context")
	}
	if !strings.Contains(retryCtxWithGate, "Gate Analysis") {
		t.Error("retry context missing 'Gate Analysis' section header")
	}

	// Must still include standard retry info
	if !strings.Contains(retryCtxWithGate, "review") {
		t.Error("retry context with gate analysis missing phase name")
	}
	if !strings.Contains(retryCtxWithGate, "gate rejected") {
		t.Error("retry context with gate analysis missing reason")
	}
}

func TestGateRetryContext_EmptyGateContext(t *testing.T) {
	t.Parallel()

	// When gate context is empty, should behave like standard BuildRetryContext
	retryCtxWithGate := BuildRetryContextWithGateAnalysis(
		"review", "gate rejected", "output", 1, "", "",
	)
	retryCtxStandard := BuildRetryContext("review", "gate rejected", "output", 1, "")

	// With empty gate context, the result should match standard (no extra section)
	if strings.Contains(retryCtxWithGate, "Gate Analysis") {
		t.Error("empty gate context should not add Gate Analysis section")
	}
	if retryCtxWithGate != retryCtxStandard {
		t.Errorf("empty gate context result differs from standard:\ngot:  %q\nwant: %q",
			retryCtxWithGate, retryCtxStandard)
	}
}

// =============================================================================
// SC-5: Gate output variables available even on rejection
// =============================================================================

func TestGateOutputOnRejection_VariableStored(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	gateResult := &GateEvaluationResult{
		Approved: false, // Rejected
		Reason:   "security issues found",
		OutputData: map[string]any{
			"paths": []any{"/api/login", "/api/admin"},
		},
		OutputVar:  "GATE_RESULT",
		RetryPhase: "implement",
	}

	applyGateOutputToVars(vars, gateResult)

	// Variable must be set even when gate rejects
	val, ok := vars["GATE_RESULT"]
	if !ok {
		t.Fatal("GATE_RESULT not set in vars after gate rejection")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("gate output on rejection is not valid JSON: %v", err)
	}
	paths, ok := parsed["paths"]
	if !ok {
		t.Fatal("parsed rejection output missing 'paths' key")
	}
	pathsList, ok := paths.([]any)
	if !ok || len(pathsList) != 2 {
		t.Errorf("paths = %v, want 2-element list", paths)
	}
}

// =============================================================================
// SC-6: End-to-end pipeline: gate output → variable → template
// =============================================================================

func TestGateOutputPipelineE2E(t *testing.T) {
	t.Parallel()

	// Step 1: Simulate AI gate returning structured data
	gateResult := &GateEvaluationResult{
		Approved: true,
		Reason:   "all checks passed",
		OutputData: map[string]any{
			"review_summary": "Code quality is excellent",
			"metrics": map[string]any{
				"complexity": float64(3),
				"coverage":   float64(95),
			},
		},
		OutputVar: "CODE_REVIEW",
	}

	// Step 2: Apply gate output to vars (what executor does after gate evaluation)
	vars := map[string]string{
		"TASK_ID":      "TASK-001",
		"SPEC_CONTENT": "existing spec content",
	}

	applyGateOutputToVars(vars, gateResult)

	// Step 3: Verify variable is stored
	codeReview, ok := vars["CODE_REVIEW"]
	if !ok {
		t.Fatal("CODE_REVIEW variable not set after gate output pipeline")
	}

	// Step 4: Verify stored value is valid JSON with full structure
	var parsed map[string]any
	if err := json.Unmarshal([]byte(codeReview), &parsed); err != nil {
		t.Fatalf("stored gate output is not valid JSON: %v", err)
	}
	if parsed["review_summary"] != "Code quality is excellent" {
		t.Errorf("review_summary = %v, want 'Code quality is excellent'", parsed["review_summary"])
	}
	metrics, ok := parsed["metrics"].(map[string]any)
	if !ok {
		t.Fatal("metrics not a map in stored output")
	}
	if metrics["coverage"] != float64(95) {
		t.Errorf("coverage = %v, want 95", metrics["coverage"])
	}

	// Step 5: Verify existing vars are preserved
	if vars["TASK_ID"] != "TASK-001" {
		t.Errorf("TASK_ID was modified: %q", vars["TASK_ID"])
	}
	if vars["SPEC_CONTENT"] != "existing spec content" {
		t.Errorf("SPEC_CONTENT was modified: %q", vars["SPEC_CONTENT"])
	}
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestGateOutputEmptyMap(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	gateResult := &GateEvaluationResult{
		Approved:   true,
		OutputData: map[string]any{}, // Empty map - still valid
		OutputVar:  "EMPTY_RESULT",
	}

	applyGateOutputToVars(vars, gateResult)

	val, ok := vars["EMPTY_RESULT"]
	if !ok {
		t.Fatal("empty map should still be stored as a variable")
	}
	if val != "{}" {
		t.Errorf("empty map serialization = %q, want %q", val, "{}")
	}
}

func TestGateOutputNestedData(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	gateResult := &GateEvaluationResult{
		Approved: true,
		OutputData: map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"value": "deep",
				},
			},
		},
		OutputVar: "NESTED_RESULT",
	}

	applyGateOutputToVars(vars, gateResult)

	val, ok := vars["NESTED_RESULT"]
	if !ok {
		t.Fatal("nested data should be stored")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("nested output is not valid JSON: %v", err)
	}

	// Navigate the nested structure
	l1, ok := parsed["level1"].(map[string]any)
	if !ok {
		t.Fatal("level1 not a map")
	}
	l2, ok := l1["level2"].(map[string]any)
	if !ok {
		t.Fatal("level2 not a map")
	}
	if l2["value"] != "deep" {
		t.Errorf("nested value = %v, want 'deep'", l2["value"])
	}
}

func TestGateOutputWhitespaceVar(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	gateResult := &GateEvaluationResult{
		Approved:   true,
		OutputData: map[string]any{"key": "value"},
		OutputVar:  "   ", // Whitespace-only
	}

	applyGateOutputToVars(vars, gateResult)

	// Whitespace-only OutputVar should be treated as empty - no variable stored
	if len(vars) != 0 {
		t.Errorf("whitespace-only OutputVar should not store variable, got %v", vars)
	}
}

func TestGateOutputOverwriteSequential(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	// First gate sets the variable
	gate1 := &GateEvaluationResult{
		Approved:   true,
		OutputData: map[string]any{"round": float64(1)},
		OutputVar:  "SHARED_VAR",
	}
	applyGateOutputToVars(vars, gate1)

	// Second gate overwrites the same variable
	gate2 := &GateEvaluationResult{
		Approved:   true,
		OutputData: map[string]any{"round": float64(2)},
		OutputVar:  "SHARED_VAR",
	}
	applyGateOutputToVars(vars, gate2)

	// Later gate should win
	val := vars["SHARED_VAR"]
	var parsed map[string]any
	if err := json.Unmarshal([]byte(val), &parsed); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if parsed["round"] != float64(2) {
		t.Errorf("SHARED_VAR should be from gate2, got round=%v", parsed["round"])
	}
}

func TestGateOutputWithTemplateSyntax(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	// Gate output intentionally contains template syntax
	gateResult := &GateEvaluationResult{
		Approved: true,
		OutputData: map[string]any{
			"instruction": "Use {{SPEC_CONTENT}} as reference",
		},
		OutputVar: "DYNAMIC_PROMPT",
	}

	applyGateOutputToVars(vars, gateResult)

	val := vars["DYNAMIC_PROMPT"]
	if !strings.Contains(val, "{{SPEC_CONTENT}}") {
		t.Errorf("template syntax should be stored literally, got %q", val)
	}
}

func TestNonAIGateNoOutput(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	// Auto/human gate with no output data
	gateResult := &GateEvaluationResult{
		Approved:   true,
		Reason:     "auto-approved",
		OutputData: nil,
		OutputVar:  "",
	}

	applyGateOutputToVars(vars, gateResult)

	if len(vars) != 0 {
		t.Errorf("non-AI gate with no output should not set any vars, got %v", vars)
	}
}

// =============================================================================
// Failure Modes
// =============================================================================

func TestGateOutputSerializationError(t *testing.T) {
	t.Parallel()

	vars := map[string]string{}

	// OutputData with a value that cannot be JSON-serialized (channel)
	// Note: In practice map[string]any from JSON unmarshal won't have this,
	// but the function should handle it gracefully.
	gateResult := &GateEvaluationResult{
		Approved:   true,
		OutputData: map[string]any{"bad": make(chan int)},
		OutputVar:  "BAD_DATA",
	}

	// Should not panic; should either skip or log error
	applyGateOutputToVars(vars, gateResult)

	// Variable should NOT be set on serialization failure
	if _, ok := vars["BAD_DATA"]; ok {
		t.Error("variable should not be set when serialization fails")
	}
}

func TestGateOutputOverwriteBuiltin(t *testing.T) {
	t.Parallel()

	vars := map[string]string{
		"TASK_ID": "TASK-001",
	}

	// Gate output overwrites a built-in variable
	gateResult := &GateEvaluationResult{
		Approved:   true,
		OutputData: map[string]any{"custom": "data"},
		OutputVar:  "TASK_ID", // Overwrites built-in
	}

	applyGateOutputToVars(vars, gateResult)

	// Per spec: gate output has higher precedence, overwrites built-in
	val := vars["TASK_ID"]
	if !strings.Contains(val, "custom") {
		t.Errorf("gate output should overwrite built-in, got %q", val)
	}
}

// =============================================================================
// Preservation: Existing behavior must not break
// =============================================================================

func TestBuildRetryContext_UnchangedSignature(t *testing.T) {
	t.Parallel()

	// Existing BuildRetryContext must continue to work with its current signature
	result := BuildRetryContext("implement", "tests failed", "error output", 2, "/tmp/context.txt")

	if !strings.Contains(result, "implement") {
		t.Error("BuildRetryContext output missing phase name")
	}
	if !strings.Contains(result, "tests failed") {
		t.Error("BuildRetryContext output missing reason")
	}
	if !strings.Contains(result, "error output") {
		t.Error("BuildRetryContext output missing failure output")
	}
	if !strings.Contains(result, "#2") {
		t.Error("BuildRetryContext output missing attempt number")
	}
	if !strings.Contains(result, "/tmp/context.txt") {
		t.Error("BuildRetryContext output missing context file")
	}
}
