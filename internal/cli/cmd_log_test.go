package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
)

// resolveSymlinks returns the canonical path with symlinks resolved.
// On macOS, /var is a symlink to /private/var, which causes path
// comparison issues between temp directories and os.Getwd().
func resolveSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

func TestDisplayFormattedContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string // strings that should appear in output
	}{
		{
			name:     "text block",
			content:  `[{"type": "text", "text": "Hello world"}]`,
			expected: []string{"Hello world"},
		},
		{
			name:     "tool use block",
			content:  `[{"type": "tool_use", "name": "Read", "input": {"file": "test.go"}}]`,
			expected: []string{"Tool: Read", "file"},
		},
		{
			name:     "plain text fallback",
			content:  "Not JSON content",
			expected: []string{"Not JSON content"},
		},
		{
			name:     "multiple text blocks",
			content:  `[{"type": "text", "text": "First"}, {"type": "text", "text": "Second"}]`,
			expected: []string{"First", "Second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			displayFormattedContent(tt.content, transcriptDisplayOptions{})

			_ = w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			for _, expected := range tt.expected {
				if !bytes.Contains([]byte(output), []byte(expected)) {
					t.Errorf("expected output to contain %q, got: %q", expected, output)
				}
			}
		})
	}
}

func TestCollectPhases(t *testing.T) {
	transcripts := []storage.Transcript{
		{Phase: "spec"},
		{Phase: "implement"},
		{Phase: "spec"},
		{Phase: "test"},
		{Phase: "implement"},
	}

	phases := collectPhases(transcripts)

	// Should have 3 unique phases in order of first appearance
	if len(phases) != 3 {
		t.Errorf("expected 3 unique phases, got %d: %v", len(phases), phases)
	}

	// Check order
	expected := []string{"spec", "implement", "test"}
	for i, p := range expected {
		if phases[i] != p {
			t.Errorf("phase[%d] = %q, want %q", i, phases[i], p)
		}
	}
}

func TestDisplaySingleTranscript(t *testing.T) {
	transcript := storage.Transcript{
		Phase:        "implement",
		Type:         "assistant",
		Model:        "claude-sonnet-4",
		Content:      `[{"type": "text", "text": "I will implement this feature."}]`,
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    1705320000000, // 2024-01-15 10:00:00 UTC
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displaySingleTranscript(transcript, transcriptDisplayOptions{useColor: false})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Check for expected elements
	expectations := []string{
		"ASSISTANT",
		"claude-sonnet-4",
		"in:100",
		"out:50",
		"I will implement this feature",
	}

	for _, expected := range expectations {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("expected output to contain %q, got: %q", expected, output)
		}
	}
}

func TestDisplayTranscriptsPhaseHeaders(t *testing.T) {
	transcripts := []storage.Transcript{
		{Phase: "spec", Type: "user", Content: `[{"type": "text", "text": "spec prompt"}]`, Timestamp: 1},
		{Phase: "spec", Type: "assistant", Content: `[{"type": "text", "text": "spec response"}]`, Timestamp: 2},
		{Phase: "implement", Type: "user", Content: `[{"type": "text", "text": "implement prompt"}]`, Timestamp: 3},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displayTranscripts(transcripts, transcriptDisplayOptions{useColor: false})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should have phase headers
	if !bytes.Contains([]byte(output), []byte("─── spec ───")) {
		t.Error("expected spec phase header")
	}
	if !bytes.Contains([]byte(output), []byte("─── implement ───")) {
		t.Error("expected implement phase header")
	}
}

func TestTranscriptDisplayOptionsRaw(t *testing.T) {
	transcript := storage.Transcript{
		Phase:     "test",
		Type:      "assistant",
		Content:   `[{"type": "text", "text": "response text"}]`,
		Timestamp: 1705320000000,
	}

	// Capture stdout with raw option
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displaySingleTranscript(transcript, transcriptDisplayOptions{raw: true})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Raw mode should show the JSON directly
	if !bytes.Contains([]byte(output), []byte(`"type": "text"`)) {
		t.Errorf("raw mode should show JSON content, got: %q", output)
	}
}

func TestNormalizeProjectPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "absolute path",
			path:     "/home/user/repos/project",
			expected: "-home-user-repos-project",
		},
		{
			name:     "worktree path",
			path:     "/home/user/repos/orc/.orc/worktrees/orc-TASK-001",
			expected: "-home-user-repos-orc-.orc-worktrees-orc-TASK-001",
		},
		{
			name:     "already normalized (no leading slash)",
			path:     "home/user/project",
			expected: "-home-user-project",
		},
		{
			name:     "root path",
			path:     "/",
			expected: "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeProjectPath(tt.path)
			if got != tt.expected {
				t.Errorf("normalizeProjectPath(%q) = %q, want %q", tt.path, got, tt.expected)
			}
		})
	}
}

