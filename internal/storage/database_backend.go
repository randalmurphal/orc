package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// DatabaseBackend uses SQLite/PostgreSQL as the sole source of truth.
// No YAML files are created or read. This enables database sync across machines.
// All operations are protected by a mutex for concurrent access safety.
type DatabaseBackend struct {
	projectPath string
	db          *db.ProjectDB
	cfg         *config.StorageConfig
	mu          sync.RWMutex
	logger      *log.Logger
}

// NewDatabaseBackend creates a new database-only storage backend.
func NewDatabaseBackend(projectPath string, cfg *config.StorageConfig) (*DatabaseBackend, error) {
	pdb, err := db.OpenProject(projectPath)
	if err != nil {
		return nil, fmt.Errorf("open project database: %w", err)
	}

	// Create a logger that discards output by default
	logger := log.New(io.Discard, "", 0)

	return &DatabaseBackend{
		projectPath: projectPath,
		db:          pdb,
		cfg:         cfg,
		logger:      logger,
	}, nil
}

// NewInMemoryBackend creates an in-memory database backend for testing.
// This is much faster than file-based databases and ideal for unit tests.
func NewInMemoryBackend() (*DatabaseBackend, error) {
	pdb, err := db.OpenProjectInMemory()
	if err != nil {
		return nil, fmt.Errorf("open in-memory database: %w", err)
	}

	return &DatabaseBackend{
		projectPath: ":memory:",
		db:          pdb,
		cfg:         nil,
		logger:      log.New(io.Discard, "", 0),
	}, nil
}

// SetLogger sets the logger for warnings and debug messages.
func (d *DatabaseBackend) SetLogger(l *log.Logger) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logger = l
}

// DB returns the underlying database for direct access.
// WARNING: Direct database access bypasses the mutex protection.
// Callers must coordinate their own locking or ensure exclusive access.
// Prefer using the Backend interface methods which provide thread-safety.
func (d *DatabaseBackend) DB() *db.ProjectDB {
	return d.db
}

// SaveTask saves a task to the database.
// Note: This preserves state-managed fields (StateStatus, RetryContext, executor info)
// which are managed by SaveState, not SaveTask. This prevents overwriting execution
// state when updating task metadata.
// All operations (task + dependencies) are wrapped in a transaction for atomicity.
// This method uses context.Background(). Use SaveTaskCtx for cancellation support.
func (d *DatabaseBackend) SaveTask(t *task.Task) error {
	return d.SaveTaskCtx(context.Background(), t)
}

// SaveTaskCtx saves a task to the database with context support.
// Note: This preserves state-managed fields (StateStatus, RetryContext, executor info)
// which are managed by SaveState, not SaveTask. This prevents overwriting execution
// state when updating task metadata (which would cause false orphan detection).
// All operations (task + dependencies) are wrapped in a transaction for atomicity.
func (d *DatabaseBackend) SaveTaskCtx(ctx context.Context, t *task.Task) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Convert to db.Task
	dbTask := taskToDBTask(t)

	// Preserve state fields from existing task - these are managed by SaveState, not SaveTask
	// Includes: StateStatus, RetryContext, and executor fields for orphan detection
	existingTask, err := d.db.GetTask(t.ID)
	if err == nil && existingTask != nil {
		dbTask.StateStatus = existingTask.StateStatus
		dbTask.RetryContext = existingTask.RetryContext
		// Preserve executor fields to avoid false orphan detection
		dbTask.ExecutorPID = existingTask.ExecutorPID
		dbTask.ExecutorHostname = existingTask.ExecutorHostname
		dbTask.ExecutorStartedAt = existingTask.ExecutorStartedAt
		dbTask.LastHeartbeat = existingTask.LastHeartbeat
	}

	// Wrap all operations in a transaction for atomicity
	return d.db.RunInTx(ctx, func(tx *db.TxOps) error {
		if err := db.SaveTaskTx(tx, dbTask); err != nil {
			return fmt.Errorf("save task: %w", err)
		}

		// Save dependencies - clear first, then add new ones
		if err := db.ClearTaskDependenciesTx(tx, t.ID); err != nil {
			return fmt.Errorf("clear task dependencies: %w", err)
		}
		for _, depID := range t.BlockedBy {
			if err := db.AddTaskDependencyTx(tx, t.ID, depID); err != nil {
				return fmt.Errorf("add task dependency %s: %w", depID, err)
			}
		}

		return nil
	})
}

// LoadTask loads a task from the database.
func (d *DatabaseBackend) LoadTask(id string) (*task.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbTask, err := d.db.GetTask(id)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if dbTask == nil {
		return nil, fmt.Errorf("task %s not found", id)
	}

	// Convert from db.Task
	t := dbTaskToTask(dbTask)

	// Load dependencies
	deps, err := d.db.GetTaskDependencies(id)
	if err != nil {
		d.logger.Printf("warning: failed to get task dependencies: %v", err)
	} else {
		t.BlockedBy = deps
	}

	return t, nil
}

// LoadAllTasks loads all tasks from the database.
func (d *DatabaseBackend) LoadAllTasks() ([]*task.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbTasks, _, err := d.db.ListTasks(db.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	// Batch load all dependencies in one query to avoid N+1
	allDeps, err := d.db.GetAllTaskDependencies()
	if err != nil {
		d.logger.Printf("warning: failed to batch load dependencies: %v", err)
		allDeps = make(map[string][]string) // Fall back to empty
	}

	tasks := make([]*task.Task, 0, len(dbTasks))
	for _, dbTask := range dbTasks {
		t := dbTaskToTask(&dbTask)

		// Use pre-fetched dependencies
		if deps, ok := allDeps[t.ID]; ok {
			t.BlockedBy = deps
		}

		tasks = append(tasks, t)
	}

	return tasks, nil
}

// DeleteTask removes a task from the database.
func (d *DatabaseBackend) DeleteTask(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Delete cascades to plans, specs, attachments, etc. via foreign keys
	if err := d.db.DeleteTask(id); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}

	return nil
}

// SaveState saves execution state to the database.
// All operations (task update + phases + gate decisions) are wrapped in a transaction for atomicity.
// This method uses context.Background(). Use SaveStateCtx for cancellation support.
func (d *DatabaseBackend) SaveState(s *state.State) error {
	return d.SaveStateCtx(context.Background(), s)
}

// SaveStateCtx saves execution state to the database with context support.
// All operations (task update + phases + gate decisions) are wrapped in a transaction for atomicity.
func (d *DatabaseBackend) SaveStateCtx(ctx context.Context, s *state.State) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Get task first (outside transaction since it's read-only)
	dbTask, err := d.db.GetTask(s.TaskID)
	if err != nil {
		return fmt.Errorf("get task for state: %w", err)
	}
	if dbTask == nil {
		return fmt.Errorf("task %s not found for state", s.TaskID)
	}

	// Update task fields from state
	// Note: state.Status and task.Status have different value sets
	// state.Status: pending, running, completed, failed, paused, interrupted, skipped
	// task.Status: created, classifying, planned, running, paused, blocked, finalizing, completed, failed
	// We store state status in a separate field (StateStatus)
	dbTask.StateStatus = string(s.Status)
	dbTask.CurrentPhase = s.CurrentPhase
	dbTask.TotalCostUSD = s.Cost.TotalCostUSD

	// Persist execution info for orphan detection
	if s.Execution != nil {
		dbTask.ExecutorPID = s.Execution.PID
		dbTask.ExecutorHostname = s.Execution.Hostname
		if !s.Execution.StartedAt.IsZero() {
			dbTask.ExecutorStartedAt = &s.Execution.StartedAt
		}
		if !s.Execution.LastHeartbeat.IsZero() {
			dbTask.LastHeartbeat = &s.Execution.LastHeartbeat
		}
	} else {
		// Clear execution info when not present
		dbTask.ExecutorPID = 0
		dbTask.ExecutorHostname = ""
		dbTask.ExecutorStartedAt = nil
		dbTask.LastHeartbeat = nil
	}

	// Serialize RetryContext if present
	if s.RetryContext != nil {
		retryContextJSON, err := json.Marshal(s.RetryContext)
		if err != nil {
			d.logger.Printf("warning: failed to serialize retry context: %v", err)
		} else {
			dbTask.RetryContext = string(retryContextJSON)
		}
	} else {
		dbTask.RetryContext = ""
	}

	// Wrap all operations in a transaction for atomicity
	return d.db.RunInTx(ctx, func(tx *db.TxOps) error {
		if err := db.SaveTaskTx(tx, dbTask); err != nil {
			return fmt.Errorf("update task from state: %w", err)
		}

		// Clear existing phases before saving new ones
		// This ensures phases removed from s.Phases are deleted from DB
		if err := db.ClearPhasesTx(tx, s.TaskID); err != nil {
			return fmt.Errorf("clear phases: %w", err)
		}

		// Save phase states
		for phaseID, ps := range s.Phases {
			var startedAt *time.Time
			if !ps.StartedAt.IsZero() {
				startedAt = &ps.StartedAt
			}
			dbPhase := &db.Phase{
				TaskID:       s.TaskID,
				PhaseID:      phaseID,
				Status:       string(ps.Status),
				Iterations:   ps.Iterations,
				StartedAt:    startedAt,
				CompletedAt:  ps.CompletedAt,
				InputTokens:  ps.Tokens.InputTokens,
				OutputTokens: ps.Tokens.OutputTokens,
				CostUSD:      0, // Cost is tracked at state level, not phase level
				ErrorMessage: ps.Error,
				CommitSHA:    ps.CommitSHA,
				SessionID:    ps.SessionID,
			}
			if err := db.SavePhaseTx(tx, dbPhase); err != nil {
				return fmt.Errorf("save phase %s: %w", phaseID, err)
			}
		}

		// Save gate decisions
		for _, gate := range s.Gates {
			dbGate := &db.GateDecision{
				TaskID:    s.TaskID,
				Phase:     gate.Phase,
				GateType:  gate.GateType,
				Approved:  gate.Approved,
				Reason:    gate.Reason,
				DecidedAt: gate.Timestamp,
			}
			if err := db.AddGateDecisionTx(tx, dbGate); err != nil {
				return fmt.Errorf("save gate decision: %w", err)
			}
		}

		return nil
	})
}

