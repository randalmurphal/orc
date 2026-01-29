package executor

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
)

func TestBuildRetryContext_FormatsCorrectly(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	context := BuildRetryContext("test", "reason", "output", 1, "/path/to/context.md")

	if !strings.Contains(context, "Detailed context saved to: /path/to/context.md") {
		t.Error("context should reference context file when provided")
	}
}

func TestBuildRetryContext_NoContextFile_NoReference(t *testing.T) {
	t.Parallel()
	context := BuildRetryContext("test", "reason", "output", 1, "")

	if strings.Contains(context, "Detailed context saved to:") {
		t.Error("context should not reference context file when not provided")
	}
}

// Tests for enhanced fresh session retry functions

func TestBuildRetryContextForFreshSession_Basic(t *testing.T) {
	t.Parallel()
	opts := RetryOptions{
		FailedPhase:   "test",
		FailureReason: "Tests failed",
		AttemptNumber: 1,
		MaxAttempts:   3,
	}

	context := BuildRetryContextForFreshSession(opts)

	// Check header
	if !strings.Contains(context, "# Retry Context") {
		t.Error("context should have header")
	}
	if !strings.Contains(context, "This is attempt 1 of 3") {
		t.Error("context should show attempt number")
	}

	// Check failure summary
	if !strings.Contains(context, "## Previous Attempt Summary") {
		t.Error("context should have previous attempt section")
	}
	if !strings.Contains(context, "Phase `test` failed") {
		t.Error("context should mention failed phase")
	}
	if !strings.Contains(context, "**Reason:** Tests failed") {
		t.Error("context should include failure reason")
	}

	// Check call to action
	if !strings.Contains(context, "Fix all identified issues") {
		t.Error("context should have call to action")
	}
}

func TestBuildRetryContextForFreshSession_WithFailureOutput(t *testing.T) {
	t.Parallel()
	opts := RetryOptions{
		FailedPhase:   "test",
		FailureOutput: "Error: test assertion failed at line 42",
		AttemptNumber: 2,
		MaxAttempts:   3,
	}

	context := BuildRetryContextForFreshSession(opts)

	if !strings.Contains(context, "### Failure Output") {
		t.Error("context should have failure output section")
	}
	if !strings.Contains(context, "Error: test assertion failed") {
		t.Error("context should include failure output")
	}
}

func TestBuildRetryContextForFreshSession_WithReviewComments(t *testing.T) {
	t.Parallel()
	opts := RetryOptions{
		FailedPhase:   "test",
		AttemptNumber: 1,
		MaxAttempts:   3,
		ReviewComments: []db.ReviewComment{
			{
				FilePath:   "main.go",
				LineNumber: 42,
				Content:    "Missing error handling",
				Severity:   db.SeverityIssue,
			},
			{
				FilePath:   "main.go",
				LineNumber: 100,
				Content:    "Consider using a constant",
				Severity:   db.SeveritySuggestion,
			},
			{
				Content:  "General code quality concern",
				Severity: db.SeverityBlocker,
			},
		},
	}

	context := BuildRetryContextForFreshSession(opts)

	if !strings.Contains(context, "## Review Comments to Address") {
		t.Error("context should have review comments section")
	}
	if !strings.Contains(context, "### General Comments") {
		t.Error("context should have general comments subsection")
	}
	if !strings.Contains(context, "### `main.go`") {
		t.Error("context should group comments by file")
	}
	if !strings.Contains(context, "**Line 42** [ISSUE]") {
		t.Error("context should include line number and severity")
	}
	if !strings.Contains(context, "[BLOCKER]") {
		t.Error("context should include blocker severity")
	}
}

func TestBuildRetryContextForFreshSession_WithPRComments(t *testing.T) {
	t.Parallel()
	opts := RetryOptions{
		FailedPhase:   "implement",
		AttemptNumber: 1,
		MaxAttempts:   3,
		PRComments: []PRCommentFeedback{
			{
				Author:   "reviewer1",
				Body:     "Please add tests for this function",
				FilePath: "handler.go",
				Line:     50,
			},
			{
				Author: "reviewer2",
				Body:   "General feedback:\nLooks good overall",
			},
		},
	}

	context := BuildRetryContextForFreshSession(opts)

	if !strings.Contains(context, "## PR Feedback to Address") {
		t.Error("context should have PR feedback section")
	}
	if !strings.Contains(context, "**handler.go:50** (@reviewer1)") {
		t.Error("context should include file location and author")
	}
	if !strings.Contains(context, "> Please add tests") {
		t.Error("context should quote PR comments")
	}
	if !strings.Contains(context, "**@reviewer2**:") {
		t.Error("context should handle comments without file")
	}
}

