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
	// MaxTurns limits the number of LLM turns per phase.
	MaxTurns int

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

// DefaultConfigForWorkflow returns the recommended configuration for a workflow.
func DefaultConfigForWorkflow(workflowID string) ExecutorConfig {
	// Base timeout settings (can be overridden by config)
	baseTurnTimeout := 10 * time.Minute
	baseHeartbeat := 30 * time.Second
	baseIdleTimeout := 2 * time.Minute

	switch workflowID {
	case "implement-trivial":
		return ExecutorConfig{
			MaxTurns:      50,
			CheckpointInterval: 0,
			SessionPersistence: false,
			TurnTimeout:        5 * time.Minute, // Shorter timeout for trivial tasks
			HeartbeatInterval:  baseHeartbeat,
			IdleTimeout:        baseIdleTimeout,
		}
	case "implement-small":
		return ExecutorConfig{
			MaxTurns:      100,
			CheckpointInterval: 0,
			SessionPersistence: false,
			TurnTimeout:        baseTurnTimeout,
			HeartbeatInterval:  baseHeartbeat,
			IdleTimeout:        baseIdleTimeout,
		}
	case "implement-medium":
		return ExecutorConfig{
			MaxTurns:      150,
			CheckpointInterval: 0,
			SessionPersistence: false,
			TurnTimeout:        baseTurnTimeout,
			HeartbeatInterval:  baseHeartbeat,
			IdleTimeout:        baseIdleTimeout,
		}
	case "implement-large":
		return ExecutorConfig{
			MaxTurns:      250,
			CheckpointInterval: 1,
			SessionPersistence: true,
			TurnTimeout:        15 * time.Minute, // Longer for large tasks
			HeartbeatInterval:  baseHeartbeat,
			IdleTimeout:        3 * time.Minute,
		}
	default:
		// Default to medium config
		return ExecutorConfig{
			MaxTurns:      150,
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