func TestFormatFollowError(t *testing.T) {
	tests := []struct {
		name         string
		taskID       string
		status       state.Status
		constructErr error
		wantContains []string
	}{
		{
			name:         "pending task",
			taskID:       "TASK-001",
			status:       state.StatusPending,
			wantContains: []string{"TASK-001", "has not started yet", "pending"},
		},
		{
			name:         "completed task",
			taskID:       "TASK-002",
			status:       state.StatusCompleted,
			wantContains: []string{"TASK-002", "already completed", "without --follow"},
		},
		{
			name:         "failed task",
			taskID:       "TASK-003",
			status:       state.StatusFailed,
			wantContains: []string{"TASK-003", "has failed", "without --follow"},
		},
		{
			name:         "paused task",
			taskID:       "TASK-004",
			status:       state.StatusPaused,
			wantContains: []string{"TASK-004", "paused", "orc resume"},
		},
		{
			name:         "interrupted task",
			taskID:       "TASK-005",
			status:       state.StatusInterrupted,
			wantContains: []string{"TASK-005", "interrupted", "orc resume"},
		},
		{
			name:         "running task with no error",
			taskID:       "TASK-006",
			status:       state.StatusRunning,
			constructErr: nil,
			wantContains: []string{"TASK-006", "running", "not yet available", "starting"},
		},
		{
			name:         "skipped status",
			taskID:       "TASK-007",
			status:       state.StatusSkipped,
			wantContains: []string{"TASK-007", "skipped"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &state.State{
				TaskID: tt.taskID,
				Status: tt.status,
			}

			err := formatFollowError(tt.taskID, st, tt.constructErr)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errMsg := err.Error()
			for _, want := range tt.wantContains {
				if !strings.Contains(errMsg, want) {
					t.Errorf("error message %q should contain %q", errMsg, want)
				}
			}
		})
	}
}

func TestConstructJSONLPathFallback_NoSessionID(t *testing.T) {
	// Test when no session ID is available
	st := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "",
		Session:      nil,
	}

	path, err := constructJSONLPathFallback("TASK-001", st)
	if err == nil {
		t.Errorf("expected error when no session ID, got path: %q", path)
	}
	if !strings.Contains(err.Error(), "no session ID") {
		t.Errorf("error should mention 'no session ID', got: %q", err.Error())
	}
}

func TestConstructJSONLPathFallback_WithCurrentPhase(t *testing.T) {
	// Test constructing session ID from taskID + currentPhase
	st := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Phases: map[string]*state.PhaseState{
			"implement": {
				SessionID: "test-session-id",
			},
		},
	}

	// This will try to construct a path but it won't exist, so it will error
	// We're testing the session ID retrieval logic
	_, err := constructJSONLPathFallback("TASK-001", st)

	// Should not error with "no session ID" since we have phase session ID
	if err != nil && strings.Contains(err.Error(), "no session ID") {
		t.Errorf("should be able to use phase session ID, got error: %q", err.Error())
	}
	// Expected error is "constructed JSONL path does not exist" or similar
	// because the test path won't exist on disk
}

func TestConstructJSONLPathFallback_WithExplicitSessionID(t *testing.T) {
	// Test with explicit session ID in phase state
	st := &state.State{
		TaskID:       "TASK-001",
		CurrentPhase: "implement",
		Phases: map[string]*state.PhaseState{
			"implement": {
				SessionID: "explicit-session-id",
			},
		},
	}

	// This will try to construct a path but it won't exist, so it will error
	// We're testing the session ID retrieval logic
	_, err := constructJSONLPathFallback("TASK-001", st)

	// Should not error with "no session ID"
	if err != nil && strings.Contains(err.Error(), "no session ID") {
		t.Errorf("should use explicit session ID from phase state, got error: %q", err.Error())
	}
}

