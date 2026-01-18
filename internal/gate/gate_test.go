package gate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/plan"
)

func TestNew(t *testing.T) {
	e := New(nil)

	if e == nil {
		t.Fatal("New() returned nil")
	}

	if e.client != nil {
		t.Error("client should be nil when not provided")
	}
}

func TestEvaluateAutoNoCriteria(t *testing.T) {
	e := New(nil)

	gate := &plan.Gate{
		Type:     plan.GateAuto,
		Criteria: nil,
	}

	decision, err := e.Evaluate(context.Background(), gate, "some output")
	if err != nil {
		t.Fatalf("Evaluate() failed: %v", err)
	}

	if !decision.Approved {
		t.Error("gate with no criteria should auto-approve")
	}
}

func TestEvaluateAutoHasOutput(t *testing.T) {
	e := New(nil)

	gate := &plan.Gate{
		Type:     plan.GateAuto,
		Criteria: []string{"has_output"},
	}

	// Should approve with output
	decision, _ := e.Evaluate(context.Background(), gate, "some output")
	if !decision.Approved {
		t.Error("gate should approve when output present")
	}

	// Should reject without output
	decision, _ = e.Evaluate(context.Background(), gate, "")
	if decision.Approved {
		t.Error("gate should reject when output is empty")
	}
}

func TestEvaluateAutoNoErrors(t *testing.T) {
	e := New(nil)

	gate := &plan.Gate{
		Type:     plan.GateAuto,
		Criteria: []string{"no_errors"},
	}

	// Should approve without errors
	decision, _ := e.Evaluate(context.Background(), gate, "all good")
	if !decision.Approved {
		t.Error("gate should approve when no errors")
	}

	// Should reject with errors
	decision, _ = e.Evaluate(context.Background(), gate, "an ERROR occurred")
	if decision.Approved {
		t.Error("gate should reject when errors present")
	}
}

func TestEvaluateAutoHasCompletionMarker(t *testing.T) {
	e := New(nil)

	gate := &plan.Gate{
		Type:     plan.GateAuto,
		Criteria: []string{"has_completion_marker"},
	}

	// Should approve with marker
	decision, _ := e.Evaluate(context.Background(), gate, "done <phase_complete>true</phase_complete>")
	if !decision.Approved {
		t.Error("gate should approve with completion marker")
	}

	// Should reject without marker
	decision, _ = e.Evaluate(context.Background(), gate, "done")
	if decision.Approved {
		t.Error("gate should reject without completion marker")
	}
}

func TestEvaluateAutoCustomCriteria(t *testing.T) {
	e := New(nil)

	gate := &plan.Gate{
		Type:     plan.GateAuto,
		Criteria: []string{"func TestSomething"},
	}

	// Should approve with custom string
	decision, _ := e.Evaluate(context.Background(), gate, "func TestSomething(t *testing.T) {}")
	if !decision.Approved {
		t.Error("gate should approve when custom criterion found")
	}

	// Should reject without custom string
	decision, _ = e.Evaluate(context.Background(), gate, "func main() {}")
	if decision.Approved {
		t.Error("gate should reject when custom criterion not found")
	}
}

func TestEvaluateAIWithoutClient(t *testing.T) {
	e := New(nil)

	gate := &plan.Gate{
		Type: plan.GateAI,
	}

	_, err := e.Evaluate(context.Background(), gate, "output")
	if err == nil {
		t.Error("AI gate without client should fail")
	}
}

func TestEvaluateUnknownType(t *testing.T) {
	e := New(nil)

	gate := &plan.Gate{
		Type: "unknown",
	}

	_, err := e.Evaluate(context.Background(), gate, "output")
	if err == nil {
		t.Error("unknown gate type should fail")
	}
}

func TestFormatCriteria(t *testing.T) {
	// Empty criteria
	result := formatCriteria(nil)
	if result != "- General quality and completeness" {
		t.Errorf("formatCriteria(nil) = %s, want default", result)
	}

	// With criteria
	result = formatCriteria([]string{"foo", "bar"})
	expected := "- foo\n- bar"
	if result != expected {
		t.Errorf("formatCriteria() = %s, want %s", result, expected)
	}
}

func TestTruncateOutput(t *testing.T) {
	// Short output
	short := "short"
	result := truncateOutput(short, 100)
	if result != short {
		t.Errorf("truncateOutput() shouldn't truncate short strings")
	}

	// Long output
	long := "a very long string that needs truncation"
	result = truncateOutput(long, 10)
	// Result should be longer than 10 due to truncation message
	if len(result) <= 10 {
		t.Error("truncateOutput() should add truncation message for long strings")
	}
	if result[:10] != long[:10] {
		t.Error("truncateOutput() should keep beginning of string")
	}
}

