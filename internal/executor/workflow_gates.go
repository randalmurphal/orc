// workflow_gates.go contains gate evaluation and related utilities for workflow execution.
// This includes gate evaluation, event publishing, resource tracking, and automation triggers.
package executor

import (
	"context"
	"fmt"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
)

// GateEvaluationResult contains the result of gate evaluation.
type GateEvaluationResult struct {
	Approved   bool
	Pending    bool
	Reason     string
	RetryPhase string // If not approved and has retry target
}

// evaluatePhaseGate evaluates the gate for a completed phase.
func (we *WorkflowExecutor) evaluatePhaseGate(ctx context.Context, tmpl *db.PhaseTemplate, phase *db.WorkflowPhase, output string, t *orcv1.Task) (*GateEvaluationResult, error) {
	result := &GateEvaluationResult{}

	// Determine effective gate type
	gateType := tmpl.GateType
	if phase.GateTypeOverride != "" {
		gateType = phase.GateTypeOverride
	}

	// If no gate or auto with auto-approve, just approve
	if gateType == "" || gateType == "auto" {
		if we.orcConfig != nil && we.orcConfig.Gates.AutoApproveOnSuccess {
			result.Approved = true
			result.Reason = "auto-approved on success"
			return result, nil
		}
	}

	// Create gate struct for evaluator
	g := &gate.Gate{
		Type: gate.GateType(gateType),
	}

	// Evaluate
	decision, err := we.gateEvaluator.Evaluate(ctx, g, output)
	if err != nil {
		return nil, fmt.Errorf("gate evaluation: %w", err)
	}

	result.Approved = decision.Approved
	result.Pending = decision.Pending
	result.Reason = decision.Reason

	// If not approved, check for retry target
	if !result.Approved && !result.Pending {
		if tmpl.RetryFromPhase != "" {
			result.RetryPhase = tmpl.RetryFromPhase
		} else if we.orcConfig != nil {
			// Fall back to config-based retry map
			result.RetryPhase = we.orcConfig.ShouldRetryFrom(tmpl.ID)
		}
	}

	return result, nil
}

// publishTaskUpdated publishes a task_updated event for real-time UI updates.
// Uses the EventTaskUpdated type which the frontend listens for.
func (we *WorkflowExecutor) publishTaskUpdated(t *orcv1.Task) {
	if we.publisher == nil || t == nil {
		return
	}
	we.publisher.Publish(events.NewEvent(events.EventTaskUpdated, t.Id, t))
}

// runResourceAnalysis runs the resource tracker analysis after task completion.
// Called via defer in Run() to run regardless of success or failure.
func (we *WorkflowExecutor) runResourceAnalysis() {
	RunResourceAnalysis(we.resourceTracker, we.logger)
}

// triggerAutomationEvent sends an event to the automation service if configured.
func (we *WorkflowExecutor) triggerAutomationEvent(ctx context.Context, eventType string, t *orcv1.Task, phase string) {
	if we.automationSvc == nil || t == nil {
		return
	}

	event := &automation.Event{
		Type:     eventType,
		TaskID:   t.Id,
		Weight:   t.Weight.String(),
		Category: t.Category.String(),
		Phase:    phase,
	}

	if err := we.automationSvc.HandleEvent(ctx, event); err != nil {
		we.logger.Warn("automation event handling failed",
			"event", eventType,
			"task", t.Id,
			"error", err)
	}
}
