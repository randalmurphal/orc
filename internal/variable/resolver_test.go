package variable

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveStatic(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	def := Definition{
		Name:         "TEST_VAR",
		SourceType:   SourceStatic,
		SourceConfig: json.RawMessage(`{"value": "hello world"}`),
	}

	resolved, err := resolver.Resolve(context.Background(), &def, nil, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Value != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", resolved.Value)
	}
}

func TestResolveEnv(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv

	resolver := NewResolver(t.TempDir())

	t.Setenv("TEST_ENV_VAR", "from environment")

	def := Definition{
		Name:         "TEST_VAR",
		SourceType:   SourceEnv,
		SourceConfig: json.RawMessage(`{"var": "TEST_ENV_VAR"}`),
	}

	resolved, err := resolver.Resolve(context.Background(), &def, nil, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Value != "from environment" {
		t.Errorf("expected 'from environment', got '%s'", resolved.Value)
	}
}

func TestResolveEnvDefault(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	def := Definition{
		Name:         "TEST_VAR",
		SourceType:   SourceEnv,
		SourceConfig: json.RawMessage(`{"var": "NONEXISTENT_VAR_12345", "default": "fallback"}`),
	}

	resolved, err := resolver.Resolve(context.Background(), &def, nil, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Value != "fallback" {
		t.Errorf("expected 'fallback', got '%s'", resolved.Value)
	}
}

func TestResolveEnvFromContext(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	def := Definition{
		Name:         "TEST_VAR",
		SourceType:   SourceEnv,
		SourceConfig: json.RawMessage(`{"var": "CONTEXT_VAR"}`),
	}

	rctx := &ResolutionContext{
		Environment: map[string]string{
			"CONTEXT_VAR": "from context",
		},
	}

	resolved, err := resolver.Resolve(context.Background(), &def, rctx, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Value != "from context" {
		t.Errorf("expected 'from context', got '%s'", resolved.Value)
	}
}

func TestResolvePhaseOutput(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	def := Definition{
		Name:         "SPEC",
		SourceType:   SourcePhaseOutput,
		SourceConfig: json.RawMessage(`{"phase": "spec"}`),
	}

	rctx := &ResolutionContext{
		PriorOutputs: map[string]string{
			"spec": "# Specification\n\nThis is the spec content.",
		},
	}

	resolved, err := resolver.Resolve(context.Background(), &def, rctx, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Value != "# Specification\n\nThis is the spec content." {
		t.Errorf("unexpected value: %s", resolved.Value)
	}
}

func TestResolvePromptFragment(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create fragment file
	fragmentDir := filepath.Join(tmpDir, ".orc", "prompts", "fragments")
	if err := os.MkdirAll(fragmentDir, 0755); err != nil {
		t.Fatalf("create fragment dir: %v", err)
	}

	fragmentPath := filepath.Join(fragmentDir, "test.md")
	if err := os.WriteFile(fragmentPath, []byte("# Test Fragment\n\nThis is reusable."), 0644); err != nil {
		t.Fatalf("write fragment: %v", err)
	}

	resolver := NewResolver(tmpDir)

	def := Definition{
		Name:         "FRAGMENT",
		SourceType:   SourcePromptFragment,
		SourceConfig: json.RawMessage(`{"path": "test.md"}`),
	}

	resolved, err := resolver.Resolve(context.Background(), &def, nil, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "# Test Fragment\n\nThis is reusable."
	if resolved.Value != expected {
		t.Errorf("expected %q, got %q", expected, resolved.Value)
	}
}

func TestResolveAllWithBuiltins(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	rctx := &ResolutionContext{
		TaskID:          "TASK-001",
		TaskTitle:       "Test Task",
		TaskDescription: "A test task",
		Phase:           "implement",
		Iteration:       3,
		WorkingDir:      "/path/to/worktree",
		PriorOutputs: map[string]string{
			"spec": "Spec content here",
		},
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check built-in variables
	tests := map[string]string{
		"TASK_ID":      "TASK-001",
		"TASK_TITLE":   "Test Task",
		"PHASE":        "implement",
		"ITERATION":    "3",
		"WORKTREE_PATH": "/path/to/worktree",
		"SPEC_CONTENT": "Spec content here",
		"OUTPUT_SPEC":  "Spec content here",
	}

	for name, expected := range tests {
		if vars[name] != expected {
			t.Errorf("%s: expected %q, got %q", name, expected, vars[name])
		}
	}
}

func TestResolveAllQAOutputDir(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// When TaskID is set and QA context is active, QA_OUTPUT_DIR should be populated
	rctx := &ResolutionContext{
		TaskID:      "TASK-123",
		QAIteration: 1,
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "/tmp/orc-qa-TASK-123"
	if vars["QA_OUTPUT_DIR"] != expected {
		t.Errorf("QA_OUTPUT_DIR: expected %q, got %q", expected, vars["QA_OUTPUT_DIR"])
	}

	// When TaskID is empty, QA_OUTPUT_DIR should be empty
	emptyCtx := &ResolutionContext{}
	vars2, err := resolver.ResolveAll(context.Background(), nil, emptyCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if vars2["QA_OUTPUT_DIR"] != "" {
		t.Errorf("QA_OUTPUT_DIR with empty TaskID: expected empty, got %q", vars2["QA_OUTPUT_DIR"])
	}
}

func TestCache(t *testing.T) {
	t.Parallel()

	cache := NewCache()

	// Set value
	cache.Set("key1", "value1", 1*time.Hour)

	// Get value
	value, ok := cache.Get("key1")
	if !ok {
		t.Error("expected to find cached value")
	}
	if value != "value1" {
		t.Errorf("expected 'value1', got '%s'", value)
	}

	// Non-existent key
	_, ok = cache.Get("nonexistent")
	if ok {
		t.Error("expected not to find non-existent key")
	}

	// Delete
	cache.Delete("key1")
	_, ok = cache.Get("key1")
	if ok {
		t.Error("expected key to be deleted")
	}
}

func TestCacheExpiration(t *testing.T) {
	t.Parallel()

	cache := NewCache()

	// Set value with very short TTL
	cache.Set("key1", "value1", 1*time.Millisecond)

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Should be expired
	_, ok := cache.Get("key1")
	if ok {
		t.Error("expected cached value to be expired")
	}
}

func TestRenderTemplate(t *testing.T) {
	t.Parallel()

	vars := VariableSet{
		"TASK_ID":    "TASK-001",
		"PHASE":      "implement",
		"SPEC_CONTENT": "The specification",
	}

	template := `Task: {{TASK_ID}}
Phase: {{PHASE}}
Spec: {{SPEC_CONTENT}}
Missing: {{MISSING_VAR}}`

	expected := `Task: TASK-001
Phase: implement
Spec: The specification
Missing: `

	result := RenderTemplate(template, vars)
	if result != expected {
		t.Errorf("expected:\n%s\n\ngot:\n%s", expected, result)
	}
}

func TestRenderTemplateStrict(t *testing.T) {
	t.Parallel()

	vars := VariableSet{
		"TASK_ID": "TASK-001",
	}

	template := `Task: {{TASK_ID}}, Missing: {{MISSING_VAR}}, Also: {{ANOTHER_MISSING}}`

	result, missing := RenderTemplateStrict(template, vars)

	if len(missing) != 2 {
		t.Errorf("expected 2 missing variables, got %d: %v", len(missing), missing)
	}

	// Check that missing variables are listed
	found := make(map[string]bool)
	for _, m := range missing {
		found[m] = true
	}

	if !found["MISSING_VAR"] {
		t.Error("expected MISSING_VAR in missing list")
	}
	if !found["ANOTHER_MISSING"] {
		t.Error("expected ANOTHER_MISSING in missing list")
	}

	// Result should still have the values replaced
	if result != "Task: TASK-001, Missing: , Also: " {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestRenderTemplateConditionals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		vars     VariableSet
		template string
		expected string
	}{
		{
			name: "conditional with content present",
			vars: VariableSet{
				"CONSTITUTION_CONTENT": "Rule 1: Never panic\nRule 2: Test everything",
			},
			template: `Start
{{#if CONSTITUTION_CONTENT}}
Constitution:
{{CONSTITUTION_CONTENT}}
{{/if}}
End`,
			expected: `Start

Constitution:
Rule 1: Never panic
Rule 2: Test everything

End`,
		},
		{
			name:     "conditional with content empty",
			vars:     VariableSet{},
			template: `Start
{{#if CONSTITUTION_CONTENT}}
Constitution:
{{CONSTITUTION_CONTENT}}
{{/if}}
End`,
			expected: `Start

End`,
		},
		{
			name: "conditional with empty string value",
			vars: VariableSet{
				"CONSTITUTION_CONTENT": "",
			},
			template: `Before{{#if CONSTITUTION_CONTENT}} - Has constitution{{/if}} - After`,
			expected: `Before - After`,
		},
		{
			name: "multiple conditionals",
			vars: VariableSet{
				"HAS_TESTS": "true",
			},
			template: `{{#if HAS_TESTS}}Has tests{{/if}}{{#if MISSING_VAR}}Missing{{/if}}`,
			expected: `Has tests`,
		},
		{
			name: "nested content with vars",
			vars: VariableSet{
				"TASK_ID":              "TASK-001",
				"CONSTITUTION_CONTENT": "Important rules",
			},
			template: `Task: {{TASK_ID}}
{{#if CONSTITUTION_CONTENT}}
Rules: {{CONSTITUTION_CONTENT}}
{{/if}}`,
			expected: `Task: TASK-001

Rules: Important rules
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderTemplate(tt.template, tt.vars)
			if result != tt.expected {
				t.Errorf("expected:\n%q\n\ngot:\n%q", tt.expected, result)
			}
		})
	}
}

func TestVariableSetMerge(t *testing.T) {
	t.Parallel()

	vs1 := VariableSet{
		"A": "1",
		"B": "2",
	}

	vs2 := VariableSet{
		"B": "overwritten",
		"C": "3",
	}

	vs1.Merge(vs2)

	if vs1["A"] != "1" {
		t.Errorf("expected A=1, got A=%s", vs1["A"])
	}
	if vs1["B"] != "overwritten" {
		t.Errorf("expected B=overwritten, got B=%s", vs1["B"])
	}
	if vs1["C"] != "3" {
		t.Errorf("expected C=3, got C=%s", vs1["C"])
	}
}

func TestResolveRequired(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// Required variable with bad config should fail
	defs := []Definition{
		{
			Name:         "BAD_VAR",
			SourceType:   SourcePhaseOutput,
			SourceConfig: json.RawMessage(`{"phase": "nonexistent"}`),
			Required:     true,
		},
	}

	_, err := resolver.ResolveAll(context.Background(), defs, &ResolutionContext{})
	if err == nil {
		t.Error("expected error for required variable that can't be resolved")
	}
}

func TestResolveNonRequiredWithDefault(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// Non-required variable with default should use default on failure
	defs := []Definition{
		{
			Name:         "OPTIONAL_VAR",
			SourceType:   SourcePhaseOutput,
			SourceConfig: json.RawMessage(`{"phase": "nonexistent"}`),
			Required:     false,
			DefaultValue: "default value",
		},
	}

	vars, err := resolver.ResolveAll(context.Background(), defs, &ResolutionContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vars["OPTIONAL_VAR"] != "default value" {
		t.Errorf("expected 'default value', got '%s'", vars["OPTIONAL_VAR"])
	}
}

func TestResolveStaticWithInterpolation(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// Variables resolved in order, later can reference earlier via {{VAR}}
	defs := []Definition{
		{
			Name:         "PREFIX",
			SourceType:   SourceStatic,
			SourceConfig: json.RawMessage(`{"value": "hello"}`),
		},
		{
			Name:         "COMBINED",
			SourceType:   SourceStatic,
			SourceConfig: json.RawMessage(`{"value": "{{PREFIX}} world"}`),
		},
	}

	vars, err := resolver.ResolveAll(context.Background(), defs, &ResolutionContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vars["PREFIX"] != "hello" {
		t.Errorf("PREFIX: expected 'hello', got '%s'", vars["PREFIX"])
	}
	if vars["COMBINED"] != "hello world" {
		t.Errorf("COMBINED: expected 'hello world', got '%s'", vars["COMBINED"])
	}
}

func TestResolvePhaseOutputWithExtraction(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// Phase output is JSON, extract specific field
	def := Definition{
		Name:         "SCORE",
		SourceType:   SourcePhaseOutput,
		SourceConfig: json.RawMessage(`{"phase": "spec"}`),
		Extract:      "data.score",
	}

	rctx := &ResolutionContext{
		PriorOutputs: map[string]string{
			"spec": `{"status": "complete", "data": {"score": 95, "notes": "excellent"}}`,
		},
	}

	resolved, err := resolver.Resolve(context.Background(), &def, rctx, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Value != "95" {
		t.Errorf("expected '95', got '%s'", resolved.Value)
	}
}

func TestResolvePhaseOutputWithNestedExtraction(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// Extract nested array element
	def := Definition{
		Name:         "FIRST_ITEM",
		SourceType:   SourcePhaseOutput,
		SourceConfig: json.RawMessage(`{"phase": "breakdown"}`),
		Extract:      "tasks.0.title",
	}

	rctx := &ResolutionContext{
		PriorOutputs: map[string]string{
			"breakdown": `{"tasks": [{"title": "First task", "done": false}, {"title": "Second task", "done": false}]}`,
		},
	}

	resolved, err := resolver.Resolve(context.Background(), &def, rctx, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Value != "First task" {
		t.Errorf("expected 'First task', got '%s'", resolved.Value)
	}
}

func TestResolveBuiltinVariablesUsedInInterpolation(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// Built-in variables should be available for interpolation
	defs := []Definition{
		{
			Name:         "TASK_LABEL",
			SourceType:   SourceStatic,
			SourceConfig: json.RawMessage(`{"value": "[{{TASK_ID}}] {{TASK_TITLE}}"}`),
		},
	}

	rctx := &ResolutionContext{
		TaskID:    "TASK-123",
		TaskTitle: "Fix the bug",
	}

	vars, err := resolver.ResolveAll(context.Background(), defs, rctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vars["TASK_LABEL"] != "[TASK-123] Fix the bug" {
		t.Errorf("expected '[TASK-123] Fix the bug', got '%s'", vars["TASK_LABEL"])
	}
}

func TestResolveVariableChaining(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// Chain of variables: A -> B -> C
	defs := []Definition{
		{
			Name:         "VAR_A",
			SourceType:   SourceStatic,
			SourceConfig: json.RawMessage(`{"value": "alpha"}`),
		},
		{
			Name:         "VAR_B",
			SourceType:   SourceStatic,
			SourceConfig: json.RawMessage(`{"value": "{{VAR_A}}-beta"}`),
		},
		{
			Name:         "VAR_C",
			SourceType:   SourceStatic,
			SourceConfig: json.RawMessage(`{"value": "{{VAR_B}}-gamma"}`),
		},
	}

	vars, err := resolver.ResolveAll(context.Background(), defs, &ResolutionContext{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if vars["VAR_A"] != "alpha" {
		t.Errorf("VAR_A: expected 'alpha', got '%s'", vars["VAR_A"])
	}
	if vars["VAR_B"] != "alpha-beta" {
		t.Errorf("VAR_B: expected 'alpha-beta', got '%s'", vars["VAR_B"])
	}
	if vars["VAR_C"] != "alpha-beta-gamma" {
		t.Errorf("VAR_C: expected 'alpha-beta-gamma', got '%s'", vars["VAR_C"])
	}
}

func TestResolveExtractionOnNonJSON(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// Extract on non-JSON should return empty (path not found)
	def := Definition{
		Name:         "FIELD",
		SourceType:   SourcePhaseOutput,
		SourceConfig: json.RawMessage(`{"phase": "spec"}`),
		Extract:      "data.field",
	}

	rctx := &ResolutionContext{
		PriorOutputs: map[string]string{
			"spec": "This is plain text, not JSON",
		},
	}

	resolved, err := resolver.Resolve(context.Background(), &def, rctx, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Value != "" {
		t.Errorf("expected empty string for non-JSON extraction, got '%s'", resolved.Value)
	}
}

func TestResolveExtractionMissingPath(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())

	// Extract path doesn't exist in JSON
	def := Definition{
		Name:         "MISSING",
		SourceType:   SourcePhaseOutput,
		SourceConfig: json.RawMessage(`{"phase": "spec"}`),
		Extract:      "nonexistent.path",
	}

	rctx := &ResolutionContext{
		PriorOutputs: map[string]string{
			"spec": `{"other": "field"}`,
		},
	}

	resolved, err := resolver.Resolve(context.Background(), &def, rctx, VariableSet{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolved.Value != "" {
		t.Errorf("expected empty string for missing path, got '%s'", resolved.Value)
	}
}
