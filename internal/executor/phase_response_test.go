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


