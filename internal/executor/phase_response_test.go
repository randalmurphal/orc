package executor

import (
	"strings"
	"testing"
)

func TestParsePhaseResponse(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
	tests := []struct {
		name       string
		content    string
		wantStatus PhaseCompletionStatus
		wantReason string
		wantErr    bool
	}{
		{
			name:       "complete",
			content:    `{"status": "complete", "summary": "Done"}`,
			wantStatus: PhaseStatusComplete,
			wantReason: "Done",
			wantErr:    false,
		},
		{
			name:       "blocked",
			content:    `{"status": "blocked", "reason": "Missing file"}`,
			wantStatus: PhaseStatusBlocked,
			wantReason: "Missing file",
			wantErr:    false,
		},
		{
			name:       "continue",
			content:    `{"status": "continue", "reason": "In progress"}`,
			wantStatus: PhaseStatusContinue,
			wantReason: "In progress",
			wantErr:    false,
		},
		{
			name:       "invalid JSON returns error",
			content:    "not json",
			wantStatus: PhaseStatusContinue,
			wantReason: "",
			wantErr:    true,
		},
		{
			name:       "prose instead of JSON returns error",
			content:    "Task complete! I've implemented the feature successfully.",
			wantStatus: PhaseStatusContinue,
			wantReason: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason, err := CheckPhaseCompletionJSON(tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPhaseCompletionJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if status != tt.wantStatus {
				t.Errorf("CheckPhaseCompletionJSON() status = %v, want %v", status, tt.wantStatus)
			}
			if !tt.wantErr && reason != tt.wantReason {
				t.Errorf("CheckPhaseCompletionJSON() reason = %v, want %v", reason, tt.wantReason)
			}
		})
	}
}

