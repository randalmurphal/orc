// Package task provides retry state helpers for task execution.
// Retry state is stored in task metadata as JSON under the key "_retry_state".
package task

import (
	"encoding/json"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// RetryState holds retry information stored in task metadata.
// This replaces the removed RetryContext proto field (DEC-005).
type RetryState struct {
	FromPhase     string `json:"from_phase"`
	ToPhase       string `json:"to_phase"`
	Reason        string `json:"reason"`
	FailureOutput string `json:"failure_output,omitempty"`
	Attempt       int32  `json:"attempt"`
}

// retryStateKey is the metadata key used to store retry state.
const retryStateKey = "_retry_state"

// SetRetryState stores retry state in the task's metadata.
// This replaces the removed SetRetryContextProto function.
func SetRetryState(t *orcv1.Task, fromPhase, toPhase, reason, failureOutput string, attempt int32) {
	if t == nil {
		return
	}
	if t.Metadata == nil {
		t.Metadata = make(map[string]string)
	}
	rs := RetryState{
		FromPhase:     fromPhase,
		ToPhase:       toPhase,
		Reason:        reason,
		FailureOutput: failureOutput,
		Attempt:       attempt,
	}
	data, err := json.Marshal(rs)
	if err != nil {
		return
	}
	t.Metadata[retryStateKey] = string(data)
}

// GetRetryState returns the retry state from task metadata, or nil if not set.
// This replaces the removed GetRetryContextProto function.
func GetRetryState(t *orcv1.Task) *RetryState {
	if t == nil || t.Metadata == nil {
		return nil
	}
	js, ok := t.Metadata[retryStateKey]
	if !ok || js == "" {
		return nil
	}
	var rs RetryState
	if err := json.Unmarshal([]byte(js), &rs); err != nil {
		return nil
	}
	return &rs
}

// ClearRetryState removes retry state from task metadata.
func ClearRetryState(t *orcv1.Task) {
	if t == nil || t.Metadata == nil {
		return
	}
	delete(t.Metadata, retryStateKey)
}

// GetRetryStateJSON returns the raw JSON retry state string from task metadata.
// This is useful for proto_convert to sync with db.Task.RetryContext.
func GetRetryStateJSON(t *orcv1.Task) string {
	if t == nil || t.Metadata == nil {
		return ""
	}
	return t.Metadata[retryStateKey]
}

// SetRetryStateJSON sets the raw JSON retry state in task metadata.
// This is useful for proto_convert to sync from db.Task.RetryContext.
func SetRetryStateJSON(t *orcv1.Task, jsonStr string) {
	if t == nil || jsonStr == "" {
		return
	}
	if t.Metadata == nil {
		t.Metadata = make(map[string]string)
	}
	t.Metadata[retryStateKey] = jsonStr
}

// ParseRetryStateJSON parses a retry state JSON string.
// Used by PopulateRetryFields and other code that needs to read retry state.
func ParseRetryStateJSON(jsonStr string) *RetryState {
	if jsonStr == "" {
		return nil
	}
	var rs RetryState
	if err := json.Unmarshal([]byte(jsonStr), &rs); err != nil {
		return nil
	}
	return &rs
}
