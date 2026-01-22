package cli

import (
	"testing"

	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/task"
)

func TestStatusIcon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status   task.Status
		expected string
	}{
		{task.StatusCreated, "📝"},
		{task.StatusClassifying, "🔍"},
		{task.StatusPlanned, "📋"},
		{task.StatusRunning, "⏳"},
		{task.StatusPaused, "⏸️"},
		{task.StatusBlocked, "🚫"},
		{task.StatusFinalizing, "🏁"},
		{task.StatusCompleted, "✅"},
		{task.StatusFailed, "❌"},
		{task.Status("unknown"), "❓"},
	}

	for _, tt := range tests {
		result := statusIcon(tt.status)
		if result != tt.expected {
			t.Errorf("statusIcon(%s) = %s, want %s", tt.status, result, tt.expected)
		}
	}
}

func TestPhaseStatusIcon(t *testing.T) {
	t.Parallel()
	tests := []struct {
		status   executor.PhaseStatus
		expected string
	}{
		{executor.PhasePending, "○"},
		{executor.PhaseRunning, "◐"},
		{executor.PhaseCompleted, "●"},
		{executor.PhaseFailed, "✗"},
		{executor.PhaseSkipped, "⊘"},
		{executor.PhaseStatus("unknown"), "?"},
	}

	for _, tt := range tests {
		result := phaseStatusIcon(tt.status)
		if result != tt.expected {
			t.Errorf("phaseStatusIcon(%s) = %s, want %s", tt.status, result, tt.expected)
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
