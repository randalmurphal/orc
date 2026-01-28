// Package storage provides storage backend abstraction for orc.
// SQLite is the source of truth for all data.
package storage

import (
	"context"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/task"
)

// Transcript represents a single message from a Claude JSONL session file.
// This stores the full message data including per-message token usage.
type Transcript struct {
	ID            int64
	TaskID        string
	Phase         string
	SessionID     string  // Claude session UUID
	WorkflowRunID string  // Links to workflow_runs.id for tracking
	MessageUUID   string  // Individual message UUID
	ParentUUID    *string // Links to parent message (threading)
	Type          string  // "user", "assistant", "queue-operation", "hook"
	Role          string  // from message.role
	Content       string  // Full content JSON (preserves structure)
	Model         string  // Model used (assistant messages only)

	// Per-message token tracking
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int

	// Tool information
	ToolCalls   string // JSON array of tool_use blocks
	ToolResults string // JSON of toolUseResult metadata

	Timestamp int64 // Unix timestamp (milliseconds)
}

// TranscriptMatch represents a search result from transcript FTS.
type TranscriptMatch struct {
	TaskID    string
	Phase     string
	SessionID string
	Snippet   string
	Rank      float64
}

// ActivityCount represents task completions for a single date.
type ActivityCount struct {
	Date  string // YYYY-MM-DD format
	Count int
}

// TranscriptPaginationOpts configures transcript pagination and filtering.
type TranscriptPaginationOpts struct {
	Phase     string // Filter by phase (optional)
	Cursor    int64  // Cursor for pagination (transcript ID, 0 = start)
	Limit     int    // Max results (default: 50, max: 200)
	Direction string // 'asc' | 'desc' (default: asc)
}

