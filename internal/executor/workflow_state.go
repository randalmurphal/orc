// workflow_state.go contains state management for workflow execution.
// This includes failure handling, interrupt handling, cost tracking, and transcript syncing.
package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// failRun marks a run as failed and syncs task status.
func (we *WorkflowExecutor) failRun(run *db.WorkflowRun, t *task.Task, err error) {
	run.Status = string(workflow.RunStatusFailed)
	run.Error = err.Error()
	run.CompletedAt = timePtr(time.Now())
	if saveErr := we.backend.SaveWorkflowRun(run); saveErr != nil {
		we.logger.Error("failed to save run failure", "error", saveErr)
	}

	// Sync task status to Failed
	if t != nil {
		t.Status = task.StatusFailed
		if saveErr := we.backend.SaveTask(t); saveErr != nil {
			we.logger.Error("failed to save task status failed", "task_id", t.ID, "error", saveErr)
		}
		// Publish task updated event for real-time UI updates
		we.publishTaskUpdated(t)
		// Trigger automation event for task failure
		we.triggerAutomationEvent(context.Background(), automation.EventTaskFailed, t, "")
	}
}

// failSetup handles failures during setup phase (before any phase runs).
func (we *WorkflowExecutor) failSetup(run *db.WorkflowRun, t *task.Task, err error) {
	taskID := ""
	if t != nil {
		taskID = t.ID
	}
	we.logger.Error("task setup failed", "task", taskID, "error", err)

	// Clear execution tracking and set error in state
	if we.execState != nil {
		we.execState.ClearExecution()
		we.execState.Error = err.Error()
		if saveErr := we.backend.SaveState(we.execState); saveErr != nil {
			we.logger.Error("failed to save state on setup failure", "error", saveErr)
		}
	}

	// Update task status
	if t != nil {
		t.Status = task.StatusFailed
		if saveErr := we.backend.SaveTask(t); saveErr != nil {
			we.logger.Error("failed to save task on setup failure", "error", saveErr)
		}
	}

	// Update run status
	run.Status = string(workflow.RunStatusFailed)
	run.Error = err.Error()
	run.CompletedAt = timePtr(time.Now())
	if saveErr := we.backend.SaveWorkflowRun(run); saveErr != nil {
		we.logger.Error("failed to save run on setup failure", "error", saveErr)
	}
}

// interruptRun marks a run as cancelled (interrupted by context cancellation) and syncs task status.
// Commits work-in-progress before updating status to preserve changes.
func (we *WorkflowExecutor) interruptRun(run *db.WorkflowRun, t *task.Task, currentPhase string, err error) {
	we.logger.Info("run interrupted", "run_id", run.ID, "phase", currentPhase, "reason", err.Error())

	// Commit work-in-progress before updating state
	if t != nil {
		we.commitWIPOnInterrupt(t, currentPhase)
	}

	run.Status = string(workflow.RunStatusCancelled)
	run.Error = err.Error()
	run.CompletedAt = timePtr(time.Now())
	if saveErr := we.backend.SaveWorkflowRun(run); saveErr != nil {
		we.logger.Error("failed to save run interruption", "error", saveErr)
	}

	// Update execution state
	if we.execState != nil {
		we.execState.InterruptPhase(currentPhase)
		we.execState.Error = fmt.Sprintf("interrupted during %s: %s", currentPhase, err.Error())
		if saveErr := we.backend.SaveState(we.execState); saveErr != nil {
			we.logger.Error("failed to save state on interrupt", "error", saveErr)
		}
	}

	// Sync task status to Paused (can be resumed)
	if t != nil {
		t.Status = task.StatusPaused
		if saveErr := we.backend.SaveTask(t); saveErr != nil {
			we.logger.Error("failed to save task status paused", "task_id", t.ID, "error", saveErr)
		}
		// Publish task updated event for real-time UI updates
		we.publishTaskUpdated(t)
	}
}

// commitWIPOnInterrupt commits any work-in-progress and pushes to remote.
func (we *WorkflowExecutor) commitWIPOnInterrupt(t *task.Task, phaseID string) {
	gitSvc := we.worktreeGit
	if gitSvc == nil {
		gitSvc = we.gitOps
	}
	if gitSvc == nil {
		return
	}

	// Use CreateCheckpoint which handles staging and committing
	checkpoint, err := gitSvc.CreateCheckpoint(t.ID, phaseID, "interrupted (work in progress)")
	if err != nil {
		// Log but don't fail - checkpoint is best effort
		we.logger.Debug("no checkpoint created on interrupt", "reason", err)
		return
	}
	if checkpoint == nil {
		return
	}

	we.logger.Info("committed WIP on interrupt", "sha", checkpoint.CommitSHA[:min(8, len(checkpoint.CommitSHA))])

	// Store commit SHA in state
	if we.execState != nil && we.execState.Phases != nil {
		if ps := we.execState.Phases[phaseID]; ps != nil {
			ps.CommitSHA = checkpoint.CommitSHA
		}
	}

	// Push to remote with timeout to avoid blocking interrupt
	pushDone := make(chan error, 1)
	go func() {
		pushDone <- gitSvc.Push("origin", t.Branch, false)
	}()

	select {
	case pushErr := <-pushDone:
		if pushErr != nil {
			we.logger.Warn("failed to push on interrupt", "error", pushErr)
		} else {
			we.logger.Info("pushed WIP to remote", "branch", t.Branch)
		}
	case <-time.After(30 * time.Second):
		we.logger.Warn("push timed out on interrupt")
	}
}

// recordCostToGlobal logs cost and token usage to the global database for cross-project analytics.
// Failures are logged but don't interrupt execution.
func (we *WorkflowExecutor) recordCostToGlobal(t *task.Task, phaseID string, result PhaseResult, model string, duration time.Duration) {
	if we.globalDB == nil {
		return // Global DB not available, skip silently
	}

	projectPath := we.workingDir
	if projectPath == "" {
		projectPath = "unknown"
	}

	taskID := ""
	initiativeID := ""
	if t != nil {
		taskID = t.ID
		initiativeID = t.InitiativeID
	}

	entry := db.CostEntry{
		ProjectID:           projectPath,
		TaskID:              taskID,
		Phase:               phaseID,
		Model:               db.DetectModel(model),
		Iteration:           result.Iterations,
		CostUSD:             result.CostUSD,
		InputTokens:         result.InputTokens,
		OutputTokens:        result.OutputTokens,
		CacheCreationTokens: result.CacheCreationTokens,
		CacheReadTokens:     result.CacheReadTokens,
		TotalTokens:         result.InputTokens + result.OutputTokens,
		InitiativeID:        initiativeID,
		DurationMs:          duration.Milliseconds(),
		Timestamp:           time.Now(),
	}

	RecordCostEntry(we.globalDB, entry, we.logger)
}

