package errors

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestOrcErrorFormat(t *testing.T) {
	tests := []struct {
		name     string
		err      *OrcError
		wantErr  string
		wantUser string
	}{
		{
			name:     "what only",
			err:      &OrcError{What: "something broke"},
			wantErr:  "something broke",
			wantUser: "Error: something broke",
		},
		{
			name:     "what and why",
			err:      &OrcError{What: "something broke", Why: "bad input"},
			wantErr:  "something broke: bad input",
			wantUser: "Error: something broke\n\nWhy: bad input",
		},
		{
			name: "full error",
			err: &OrcError{
				What:    "something broke",
				Why:     "bad input",
				Fix:     "try again",
				DocsURL: "https://example.com",
			},
			wantErr:  "something broke: bad input",
			wantUser: "Error: something broke\n\nWhy: bad input\n\nFix: try again\n\nDocs: https://example.com",
		},
		{
			name: "with cause",
			err: &OrcError{
				What:  "something broke",
				Cause: errors.New("underlying error"),
			},
			wantErr:  "something broke: underlying error",
			wantUser: "Error: something broke",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantErr {
				t.Errorf("Error() = %q, want %q", got, tt.wantErr)
			}
			if got := tt.err.UserMessage(); got != tt.wantUser {
				t.Errorf("UserMessage() = %q, want %q", got, tt.wantUser)
			}
		})
	}
}

