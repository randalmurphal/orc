package executor

import (
	"errors"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/task"
)

func TestBuildPRBody_IncludesTaskTitle(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:     "TEST-001",
		Title:  "Implement feature X",
		Weight: "medium",
	}

	body := e.buildPRBody(tsk)

	if !strings.Contains(body, "Implement feature X") {
		t.Errorf("expected body to contain task title, got: %s", body)
	}
	if !strings.Contains(body, "TEST-001") {
		t.Errorf("expected body to contain task ID, got: %s", body)
	}
}

func TestBuildPRBody_IncludesPhases(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:          "TEST-002",
		Title:       "Add new API endpoint",
		Description: "Create POST /api/widgets endpoint with validation",
		Weight:      "large",
	}

	body := e.buildPRBody(tsk)

	// Should include description when present
	if !strings.Contains(body, "Create POST /api/widgets endpoint") {
		t.Errorf("expected body to contain description, got: %s", body)
	}

	// Should include weight
	if !strings.Contains(body, "large") {
		t.Errorf("expected body to contain weight, got: %s", body)
	}

	// Should have standard sections
	if !strings.Contains(body, "## Summary") {
		t.Errorf("expected body to contain Summary section, got: %s", body)
	}
	if !strings.Contains(body, "## Task Details") {
		t.Errorf("expected body to contain Task Details section, got: %s", body)
	}
	if !strings.Contains(body, "## Test Plan") {
		t.Errorf("expected body to contain Test Plan section, got: %s", body)
	}
}

func TestBuildPRBody_UsesDescription(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:          "TEST-003",
		Title:       "Short title",
		Description: "This is a longer description that explains the task in detail",
		Weight:      "small",
	}

	body := e.buildPRBody(tsk)

	// Description should be in summary, not title
	if !strings.Contains(body, "This is a longer description") {
		t.Errorf("expected body to use description in summary, got: %s", body)
	}
}

func TestBuildPRBody_FallsBackToTitle(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:          "TEST-004",
		Title:       "Title only task",
		Description: "", // Empty description
		Weight:      "trivial",
	}

	body := e.buildPRBody(tsk)

	// Should use title when description is empty
	if !strings.Contains(body, "Title only task") {
		t.Errorf("expected body to fall back to title, got: %s", body)
	}
}

func TestBuildPRBody_HasOrcFooter(t *testing.T) {
	e := &Executor{}
	tsk := &task.Task{
		ID:     "TEST-005",
		Title:  "Any task",
		Weight: "small",
	}

	body := e.buildPRBody(tsk)

	if !strings.Contains(body, "Created by [orc]") {
		t.Errorf("expected body to have orc footer, got: %s", body)
	}
	if !strings.Contains(body, "github.com/randalmurphal/orc") {
		t.Errorf("expected body to have orc link, got: %s", body)
	}
}

func TestIsLabelError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "label not found",
			err:      errors.New("could not add label: automated not found"),
			expected: true,
		},
		{
			name:     "label not found uppercase",
			err:      errors.New("Could not add Label: AUTOMATED not found"),
			expected: true,
		},
		{
			name:     "multiple labels not found",
			err:      errors.New("could not add label: bug-fix not found"),
			expected: true,
		},
		{
			name:     "gh cli error with label",
			err:      errors.New("gh pr create: label 'automated' not found: exit status 1"),
			expected: true,
		},
		{
			name:     "unrelated error",
			err:      errors.New("network timeout"),
			expected: false,
		},
		{
			name:     "auth error",
			err:      errors.New("gh: not authenticated"),
			expected: false,
		},
		{
			name:     "branch not found",
			err:      errors.New("branch not found: feature-branch"),
			expected: false,
		},
		{
			name:     "generic not found without label",
			err:      errors.New("repository not found"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isLabelError(tt.err)
			if got != tt.expected {
				t.Errorf("isLabelError(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}