func TestHasJSONCompletion(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

func TestParsePhaseSpecificResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		phaseID     string
		reviewRound int
		content     string
		wantStatus  PhaseCompletionStatus
		wantReason  string
		wantErr     bool
	}{
		// Standard phase completion schema
		{
			name:       "standard phase - complete",
			phaseID:    "implement",
			content:    `{"status": "complete", "summary": "Done"}`,
			wantStatus: PhaseStatusComplete,
			wantReason: "Done",
		},
		{
			name:       "standard phase - blocked",
			phaseID:    "implement",
			content:    `{"status": "blocked", "reason": "Missing deps"}`,
			wantStatus: PhaseStatusBlocked,
			wantReason: "Missing deps",
		},
		{
			name:       "standard phase - continue",
			phaseID:    "implement",
			content:    `{"status": "continue", "reason": "In progress"}`,
			wantStatus: PhaseStatusContinue,
			wantReason: "In progress",
		},

		// Review round 1 - ReviewFindingsSchema (status: complete/blocked)
		{
			name:        "review round 1 - valid findings",
			phaseID:     "review",
			reviewRound: 1,
			content:     `{"status": "complete", "round": 1, "summary": "Review complete", "issues": []}`,
			wantStatus:  PhaseStatusComplete,
			wantReason:  "Review complete",
		},
		{
			name:        "review round 1 - findings with issues",
			phaseID:     "review",
			reviewRound: 1,
			content:     `{"status": "complete", "round": 1, "summary": "Found issues", "issues": [{"severity": "high", "description": "Bug found"}]}`,
			wantStatus:  PhaseStatusComplete,
			wantReason:  "Found issues",
		},
		{
			name:        "review round 1 - blocked (no implementation)",
			phaseID:     "review",
			reviewRound: 1,
			content:     `{"status": "blocked", "round": 1, "summary": "No implementation exists to review", "issues": []}`,
			wantStatus:  PhaseStatusBlocked,
			wantReason:  "No implementation exists to review",
		},
		{
			name:        "review round 1 - invalid JSON",
			phaseID:     "review",
			reviewRound: 1,
			content:     `not valid json`,
			wantStatus:  PhaseStatusContinue,
			wantErr:     true,
		},

		// Review round 2 - ReviewDecisionSchema (pass/fail/needs_user_input)
		{
			name:        "review round 2 - pass",
			phaseID:     "review",
			reviewRound: 2,
			content:     `{"status": "pass", "summary": "All good", "gaps_addressed": true, "recommendation": "Merge it"}`,
			wantStatus:  PhaseStatusComplete,
			wantReason:  "All good",
		},
		{
			name:        "review round 2 - fail",
			phaseID:     "review",
			reviewRound: 2,
			content:     `{"status": "fail", "summary": "Needs work", "recommendation": "Fix the bugs"}`,
			wantStatus:  PhaseStatusBlocked,
			wantReason:  "Fix the bugs",
		},
		{
			name:        "review round 2 - needs_user_input",
			phaseID:     "review",
			reviewRound: 2,
			content:     `{"status": "needs_user_input", "summary": "Question", "recommendation": "Ask about X"}`,
			wantStatus:  PhaseStatusBlocked,
			wantReason:  "Ask about X",
		},

		// QA phase - QAResultSchema (pass/fail/needs_attention)
		{
			name:       "qa - pass",
			phaseID:    "qa",
			content:    `{"status": "pass", "summary": "All tests pass", "recommendation": "Ship it"}`,
			wantStatus: PhaseStatusComplete,
			wantReason: "All tests pass",
		},
		{
			name:       "qa - fail",
			phaseID:    "qa",
			content:    `{"status": "fail", "summary": "Tests failed", "recommendation": "Fix tests"}`,
			wantStatus: PhaseStatusBlocked,
			wantReason: "Fix tests",
		},
		{
			name:       "qa - needs_attention",
			phaseID:    "qa",
			content:    `{"status": "needs_attention", "summary": "Low coverage", "recommendation": "Add tests"}`,
			wantStatus: PhaseStatusBlocked,
			wantReason: "Add tests",
		},

		// Unknown/empty phase falls through to standard parsing
		{
			name:       "empty phase - uses standard parsing",
			phaseID:    "",
			content:    `{"status": "complete", "summary": "Done"}`,
			wantStatus: PhaseStatusComplete,
			wantReason: "Done",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, reason, err := ParsePhaseSpecificResponse(tt.phaseID, tt.reviewRound, tt.content)

			if tt.wantErr {
				if err == nil {
					t.Error("ParsePhaseSpecificResponse() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParsePhaseSpecificResponse() unexpected error: %v", err)
				return
			}

			if status != tt.wantStatus {
				t.Errorf("ParsePhaseSpecificResponse() status = %v, want %v", status, tt.wantStatus)
			}
			if reason != tt.wantReason {
				t.Errorf("ParsePhaseSpecificResponse() reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestValidateImplementCompletion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid completion with all checks passing",
			content: `{
				"status": "complete",
				"summary": "Implemented feature",
				"verification": {
					"tests": {"status": "PASS", "evidence": "ok all tests pass"},
					"success_criteria": [
						{"id": "SC-1", "status": "PASS", "evidence": "Test passes"},
						{"id": "SC-2", "status": "PASS", "evidence": "API returns 200"}
					],
					"build": {"status": "PASS"},
					"linting": {"status": "PASS"}
				}
			}`,
			wantErr: false,
		},
		{
			name: "completion without verification - should fail",
			content: `{
				"status": "complete",
				"summary": "Implemented feature"
			}`,
			wantErr: true,
			errMsg:  "completion claimed without verification evidence",
		},
		{
			name: "completion with failing tests - should fail",
			content: `{
				"status": "complete",
				"summary": "Implemented feature",
				"verification": {
					"tests": {"status": "FAIL", "evidence": "1 test failed"},
					"build": {"status": "PASS"}
				}
			}`,
			wantErr: true,
			errMsg:  "tests failed",
		},
		{
			name: "completion with failing success criterion - should fail",
			content: `{
				"status": "complete",
				"summary": "Implemented feature",
				"verification": {
					"tests": {"status": "PASS"},
					"success_criteria": [
						{"id": "SC-1", "status": "PASS"},
						{"id": "SC-2", "status": "FAIL"}
					]
				}
			}`,
			wantErr: true,
			errMsg:  "success criterion SC-2 failed",
		},
		{
			name: "completion with failing build - should fail",
			content: `{
				"status": "complete",
				"summary": "Implemented feature",
				"verification": {
					"tests": {"status": "PASS"},
					"build": {"status": "FAIL"}
				}
			}`,
			wantErr: true,
			errMsg:  "build failed",
		},
		{
			name: "blocked status - no verification required",
			content: `{
				"status": "blocked",
				"reason": "Need clarification"
			}`,
			wantErr: false,
		},
		{
			name: "continue status - no verification required",
			content: `{
				"status": "continue",
				"reason": "Still working"
			}`,
			wantErr: false,
		},
		{
			name: "completion with skipped checks - valid",
			content: `{
				"status": "complete",
				"summary": "Implemented feature",
				"verification": {
					"tests": {"status": "PASS"},
					"build": {"status": "SKIPPED"},
					"linting": {"status": "SKIPPED"}
				}
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateImplementCompletion(tt.content)

			if tt.wantErr {
				if err == nil {
					t.Error("ValidateImplementCompletion() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateImplementCompletion() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("ValidateImplementCompletion() unexpected error: %v", err)
			}
		})
	}
}

func TestGetSchemaForPhaseWithRound_Implement(t *testing.T) {
	t.Parallel()
	schema := GetSchemaForPhaseWithRound("implement", 0)
	if schema != ImplementCompletionSchema {
		t.Error("GetSchemaForPhaseWithRound(implement) should return ImplementCompletionSchema")
	}
}
