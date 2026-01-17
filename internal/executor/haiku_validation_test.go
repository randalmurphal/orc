package executor

import "testing"

func TestParseValidationResponse(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		wantDecision ValidationDecision
		wantReason   string
	}{
		{
			name:         "continue with reason",
			content:      "CONTINUE\nMaking good progress on the authentication flow.",
			wantDecision: ValidationContinue,
			wantReason:   "Making good progress on the authentication flow.",
		},
		{
			name:         "continue uppercase only",
			content:      "CONTINUE",
			wantDecision: ValidationContinue,
			wantReason:   "",
		},
		{
			name:         "continue lowercase",
			content:      "continue\nall good",
			wantDecision: ValidationContinue,
			wantReason:   "all good",
		},
		{
			name:         "retry with reason",
			content:      "RETRY\nThe agent is implementing a REST API but the spec calls for GraphQL.",
			wantDecision: ValidationRetry,
			wantReason:   "The agent is implementing a REST API but the spec calls for GraphQL.",
		},
		{
			name:         "retry mixed case",
			content:      "Retry\nWrong approach.",
			wantDecision: ValidationRetry,
			wantReason:   "Wrong approach.",
		},
		{
			name:         "stop with reason",
			content:      "STOP\nThe spec requires a third-party service that doesn't exist.",
			wantDecision: ValidationStop,
			wantReason:   "The spec requires a third-party service that doesn't exist.",
		},
		{
			name:         "empty content defaults to continue",
			content:      "",
			wantDecision: ValidationContinue,
			wantReason:   "",
		},
		{
			name:         "whitespace only defaults to continue",
			content:      "   \n  ",
			wantDecision: ValidationContinue,
			wantReason:   "",
		},
		{
			name:         "unknown response defaults to continue",
			content:      "UNKNOWN\nSomething weird happened.",
			wantDecision: ValidationContinue,
			wantReason:   "",
		},
		{
			name:         "multiline reason",
			content:      "RETRY\nFirst line.\nSecond line.",
			wantDecision: ValidationRetry,
			wantReason:   "First line.\nSecond line.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, reason := parseValidationResponse(tt.content)
			if decision != tt.wantDecision {
				t.Errorf("parseValidationResponse() decision = %v, want %v", decision, tt.wantDecision)
			}
			if reason != tt.wantReason {
				t.Errorf("parseValidationResponse() reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestParseReadinessResponse(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		wantReady      bool
		wantSuggCount  int
		wantFirstSugg  string
	}{
		{
			name:          "ready response",
			content:       "READY",
			wantReady:     true,
			wantSuggCount: 0,
		},
		{
			name:          "ready lowercase",
			content:       "ready",
			wantReady:     true,
			wantSuggCount: 0,
		},
		{
			name:           "not ready with suggestions",
			content:        "NOT READY\n- Success criteria are vague\n- No testing section",
			wantReady:      false,
			wantSuggCount:  2,
			wantFirstSugg:  "Success criteria are vague",
		},
		{
			name:           "not ready with asterisk bullets",
			content:        "NOT READY\n* First issue\n* Second issue",
			wantReady:      false,
			wantSuggCount:  2,
			wantFirstSugg:  "First issue",
		},
		{
			name:          "empty content defaults to ready",
			content:       "",
			wantReady:     true,
			wantSuggCount: 0,
		},
		{
			name:           "not ready mixed bullets",
			content:        "NOT READY\n- Dash bullet\n* Star bullet\n- Another dash",
			wantReady:      false,
			wantSuggCount:  3,
			wantFirstSugg:  "Dash bullet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ready, suggestions, err := parseReadinessResponse(tt.content)
			if err != nil {
				t.Fatalf("parseReadinessResponse() error = %v", err)
			}
			if ready != tt.wantReady {
				t.Errorf("parseReadinessResponse() ready = %v, want %v", ready, tt.wantReady)
			}
			if len(suggestions) != tt.wantSuggCount {
				t.Errorf("parseReadinessResponse() suggestion count = %d, want %d", len(suggestions), tt.wantSuggCount)
			}
			if tt.wantSuggCount > 0 && len(suggestions) > 0 && suggestions[0] != tt.wantFirstSugg {
				t.Errorf("parseReadinessResponse() first suggestion = %q, want %q", suggestions[0], tt.wantFirstSugg)
			}
		})
	}
}

func TestValidationDecision_String(t *testing.T) {
	tests := []struct {
		decision ValidationDecision
		want     string
	}{
		{ValidationContinue, "continue"},
		{ValidationRetry, "retry"},
		{ValidationStop, "stop"},
		{ValidationDecision(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.decision.String(); got != tt.want {
				t.Errorf("ValidationDecision.String() = %v, want %v", got, tt.want)
			}
		})
	}
}
