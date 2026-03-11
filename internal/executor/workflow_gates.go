// workflow_gates.go contains gate evaluation and related utilities for workflow execution.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// GateEvaluationResult contains the result of gate evaluation.
type GateEvaluationResult struct {
	Approved   bool
	Pending    bool
	Reason     string
	RetryPhase string // If not approved and has retry target

	// Gate output pipeline fields (propagated from gate.Decision)
	OutputData map[string]any // Structured data from gate agent for variable pipeline
	OutputVar  string         // Variable name to store output as

	// OutputConfig from PhaseTemplate; nil when not configured or gates skipped.
	OutputConfig *db.GateOutputConfig
}

// evaluatePhaseGate evaluates the gate for a completed phase.
func (we *WorkflowExecutor) evaluatePhaseGate(ctx context.Context, tmpl *db.PhaseTemplate, phase *db.WorkflowPhase, output string, t *orcv1.Task, rctxs ...*variable.ResolutionContext) (*GateEvaluationResult, error) {
	result := &GateEvaluationResult{}
	var rctx *variable.ResolutionContext
	if len(rctxs) > 0 {
		rctx = rctxs[0]
	}

	// Skip all gate evaluations when --skip-gates flag is set
	if we.skipGates {
		result.Approved = true
		result.Reason = "gates skipped by --skip-gates flag"
		we.logger.Info("gate skipped by --skip-gates flag", "phase", tmpl.ID)
		return result, nil
	}

	// Use gate resolver if available, fall back to legacy resolution
	gateType := we.resolveGateType(tmpl, phase, t, rctx)

	// No gate type configured: auto-approve without evaluation
	if gateType == "" {
		result.Approved = true
		result.Reason = "no gate configured"
		return result, nil
	}

	// Auto gate with auto-approve config: auto-approve
	if gateType == gate.GateAuto {
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

	g := &gate.Gate{
		Type: gateType,
	}

	// Parse gate input/output configs from PhaseTemplate.
	var inputCfg *db.GateInputConfig
	if tmpl.GateInputConfig != "" {
		parsed, parseErr := db.ParseGateInputConfig(tmpl.GateInputConfig)
		if parseErr != nil {
			return nil, fmt.Errorf("parse gate input config for %s: %w", tmpl.ID, parseErr)
		}
		inputCfg = parsed
	}

	var outputCfg *db.GateOutputConfig
	if tmpl.GateOutputConfig != "" {
		parsed, parseErr := db.ParseGateOutputConfig(tmpl.GateOutputConfig)
		if parseErr != nil {
			return nil, fmt.Errorf("parse gate output config for %s: %w", tmpl.ID, parseErr)
		}
		outputCfg = parsed
	}

	opts := &gate.EvaluateOptions{
		ProjectID:     we.projectIDForEvents(),
		Phase:         tmpl.ID,
		AgentID:       tmpl.GateAgentID,
		InputConfig:   inputCfg,
		OutputConfig:  outputCfg,
		Headless:      true,
		Publisher:     we.publisher.Publisher(),
		DecisionStore: we.pendingDecisions,
	}
	if t != nil {
		opts.TaskID = t.Id
		opts.TaskTitle = t.Title
		opts.TaskDesc = task.GetDescriptionProto(t)
		opts.TaskCategory = t.Category.String()
		opts.TaskWeight = task.GetWorkflowIDProto(t) // Use workflow ID for gate context
	}

	decision, err := we.gateEvaluator.EvaluateWithOptions(ctx, g, output, opts)
	if err != nil {
		return nil, fmt.Errorf("gate evaluation: %w", err)
	}

	result.Approved = decision.Approved
	result.Pending = decision.Pending
	result.Reason = decision.Reason
	result.OutputData = decision.OutputData
	result.OutputVar = decision.OutputVar
	result.OutputConfig = outputCfg

	if outputCfg != nil && outputCfg.Script != "" {
		scriptResult, scriptErr := we.runGateScript(ctx, outputCfg.Script, decision, result)
		if scriptErr != nil {
			return nil, fmt.Errorf("run gate script for %s: %w", tmpl.ID, scriptErr)
		}
		if scriptResult != nil && scriptResult.Override {
			// Script wants to override the gate decision
			result.Approved = !result.Approved
			result.Reason = fmt.Sprintf("script override: %s (original: %s)", scriptResult.Reason, result.Reason)
		}
	}

	// SC-10: outputCfg.RetryFrom takes precedence over tmpl.RetryFromPhase
	if !result.Approved && !result.Pending {
		result.RetryPhase = resolveRetryFrom(outputCfg, tmpl.RetryFromPhase)
		if result.RetryPhase == "" && we.orcConfig != nil {
			// Fall back to config-based retry map
			result.RetryPhase = we.orcConfig.ShouldRetryFrom(tmpl.ID)
		}
	}

	return result, nil
}

// runGateScript executes a gate output script and returns the result.
func (we *WorkflowExecutor) runGateScript(ctx context.Context, scriptPath string, _ *gate.Decision, gateResult *GateEvaluationResult) (*gate.ScriptResult, error) {
	// Validate and resolve script path
	resolvedPath, err := gate.ValidateScriptPath(scriptPath, we.workingDir)
	if err != nil {
		return nil, fmt.Errorf("validate gate script path %q: %w", scriptPath, err)
	}

	// Build gate output JSON to pipe to script
	outputJSON, err := json.Marshal(map[string]any{
		"approved":    gateResult.Approved,
		"reason":      gateResult.Reason,
		"output_data": gateResult.OutputData,
		"output_var":  gateResult.OutputVar,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal gate output: %w", err)
	}

	handler := gate.NewScriptHandler(we.logger)
	scriptResult, err := handler.Run(ctx, resolvedPath, string(outputJSON), we.workingDir)
	if err != nil {
		return nil, fmt.Errorf("execute gate script %q: %w", resolvedPath, err)
	}

	return scriptResult, nil
}

// resolveGateType determines the effective gate type for a phase.
// Uses the GateResolver if available (when we have a task with potential overrides),
// otherwise falls back to legacy resolution (template + phase override).
func (we *WorkflowExecutor) resolveGateType(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase, t *orcv1.Task, rctx *variable.ResolutionContext) gate.GateType {
	if t != nil && we.projectDB != nil {
		taskOverrides, err := we.projectDB.GetTaskGateOverridesMap(t.Id)
		if err != nil {
			we.logger.Warn("failed to load task gate overrides", "task", t.Id, "error", err)
			taskOverrides = nil
		}

		phaseGates, err := we.projectDB.GetPhaseGatesMap()
		if err != nil {
			we.logger.Warn("failed to load phase gates", "error", err)
			phaseGates = nil
		}

		resolver := gate.NewResolver(
			we.orcConfig,
			gate.WithTaskOverrides(taskOverrides),
			gate.WithPhaseGates(phaseGates),
		)

		// Use workflow ID for gate resolution (replaces weight-based resolution)
		taskWorkflowID := task.GetWorkflowIDProto(t)
		resolved := resolver.Resolve(tmpl.ID, taskWorkflowID)

		// When the resolver used its default (no explicit override found),
		// fall back to the template's gate type for backward compatibility.
		// This ensures templates with GateType="ai" are respected, and
		// templates with no gate type don't get an unwanted auto gate.
		if resolved.Source == "default" {
			return we.applyQualityPolicyGateEscalation(tmpl.ID, gate.GateType(tmpl.GateType), rctx)
		}

		we.logger.Debug("gate type resolved",
			"phase", tmpl.ID,
			"gate_type", resolved.GateType,
			"source", resolved.Source,
			"task", t.Id,
		)

		return we.applyQualityPolicyGateEscalation(tmpl.ID, resolved.GateType, rctx)
	}

	// Legacy resolution: template gate type with optional phase override
	gateType := tmpl.GateType
	if phase.GateTypeOverride != "" {
		gateType = phase.GateTypeOverride
	}

	return we.applyQualityPolicyGateEscalation(tmpl.ID, gate.GateType(gateType), rctx)
}

// applyGateOutputToVars stores gate output data as a workflow variable.
// Called for both approved and rejected gates so retry phases can access gate analysis.
// Gate output is persisted to rctx.PhaseOutputVars so it survives ResolveAll() during retry.
func applyGateOutputToVars(vars map[string]string, rctx *variable.ResolutionContext, gateResult *GateEvaluationResult) {
	if gateResult == nil {
		return
	}

	varName := strings.TrimSpace(gateResult.OutputVar)
	if varName == "" || gateResult.OutputData == nil {
		return
	}

	data, err := json.Marshal(gateResult.OutputData)
	if err != nil {
		return
	}

	vars[varName] = string(data)

	// Persist to rctx so variable survives ResolveAll() during retry
	if rctx != nil {
		if rctx.PhaseOutputVars == nil {
			rctx.PhaseOutputVars = make(map[string]string)
		}
		rctx.PhaseOutputVars[varName] = string(data)
	}
}

// publishTaskUpdated publishes a task_updated event for real-time UI updates.
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
		Weight:   task.GetWorkflowIDProto(t), // Use workflow ID
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