func TestFormatFollowError_RunningWithConstructErr(t *testing.T) {
	// Test running task WITH constructErr (different branch than running without error)
	st := &state.State{
		TaskID: "TASK-001",
		Status: state.StatusRunning,
	}

	testErr := fmt.Errorf("test construction error")
	err := formatFollowError("TASK-001", st, testErr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	// Should include the wrapped error
	if !strings.Contains(errMsg, "test construction error") {
		t.Errorf("error message %q should contain wrapped error", errMsg)
	}
	if !strings.Contains(errMsg, "TASK-001") {
		t.Errorf("error message %q should contain task ID", errMsg)
	}
	if !strings.Contains(errMsg, "running") {
		t.Errorf("error message %q should mention running status", errMsg)
	}
}

func TestConstructJSONLPathFallback_PathConstruction(t *testing.T) {
	// Create a temp directory structure to test path construction
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Resolve symlinks in tmpDir - on macOS /var -> /private/var
	// os.Getwd() after chdir returns resolved paths, so we need to match
	tmpDir = resolveSymlinks(t, tmpDir)

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	// Create the expected JSONL file path
	// Path format: ~/.claude/projects/{normalized-workdir}/{sessionId}.jsonl
	normalizedPath := normalizeProjectPath(tmpDir)
	jsonlDir := fmt.Sprintf("%s/.claude/projects/%s", homeDir, normalizedPath)
	if err := os.MkdirAll(jsonlDir, 0755); err != nil {
		t.Fatalf("failed to create jsonl dir: %v", err)
	}

	sessionID := "TASK-TEST-implement"
	jsonlFile := fmt.Sprintf("%s/%s.jsonl", jsonlDir, sessionID)
	if err := os.WriteFile(jsonlFile, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create jsonl file: %v", err)
	}
	defer func() { _ = os.Remove(jsonlFile) }()
	defer func() { _ = os.Remove(jsonlDir) }()

	// Change to temp directory for the test
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Test with state that has CurrentPhase and session ID set
	st := &state.State{
		TaskID:       "TASK-TEST",
		CurrentPhase: "implement",
		Phases: map[string]*state.PhaseState{
			"implement": {
				SessionID: sessionID,
			},
		},
	}

	path, err := constructJSONLPathFallback("TASK-TEST", st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != jsonlFile {
		t.Errorf("got path %q, want %q", path, jsonlFile)
	}
}

func TestConstructJSONLPathFallback_WorktreeExists(t *testing.T) {
	// Create a temp directory structure simulating an orc project with worktree
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Resolve symlinks in tmpDir - on macOS /var -> /private/var
	// os.Getwd() after chdir returns resolved paths, so we need to match
	tmpDir = resolveSymlinks(t, tmpDir)

	// Create worktree directory structure: .orc/worktrees/orc-TASK-WT
	worktreeDir := fmt.Sprintf("%s/.orc/worktrees/orc-TASK-WT", tmpDir)
	if err := os.MkdirAll(worktreeDir, 0755); err != nil {
		t.Fatalf("failed to create worktree dir: %v", err)
	}

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home dir: %v", err)
	}

	// Create the expected JSONL file path for the worktree
	normalizedWorktreePath := normalizeProjectPath(worktreeDir)
	jsonlDir := fmt.Sprintf("%s/.claude/projects/%s", homeDir, normalizedWorktreePath)
	if err := os.MkdirAll(jsonlDir, 0755); err != nil {
		t.Fatalf("failed to create jsonl dir: %v", err)
	}

	sessionID := "TASK-WT-implement"
	jsonlFile := fmt.Sprintf("%s/%s.jsonl", jsonlDir, sessionID)
	// Don't need to create the file - the worktree path returns directly without checking file existence
	defer func() { _ = os.RemoveAll(jsonlDir) }()

	// Change to temp directory for the test (simulating being in the orc project root)
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Test with state that has CurrentPhase and session ID set
	st := &state.State{
		TaskID:       "TASK-WT",
		CurrentPhase: "implement",
		Phases: map[string]*state.PhaseState{
			"implement": {
				SessionID: sessionID,
			},
		},
	}

	// This should find the worktree and construct path using worktree path
	path, err := constructJSONLPathFallback("TASK-WT", st)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if path != jsonlFile {
		t.Errorf("got path %q, want %q", path, jsonlFile)
	}
}

func TestParseTranscriptFilename(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		wantPhase   string
		wantSeq     int
		expectError bool
	}{
		{
			name:        "sequence-phase-iteration format",
			filename:    "02-implement-003.md",
			wantPhase:   "implement",
			wantSeq:     2,
			expectError: false,
		},
		{
			name:        "phase-sequence format",
			filename:    "spec-001.md",
			wantPhase:   "spec",
			wantSeq:     1,
			expectError: false,
		},
		{
			name:        "review with high iteration",
			filename:    "03-review-030.md",
			wantPhase:   "review",
			wantSeq:     3,
			expectError: false,
		},
		{
			name:        "test phase simple format",
			filename:    "test-005.md",
			wantPhase:   "test",
			wantSeq:     5,
			expectError: false,
		},
		{
			name:        "too few parts",
			filename:    "spec.md",
			expectError: true,
		},
		{
			name:        "sequence format with too few parts",
			filename:    "01-implement.md",
			expectError: true,
		},
		{
			name:        "phase format with non-numeric sequence",
			filename:    "spec-abc.md",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phase, seq, err := parseTranscriptFilename(tt.filename)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got phase=%q seq=%d", phase, seq)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if phase != tt.wantPhase {
				t.Errorf("phase = %q, want %q", phase, tt.wantPhase)
			}
			if seq != tt.wantSeq {
				t.Errorf("sequence = %d, want %d", seq, tt.wantSeq)
			}
		})
	}
}

