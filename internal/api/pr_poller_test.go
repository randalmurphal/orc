package api

import (
	"context"
	"sync"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/github"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPRPoller_ShouldPoll(t *testing.T) {
	t.Parallel()
	poller := &PRPoller{}

	prURL := "https://github.com/owner/repo/pull/123"
	recentTime := timestamppb.New(time.Now().Add(-10 * time.Second))
	oldTime := timestamppb.New(time.Now().Add(-60 * time.Second))

	tests := []struct {
		name     string
		task     *orcv1.Task
		expected bool
	}{
		{
			name:     "no PR info",
			task:     &orcv1.Task{Id: "TASK-001"},
			expected: false,
		},
		{
			name: "empty PR URL",
			task: &orcv1.Task{
				Id: "TASK-001",
				Pr: &orcv1.PRInfo{},
			},
			expected: false,
		},
		{
			name: "merged PR",
			task: &orcv1.Task{
				Id: "TASK-001",
				Pr: &orcv1.PRInfo{
					Url:    &prURL,
					Status: orcv1.PRStatus_PR_STATUS_MERGED,
				},
			},
			expected: false,
		},
		{
			name: "closed PR",
			task: &orcv1.Task{
				Id: "TASK-001",
				Pr: &orcv1.PRInfo{
					Url:    &prURL,
					Status: orcv1.PRStatus_PR_STATUS_CLOSED,
				},
			},
			expected: false,
		},
		{
			name: "pending review - should poll",
			task: &orcv1.Task{
				Id: "TASK-001",
				Pr: &orcv1.PRInfo{
					Url:    &prURL,
					Status: orcv1.PRStatus_PR_STATUS_PENDING_REVIEW,
				},
			},
			expected: true,
		},
		{
			name: "approved - should poll",
			task: &orcv1.Task{
				Id: "TASK-001",
				Pr: &orcv1.PRInfo{
					Url:    &prURL,
					Status: orcv1.PRStatus_PR_STATUS_APPROVED,
				},
			},
			expected: true,
		},
		{
			name: "recently checked - skip",
			task: &orcv1.Task{
				Id: "TASK-001",
				Pr: &orcv1.PRInfo{
					Url:           &prURL,
					Status:        orcv1.PRStatus_PR_STATUS_PENDING_REVIEW,
					LastCheckedAt: recentTime,
				},
			},
			expected: false,
		},
		{
			name: "checked a while ago - should poll",
			task: &orcv1.Task{
				Id: "TASK-001",
				Pr: &orcv1.PRInfo{
					Url:           &prURL,
					Status:        orcv1.PRStatus_PR_STATUS_PENDING_REVIEW,
					LastCheckedAt: oldTime,
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

func TestDeterminePRStatusProto(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pr       *github.PR
		summary  *github.PRStatusSummary
		expected orcv1.PRStatus
	}{
		{
			name:     "merged PR",
			pr:       &github.PR{State: "MERGED"},
			summary:  &github.PRStatusSummary{ReviewStatus: "approved"},
			expected: orcv1.PRStatus_PR_STATUS_MERGED,
		},
		{
			name:     "closed PR",
			pr:       &github.PR{State: "CLOSED"},
			summary:  &github.PRStatusSummary{ReviewStatus: "pending_review"},
			expected: orcv1.PRStatus_PR_STATUS_CLOSED,
		},
		{
			name:     "draft PR",
			pr:       &github.PR{State: "OPEN", Draft: true},
			summary:  &github.PRStatusSummary{ReviewStatus: "pending_review"},
			expected: orcv1.PRStatus_PR_STATUS_DRAFT,
		},
		{
			name:     "pending review",
			pr:       &github.PR{State: "OPEN"},
			summary:  &github.PRStatusSummary{ReviewStatus: "pending_review"},
			expected: orcv1.PRStatus_PR_STATUS_PENDING_REVIEW,
		},
		{
			name:     "approved",
			pr:       &github.PR{State: "OPEN"},
			summary:  &github.PRStatusSummary{ReviewStatus: "approved"},
			expected: orcv1.PRStatus_PR_STATUS_APPROVED,
		},
		{
			name:     "changes requested",
			pr:       &github.PR{State: "OPEN"},
			summary:  &github.PRStatusSummary{ReviewStatus: "changes_requested"},
			expected: orcv1.PRStatus_PR_STATUS_CHANGES_REQUESTED,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeterminePRStatusProto(tt.pr, tt.summary)
			if got != tt.expected {
				t.Errorf("DeterminePRStatusProto() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewPRPoller(t *testing.T) {
	t.Parallel()
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

// emptyBackend implements storage.Backend with LoadAllTasks returning empty list.
// Used for testing the poller's stop behavior without real task data.
type emptyBackend struct{}

func (b *emptyBackend) LoadAllTasks() ([]*orcv1.Task, error) { return nil, nil }
func (b *emptyBackend) SaveTask(*orcv1.Task) error           { return nil }
func (b *emptyBackend) LoadTask(string) (*orcv1.Task, error) { return nil, nil }
func (b *emptyBackend) DeleteTask(string) error              { return nil }
func (b *emptyBackend) TaskExists(string) (bool, error)     { return false, nil }
func (b *emptyBackend) GetNextTaskID() (string, error)      { return "", nil }
func (b *emptyBackend) GetTaskActivityByDate(string, string) ([]storage.ActivityCount, error) {
	return nil, nil
}
func (b *emptyBackend) UpdateTaskHeartbeat(string) error          { return nil }
func (b *emptyBackend) SetTaskExecutor(string, int, string) error { return nil }
func (b *emptyBackend) ClearTaskExecutor(string) error            { return nil }
// Phase output methods
func (b *emptyBackend) SavePhaseOutput(*storage.PhaseOutputInfo) error                     { return nil }
func (b *emptyBackend) GetPhaseOutput(string, string) (*storage.PhaseOutputInfo, error)   { return nil, nil }
func (b *emptyBackend) GetPhaseOutputByVarName(string, string) (*storage.PhaseOutputInfo, error) { return nil, nil }
func (b *emptyBackend) GetAllPhaseOutputs(string) ([]*storage.PhaseOutputInfo, error)     { return nil, nil }
func (b *emptyBackend) LoadPhaseOutputsAsMap(string) (map[string]string, error)           { return nil, nil }
func (b *emptyBackend) GetPhaseOutputsForTask(string) ([]*storage.PhaseOutputInfo, error) { return nil, nil }
func (b *emptyBackend) DeletePhaseOutput(string, string) error                             { return nil }
func (b *emptyBackend) PhaseOutputExists(string, string) (bool, error)                     { return false, nil }
// Spec methods (now backed by phase_outputs)
func (b *emptyBackend) GetSpecForTask(string) (string, error)                              { return "", nil }
func (b *emptyBackend) GetFullSpecForTask(string) (*storage.PhaseOutputInfo, error)       { return nil, nil }
func (b *emptyBackend) SpecExistsForTask(string) (bool, error)                             { return false, nil }
func (b *emptyBackend) SaveSpecForTask(string, string, string) error                       { return nil }
func (b *emptyBackend) SaveInitiative(*initiative.Initiative) error           { return nil }
func (b *emptyBackend) LoadInitiative(string) (*initiative.Initiative, error) { return nil, nil }
func (b *emptyBackend) LoadAllInitiatives() ([]*initiative.Initiative, error) { return nil, nil }
func (b *emptyBackend) DeleteInitiative(string) error                         { return nil }
func (b *emptyBackend) InitiativeExists(string) (bool, error)                 { return false, nil }
func (b *emptyBackend) GetNextInitiativeID() (string, error)                  { return "", nil }
func (b *emptyBackend) AddTranscript(*storage.Transcript) error               { return nil }
func (b *emptyBackend) AddTranscriptBatch(context.Context, []storage.Transcript) error {
	return nil
}
func (b *emptyBackend) GetTranscripts(string) ([]storage.Transcript, error) { return nil, nil }
func (b *emptyBackend) GetTranscriptsPaginated(string, storage.TranscriptPaginationOpts) ([]storage.Transcript, storage.PaginationResult, error) {
	return nil, storage.PaginationResult{}, nil
}
func (b *emptyBackend) GetPhaseSummary(string) ([]storage.PhaseSummary, error) { return nil, nil }
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
func (b *emptyBackend) SaveBranch(*storage.Branch) error                   { return nil }
func (b *emptyBackend) LoadBranch(string) (*storage.Branch, error) {
	return nil, nil
}
func (b *emptyBackend) ListBranches(storage.BranchListOpts) ([]*storage.Branch, error) {
	return nil, nil
}
func (b *emptyBackend) UpdateBranchStatus(string, storage.BranchStatus) error { return nil }
func (b *emptyBackend) UpdateBranchActivity(string) error                     { return nil }
func (b *emptyBackend) DeleteBranch(string) error                             { return nil }
func (b *emptyBackend) GetStaleBranches(time.Time) ([]*storage.Branch, error) {
	return nil, nil
}
func (b *emptyBackend) ListTaskComments(string) ([]storage.TaskComment, error)     { return nil, nil }
func (b *emptyBackend) SaveTaskComment(*storage.TaskComment) error                 { return nil }
func (b *emptyBackend) ListReviewComments(string) ([]storage.ReviewComment, error) { return nil, nil }
func (b *emptyBackend) SaveReviewComment(*storage.ReviewComment) error             { return nil }
func (b *emptyBackend) SaveEvent(*db.EventLog) error                               { return nil }
func (b *emptyBackend) SaveEvents([]*db.EventLog) error                            { return nil }
func (b *emptyBackend) QueryEvents(db.QueryEventsOptions) ([]db.EventLog, error)   { return nil, nil }
func (b *emptyBackend) ListGateDecisions(string) ([]db.GateDecision, error) { return nil, nil }
func (b *emptyBackend) SaveGateDecision(*db.GateDecision) error             { return nil }
func (b *emptyBackend) SaveReviewFindings(*orcv1.ReviewRoundFindings) error { return nil }
func (b *emptyBackend) LoadReviewFindings(string, int) (*orcv1.ReviewRoundFindings, error) {
	return nil, nil
}
func (b *emptyBackend) LoadAllReviewFindings(string) ([]*orcv1.ReviewRoundFindings, error) {
	return nil, nil
}
func (b *emptyBackend) SaveQAResult(*storage.QAResult) error           { return nil }
func (b *emptyBackend) LoadQAResult(string) (*storage.QAResult, error) { return nil, nil }
func (b *emptyBackend) SaveConstitution(string) error            { return nil }
func (b *emptyBackend) LoadConstitution() (string, string, error) { return "", "", nil }
func (b *emptyBackend) ConstitutionExists() (bool, error)              { return false, nil }
func (b *emptyBackend) DeleteConstitution() error                      { return nil }

// Workflow operations (stub implementations)
func (b *emptyBackend) SavePhaseTemplate(*db.PhaseTemplate) error              { return nil }
func (b *emptyBackend) GetPhaseTemplate(string) (*db.PhaseTemplate, error)     { return nil, nil }
func (b *emptyBackend) ListPhaseTemplates() ([]*db.PhaseTemplate, error)       { return nil, nil }
func (b *emptyBackend) DeletePhaseTemplate(string) error                       { return nil }
func (b *emptyBackend) SaveWorkflow(*db.Workflow) error                        { return nil }
func (b *emptyBackend) GetWorkflow(string) (*db.Workflow, error)               { return nil, nil }
func (b *emptyBackend) ListWorkflows() ([]*db.Workflow, error)                 { return nil, nil }
func (b *emptyBackend) DeleteWorkflow(string) error                            { return nil }
func (b *emptyBackend) GetWorkflowPhases(string) ([]*db.WorkflowPhase, error)  { return nil, nil }
func (b *emptyBackend) SaveWorkflowPhase(*db.WorkflowPhase) error              { return nil }
func (b *emptyBackend) DeleteWorkflowPhase(string, string) error               { return nil }
func (b *emptyBackend) UpdateWorkflowPhasePositions(string, map[string][2]float64) error { return nil }
func (b *emptyBackend) GetWorkflowVariables(string) ([]*db.WorkflowVariable, error) {
	return nil, nil
}
func (b *emptyBackend) SaveWorkflowVariable(*db.WorkflowVariable) error  { return nil }
func (b *emptyBackend) DeleteWorkflowVariable(string, string) error      { return nil }
func (b *emptyBackend) SaveWorkflowRun(*db.WorkflowRun) error            { return nil }
func (b *emptyBackend) GetWorkflowRun(string) (*db.WorkflowRun, error)   { return nil, nil }
func (b *emptyBackend) ListWorkflowRuns(db.WorkflowRunListOpts) ([]*db.WorkflowRun, error) {
	return nil, nil
}
func (b *emptyBackend) DeleteWorkflowRun(string) error                          { return nil }
func (b *emptyBackend) GetNextWorkflowRunID() (string, error)                   { return "", nil }
func (b *emptyBackend) GetWorkflowRunPhases(string) ([]*db.WorkflowRunPhase, error) {
	return nil, nil
}
func (b *emptyBackend) SaveWorkflowRunPhase(*db.WorkflowRunPhase) error  { return nil }
func (b *emptyBackend) UpdatePhaseIterations(string, string, int) error { return nil }
func (b *emptyBackend) GetRunningWorkflowsByTask() (map[string]*db.WorkflowRun, error) {
	return nil, nil
}
func (b *emptyBackend) TryClaimTaskExecution(context.Context, string, int, string) error {
	return nil
}
func (b *emptyBackend) SaveProjectCommand(*db.ProjectCommand) error { return nil }
func (b *emptyBackend) GetProjectCommand(string) (*db.ProjectCommand, error) {
	return nil, nil
}
func (b *emptyBackend) ListProjectCommands() ([]*db.ProjectCommand, error) { return nil, nil }
func (b *emptyBackend) GetProjectCommandsMap() (map[string]*db.ProjectCommand, error) {
	return nil, nil
}
func (b *emptyBackend) DeleteProjectCommand(string) error            { return nil }
func (b *emptyBackend) SetProjectCommandEnabled(string, bool) error  { return nil }
func (b *emptyBackend) DB() *db.ProjectDB { return nil }

// Proto initiative methods
func (b *emptyBackend) SaveInitiativeProto(*orcv1.Initiative) error { return nil }
func (b *emptyBackend) LoadInitiativeProto(string) (*orcv1.Initiative, error) {
	return nil, nil
}
func (b *emptyBackend) LoadAllInitiativesProto() ([]*orcv1.Initiative, error) {
	return nil, nil
}

func TestPRPoller_StopTwice(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
