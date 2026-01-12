package executor

import (
	"context"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// PhaseExecutor defines the interface for executing a single phase.
// Different implementations provide varying levels of session management
// and checkpointing based on task weight.
type PhaseExecutor interface {
	// Execute runs a phase to completion.
	// Returns a Result containing the phase outcome.
	Execute(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error)

	// Name returns a human-readable name for this executor type.
	Name() string
}

// ExecutorConfig provides weight-specific execution configuration.
type ExecutorConfig struct {
	// MaxIterations limits the number of LLM turns per phase.
	MaxIterations int

	// CheckpointInterval determines how often to save state.
	// 0 = only on phase complete, 1 = every iteration.
	CheckpointInterval int

	// SessionPersistence controls whether sessions are saved for resume.
	SessionPersistence bool

	// Model specifies which model to use (empty = default).
	Model string

	// TargetBranch is the target branch for merging (used in prompt templates).
	// Defaults to "main" if empty.
	TargetBranch string
}

// GetTargetBranch returns the target branch, defaulting to "main" if not set.
func (c ExecutorConfig) GetTargetBranch() string {
	if c.TargetBranch == "" {
		return "main"
	}
	return c.TargetBranch
}

// DefaultConfigForWeight returns the recommended configuration for a task weight.
func DefaultConfigForWeight(weight task.Weight) ExecutorConfig {
	switch weight {
	case task.WeightTrivial:
		return ExecutorConfig{
			MaxIterations:      5,
			CheckpointInterval: 0,
			SessionPersistence: false,
		}
	case task.WeightSmall:
		return ExecutorConfig{
			MaxIterations:      10,
			CheckpointInterval: 0,
			SessionPersistence: false,
		}
	case task.WeightMedium:
		return ExecutorConfig{
			MaxIterations:      20,
			CheckpointInterval: 0,
			SessionPersistence: false,
		}
	case task.WeightLarge:
		return ExecutorConfig{
			MaxIterations:      30,
			CheckpointInterval: 1,
			SessionPersistence: true,
		}
	case task.WeightGreenfield:
		return ExecutorConfig{
			MaxIterations:      50,
			CheckpointInterval: 1,
			SessionPersistence: true,
		}
	default:
		// Default to medium config
		return ExecutorConfig{
			MaxIterations:      20,
			CheckpointInterval: 0,
			SessionPersistence: false,
		}
	}
}

// ExecutorType represents the type of phase executor.
type ExecutorType string

const (
	ExecutorTypeTrivial  ExecutorType = "trivial"
	ExecutorTypeStandard ExecutorType = "standard"
	ExecutorTypeFull     ExecutorType = "full"
)

// ExecutorTypeForWeight returns the recommended executor type for a task weight.
func ExecutorTypeForWeight(weight task.Weight) ExecutorType {
	switch weight {
	case task.WeightTrivial:
		return ExecutorTypeTrivial
	case task.WeightSmall, task.WeightMedium:
		return ExecutorTypeStandard
	case task.WeightLarge, task.WeightGreenfield:
		return ExecutorTypeFull
	default:
		return ExecutorTypeStandard
	}
}
