package executor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

	taskRecord := task.NewProtoTask("TASK-001", "streaming transcripts")
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

	taskRecord := task.NewProtoTask("TASK-001", "tool streaming transcripts")
	task.StartPhaseProto(taskRecord.Execution, "implement_codex")
	backend := &codexSessionBackend{
		mockTranscriptBackend: mockTranscriptBackend{},
		task:                  taskRecord,
	}
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

func TestCodexExecutor_ExecuteSingleTurn_FailsWhenLiveSessionPersistenceFails(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "fake-codex-session-save-fail.sh")
	script := `#!/bin/sh
echo '{"type":"thread.started","thread_id":"sess-live-123"}'
sleep 0.1
echo '{"type":"turn.completed","output":[{"text":"final answer"}],"turn_usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}'
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake codex script: %v", err)
	}

	taskRecord := task.NewProtoTask("TASK-LIVE-002", "live codex session save failure")
	task.StartPhaseProto(taskRecord.Execution, "implement")
	backend := &codexSessionBackend{
		mockTranscriptBackend: mockTranscriptBackend{},
		task:                  taskRecord,
		saveErr:               errors.New("save failed"),
	}

	exec := NewCodexExecutor(
		WithCodexPath(scriptPath),
		WithCodexWorkdir(tmpDir),
		WithCodexModel("gpt-5.4"),
		WithCodexPhaseID("implement"),
		WithCodexBackend(backend),
		WithCodexTaskID("TASK-LIVE-002"),
	)

	_, err := exec.executeSingleTurn(context.Background(), "do the thing", "", time.Now())
	if err == nil {
		t.Fatal("expected live session persistence failure")
	}
	if !strings.Contains(err.Error(), "persist live codex session metadata") {
		t.Fatalf("error = %v, want live session persistence failure", err)
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

	taskRecord := task.NewProtoTask("TASK-001", "stalled codex session")
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
		WithCodexTaskID("TASK-001"),
		WithCodexRunID("RUN-001"),
		WithCodexInactivityTimeout(750*time.Millisecond),
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

// errString is a simple error type for testing.
type errString string

func (e errString) Error() string { return string(e) }

type codexSessionBackend struct {
	mockTranscriptBackend
	task    *orcv1.Task
	saveErr error
}

func (b *codexSessionBackend) LoadTask(taskID string) (*orcv1.Task, error) {
	if b.task != nil && b.task.Id == taskID {
		return b.task, nil
	}
	return nil, nil
}

func (b *codexSessionBackend) SaveTask(t *orcv1.Task) error {
	if b.saveErr != nil {
		return b.saveErr
	}
	b.task = t
	return nil
}
