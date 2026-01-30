// Package trigger provides lifecycle event trigger evaluation for workflows.
// TriggerRunner is the shared component used by executor, CLI, and API to evaluate
// before-phase triggers and lifecycle triggers (on_task_created, on_task_completed, etc.).
package trigger

import (
	"context"
	"fmt"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/workflow"
)

// Event type constants for trigger execution events.
const (
	EventTriggerStarted   events.EventType = "trigger_started"
	EventTriggerCompleted events.EventType = "trigger_completed"
	EventTriggerFailed    events.EventType = "trigger_failed"
)

// TriggerResult holds the result of executing a single trigger agent.
type TriggerResult struct {
	Approved   bool
	Reason     string
	Output     string
	ParseError error
}

// TriggerInput provides context to the trigger agent.
type TriggerInput struct {
	TaskID      string
	Phase       string
	Event       string
	Variables   map[string]string
	ExtraFields map[string]string
}

// BeforePhaseTriggerResult holds the aggregate result of evaluating before-phase triggers.
type BeforePhaseTriggerResult struct {
	Blocked       bool
	BlockedReason string
	UpdatedVars   map[string]string
}

// GateRejectionError indicates a gate-mode trigger rejected the action.
type GateRejectionError struct {
	AgentID string
	Reason  string
}

func (e *GateRejectionError) Error() string {
	return fmt.Sprintf("trigger gate rejected by %s: %s", e.AgentID, e.Reason)
}

// AgentExecutor is the interface for invoking trigger agents.
type AgentExecutor interface {
	ExecuteTriggerAgent(ctx context.Context, agentID string, input *TriggerInput) (*TriggerResult, error)
}

// Runner defines the interface for trigger evaluation, used by the executor.
// The executor mock returns *BeforePhaseTriggerResult directly; the real TriggerRunner
// returns (map[string]string, error) and the executor wraps the conversion.
type Runner interface {
	RunBeforePhaseTriggers(ctx context.Context, phase string, triggers []workflow.BeforePhaseTrigger, vars map[string]string, task *orcv1.Task) (*BeforePhaseTriggerResult, error)
	RunLifecycleTriggers(ctx context.Context, event workflow.WorkflowTriggerEvent, triggers []workflow.WorkflowTrigger, task *orcv1.Task) error
}
