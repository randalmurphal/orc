package gate

import (
	"context"
	"testing"

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
	if len(result) <= 10 {
		// Should have truncation message
	}
	if result[:10] != long[:10] {
		t.Error("truncateOutput() should keep beginning of string")
	}
}

func TestParseAIResponse(t *testing.T) {
	tests := []struct {
		input    string
		approved bool
		hasQ     bool
	}{
		{"APPROVED: looks good", true, false},
		{"REJECTED: needs work", false, false},
		{"NEEDS_CLARIFICATION: what about X?", false, true},
		{"some other response", true, false}, // defaults to approved
	}

	for _, tt := range tests {
		decision, err := parseAIResponse(tt.input)
		if err != nil {
			t.Errorf("parseAIResponse(%q) failed: %v", tt.input, err)
			continue
		}

		if decision.Approved != tt.approved {
			t.Errorf("parseAIResponse(%q).Approved = %v, want %v", tt.input, decision.Approved, tt.approved)
		}

		hasQuestions := len(decision.Questions) > 0
		if hasQuestions != tt.hasQ {
			t.Errorf("parseAIResponse(%q) hasQuestions = %v, want %v", tt.input, hasQuestions, tt.hasQ)
		}
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
