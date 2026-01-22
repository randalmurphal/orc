package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// testBackend creates a DatabaseBackend for testing
func testBackend(t *testing.T, tmpDir string) *storage.DatabaseBackend {
	backend, err := storage.NewDatabaseBackend(tmpDir, nil)
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	return backend
}

func TestJSONLSyncer_SyncMessages(t *testing.T) {
	t.Parallel()
	// Create temp dir and database
	tmpDir := t.TempDir()
	backend := testBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create task first (transcripts have FK to tasks)
	if err := backend.SaveTask(&task.Task{ID: "TASK-TEST-001", Title: "Test", Status: task.StatusRunning}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Create syncer
	syncer := NewJSONLSyncer(backend, nil)

	// Create test messages
	messages := []session.JSONLMessage{
		{
			Type:      "user",
			Timestamp: "2024-01-15T10:00:00.000Z",
			SessionID: "sess-test-001",
			UUID:      "msg-001",
			Message: &session.JSONLMessageBody{
				Role:    "user",
				Content: []byte(`[{"type": "text", "text": "Fix the bug"}]`),
			},
		},
		{
			Type:       "assistant",
			Timestamp:  "2024-01-15T10:00:01.000Z",
			SessionID:  "sess-test-001",
			UUID:       "msg-002",
			ParentUUID: strPtr("msg-001"),
			Message: &session.JSONLMessageBody{
				Role:    "assistant",
				Model:   "claude-sonnet-4-20250514",
				Content: []byte(`[{"type": "text", "text": "I'll fix the bug now"}]`),
				Usage: &session.JSONLUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
		},
	}

	// Sync messages
	err := syncer.SyncMessages(context.Background(), messages, SyncOptions{
		TaskID: "TASK-TEST-001",
		Phase:  "implement",
	})
	if err != nil {
		t.Fatalf("SyncMessages failed: %v", err)
	}

	// Verify transcripts were created
	transcripts, err := backend.GetTranscripts("TASK-TEST-001")
	if err != nil {
		t.Fatalf("GetTranscripts failed: %v", err)
	}

	if len(transcripts) != 2 {
		t.Errorf("expected 2 transcripts, got %d", len(transcripts))
	}

	// Verify first transcript
	if transcripts[0].MessageUUID != "msg-001" {
		t.Errorf("expected message UUID msg-001, got %s", transcripts[0].MessageUUID)
	}
	if transcripts[0].Type != "user" {
		t.Errorf("expected type user, got %s", transcripts[0].Type)
	}

	// Verify second transcript has token info
	if transcripts[1].InputTokens != 100 {
		t.Errorf("expected 100 input tokens, got %d", transcripts[1].InputTokens)
	}
	if transcripts[1].Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got %s", transcripts[1].Model)
	}
}

func TestJSONLSyncer_AppendMode(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := testBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create task first (transcripts have FK to tasks)
	if err := backend.SaveTask(&task.Task{ID: "TASK-TEST", Title: "Test", Status: task.StatusRunning}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	syncer := NewJSONLSyncer(backend, nil)

	// First sync
	messages1 := []session.JSONLMessage{
		{
			Type:      "user",
			Timestamp: "2024-01-15T10:00:00.000Z",
			SessionID: "sess-001",
			UUID:      "msg-001",
			Message: &session.JSONLMessageBody{
				Role:    "user",
				Content: []byte(`[{"type": "text", "text": "Hello"}]`),
			},
		},
	}

	err := syncer.SyncMessages(context.Background(), messages1, SyncOptions{
		TaskID: "TASK-TEST",
		Phase:  "implement",
	})
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	// Second sync with append mode (same message + new message)
	messages2 := []session.JSONLMessage{
		{
			Type:      "user",
			Timestamp: "2024-01-15T10:00:00.000Z",
			SessionID: "sess-001",
			UUID:      "msg-001", // Same as before - should be skipped
			Message: &session.JSONLMessageBody{
				Role:    "user",
				Content: []byte(`[{"type": "text", "text": "Hello"}]`),
			},
		},
		{
			Type:      "assistant",
			Timestamp: "2024-01-15T10:00:01.000Z",
			SessionID: "sess-001",
			UUID:      "msg-002", // New message
			Message: &session.JSONLMessageBody{
				Role:    "assistant",
				Content: []byte(`[{"type": "text", "text": "Hi there!"}]`),
			},
		},
	}

	err = syncer.SyncMessages(context.Background(), messages2, SyncOptions{
		TaskID: "TASK-TEST",
		Phase:  "implement",
		Append: true,
	})
	if err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	// Should have 2 transcripts (first one not duplicated)
	transcripts, err := backend.GetTranscripts("TASK-TEST")
	if err != nil {
		t.Fatalf("GetTranscripts failed: %v", err)
	}

	if len(transcripts) != 2 {
		t.Errorf("expected 2 transcripts (deduped), got %d", len(transcripts))
	}
}

func TestJSONLSyncer_SyncFromFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create test JSONL file
	jsonlContent := `{"type":"user","timestamp":"2024-01-15T10:00:00.000Z","sessionId":"sess-001","uuid":"msg-001","message":{"role":"user","content":[{"type":"text","text":"Hello"}]}}
{"type":"assistant","timestamp":"2024-01-15T10:00:01.000Z","sessionId":"sess-001","uuid":"msg-002","message":{"role":"assistant","model":"claude-sonnet-4","content":[{"type":"text","text":"Hi!"}],"usage":{"input_tokens":10,"output_tokens":5}}}
`
	jsonlPath := filepath.Join(tmpDir, "test-session.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(jsonlContent), 0644); err != nil {
		t.Fatalf("failed to write test JSONL: %v", err)
	}

	backend := testBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create task first (transcripts have FK to tasks)
	if err := backend.SaveTask(&task.Task{ID: "TASK-FILE-TEST", Title: "Test", Status: task.StatusRunning}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	syncer := NewJSONLSyncer(backend, nil)

	err := syncer.SyncFromFile(context.Background(), jsonlPath, SyncOptions{
		TaskID: "TASK-FILE-TEST",
		Phase:  "test",
	})
	if err != nil {
		t.Fatalf("SyncFromFile failed: %v", err)
	}

	transcripts, err := backend.GetTranscripts("TASK-FILE-TEST")
	if err != nil {
		t.Fatalf("GetTranscripts failed: %v", err)
	}

	if len(transcripts) != 2 {
		t.Errorf("expected 2 transcripts, got %d", len(transcripts))
	}
}

func TestComputeTokenUsage(t *testing.T) {
	t.Parallel()
	transcripts := []storage.Transcript{
		{Type: "user", InputTokens: 100, OutputTokens: 0},
		{Type: "assistant", InputTokens: 100, OutputTokens: 50, CacheCreationTokens: 10, CacheReadTokens: 80},
		{Type: "assistant", InputTokens: 200, OutputTokens: 100, CacheReadTokens: 150},
	}

	summary := ComputeTokenUsage(transcripts)

	// Should only count assistant messages
	if summary.InputTokens != 300 {
		t.Errorf("expected 300 input tokens, got %d", summary.InputTokens)
	}
	if summary.OutputTokens != 150 {
		t.Errorf("expected 150 output tokens, got %d", summary.OutputTokens)
	}
	if summary.CacheCreationTokens != 10 {
		t.Errorf("expected 10 cache creation tokens, got %d", summary.CacheCreationTokens)
	}
	if summary.CacheReadTokens != 230 {
		t.Errorf("expected 230 cache read tokens, got %d", summary.CacheReadTokens)
	}
	if summary.MessageCount != 2 {
		t.Errorf("expected 2 message count, got %d", summary.MessageCount)
	}
}

func TestParseTimestamp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input string
		want  time.Time
	}{
		{
			input: "2024-01-15T10:00:00.000Z",
			want:  time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			input: "2024-01-15T10:00:00.123456789Z",
			want:  time.Date(2024, 1, 15, 10, 0, 0, 123456789, time.UTC),
		},
	}

	for _, tt := range tests {
		got := parseTimestamp(tt.input)
		if !got.Equal(tt.want) {
			t.Errorf("parseTimestamp(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func strPtr(s string) *string {
	return &s
}

func TestJSONLSyncer_FileNotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := testBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	syncer := NewJSONLSyncer(backend, nil)

	// Try to sync from a non-existent file
	nonExistentPath := filepath.Join(tmpDir, "does-not-exist.jsonl")
	err := syncer.SyncFromFile(context.Background(), nonExistentPath, SyncOptions{
		TaskID: "TASK-001",
		Phase:  "implement",
	})

	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}

	// Verify error contains path information
	if !strings.Contains(err.Error(), "read jsonl file") {
		t.Errorf("expected error to mention 'read jsonl file', got: %v", err)
	}
}

func TestJSONLSyncer_MalformedJSONL(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := testBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create task first (required for FK constraint)
	if err := backend.SaveTask(&task.Task{ID: "TASK-MALFORMED", Title: "Test", Status: task.StatusRunning}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	// Create JSONL file with invalid JSON on some lines
	// The jsonl.ReadFile implementation skips malformed lines rather than failing
	malformedContent := `{"type":"user","timestamp":"2024-01-15T10:00:00.000Z","sessionId":"sess-001","uuid":"msg-001","message":{"role":"user","content":[{"type":"text","text":"Hello"}]}}
{this is not valid json}
{"type":"assistant","timestamp":"2024-01-15T10:00:01.000Z","sessionId":"sess-001","uuid":"msg-002","message":{"role":"assistant","content":[{"type":"text","text":"Hi"}]}}
`
	jsonlPath := filepath.Join(tmpDir, "malformed.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(malformedContent), 0644); err != nil {
		t.Fatalf("failed to write test JSONL: %v", err)
	}

	syncer := NewJSONLSyncer(backend, nil)

	// The jsonl.ReadFile skips malformed lines, so this should succeed
	// with only the valid messages being synced
	err := syncer.SyncFromFile(context.Background(), jsonlPath, SyncOptions{
		TaskID: "TASK-MALFORMED",
		Phase:  "implement",
	})

	if err != nil {
		t.Fatalf("SyncFromFile failed unexpectedly: %v", err)
	}

	// Verify only valid messages were stored (malformed line skipped)
	transcripts, err := backend.GetTranscripts("TASK-MALFORMED")
	if err != nil {
		t.Fatalf("GetTranscripts failed: %v", err)
	}

	// Should have 2 transcripts (the valid user and assistant messages)
	if len(transcripts) != 2 {
		t.Errorf("expected 2 transcripts (malformed line skipped), got %d", len(transcripts))
	}

	// Verify the stored transcripts are the expected ones
	uuids := make(map[string]bool)
	for _, tr := range transcripts {
		uuids[tr.MessageUUID] = true
	}

	if !uuids["msg-001"] {
		t.Error("expected msg-001 (user) to be stored")
	}
	if !uuids["msg-002"] {
		t.Error("expected msg-002 (assistant) to be stored")
	}
}

func TestJSONLSyncer_QueueOperationFiltered(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backend := testBackend(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create task first
	if err := backend.SaveTask(&task.Task{ID: "TASK-QUEUE", Title: "Test", Status: task.StatusRunning}); err != nil {
		t.Fatalf("failed to create task: %v", err)
	}

	syncer := NewJSONLSyncer(backend, nil)

	// Create messages including queue-operation type which should be filtered out
	messages := []session.JSONLMessage{
		{
			Type:      "user",
			Timestamp: "2024-01-15T10:00:00.000Z",
			SessionID: "sess-test-001",
			UUID:      "msg-001",
			Message: &session.JSONLMessageBody{
				Role:    "user",
				Content: []byte(`[{"type": "text", "text": "Start task"}]`),
			},
		},
		{
			Type:      "queue-operation",
			Timestamp: "2024-01-15T10:00:01.000Z",
			SessionID: "sess-test-001",
			UUID:      "msg-002",
			// queue-operation messages are internal bookkeeping, should be skipped
		},
		{
			Type:      "queue-operation",
			Timestamp: "2024-01-15T10:00:02.000Z",
			SessionID: "sess-test-001",
			UUID:      "msg-003",
		},
		{
			Type:      "assistant",
			Timestamp: "2024-01-15T10:00:03.000Z",
			SessionID: "sess-test-001",
			UUID:      "msg-004",
			Message: &session.JSONLMessageBody{
				Role:    "assistant",
				Content: []byte(`[{"type": "text", "text": "Task completed"}]`),
				Usage: &session.JSONLUsage{
					InputTokens:  100,
					OutputTokens: 50,
				},
			},
		},
	}

	// Sync messages
	err := syncer.SyncMessages(context.Background(), messages, SyncOptions{
		TaskID: "TASK-QUEUE",
		Phase:  "implement",
	})
	if err != nil {
		t.Fatalf("SyncMessages failed: %v", err)
	}

	// Verify only non-queue-operation messages were stored
	transcripts, err := backend.GetTranscripts("TASK-QUEUE")
	if err != nil {
		t.Fatalf("GetTranscripts failed: %v", err)
	}

	// Should have 2 transcripts (user + assistant), not 4
	if len(transcripts) != 2 {
		t.Errorf("expected 2 transcripts (queue-operation filtered), got %d", len(transcripts))
	}

	// Verify the stored transcripts are the expected ones
	uuids := make(map[string]bool)
	for _, tr := range transcripts {
		uuids[tr.MessageUUID] = true
		// Verify no queue-operation types were stored
		if tr.Type == "queue-operation" {
			t.Errorf("queue-operation message should not be stored, found UUID: %s", tr.MessageUUID)
		}
	}

	if !uuids["msg-001"] {
		t.Error("expected msg-001 (user) to be stored")
	}
	if !uuids["msg-004"] {
		t.Error("expected msg-004 (assistant) to be stored")
	}
	if uuids["msg-002"] || uuids["msg-003"] {
		t.Error("queue-operation messages should not be stored")
	}
}

func TestComputeJSONLPath(t *testing.T) {
	t.Parallel()
	// Test basic path computation
	path, err := ComputeJSONLPath("/home/user/repos/project", "test-session-123")
	if err != nil {
		t.Fatalf("ComputeJSONLPath failed: %v", err)
	}

	// Should contain normalized path and session ID
	if !strings.Contains(path, "-home-user-repos-project") {
		t.Errorf("path should contain normalized workdir, got: %s", path)
	}
	if !strings.Contains(path, "test-session-123.jsonl") {
		t.Errorf("path should contain session ID, got: %s", path)
	}
	if !strings.Contains(path, ".claude/projects/") {
		t.Errorf("path should be in .claude/projects/, got: %s", path)
	}
}

func TestComputeJSONLPath_EmptySessionID(t *testing.T) {
	t.Parallel()
	_, err := ComputeJSONLPath("/home/user/project", "")
	if err == nil {
		t.Error("ComputeJSONLPath should fail with empty session ID")
	}
}

func TestComputeJSONLPath_DotInPath(t *testing.T) {
	t.Parallel()
	// Claude Code converts dots to dashes in paths (e.g., .orc -> -orc)
	path, err := ComputeJSONLPath("/home/user/repos/orc/.orc/worktrees/orc-TASK-123", "session-abc")
	if err != nil {
		t.Fatalf("ComputeJSONLPath failed: %v", err)
	}

	// .orc should become -orc, resulting in double dash after "orc"
	if !strings.Contains(path, "-home-user-repos-orc--orc-worktrees-orc-TASK-123") {
		t.Errorf("path should have dots converted to dashes, got: %s", path)
	}
}

func TestNormalizeProjectPath(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"/home/user/repos/project", "-home-user-repos-project"},
		{"/tmp/worktree", "-tmp-worktree"},
		{"relative/path", "-relative-path"},
	}

	for _, tt := range tests {
		result := normalizeProjectPath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeProjectPath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
