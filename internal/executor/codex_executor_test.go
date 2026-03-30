package executor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	llmkit "github.com/randalmurphal/llmkit/v2"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
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

func TestCodexExecutor_ExecuteSingleTurn_StreamsAndCapturesSession(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fake-codex.sh")
	script := `#!/bin/sh
echo '{"type":"thread.started","thread_id":"sess-123"}'
echo '{"type":"item.completed","item":{"type":"assistant_message","text":"partial "}}'
sleep 0.2
echo '{"type":"turn.completed","output":[{"text":"final answer"}],"turn_usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex script: %v", err)
	}

	backend := &mockTranscriptBackend{}
	exec := NewCodexExecutor(
		WithCodexPath(scriptPath),
		WithCodexWorkdir(tmpDir),
		WithCodexModel("gpt-5.4"),
		WithCodexPhaseID("implement"),
		WithCodexBackend(backend),
		WithCodexTaskID("TASK-001"),
		WithCodexRunID("RUN-001"),
	)

	done := make(chan struct {
		result *TurnResult
		err    error
	}, 1)
	go func() {
		result, err := exec.executeSingleTurn(context.Background(), "do the thing", "", time.Now())
		done <- struct {
			result *TurnResult
			err    error
		}{result: result, err: err}
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if backend.Count() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	transcripts := backend.Snapshot()
	if len(transcripts) < 2 {
		t.Fatalf("expected prompt and chunk transcripts before completion, got %d", len(transcripts))
	}
	if transcripts[0].Type != "user" {
		t.Fatalf("first transcript type = %q, want user", transcripts[0].Type)
	}
	if transcripts[1].Type != "chunk" {
		t.Fatalf("second transcript type = %q, want chunk", transcripts[1].Type)
	}

	outcome := <-done
	if outcome.err != nil {
		t.Fatalf("executeSingleTurn failed: %v", outcome.err)
	}
	if outcome.result.SessionID != "sess-123" {
		t.Fatalf("session id = %q, want sess-123", outcome.result.SessionID)
	}
	if outcome.result.Content != "final answer" {
		t.Fatalf("content = %q, want %q", outcome.result.Content, "final answer")
	}
	if outcome.result.Usage == nil || outcome.result.Usage.TotalTokens != 15 {
		t.Fatalf("usage = %+v, want total 15", outcome.result.Usage)
	}

	transcripts = backend.Snapshot()
	if len(transcripts) != 3 {
		t.Fatalf("expected 3 transcripts after completion, got %d", len(transcripts))
	}
	if transcripts[2].Type != "assistant" {
		t.Fatalf("final transcript type = %q, want assistant", transcripts[2].Type)
	}
	if transcripts[2].SessionID != "sess-123" {
		t.Fatalf("final transcript session_id = %q, want sess-123", transcripts[2].SessionID)
	}
	if transcripts[2].Content != "final answer" {
		t.Fatalf("final transcript content = %q, want final answer", transcripts[2].Content)
	}
}

func TestCodexExecutor_ExecuteSingleTurn_StreamsToolCallsBeforeCompletion(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fake-codex-tool.sh")
	script := `#!/bin/sh
echo '{"type":"thread.started","thread_id":"sess-tool-123"}'
echo '{"type":"item.started","item":{"type":"tool_call","id":"tool-1","name":"Read","arguments":{"file_path":"main.go"}}}'
sleep 0.2
echo '{"type":"turn.completed","output":[{"text":"done"}],"turn_usage":{"input_tokens":5,"output_tokens":2,"total_tokens":7}}'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex script: %v", err)
	}

	backend := &mockTranscriptBackend{}
	exec := NewCodexExecutor(
		WithCodexPath(scriptPath),
		WithCodexWorkdir(tmpDir),
		WithCodexModel("gpt-5.4"),
		WithCodexPhaseID("implement_codex"),
		WithCodexBackend(backend),
		WithCodexTaskID("TASK-001"),
		WithCodexRunID("RUN-001"),
	)

	done := make(chan struct {
		result *TurnResult
		err    error
	}, 1)
	go func() {
		result, err := exec.executeSingleTurn(context.Background(), "do the thing", "", time.Now())
		done <- struct {
			result *TurnResult
			err    error
		}{result: result, err: err}
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if backend.Count() >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	transcripts := backend.Snapshot()
	if len(transcripts) < 2 {
		t.Fatalf("expected prompt and tool transcripts before completion, got %d", len(transcripts))
	}
	if transcripts[0].Type != "user" {
		t.Fatalf("first transcript type = %q, want user", transcripts[0].Type)
	}
	if transcripts[1].Type != "tool" {
		t.Fatalf("second transcript type = %q, want tool", transcripts[1].Type)
	}
	if transcripts[1].SessionID != "sess-tool-123" {
		t.Fatalf("tool transcript session_id = %q, want sess-tool-123", transcripts[1].SessionID)
	}

	outcome := <-done
	if outcome.err != nil {
		t.Fatalf("executeSingleTurn failed: %v", outcome.err)
	}
	if outcome.result.Content != "done" {
		t.Fatalf("content = %q, want done", outcome.result.Content)
	}
	transcripts = backend.Snapshot()
	if len(transcripts) < 3 {
		t.Fatalf("expected tool streaming plus final assistant transcript, got %d rows", len(transcripts))
	}
	last := transcripts[len(transcripts)-1]
	if last.Type != "assistant" {
		t.Fatalf("final transcript type = %q, want assistant", last.Type)
	}
}