func TestOrcErrorJSON(t *testing.T) {
	err := &OrcError{
		Code:    CodeTaskNotFound,
		What:    "task TASK-001 not found",
		Why:     "No task with this ID exists",
		Fix:     "Run 'orc status' to list tasks",
		DocsURL: "https://example.com",
		Cause:   errors.New("file not found"),
	}

	data, marshalErr := json.Marshal(err)
	if marshalErr != nil {
		t.Fatalf("MarshalJSON failed: %v", marshalErr)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result["code"] != string(CodeTaskNotFound) {
		t.Errorf("code = %v, want %v", result["code"], CodeTaskNotFound)
	}
	if result["what"] != "task TASK-001 not found" {
		t.Errorf("what = %v, want %v", result["what"], "task TASK-001 not found")
	}
	if result["cause"] != "file not found" {
		t.Errorf("cause = %v, want %v", result["cause"], "file not found")
	}
}

func TestErrNotInitializedError(t *testing.T) {
	err := ErrNotInitialized()

	if err.Code != CodeNotInitialized {
		t.Errorf("Code = %v, want %v", err.Code, CodeNotInitialized)
	}
	if err.What == "" {
		t.Error("What should not be empty")
	}
	if err.Fix == "" {
		t.Error("Fix should not be empty")
	}
}

func TestErrAlreadyInitializedError(t *testing.T) {
	err := ErrAlreadyInitialized("/path/to/.orc")

	if err.Code != CodeAlreadyInitialized {
		t.Errorf("Code = %v, want %v", err.Code, CodeAlreadyInitialized)
	}
	if err.Why == "" {
		t.Error("Why should include the path")
	}
}

func TestErrTaskNotFoundError(t *testing.T) {
	err := ErrTaskNotFound("TASK-123")

	if err.Code != CodeTaskNotFound {
		t.Errorf("Code = %v, want %v", err.Code, CodeTaskNotFound)
	}
	if err.What != "task TASK-123 not found" {
		t.Errorf("What = %v, want 'task TASK-123 not found'", err.What)
	}
}

func TestErrTaskInvalidStateError(t *testing.T) {
	err := ErrTaskInvalidState("TASK-001", "completed", "pending")

	if err.Code != CodeTaskInvalidState {
		t.Errorf("Code = %v, want %v", err.Code, CodeTaskInvalidState)
	}
	if err.What == "" {
		t.Error("What should not be empty")
	}
}

func TestErrTaskRunningError(t *testing.T) {
	err := ErrTaskRunning("TASK-001")

	if err.Code != CodeTaskRunning {
		t.Errorf("Code = %v, want %v", err.Code, CodeTaskRunning)
	}
}

func TestErrClaudeUnavailableError(t *testing.T) {
	err := ErrClaudeUnavailable()

	if err.Code != CodeClaudeUnavailable {
		t.Errorf("Code = %v, want %v", err.Code, CodeClaudeUnavailable)
	}
}

func TestErrClaudeTimeoutError(t *testing.T) {
	err := ErrClaudeTimeout("implement", "30m")

	if err.Code != CodeClaudeTimeout {
		t.Errorf("Code = %v, want %v", err.Code, CodeClaudeTimeout)
	}
	if err.What != "Claude timed out during implement phase" {
		t.Errorf("What = %v, want specific message", err.What)
	}
	if err.Why != "No response received after 30m" {
		t.Errorf("Why = %v, want duration", err.Why)
	}
}

func TestErrPhaseStuckError(t *testing.T) {
	err := ErrPhaseStuck("test", "tests failing repeatedly")

	if err.Code != CodePhaseStuck {
		t.Errorf("Code = %v, want %v", err.Code, CodePhaseStuck)
	}
}

func TestErrMaxRetriesError(t *testing.T) {
	err := ErrMaxRetries("test", 3)

	if err.Code != CodeMaxRetries {
		t.Errorf("Code = %v, want %v", err.Code, CodeMaxRetries)
	}
	if err.What != "phase test failed after 3 attempts" {
		t.Errorf("What = %v, want specific message", err.What)
	}
}

func TestErrConfigInvalidError(t *testing.T) {
	err := ErrConfigInvalid("profile", "must be one of: auto, safe, strict")

	if err.Code != CodeConfigInvalid {
		t.Errorf("Code = %v, want %v", err.Code, CodeConfigInvalid)
	}
}

func TestErrConfigMissingError(t *testing.T) {
	err := ErrConfigMissing("profile")

	if err.Code != CodeConfigMissing {
		t.Errorf("Code = %v, want %v", err.Code, CodeConfigMissing)
	}
}

func TestErrGitDirtyError(t *testing.T) {
	err := ErrGitDirty()

	if err.Code != CodeGitDirty {
		t.Errorf("Code = %v, want %v", err.Code, CodeGitDirty)
	}
}

func TestErrGitBranchExistsError(t *testing.T) {
	err := ErrGitBranchExists("orc/task-001")

	if err.Code != CodeGitBranchExists {
		t.Errorf("Code = %v, want %v", err.Code, CodeGitBranchExists)
	}
}

func TestErrorCodeUniqueness(t *testing.T) {
	codes := []Code{
		CodeNotInitialized,
		CodeAlreadyInitialized,
		CodeTaskNotFound,
		CodeTaskInvalidState,
		CodeTaskRunning,
		CodeClaudeUnavailable,
		CodeClaudeTimeout,
		CodePhaseStuck,
		CodeMaxRetries,
		CodeConfigInvalid,
		CodeConfigMissing,
		CodeGitDirty,
		CodeGitBranchExists,
	}

	seen := make(map[Code]bool)
	for _, code := range codes {
		if seen[code] {
			t.Errorf("duplicate error code: %s", code)
		}
		seen[code] = true
	}
}

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		err        *OrcError
		wantStatus int
	}{
		{ErrNotInitialized(), 400},
		{ErrAlreadyInitialized("/path"), 409},
		{ErrTaskNotFound("X"), 404},
		{ErrTaskInvalidState("X", "a", "b"), 400},
		{ErrTaskRunning("X"), 409},
		{ErrClaudeUnavailable(), 503},
		{ErrClaudeTimeout("x", "1m"), 504},
		{ErrPhaseStuck("x", "y"), 500},
		{ErrMaxRetries("x", 1), 500},
		{ErrConfigInvalid("x", "y"), 400},
		{ErrConfigMissing("x"), 400},
		{ErrGitDirty(), 400},
		{ErrGitBranchExists("x"), 409},
	}

	for _, tt := range tests {
		t.Run(string(tt.err.Code), func(t *testing.T) {
			if got := tt.err.HTTPStatus(); got != tt.wantStatus {
				t.Errorf("HTTPStatus() = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}

func TestUnwrap(t *testing.T) {
	cause := errors.New("underlying error")
	err := ErrTaskNotFound("X").WithCause(cause)

	if errors.Unwrap(err) != cause {
		t.Error("Unwrap should return the cause")
	}
}

func TestWithCause(t *testing.T) {
	original := ErrTaskNotFound("TASK-001")
	cause := errors.New("file not found")
	wrapped := original.WithCause(cause)

	// Wrapped should have cause
	if wrapped.Cause != cause {
		t.Error("WithCause should set the cause")
	}

	// Original should be unchanged
	if original.Cause != nil {
		t.Error("Original should not be modified")
	}

	// All other fields should be copied
	if wrapped.Code != original.Code {
		t.Error("Code should be copied")
	}
	if wrapped.What != original.What {
		t.Error("What should be copied")
	}
}

func TestIs(t *testing.T) {
	err1 := ErrTaskNotFound("TASK-001")
	err2 := ErrTaskNotFound("TASK-002")
	err3 := ErrTaskRunning("TASK-001")

	if !errors.Is(err1, err2) {
		t.Error("errors with same code should match with Is")
	}
	if errors.Is(err1, err3) {
		t.Error("errors with different codes should not match")
	}
}

func TestAsOrcError(t *testing.T) {
	orcErr := ErrTaskNotFound("X")

	// Direct OrcError
	result := AsOrcError(orcErr)
	if result == nil {
		t.Error("AsOrcError should return the error")
	}

	// Wrapped OrcError
	wrapped := orcErr.WithCause(errors.New("cause"))
	result = AsOrcError(wrapped)
	if result == nil {
		t.Error("AsOrcError should return wrapped OrcError")
	}

	// Non-OrcError
	result = AsOrcError(errors.New("regular error"))
	if result != nil {
		t.Error("AsOrcError should return nil for non-OrcError")
	}

	// Nil error
	result = AsOrcError(nil)
	if result != nil {
		t.Error("AsOrcError should return nil for nil error")
	}
}

func TestWrap(t *testing.T) {
	cause := errors.New("underlying")
	err := Wrap(cause, "operation failed")

	if err.What != "operation failed" {
		t.Errorf("What = %v, want 'operation failed'", err.What)
	}
	if err.Cause != cause {
		t.Error("Cause should be set")
	}
	if err.Code != Code("UNKNOWN") {
		t.Errorf("Code = %v, want UNKNOWN", err.Code)
	}
}
