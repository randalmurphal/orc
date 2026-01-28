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

	// Use gate resolver if available, fall back to legacy resolution
	gateType := we.resolveGateType(tmpl, phase, t)

	// If no gate or auto with auto-approve, just approve
	if gateType == "" || gateType == gate.GateAuto {
		if we.orcConfig != nil && we.orcConfig.Gates.AutoApproveOnSuccess {
			result.Approved = true
			result.Reason = "auto-approved on success"
			return result, nil
		}
	}

	// Skip gate if disabled
	if gateType == gate.GateSkip {
		result.Approved = true
		result.Reason = "gate skipped by configuration"
		return result, nil
	}

	// Create gate struct for evaluator
	g := &gate.Gate{
		Type: gateType,
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

// resolveGateType determines the effective gate type for a phase.
// Uses the GateResolver if available (when we have a task with potential overrides),
// otherwise falls back to legacy resolution (template + phase override).
func (we *WorkflowExecutor) resolveGateType(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase, t *orcv1.Task) gate.GateType {
	// If we have a task and project DB, use full resolution
	if t != nil && we.projectDB != nil {
		// Load task overrides from database
		taskOverrides, err := we.projectDB.GetTaskGateOverridesMap(t.Id)
		if err != nil {
			we.logger.Warn("failed to load task gate overrides", "task", t.Id, "error", err)
			taskOverrides = nil
		}

		// Load phase gates from database
		phaseGates, err := we.projectDB.GetPhaseGatesMap()
		if err != nil {
			we.logger.Warn("failed to load phase gates", "error", err)
			phaseGates = nil
		}

		// Build resolver with task context
		resolver := gate.NewResolver(
			we.orcConfig,
			gate.WithTaskOverrides(taskOverrides),
			gate.WithPhaseGates(phaseGates),
		)

		// Resolve gate type
		taskWeight := ""
		if t.Weight != 0 {
			taskWeight = t.Weight.String()
		}
		resolved := resolver.Resolve(tmpl.ID, taskWeight)

		// Log resolution for debugging
		we.logger.Debug("gate type resolved",
			"phase", tmpl.ID,
			"gate_type", resolved.GateType,
			"source", resolved.Source,
			"task", t.Id,
		)

		return resolved.GateType
	}

	// Legacy resolution: template gate type with optional phase override
	gateType := tmpl.GateType
	if phase.GateTypeOverride != "" {
		gateType = phase.GateTypeOverride
	}

	return gate.GateType(gateType)
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
