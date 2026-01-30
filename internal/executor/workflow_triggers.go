// workflow_triggers.go contains trigger evaluation methods for workflow execution.
// This includes before-phase trigger evaluation, lifecycle trigger firing,
// and completion handling with gate-mode triggers.
package executor

import (
	"context"
	"errors"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/trigger"
	"github.com/randalmurphal/orc/internal/workflow"
)

// evaluateBeforePhaseTriggers evaluates before-phase triggers for a workflow phase.
// Returns a result indicating whether the phase is blocked and any updated variables.
// When no trigger runner is set or no triggers are configured, returns a non-blocking result.
func (we *WorkflowExecutor) evaluateBeforePhaseTriggers(
	ctx context.Context,
	phase *workflow.WorkflowPhase,
	t *orcv1.Task,
	vars map[string]string,
) *trigger.BeforePhaseTriggerResult {
	// No trigger runner configured
	if we.triggerRunner == nil {
		return &trigger.BeforePhaseTriggerResult{
			UpdatedVars: vars,
		}
	}

	// No triggers on this phase
	if len(phase.BeforeTriggers) == 0 {
		return &trigger.BeforePhaseTriggerResult{
			UpdatedVars: vars,
		}
	}

	result, err := we.triggerRunner.RunBeforePhaseTriggers(ctx, phase.PhaseTemplateID, phase.BeforeTriggers, vars, t)
	if err != nil {
		// Check if it's a gate rejection
		var rejErr *trigger.GateRejectionError
		if errors.As(err, &rejErr) {
			blocked := &trigger.BeforePhaseTriggerResult{
				Blocked:       true,
				BlockedReason: rejErr.Reason,
			}
			if result != nil {
				blocked.UpdatedVars = result.UpdatedVars
			}
			return blocked
		}
		// Infrastructure error — log and continue (don't block the phase)
		we.logger.Warn("before-phase trigger evaluation failed",
			"phase", phase.PhaseTemplateID,
			"error", err)
		if result != nil {
			return result
		}
		return &trigger.BeforePhaseTriggerResult{
			UpdatedVars: vars,
		}
	}

	if result != nil {
		return result
	}
	return &trigger.BeforePhaseTriggerResult{
		UpdatedVars: vars,
	}
}

// fireLifecycleTriggers fires workflow-level lifecycle triggers for a given event.
// Nil-safe: no-ops when workflow is nil, has no triggers, or no trigger runner is set.
// Errors from reaction-mode triggers are logged but don't affect the caller.
func (we *WorkflowExecutor) fireLifecycleTriggers(
	ctx context.Context,
	event workflow.WorkflowTriggerEvent,
	wf *workflow.Workflow,
	t *orcv1.Task,
) {
	if we.triggerRunner == nil || wf == nil || len(wf.Triggers) == 0 {
		return
	}

	if err := we.triggerRunner.RunLifecycleTriggers(ctx, event, wf.Triggers, t); err != nil {
		we.logger.Warn("lifecycle trigger failed",
			"event", event,
			"error", err)
	}
}

// handleCompletionWithTriggers handles task completion with optional gate-mode triggers.
// If a gate-mode on_task_completed trigger rejects, the task is set to BLOCKED instead of COMPLETED.
func (we *WorkflowExecutor) handleCompletionWithTriggers(
	ctx context.Context,
	wf *workflow.Workflow,
	t *orcv1.Task,
) error {
	if we.triggerRunner == nil || wf == nil || len(wf.Triggers) == 0 {
		return nil
	}

	err := we.triggerRunner.RunLifecycleTriggers(ctx, workflow.WorkflowTriggerEventOnTaskCompleted, wf.Triggers, t)
	if err != nil {
		var rejErr *trigger.GateRejectionError
		if errors.As(err, &rejErr) {
			// Gate rejected: set task to BLOCKED
			t.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
			task.UpdateTimestampProto(t)
			if saveErr := we.backend.SaveTask(t); saveErr != nil {
				we.logger.Error("failed to save blocked task after gate rejection",
					"task_id", t.Id, "error", saveErr)
			}
			return err
		}
		// Non-gate error: log and continue
		we.logger.Warn("completion trigger failed",
			"task_id", t.Id, "error", err)
	}

	return nil
}
