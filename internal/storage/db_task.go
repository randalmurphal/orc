package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
)

// SaveTask saves a task to the database.
// This method uses context.Background(). Use SaveTaskCtx for cancellation support.
func (d *DatabaseBackend) SaveTask(t *task.Task) error {
	return d.SaveTaskCtx(context.Background(), t)
}

// SaveTaskCtx saves a task and its execution state to the database with context support.
// All operations (task + dependencies + phases + gates) are wrapped in a transaction for atomicity.
func (d *DatabaseBackend) SaveTaskCtx(ctx context.Context, t *task.Task) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbTask := taskToDBTask(t)

	// Preserve executor fields from existing task to avoid false orphan detection
	// (executor fields are managed by SetTaskExecutor/ClearTaskExecutor, not SaveTask)
	existingTask, err := d.db.GetTask(t.ID)
	if err == nil && existingTask != nil {
		dbTask.ExecutorPID = existingTask.ExecutorPID
		dbTask.ExecutorHostname = existingTask.ExecutorHostname
		dbTask.ExecutorStartedAt = existingTask.ExecutorStartedAt
		dbTask.LastHeartbeat = existingTask.LastHeartbeat
	}

	return d.db.RunInTx(ctx, func(tx *db.TxOps) error {
		if err := db.SaveTaskTx(tx, dbTask); err != nil {
			return fmt.Errorf("save task: %w", err)
		}

		// Save dependencies
		if err := db.ClearTaskDependenciesTx(tx, t.ID); err != nil {
			return fmt.Errorf("clear task dependencies: %w", err)
		}
		for _, depID := range t.BlockedBy {
			if err := db.AddTaskDependencyTx(tx, t.ID, depID); err != nil {
				return fmt.Errorf("add task dependency %s: %w", depID, err)
			}
		}

		// Save execution state: phases
		if err := db.ClearPhasesTx(tx, t.ID); err != nil {
			return fmt.Errorf("clear phases: %w", err)
		}
		for phaseID, ps := range t.Execution.Phases {
			dbPhase := phaseStateToDBPhase(t.ID, phaseID, ps)
			if err := db.SavePhaseTx(tx, dbPhase); err != nil {
				return fmt.Errorf("save phase %s: %w", phaseID, err)
			}
		}

		// Save execution state: gate decisions
		for _, gate := range t.Execution.Gates {
			dbGate := &db.GateDecision{
				TaskID:    t.ID,
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

// LoadTask loads a task and its execution state from the database.
func (d *DatabaseBackend) LoadTask(id string) (*task.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.loadTaskUnlocked(id)
}

// loadTaskUnlocked loads a task without holding the lock.
// Caller must hold d.mu.RLock() or d.mu.Lock().
func (d *DatabaseBackend) loadTaskUnlocked(id string) (*task.Task, error) {
	dbTask, err := d.db.GetTask(id)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if dbTask == nil {
		return nil, fmt.Errorf("task %s not found", id)
	}

	t := dbTaskToTask(dbTask)

	// Load dependencies
	deps, err := d.db.GetTaskDependencies(id)
	if err != nil {
		d.logger.Printf("warning: failed to get task dependencies: %v", err)
	} else {
		t.BlockedBy = deps
	}

	// Load execution state: phases
	dbPhases, err := d.db.GetPhases(id)
	if err != nil {
		d.logger.Printf("warning: failed to get phases: %v", err)
	} else {
		for _, dbPhase := range dbPhases {
			t.Execution.Phases[dbPhase.PhaseID] = dbPhaseToPhaseState(&dbPhase)
		}
	}

	// Load execution state: gates
	dbGates, err := d.db.GetGateDecisions(id)
	if err != nil {
		d.logger.Printf("warning: failed to get gate decisions: %v", err)
	} else {
		for _, dbGate := range dbGates {
			t.Execution.Gates = append(t.Execution.Gates, task.GateDecision{
				Phase:     dbGate.Phase,
				GateType:  dbGate.GateType,
				Approved:  dbGate.Approved,
				Reason:    dbGate.Reason,
				Timestamp: dbGate.DecidedAt,
			})
		}
	}

	return t, nil
}

// LoadAllTasks loads all tasks with their execution state from the database.
func (d *DatabaseBackend) LoadAllTasks() ([]*task.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbTasks, _, err := d.db.ListTasks(db.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	// Batch load all related data to avoid N+1 queries
	allDeps, err := d.db.GetAllTaskDependencies()
	if err != nil {
		d.logger.Printf("warning: failed to batch load dependencies: %v", err)
		allDeps = make(map[string][]string)
	}

	allPhases, err := d.db.GetAllPhasesGrouped()
	if err != nil {
		d.logger.Printf("warning: failed to batch load phases: %v", err)
		allPhases = make(map[string][]db.Phase)
	}

	allGates, err := d.db.GetAllGateDecisionsGrouped()
	if err != nil {
		d.logger.Printf("warning: failed to batch load gates: %v", err)
		allGates = make(map[string][]db.GateDecision)
	}

	tasks := make([]*task.Task, 0, len(dbTasks))
	for _, dbTask := range dbTasks {
		t := dbTaskToTask(&dbTask)

		// Apply pre-fetched dependencies
		if deps, ok := allDeps[t.ID]; ok {
			t.BlockedBy = deps
		}

		// Apply pre-fetched phases
		if phases, ok := allPhases[t.ID]; ok {
			for _, dbPhase := range phases {
				t.Execution.Phases[dbPhase.PhaseID] = dbPhaseToPhaseState(&dbPhase)
			}
		}

		// Apply pre-fetched gates
		if gates, ok := allGates[t.ID]; ok {
			for _, dbGate := range gates {
				t.Execution.Gates = append(t.Execution.Gates, task.GateDecision{
					Phase:     dbGate.Phase,
					GateType:  dbGate.GateType,
					Approved:  dbGate.Approved,
					Reason:    dbGate.Reason,
					Timestamp: dbGate.DecidedAt,
				})
			}
		}

		tasks = append(tasks, t)
	}

	return tasks, nil
}

// DeleteTask removes a task from the database.
func (d *DatabaseBackend) DeleteTask(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.db.DeleteTask(id); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
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

// GetNextTaskID generates the next task ID from the database.
func (d *DatabaseBackend) GetNextTaskID() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.GetNextTaskID()
}

// UpdateTaskHeartbeat updates the last_heartbeat timestamp for a task.
func (d *DatabaseBackend) UpdateTaskHeartbeat(taskID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.UpdateTaskHeartbeat(taskID)
}

// SetTaskExecutor sets the executor info (PID, hostname, heartbeat) for a task.
func (d *DatabaseBackend) SetTaskExecutor(taskID string, pid int, hostname string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SetTaskExecutor(taskID, pid, hostname)
}

// ClearTaskExecutor clears the executor info for a task.
func (d *DatabaseBackend) ClearTaskExecutor(taskID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.ClearTaskExecutor(taskID)
}

// GetTaskActivityByDate returns task completion counts grouped by date.
func (d *DatabaseBackend) GetTaskActivityByDate(startDate, endDate string) ([]ActivityCount, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbActivity, err := d.db.GetTaskActivityByDate(startDate, endDate)
	if err != nil {
		return nil, err
	}

	result := make([]ActivityCount, len(dbActivity))
	for i, ac := range dbActivity {
		result[i] = ActivityCount{
			Date:  ac.Date,
			Count: ac.Count,
		}
	}
	return result, nil
}

// LoadAutomationTasks loads only automation tasks (is_automation = 1).
func (d *DatabaseBackend) LoadAutomationTasks() ([]*task.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	isAutomation := true
	dbTasks, _, err := d.db.ListTasks(db.ListOpts{IsAutomation: &isAutomation})
	if err != nil {
		return nil, fmt.Errorf("list automation tasks: %w", err)
	}

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
func (d *DatabaseBackend) GetAutomationTaskStats() (*db.AutomationTaskStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetAutomationTaskStats()
}

// TryClaimTaskExecution atomically claims a task for execution.
// Returns error if task is already claimed by another running process.
func (d *DatabaseBackend) TryClaimTaskExecution(ctx context.Context, taskID string, pid int, hostname string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbTask, err := d.db.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("get task: %w", err)
	}
	if dbTask == nil {
		return fmt.Errorf("task %s not found", taskID)
	}

	if !isResumableTaskStatus(dbTask.Status) {
		if dbTask.Status == "created" || dbTask.Status == "planned" {
			return fmt.Errorf("task %s has not been started yet (status: %s) - use 'orc run' instead of 'orc resume'", taskID, dbTask.Status)
		}
		return fmt.Errorf("task %s cannot be resumed (status: %s)", taskID, dbTask.Status)
	}

	currentPID := dbTask.ExecutorPID

	if currentPID > 0 {
		if task.IsPIDAlive(currentPID) {
			return fmt.Errorf("task execution already claimed by process %d", currentPID)
		}
	}

	now := time.Now()
	heartbeat := now.Format(time.RFC3339)
	result, err := d.db.ExecContext(ctx, `
		UPDATE tasks
		SET status = 'running',
		    executor_pid = ?,
		    executor_hostname = ?,
		    last_heartbeat = ?
		WHERE id = ?
		  AND status IN ('running', 'paused', 'blocked', 'failed')
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
		return fmt.Errorf("task claimed by another process (race condition)")
	}

	return nil
}

// isResumableTaskStatus checks if a task status allows claiming for execution.
func isResumableTaskStatus(status string) bool {
	switch status {
	case "running", "paused", "blocked", "failed":
		return true
	default:
		return false
	}
}

// ============================================================================
// Conversion helpers
// ============================================================================

// taskToDBTask converts a task.Task to db.Task.
func taskToDBTask(t *task.Task) *db.Task {
	var metadataJSON string
	if len(t.Metadata) > 0 {
		if data, err := json.Marshal(t.Metadata); err == nil {
			metadataJSON = string(data)
		}
	}

	var qualityJSON string
	if t.Quality != nil {
		if data, err := json.Marshal(t.Quality); err == nil {
			qualityJSON = string(data)
		}
	}

	var retryContextJSON string
	if t.Execution.RetryContext != nil {
		if data, err := json.Marshal(t.Execution.RetryContext); err == nil {
			retryContextJSON = string(data)
		}
	}

	return &db.Task{
		ID:           t.ID,
		Title:        t.Title,
		Description:  t.Description,
		Weight:       string(t.Weight),
		WorkflowID:   t.WorkflowID,
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
		TotalCostUSD: t.Execution.Cost.TotalCostUSD,
		RetryContext: retryContextJSON,
		ExecutorPID:       t.ExecutorPID,
		ExecutorHostname:  t.ExecutorHostname,
		ExecutorStartedAt: t.ExecutorStartedAt,
		LastHeartbeat:     t.LastHeartbeat,
	}
}

// dbTaskToTask converts a db.Task to task.Task.
func dbTaskToTask(dbTask *db.Task) *task.Task {
	var metadata map[string]string
	if dbTask.Metadata != "" {
		_ = json.Unmarshal([]byte(dbTask.Metadata), &metadata)
	}

	var quality *task.QualityMetrics
	if dbTask.Quality != "" {
		quality = &task.QualityMetrics{}
		_ = json.Unmarshal([]byte(dbTask.Quality), quality)
	}

	t := &task.Task{
		ID:           dbTask.ID,
		Title:        dbTask.Title,
		Description:  dbTask.Description,
		Weight:       task.Weight(dbTask.Weight),
		WorkflowID:   dbTask.WorkflowID,
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
		ExecutorPID:       dbTask.ExecutorPID,
		ExecutorHostname:  dbTask.ExecutorHostname,
		ExecutorStartedAt: dbTask.ExecutorStartedAt,
		LastHeartbeat:     dbTask.LastHeartbeat,
		Execution:         task.InitExecutionState(),
	}

	// Deserialize RetryContext into Execution
	if dbTask.RetryContext != "" {
		var retryCtx task.RetryContext
		if err := json.Unmarshal([]byte(dbTask.RetryContext), &retryCtx); err == nil {
			t.Execution.RetryContext = &retryCtx
		}
	}

	// Set cost from task
	t.Execution.Cost.TotalCostUSD = dbTask.TotalCostUSD

	return t
}

// phaseStateToDBPhase converts task.PhaseState to db.Phase.
func phaseStateToDBPhase(taskID, phaseID string, ps *task.PhaseState) *db.Phase {
	var startedAt *time.Time
	if !ps.StartedAt.IsZero() {
		startedAt = &ps.StartedAt
	}
	return &db.Phase{
		TaskID:       taskID,
		PhaseID:      phaseID,
		Status:       string(ps.Status),
		Iterations:   ps.Iterations,
		StartedAt:    startedAt,
		CompletedAt:  ps.CompletedAt,
		InputTokens:  ps.Tokens.InputTokens,
		OutputTokens: ps.Tokens.OutputTokens,
		CostUSD:      0,
		ErrorMessage: ps.Error,
		CommitSHA:    ps.CommitSHA,
		SessionID:    ps.SessionID,
	}
}

// dbPhaseToPhaseState converts db.Phase to task.PhaseState.
func dbPhaseToPhaseState(dbPhase *db.Phase) *task.PhaseState {
	var phaseStartedAt time.Time
	if dbPhase.StartedAt != nil {
		phaseStartedAt = *dbPhase.StartedAt
	}
	return &task.PhaseState{
		Status:      task.PhaseStatus(dbPhase.Status),
		Iterations:  dbPhase.Iterations,
		StartedAt:   phaseStartedAt,
		CompletedAt: dbPhase.CompletedAt,
		Error:       dbPhase.ErrorMessage,
		CommitSHA:   dbPhase.CommitSHA,
		SessionID:   dbPhase.SessionID,
		Tokens: task.TokenUsage{
			InputTokens:  dbPhase.InputTokens,
			OutputTokens: dbPhase.OutputTokens,
		},
	}
}