func TestCodexExecutor_ExecuteSingleTurn_PersistsLiveSessionID(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fake-codex-session.sh")
	script := `#!/bin/sh
echo '{"type":"thread.started","thread_id":"sess-live-123"}'
sleep 0.2
echo '{"type":"turn.completed","output":[{"text":"final answer"}],"turn_usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex script: %v", err)
	}

	taskRecord := task.NewProtoTask("TASK-LIVE-001", "live codex session")
	task.StartPhaseProto(taskRecord.Execution, "implement")
	backend := &codexSessionBackend{
		mockTranscriptBackend: mockTranscriptBackend{},
		task:                  taskRecord,
	}

	exec := NewCodexExecutor(
		WithCodexPath(scriptPath),
		WithCodexWorkdir(tmpDir),
		WithCodexModel("gpt-5.4"),
		WithCodexPhaseID("implement"),
		WithCodexBackend(backend),
		WithCodexTaskID("TASK-LIVE-001"),
	)

	done := make(chan error, 1)
	go func() {
		_, err := exec.executeSingleTurn(context.Background(), "do the thing", "", time.Now())
		done <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := sessionIDFromMetadata(t, task.GetPhaseSessionMetadataProto(backend.task, "implement")); got == "sess-live-123" {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}

	if got := sessionIDFromMetadata(t, task.GetPhaseSessionMetadataProto(backend.task, "implement")); got != "sess-live-123" {
		t.Fatalf("live phase session id = %q, want %q", got, "sess-live-123")
	}

	if err := <-done; err != nil {
		t.Fatalf("executeSingleTurn failed: %v", err)
	}
}

func sessionIDFromMetadata(t *testing.T, metadata string) string {
	t.Helper()
	session, err := llmkit.ParseSessionMetadata(metadata)
	if err != nil {
		t.Fatalf("parse session metadata: %v", err)
	}
	return llmkit.SessionID(session)
}

func TestCodexExecutor_ExecuteSingleTurn_StallReturnsToolFailureContext(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fake-codex-stall.sh")
	script := `#!/bin/sh
cat <<'JSON'
{"type":"thread.started","thread_id":"sess-stall-123"}
{"type":"item.started","item":{"id":"item_0","type":"command_execution","command":"golangci-lint run","aggregated_output":"","exit_code":null,"status":"in_progress"}}
{"type":"item.completed","item":{"id":"item_0","type":"command_execution","command":"golangci-lint run","aggregated_output":"typecheck failed\n","exit_code":1,"status":"completed"}}
JSON
# Stay alive long enough for the inactivity watchdog to cancel the stream.
sleep 30
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex script: %v", err)
	}

	backend := &mockTranscriptBackend{}
	exec := NewCodexExecutor(
		WithCodexPath(scriptPath),
		WithCodexWorkdir(tmpDir),
		WithCodexModel("gpt-5.4"),
		WithCodexPhaseID("implement"),
		WithCodexBackend(backend),
		WithCodexTaskID("TASK-001"),
		WithCodexRunID("RUN-001"),
		WithCodexInactivityTimeout(250*time.Millisecond),
	)

	_, err := exec.executeSingleTurn(context.Background(), "do the thing", "", time.Now())
	if err == nil {
		t.Fatal("expected stalled turn error")
	}

	var stalledErr *codexTurnStalledError
	if !errors.As(err, &stalledErr) {
		t.Fatalf("expected codexTurnStalledError, got %T (%v)", err, err)
	}
	if stalledErr.lastToolResult == nil {
		t.Fatal("expected stalled error to include last tool result")
	}
	if stalledErr.lastToolResult.Output != "typecheck failed\n" {
		t.Fatalf("last tool output = %q, want %q", stalledErr.lastToolResult.Output, "typecheck failed\n")
	}

	if len(backend.transcripts) != 3 {
		t.Fatalf("expected prompt, tool call, tool result transcripts only, got %d", len(backend.transcripts))
	}
	if backend.transcripts[2].Type != "tool_result" {
		t.Fatalf("final transcript type = %q, want tool_result", backend.transcripts[2].Type)
	}
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

type codexSessionBackend struct {
	mockTranscriptBackend
	task *orcv1.Task
}

func (b *codexSessionBackend) LoadTask(taskID string) (*orcv1.Task, error) {
	if b.task != nil && b.task.Id == taskID {
		return b.task, nil
	}
	return nil, nil
}

func (b *codexSessionBackend) SaveTask(t *orcv1.Task) error {
	b.task = t
	return nil
}
