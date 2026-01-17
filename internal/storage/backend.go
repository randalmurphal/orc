// Package storage provides storage backend abstraction for orc.
// SQLite is the source of truth for all data.
package storage

import (
	"context"
	"time"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// Transcript represents a conversation transcript entry.
type Transcript struct {
	ID        int64
	TaskID    string
	Phase     string
	Iteration int    // Iteration number within the phase (1-based)
	Role      string // "prompt", "response", or "combined" for full transcript
	Content   string
	Timestamp int64
}

// TranscriptMatch represents a search result from transcript FTS.
type TranscriptMatch struct {
	TaskID  string
	Phase   string
	Snippet string
	Rank    float64
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

// GateDecision represents a phase gate approval decision.
type GateDecision struct {
	ID        int64
	TaskID    string
	Phase     string
	GateType  string // "auto", "ai", "human", "skip"
	Approved  bool
	Reason    string
	DecidedBy string
	DecidedAt time.Time
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

// Backend defines the storage operations for orc.
// All implementations must be safe for concurrent access.
type Backend interface {
	// Task operations
	SaveTask(t *task.Task) error
	LoadTask(id string) (*task.Task, error)
	LoadAllTasks() ([]*task.Task, error)
	DeleteTask(id string) error
	TaskExists(id string) (bool, error)
	GetNextTaskID() (string, error)

	// State operations
	SaveState(s *state.State) error
	LoadState(taskID string) (*state.State, error)
	LoadAllStates() ([]*state.State, error)

	// Plan operations
	SavePlan(p *plan.Plan, taskID string) error
	LoadPlan(taskID string) (*plan.Plan, error)

	// Spec operations
	SaveSpec(taskID, content, source string) error
	LoadSpec(taskID string) (string, error)
	SpecExists(taskID string) (bool, error)

	// Initiative operations
	SaveInitiative(i *initiative.Initiative) error
	LoadInitiative(id string) (*initiative.Initiative, error)
	LoadAllInitiatives() ([]*initiative.Initiative, error)
	DeleteInitiative(id string) error
	InitiativeExists(id string) (bool, error)
	GetNextInitiativeID() (string, error)

	// Transcript operations
	AddTranscript(t *Transcript) error
	AddTranscriptBatch(ctx context.Context, transcripts []Transcript) error
	GetTranscripts(taskID string) ([]Transcript, error)
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

	// Gate decision operations (for export/import)
	ListGateDecisions(taskID string) ([]GateDecision, error)
	SaveGateDecision(d *GateDecision) error

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

	// Lifecycle
	Sync() error    // Flush caches to disk
	Cleanup() error // Remove old data per retention policy
	Close() error   // Release resources
}

// ExportOptions configures what to export to a branch.
type ExportOptions struct {
	TaskDefinition bool // task.yaml, plan.yaml
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