func TestBuildRetryContextForFreshSession_WithInstructions(t *testing.T) {
	t.Parallel()
	opts := RetryOptions{
		FailedPhase:   "test",
		AttemptNumber: 1,
		MaxAttempts:   3,
		Instructions:  "Focus on the authentication module first",
	}

	context := BuildRetryContextForFreshSession(opts)

	if !strings.Contains(context, "## Additional Instructions") {
		t.Error("context should have instructions section")
	}
	if !strings.Contains(context, "Focus on the authentication module") {
		t.Error("context should include user instructions")
	}
}

func TestBuildRetryContextForFreshSession_WithPreviousContext(t *testing.T) {
	t.Parallel()
	opts := RetryOptions{
		FailedPhase:     "test",
		AttemptNumber:   2,
		MaxAttempts:     3,
		PreviousContext: "Previous session summary:\n- Phase `implement` was executed\n- Tests were written but failed",
	}

	context := BuildRetryContextForFreshSession(opts)

	if !strings.Contains(context, "## Context from Previous Session") {
		t.Error("context should have previous session section")
	}
	if !strings.Contains(context, "Phase `implement` was executed") {
		t.Error("context should include previous session summary")
	}
}

func TestBuildRetryContextForFreshSession_Full(t *testing.T) {
	t.Parallel()
	opts := RetryOptions{
		FailedPhase:   "test",
		FailureReason: "3 tests failed",
		FailureOutput: "FAIL main_test.go:42",
		ReviewComments: []db.ReviewComment{
			{
				FilePath: "main.go",
				Content:  "Fix this",
				Severity: db.SeverityIssue,
			},
		},
		PRComments: []PRCommentFeedback{
			{
				Author: "bob",
				Body:   "Needs work",
			},
		},
		Instructions:    "Priority: fix tests",
		PreviousContext: "Summary here",
		AttemptNumber:   2,
		MaxAttempts:     3,
	}

	context := BuildRetryContextForFreshSession(opts)

	// All sections should be present
	sections := []string{
		"# Retry Context",
		"## Previous Attempt Summary",
		"### Failure Output",
		"## Review Comments to Address",
		"## PR Feedback to Address",
		"## Additional Instructions",
		"## Context from Previous Session",
	}

	for _, section := range sections {
		if !strings.Contains(context, section) {
			t.Errorf("full context should contain section: %s", section)
		}
	}
}

func TestTruncateOutput(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "short",
			maxLen:   100,
			expected: "short",
		},
		{
			name:     "exact length unchanged",
			input:    "12345",
			maxLen:   5,
			expected: "12345",
		},
		{
			name:   "long string truncated",
			input:  "0123456789",
			maxLen: 5,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := truncateOutput(tc.input, tc.maxLen)
			if tc.expected != "" {
				if result != tc.expected {
					t.Errorf("truncateOutput() = %q, want %q", result, tc.expected)
				}
			} else {
				// For truncated case, verify it's truncated and ends with end of input
				if !strings.HasPrefix(result, "...(truncated)...") {
					t.Error("truncated output should start with truncation marker")
				}
				if len(result) > tc.maxLen+20 { // allow for prefix
					t.Errorf("truncated output too long: %d chars", len(result))
				}
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string unchanged",
			input:    "hello",
			maxLen:   10,
			expected: "hello",
		},
		{
			name:     "exact length unchanged",
			input:    "hello",
			maxLen:   5,
			expected: "hello",
		},
		{
			name:     "long string gets ellipsis",
			input:    "hello world",
			maxLen:   8,
			expected: "hello...",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := truncateString(tc.input, tc.maxLen)
			if result != tc.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tc.input, tc.maxLen, result, tc.expected)
			}
		})
	}
}

func TestCompressPreviousContext_Empty(t *testing.T) {
	t.Parallel()
	result := CompressPreviousContext(nil)
	if result != "" {
		t.Error("empty transcripts should return empty string")
	}

	result = CompressPreviousContext([]db.Transcript{})
	if result != "" {
		t.Error("empty slice should return empty string")
	}
}

func TestCompressPreviousContext_WithPhases(t *testing.T) {
	t.Parallel()
	transcripts := []db.Transcript{
		{Phase: "implement", Content: "Starting implementation"},
		{Phase: "implement", Content: "Added function foo"},
		{Phase: "test", Content: "Running tests"},
		{Phase: "test", Content: "All tests passed"},
	}

	result := CompressPreviousContext(transcripts)

	if !strings.Contains(result, "Previous session summary:") {
		t.Error("should have summary header")
	}
	if !strings.Contains(result, "Phase `implement` was executed") {
		t.Error("should mention implement phase")
	}
	if !strings.Contains(result, "Phase `test` was executed") {
		t.Error("should mention test phase")
	}
}

