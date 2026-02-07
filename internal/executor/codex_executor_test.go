package executor

import (
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
	defer func() {
		if rmErr := os.Remove(path); rmErr != nil && !os.IsNotExist(rmErr) {
			t.Fatalf("cleanup schema file: %v", rmErr)
		}
	}()

	// Verify file exists and has correct content
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != schema {
		t.Errorf("schema file content = %q, want %q", string(data), schema)
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

// errString is a simple error type for testing.
type errString string

func (e errString) Error() string { return string(e) }
