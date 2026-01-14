package api

import (
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/github"
	"github.com/randalmurphal/orc/internal/task"
)

func TestPRPoller_ShouldPoll(t *testing.T) {
	poller := &PRPoller{}

	tests := []struct {
		name     string
		task     *task.Task
		expected bool
	}{
		{
			name:     "no PR info",
			task:     &task.Task{ID: "TASK-001"},
			expected: false,
		},
		{
			name: "empty PR URL",
			task: &task.Task{
				ID: "TASK-001",
				PR: &task.PRInfo{},
			},
			expected: false,
		},
		{
			name: "merged PR",
			task: &task.Task{
				ID: "TASK-001",
				PR: &task.PRInfo{
					URL:    "https://github.com/owner/repo/pull/123",
					Status: task.PRStatusMerged,
				},
			},
			expected: false,
		},
		{
			name: "closed PR",
			task: &task.Task{
				ID: "TASK-001",
				PR: &task.PRInfo{
					URL:    "https://github.com/owner/repo/pull/123",
					Status: task.PRStatusClosed,
				},
			},
			expected: false,
		},
		{
			name: "pending review - should poll",
			task: &task.Task{
				ID: "TASK-001",
				PR: &task.PRInfo{
					URL:    "https://github.com/owner/repo/pull/123",
					Status: task.PRStatusPendingReview,
				},
			},
			expected: true,
		},
		{
			name: "approved - should poll",
			task: &task.Task{
				ID: "TASK-001",
				PR: &task.PRInfo{
					URL:    "https://github.com/owner/repo/pull/123",
					Status: task.PRStatusApproved,
				},
			},
			expected: true,
		},
		{
			name: "recently checked - skip",
			task: &task.Task{
				ID: "TASK-001",
				PR: &task.PRInfo{
					URL:           "https://github.com/owner/repo/pull/123",
					Status:        task.PRStatusPendingReview,
					LastCheckedAt: ptrTime(time.Now().Add(-10 * time.Second)),
				},
			},
			expected: false,
		},
		{
			name: "checked a while ago - should poll",
			task: &task.Task{
				ID: "TASK-001",
				PR: &task.PRInfo{
					URL:           "https://github.com/owner/repo/pull/123",
					Status:        task.PRStatusPendingReview,
					LastCheckedAt: ptrTime(time.Now().Add(-60 * time.Second)),
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := poller.shouldPoll(tt.task)
			if got != tt.expected {
				t.Errorf("shouldPoll() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPRPoller_DeterminePRStatus(t *testing.T) {
	poller := &PRPoller{}

	tests := []struct {
		name     string
		pr       *github.PR
		summary  *github.PRStatusSummary
		expected task.PRStatus
	}{
		{
			name:     "merged PR",
			pr:       &github.PR{State: "MERGED"},
			summary:  &github.PRStatusSummary{ReviewStatus: "approved"},
			expected: task.PRStatusMerged,
		},
		{
			name:     "closed PR",
			pr:       &github.PR{State: "CLOSED"},
			summary:  &github.PRStatusSummary{ReviewStatus: "pending_review"},
			expected: task.PRStatusClosed,
		},
		{
			name:     "draft PR",
			pr:       &github.PR{State: "OPEN", Draft: true},
			summary:  &github.PRStatusSummary{ReviewStatus: "pending_review"},
			expected: task.PRStatusDraft,
		},
		{
			name:     "pending review",
			pr:       &github.PR{State: "OPEN"},
			summary:  &github.PRStatusSummary{ReviewStatus: "pending_review"},
			expected: task.PRStatusPendingReview,
		},
		{
			name:     "approved",
			pr:       &github.PR{State: "OPEN"},
			summary:  &github.PRStatusSummary{ReviewStatus: "approved"},
			expected: task.PRStatusApproved,
		},
		{
			name:     "changes requested",
			pr:       &github.PR{State: "OPEN"},
			summary:  &github.PRStatusSummary{ReviewStatus: "changes_requested"},
			expected: task.PRStatusChangesRequested,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := poller.determinePRStatus(tt.pr, tt.summary)
			if got != tt.expected {
				t.Errorf("determinePRStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewPRPoller(t *testing.T) {
	// Test default interval
	poller := NewPRPoller(PRPollerConfig{
		WorkDir: "/tmp/test",
	})

	if poller.interval != 60*time.Second {
		t.Errorf("default interval = %v, want 60s", poller.interval)
	}
	if poller.workDir != "/tmp/test" {
		t.Errorf("workDir = %s, want /tmp/test", poller.workDir)
	}
	if poller.stopCh == nil {
		t.Error("stopCh should be initialized")
	}

	// Test custom interval
	poller2 := NewPRPoller(PRPollerConfig{
		WorkDir:  "/tmp/test",
		Interval: 30 * time.Second,
	})

	if poller2.interval != 30*time.Second {
		t.Errorf("custom interval = %v, want 30s", poller2.interval)
	}
}

// Helper for pointer to time
func ptrTime(t time.Time) *time.Time {
	return &t
}
