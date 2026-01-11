package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/state"
)

func TestSaveRetryContextFile_WritesToDisk(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()

	// Save retry context
	path, err := SaveRetryContextFile(tmpDir, "TASK-001", "test", "implement", "Tests failed", "Error: assertion failed", 1)
	if err != nil {
		t.Fatalf("SaveRetryContextFile failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("retry context file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read retry context file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "**From Phase**: test") {
		t.Error("file should contain from phase")
	}
	if !strings.Contains(contentStr, "**To Phase**: implement") {
		t.Error("file should contain to phase")
	}
	if !strings.Contains(contentStr, "**Attempt**: 1") {
		t.Error("file should contain attempt number")
	}
	if !strings.Contains(contentStr, "Tests failed") {
		t.Error("file should contain reason")
	}
	if !strings.Contains(contentStr, "Error: assertion failed") {
		t.Error("file should contain failure output")
	}
}

func TestSaveRetryContextFile_CreatesTaskDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Task directory doesn't exist yet
	taskDir := filepath.Join(tmpDir, ".orc", "tasks", "TASK-002")
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Fatal("task directory should not exist before test")
	}

	// Save retry context should create directory
	_, err := SaveRetryContextFile(tmpDir, "TASK-002", "validate", "implement", "Validation failed", "output", 2)
	if err != nil {
		t.Fatalf("SaveRetryContextFile failed: %v", err)
	}

	// Verify directory was created
	if _, err := os.Stat(taskDir); os.IsNotExist(err) {
		t.Error("task directory was not created")
	}
}

func TestLoadRetryContextForPhase_ReadsFromState(t *testing.T) {
	s := state.New("TASK-001")
	s.SetRetryContext("test", "implement", "Tests failed", "Error output", 1)

	context := LoadRetryContextForPhase(s)

	if context == "" {
		t.Error("LoadRetryContextForPhase should return non-empty context")
	}
	if !strings.Contains(context, "Phase \"test\" failed") {
		t.Error("context should contain failed phase name")
	}
	if !strings.Contains(context, "Tests failed") {
		t.Error("context should contain reason")
	}
	if !strings.Contains(context, "retry attempt #1") {
		t.Error("context should contain attempt number")
	}
	if !strings.Contains(context, "Error output") {
		t.Error("context should contain failure output")
	}
}

func TestLoadRetryContextForPhase_NilState_ReturnsEmpty(t *testing.T) {
	context := LoadRetryContextForPhase(nil)
	if context != "" {
		t.Error("LoadRetryContextForPhase with nil state should return empty string")
	}
}

func TestLoadRetryContextForPhase_NoRetryContext_ReturnsEmpty(t *testing.T) {
	s := state.New("TASK-001")
	// No retry context set

	context := LoadRetryContextForPhase(s)
	if context != "" {
		t.Error("LoadRetryContextForPhase with no retry context should return empty string")
	}
}

func TestBuildRetryContext_FormatsCorrectly(t *testing.T) {
	context := BuildRetryContext("test", "Tests failed: 2 errors", "Error: foo\nError: bar", 1, "")

	// Check structure
	if !strings.Contains(context, "## Retry Context") {
		t.Error("context should have header")
	}
	if !strings.Contains(context, "This phase is being re-executed") {
		t.Error("context should explain retry situation")
	}
	if !strings.Contains(context, "**What happened:**") {
		t.Error("context should have 'What happened' section")
	}
	if !strings.Contains(context, "**What to fix:**") {
		t.Error("context should have 'What to fix' section")
	}
	if !strings.Contains(context, "Focus on fixing the root cause") {
		t.Error("context should contain guidance")
	}
}

func TestBuildRetryContext_IncludesAttemptNumber(t *testing.T) {
	tests := []struct {
		attempt  int
		expected string
	}{
		{1, "retry attempt #1"},
		{2, "retry attempt #2"},
		{3, "retry attempt #3"},
	}

	for _, tc := range tests {
		context := BuildRetryContext("test", "reason", "output", tc.attempt, "")
		if !strings.Contains(context, tc.expected) {
			t.Errorf("attempt %d: expected context to contain %q", tc.attempt, tc.expected)
		}
	}
}