// LoadState loads execution state from the database.
func (d *DatabaseBackend) LoadState(taskID string) (*state.State, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.loadStateUnlocked(taskID)
}

// loadStateUnlocked is the internal implementation of LoadState without locking.
// Caller must hold d.mu.RLock() or d.mu.Lock().
func (d *DatabaseBackend) loadStateUnlocked(taskID string) (*state.State, error) {
	// Get task for basic info
	dbTask, err := d.db.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("get task for state: %w", err)
	}
	if dbTask == nil {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	// Get phases
	dbPhases, err := d.db.GetPhases(taskID)
	if err != nil {
		return nil, fmt.Errorf("get phases: %w", err)
	}

	// Get gate decisions
	dbGates, err := d.db.GetGateDecisions(taskID)
	if err != nil {
		d.logger.Printf("warning: failed to get gate decisions: %v", err)
	}

	// Build state
	var startedAt time.Time
	if dbTask.StartedAt != nil {
		startedAt = *dbTask.StartedAt
	}

	// Get state status from StateStatus field (not Status which is task status)
	stateStatus := dbTask.StateStatus
	if stateStatus == "" {
		stateStatus = "pending" // Default
	}

	s := &state.State{
		TaskID:       taskID,
		CurrentPhase: dbTask.CurrentPhase,
		Status:       state.Status(stateStatus),
		Phases:       make(map[string]*state.PhaseState),
		StartedAt:    startedAt,
	}

	// Deserialize RetryContext if present
	if dbTask.RetryContext != "" {
		var retryCtx state.RetryContext
		if err := json.Unmarshal([]byte(dbTask.RetryContext), &retryCtx); err != nil {
			d.logger.Printf("warning: failed to deserialize retry context: %v", err)
		} else {
			s.RetryContext = &retryCtx
		}
	}

	// Populate phases
	for _, dbPhase := range dbPhases {
		var phaseStartedAt time.Time
		if dbPhase.StartedAt != nil {
			phaseStartedAt = *dbPhase.StartedAt
		}
		s.Phases[dbPhase.PhaseID] = &state.PhaseState{
			Status:      state.Status(dbPhase.Status),
			Iterations:  dbPhase.Iterations,
			StartedAt:   phaseStartedAt,
			CompletedAt: dbPhase.CompletedAt,
			Error:       dbPhase.ErrorMessage,
			CommitSHA:   dbPhase.CommitSHA,
			SessionID:   dbPhase.SessionID,
			Tokens: state.TokenUsage{
				InputTokens:  dbPhase.InputTokens,
				OutputTokens: dbPhase.OutputTokens,
			},
		}
	}

	// Populate gates
	for _, dbGate := range dbGates {
		s.Gates = append(s.Gates, state.GateDecision{
			Phase:     dbGate.Phase,
			GateType:  dbGate.GateType,
			Approved:  dbGate.Approved,
			Reason:    dbGate.Reason,
			Timestamp: dbGate.DecidedAt,
		})
	}

	// Reconstruct ExecutionInfo for orphan detection
	// Only create ExecutionInfo if there's meaningful execution data
	if dbTask.ExecutorPID > 0 || dbTask.ExecutorHostname != "" ||
		dbTask.ExecutorStartedAt != nil || dbTask.LastHeartbeat != nil {
		s.Execution = &state.ExecutionInfo{
			PID:      dbTask.ExecutorPID,
			Hostname: dbTask.ExecutorHostname,
		}
		if dbTask.ExecutorStartedAt != nil {
			s.Execution.StartedAt = *dbTask.ExecutorStartedAt
		}
		if dbTask.LastHeartbeat != nil {
			s.Execution.LastHeartbeat = *dbTask.LastHeartbeat
		}
	}

	// Load cost from task
	s.Cost.TotalCostUSD = dbTask.TotalCostUSD

	return s, nil
}

// LoadAllStates loads all task states from the database.
// Note: This holds the read lock for the entire operation to ensure consistency.
func (d *DatabaseBackend) LoadAllStates() ([]*state.State, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Use internal unlocked version to avoid deadlock
	dbTasks, _, err := d.db.ListTasks(db.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	var states []*state.State
	for _, dbTask := range dbTasks {
		// Load state for each task using internal unlocked access
		s, err := d.loadStateUnlocked(dbTask.ID)
		if err != nil {
			// Skip tasks without state (e.g., never started)
			continue
		}
		states = append(states, s)
	}

	return states, nil
}

// AddTranscript adds a transcript to database (for FTS).
func (d *DatabaseBackend) AddTranscript(t *Transcript) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbTranscript := &db.Transcript{
		TaskID:              t.TaskID,
		Phase:               t.Phase,
		SessionID:           t.SessionID,
		MessageUUID:         t.MessageUUID,
		ParentUUID:          t.ParentUUID,
		Type:                t.Type,
		Role:                t.Role,
		Content:             t.Content,
		Model:               t.Model,
		InputTokens:         t.InputTokens,
		OutputTokens:        t.OutputTokens,
		CacheCreationTokens: t.CacheCreationTokens,
		CacheReadTokens:     t.CacheReadTokens,
		ToolCalls:           t.ToolCalls,
		ToolResults:         t.ToolResults,
		Timestamp:           time.UnixMilli(t.Timestamp),
	}
	if err := d.db.AddTranscript(dbTranscript); err != nil {
		return fmt.Errorf("add transcript: %w", err)
	}
	t.ID = dbTranscript.ID
	return nil
}

