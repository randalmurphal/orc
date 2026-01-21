package executor

import (
	"strings"
	"testing"
)

func TestParsePhaseResponse(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantStatus  string
		wantReason  string
		wantSummary string
		wantErr     bool
	}{
		{
			name:        "complete status",
			content:     `{"status": "complete", "summary": "Task completed successfully"}`,
			wantStatus:  "complete",
			wantSummary: "Task completed successfully",
			wantErr:     false,
		},
		{
			name:       "blocked status",
			content:    `{"status": "blocked", "reason": "Missing dependencies"}`,
			wantStatus: "blocked",
			wantReason: "Missing dependencies",
			wantErr:    false,
		},
		{
			name:       "continue status",
			content:    `{"status": "continue", "reason": "Still working on implementation"}`,
			wantStatus: "continue",
			wantReason: "Still working on implementation",
			wantErr:    false,
		},
		{
			name:       "minimal complete",
			content:    `{"status": "complete"}`,
			wantStatus: "complete",
			wantErr:    false,
		},
		{
			name:    "invalid JSON",
			content: `{"status": incomplete`,
			wantErr: true,
		},
		{
			name:    "invalid status value",
			content: `{"status": "done"}`,
			wantErr: true,
		},
		{
			name:    "missing status",
			content: `{"summary": "some work"}`,
			wantErr: true,
		},
		{
			name:    "empty content",
			content: "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ParsePhaseResponse(tt.content)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParsePhaseResponse() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParsePhaseResponse() unexpected error: %v", err)
				return
			}

			if resp.Status != tt.wantStatus {
				t.Errorf("ParsePhaseResponse() status = %v, want %v", resp.Status, tt.wantStatus)
			}
			if resp.Reason != tt.wantReason {
				t.Errorf("ParsePhaseResponse() reason = %v, want %v", resp.Reason, tt.wantReason)
			}
			if resp.Summary != tt.wantSummary {
				t.Errorf("ParsePhaseResponse() summary = %v, want %v", resp.Summary, tt.wantSummary)
			}
		})
	}
}