func TestBuildRetryContext_IncludesContextFile(t *testing.T) {
	context := BuildRetryContext("test", "reason", "output", 1, "/path/to/context.md")

	if !strings.Contains(context, "Detailed context saved to: /path/to/context.md") {
		t.Error("context should reference context file when provided")
	}
}

func TestBuildRetryContext_NoContextFile_NoReference(t *testing.T) {
	context := BuildRetryContext("test", "reason", "output", 1, "")

	if strings.Contains(context, "Detailed context saved to:") {
		t.Error("context should not reference context file when not provided")
	}
}

func TestRetryTracker_CanRetry(t *testing.T) {
	tracker := NewRetryTracker(2)

	// First attempt should be allowed
	if !tracker.CanRetry("test") {
		t.Error("should be able to retry initially")
	}

	// Increment once
	tracker.Increment("test")
	if !tracker.CanRetry("test") {
		t.Error("should be able to retry after 1 attempt")
	}

	// Increment again - now at max
	tracker.Increment("test")
	if tracker.CanRetry("test") {
		t.Error("should not be able to retry after reaching max")
	}
}

func TestRetryTracker_Increment(t *testing.T) {
	tracker := NewRetryTracker(3)

	if count := tracker.Increment("test"); count != 1 {
		t.Errorf("first increment should return 1, got %d", count)
	}
	if count := tracker.Increment("test"); count != 2 {
		t.Errorf("second increment should return 2, got %d", count)
	}
}

func TestRetryTracker_GetCount(t *testing.T) {
	tracker := NewRetryTracker(3)

	if count := tracker.GetCount("test"); count != 0 {
		t.Errorf("initial count should be 0, got %d", count)
	}

	tracker.Increment("test")
	tracker.Increment("test")

	if count := tracker.GetCount("test"); count != 2 {
		t.Errorf("count after 2 increments should be 2, got %d", count)
	}
}

func TestRetryTracker_Reset(t *testing.T) {
	tracker := NewRetryTracker(2)

	tracker.Increment("test")
	tracker.Increment("test")
	if tracker.CanRetry("test") {
		t.Error("should not be able to retry at max")
	}

	tracker.Reset("test")
	if !tracker.CanRetry("test") {
		t.Error("should be able to retry after reset")
	}
	if count := tracker.GetCount("test"); count != 0 {
		t.Errorf("count after reset should be 0, got %d", count)
	}
}

func TestRetryTracker_ResetAll(t *testing.T) {
	tracker := NewRetryTracker(2)

	tracker.Increment("test")
	tracker.Increment("validate")

	tracker.ResetAll()

	if count := tracker.GetCount("test"); count != 0 {
		t.Errorf("test count after ResetAll should be 0, got %d", count)
	}
	if count := tracker.GetCount("validate"); count != 0 {
		t.Errorf("validate count after ResetAll should be 0, got %d", count)
	}
}

func TestRetryTracker_IndependentPhases(t *testing.T) {
	tracker := NewRetryTracker(2)

	tracker.Increment("test")
	tracker.Increment("test")
	tracker.Increment("validate")

	if tracker.CanRetry("test") {
		t.Error("test should be at max")
	}
	if !tracker.CanRetry("validate") {
		t.Error("validate should still have retries available")
	}
}

func TestDefaultRetryMap(t *testing.T) {
	retryMap := DefaultRetryMap()

	expected := map[string]string{
		"test":      "implement",
		"test_unit": "implement",
		"test_e2e":  "implement",
		"validate":  "implement",
	}

	for phase, target := range expected {
		if retryMap[phase] != target {
			t.Errorf("retry map[%s] = %q, want %q", phase, retryMap[phase], target)
		}
	}
}

func TestNewRetryTracker_DefaultMaxRetries(t *testing.T) {
	// Zero should use default
	tracker := NewRetryTracker(0)
	tracker.Increment("test")
	tracker.Increment("test")
	if tracker.CanRetry("test") {
		t.Error("default max retries should be 2")
	}

	// Negative should use default
	tracker2 := NewRetryTracker(-1)
	tracker2.Increment("test")
	tracker2.Increment("test")
	if tracker2.CanRetry("test") {
		t.Error("negative max retries should default to 2")
	}
}