// AddTranscriptBatch adds multiple transcripts in a single transaction.
// This is more efficient than calling AddTranscript repeatedly for streaming data.
func (d *DatabaseBackend) AddTranscriptBatch(ctx context.Context, transcripts []Transcript) error {
	if len(transcripts) == 0 {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	dbTranscripts := make([]db.Transcript, len(transcripts))
	for i, t := range transcripts {
		dbTranscripts[i] = db.Transcript{
			TaskID:              t.TaskID,
			Phase:               t.Phase,
			SessionID:           t.SessionID,
			MessageUUID:         t.MessageUUID,
			ParentUUID:          t.ParentUUID,
			Type:                t.Type,
			Role:                t.Role,
			Content:             t.Content,
			Model:               t.Model,
			InputTokens:         t.InputTokens,
			OutputTokens:        t.OutputTokens,
			CacheCreationTokens: t.CacheCreationTokens,
			CacheReadTokens:     t.CacheReadTokens,
			ToolCalls:           t.ToolCalls,
			ToolResults:         t.ToolResults,
			Timestamp:           time.UnixMilli(t.Timestamp),
		}
	}

	if err := d.db.AddTranscriptBatch(ctx, dbTranscripts); err != nil {
		return fmt.Errorf("add transcript batch: %w", err)
	}

	// Update IDs in the original slice
	for i := range transcripts {
		transcripts[i].ID = dbTranscripts[i].ID
	}
	return nil
}

// GetTranscripts retrieves transcripts for a task.
func (d *DatabaseBackend) GetTranscripts(taskID string) ([]Transcript, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbTranscripts, err := d.db.GetTranscripts(taskID)
	if err != nil {
		return nil, fmt.Errorf("get transcripts: %w", err)
	}

	result := make([]Transcript, len(dbTranscripts))
	for i, t := range dbTranscripts {
		result[i] = Transcript{
			ID:                  t.ID,
			TaskID:              t.TaskID,
			Phase:               t.Phase,
			SessionID:           t.SessionID,
			MessageUUID:         t.MessageUUID,
			ParentUUID:          t.ParentUUID,
			Type:                t.Type,
			Role:                t.Role,
			Content:             t.Content,
			Model:               t.Model,
			InputTokens:         t.InputTokens,
			OutputTokens:        t.OutputTokens,
			CacheCreationTokens: t.CacheCreationTokens,
			CacheReadTokens:     t.CacheReadTokens,
			ToolCalls:           t.ToolCalls,
			ToolResults:         t.ToolResults,
			Timestamp:           t.Timestamp.UnixMilli(),
		}
	}
	return result, nil
}

// GetTranscriptsPaginated retrieves paginated transcripts with filtering.
func (d *DatabaseBackend) GetTranscriptsPaginated(taskID string, opts TranscriptPaginationOpts) ([]Transcript, PaginationResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Convert storage opts to db opts
	dbOpts := db.TranscriptPaginationOpts{
		Phase:     opts.Phase,
		Cursor:    opts.Cursor,
		Limit:     opts.Limit,
		Direction: opts.Direction,
	}

	dbTranscripts, dbPagination, err := d.db.GetTranscriptsPaginated(taskID, dbOpts)
	if err != nil {
		return nil, PaginationResult{}, fmt.Errorf("get paginated transcripts: %w", err)
	}

	// Convert db.Transcript to storage.Transcript
	result := make([]Transcript, len(dbTranscripts))
	for i, t := range dbTranscripts {
		result[i] = Transcript{
			ID:                  t.ID,
			TaskID:              t.TaskID,
			Phase:               t.Phase,
			SessionID:           t.SessionID,
			MessageUUID:         t.MessageUUID,
			ParentUUID:          t.ParentUUID,
			Type:                t.Type,
			Role:                t.Role,
			Content:             t.Content,
			Model:               t.Model,
			InputTokens:         t.InputTokens,
			OutputTokens:        t.OutputTokens,
			CacheCreationTokens: t.CacheCreationTokens,
			CacheReadTokens:     t.CacheReadTokens,
			ToolCalls:           t.ToolCalls,
			ToolResults:         t.ToolResults,
			Timestamp:           t.Timestamp.UnixMilli(),
		}
	}

	// Convert db.PaginationResult to storage.PaginationResult
	pagination := PaginationResult{
		NextCursor: dbPagination.NextCursor,
		PrevCursor: dbPagination.PrevCursor,
		HasMore:    dbPagination.HasMore,
		TotalCount: dbPagination.TotalCount,
	}

	return result, pagination, nil
}

// GetPhaseSummary returns transcript counts grouped by phase.
func (d *DatabaseBackend) GetPhaseSummary(taskID string) ([]PhaseSummary, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbSummaries, err := d.db.GetPhaseSummary(taskID)
	if err != nil {
		return nil, fmt.Errorf("get phase summary: %w", err)
	}

	result := make([]PhaseSummary, len(dbSummaries))
	for i, s := range dbSummaries {
		result[i] = PhaseSummary{
			Phase:           s.Phase,
			TranscriptCount: s.TranscriptCount,
		}
	}
	return result, nil
}

// SearchTranscripts performs FTS search across transcripts.
func (d *DatabaseBackend) SearchTranscripts(query string) ([]TranscriptMatch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbMatches, err := d.db.SearchTranscripts(query)
	if err != nil {
		return nil, fmt.Errorf("search transcripts: %w", err)
	}

	result := make([]TranscriptMatch, len(dbMatches))
	for i, m := range dbMatches {
		result[i] = TranscriptMatch{
			TaskID:    m.TaskID,
			Phase:     m.Phase,
			SessionID: m.SessionID,
			Snippet:   m.Snippet,
			Rank:      m.Rank,
		}
	}
	return result, nil
}

// ListTaskComments retrieves all comments for a task.
func (d *DatabaseBackend) ListTaskComments(taskID string) ([]TaskComment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbComments, err := d.db.ListTaskComments(taskID)
	if err != nil {
		return nil, fmt.Errorf("list task comments: %w", err)
	}

	result := make([]TaskComment, len(dbComments))
	for i, c := range dbComments {
		result[i] = TaskComment{
			ID:         c.ID,
			TaskID:     c.TaskID,
			Author:     c.Author,
			AuthorType: string(c.AuthorType),
			Content:    c.Content,
			Phase:      c.Phase,
			CreatedAt:  c.CreatedAt,
			UpdatedAt:  c.UpdatedAt,
		}
	}
	return result, nil
}

// SaveTaskComment saves a task comment.
func (d *DatabaseBackend) SaveTaskComment(c *TaskComment) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbComment := &db.TaskComment{
		ID:         c.ID,
		TaskID:     c.TaskID,
		Author:     c.Author,
		AuthorType: db.AuthorType(c.AuthorType),
		Content:    c.Content,
		Phase:      c.Phase,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
	}
	if err := d.db.CreateTaskComment(dbComment); err != nil {
		return fmt.Errorf("save task comment: %w", err)
	}
	c.ID = dbComment.ID
	return nil
}

// ListReviewComments retrieves all review comments for a task.
func (d *DatabaseBackend) ListReviewComments(taskID string) ([]ReviewComment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// List all comments regardless of status
	dbComments, err := d.db.ListReviewComments(taskID, "")
	if err != nil {
		return nil, fmt.Errorf("list review comments: %w", err)
	}

	result := make([]ReviewComment, len(dbComments))
	for i, c := range dbComments {
		result[i] = ReviewComment{
			ID:          c.ID,
			TaskID:      c.TaskID,
			ReviewRound: c.ReviewRound,
			FilePath:    c.FilePath,
			LineNumber:  c.LineNumber,
			Content:     c.Content,
			Severity:    string(c.Severity),
			Status:      string(c.Status),
			CreatedAt:   c.CreatedAt,
			ResolvedAt:  c.ResolvedAt,
			ResolvedBy:  c.ResolvedBy,
		}
	}
	return result, nil
}

// SaveReviewComment saves a review comment.
func (d *DatabaseBackend) SaveReviewComment(c *ReviewComment) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbComment := &db.ReviewComment{
		ID:          c.ID,
		TaskID:      c.TaskID,
		ReviewRound: c.ReviewRound,
		FilePath:    c.FilePath,
		LineNumber:  c.LineNumber,
		Content:     c.Content,
		Severity:    db.ReviewCommentSeverity(c.Severity),
		Status:      db.ReviewCommentStatus(c.Status),
		CreatedAt:   c.CreatedAt,
		ResolvedAt:  c.ResolvedAt,
		ResolvedBy:  c.ResolvedBy,
	}
	if err := d.db.CreateReviewComment(dbComment); err != nil {
		return fmt.Errorf("save review comment: %w", err)
	}
	c.ID = dbComment.ID
	return nil
}

// ListGateDecisions retrieves all gate decisions for a task.
func (d *DatabaseBackend) ListGateDecisions(taskID string) ([]GateDecision, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbDecisions, err := d.db.GetGateDecisions(taskID)
	if err != nil {
		return nil, fmt.Errorf("list gate decisions: %w", err)
	}

	result := make([]GateDecision, len(dbDecisions))
	for i, d := range dbDecisions {
		result[i] = GateDecision{
			ID:        d.ID,
			TaskID:    d.TaskID,
			Phase:     d.Phase,
			GateType:  d.GateType,
			Approved:  d.Approved,
			Reason:    d.Reason,
			DecidedBy: d.DecidedBy,
			DecidedAt: d.DecidedAt,
		}
	}
	return result, nil
}

// SaveGateDecision saves a gate decision.
func (d *DatabaseBackend) SaveGateDecision(gd *GateDecision) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbDecision := &db.GateDecision{
		ID:        gd.ID,
		TaskID:    gd.TaskID,
		Phase:     gd.Phase,
		GateType:  gd.GateType,
		Approved:  gd.Approved,
		Reason:    gd.Reason,
		DecidedBy: gd.DecidedBy,
		DecidedAt: gd.DecidedAt,
	}
	if err := d.db.AddGateDecision(dbDecision); err != nil {
		return fmt.Errorf("save gate decision: %w", err)
	}
	gd.ID = dbDecision.ID
	return nil
}

