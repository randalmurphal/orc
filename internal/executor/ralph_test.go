package executor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRalphStateManager_Create(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr := NewRalphStateManager(dir)

	prompt := "Implement the feature according to the spec."
	err := mgr.Create("TASK-001", "implement", prompt)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify file exists
	path := filepath.Join(dir, RalphStateDir, RalphStateFile)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("State file was not created")
	}

	// Load and verify
	state, loadedPrompt, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state.TaskID != "TASK-001" {
		t.Errorf("TaskID = %q, want %q", state.TaskID, "TASK-001")
	}
	if state.Phase != "implement" {
		t.Errorf("Phase = %q, want %q", state.Phase, "implement")
	}
	if state.Iteration != 1 {
		t.Errorf("Iteration = %d, want %d", state.Iteration, 1)
	}
	if state.MaxIterations != DefaultMaxIterations {
		t.Errorf("MaxIterations = %d, want %d", state.MaxIterations, DefaultMaxIterations)
	}
	if state.CompletionPromise != DefaultCompletionPromise {
		t.Errorf("CompletionPromise = %q, want %q", state.CompletionPromise, DefaultCompletionPromise)
	}
	if loadedPrompt != prompt {
		t.Errorf("Prompt = %q, want %q", loadedPrompt, prompt)
	}
}

func TestRalphStateManager_CreateWithOptions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr := NewRalphStateManager(dir)

	prompt := "Test prompt"
	err := mgr.Create("TASK-002", "test", prompt,
		WithMaxIterations(50),
		WithCompletionPromise("TESTS_PASS"),
		WithSessionID("session-abc"),
	)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	state, _, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state.MaxIterations != 50 {
		t.Errorf("MaxIterations = %d, want %d", state.MaxIterations, 50)
	}
	if state.CompletionPromise != "TESTS_PASS" {
		t.Errorf("CompletionPromise = %q, want %q", state.CompletionPromise, "TESTS_PASS")
	}
	if state.SessionID != "session-abc" {
		t.Errorf("SessionID = %q, want %q", state.SessionID, "session-abc")
	}
}

func TestRalphStateManager_IncrementIteration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr := NewRalphStateManager(dir)

	err := mgr.Create("TASK-001", "implement", "prompt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Increment multiple times
	for i := 2; i <= 5; i++ {
		if err := mgr.IncrementIteration(); err != nil {
			t.Fatalf("IncrementIteration failed: %v", err)
		}

		state, _, err := mgr.Load()
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if state.Iteration != i {
			t.Errorf("After increment %d: Iteration = %d, want %d", i-1, state.Iteration, i)
		}
	}
}

func TestRalphStateManager_UpdateSessionID(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr := NewRalphStateManager(dir)

	err := mgr.Create("TASK-001", "implement", "prompt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update session ID
	if err := mgr.UpdateSessionID("new-session-123"); err != nil {
		t.Fatalf("UpdateSessionID failed: %v", err)
	}

	state, _, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state.SessionID != "new-session-123" {
		t.Errorf("SessionID = %q, want %q", state.SessionID, "new-session-123")
	}
}

func TestRalphStateManager_Remove(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr := NewRalphStateManager(dir)

	err := mgr.Create("TASK-001", "implement", "prompt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if !mgr.Exists() {
		t.Fatal("State file should exist after Create")
	}

	if err := mgr.Remove(); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if mgr.Exists() {
		t.Fatal("State file should not exist after Remove")
	}

	// Remove again should not error
	if err := mgr.Remove(); err != nil {
		t.Fatalf("Second Remove failed: %v", err)
	}
}

func TestRalphStateManager_LoadNonExistent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr := NewRalphStateManager(dir)

	state, prompt, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load should not error for non-existent: %v", err)
	}
	if state != nil {
		t.Error("State should be nil for non-existent file")
	}
	if prompt != "" {
		t.Error("Prompt should be empty for non-existent file")
	}
}

func TestRalphStateManager_PromptPreservation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr := NewRalphStateManager(dir)

	// Prompt with various special characters
	prompt := `# Implementation Task

You need to implement the following:
- Feature A with "quotes"
- Feature B with 'apostrophes'
- Feature C with ---dashes---

## Code Example

` + "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```" + `

Complete when done.`

	err := mgr.Create("TASK-001", "implement", prompt)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Increment to verify prompt is preserved
	if err := mgr.IncrementIteration(); err != nil {
		t.Fatalf("IncrementIteration failed: %v", err)
	}

	_, loadedPrompt, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loadedPrompt != prompt {
		t.Errorf("Prompt not preserved.\nGot:\n%s\n\nWant:\n%s", loadedPrompt, prompt)
	}
}

func TestRalphStateManager_StartedAt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mgr := NewRalphStateManager(dir)

	before := time.Now()
	err := mgr.Create("TASK-001", "implement", "prompt")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	after := time.Now()

	state, _, err := mgr.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if state.StartedAt.Before(before) || state.StartedAt.After(after) {
		t.Errorf("StartedAt = %v, should be between %v and %v", state.StartedAt, before, after)
	}
}

func TestIsOrcWorktree(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want bool
	}{
		{"/home/user/project/.orc/worktrees/orc-TASK-001", true},
		{"/home/user/project/.orc/worktrees/orc-TASK-001/src", true},
		{"/home/user/project/.orc/worktrees/orc-TEST-123/deep/nested/path", true},
		{"/home/user/project", false},
		{"/home/user/project/.orc/tasks/TASK-001", false},
		{"/home/user/project/.orc/worktrees", false},
		{"/home/user/.orc/worktrees/something", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := IsOrcWorktree(tt.path)
			if got != tt.want {
				t.Errorf("IsOrcWorktree(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestExtractTaskIDFromWorktree(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/project/.orc/worktrees/orc-TASK-001", "TASK-001"},
		{"/home/user/project/.orc/worktrees/orc-TASK-001/src/main.go", "TASK-001"},
		{"/home/user/project/.orc/worktrees/orc-TEST-123", "TEST-123"},
		{"/home/user/project", ""},
		{"/home/user/project/.orc/tasks/TASK-001", ""},
		{"/home/user/project/.orc/worktrees", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := ExtractTaskIDFromWorktree(tt.path)
			if got != tt.want {
				t.Errorf("ExtractTaskIDFromWorktree(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestParseRalphFile_InvalidFormats(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
	}{
		{"no frontmatter", "just content"},
		{"missing start delimiter", "task_id: x\n---\ncontent"},
		{"unclosed frontmatter", "---\ntask_id: x\ncontent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseRalphFile(tt.content)
			if err == nil {
				t.Errorf("parseRalphFile should error for %q", tt.name)
			}
		})
	}
}
