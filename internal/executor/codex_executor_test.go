package executor

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCodexExecutor_ImplementsTurnExecutor(t *testing.T) {
	// Compile-time check via var _ TurnExecutor = (*CodexExecutor)(nil) in source.
	// This test validates the constructor produces a usable executor.
	exec := NewCodexExecutor(
		WithCodexWorkdir("/tmp"),
		WithCodexModel("gpt-5"),
		WithCodexPhaseID("implement"),
	)
	if exec == nil {
		t.Fatal("NewCodexExecutor returned nil")
	}
	if exec.model != "gpt-5" {
		t.Errorf("model = %q, want %q", exec.model, "gpt-5")
	}
	if exec.phaseID != "implement" {
		t.Errorf("phaseID = %q, want %q", exec.phaseID, "implement")
	}
}

func TestCodexExecutor_SessionManagement(t *testing.T) {
	exec := NewCodexExecutor()

	if exec.SessionID() != "" {
		t.Errorf("initial session ID should be empty, got %q", exec.SessionID())
	}

	exec.UpdateSessionID("test-session-123")
	if exec.SessionID() != "test-session-123" {
		t.Errorf("session ID = %q, want %q", exec.SessionID(), "test-session-123")
	}
	if !exec.resume {
		t.Error("resume should be true after UpdateSessionID")
	}
}

func TestCodexExecutor_Defaults(t *testing.T) {
	exec := NewCodexExecutor()

	if exec.codexPath != "codex" {
		t.Errorf("default codexPath = %q, want %q", exec.codexPath, "codex")
	}
	if exec.schemaRetries != 2 {
		t.Errorf("default schemaRetries = %d, want 2", exec.schemaRetries)
	}
	if !exec.bypassApprovalsAndSandbox {
		t.Error("default bypassApprovalsAndSandbox should be true")
	}
}

func TestCodexExecutor_WriteSchemaFile(t *testing.T) {
	exec := NewCodexExecutor()

	schema := `{"type":"object","properties":{"status":{"type":"string"}}}`
	path, err := exec.writeSchemaFile(schema)
	if err != nil {
		t.Fatalf("writeSchemaFile failed: %v", err)
	}
	defer os.Remove(path)

	// Verify file exists and has correct content.
	// writeSchemaFile applies OpenAI schema rules (additionalProperties:false,
	// required:all, nullable for originally-optional fields).
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	want := `{"additionalProperties":false,"properties":{"status":{"type":["string","null"]}},"required":["status"],"type":"object"}`
	if string(data) != want {
		t.Errorf("schema file content = %q, want %q", string(data), want)
	}
}

func TestCodexExecutor_WriteSchemaFile_InvalidJSON(t *testing.T) {
	exec := NewCodexExecutor()

	_, err := exec.writeSchemaFile("not valid json")
	if err == nil {
		t.Error("expected error for invalid JSON schema")
	}
}

func TestCodexExecutor_AllOptions(t *testing.T) {
	exec := NewCodexExecutor(
		WithCodexPath("/usr/bin/codex"),
		WithCodexWorkdir("/project"),
		WithCodexModel("o3"),
		WithCodexSessionID("sess-1"),
		WithCodexResume(true),
		WithCodexPhaseID("review"),
		WithCodexProducesArtifact(true),
		WithCodexReviewRound(2),
		WithCodexSchemaRetries(5),
		WithCodexLocalProvider("ollama"),
		WithCodexTaskID("TASK-001"),
		WithCodexRunID("RUN-001"),
	)

	if exec.codexPath != "/usr/bin/codex" {
		t.Errorf("codexPath = %q", exec.codexPath)
	}
	if exec.workdir != "/project" {
		t.Errorf("workdir = %q", exec.workdir)
	}
	if exec.model != "o3" {
		t.Errorf("model = %q", exec.model)
	}
	if exec.sessionID != "sess-1" {
		t.Errorf("sessionID = %q", exec.sessionID)
	}
	if !exec.resume {
		t.Error("resume should be true")
	}
	if exec.phaseID != "review" {
		t.Errorf("phaseID = %q", exec.phaseID)
	}
	if !exec.producesArtifact {
		t.Error("producesArtifact should be true")
	}
	if exec.reviewRound != 2 {
		t.Errorf("reviewRound = %d", exec.reviewRound)
	}
	if exec.schemaRetries != 5 {
		t.Errorf("schemaRetries = %d", exec.schemaRetries)
	}
	if exec.localProvider != "ollama" {
		t.Errorf("localProvider = %q", exec.localProvider)
	}
	if exec.taskID != "TASK-001" {
		t.Errorf("taskID = %q", exec.taskID)
	}
	if exec.runID != "RUN-001" {
		t.Errorf("runID = %q", exec.runID)
	}
}

func TestIsJSONParseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"json error", errString("invalid JSON"), true},
		{"parse error", errString("failed to parse response"), true},
		{"unmarshal", errString("cannot unmarshal string"), true},
		{"network error", errString("connection refused"), false},
		{"context canceled", errString("context canceled"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isJSONParseError(tt.err); got != tt.want {
				t.Errorf("isJSONParseError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short = %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("truncate long = %q, want %q", got, "hello...")
	}
}

func TestEnsureAdditionalPropertiesFalse(t *testing.T) {
	t.Run("adds additionalProperties and required to root", func(t *testing.T) {
		schema := `{"type":"object","properties":{"status":{"type":"string"},"reason":{"type":"string"}},"required":["status"]}`
		result := ensureAdditionalPropertiesFalse(schema)

		var parsed map[string]any
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if ap, ok := parsed["additionalProperties"]; !ok || ap != false {
			t.Fatalf("expected additionalProperties:false at root, got %v", parsed)
		}
		// OpenAI requires ALL property keys in required
		req, ok := parsed["required"].([]any)
		if !ok {
			t.Fatalf("expected required to be array, got %T", parsed["required"])
		}
		reqSet := map[string]bool{}
		for _, v := range req {
			reqSet[v.(string)] = true
		}
		if !reqSet["status"] || !reqSet["reason"] {
			t.Fatalf("required should include all properties, got %v", req)
		}

		reason := parsed["properties"].(map[string]any)["reason"].(map[string]any)
		reasonTypes := reason["type"].([]any)
		if len(reasonTypes) != 2 || reasonTypes[0] != "string" || reasonTypes[1] != "null" {
			t.Fatalf("optional reason field should become nullable, got %v", reasonTypes)
		}
	})

	t.Run("adds to nested objects and array items", func(t *testing.T) {
		schema := `{"type":"object","properties":{"items":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}}}}}}`
		result := ensureAdditionalPropertiesFalse(schema)

		var parsed map[string]any
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("unmarshal schema: %v", err)
		}
		props := parsed["properties"].(map[string]any)
		items := props["items"].(map[string]any)
		itemSchema := items["items"].(map[string]any)
		if ap, ok := itemSchema["additionalProperties"]; !ok || ap != false {
			t.Fatalf("expected additionalProperties:false on nested items object, got %v", itemSchema)
		}
		req := itemSchema["required"].([]any)
		reqSet := map[string]bool{}
		for _, v := range req {
			reqSet[v.(string)] = true
		}
		if !reqSet["id"] || !reqSet["name"] {
			t.Fatalf("required on nested items should include all properties, got %v", req)
		}
	})

	t.Run("handles invalid JSON gracefully", func(t *testing.T) {
		schema := `{not valid`
		result := ensureAdditionalPropertiesFalse(schema)
		if result != schema {
			t.Fatal("should return unchanged for invalid JSON")
		}
	})

	t.Run("works on real implement schema", func(t *testing.T) {
		schema := ImplementCompletionSchema
		result := ensureAdditionalPropertiesFalse(schema)

		var parsed map[string]any
		if err := json.Unmarshal([]byte(result), &parsed); err != nil {
			t.Fatalf("invalid JSON after transform: %v", err)
		}
		// Root should have additionalProperties: false
		if ap := parsed["additionalProperties"]; ap != false {
			t.Fatalf("root missing additionalProperties:false")
		}
		// Nested verification.build should also have additionalProperties and required
		props := parsed["properties"].(map[string]any)
		verif := props["verification"].(map[string]any)
		verifProps := verif["properties"].(map[string]any)
		build := verifProps["build"].(map[string]any)
		if ap := build["additionalProperties"]; ap != false {
			t.Fatalf("verification.build missing additionalProperties:false")
		}
		buildReq := build["required"].([]any)
		if len(buildReq) != 1 || buildReq[0] != "status" {
			t.Fatalf("verification.build.required should be [status], got %v", buildReq)
		}

		tests := verifProps["tests"].(map[string]any)
		testsProps := tests["properties"].(map[string]any)
		command := testsProps["command"].(map[string]any)
		commandTypes := command["type"].([]any)
		if len(commandTypes) != 2 || commandTypes[0] != "string" || commandTypes[1] != "null" {
			t.Fatalf("optional command field should become nullable, got %v", commandTypes)
		}
	})
}

func TestExtractLastJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "single valid JSON object",
			in:   `{"status":"complete","summary":"done"}`,
			want: `{"status":"complete","summary":"done"}`,
		},
		{
			name: "concatenated JSON objects returns last",
			in:   `{"status":"continue"}{"status":"continue"}{"status":"complete","summary":"done"}`,
			want: `{"status":"complete","summary":"done"}`,
		},
		{
			name: "no valid JSON returns original",
			in:   "this is not json at all",
			want: "this is not json at all",
		},
		{
			name: "whitespace only returns empty",
			in:   "   \n\t  ",
			want: "",
		},
		{
			name: "nested braces in values",
			in:   `{"a":1}{"msg":"use {x}","status":"complete"}`,
			want: `{"msg":"use {x}","status":"complete"}`,
		},
		{
			name: "single JSON with nested objects unchanged",
			in:   `{"outer":{"inner":{"deep":"value"}},"status":"complete"}`,
			want: `{"outer":{"inner":{"deep":"value"}},"status":"complete"}`,
		},
		{
			name: "whitespace between concatenated objects",
			in:   `{"status":"continue"}  {"status":"complete"}`,
			want: `{"status":"complete"}`,
		},
		{
			name: "three concatenated objects",
			in:   `{"round":1}{"round":2}{"round":3}`,
			want: `{"round":3}`,
		},
		{
			name: "leading whitespace on valid JSON",
			in:   "  \n  " + `{"status":"complete"}`,
			want: `{"status":"complete"}`,
		},
		{
			name: "trailing whitespace on valid JSON",
			in:   `{"status":"complete"}` + "  \n  ",
			want: `{"status":"complete"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLastJSON(tt.in)
			if got != tt.want {
				t.Errorf("extractLastJSON(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// errString is a simple error type for testing.
type errString string

func (e errString) Error() string { return string(e) }