// ============================================================================
// Review findings operations (structured review output)
// ============================================================================

// SaveReviewFindings saves review findings for a task/round.
func (d *DatabaseBackend) SaveReviewFindings(f *ReviewFindings) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbFindings := &db.ReviewFindings{
		TaskID:      f.TaskID,
		Round:       f.Round,
		Summary:     f.Summary,
		Issues:      make([]db.ReviewFinding, len(f.Issues)),
		Questions:   f.Questions,
		Positives:   f.Positives,
		Perspective: f.Perspective,
		CreatedAt:   f.CreatedAt,
	}
	for i, issue := range f.Issues {
		dbFindings.Issues[i] = db.ReviewFinding{
			Severity:    issue.Severity,
			File:        issue.File,
			Line:        issue.Line,
			Description: issue.Description,
			Suggestion:  issue.Suggestion,
			Perspective: issue.Perspective,
		}
	}
	return d.db.SaveReviewFindings(dbFindings)
}

// LoadReviewFindings loads review findings for a task and round.
func (d *DatabaseBackend) LoadReviewFindings(taskID string, round int) (*ReviewFindings, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbFindings, err := d.db.GetReviewFindings(taskID, round)
	if err != nil {
		return nil, err
	}
	if dbFindings == nil {
		return nil, nil
	}
	return convertDBReviewFindings(dbFindings), nil
}

// LoadAllReviewFindings loads all review findings for a task (all rounds).
func (d *DatabaseBackend) LoadAllReviewFindings(taskID string) ([]*ReviewFindings, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbList, err := d.db.GetAllReviewFindings(taskID)
	if err != nil {
		return nil, err
	}
	result := make([]*ReviewFindings, len(dbList))
	for i, dbFindings := range dbList {
		result[i] = convertDBReviewFindings(dbFindings)
	}
	return result, nil
}

// convertDBReviewFindings converts db.ReviewFindings to storage.ReviewFindings.
func convertDBReviewFindings(dbFindings *db.ReviewFindings) *ReviewFindings {
	f := &ReviewFindings{
		TaskID:      dbFindings.TaskID,
		Round:       dbFindings.Round,
		Summary:     dbFindings.Summary,
		Issues:      make([]ReviewFinding, len(dbFindings.Issues)),
		Questions:   dbFindings.Questions,
		Positives:   dbFindings.Positives,
		Perspective: dbFindings.Perspective,
		CreatedAt:   dbFindings.CreatedAt,
	}
	for i, issue := range dbFindings.Issues {
		f.Issues[i] = ReviewFinding{
			Severity:    issue.Severity,
			File:        issue.File,
			Line:        issue.Line,
			Description: issue.Description,
			Suggestion:  issue.Suggestion,
			Perspective: issue.Perspective,
		}
	}
	if f.Issues == nil {
		f.Issues = []ReviewFinding{}
	}
	if f.Questions == nil {
		f.Questions = []string{}
	}
	if f.Positives == nil {
		f.Positives = []string{}
	}
	return f
}

// SaveQAResult saves QA results to the database.
func (d *DatabaseBackend) SaveQAResult(r *QAResult) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbResult := &db.QAResult{
		TaskID:         r.TaskID,
		Status:         r.Status,
		Summary:        r.Summary,
		Recommendation: r.Recommendation,
	}

	// Convert nested types
	for _, t := range r.TestsWritten {
		dbResult.TestsWritten = append(dbResult.TestsWritten, db.QATest{
			File:        t.File,
			Description: t.Description,
			Type:        t.Type,
		})
	}

	if r.TestsRun != nil {
		dbResult.TestsRun = &db.QATestRun{
			Total:   r.TestsRun.Total,
			Passed:  r.TestsRun.Passed,
			Failed:  r.TestsRun.Failed,
			Skipped: r.TestsRun.Skipped,
		}
	}

	if r.Coverage != nil {
		dbResult.Coverage = &db.QACoverage{
			Percentage:     r.Coverage.Percentage,
			UncoveredAreas: r.Coverage.UncoveredAreas,
		}
	}

	for _, doc := range r.Documentation {
		dbResult.Documentation = append(dbResult.Documentation, db.QADoc{
			File: doc.File,
			Type: doc.Type,
		})
	}

	for _, issue := range r.Issues {
		dbResult.Issues = append(dbResult.Issues, db.QAIssue{
			Severity:     issue.Severity,
			Description:  issue.Description,
			Reproduction: issue.Reproduction,
		})
	}

	return d.db.SaveQAResult(dbResult)
}

// LoadQAResult loads QA results from the database.
func (d *DatabaseBackend) LoadQAResult(taskID string) (*QAResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbResult, err := d.db.GetQAResult(taskID)
	if err != nil {
		return nil, err
	}
	if dbResult == nil {
		return nil, nil
	}

	return convertDBQAResult(dbResult), nil
}

// convertDBQAResult converts a db.QAResult to a storage.QAResult.
func convertDBQAResult(dbResult *db.QAResult) *QAResult {
	r := &QAResult{
		TaskID:         dbResult.TaskID,
		Status:         dbResult.Status,
		Summary:        dbResult.Summary,
		Recommendation: dbResult.Recommendation,
		CreatedAt:      dbResult.CreatedAt,
	}

	// Convert nested types
	for _, t := range dbResult.TestsWritten {
		r.TestsWritten = append(r.TestsWritten, QATest{
			File:        t.File,
			Description: t.Description,
			Type:        t.Type,
		})
	}
	if r.TestsWritten == nil {
		r.TestsWritten = []QATest{}
	}

	if dbResult.TestsRun != nil {
		r.TestsRun = &QATestRun{
			Total:   dbResult.TestsRun.Total,
			Passed:  dbResult.TestsRun.Passed,
			Failed:  dbResult.TestsRun.Failed,
			Skipped: dbResult.TestsRun.Skipped,
		}
	}

	if dbResult.Coverage != nil {
		r.Coverage = &QACoverage{
			Percentage:     dbResult.Coverage.Percentage,
			UncoveredAreas: dbResult.Coverage.UncoveredAreas,
		}
	}

	for _, doc := range dbResult.Documentation {
		r.Documentation = append(r.Documentation, QADoc{
			File: doc.File,
			Type: doc.Type,
		})
	}
	if r.Documentation == nil {
		r.Documentation = []QADoc{}
	}

	for _, issue := range dbResult.Issues {
		r.Issues = append(r.Issues, QAIssue{
			Severity:     issue.Severity,
			Description:  issue.Description,
			Reproduction: issue.Reproduction,
		})
	}
	if r.Issues == nil {
		r.Issues = []QAIssue{}
	}

	return r
}

// MaterializeContext generates context files for worktree execution.
// In database mode, this writes task info to the specified path.
func (d *DatabaseBackend) MaterializeContext(taskID, outputPath string) error {
	// TODO: Generate context.md from database data
	// For now, return nil as the executor can read from DB directly
	return nil
}

// NeedsMaterialization returns true for database mode.
func (d *DatabaseBackend) NeedsMaterialization() bool {
	return true
}

// Sync flushes any pending operations.
func (d *DatabaseBackend) Sync() error {
	// Database operations are synchronous, nothing to sync
	return nil
}

// Cleanup removes old data based on retention policy.
func (d *DatabaseBackend) Cleanup() error {
	// TODO: Implement cleanup based on retention policy
	return nil
}

// Close releases database resources.
// Note: Acquires write lock to ensure no operations are in progress.
func (d *DatabaseBackend) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.Close()
}

// GetNextTaskID generates the next task ID from the database.
func (d *DatabaseBackend) GetNextTaskID() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.GetNextTaskID()
}

// GetTaskActivityByDate returns task completion counts grouped by date.
func (d *DatabaseBackend) GetTaskActivityByDate(startDate, endDate string) ([]ActivityCount, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbActivity, err := d.db.GetTaskActivityByDate(startDate, endDate)
	if err != nil {
		return nil, err
	}

	// Convert from db.ActivityCount to storage.ActivityCount
	result := make([]ActivityCount, len(dbActivity))
	for i, ac := range dbActivity {
		result[i] = ActivityCount{
			Date:  ac.Date,
			Count: ac.Count,
		}
	}
	return result, nil
}

