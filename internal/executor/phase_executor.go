package executor

import (
	"context"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
)

// PhaseExecutor defines the interface for executing a single phase.
// Different implementations provide varying levels of session management
// and checkpointing based on task weight.
type PhaseExecutor interface {
	// Execute runs a phase to completion.
	// Returns a Result containing the phase outcome.
	Execute(ctx context.Context, t *orcv1.Task, p *PhaseDisplay, s *orcv1.ExecutionState) (*Result, error)

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
	// This is the fallback model if OrcConfig is not set.
	Model string

	// TargetBranch is the target branch for merging (used in prompt templates).
	// Defaults to "main" if empty.
	TargetBranch string

	// TurnTimeout is the maximum duration for a single API turn.
	// 0 means no timeout (uses parent context only).
	TurnTimeout time.Duration

	// HeartbeatInterval is how often to emit progress heartbeats.
	// 0 means no heartbeats.
	HeartbeatInterval time.Duration

	// IdleTimeout is how long without activity before warning.
	// 0 means no idle warnings.
	IdleTimeout time.Duration

	// OrcConfig is a reference to the full orc config.
	// Used for default model and other global settings.
	OrcConfig *config.Config
}

// DefaultConfigForWeight returns the recommended configuration for a task weight.
func DefaultConfigForWeight(weight orcv1.TaskWeight) ExecutorConfig {
	// Base timeout settings (can be overridden by config)
	baseTurnTimeout := 10 * time.Minute
	baseHeartbeat := 30 * time.Second
	baseIdleTimeout := 2 * time.Minute

	switch weight {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL:
		return ExecutorConfig{
			MaxIterations:      5,
			CheckpointInterval: 0,
			SessionPersistence: false,
			TurnTimeout:        5 * time.Minute, // Shorter timeout for trivial tasks
			HeartbeatInterval:  baseHeartbeat,
			IdleTimeout:        baseIdleTimeout,
		}
	case orcv1.TaskWeight_TASK_WEIGHT_SMALL:
		return ExecutorConfig{
			MaxIterations:      10,
			CheckpointInterval: 0,
			SessionPersistence: false,
			TurnTimeout:        baseTurnTimeout,
			HeartbeatInterval:  baseHeartbeat,
			IdleTimeout:        baseIdleTimeout,
		}
	case orcv1.TaskWeight_TASK_WEIGHT_MEDIUM:
		return ExecutorConfig{
			MaxIterations:      20,
			CheckpointInterval: 0,
			SessionPersistence: false,
			TurnTimeout:        baseTurnTimeout,
			HeartbeatInterval:  baseHeartbeat,
			IdleTimeout:        baseIdleTimeout,
		}
	case orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		return ExecutorConfig{
			MaxIterations:      30,
			CheckpointInterval: 1,
			SessionPersistence: true,
			TurnTimeout:        15 * time.Minute, // Longer for large tasks
			HeartbeatInterval:  baseHeartbeat,
			IdleTimeout:        3 * time.Minute,
		}
	default:
		// Default to medium config
		return ExecutorConfig{
			MaxIterations:      20,
			CheckpointInterval: 0,
			SessionPersistence: false,
			TurnTimeout:        baseTurnTimeout,
			HeartbeatInterval:  baseHeartbeat,
			IdleTimeout:        baseIdleTimeout,
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

