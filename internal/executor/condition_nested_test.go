package executor

import (
	"testing"

	"github.com/randalmurphal/orc/internal/variable"
)

func TestEvaluateCondition_NestedPhaseOutputField(t *testing.T) {
	t.Parallel()

	ctx := &ConditionContext{
		RCtx: &variable.ResolutionContext{
			PriorOutputs: map[string]string{
				"plan": `{
					"status": "complete",
					"risk_assessment": {
						"requires_browser_qa": true,
						"requires_human_gate": false
					}
				}`,
			},
		},
	}

	condition := `{"field":"phase_output.plan.risk_assessment.requires_browser_qa","op":"eq","value":"true"}`
	ok, err := EvaluateCondition(condition, ctx)
	if err != nil {
		t.Fatalf("EvaluateCondition() error = %v", err)
	}
	if !ok {
		t.Error("EvaluateCondition() should resolve nested phase_output fields")
	}
}

func TestEvaluateCondition_AnyWithVerificationPlanE2E(t *testing.T) {
	t.Parallel()

	ctx := &ConditionContext{
		RCtx: &variable.ResolutionContext{
			PriorOutputs: map[string]string{
				"plan": `{
					"status": "complete",
					"risk_assessment": {
						"requires_browser_qa": false
					},
					"verification_plan": {
						"e2e": "cd web && bun run e2e"
					}
				}`,
			},
		},
	}

	condition := `{"any":[{"field":"phase_output.plan.risk_assessment.requires_browser_qa","op":"eq","value":"true"},{"field":"phase_output.plan.verification_plan.e2e","op":"neq","value":""}]}`
	ok, err := EvaluateCondition(condition, ctx)
	if err != nil {
		t.Fatalf("EvaluateCondition() error = %v", err)
	}
	if !ok {
		t.Error("EvaluateCondition() should trigger when the plan includes an explicit e2e command")
	}
}
