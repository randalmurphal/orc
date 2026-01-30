// cmd_trigger.go provides lifecycle trigger support for CLI commands.
package cli

import (
	"context"
	"fmt"
	"log/slog"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/trigger"
	"github.com/randalmurphal/orc/internal/workflow"
)

// CLILifecycleTriggerRunner evaluates lifecycle triggers from CLI commands.
type CLILifecycleTriggerRunner interface {
	RunLifecycleTriggers(ctx context.Context, event workflow.WorkflowTriggerEvent, triggers []workflow.WorkflowTrigger, task *orcv1.Task) error
}

// CLIInitiativeTriggerRunner evaluates initiative-planned triggers from CLI commands.
type CLIInitiativeTriggerRunner interface {
	RunInitiativePlannedTrigger(ctx context.Context, triggers []workflow.WorkflowTrigger, initiativeID string, taskIDs []string) error
}

// fireOnTaskCreatedTrigger fires on_task_created lifecycle trigger after task creation.
// If the gate rejects, sets the task to BLOCKED and returns the rejection message.
// Returns empty string on success or no triggers.
func fireOnTaskCreatedTrigger(
	runner CLILifecycleTriggerRunner,
	backend storage.Backend,
	t *orcv1.Task,
) string {
	if runner == nil {
		return ""
	}
	if t.WorkflowId == nil || *t.WorkflowId == "" {
		return ""
	}

	err := runner.RunLifecycleTriggers(
		context.Background(),
		workflow.WorkflowTriggerEventOnTaskCreated,
		nil, // Triggers are resolved by the runner
		t,
	)
	if err != nil {
		var rejErr *trigger.GateRejectionError
		if isGateRejectionErr(err, &rejErr) {
			// Gate rejected: set task to BLOCKED
			t.Status = orcv1.TaskStatus_TASK_STATUS_BLOCKED
			task.UpdateTimestampProto(t)
			if saveErr := backend.SaveTask(t); saveErr != nil {
				slog.Error("failed to save blocked task after gate rejection",
					"task_id", t.Id, "error", saveErr)
			}
			return fmt.Sprintf("Task %s BLOCKED: %s", t.Id, rejErr.Reason)
		}
		// Non-gate error: log warning, task still created
		slog.Warn("on_task_created trigger failed", "task_id", t.Id, "error", err)
	}
	return ""
}

// isGateRejectionErr checks if an error is a gate rejection.
func isGateRejectionErr(err error, target **trigger.GateRejectionError) bool {
	if err == nil {
		return false
	}
	for e := err; e != nil; {
		if gre, ok := e.(*trigger.GateRejectionError); ok {
			*target = gre
			return true
		}
		if unwrap, ok := e.(interface{ Unwrap() error }); ok {
			e = unwrap.Unwrap()
		} else {
			break
		}
	}
	return false
}
