package executor

import (
	"strings"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/variable"
)

func qualityPolicyEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.QualityPolicy.Mode), "adaptive_strict")
}

func (we *WorkflowExecutor) applyQualityPolicyGateEscalation(
	phaseID string,
	current gate.GateType,
	rctx *variable.ResolutionContext,
) gate.GateType {
	phaseID = canonicalPhaseID(phaseID)

	if !qualityPolicyEnabled(we.orcConfig) {
		return current
	}
	if current == gate.GateSkip || current == gate.GateHuman {
		return current
	}

	switch phaseID {
	case "plan":
		if planRequiresHumanGate(rctx, we.orcConfig.QualityPolicy.HumanGateRiskThreshold) {
			return gate.GateHuman
		}
	case "review_cross":
		if !we.orcConfig.QualityPolicy.PostReviewHumanGate {
			return current
		}
		if planRequiresHumanGate(rctx, we.orcConfig.QualityPolicy.HumanGateRiskThreshold) {
			return gate.GateHuman
		}
		if phaseOutputHasQuestions(rctx, "review_cross") {
			return gate.GateHuman
		}
	}

	return current
}

func planRequiresHumanGate(rctx *variable.ResolutionContext, threshold string) bool {
	resp, ok := loadPlanResponse(rctx)
	if !ok {
		return false
	}
	if resp.RiskAssessment == nil {
		return false
	}
	if resp.RiskAssessment.RequiresHumanGate {
		return true
	}
	return riskLevelMeetsOrExceeds(resp.RiskAssessment.Level, threshold)
}

func loadPlanResponse(rctx *variable.ResolutionContext) (*PlanResponse, bool) {
	if rctx == nil || rctx.PriorOutputs == nil {
		return nil, false
	}
	for _, key := range []string{"plan", "plan_gpt"} {
		output, ok := rctx.PriorOutputs[key]
		if !ok || strings.TrimSpace(output) == "" {
			continue
		}
		resp, err := ParsePlanResponse(output)
		if err != nil {
			continue
		}
		return resp, true
	}
	return nil, false
}

func phaseOutputHasQuestions(rctx *variable.ResolutionContext, phaseID string) bool {
	if rctx == nil || rctx.PriorOutputs == nil {
		return false
	}
	output, ok := rctx.PriorOutputs[phaseID]
	if !ok || strings.TrimSpace(output) == "" {
		return false
	}
	value, found, err := lookupJSONPathValue(output, "questions")
	if err != nil || !found {
		return false
	}
	items, ok := value.([]any)
	return ok && len(items) > 0
}