func TestGateResponseJSON(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		approved bool
		reason   string
		hasQ     bool
	}{
		{
			name:     "approved",
			json:     `{"decision":"APPROVED","reason":"looks good","questions":[]}`,
			approved: true,
			reason:   "looks good",
			hasQ:     false,
		},
		{
			name:     "rejected",
			json:     `{"decision":"REJECTED","reason":"needs work","questions":[]}`,
			approved: false,
			reason:   "needs work",
			hasQ:     false,
		},
		{
			name:     "needs clarification",
			json:     `{"decision":"NEEDS_CLARIFICATION","reason":"unclear","questions":["what about X?"]}`,
			approved: false,
			reason:   "unclear",
			hasQ:     true,
		},
		{
			name:     "lowercase decision normalized",
			json:     `{"decision":"approved","reason":"all good","questions":[]}`,
			approved: true,
			reason:   "all good",
			hasQ:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var resp gateResponse
			if err := json.Unmarshal([]byte(tt.json), &resp); err != nil {
				t.Fatalf("failed to unmarshal: %v", err)
			}

			// Validate decision normalization
			decision := strings.ToUpper(resp.Decision)
			isApproved := decision == "APPROVED"
			if isApproved != tt.approved {
				t.Errorf("approved = %v, want %v", isApproved, tt.approved)
			}

			if resp.Reason != tt.reason {
				t.Errorf("reason = %q, want %q", resp.Reason, tt.reason)
			}

			hasQuestions := len(resp.Questions) > 0
			if hasQuestions != tt.hasQ {
				t.Errorf("hasQuestions = %v, want %v", hasQuestions, tt.hasQ)
			}
		})
	}
}

func TestDecision(t *testing.T) {
	d := &Decision{
		Approved:  true,
		Reason:    "test reason",
		Questions: []string{"q1", "q2"},
	}

	if !d.Approved {
		t.Error("Approved should be true")
	}

	if d.Reason != "test reason" {
		t.Errorf("Reason = %s, want test reason", d.Reason)
	}

	if len(d.Questions) != 2 {
		t.Errorf("len(Questions) = %d, want 2", len(d.Questions))
	}
}

func TestEvaluateAI_Approved(t *testing.T) {
	mockClient := claude.NewMockClient(`{"decision":"APPROVED","reason":"looks good and meets all criteria","questions":[]}`)
	e := New(mockClient)

	gate := &plan.Gate{
		Type:     plan.GateAI,
		Criteria: []string{"code quality", "test coverage"},
	}

	decision, err := e.Evaluate(context.Background(), gate, "some phase output")
	if err != nil {
		t.Fatalf("Evaluate() failed: %v", err)
	}

	if !decision.Approved {
		t.Error("decision should be approved")
	}

	if decision.Reason != "looks good and meets all criteria" {
		t.Errorf("Reason = %q, want 'looks good and meets all criteria'", decision.Reason)
	}
}

func TestEvaluateAI_Rejected(t *testing.T) {
	mockClient := claude.NewMockClient(`{"decision":"REJECTED","reason":"missing test cases","questions":[]}`)
	e := New(mockClient)

	gate := &plan.Gate{
		Type:     plan.GateAI,
		Criteria: []string{"test coverage"},
	}

	decision, err := e.Evaluate(context.Background(), gate, "incomplete output")
	if err != nil {
		t.Fatalf("Evaluate() failed: %v", err)
	}

	if decision.Approved {
		t.Error("decision should be rejected")
	}

	if decision.Reason != "missing test cases" {
		t.Errorf("Reason = %q, want 'missing test cases'", decision.Reason)
	}
}

func TestEvaluateAI_NeedsClarification(t *testing.T) {
	mockClient := claude.NewMockClient(`{"decision":"NEEDS_CLARIFICATION","reason":"unclear requirements","questions":["what about edge cases?","are there integration tests?"]}`)
	e := New(mockClient)

	gate := &plan.Gate{
		Type:     plan.GateAI,
		Criteria: []string{"completeness"},
	}

	decision, err := e.Evaluate(context.Background(), gate, "some output")
	if err != nil {
		t.Fatalf("Evaluate() failed: %v", err)
	}

	if decision.Approved {
		t.Error("decision should not be approved")
	}

	if len(decision.Questions) == 0 {
		t.Error("should have questions")
	}

	if len(decision.Questions) != 2 {
		t.Errorf("should have 2 questions, got %d", len(decision.Questions))
	}
}

func TestEvaluateAI_ClientError(t *testing.T) {
	mockClient := claude.NewMockClient("").WithError(fmt.Errorf("API error"))
	e := New(mockClient)

	gate := &plan.Gate{
		Type: plan.GateAI,
	}

	_, err := e.Evaluate(context.Background(), gate, "output")
	if err == nil {
		t.Error("Evaluate() should fail with client error")
	}
}

func TestEvaluateAutoMultipleCriteria(t *testing.T) {
	e := New(nil)

	gate := &plan.Gate{
		Type:     plan.GateAuto,
		Criteria: []string{"has_output", "no_errors", "has_completion_marker"},
	}

	// Should pass when all criteria met
	output := "good output <phase_complete>true</phase_complete>"
	decision, _ := e.Evaluate(context.Background(), gate, output)
	if !decision.Approved {
		t.Error("gate should approve when all criteria met")
	}

	// Should fail when has_output fails
	decision, _ = e.Evaluate(context.Background(), gate, "")
	if decision.Approved {
		t.Error("gate should reject when output empty")
	}

	// Should fail when no_errors fails
	decision, _ = e.Evaluate(context.Background(), gate, "error occurred <phase_complete>true</phase_complete>")
	if decision.Approved {
		t.Error("gate should reject when errors present")
	}

	// Should fail when completion marker missing
	decision, _ = e.Evaluate(context.Background(), gate, "good output")
	if decision.Approved {
		t.Error("gate should reject when completion marker missing")
	}
}

func TestEvaluateHumanWithoutStdin(t *testing.T) {
	// We can't easily test human approval in unit tests without mocking stdin
	// but we can test that it returns an error when stdin fails
	// Skip this test as it requires interactive input
	t.Skip("Human gate requires interactive input")
}