// ============================================================================
// Phase output operations (unified storage for all phase artifacts)
// ============================================================================

// SavePhaseOutput saves a phase output to the database.
func (d *DatabaseBackend) SavePhaseOutput(output *PhaseOutputInfo) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbOutput := &db.PhaseOutput{
		ID:              output.ID,
		WorkflowRunID:   output.WorkflowRunID,
		PhaseTemplateID: output.PhaseTemplateID,
		TaskID:          output.TaskID,
		Content:         output.Content,
		ContentHash:     output.ContentHash,
		OutputVarName:   output.OutputVarName,
		ArtifactType:    output.ArtifactType,
		Source:          output.Source,
		Iteration:       output.Iteration,
		CreatedAt:       output.CreatedAt,
		UpdatedAt:       output.UpdatedAt,
	}
	if err := d.db.SavePhaseOutput(dbOutput); err != nil {
		return fmt.Errorf("save phase output: %w", err)
	}
	output.ID = dbOutput.ID
	return nil
}

// GetPhaseOutput retrieves a phase output by run ID and phase template ID.
func (d *DatabaseBackend) GetPhaseOutput(runID, phaseTemplateID string) (*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutput, err := d.db.GetPhaseOutput(runID, phaseTemplateID)
	if err != nil {
		return nil, fmt.Errorf("get phase output: %w", err)
	}
	if dbOutput == nil {
		return nil, nil
	}
	return dbPhaseOutputToInfo(dbOutput), nil
}

// GetPhaseOutputByVarName retrieves a phase output by run ID and variable name.
func (d *DatabaseBackend) GetPhaseOutputByVarName(runID, varName string) (*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutput, err := d.db.GetPhaseOutputByVarName(runID, varName)
	if err != nil {
		return nil, fmt.Errorf("get phase output by var name: %w", err)
	}
	if dbOutput == nil {
		return nil, nil
	}
	return dbPhaseOutputToInfo(dbOutput), nil
}

// GetAllPhaseOutputs retrieves all phase outputs for a run.
func (d *DatabaseBackend) GetAllPhaseOutputs(runID string) ([]*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutputs, err := d.db.GetAllPhaseOutputs(runID)
	if err != nil {
		return nil, fmt.Errorf("get all phase outputs: %w", err)
	}

	outputs := make([]*PhaseOutputInfo, len(dbOutputs))
	for i, dbOutput := range dbOutputs {
		outputs[i] = dbPhaseOutputToInfo(dbOutput)
	}
	return outputs, nil
}

// LoadPhaseOutputsAsMap returns all phase outputs for a run as a map of varName -> content.
func (d *DatabaseBackend) LoadPhaseOutputsAsMap(runID string) (map[string]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.LoadPhaseOutputsAsMap(runID)
}

// GetSpecForTask retrieves the spec content for a task.
func (d *DatabaseBackend) GetSpecForTask(taskID string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetSpecForTask(taskID)
}

// GetFullSpecForTask retrieves the full spec phase output for a task.
func (d *DatabaseBackend) GetFullSpecForTask(taskID string) (*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutput, err := d.db.GetFullSpecForTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("get full spec for task: %w", err)
	}
	if dbOutput == nil {
		return nil, nil
	}
	return dbPhaseOutputToInfo(dbOutput), nil
}

// SpecExistsForTask checks if a spec exists for a task.
func (d *DatabaseBackend) SpecExistsForTask(taskID string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.SpecExistsForTask(taskID)
}

// SaveSpecForTask saves a spec for a task (for import compatibility).
func (d *DatabaseBackend) SaveSpecForTask(taskID, content, source string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveSpecForTask(taskID, content, source)
}

// GetPhaseOutputsForTask retrieves all phase outputs for a task (across all runs).
func (d *DatabaseBackend) GetPhaseOutputsForTask(taskID string) ([]*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutputs, err := d.db.GetPhaseOutputsForTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("get phase outputs for task: %w", err)
	}

	outputs := make([]*PhaseOutputInfo, len(dbOutputs))
	for i, dbOutput := range dbOutputs {
		outputs[i] = dbPhaseOutputToInfo(dbOutput)
	}
	return outputs, nil
}

// DeletePhaseOutput removes a phase output.
func (d *DatabaseBackend) DeletePhaseOutput(runID, phaseTemplateID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeletePhaseOutput(runID, phaseTemplateID)
}

// PhaseOutputExists checks if a phase output exists.
func (d *DatabaseBackend) PhaseOutputExists(runID, phaseTemplateID string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.PhaseOutputExists(runID, phaseTemplateID)
}

// dbPhaseOutputToInfo converts db.PhaseOutput to storage.PhaseOutputInfo.
func dbPhaseOutputToInfo(dbOutput *db.PhaseOutput) *PhaseOutputInfo {
	return &PhaseOutputInfo{
		ID:              dbOutput.ID,
		WorkflowRunID:   dbOutput.WorkflowRunID,
		PhaseTemplateID: dbOutput.PhaseTemplateID,
		TaskID:          dbOutput.TaskID,
		Content:         dbOutput.Content,
		ContentHash:     dbOutput.ContentHash,
		OutputVarName:   dbOutput.OutputVarName,
		ArtifactType:    dbOutput.ArtifactType,
		Source:          dbOutput.Source,
		Iteration:       dbOutput.Iteration,
		CreatedAt:       dbOutput.CreatedAt,
		UpdatedAt:       dbOutput.UpdatedAt,
	}
}

// ============================================================================
// Attachment operations
// ============================================================================

// SaveAttachment stores an attachment in the database.
func (d *DatabaseBackend) SaveAttachment(taskID, filename, contentType string, data []byte) (*task.Attachment, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	isImage := isImageContentType(contentType)
	dbAttachment := &db.Attachment{
		TaskID:      taskID,
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		Data:        data,
		IsImage:     isImage,
	}
	if err := d.db.SaveAttachment(dbAttachment); err != nil {
		return nil, err
	}

	return &task.Attachment{
		Filename:    filename,
		Size:        int64(len(data)),
		ContentType: contentType,
		CreatedAt:   dbAttachment.CreatedAt,
		IsImage:     isImage,
	}, nil
}

// GetAttachment retrieves an attachment from the database.
func (d *DatabaseBackend) GetAttachment(taskID, filename string) (*task.Attachment, []byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbAttachment, err := d.db.GetAttachment(taskID, filename)
	if err != nil {
		return nil, nil, err
	}
	if dbAttachment == nil {
		return nil, nil, fmt.Errorf("attachment %s not found", filename)
	}

	attachment := &task.Attachment{
		Filename:    dbAttachment.Filename,
		Size:        dbAttachment.SizeBytes,
		ContentType: dbAttachment.ContentType,
		CreatedAt:   dbAttachment.CreatedAt,
		IsImage:     dbAttachment.IsImage,
	}
	return attachment, dbAttachment.Data, nil
}

// ListAttachments lists attachments for a task.
func (d *DatabaseBackend) ListAttachments(taskID string) ([]*task.Attachment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbAttachments, err := d.db.ListAttachments(taskID)
	if err != nil {
		return nil, err
	}

	attachments := make([]*task.Attachment, len(dbAttachments))
	for i, a := range dbAttachments {
		attachments[i] = &task.Attachment{
			Filename:    a.Filename,
			Size:        a.SizeBytes,
			ContentType: a.ContentType,
			CreatedAt:   a.CreatedAt,
			IsImage:     a.IsImage,
		}
	}
	return attachments, nil
}

// DeleteAttachment removes an attachment.
func (d *DatabaseBackend) DeleteAttachment(taskID, filename string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeleteAttachment(taskID, filename)
}

// ============================================================================
// Helper functions
// ============================================================================

// taskToDBTask converts a task.Task to db.Task.
func taskToDBTask(t *task.Task) *db.Task {
	// Serialize metadata to JSON
	var metadataJSON string
	if len(t.Metadata) > 0 {
		if data, err := json.Marshal(t.Metadata); err == nil {
			metadataJSON = string(data)
		}
	}

	// Serialize quality metrics to JSON
	var qualityJSON string
	if t.Quality != nil {
		if data, err := json.Marshal(t.Quality); err == nil {
			qualityJSON = string(data)
		}
	}

	return &db.Task{
		ID:           t.ID,
		Title:        t.Title,
		Description:  t.Description,
		Weight:       string(t.Weight),
		Status:       string(t.Status),
		CurrentPhase: t.CurrentPhase,
		Branch:       t.Branch,
		TargetBranch: t.TargetBranch,
		Queue:        string(t.GetQueue()),
		Priority:     string(t.GetPriority()),
		Category:     string(t.GetCategory()),
		InitiativeID: t.InitiativeID,
		CreatedAt:    t.CreatedAt,
		StartedAt:    t.StartedAt,
		CompletedAt:  t.CompletedAt,
		UpdatedAt:    t.UpdatedAt,
		Metadata:     metadataJSON,
		Quality:      qualityJSON,
		IsAutomation: t.IsAutomation,
	}
}

