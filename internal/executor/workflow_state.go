// workflow_state.go contains state management for workflow execution.
// This includes failure handling, interrupt handling, cost tracking, and transcript syncing.
package executor

import (
	"context"
	"fmt"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/project"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// failRun marks a run as failed and syncs task status.
// Commits any work-in-progress before updating status to preserve changes.
func (we *WorkflowExecutor) failRun(run *db.WorkflowRun, t *orcv1.Task, err error) {
	// Commit work-in-progress before marking failed (match interruptRun behavior)
	// This preserves any uncommitted changes so they can be recovered on retry
	if t != nil && run.CurrentPhase != "" {
		we.commitWIPOnInterrupt(t, run.CurrentPhase)
	}

	run.Status = string(workflow.RunStatusFailed)
	run.Error = err.Error()
	run.CompletedAt = timePtr(time.Now())
	if saveErr := we.backend.SaveWorkflowRun(run); saveErr != nil {
		we.logger.Error("failed to save run failure", "error", saveErr)
	}

	// Sync task status to Failed
	if t != nil {
		t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
		task.UpdateTimestampProto(t)
		if saveErr := we.backend.SaveTask(t); saveErr != nil {
			we.logger.Error("failed to save task status failed", "task_id", t.Id, "error", saveErr)
		}
		if signalErr := we.upsertTaskAttentionSignal(t, controlplane.AttentionSignalStatusFailed, err.Error()); signalErr != nil {
			we.logger.Error("failed to save failed-task attention signal", "task_id", t.Id, "error", signalErr)
		}
		// Publish task updated event for real-time UI updates
		we.publishTaskUpdated(t)
		// Trigger automation event for task failure
		we.triggerAutomationEvent(context.Background(), automation.EventTaskFailed, t, "")
		// Fire lifecycle triggers for task failure (fire-and-forget)
		we.fireLifecycleTriggers(context.Background(), workflow.WorkflowTriggerEventOnTaskFailed, we.wf, t)
	}
}

// failGateRejection records the error in execution state, fails the run, and clears the executor.
// Used by the gate handling section when a rejection should terminate the task.
func (we *WorkflowExecutor) failGateRejection(run *db.WorkflowRun, t *orcv1.Task, failErr error) {
	if we.task != nil {
		task.SetErrorProto(we.task.Execution, failErr.Error())
		if saveErr := we.backend.SaveTask(we.task); saveErr != nil {
			we.logger.Warn("failed to save error state", "error", saveErr)
		}
	}
	we.failRun(run, t, failErr)
	if t != nil {
		if clearErr := we.backend.ClearTaskExecutor(t.Id); clearErr != nil {
			we.logger.Warn("failed to clear task executor", "error", clearErr)
		}
	}
}

// failSetup handles failures during setup phase (before any phase runs).
func (we *WorkflowExecutor) failSetup(run *db.WorkflowRun, t *orcv1.Task, err error) {
	taskID := ""
	if t != nil {
		taskID = t.Id
	}
	we.logger.Error("task setup failed", "task", taskID, "error", err)

	// Clear execution tracking on task and set error in state
	if t != nil {
		if clearErr := we.backend.ClearTaskExecutor(t.Id); clearErr != nil {
			we.logger.Warn("failed to clear task executor on setup failure", "error", clearErr)
		}
	}
	if we.task != nil {
		task.SetErrorProto(we.task.Execution, err.Error())
		if saveErr := we.backend.SaveTask(we.task); saveErr != nil {
			we.logger.Error("failed to save state on setup failure", "error", saveErr)
		}
	}

	// Update task status
	if t != nil {
		t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
		task.UpdateTimestampProto(t)
		if saveErr := we.backend.SaveTask(t); saveErr != nil {
			we.logger.Error("failed to save task on setup failure", "error", saveErr)
		}
		if signalErr := we.upsertTaskAttentionSignal(t, controlplane.AttentionSignalStatusFailed, err.Error()); signalErr != nil {
			we.logger.Error("failed to save setup-failure attention signal", "task_id", t.Id, "error", signalErr)
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

func (we *WorkflowExecutor) failTaskAfterCompletionError(t *orcv1.Task, completionErr error) {
	if t == nil || completionErr == nil {
		return
	}

	t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	task.EnsureMetadataProto(t)
	t.Metadata["failed_reason"] = "completion_failed"
	t.Metadata["failed_error"] = completionErr.Error()
	task.UpdateTimestampProto(t)
	if err := we.backend.SaveTask(t); err != nil {
		we.logger.Warn("failed to save failed task", "task", t.Id, "error", err)
	}
	if err := we.upsertTaskAttentionSignal(t, controlplane.AttentionSignalStatusFailed, completionErr.Error()); err != nil {
		we.logger.Error("failed to save completion-failure attention signal", "task_id", t.Id, "error", err)
	}
	we.publishTaskUpdated(t)
}

// interruptRun marks a run as cancelled (interrupted by context cancellation) and syncs task status.
// Commits work-in-progress before updating status to preserve changes.
func (we *WorkflowExecutor) interruptRun(run *db.WorkflowRun, t *orcv1.Task, currentPhase string, err error) {
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
	if we.task != nil {
		task.InterruptPhaseProto(we.task.Execution, currentPhase)
		task.SetErrorProto(we.task.Execution, fmt.Sprintf("interrupted during %s: %s", currentPhase, err.Error()))
		if saveErr := we.backend.SaveTask(we.task); saveErr != nil {
			we.logger.Error("failed to save state on interrupt", "error", saveErr)
		}
	}

	// Sync task status to Paused (can be resumed)
	if t != nil {
		t.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
		task.UpdateTimestampProto(t)
		if saveErr := we.backend.SaveTask(t); saveErr != nil {
			we.logger.Error("failed to save task status paused", "task_id", t.Id, "error", saveErr)
		}
		if resolveErr := we.resolveAttentionSignalsForTask(t.Id, "executor"); resolveErr != nil {
			we.logger.Error("failed to resolve attention signals on interrupt", "task_id", t.Id, "error", resolveErr)
		}
		// Publish task updated event for real-time UI updates
		we.publishTaskUpdated(t)
	}
}

// commitWIPOnInterrupt commits any work-in-progress and pushes to remote.
func (we *WorkflowExecutor) commitWIPOnInterrupt(t *orcv1.Task, phaseID string) {
	gitSvc := we.worktreeGit
	if gitSvc == nil {
		gitSvc = we.gitOps
	}
	if gitSvc == nil {
		return
	}

	// Use CreateCheckpoint which handles staging and committing
	checkpoint, err := gitSvc.CreateCheckpoint(t.Id, phaseID, "interrupted (work in progress)")
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
	if we.task != nil && we.task.Execution != nil && we.task.Execution.Phases != nil {
		task.SetPhaseCommitSHAProto(we.task.Execution, phaseID, checkpoint.CommitSHA)
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
// The context is used to extract the user ID for cost attribution.
func (we *WorkflowExecutor) recordCostToGlobal(ctx context.Context, t *orcv1.Task, phaseID string, result PhaseResult, model, provider string, duration time.Duration) {
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
		taskID = t.Id
		initiativeID = task.GetInitiativeIDProto(t)
	}

	userID, _ := UserIDFromContext(ctx)

	entry := db.CostEntry{
		ProjectID:           projectPath,
		TaskID:              taskID,
		Phase:               phaseID,
		Model:               db.DetectModel(provider, model),
		Provider:            provider,
		Iteration:           result.Iterations,
		CostUSD:             result.CostUSD,
		InputTokens:         result.InputTokens,
		OutputTokens:        result.OutputTokens,
		CacheCreationTokens: result.CacheCreationTokens,
		CacheReadTokens:     result.CacheReadTokens,
		TotalTokens:         result.InputTokens + result.OutputTokens + result.CacheCreationTokens + result.CacheReadTokens,
		InitiativeID:        initiativeID,
		DurationMs:          duration.Milliseconds(),
		UserID:              userID,
		Timestamp:           time.Now(),
	}

	RecordCostEntry(we.globalDB, entry, we.logger)
}

func (we *WorkflowExecutor) upsertTaskAttentionSignal(t *orcv1.Task, status string, summary string) error {
	if t == nil {
		return nil
	}

	signal := &controlplane.PersistedAttentionSignal{
		Kind:          controlplane.AttentionSignalKindBlocker,
		Status:        status,
		ReferenceType: controlplane.AttentionSignalReferenceTypeTask,
		ReferenceID:   t.Id,
		Title:         t.Title,
		Summary:       summary,
	}
	if signal.Summary == "" {
		signal.Summary = attentionSummaryForTask(t)
	}

	if err := we.backend.SaveAttentionSignal(signal); err != nil {
		return err
	}

	we.publishAttentionSignalCreated(t.Id, signal)
	return nil
}

func (we *WorkflowExecutor) resolveAttentionSignalsForTask(taskID string, resolvedBy string) error {
	if taskID == "" {
		return nil
	}

	signals, err := we.backend.LoadActiveAttentionSignals()
	if err != nil {
		return fmt.Errorf("load active attention signals for task %s: %w", taskID, err)
	}

	for _, signal := range signals {
		if signal == nil {
			continue
		}
		if signal.ReferenceType != controlplane.AttentionSignalReferenceTypeTask || signal.ReferenceID != taskID {
			continue
		}
		resolvedSignal, resolveErr := we.backend.ResolveAttentionSignal(signal.ID, resolvedBy)
		if resolveErr != nil {
			return fmt.Errorf("resolve attention signal %s for task %s: %w", signal.ID, taskID, resolveErr)
		}
		we.publishAttentionSignalResolved(taskID, resolvedSignal)
	}

	return nil
}

func (we *WorkflowExecutor) publishAttentionSignalCreated(taskID string, signal *controlplane.PersistedAttentionSignal) {
	if we.publisher == nil || signal == nil {
		return
	}

	event := events.NewProjectEvent(
		events.EventAttentionSignalCreated,
		we.projectIDForEvents(),
		taskID,
		events.AttentionSignalCreatedData{
			SignalID:      signal.ID,
			Kind:          string(signal.Kind),
			Status:        signal.Status,
			ReferenceType: signal.ReferenceType,
			ReferenceID:   signal.ReferenceID,
			Title:         signal.Title,
			Summary:       signal.Summary,
		},
	)
	we.publisher.Publish(event)
}

func (we *WorkflowExecutor) publishAttentionSignalResolved(taskID string, signal *controlplane.PersistedAttentionSignal) {
	if we.publisher == nil || signal == nil || signal.ResolvedAt == nil {
		return
	}

	event := events.NewProjectEvent(
		events.EventAttentionSignalResolved,
		we.projectIDForEvents(),
		taskID,
		events.AttentionSignalResolvedData{
			SignalID:      signal.ID,
			Kind:          string(signal.Kind),
			ReferenceType: signal.ReferenceType,
			ReferenceID:   signal.ReferenceID,
			ResolvedBy:    signal.ResolvedBy,
			ResolvedAt:    *signal.ResolvedAt,
		},
	)
	we.publisher.Publish(event)
}

func (we *WorkflowExecutor) projectIDForEvents() string {
	if we.projectDB == nil || we.projectDB.ProjectDir() == "" {
		return ""
	}

	projectID, err := project.ResolveProjectID(we.projectDB.ProjectDir())
	if err != nil {
		return ""
	}
	return projectID
}
