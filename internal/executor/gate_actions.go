// gate_actions.go contains action resolution functions for gate output actions.
package executor

import (
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
)

var validGateActions = map[workflow.GateAction]bool{
	workflow.GateActionContinue:  true,
	workflow.GateActionRetry:     true,
	workflow.GateActionFail:      true,
	workflow.GateActionSkipPhase: true,
	workflow.GateActionRunScript: true,
}

// resolveApprovedAction maps GateOutputConfig.OnApproved to a GateAction.
func resolveApprovedAction(cfg *db.GateOutputConfig) workflow.GateAction {
	if cfg == nil || cfg.OnApproved == "" {
		return workflow.GateActionContinue
	}
	action := workflow.GateAction(cfg.OnApproved)
	if validGateActions[action] {
		return action
	}
	return workflow.GateActionContinue
}

// resolveRejectedAction maps GateOutputConfig.OnRejected to a GateAction.
// Empty string signals legacy behavior (phase-specific dispatch in the main loop).
func resolveRejectedAction(cfg *db.GateOutputConfig) workflow.GateAction {
	if cfg == nil || cfg.OnRejected == "" {
		return ""
	}
	action := workflow.GateAction(cfg.OnRejected)
	if validGateActions[action] {
		return action
	}
	return ""
}

// resolveRetryFrom determines the retry target phase.
// outputCfg.RetryFrom takes precedence over tmplRetryFrom (SC-10).
func resolveRetryFrom(outputCfg *db.GateOutputConfig, tmplRetryFrom string) string {
	if outputCfg != nil && outputCfg.RetryFrom != "" {
		return outputCfg.RetryFrom
	}
	return tmplRetryFrom
}