// dbTaskToTask converts a db.Task to task.Task.
func dbTaskToTask(dbTask *db.Task) *task.Task {
	// Deserialize metadata from JSON
	var metadata map[string]string
	if dbTask.Metadata != "" {
		_ = json.Unmarshal([]byte(dbTask.Metadata), &metadata)
	}

	// Deserialize quality metrics from JSON
	var quality *task.QualityMetrics
	if dbTask.Quality != "" {
		quality = &task.QualityMetrics{}
		_ = json.Unmarshal([]byte(dbTask.Quality), quality)
	}

	return &task.Task{
		ID:           dbTask.ID,
		Title:        dbTask.Title,
		Description:  dbTask.Description,
		Weight:       task.Weight(dbTask.Weight),
		Status:       task.Status(dbTask.Status),
		CurrentPhase: dbTask.CurrentPhase,
		Branch:       dbTask.Branch,
		TargetBranch: dbTask.TargetBranch,
		Queue:        task.Queue(dbTask.Queue),
		Priority:     task.Priority(dbTask.Priority),
		Category:     task.Category(dbTask.Category),
		InitiativeID: dbTask.InitiativeID,
		CreatedAt:    dbTask.CreatedAt,
		StartedAt:    dbTask.StartedAt,
		CompletedAt:  dbTask.CompletedAt,
		UpdatedAt:    dbTask.UpdatedAt,
		Metadata:     metadata,
		Quality:      quality,
		IsAutomation: dbTask.IsAutomation,
	}
}

// isImageContentType checks if a content type is an image.
func isImageContentType(contentType string) bool {
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml":
		return true
	default:
		return false
	}
}

// TaskExists checks if a task exists in the database.
func (d *DatabaseBackend) TaskExists(id string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbTask, err := d.db.GetTask(id)
	if err != nil {
		return false, err
	}
	return dbTask != nil, nil
}

// LoadAutomationTasks loads only automation tasks (is_automation = 1).
// More efficient than LoadAllTasks followed by filtering.
func (d *DatabaseBackend) LoadAutomationTasks() ([]*task.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	isAutomation := true
	dbTasks, _, err := d.db.ListTasks(db.ListOpts{IsAutomation: &isAutomation})
	if err != nil {
		return nil, fmt.Errorf("list automation tasks: %w", err)
	}

	// Batch load dependencies for automation tasks
	allDeps, err := d.db.GetAllTaskDependencies()
	if err != nil {
		d.logger.Printf("warning: failed to batch load dependencies: %v", err)
		allDeps = make(map[string][]string)
	}

	tasks := make([]*task.Task, 0, len(dbTasks))
	for _, dbTask := range dbTasks {
		t := dbTaskToTask(&dbTask)
		if deps, ok := allDeps[t.ID]; ok {
			t.BlockedBy = deps
		}
		tasks = append(tasks, t)
	}

	return tasks, nil
}

// GetAutomationTaskStats returns counts of automation tasks by status.
// Uses a single aggregated query for efficiency.
func (d *DatabaseBackend) GetAutomationTaskStats() (*db.AutomationTaskStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetAutomationTaskStats()
}

// ============================================================================
// Initiative operations
// ============================================================================

// SaveInitiative saves an initiative to the database.
// All operations (initiative + decisions + tasks + dependencies) are wrapped in a transaction for atomicity.
// This method uses context.Background(). Use SaveInitiativeCtx for cancellation support.
func (d *DatabaseBackend) SaveInitiative(i *initiative.Initiative) error {
	return d.SaveInitiativeCtx(context.Background(), i)
}

// SaveInitiativeCtx saves an initiative to the database with context support.
// All operations (initiative + decisions + tasks + dependencies) are wrapped in a transaction for atomicity.
func (d *DatabaseBackend) SaveInitiativeCtx(ctx context.Context, i *initiative.Initiative) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbInit := initiativeToDBInitiative(i)

	// Wrap all operations in a transaction for atomicity
	return d.db.RunInTx(ctx, func(tx *db.TxOps) error {
		if err := db.SaveInitiativeTx(tx, dbInit); err != nil {
			return fmt.Errorf("save initiative: %w", err)
		}

		// Clear and save decisions
		if err := db.ClearInitiativeDecisionsTx(tx, i.ID); err != nil {
			return fmt.Errorf("clear initiative decisions: %w", err)
		}
		for _, decision := range i.Decisions {
			dbDecision := &db.InitiativeDecision{
				ID:           decision.ID,
				InitiativeID: i.ID,
				DecidedAt:    decision.Date,
				DecidedBy:    decision.By,
				Decision:     decision.Decision,
				Rationale:    decision.Rationale,
			}
			if err := db.AddInitiativeDecisionTx(tx, dbDecision); err != nil {
				return fmt.Errorf("save decision %s: %w", decision.ID, err)
			}
		}

		// Clear and save task references
		if err := db.ClearInitiativeTasksTx(tx, i.ID); err != nil {
			return fmt.Errorf("clear initiative tasks: %w", err)
		}
		for idx, taskRef := range i.Tasks {
			if err := db.AddTaskToInitiativeTx(tx, i.ID, taskRef.ID, idx); err != nil {
				return fmt.Errorf("add task %s to initiative: %w", taskRef.ID, err)
			}
		}

		// Save dependencies (blocked_by)
		if err := db.ClearInitiativeDependenciesTx(tx, i.ID); err != nil {
			return fmt.Errorf("clear initiative dependencies: %w", err)
		}
		for _, depID := range i.BlockedBy {
			if err := db.AddInitiativeDependencyTx(tx, i.ID, depID); err != nil {
				return fmt.Errorf("add initiative dependency %s: %w", depID, err)
			}
		}

		return nil
	})
}

// LoadInitiative loads an initiative from the database.
func (d *DatabaseBackend) LoadInitiative(id string) (*initiative.Initiative, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbInit, err := d.db.GetInitiative(id)
	if err != nil {
		return nil, fmt.Errorf("get initiative: %w", err)
	}
	if dbInit == nil {
		return nil, fmt.Errorf("initiative %s not found", id)
	}

	i := dbInitiativeToInitiative(dbInit)

	// Load decisions
	dbDecisions, err := d.db.GetInitiativeDecisions(id)
	if err != nil {
		d.logger.Printf("warning: failed to get decisions: %v", err)
	} else {
		for _, dbDec := range dbDecisions {
			i.Decisions = append(i.Decisions, initiative.Decision{
				ID:        dbDec.ID,
				Date:      dbDec.DecidedAt,
				By:        dbDec.DecidedBy,
				Decision:  dbDec.Decision,
				Rationale: dbDec.Rationale,
			})
		}
	}

	// Load task references
	taskIDs, err := d.db.GetInitiativeTasks(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative tasks: %v", err)
	} else {
		for _, taskID := range taskIDs {
			dbTask, err := d.db.GetTask(taskID)
			if err != nil || dbTask == nil {
				continue
			}
			i.Tasks = append(i.Tasks, initiative.TaskRef{
				ID:     taskID,
				Title:  dbTask.Title,
				Status: dbTask.Status,
			})
		}
	}

	// Load dependencies (blocked_by)
	deps, err := d.db.GetInitiativeDependencies(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative dependencies: %v", err)
	} else {
		i.BlockedBy = deps
	}

	// Load dependents (blocks)
	dependents, err := d.db.GetInitiativeDependents(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative dependents: %v", err)
	} else {
		i.Blocks = dependents
	}

	return i, nil
}