func TestCompressPreviousContext_WithErrors(t *testing.T) {
	t.Parallel()
	transcripts := []db.Transcript{
		{Phase: "implement", Content: "Starting implementation"},
		{Phase: "test", Content: "Error: test failed at line 42"},
		{Phase: "test", Content: "Error: assertion error in main.go"},
	}

	result := CompressPreviousContext(transcripts)

	if !strings.Contains(result, "Key issues encountered:") {
		t.Error("should have key issues section when errors present")
	}
	if !strings.Contains(result, "Error: test failed") {
		t.Error("should extract error messages")
	}
}

func TestBuildRetryPreview(t *testing.T) {
	t.Parallel()
	opts := RetryOptions{
		FailedPhase: "test",
		ReviewComments: []db.ReviewComment{
			{Content: "Issue 1"},
			{Content: "Issue 2"},
		},
		PRComments: []PRCommentFeedback{
			{Body: "PR comment"},
		},
		AttemptNumber: 1,
		MaxAttempts:   3,
	}

	preview := BuildRetryPreview(opts)

	if preview.CurrentPhase != "test" {
		t.Errorf("CurrentPhase = %q, want %q", preview.CurrentPhase, "test")
	}
	if preview.OpenComments != 2 {
		t.Errorf("OpenComments = %d, want 2", preview.OpenComments)
	}
	if preview.PRComments != 1 {
		t.Errorf("PRComments = %d, want 1", preview.PRComments)
	}
	if preview.ContextPreview == "" {
		t.Error("ContextPreview should not be empty")
	}
	if preview.EstimatedTokens <= 0 {
		t.Error("EstimatedTokens should be positive")
	}
}

func TestFormatReviewCommentsForContext_GroupsByFile(t *testing.T) {
	t.Parallel()
	comments := []db.ReviewComment{
		{FilePath: "a.go", LineNumber: 10, Content: "Issue in a.go", Severity: db.SeverityIssue},
		{FilePath: "b.go", LineNumber: 20, Content: "Issue in b.go", Severity: db.SeverityBlocker},
		{FilePath: "a.go", LineNumber: 30, Content: "Another in a.go", Severity: db.SeveritySuggestion},
		{Content: "General comment", Severity: db.SeverityIssue},
	}

	result := formatReviewCommentsForContext(comments)

	// Check grouping
	if !strings.Contains(result, "### General Comments") {
		t.Error("should have general comments section")
	}
	if !strings.Contains(result, "### `a.go`") {
		t.Error("should have a.go section")
	}
	if !strings.Contains(result, "### `b.go`") {
		t.Error("should have b.go section")
	}

	// Check formatting
	if !strings.Contains(result, "**Line 10** [ISSUE]") {
		t.Error("should format line number and severity")
	}
	if !strings.Contains(result, "[BLOCKER]") {
		t.Error("should uppercase severity")
	}
}

func TestFormatPRCommentsForContext(t *testing.T) {
	t.Parallel()
	comments := []PRCommentFeedback{
		{Author: "alice", Body: "Fix this please", FilePath: "main.go", Line: 42},
		{Author: "bob", Body: "Multi-line\ncomment here"},
	}

	result := formatPRCommentsForContext(comments)

	// Check file-specific comment
	if !strings.Contains(result, "**main.go:42** (@alice)") {
		t.Error("should format file path and author")
	}
	if !strings.Contains(result, "> Fix this please") {
		t.Error("should quote comment body")
	}

	// Check general comment
	if !strings.Contains(result, "**@bob**:") {
		t.Error("should format author for general comment")
	}
	if !strings.Contains(result, "> Multi-line\n> comment here") {
		t.Error("should handle multi-line comments")
	}
}

func TestRetryPreview_Fields(t *testing.T) {
	t.Parallel()
	preview := RetryPreview{
		TaskID:          "TASK-001",
		CurrentPhase:    "test",
		OpenComments:    5,
		PRComments:      2,
		ContextPreview:  "preview here",
		EstimatedTokens: 100,
	}

	if preview.TaskID != "TASK-001" {
		t.Error("TaskID should be set")
	}
	if preview.CurrentPhase != "test" {
		t.Error("CurrentPhase should be set")
	}
	if preview.OpenComments != 5 {
		t.Error("OpenComments should be set")
	}
	if preview.PRComments != 2 {
		t.Error("PRComments should be set")
	}
	if preview.ContextPreview == "" {
		t.Error("ContextPreview should be set")
	}
	if preview.EstimatedTokens != 100 {
		t.Error("EstimatedTokens should be set")
	}
}

func TestIsErrorLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		line     string
		expected bool
	}{
		// True positives - should detect as error
		{"error: prefix", "error: something went wrong", true},
		{"Error: prefix", "Error: something went wrong", true},
		{"error with space prefix", "error something went wrong", true},
		{"colon error colon", "main.go:42: error: undefined", true},
		{"colon error space", "main.go:42: error something", true},
		{"space error colon", "FAIL  error: timeout", true},
		{"tab error colon", "	error: tab prefixed", true},
		{"rust style error", "error[E0425]: cannot find value", true},
		{"failed colon", "command failed: exit code 1", true},
		{"failure colon", "test failure: assertion failed", true},
		{"panic colon", "panic: runtime error", true},
		{"fatal colon", "fatal: not a git repository", true},
		{"uppercase ERROR", "2024-01-01 ERROR something happened", true},
		{"uppercase FAILED", "Test FAILED", true},
		{"uppercase FATAL", "FATAL: database connection lost", true},

		// False positives - should NOT detect as error
		{"no error", "No error occurred", false},
		{"no errors plural", "No errors found in codebase", false},
		{"zero errors", "0 errors, 5 warnings", false},
		{"without error", "completed without error", false},
		{"zero error singular", "0 error found", false},
		{"contains error but negated", "Test passed with zero errors", false},
		{"error in word", "terrorize", false},
		{"error in variable name", "errorHandler := func(){}", false},
		{"normal log message", "processing request 123", false},
		{"success message", "Build completed successfully", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isErrorLine(tc.line)
			if result != tc.expected {
				t.Errorf("isErrorLine(%q) = %v, want %v", tc.line, result, tc.expected)
			}
		})
	}
}

func TestNormalizeSeverity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		expected string
	}{
		{"", "INFO"},
		{"issue", "ISSUE"},
		{"ISSUE", "ISSUE"},
		{"Issue", "ISSUE"},
		{"blocker", "BLOCKER"},
		{"suggestion", "SUGGESTION"},
		{"info", "INFO"},
	}

	for _, tc := range tests {
		t.Run("input_"+tc.input, func(t *testing.T) {
			result := normalizeSeverity(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeSeverity(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestFormatReviewCommentsForContext_EmptySeverity(t *testing.T) {
	t.Parallel()
	comments := []db.ReviewComment{
		{FilePath: "test.go", LineNumber: 10, Content: "Missing something", Severity: ""},
	}

	result := formatReviewCommentsForContext(comments)

	// Should default empty severity to INFO
	if !strings.Contains(result, "[INFO]") {
		t.Error("empty severity should default to INFO, got: " + result)
	}
}

func TestFormatReviewCommentsForContext_DeterministicOrder(t *testing.T) {
	t.Parallel()
	// Create comments for multiple files
	comments := []db.ReviewComment{
		{FilePath: "zebra.go", Content: "Z comment", Severity: "issue"},
		{FilePath: "alpha.go", Content: "A comment", Severity: "issue"},
		{FilePath: "middle.go", Content: "M comment", Severity: "issue"},
	}

	// Run multiple times and verify order is consistent
	var results []string
	for i := 0; i < 5; i++ {
		results = append(results, formatReviewCommentsForContext(comments))
	}

	// All results should be identical
	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("run %d produced different output than run 0", i)
		}
	}

	// Verify alphabetical order
	result := results[0]
	alphaIdx := strings.Index(result, "### `alpha.go`")
	middleIdx := strings.Index(result, "### `middle.go`")
	zebraIdx := strings.Index(result, "### `zebra.go`")

	if alphaIdx == -1 || middleIdx == -1 || zebraIdx == -1 {
		t.Fatal("not all files found in output")
	}
	if alphaIdx > middleIdx || middleIdx > zebraIdx {
		t.Error("files should be in alphabetical order: alpha < middle < zebra")
	}
}

func TestCompressPreviousContext_FalsePositivesFiltered(t *testing.T) {
	t.Parallel()
	transcripts := []db.Transcript{
		{Phase: "test", Content: "No errors found"},
		{Phase: "test", Content: "0 errors, 10 tests passed"},
		{Phase: "test", Content: "completed without error"},
		{Phase: "test", Content: "error: actual error here"},
	}

	result := CompressPreviousContext(transcripts)

	// Should NOT include false positives
	if strings.Contains(result, "No errors found") {
		t.Error("should not include 'No errors found' as key issue")
	}
	if strings.Contains(result, "0 errors") {
		t.Error("should not include '0 errors' as key issue")
	}
	if strings.Contains(result, "without error") {
		t.Error("should not include 'without error' as key issue")
	}

	// Should include actual error
	if !strings.Contains(result, "error: actual error here") {
		t.Error("should include actual error line")
	}
}
