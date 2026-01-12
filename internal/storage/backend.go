// Package storage provides storage backend abstraction for orc.
// It supports multiple storage modes: hybrid (files + SQLite cache),
// files-only, and database-primary.
package storage

import (
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// Transcript represents a conversation transcript entry.
type Transcript struct {
	ID        int64
	TaskID    string
	Phase     string
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

// Backend defines the storage operations for orc.
// All implementations must be safe for concurrent access.
type Backend interface {
	// Task operations
	SaveTask(t *task.Task) error
	LoadTask(id string) (*task.Task, error)
	LoadAllTasks() ([]*task.Task, error)
	DeleteTask(id string) error

	// State operations
	SaveState(s *state.State) error
	LoadState(taskID string) (*state.State, error)

	// Plan operations
	SavePlan(p *plan.Plan, taskID string) error
	LoadPlan(taskID string) (*plan.Plan, error)

	// Transcript operations
	AddTranscript(t *Transcript) error
	GetTranscripts(taskID string) ([]Transcript, error)
	SearchTranscripts(query string) ([]TranscriptMatch, error)

	// Context materialization (for agents working in worktrees)
	// Generates context.md with all relevant task information
	MaterializeContext(taskID, outputPath string) error

	// NeedsMaterialization returns true if this backend needs context
	// materialization (e.g., database-primary mode)
	NeedsMaterialization() bool

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