// LoadAllInitiatives loads all initiatives from the database.
// Uses batch loading to avoid N+1 query patterns.
func (d *DatabaseBackend) LoadAllInitiatives() ([]*initiative.Initiative, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbInits, err := d.db.ListInitiatives(db.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("list initiatives: %w", err)
	}

	// Batch load all related data in parallel queries to avoid N+1
	allDecisions, err := d.db.GetAllInitiativeDecisions()
	if err != nil {
		d.logger.Printf("warning: failed to batch load decisions: %v", err)
		allDecisions = make(map[string][]db.InitiativeDecision)
	}

	allTaskRefs, err := d.db.GetAllInitiativeTaskRefs()
	if err != nil {
		d.logger.Printf("warning: failed to batch load task refs: %v", err)
		allTaskRefs = make(map[string][]db.InitiativeTaskRef)
	}

	allDeps, err := d.db.GetAllInitiativeDependencies()
	if err != nil {
		d.logger.Printf("warning: failed to batch load dependencies: %v", err)
		allDeps = make(map[string][]string)
	}

	allDependents, err := d.db.GetAllInitiativeDependents()
	if err != nil {
		d.logger.Printf("warning: failed to batch load dependents: %v", err)
		allDependents = make(map[string][]string)
	}

	initiatives := make([]*initiative.Initiative, 0, len(dbInits))
	for _, dbInit := range dbInits {
		i := dbInitiativeToInitiative(&dbInit)

		// Use pre-fetched decisions
		if dbDecisions, ok := allDecisions[i.ID]; ok {
			for _, dbDec := range dbDecisions {
				i.Decisions = append(i.Decisions, initiative.Decision{
					ID:        dbDec.ID,
					Date:      dbDec.DecidedAt,
					By:        dbDec.DecidedBy,
					Decision:  dbDec.Decision,
					Rationale: dbDec.Rationale,
				})
			}
		}

		// Use pre-fetched task refs (already joined with task details)
		if taskRefs, ok := allTaskRefs[i.ID]; ok {
			for _, ref := range taskRefs {
				i.Tasks = append(i.Tasks, initiative.TaskRef{
					ID:     ref.TaskID,
					Title:  ref.Title,
					Status: ref.Status,
				})
			}
		}

		// Use pre-fetched dependencies
		if deps, ok := allDeps[i.ID]; ok {
			i.BlockedBy = deps
		}

		// Use pre-fetched dependents
		if dependents, ok := allDependents[i.ID]; ok {
			i.Blocks = dependents
		}

		initiatives = append(initiatives, i)
	}

	return initiatives, nil
}

// DeleteInitiative removes an initiative from the database.
func (d *DatabaseBackend) DeleteInitiative(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.db.DeleteInitiative(id); err != nil {
		return fmt.Errorf("delete initiative: %w", err)
	}
	return nil
}

// InitiativeExists checks if an initiative exists in the database.
func (d *DatabaseBackend) InitiativeExists(id string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	init, err := d.db.GetInitiative(id)
	if err != nil {
		return false, fmt.Errorf("check initiative: %w", err)
	}
	return init != nil, nil
}

// GetNextInitiativeID generates the next initiative ID from the database.
func (d *DatabaseBackend) GetNextInitiativeID() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Get all initiatives and find the max numeric ID
	dbInits, err := d.db.ListInitiatives(db.ListOpts{})
	if err != nil {
		return "", fmt.Errorf("list initiatives: %w", err)
	}

	maxNum := 0
	for _, init := range dbInits {
		var num int
		if _, err := fmt.Sscanf(init.ID, "INIT-%d", &num); err == nil {
			if num > maxNum {
				maxNum = num
			}
		}
	}

	return fmt.Sprintf("INIT-%03d", maxNum+1), nil
}

// ============================================================================
// Initiative helper functions
// ============================================================================

// initiativeToDBInitiative converts an initiative.Initiative to db.Initiative.
func initiativeToDBInitiative(i *initiative.Initiative) *db.Initiative {
	return &db.Initiative{
		ID:               i.ID,
		Title:            i.Title,
		Status:           string(i.Status),
		OwnerInitials:    i.Owner.Initials,
		OwnerDisplayName: i.Owner.DisplayName,
		OwnerEmail:       i.Owner.Email,
		Vision:           i.Vision,
		BranchBase:       i.BranchBase,
		BranchPrefix:     i.BranchPrefix,
		MergeStatus:      i.MergeStatus,
		MergeCommit:      i.MergeCommit,
		CreatedAt:        i.CreatedAt,
		UpdatedAt:        i.UpdatedAt,
	}
}

// dbInitiativeToInitiative converts a db.Initiative to initiative.Initiative.
func dbInitiativeToInitiative(dbInit *db.Initiative) *initiative.Initiative {
	return &initiative.Initiative{
		ID:     dbInit.ID,
		Title:  dbInit.Title,
		Status: initiative.Status(dbInit.Status),
		Owner: initiative.Identity{
			Initials:    dbInit.OwnerInitials,
			DisplayName: dbInit.OwnerDisplayName,
			Email:       dbInit.OwnerEmail,
		},
		Vision:       dbInit.Vision,
		BranchBase:   dbInit.BranchBase,
		BranchPrefix: dbInit.BranchPrefix,
		MergeStatus:  dbInit.MergeStatus,
		MergeCommit:  dbInit.MergeCommit,
		CreatedAt:    dbInit.CreatedAt,
		UpdatedAt:    dbInit.UpdatedAt,
	}
}

// =============================================================================
// Branch Registry Operations
// =============================================================================

// SaveBranch creates or updates a branch in the registry.
func (d *DatabaseBackend) SaveBranch(b *Branch) error {
	// Validate branch name for security
	if err := git.ValidateBranchName(b.Name); err != nil {
		return fmt.Errorf("save branch: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	dbBranch := &db.Branch{
		Name:         b.Name,
		Type:         db.BranchType(b.Type),
		OwnerID:      b.OwnerID,
		CreatedAt:    b.CreatedAt,
		LastActivity: b.LastActivity,
		Status:       db.BranchStatus(b.Status),
	}

	return d.db.SaveBranch(dbBranch)
}

// LoadBranch retrieves a branch by name.
func (d *DatabaseBackend) LoadBranch(name string) (*Branch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbBranch, err := d.db.GetBranch(name)
	if err != nil {
		return nil, err
	}
	if dbBranch == nil {
		return nil, nil
	}

	return &Branch{
		Name:         dbBranch.Name,
		Type:         BranchType(dbBranch.Type),
		OwnerID:      dbBranch.OwnerID,
		CreatedAt:    dbBranch.CreatedAt,
		LastActivity: dbBranch.LastActivity,
		Status:       BranchStatus(dbBranch.Status),
	}, nil
}

// ListBranches returns all tracked branches, optionally filtered.
func (d *DatabaseBackend) ListBranches(opts BranchListOpts) ([]*Branch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOpts := db.BranchListOpts{
		Type:   db.BranchType(opts.Type),
		Status: db.BranchStatus(opts.Status),
	}

	dbBranches, err := d.db.ListBranches(dbOpts)
	if err != nil {
		return nil, err
	}

	branches := make([]*Branch, len(dbBranches))
	for i, dbBranch := range dbBranches {
		branches[i] = &Branch{
			Name:         dbBranch.Name,
			Type:         BranchType(dbBranch.Type),
			OwnerID:      dbBranch.OwnerID,
			CreatedAt:    dbBranch.CreatedAt,
			LastActivity: dbBranch.LastActivity,
			Status:       BranchStatus(dbBranch.Status),
		}
	}

	return branches, nil
}

// UpdateBranchStatus updates a branch's status.
func (d *DatabaseBackend) UpdateBranchStatus(name string, status BranchStatus) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.UpdateBranchStatus(name, db.BranchStatus(status))
}

// UpdateBranchActivity updates a branch's last_activity timestamp.
func (d *DatabaseBackend) UpdateBranchActivity(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.UpdateBranchActivity(name)
}

// DeleteBranch removes a branch from the registry.
func (d *DatabaseBackend) DeleteBranch(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeleteBranch(name)
}

// GetStaleBranches returns branches that haven't had activity since the given time.
func (d *DatabaseBackend) GetStaleBranches(since time.Time) ([]*Branch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbBranches, err := d.db.GetStaleBranches(since)
	if err != nil {
		return nil, err
	}

	branches := make([]*Branch, len(dbBranches))
	for i, dbBranch := range dbBranches {
		branches[i] = &Branch{
			Name:         dbBranch.Name,
			Type:         BranchType(dbBranch.Type),
			OwnerID:      dbBranch.OwnerID,
			CreatedAt:    dbBranch.CreatedAt,
			LastActivity: dbBranch.LastActivity,
			Status:       BranchStatus(dbBranch.Status),
		}
	}

	return branches, nil
}