func TestPhaseResponse_IsComplete(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"complete", "complete", true},
		{"blocked", "blocked", false},
		{"continue", "continue", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &PhaseResponse{Status: tt.status}
			if got := r.IsComplete(); got != tt.want {
				t.Errorf("IsComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPhaseResponse_IsBlocked(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"complete", "complete", false},
		{"blocked", "blocked", true},
		{"continue", "continue", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &PhaseResponse{Status: tt.status}
			if got := r.IsBlocked(); got != tt.want {
				t.Errorf("IsBlocked() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCheckPhaseCompletionJSON(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantStatus PhaseCompletionStatus
		wantReason string
	}{
		{
			name:       "complete",
			content:    `{"status": "complete", "summary": "Done"}`,
			wantStatus: PhaseStatusComplete,
			wantReason: "Done",
		},
		{
			name:       "blocked",
			content:    `{"status": "blocked", "reason": "Missing file"}`,
			wantStatus: PhaseStatusBlocked,
			wantReason: "Missing file",
		},
		{
			name:       "continue",
			content:    `{"status": "continue", "reason": "In progress"}`,
			wantStatus: PhaseStatusContinue,
			wantReason: "In progress",
		},
		{
			name:       "invalid JSON returns continue",
			content:    "not json",
			wantStatus: PhaseStatusContinue,
			wantReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := CheckPhaseCompletionJSON(tt.content)
			if status != tt.wantStatus {
				t.Errorf("CheckPhaseCompletionJSON() status = %v, want %v", status, tt.wantStatus)
			}
			if reason != tt.wantReason {
				t.Errorf("CheckPhaseCompletionJSON() reason = %v, want %v", reason, tt.wantReason)
			}
		})
	}
}

func TestHasJSONCompletion(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"complete", `{"status": "complete"}`, true},
		{"blocked", `{"status": "blocked", "reason": "test"}`, true},
		{"continue", `{"status": "continue"}`, false},
		{"invalid JSON", "not json", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasJSONCompletion(tt.content); got != tt.want {
				t.Errorf("HasJSONCompletion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPhaseResponse_IsContinue(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"complete", "complete", false},
		{"blocked", "blocked", false},
		{"continue", "continue", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &PhaseResponse{Status: tt.status}
			if got := r.IsContinue(); got != tt.want {
				t.Errorf("IsContinue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildJSONRetryPrompt(t *testing.T) {
	invalidContent := "this is not json"
	_, parseErr := ParsePhaseResponse(invalidContent)
	if parseErr == nil {
		t.Fatal("expected parse error for invalid content")
	}

	prompt := BuildJSONRetryPrompt(invalidContent, parseErr)

	// Should contain the schema
	if !strings.Contains(prompt, `"status"`) {
		t.Error("prompt should contain JSON schema")
	}

	// Should contain the error info
	if !strings.Contains(prompt, "Error:") {
		t.Error("prompt should contain error info")
	}

	// Should contain the invalid content
	if !strings.Contains(prompt, invalidContent) {
		t.Error("prompt should contain the invalid content")
	}
}

func TestTruncateForPrompt(t *testing.T) {
	tests := []struct {
		name    string
		content string
		maxLen  int
		want    string
	}{
		{
			name:    "short content unchanged",
			content: "hello",
			maxLen:  10,
			want:    "hello",
		},
		{
			name:    "exact length unchanged",
			content: "hello",
			maxLen:  5,
			want:    "hello",
		},
		{
			name:    "long content truncated",
			content: "hello world",
			maxLen:  5,
			want:    "hello...[truncated]",
		},
		{
			name:    "empty content",
			content: "",
			maxLen:  10,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateForPrompt(tt.content, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateForPrompt() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMightContainPhaseResponse(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "contains status field",
			output: `some text {"status": "complete"} more text`,
			want:   true,
		},
		{
			name:   "contains complete word",
			output: "The task is complete",
			want:   true,
		},
		{
			name:   "contains blocked word",
			output: "The task is blocked by dependencies",
			want:   true,
		},
		{
			name:   "case insensitive",
			output: "COMPLETE the implementation",
			want:   true,
		},
		{
			name:   "no phase-related content",
			output: "just some random text about coding",
			want:   false,
		},
		{
			name:   "empty",
			output: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mightContainPhaseResponse(tt.output); got != tt.want {
				t.Errorf("mightContainPhaseResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractPhaseResponseFromMixed(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantStatus  string
		wantSummary string
		wantReason  string
		wantErr     bool
	}{
		{
			name:        "pure JSON",
			content:     `{"status": "complete", "summary": "Done"}`,
			wantStatus:  "complete",
			wantSummary: "Done",
		},
		{
			name: "JSON in markdown code block",
			content: `Here is my analysis:

The implementation looks good.

` + "```json\n" + `{"status": "complete", "summary": "Review passed"}` + "\n```",
			wantStatus:  "complete",
			wantSummary: "Review passed",
		},
		{
			name: "JSON embedded in text (brace matching)",
			content: `I've completed the review.

All tests pass. No issues found.

{"status": "complete", "summary": "All checks passed"}

That's all for this phase.`,
			wantStatus:  "complete",
			wantSummary: "All checks passed",
		},
		{
			name: "blocked status in code block",
			content: `I cannot proceed.

` + "```json\n" + `{"status": "blocked", "reason": "Missing dependencies"}` + "\n```",
			wantStatus: "blocked",
			wantReason: "Missing dependencies",
		},
		{
			name: "real-world review output (TASK-392 style)",
			content: `**Spec Compliance Check:**

| Success Criterion | Status |
|------------------|--------|
| Migration file exists | ✅ |
| Tests pass | ✅ |

No issues found. The implementation is complete.

` + "```json\n" + `{"status": "complete", "summary": "Review PASSED: No issues found. All success criteria verified."}` + "\n```",
			wantStatus:  "complete",
			wantSummary: "Review PASSED: No issues found. All success criteria verified.",
		},
		{
			name:    "no JSON at all",
			content: "Just some text without any JSON",
			wantErr: true,
		},
		{
			name:    "empty content",
			content: "",
			wantErr: true,
		},
		{
			name: "JSON with nested braces",
			content: `Result: {"status": "complete", "summary": "Created file with content: {\"key\": \"value\"}"}`,
			wantStatus:  "complete",
			wantSummary: `Created file with content: {"key": "value"}`,
		},
		{
			name: "JSON with escaped quotes",
			content: `Output: {"status": "blocked", "reason": "Error: \"file not found\""}`,
			wantStatus: "blocked",
			wantReason: `Error: "file not found"`,
		},
		{
			name: "continue status",
			content: `Still working...

{"status": "continue", "reason": "Processing files"}`,
			wantStatus: "continue",
			wantReason: "Processing files",
		},
		{
			name: "JSON with whitespace inside braces",
			content: `Result: { "status": "complete", "summary": "Done" }`,
			wantStatus:  "complete",
			wantSummary: "Done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ExtractPhaseResponseFromMixed(tt.content)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractPhaseResponseFromMixed() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractPhaseResponseFromMixed() unexpected error: %v", err)
				return
			}

			if resp.Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", resp.Status, tt.wantStatus)
			}
			if resp.Summary != tt.wantSummary {
				t.Errorf("summary = %v, want %v", resp.Summary, tt.wantSummary)
			}
			if resp.Reason != tt.wantReason {
				t.Errorf("reason = %v, want %v", resp.Reason, tt.wantReason)
			}
		})
	}
}

func TestExtractJSONFromCodeBlock(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "json code block",
			content: "text\n```json\n{\"key\": \"value\"}\n```\nmore",
			want:    `{"key": "value"}`,
		},
		{
			name:    "json code block no newline after fence",
			content: "text\n```json{\"key\": \"value\"}\n```",
			want:    `{"key": "value"}`,
		},
		{
			name:    "untyped code block with status",
			content: "text\n```\n{\"status\": \"complete\"}\n```",
			want:    `{"status": "complete"}`,
		},
		{
			name:    "skip typed non-json block",
			content: "```go\nfunc main() {}\n```\n```json\n{\"status\": \"complete\"}\n```",
			want:    `{"status": "complete"}`,
		},
		{
			name:    "no code block",
			content: "just text {\"status\": \"complete\"}",
			want:    "",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "unclosed code block",
			content: "```json\n{\"key\": \"value\"}",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONFromCodeBlock(tt.content)
			if got != tt.want {
				t.Errorf("extractJSONFromCodeBlock() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractJSONByBraceMatching(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "simple JSON with status",
			content: `text {"status": "complete"} more`,
			want:    `{"status": "complete"}`,
		},
		{
			name:    "JSON with whitespace",
			content: `text { "status": "blocked" } more`,
			want:    `{ "status": "blocked" }`,
		},
		{
			name:    "nested braces",
			content: `result: {"status": "complete", "data": {"nested": true}}`,
			want:    `{"status": "complete", "data": {"nested": true}}`,
		},
		{
			name:    "escaped quotes in string",
			content: `{"status": "complete", "msg": "said \"hello\""}`,
			want:    `{"status": "complete", "msg": "said \"hello\""}`,
		},
		{
			name:    "braces inside string",
			content: `{"status": "complete", "code": "if (x) { y }"}`,
			want:    `{"status": "complete", "code": "if (x) { y }"}`,
		},
		{
			name:    "no status pattern",
			content: `{"key": "value"}`,
			want:    "",
		},
		{
			name:    "empty content",
			content: "",
			want:    "",
		},
		{
			name:    "unclosed brace",
			content: `{"status": "complete"`,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONByBraceMatching(tt.content)
			if got != tt.want {
				t.Errorf("extractJSONByBraceMatching() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckPhaseCompletionMixed(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantStatus PhaseCompletionStatus
		wantReason string
	}{
		{
			name:       "pure JSON complete",
			content:    `{"status": "complete", "summary": "Done"}`,
			wantStatus: PhaseStatusComplete,
			wantReason: "Done",
		},
		{
			name: "mixed text with JSON code block",
			content: "Analysis complete.\n```json\n" + `{"status": "complete", "summary": "All good"}` + "\n```",
			wantStatus: PhaseStatusComplete,
			wantReason: "All good",
		},
		{
			name:       "embedded JSON",
			content:    `Here is the result: {"status": "blocked", "reason": "Missing file"} End.`,
			wantStatus: PhaseStatusBlocked,
			wantReason: "Missing file",
		},
		{
			name:       "no JSON returns continue",
			content:    "Just some text without JSON",
			wantStatus: PhaseStatusContinue,
			wantReason: "",
		},
		{
			name:       "empty returns continue",
			content:    "",
			wantStatus: PhaseStatusContinue,
			wantReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason := CheckPhaseCompletionMixed(tt.content)
			if status != tt.wantStatus {
				t.Errorf("CheckPhaseCompletionMixed() status = %v, want %v", status, tt.wantStatus)
			}
			if reason != tt.wantReason {
				t.Errorf("CheckPhaseCompletionMixed() reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