func TestReadFilesystemTranscripts_NoDirectory(t *testing.T) {
	// Create temp orc root
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create .orc but not transcripts directory
	orcDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-001")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("failed to create .orc dir: %v", err)
	}

	// Change directory to tmpDir
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	transcripts, err := readFilesystemTranscripts("TASK-001")
	if err != nil {
		t.Errorf("unexpected error when directory doesn't exist: %v", err)
	}
	if len(transcripts) != 0 {
		t.Errorf("expected empty slice, got %d transcripts", len(transcripts))
	}
}

func TestReadFilesystemTranscripts_EmptyDirectory(t *testing.T) {
	// Create temp orc root with empty transcripts directory
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	transcriptDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-002", "transcripts")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}

	// Change directory to tmpDir
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	transcripts, err := readFilesystemTranscripts("TASK-002")
	if err != nil {
		t.Errorf("unexpected error for empty directory: %v", err)
	}
	if len(transcripts) != 0 {
		t.Errorf("expected empty slice for empty directory, got %d transcripts", len(transcripts))
	}
}

func TestReadFilesystemTranscripts_ValidFiles(t *testing.T) {
	// Create temp orc root with transcript files
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	transcriptDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-003", "transcripts")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}

	// Create test transcript files
	testFiles := map[string]string{
		"spec-001.md":         "# Specification\n\nThis is the spec phase transcript.",
		"02-implement-001.md": "# Implementation\n\nImplementing the feature.",
		"03-review-030.md":    "# Review Round 30\n\nFinal review comments.",
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(transcriptDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test file %s: %v", filename, err)
		}
	}

	// Change directory to tmpDir
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	transcripts, err := readFilesystemTranscripts("TASK-003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 3 transcripts
	if len(transcripts) != 3 {
		t.Fatalf("expected 3 transcripts, got %d", len(transcripts))
	}

	// Verify sorting (spec-001 has seq=1, 02-implement-001 has seq=2, 03-review-030 has seq=3)
	if transcripts[0].Phase != "spec" {
		t.Errorf("first transcript phase = %q, want 'spec'", transcripts[0].Phase)
	}
	if transcripts[1].Phase != "implement" {
		t.Errorf("second transcript phase = %q, want 'implement'", transcripts[1].Phase)
	}
	if transcripts[2].Phase != "review" {
		t.Errorf("third transcript phase = %q, want 'review'", transcripts[2].Phase)
	}

	// Verify content
	if !strings.Contains(transcripts[0].Content, "Specification") {
		t.Errorf("spec transcript content missing expected text")
	}
	if !strings.Contains(transcripts[1].Content, "Implementation") {
		t.Errorf("implement transcript content missing expected text")
	}
	if !strings.Contains(transcripts[2].Content, "Review Round 30") {
		t.Errorf("review transcript content missing expected text")
	}

	// Verify metadata
	if transcripts[0].TaskID != "TASK-003" {
		t.Errorf("task ID = %q, want 'TASK-003'", transcripts[0].TaskID)
	}
	if transcripts[0].Type != "assistant" {
		t.Errorf("type = %q, want 'assistant'", transcripts[0].Type)
	}
}

func TestReadFilesystemTranscripts_IgnoreNonMd(t *testing.T) {
	// Create temp orc root with mixed files
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	transcriptDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-004", "transcripts")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}

	// Create valid .md file
	mdFile := filepath.Join(transcriptDir, "spec-001.md")
	if err := os.WriteFile(mdFile, []byte("# Spec"), 0644); err != nil {
		t.Fatalf("failed to write md file: %v", err)
	}

	// Create non-.md files (should be ignored)
	txtFile := filepath.Join(transcriptDir, "notes.txt")
	if err := os.WriteFile(txtFile, []byte("notes"), 0644); err != nil {
		t.Fatalf("failed to write txt file: %v", err)
	}

	// Create subdirectory (should be ignored)
	subDir := filepath.Join(transcriptDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	// Change directory to tmpDir
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	transcripts, err := readFilesystemTranscripts("TASK-004")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only have 1 transcript (the .md file)
	if len(transcripts) != 1 {
		t.Errorf("expected 1 transcript, got %d", len(transcripts))
	}

	if transcripts[0].Phase != "spec" {
		t.Errorf("phase = %q, want 'spec'", transcripts[0].Phase)
	}
}

func TestReadFilesystemTranscripts_MalformedFilename(t *testing.T) {
	// Create temp orc root with malformed filename
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	transcriptDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-005", "transcripts")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}

	// Create valid file
	validFile := filepath.Join(transcriptDir, "spec-001.md")
	if err := os.WriteFile(validFile, []byte("# Valid"), 0644); err != nil {
		t.Fatalf("failed to write valid file: %v", err)
	}

	// Create malformed filename (should be skipped with warning)
	malformedFile := filepath.Join(transcriptDir, "invalid.md")
	if err := os.WriteFile(malformedFile, []byte("# Malformed"), 0644); err != nil {
		t.Fatalf("failed to write malformed file: %v", err)
	}

	// Change directory to tmpDir
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Capture stderr to verify warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	transcripts, err := readFilesystemTranscripts("TASK-005")

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	stderrOutput := buf.String()

	// Should not error
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 1 valid transcript (malformed file skipped)
	if len(transcripts) != 1 {
		t.Errorf("expected 1 transcript (malformed skipped), got %d", len(transcripts))
	}

	// Should have warning in stderr
	if !strings.Contains(stderrOutput, "Warning") || !strings.Contains(stderrOutput, "invalid.md") {
		t.Errorf("expected warning about malformed file in stderr, got: %q", stderrOutput)
	}
}

func TestReadFilesystemTranscripts_PermissionDenied(t *testing.T) {
	// Skip on Windows (permissions work differently)
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		t.Skip("skipping permission test on Windows")
	}

	// Create temp orc root
	tmpDir, err := os.MkdirTemp("", "orc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	transcriptDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-006", "transcripts")
	if err := os.MkdirAll(transcriptDir, 0755); err != nil {
		t.Fatalf("failed to create transcript dir: %v", err)
	}

	// Remove read permissions
	if err := os.Chmod(transcriptDir, 0000); err != nil {
		t.Fatalf("failed to chmod: %v", err)
	}
	// Restore permissions in cleanup
	defer func() { _ = os.Chmod(transcriptDir, 0755) }()

	// Change directory to tmpDir
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	_, err = readFilesystemTranscripts("TASK-006")
	if err == nil {
		t.Error("expected permission error, got nil")
	}
	if !strings.Contains(err.Error(), "read transcripts directory") {
		t.Errorf("error should mention 'read transcripts directory', got: %q", err.Error())
	}
}
