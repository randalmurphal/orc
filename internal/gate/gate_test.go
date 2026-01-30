package gate

import (
	"context"
	"testing"
)

func TestNew(t *testing.T) {
	e := New()

	if e == nil {
		t.Fatal("New() returned nil")
	}
}

func TestEvaluateAutoNoCriteria(t *testing.T) {
	e := New()

	gate := &Gate{
		Type:     GateAuto,
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
	e := New()

	gate := &Gate{
		Type:     GateAuto,
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
	e := New()

	gate := &Gate{
		Type:     GateAuto,
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
	e := New()

	gate := &Gate{
		Type:     GateAuto,
		Criteria: []string{"has_completion_marker"},
	}

	// Should approve with JSON completion status
	decision, _ := e.Evaluate(context.Background(), gate, `{"status": "complete", "summary": "done"}`)
	if !decision.Approved {
		t.Error("gate should approve with JSON completion status")
	}

	// Should reject without completion status
	decision, _ = e.Evaluate(context.Background(), gate, "done")
	if decision.Approved {
		t.Error("gate should reject without JSON completion status")
	}
}

func TestEvaluateAutoCustomCriteria(t *testing.T) {
	e := New()

	gate := &Gate{
		Type:     GateAuto,
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

func TestEvaluateUnknownType(t *testing.T) {
	e := New()

	gate := &Gate{
		Type: "unknown",
	}

	_, err := e.Evaluate(context.Background(), gate, "output")
	if err == nil {
		t.Error("unknown gate type should fail")
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

func TestEvaluateAutoMultipleCriteria(t *testing.T) {
	e := New()

	gate := &Gate{
		Type:     GateAuto,
		Criteria: []string{"has_output", "no_errors", "has_completion_marker"},
	}

	// Should pass when all criteria met (JSON format)
	output := `{"status": "complete", "summary": "good output"}`
	decision, _ := e.Evaluate(context.Background(), gate, output)
	if !decision.Approved {
		t.Error("gate should approve when all criteria met")
	}

	// Should fail when has_output fails
	decision, _ = e.Evaluate(context.Background(), gate, "")
	if decision.Approved {
		t.Error("gate should reject when output empty")
	}

	// Should fail when no_errors fails (note: JSON with "error" in content would fail no_errors)
	decision, _ = e.Evaluate(context.Background(), gate, `{"status": "complete", "summary": "an error occurred"}`)
	if decision.Approved {
		t.Error("gate should reject when errors present in content")
	}

	// Should fail when completion status missing
	decision, _ = e.Evaluate(context.Background(), gate, "good output without json completion")
	if decision.Approved {
		t.Error("gate should reject when JSON completion status missing")
	}
}

func TestEvaluateHumanWithoutStdin(t *testing.T) {
	// We can't easily test human approval in unit tests without mocking stdin
	// but we can test that it returns an error when stdin fails
	// Skip this test as it requires interactive input
	t.Skip("Human gate requires interactive input")
}