// PaginationResult contains pagination metadata.
type PaginationResult struct {
	NextCursor *int64 `json:"next_cursor,omitempty"`
	PrevCursor *int64 `json:"prev_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
	TotalCount int    `json:"total_count"`
}

// PhaseSummary contains transcript count for a single phase.
type PhaseSummary struct {
	Phase           string `json:"phase"`
	TranscriptCount int    `json:"transcript_count"`
}

// TaskComment represents a discussion comment on a task.
type TaskComment struct {
	ID         string
	TaskID     string
	Author     string
	AuthorType string // "human", "agent", "system"
	Content    string
	Phase      string // Optional: which phase it relates to
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ReviewComment represents a code review comment.
type ReviewComment struct {
	ID          string
	TaskID      string
	ReviewRound int
	FilePath    string
	LineNumber  int
	Content     string
	Severity    string // "suggestion", "issue", "blocker"
	Status      string // "open", "resolved", "wont_fix"
	CreatedAt   time.Time
	ResolvedAt  *time.Time
	ResolvedBy  string
}

// QATest represents a test written during QA.
type QATest struct {
	File        string `json:"file"`
	Description string `json:"description"`
	Type        string `json:"type"`
}

// QATestRun represents test execution results.
type QATestRun struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// QACoverage represents code coverage information.
type QACoverage struct {
	Percentage     float64 `json:"percentage"`
	UncoveredAreas string  `json:"uncovered_areas,omitempty"`
}

// QADoc represents documentation created during QA.
type QADoc struct {
	File string `json:"file"`
	Type string `json:"type"`
}

// QAIssue represents an issue found during QA.
type QAIssue struct {
	Severity     string `json:"severity"`
	Description  string `json:"description"`
	Reproduction string `json:"reproduction,omitempty"`
}

// QAResult represents the complete result of a QA session.
type QAResult struct {
	TaskID         string      `json:"task_id"`
	Status         string      `json:"status"`
	Summary        string      `json:"summary"`
	TestsWritten   []QATest    `json:"tests_written,omitempty"`
	TestsRun       *QATestRun  `json:"tests_run,omitempty"`
	Coverage       *QACoverage `json:"coverage,omitempty"`
	Documentation  []QADoc     `json:"documentation,omitempty"`
	Issues         []QAIssue   `json:"issues,omitempty"`
	Recommendation string      `json:"recommendation"`
	CreatedAt      time.Time   `json:"created_at"`
}

// BranchType represents the type of branch being tracked.
type BranchType string

const (
	BranchTypeInitiative BranchType = "initiative"
	BranchTypeStaging    BranchType = "staging"
	BranchTypeTask       BranchType = "task"
)

// BranchStatus represents the lifecycle status of a branch.
type BranchStatus string

const (
	BranchStatusActive   BranchStatus = "active"
	BranchStatusMerged   BranchStatus = "merged"
	BranchStatusStale    BranchStatus = "stale"
	BranchStatusOrphaned BranchStatus = "orphaned"
)

// Branch represents a tracked branch in the registry.
type Branch struct {
	Name         string       // Branch name (primary key)
	Type         BranchType   // 'initiative' | 'staging' | 'task'
	OwnerID      string       // INIT-001, TASK-XXX, or developer name
	CreatedAt    time.Time    // When branch was registered
	LastActivity time.Time    // Last activity timestamp
	Status       BranchStatus // 'active' | 'merged' | 'stale' | 'orphaned'
}

// BranchListOpts provides filtering options for listing branches.
type BranchListOpts struct {
	Type   BranchType   // Filter by type (empty = all)
	Status BranchStatus // Filter by status (empty = all)
}

// PhaseOutputInfo represents phase output metadata for display purposes.
type PhaseOutputInfo struct {
	ID              int64
	WorkflowRunID   string
	PhaseTemplateID string
	TaskID          *string
	Content         string
	ContentHash     string
	OutputVarName   string
	ArtifactType    string
	Source          string
	Iteration       int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// Backend defines the storage operations for orc.
// All implementations must be safe for concurrent access.
type Backend interface {
	// Task operations (using orcv1.Task - the ONLY task type)
	SaveTask(t *orcv1.Task) error
	LoadTask(id string) (*orcv1.Task, error)
	LoadAllTasks() ([]*orcv1.Task, error)
	DeleteTask(id string) error
	TaskExists(id string) (bool, error)
	GetNextTaskID() (string, error)

	// Task heartbeat (for orphan detection during long-running phases)
	UpdateTaskHeartbeat(taskID string) error

	// Task executor info (for orphan detection)
	SetTaskExecutor(taskID string, pid int, hostname string) error
	ClearTaskExecutor(taskID string) error

	// Task activity operations (for heatmap)
	GetTaskActivityByDate(startDate, endDate string) ([]ActivityCount, error)

	// Phase output operations (unified storage for all phase artifacts)
	SavePhaseOutput(output *PhaseOutputInfo) error
	GetPhaseOutput(runID, phaseTemplateID string) (*PhaseOutputInfo, error)
	GetPhaseOutputByVarName(runID, varName string) (*PhaseOutputInfo, error)
	GetAllPhaseOutputs(runID string) ([]*PhaseOutputInfo, error)
	LoadPhaseOutputsAsMap(runID string) (map[string]string, error)
	GetPhaseOutputsForTask(taskID string) ([]*PhaseOutputInfo, error)
	DeletePhaseOutput(runID, phaseTemplateID string) error
	PhaseOutputExists(runID, phaseTemplateID string) (bool, error)

	// Task-level spec convenience methods (queries phase_outputs via workflow_runs)
	GetSpecForTask(taskID string) (string, error)
	GetFullSpecForTask(taskID string) (*PhaseOutputInfo, error)
	SpecExistsForTask(taskID string) (bool, error)
	SaveSpecForTask(taskID, content, source string) error // For import compatibility

	// Initiative operations (legacy - using initiative.Initiative)
	SaveInitiative(i *initiative.Initiative) error
	LoadInitiative(id string) (*initiative.Initiative, error)
	LoadAllInitiatives() ([]*initiative.Initiative, error)
	DeleteInitiative(id string) error
	InitiativeExists(id string) (bool, error)
	GetNextInitiativeID() (string, error)

	// Initiative operations (proto types - preferred for new code)
	SaveInitiativeProto(i *orcv1.Initiative) error
	LoadInitiativeProto(id string) (*orcv1.Initiative, error)
	LoadAllInitiativesProto() ([]*orcv1.Initiative, error)

	// Phase template operations
	SavePhaseTemplate(pt *db.PhaseTemplate) error
	GetPhaseTemplate(id string) (*db.PhaseTemplate, error)
	ListPhaseTemplates() ([]*db.PhaseTemplate, error)
	DeletePhaseTemplate(id string) error

	// Workflow operations
	SaveWorkflow(w *db.Workflow) error
	GetWorkflow(id string) (*db.Workflow, error)
	ListWorkflows() ([]*db.Workflow, error)
	DeleteWorkflow(id string) error
	GetWorkflowPhases(workflowID string) ([]*db.WorkflowPhase, error)
	SaveWorkflowPhase(wp *db.WorkflowPhase) error
	DeleteWorkflowPhase(workflowID, phaseTemplateID string) error
	GetWorkflowVariables(workflowID string) ([]*db.WorkflowVariable, error)
	SaveWorkflowVariable(wv *db.WorkflowVariable) error
	DeleteWorkflowVariable(workflowID, name string) error

	// Workflow run operations
	SaveWorkflowRun(wr *db.WorkflowRun) error
	GetWorkflowRun(id string) (*db.WorkflowRun, error)
	ListWorkflowRuns(opts db.WorkflowRunListOpts) ([]*db.WorkflowRun, error)
	DeleteWorkflowRun(id string) error
	GetNextWorkflowRunID() (string, error)
	GetWorkflowRunPhases(runID string) ([]*db.WorkflowRunPhase, error)
	SaveWorkflowRunPhase(wrp *db.WorkflowRunPhase) error
	UpdatePhaseIterations(runID, phaseID string, iterations int) error
	GetRunningWorkflowsByTask() (map[string]*db.WorkflowRun, error)

	// Transcript operations
	AddTranscript(t *Transcript) error
	AddTranscriptBatch(ctx context.Context, transcripts []Transcript) error
	GetTranscripts(taskID string) ([]Transcript, error)
	GetTranscriptsPaginated(taskID string, opts TranscriptPaginationOpts) ([]Transcript, PaginationResult, error)
	GetPhaseSummary(taskID string) ([]PhaseSummary, error)
	SearchTranscripts(query string) ([]TranscriptMatch, error)

	// Attachment operations
	SaveAttachment(taskID, filename, contentType string, data []byte) (*task.Attachment, error)
	GetAttachment(taskID, filename string) (*task.Attachment, []byte, error)
	ListAttachments(taskID string) ([]*task.Attachment, error)
	DeleteAttachment(taskID, filename string) error

	// Comment operations (for export/import)
	ListTaskComments(taskID string) ([]TaskComment, error)
	SaveTaskComment(c *TaskComment) error
	ListReviewComments(taskID string) ([]ReviewComment, error)
	SaveReviewComment(c *ReviewComment) error

	// Review findings operations (structured review output for multi-round review)
	// Uses proto types directly - orcv1.ReviewRoundFindings
	SaveReviewFindings(f *orcv1.ReviewRoundFindings) error
	LoadReviewFindings(taskID string, round int) (*orcv1.ReviewRoundFindings, error)
	LoadAllReviewFindings(taskID string) ([]*orcv1.ReviewRoundFindings, error)

	// QA result operations (structured QA output for reporting)
	SaveQAResult(r *QAResult) error
	LoadQAResult(taskID string) (*QAResult, error)

	// Gate decision operations (for export/import)
	ListGateDecisions(taskID string) ([]db.GateDecision, error)
	SaveGateDecision(d *db.GateDecision) error

	// Event log operations (for timeline reconstruction)
	SaveEvent(e *db.EventLog) error
	SaveEvents(events []*db.EventLog) error
	QueryEvents(opts db.QueryEventsOptions) ([]db.EventLog, error)

	// Context materialization (for agents working in worktrees)
	// Generates context.md with all relevant task information
	MaterializeContext(taskID, outputPath string) error

	// NeedsMaterialization returns true if this backend needs context
	// materialization (e.g., database-primary mode)
	NeedsMaterialization() bool

	// Branch registry operations
	SaveBranch(b *Branch) error
	LoadBranch(name string) (*Branch, error)
	ListBranches(opts BranchListOpts) ([]*Branch, error)
	UpdateBranchStatus(name string, status BranchStatus) error
	UpdateBranchActivity(name string) error
	DeleteBranch(name string) error
	GetStaleBranches(since time.Time) ([]*Branch, error)

	// Constitution operations (file-based at .orc/CONSTITUTION.md)
	SaveConstitution(content string) error
	LoadConstitution() (content string, path string, err error)
	ConstitutionExists() (bool, error)
	DeleteConstitution() error

	// Task execution claim operations (for resume race condition prevention)
	TryClaimTaskExecution(ctx context.Context, taskID string, pid int, hostname string) error

	// Project command operations (for quality checks)
	SaveProjectCommand(cmd *db.ProjectCommand) error
	GetProjectCommand(name string) (*db.ProjectCommand, error)
	ListProjectCommands() ([]*db.ProjectCommand, error)
	GetProjectCommandsMap() (map[string]*db.ProjectCommand, error)
	DeleteProjectCommand(name string) error
	SetProjectCommandEnabled(name string, enabled bool) error

	// Database access (for WorkflowExecutor which needs ProjectDB directly)
	DB() *db.ProjectDB

	// Lifecycle
	Sync() error    // Flush caches to disk
	Cleanup() error // Remove old data per retention policy
	Close() error   // Release resources
}

// ExportOptions configures what to export to a branch.
type ExportOptions struct {
	TaskDefinition bool // task.yaml
	FinalState     bool // state.yaml
	Transcripts    bool // Full transcript files
	ContextSummary bool // context.md
}

// Exporter handles exporting task data to branches.
type Exporter interface {
	// ExportToBranch exports task artifacts to the specified branch.
	// This is called before PR creation when export is enabled.
	ExportToBranch(taskID, branch string, opts *ExportOptions) error
}
