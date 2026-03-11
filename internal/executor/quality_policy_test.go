package executor

import (
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/variable"
)

func TestParsePlanResponse(t *testing.T) {
	t.Parallel()

	content := `{
		"status": "complete",
		"summary": "plan ready",
		"content": "# Plan",
		"invariants": ["payments remain idempotent"],
		"risk_assessment": {
			"level": "high",
			"tags": ["payments", "persistence"],
			"rationale": "Touches payment state",
			"requires_human_gate": true,
			"requires_browser_qa": false
		},
		"operational_notes": {
			"rollback": "revert migration",
			"migration": "backfill old rows",
			"observability": ["payment failure metric"],
			"external_dependencies": ["stripe"],
			"non_goals": ["ui redesign"]
		},
		"verification_plan": {
			"build": "go test ./...",
			"lint": "golangci-lint run",
			"tests": ["go test ./...", "go test ./internal/executor"],
			"e2e": "make e2e"
		}
	}`

	resp, err := ParsePlanResponse(content)
	if err != nil {
		t.Fatalf("ParsePlanResponse() error = %v", err)
	}
	if resp.RiskAssessment == nil || !resp.RiskAssessment.RequiresHumanGate {
		t.Fatal("ParsePlanResponse() should preserve risk_assessment.requires_human_gate")
	}
	if got := resp.RiskAssessment.Level; got != "high" {
		t.Errorf("risk_assessment.level = %q, want high", got)
	}
	if len(resp.Invariants) != 1 {
		t.Errorf("len(invariants) = %d, want 1", len(resp.Invariants))
	}
	if resp.OperationalNotes == nil || len(resp.OperationalNotes.Observability) != 1 {
		t.Fatalf("observability notes not preserved: %#v", resp.OperationalNotes)
	}
	if resp.VerificationPlan == nil || len(resp.VerificationPlan.Tests) != 2 {
		t.Fatalf("verification plan tests not preserved: %#v", resp.VerificationPlan)
	}
}

func TestPlanRequiresHumanGate(t *testing.T) {
	t.Parallel()

	planOutput := `{"status":"complete","risk_assessment":{"level":"critical","requires_human_gate":false}}`

	if !PlanRequiresHumanGate(planOutput, "high") {
		t.Error("PlanRequiresHumanGate() should trigger when risk level meets threshold")
	}
}

func TestApplyQualityPolicyGateEscalation(t *testing.T) {
	t.Parallel()

	we := &WorkflowExecutor{
		orcConfig: &config.Config{
			QualityPolicy: config.QualityPolicyConfig{
				Mode:                   "adaptive_strict",
				HumanGateRiskThreshold: "high",
				PostReviewHumanGate:    true,
			},
		},
	}

	rctx := &variable.ResolutionContext{
		PriorOutputs: map[string]string{
			"plan": `{"status":"complete","risk_assessment":{"level":"high","requires_human_gate":false}}`,
		},
	}

	got := we.applyQualityPolicyGateEscalation("review_cross", gate.GateAuto, rctx)
	if got != gate.GateHuman {
		t.Errorf("applyQualityPolicyGateEscalation(review_cross) = %s, want human", got)
	}
}

func TestGetSchemaForPhaseWithRound_PlanAndReviewCross(t *testing.T) {
	t.Parallel()

	planSchema := GetSchemaForPhaseWithRound("plan", 0, true)
	if !containsAllSnippets(planSchema, `"risk_assessment"`, `"verification_plan"`, `"invariants"`) {
		t.Errorf("plan schema missing policy fields: %s", planSchema)
	}

	reviewCrossSchema := GetSchemaForPhaseWithRound("review_cross", 1, false)
	if !containsAllSnippets(reviewCrossSchema, `"needs_changes"`, `"issues"`) {
		t.Errorf("review_cross schema should reuse review findings schema: %s", reviewCrossSchema)
	}
}

func TestGetSchemaForPhaseWithRound_PlanAlias(t *testing.T) {
	t.Parallel()

	planSchema := GetSchemaForPhaseWithRound("plan_gpt", 0, true)
	if !containsAllSnippets(planSchema, `"risk_assessment"`, `"verification_plan"`, `"invariants"`) {
		t.Fatalf("plan_gpt schema should reuse plan schema: %s", planSchema)
	}
}

func TestApplyQualityPolicyGateEscalation_PlanAlias(t *testing.T) {
	t.Parallel()

	we := &WorkflowExecutor{
		orcConfig: &config.Config{
			QualityPolicy: config.QualityPolicyConfig{
				Mode:                   "adaptive_strict",
				HumanGateRiskThreshold: "high",
				PostReviewHumanGate:    true,
			},
		},
	}

	rctx := &variable.ResolutionContext{
		PriorOutputs: map[string]string{
			"plan_gpt": `{"status":"complete","risk_assessment":{"level":"high","requires_human_gate":true}}`,
		},
	}

	got := we.applyQualityPolicyGateEscalation("plan_gpt", gate.GateAuto, rctx)
	if got != gate.GateHuman {
		t.Fatalf("applyQualityPolicyGateEscalation(plan_gpt) = %s, want human", got)
	}
}

func containsAllSnippets(content string, snippets ...string) bool {
	for _, snippet := range snippets {
		if !strings.Contains(content, snippet) {
			return false
		}
	}
	return true
}
