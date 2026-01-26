package storage

import (
	"context"
	"fmt"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
)

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
func (d *DatabaseBackend) LoadAutomationTasks() ([]*orcv1.Task, error) {
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

	tasks := make([]*orcv1.Task, 0, len(dbTasks))
	for _, dbTask := range dbTasks {
		t := dbTaskToProtoTask(&dbTask)
		if deps, ok := allDeps[t.Id]; ok {
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
// Task Operations (using orcv1.Task - the ONLY task type)
// ============================================================================

// SaveTask saves a task to the database.
// This method uses context.Background(). Use SaveTaskCtx for cancellation support.
func (d *DatabaseBackend) SaveTask(t *orcv1.Task) error {
	return d.SaveTaskCtx(context.Background(), t)
}

// SaveTaskCtx saves a task and its execution state to the database with context support.
// All operations (task + dependencies + phases + gates) are wrapped in a transaction for atomicity.
func (d *DatabaseBackend) SaveTaskCtx(ctx context.Context, t *orcv1.Task) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbTask := protoTaskToDBTask(t)

	// Preserve executor fields from existing task
	existingTask, err := d.db.GetTask(t.Id)
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
		if err := db.ClearTaskDependenciesTx(tx, t.Id); err != nil {
			return fmt.Errorf("clear task dependencies: %w", err)
		}
		for _, depID := range t.BlockedBy {
			if err := db.AddTaskDependencyTx(tx, t.Id, depID); err != nil {
				return fmt.Errorf("add task dependency %s: %w", depID, err)
			}
		}

		// Save execution state: phases
		if err := db.ClearPhasesTx(tx, t.Id); err != nil {
			return fmt.Errorf("clear phases: %w", err)
		}
		if t.Execution != nil {
			for phaseID, ps := range t.Execution.Phases {
				dbPhase := protoPhaseToDBPhase(t.Id, phaseID, ps)
				if err := db.SavePhaseTx(tx, dbPhase); err != nil {
					return fmt.Errorf("save phase %s: %w", phaseID, err)
				}
			}

			// Save execution state: gate decisions
			for _, gate := range t.Execution.Gates {
				dbGate := protoGateToDBGate(t.Id, gate)
				if err := db.AddGateDecisionTx(tx, dbGate); err != nil {
					return fmt.Errorf("save gate decision: %w", err)
				}
			}
		}

		return nil
	})
}

// LoadTask loads a task and its execution state from the database.
func (d *DatabaseBackend) LoadTask(id string) (*orcv1.Task, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.loadTaskUnlocked(id)
}

// loadTaskUnlocked loads a task without holding the lock.
// Caller must hold d.mu.RLock() or d.mu.Lock().
func (d *DatabaseBackend) loadTaskUnlocked(id string) (*orcv1.Task, error) {
	dbTask, err := d.db.GetTask(id)
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	if dbTask == nil {
		return nil, fmt.Errorf("task %s not found", id)
	}

	t := dbTaskToProtoTask(dbTask)

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
		if t.Execution == nil {
			t.Execution = &orcv1.ExecutionState{
				Phases: make(map[string]*orcv1.PhaseState),
			}
		}
		for _, dbPhase := range dbPhases {
			t.Execution.Phases[dbPhase.PhaseID] = dbPhaseToProtoPhase(&dbPhase)
		}
	}

	// Load execution state: gates
	dbGates, err := d.db.GetGateDecisions(id)
	if err != nil {
		d.logger.Printf("warning: failed to get gate decisions: %v", err)
	} else {
		t.Execution.Gates = dbGatesToProtoGates(dbGates)
	}

	return t, nil
}

// LoadAllTasks loads all tasks with their execution state from the database.
func (d *DatabaseBackend) LoadAllTasks() ([]*orcv1.Task, error) {
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

	tasks := make([]*orcv1.Task, 0, len(dbTasks))
	for _, dbTask := range dbTasks {
		t := dbTaskToProtoTask(&dbTask)

		// Apply pre-fetched dependencies
		if deps, ok := allDeps[t.Id]; ok {
			t.BlockedBy = deps
		}

		// Apply pre-fetched phases
		if phases, ok := allPhases[t.Id]; ok {
			if t.Execution == nil {
				t.Execution = &orcv1.ExecutionState{
					Phases: make(map[string]*orcv1.PhaseState),
				}
			}
			for _, dbPhase := range phases {
				t.Execution.Phases[dbPhase.PhaseID] = dbPhaseToProtoPhase(&dbPhase)
			}
		}

		// Apply pre-fetched gates
		if gates, ok := allGates[t.Id]; ok {
			t.Execution.Gates = dbGatesToProtoGates(gates)
		}

		tasks = append(tasks, t)
	}

	return tasks, nil
}
