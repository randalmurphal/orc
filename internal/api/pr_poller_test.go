package api

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/randalmurphal/orc/internal/github"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
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

// emptyBackend implements storage.Backend with LoadAllTasks returning empty list.
// Used for testing the poller's stop behavior without real task data.
type emptyBackend struct{}

func (b *emptyBackend) LoadAllTasks() ([]*task.Task, error)                   { return nil, nil }
func (b *emptyBackend) SaveTask(*task.Task) error                             { return nil }
func (b *emptyBackend) LoadTask(string) (*task.Task, error)                   { return nil, nil }
func (b *emptyBackend) DeleteTask(string) error                               { return nil }
func (b *emptyBackend) TaskExists(string) (bool, error)                       { return false, nil }
func (b *emptyBackend) GetNextTaskID() (string, error)                        { return "", nil }
func (b *emptyBackend) SaveState(*state.State) error                          { return nil }
func (b *emptyBackend) LoadState(string) (*state.State, error)                { return nil, nil }
func (b *emptyBackend) LoadAllStates() ([]*state.State, error)                { return nil, nil }
func (b *emptyBackend) SavePlan(*plan.Plan, string) error                     { return nil }
func (b *emptyBackend) LoadPlan(string) (*plan.Plan, error)                   { return nil, nil }
func (b *emptyBackend) SaveSpec(string, string, string) error                 { return nil }
func (b *emptyBackend) LoadSpec(string) (string, error)                       { return "", nil }
func (b *emptyBackend) SpecExists(string) (bool, error)                       { return false, nil }
func (b *emptyBackend) SaveInitiative(*initiative.Initiative) error           { return nil }
func (b *emptyBackend) LoadInitiative(string) (*initiative.Initiative, error) { return nil, nil }
func (b *emptyBackend) LoadAllInitiatives() ([]*initiative.Initiative, error) { return nil, nil }
func (b *emptyBackend) DeleteInitiative(string) error                         { return nil }
func (b *emptyBackend) InitiativeExists(string) (bool, error)                 { return false, nil }
func (b *emptyBackend) GetNextInitiativeID() (string, error)                  { return "", nil }
func (b *emptyBackend) AddTranscript(*storage.Transcript) error               { return nil }
func (b *emptyBackend) GetTranscripts(string) ([]storage.Transcript, error)   { return nil, nil }
func (b *emptyBackend) SearchTranscripts(string) ([]storage.TranscriptMatch, error) {
	return nil, nil
}
func (b *emptyBackend) SaveAttachment(string, string, string, []byte) (*task.Attachment, error) {
	return nil, nil
}
func (b *emptyBackend) GetAttachment(string, string) (*task.Attachment, []byte, error) {
	return nil, nil, nil
}
func (b *emptyBackend) ListAttachments(string) ([]*task.Attachment, error) { return nil, nil }
func (b *emptyBackend) DeleteAttachment(string, string) error              { return nil }
func (b *emptyBackend) MaterializeContext(string, string) error            { return nil }
func (b *emptyBackend) NeedsMaterialization() bool                         { return false }
func (b *emptyBackend) Sync() error                                        { return nil }
func (b *emptyBackend) Cleanup() error                                     { return nil }
func (b *emptyBackend) Close() error                                       { return nil }

func TestPRPoller_StopTwice(t *testing.T) {
	// Create a poller with a backend that returns no tasks
	poller := NewPRPoller(PRPollerConfig{
		WorkDir:  "/tmp/test",
		Interval: time.Hour, // Long interval so it doesn't trigger during test
		Backend:  &emptyBackend{},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller.Start(ctx)

	// First stop should work normally
	poller.Stop()

	// Second stop should not panic (this would panic without sync.Once)
	poller.Stop()

	// Third stop for good measure
	poller.Stop()
}

func TestPRPoller_StopConcurrent(t *testing.T) {
	// Create a poller with a backend that returns no tasks
	poller := NewPRPoller(PRPollerConfig{
		WorkDir:  "/tmp/test",
		Interval: time.Hour, // Long interval so it doesn't trigger during test
		Backend:  &emptyBackend{},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	poller.Start(ctx)

	// Call Stop from multiple goroutines simultaneously
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			poller.Stop()
		}()
	}

	// Wait for all goroutines to complete - this would panic without sync.Once
	wg.Wait()
}
