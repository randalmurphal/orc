package trigger

import (
	"context"
	"fmt"
	"log/slog"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/workflow"
)

// TriggerRunner evaluates triggers using an agent executor.
// All call sites (executor, CLI, API) use this runner for consistent behavior.
type TriggerRunner struct {
	backend   storage.Backend
	logger    *slog.Logger
	executor  AgentExecutor
	publisher events.Publisher
}

// TriggerRunnerOption configures a TriggerRunner.
type TriggerRunnerOption func(*TriggerRunner)

// WithAgentExecutor sets the agent executor for trigger evaluation.
func WithAgentExecutor(executor AgentExecutor) TriggerRunnerOption {
	return func(r *TriggerRunner) {
		r.executor = executor
	}
}

// WithEventPublisher sets the event publisher for trigger event logging.
func WithEventPublisher(pub events.Publisher) TriggerRunnerOption {
	return func(r *TriggerRunner) {
		r.publisher = pub
	}
}

// NewTriggerRunner creates a new TriggerRunner.
func NewTriggerRunner(backend storage.Backend, logger *slog.Logger, opts ...TriggerRunnerOption) *TriggerRunner {
	r := &TriggerRunner{
		backend: backend,
		logger:  logger,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// RunLifecycleTriggers evaluates workflow-level lifecycle triggers for a given event.
// Triggers are filtered by event type, executed sequentially. Gate-mode triggers
// that reject return a GateRejectionError. Reaction-mode triggers fire in goroutines.
func (r *TriggerRunner) RunLifecycleTriggers(
	ctx context.Context,
	event workflow.WorkflowTriggerEvent,
	triggers []workflow.WorkflowTrigger,
	task *orcv1.Task,
) error {
	if len(triggers) == 0 {
		return nil
	}

	taskID := ""
	if task != nil {
		taskID = task.Id
	}

	for _, trig := range triggers {
		// Filter by event type
		if trig.Event != event {
			continue
		}

		// Skip disabled triggers
		if !trig.Enabled {
			continue
		}

		// Skip triggers with empty agent ID
		if trig.AgentID == "" {
			r.logger.Warn("skipping trigger with empty agent ID", "event", event)
			continue
		}

		// Determine effective mode (default to gate)
		mode := trig.Mode
		if mode == "" {
			mode = workflow.GateModeGate
		}

		input := &TriggerInput{
			TaskID: taskID,
			Event:  string(event),
		}

		if mode == workflow.GateModeReaction {
			// Reaction mode: fire and forget in goroutine
			r.fireReaction(ctx, trig.AgentID, input, taskID)
			continue
		}

		// Gate mode: execute synchronously
		result, err := r.executeTrigger(ctx, trig.AgentID, input, taskID)
		if err != nil {
			return fmt.Errorf("execute trigger agent %s: %w", trig.AgentID, err)
		}

		if result.ParseError != nil {
			return fmt.Errorf("parse trigger output for %s: %w", trig.AgentID, result.ParseError)
		}

		if !result.Approved {
			return &GateRejectionError{
				AgentID: trig.AgentID,
				Reason:  result.Reason,
			}
		}
	}

	return nil
}

// RunBeforePhaseTriggers evaluates before-phase triggers for a specific phase.
// Returns updated variables (with trigger output merged in) and an error if a gate blocks.
// Infrastructure failures (agent errors) log a warning and continue (don't block the phase).
func (r *TriggerRunner) RunBeforePhaseTriggers(
	ctx context.Context,
	phase string,
	triggers []workflow.BeforePhaseTrigger,
	vars map[string]string,
	task *orcv1.Task,
) (map[string]string, error) {
	// Copy vars to avoid mutating the input
	updatedVars := make(map[string]string, len(vars))
	for k, v := range vars {
		updatedVars[k] = v
	}

	if len(triggers) == 0 {
		return updatedVars, nil
	}

	taskID := ""
	if task != nil {
		taskID = task.Id
	}

	for _, trig := range triggers {
		// Skip triggers with empty agent ID
		if trig.AgentID == "" {
			r.logger.Warn("skipping before-phase trigger with empty agent ID", "phase", phase)
			continue
		}

		// Determine effective mode (default to gate)
		mode := trig.Mode
		if mode == "" {
			mode = workflow.GateModeGate
		}

		input := &TriggerInput{
			TaskID:    taskID,
			Phase:     phase,
			Variables: vars,
		}

		if mode == workflow.GateModeReaction {
			// Reaction mode: fire and forget, never blocks
			r.fireReaction(ctx, trig.AgentID, input, taskID)
			continue
		}

		// Gate mode: execute synchronously
		result, err := r.executeTrigger(ctx, trig.AgentID, input, taskID)
		if err != nil {
			// Per spec SC-1: "Agent error → log warning, continue phase execution"
			r.logger.Warn("before-phase trigger agent failed, continuing",
				"agent", trig.AgentID, "phase", phase, "error", err)
			continue
		}

		if result.ParseError != nil {
			r.logger.Warn("before-phase trigger parse error, continuing",
				"agent", trig.AgentID, "phase", phase, "error", result.ParseError)
			continue
		}

		if !result.Approved {
			return updatedVars, &GateRejectionError{
				AgentID: trig.AgentID,
				Reason:  result.Reason,
			}
		}

		// Capture output into variable if configured
		if trig.OutputConfig != nil && trig.OutputConfig.VariableName != "" && result.Output != "" {
			updatedVars[trig.OutputConfig.VariableName] = result.Output
		}
	}

	return updatedVars, nil
}

// RunInitiativePlannedTrigger evaluates on_initiative_planned triggers.
func (r *TriggerRunner) RunInitiativePlannedTrigger(
	ctx context.Context,
	triggers []workflow.WorkflowTrigger,
	initiativeID string,
	taskIDs []string,
) error {
	if len(triggers) == 0 {
		return nil
	}

	for _, trig := range triggers {
		if trig.Event != workflow.WorkflowTriggerEventOnInitiativePlanned {
			continue
		}
		if !trig.Enabled {
			continue
		}
		if trig.AgentID == "" {
			r.logger.Warn("skipping initiative trigger with empty agent ID")
			continue
		}

		input := &TriggerInput{
			Event: string(workflow.WorkflowTriggerEventOnInitiativePlanned),
			ExtraFields: map[string]string{
				"initiative_id": initiativeID,
			},
		}

		mode := trig.Mode
		if mode == "" {
			mode = workflow.GateModeGate
		}

		if mode == workflow.GateModeReaction {
			r.fireReaction(ctx, trig.AgentID, input, "")
			continue
		}

		result, err := r.executeTrigger(ctx, trig.AgentID, input, "")
		if err != nil {
			return fmt.Errorf("execute initiative trigger agent %s: %w", trig.AgentID, err)
		}
		if result.ParseError != nil {
			return fmt.Errorf("parse initiative trigger output for %s: %w", trig.AgentID, result.ParseError)
		}
		if !result.Approved {
			return &GateRejectionError{
				AgentID: trig.AgentID,
				Reason:  result.Reason,
			}
		}
	}

	return nil
}

// executeTrigger runs a single trigger agent synchronously and publishes events.
// Respects context cancellation — if the context is done before the agent returns,
// the call returns the context error.
func (r *TriggerRunner) executeTrigger(ctx context.Context, agentID string, input *TriggerInput, taskID string) (*TriggerResult, error) {
	r.publishEvent(EventTriggerStarted, taskID, map[string]string{
		"agent_id": agentID,
		"event":    input.Event,
		"phase":    input.Phase,
	})

	type execResult struct {
		result *TriggerResult
		err    error
	}
	ch := make(chan execResult, 1)
	go func() {
		result, err := r.executor.ExecuteTriggerAgent(ctx, agentID, input)
		ch <- execResult{result, err}
	}()

	select {
	case <-ctx.Done():
		r.publishEvent(EventTriggerFailed, taskID, map[string]string{
			"agent_id": agentID,
			"error":    ctx.Err().Error(),
		})
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			r.publishEvent(EventTriggerFailed, taskID, map[string]string{
				"agent_id": agentID,
				"error":    res.err.Error(),
			})
			return nil, res.err
		}
		r.publishEvent(EventTriggerCompleted, taskID, map[string]string{
			"agent_id": agentID,
			"approved": fmt.Sprintf("%v", res.result.Approved),
		})
		return res.result, nil
	}
}

// fireReaction launches a trigger agent in a background goroutine with panic recovery.
func (r *TriggerRunner) fireReaction(ctx context.Context, agentID string, input *TriggerInput, taskID string) {
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				r.logger.Error("reaction trigger panicked",
					"agent", agentID, "panic", rec)
			}
		}()

		result, err := r.executor.ExecuteTriggerAgent(ctx, agentID, input)
		if err != nil {
			r.logger.Warn("reaction trigger failed",
				"agent", agentID, "error", err)
			return
		}
		if result != nil && !result.Approved {
			r.logger.Info("reaction trigger rejected (ignored)",
				"agent", agentID, "reason", result.Reason)
		}
	}()
}

// publishEvent publishes a trigger event if a publisher is configured.
func (r *TriggerRunner) publishEvent(eventType events.EventType, taskID string, data any) {
	if r.publisher == nil {
		return
	}
	r.publisher.Publish(events.NewEvent(eventType, taskID, data))
}
