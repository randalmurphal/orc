package executor

import (
	"errors"
	"fmt"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

func joinExecutionError(base error, action string, err error) error {
	if err == nil {
		return base
	}
	wrapped := fmt.Errorf("%s: %w", action, err)
	if base == nil {
		return wrapped
	}
	return errors.Join(base, wrapped)
}

func combineExecutionErrors(base error, errs ...error) error {
	combined := base
	for _, err := range errs {
		if err == nil {
			continue
		}
		if combined == nil {
			combined = err
			continue
		}
		combined = errors.Join(combined, err)
	}
	return combined
}

func (we *WorkflowExecutor) saveTaskStrict(t *orcv1.Task, action string) error {
	if t == nil {
		return nil
	}
	if we.backend == nil {
		return fmt.Errorf("%s: backend not configured", action)
	}
	if err := we.backend.SaveTask(t); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

func (we *WorkflowExecutor) saveWorkflowRunPhaseStrict(runPhase *db.WorkflowRunPhase, action string) error {
	if runPhase == nil {
		return nil
	}
	if we.backend == nil {
		return fmt.Errorf("%s: backend not configured", action)
	}
	if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

func (we *WorkflowExecutor) saveWorkflowRunStrict(run *db.WorkflowRun, action string) error {
	if run == nil {
		return nil
	}
	if we.backend == nil {
		return fmt.Errorf("%s: backend not configured", action)
	}
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}

func (we *WorkflowExecutor) clearTaskExecutorStrict(taskID, action string) error {
	if taskID == "" {
		return nil
	}
	if we.backend == nil {
		return fmt.Errorf("%s: backend not configured", action)
	}
	if err := we.backend.ClearTaskExecutor(taskID); err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}