// TryClaimTaskExecution atomically claims a task for execution.
// Returns error if task is already claimed by another running process.
// This prevents race conditions when multiple resume attempts occur simultaneously.
func (d *DatabaseBackend) TryClaimTaskExecution(ctx context.Context, taskID string, pid int, hostname string) error {
	// Use write lock for entire operation to ensure true atomicity
	// We need read+write to be atomic, not just the SQL UPDATE
	d.mu.Lock()
	defer d.mu.Unlock()

	// Check if task exists and get current state
	task, err := d.db.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	if task == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	// Validate task is in a resumable state
	if !isResumableStatus(task.StateStatus) {
		return fmt.Errorf("task cannot be claimed (status: %s)", task.StateStatus)
	}

	// Store the current PID for atomic comparison in WHERE clause
	currentPID := task.ExecutorPID

	// If task has a PID, check if it's alive
	if currentPID > 0 {
		if state.IsPIDAlive(currentPID) {
			return fmt.Errorf("task execution already claimed by process %d", currentPID)
		}
		// PID is dead, we can claim it
	}

	// Atomic CAS update: claim the task
	// The WHERE clause ensures state hasn't changed since we read it
	now := time.Now()
	heartbeat := now.Format(time.RFC3339)
	result, err := d.db.DB.ExecContext(ctx, `
		UPDATE tasks
		SET state_status = 'running',
		    executor_pid = ?,
		    executor_hostname = ?,
		    last_heartbeat = ?
		WHERE id = ?
		  AND state_status IN ('failed', 'paused', 'blocked', 'running', 'interrupted')
		  AND executor_pid = ?
	`, pid, hostname, heartbeat, taskID, currentPID)
	if err != nil {
		return fmt.Errorf("claim task execution: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check claim result: %w", err)
	}

	if rowsAffected == 0 {
		// Task was claimed by another process between our check and update
		return fmt.Errorf("task claimed by another process (race condition)")
	}

	return nil
}

// isResumableStatus checks if a task status allows claiming for execution.
func isResumableStatus(status string) bool {
	switch status {
	case "failed", "paused", "blocked", "running", "interrupted":
		return true
	default:
		return false
	}
}

// SaveEvent saves a single event to the event log.
func (d *DatabaseBackend) SaveEvent(e *db.EventLog) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveEvent(e)
}

// SaveEvents saves multiple events to the event log in a single transaction.
func (d *DatabaseBackend) SaveEvents(events []*db.EventLog) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveEvents(events)
}

// QueryEvents retrieves events matching the specified filters.
func (d *DatabaseBackend) QueryEvents(opts db.QueryEventsOptions) ([]db.EventLog, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.QueryEvents(opts)
}

// SaveConstitution saves or updates the project's constitution.
func (d *DatabaseBackend) SaveConstitution(content, version string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	c := &db.Constitution{
		Content: content,
		Version: version,
	}
	return d.db.SaveConstitution(c)
}

// LoadConstitution loads the project's constitution content and version.
func (d *DatabaseBackend) LoadConstitution() (content string, version string, err error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	c, err := d.db.LoadConstitution()
	if err != nil {
		return "", "", err
	}
	return c.Content, c.Version, nil
}

// ConstitutionExists checks if a constitution is configured for the project.
func (d *DatabaseBackend) ConstitutionExists() (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.ConstitutionExists()
}

// DeleteConstitution removes the project's constitution.
func (d *DatabaseBackend) DeleteConstitution() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeleteConstitution()
}

// --------- Phase Template Operations ---------

// SavePhaseTemplate creates or updates a phase template.
func (d *DatabaseBackend) SavePhaseTemplate(pt *db.PhaseTemplate) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SavePhaseTemplate(pt)
}

// GetPhaseTemplate retrieves a phase template by ID.
func (d *DatabaseBackend) GetPhaseTemplate(id string) (*db.PhaseTemplate, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetPhaseTemplate(id)
}

// ListPhaseTemplates returns all phase templates.
func (d *DatabaseBackend) ListPhaseTemplates() ([]*db.PhaseTemplate, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.ListPhaseTemplates()
}

// DeletePhaseTemplate removes a phase template (built-ins cannot be deleted).
func (d *DatabaseBackend) DeletePhaseTemplate(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeletePhaseTemplate(id)
}

// --------- Workflow Operations ---------

// SaveWorkflow creates or updates a workflow.
func (d *DatabaseBackend) SaveWorkflow(w *db.Workflow) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveWorkflow(w)
}

// GetWorkflow retrieves a workflow by ID.
func (d *DatabaseBackend) GetWorkflow(id string) (*db.Workflow, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetWorkflow(id)
}

// ListWorkflows returns all workflows.
func (d *DatabaseBackend) ListWorkflows() ([]*db.Workflow, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.ListWorkflows()
}

// DeleteWorkflow removes a workflow (built-ins cannot be deleted).
func (d *DatabaseBackend) DeleteWorkflow(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeleteWorkflow(id)
}

// GetWorkflowPhases returns all phases for a workflow in sequence order.
func (d *DatabaseBackend) GetWorkflowPhases(workflowID string) ([]*db.WorkflowPhase, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetWorkflowPhases(workflowID)
}

// SaveWorkflowPhase creates or updates a workflow-phase link.
func (d *DatabaseBackend) SaveWorkflowPhase(wp *db.WorkflowPhase) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveWorkflowPhase(wp)
}

// DeleteWorkflowPhase removes a phase from a workflow.
func (d *DatabaseBackend) DeleteWorkflowPhase(workflowID, phaseTemplateID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeleteWorkflowPhase(workflowID, phaseTemplateID)
}

// GetWorkflowVariables returns all variables for a workflow.
func (d *DatabaseBackend) GetWorkflowVariables(workflowID string) ([]*db.WorkflowVariable, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetWorkflowVariables(workflowID)
}

// SaveWorkflowVariable creates or updates a workflow variable.
func (d *DatabaseBackend) SaveWorkflowVariable(wv *db.WorkflowVariable) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveWorkflowVariable(wv)
}

// DeleteWorkflowVariable removes a variable from a workflow.
func (d *DatabaseBackend) DeleteWorkflowVariable(workflowID, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeleteWorkflowVariable(workflowID, name)
}

// --------- Workflow Run Operations ---------

// SaveWorkflowRun creates or updates a workflow run.
func (d *DatabaseBackend) SaveWorkflowRun(wr *db.WorkflowRun) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveWorkflowRun(wr)
}

// GetWorkflowRun retrieves a workflow run by ID.
func (d *DatabaseBackend) GetWorkflowRun(id string) (*db.WorkflowRun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetWorkflowRun(id)
}

// ListWorkflowRuns returns workflow runs with optional filtering.
func (d *DatabaseBackend) ListWorkflowRuns(opts db.WorkflowRunListOpts) ([]*db.WorkflowRun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.ListWorkflowRuns(opts)
}

// DeleteWorkflowRun removes a workflow run and its phases.
func (d *DatabaseBackend) DeleteWorkflowRun(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeleteWorkflowRun(id)
}

// GetNextWorkflowRunID generates the next run ID.
func (d *DatabaseBackend) GetNextWorkflowRunID() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.GetNextWorkflowRunID()
}

// GetWorkflowRunPhases returns all phases for a workflow run.
func (d *DatabaseBackend) GetWorkflowRunPhases(runID string) ([]*db.WorkflowRunPhase, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetWorkflowRunPhases(runID)
}

// SaveWorkflowRunPhase creates or updates a run phase.
func (d *DatabaseBackend) SaveWorkflowRunPhase(wrp *db.WorkflowRunPhase) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveWorkflowRunPhase(wrp)
}

// SaveProjectCommand creates or updates a project command.
func (d *DatabaseBackend) SaveProjectCommand(cmd *db.ProjectCommand) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveProjectCommand(cmd)
}

// GetProjectCommand retrieves a project command by name.
func (d *DatabaseBackend) GetProjectCommand(name string) (*db.ProjectCommand, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetProjectCommand(name)
}

// ListProjectCommands returns all project commands.
func (d *DatabaseBackend) ListProjectCommands() ([]*db.ProjectCommand, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.ListProjectCommands()
}

// GetProjectCommandsMap returns all enabled project commands as a map by name.
func (d *DatabaseBackend) GetProjectCommandsMap() (map[string]*db.ProjectCommand, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetProjectCommandsMap()
}

// DeleteProjectCommand removes a project command by name.
func (d *DatabaseBackend) DeleteProjectCommand(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeleteProjectCommand(name)
}

// SetProjectCommandEnabled enables or disables a project command.
func (d *DatabaseBackend) SetProjectCommandEnabled(name string, enabled bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SetProjectCommandEnabled(name, enabled)
}

// Ensure DatabaseBackend implements Backend
var _ Backend = (*DatabaseBackend)(nil)
