package cli

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

func TestStatusIcon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status   orcv1.TaskStatus
		expected string
	}{
		{orcv1.TaskStatus_TASK_STATUS_CREATED, "📝"},
		{orcv1.TaskStatus_TASK_STATUS_CLASSIFYING, "🔍"},
		{orcv1.TaskStatus_TASK_STATUS_PLANNED, "📋"},
		{orcv1.TaskStatus_TASK_STATUS_RUNNING, "⏳"},
		{orcv1.TaskStatus_TASK_STATUS_PAUSED, "⏸️"},
		{orcv1.TaskStatus_TASK_STATUS_BLOCKED, "🚫"},
		{orcv1.TaskStatus_TASK_STATUS_FINALIZING, "🏁"},
		{orcv1.TaskStatus_TASK_STATUS_COMPLETED, "✅"},
		{orcv1.TaskStatus_TASK_STATUS_FAILED, "❌"},
		{orcv1.TaskStatus(9999), "❓"}, // Unknown status
	}

	for _, tt := range tests {
		result := statusIcon(tt.status)
		if result != tt.expected {
			t.Errorf("statusIcon(%v) = %s, want %s", tt.status, result, tt.expected)
		}
	}
}

func TestPhaseStatusIcon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status   orcv1.PhaseStatus
		expected string
	}{
		{orcv1.PhaseStatus_PHASE_STATUS_PENDING, "○"},
		{orcv1.PhaseStatus_PHASE_STATUS_RUNNING, "◐"},
		{orcv1.PhaseStatus_PHASE_STATUS_COMPLETED, "●"},
		{orcv1.PhaseStatus_PHASE_STATUS_FAILED, "✗"},
		{orcv1.PhaseStatus_PHASE_STATUS_SKIPPED, "⊘"},
		{orcv1.PhaseStatus(9999), "?"}, // Unknown status
	}

	for _, tt := range tests {
		result := phaseStatusIcon(tt.status)
		if result != tt.expected {
			t.Errorf("phaseStatusIcon(%v) = %s, want %s", tt.status, result, tt.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "..."},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}
