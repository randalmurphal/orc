// Package executor provides the execution engine for orc.
// This file contains phase execution methods for the Executor type.
package executor

import (
	"context"

	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// ExecutePhase runs a single phase using session-based execution.
// Delegates to the weight-appropriate executor (trivial, standard, or full).
func (e *Executor) ExecutePhase(ctx context.Context, t *task.Task, p *Phase, s *state.State) (*Result, error) {
	// Get the appropriate executor for this task's weight
	executor := e.getPhaseExecutor(t.Weight)

	e.logger.Info("executing phase",
		"phase", p.ID,
		"task", t.ID,
		"weight", t.Weight,
		"executor", executor.Name(),
	)

	// Delegate to the weight-appropriate executor
	return executor.Execute(ctx, t, p, s)
}
