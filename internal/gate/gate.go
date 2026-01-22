// Package gate provides gate evaluation for orc phase transitions.
package gate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/plan"
)

// Decision represents a gate evaluation result.
type Decision struct {
	Approved  bool
	Reason    string
	Questions []string // For NEEDS_CLARIFICATION
	Pending   bool     // True if decision is pending (headless mode)
}

// EvaluateOptions contains context for gate evaluation.
type EvaluateOptions struct {
	TaskID        string
	TaskTitle     string
	Phase         string
	Headless      bool                  // True if running in API/headless mode
	Publisher     events.Publisher      // Event publisher for decision_required events
	DecisionStore *PendingDecisionStore // Store for pending decisions
}

// Evaluator evaluates gates between phases.
// Note: The struct is kept for API compatibility but no longer requires a client.
type Evaluator struct{}

// New creates a new gate evaluator.
// Note: Client parameter kept for API compatibility but is no longer used.
func New(_ any) *Evaluator {
	return &Evaluator{}
}

// Evaluate determines if a gate passes.
func (e *Evaluator) Evaluate(ctx context.Context, gate *plan.Gate, phaseOutput string) (*Decision, error) {
	return e.EvaluateWithOptions(ctx, gate, phaseOutput, nil)
}

// EvaluateWithOptions determines if a gate passes with additional context.
func (e *Evaluator) EvaluateWithOptions(ctx context.Context, gate *plan.Gate, phaseOutput string, opts *EvaluateOptions) (*Decision, error) {
	switch gate.Type {
	case plan.GateAuto:
		return e.evaluateAuto(gate, phaseOutput)
	case plan.GateHuman:
		return e.requestHumanApproval(gate, opts)
	default:
		return nil, fmt.Errorf("unknown gate type: %s", gate.Type)
	}
}

// evaluateAuto evaluates an automatic gate based on criteria.
func (e *Evaluator) evaluateAuto(gate *plan.Gate, phaseOutput string) (*Decision, error) {
	// If no criteria, auto-approve
	if len(gate.Criteria) == 0 {
		return &Decision{
			Approved: true,
			Reason:   "no criteria specified - auto-approved",
		}, nil
	}

	// Check each criterion
	for _, criterion := range gate.Criteria {
		switch criterion {
		case "has_output":
			if phaseOutput == "" {
				return &Decision{
					Approved: false,
					Reason:   "phase produced no output",
				}, nil
			}
		case "no_errors":
			if strings.Contains(strings.ToLower(phaseOutput), "error") {
				return &Decision{
					Approved: false,
					Reason:   "output contains errors",
				}, nil
			}
		case "has_completion_marker":
			// Check for JSON completion status
			var resp struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal([]byte(phaseOutput), &resp); err != nil || resp.Status != "complete" {
				return &Decision{
					Approved: false,
					Reason:   "no completion status found in JSON response",
				}, nil
			}
		default:
			// For custom criteria, check if the string is present in output
			if !strings.Contains(phaseOutput, criterion) {
				return &Decision{
					Approved: false,
					Reason:   fmt.Sprintf("criterion not met: %s", criterion),
				}, nil
			}
		}
	}

	return &Decision{
		Approved: true,
		Reason:   "all criteria met",
	}, nil
}

// requestHumanApproval prompts for human approval.
func (e *Evaluator) requestHumanApproval(gate *plan.Gate, opts *EvaluateOptions) (*Decision, error) {
	// Check if running in headless mode (API/WebSocket context)
	if opts != nil && opts.Headless && opts.Publisher != nil && opts.DecisionStore != nil {
		return e.emitDecisionRequired(gate, opts)
	}

	// Interactive CLI mode - prompt on stdin
	fmt.Println("\n👤 Human approval required")
	if len(gate.Criteria) > 0 {
		fmt.Println("Please verify the following:")
		for _, c := range gate.Criteria {
			fmt.Printf("  - %s\n", c)
		}
	}

	fmt.Print("\nApprove? [y/n/q(questions)]: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("read input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))

	switch input {
	case "y", "yes":
		return &Decision{
			Approved: true,
			Reason:   "human approved",
		}, nil
	case "n", "no":
		fmt.Print("Reason for rejection: ")
		reason, _ := reader.ReadString('\n')
		return &Decision{
			Approved: false,
			Reason:   strings.TrimSpace(reason),
		}, nil
	case "q", "questions":
		fmt.Println("Enter questions (empty line to finish):")
		var questions []string
		for {
			q, _ := reader.ReadString('\n')
			q = strings.TrimSpace(q)
			if q == "" {
				break
			}
			questions = append(questions, q)
		}
		return &Decision{
			Approved:  false,
			Reason:    "needs clarification",
			Questions: questions,
		}, nil
	default:
		return &Decision{
			Approved: false,
			Reason:   "invalid input",
		}, nil
	}
}

// emitDecisionRequired emits a decision_required event for headless mode.
func (e *Evaluator) emitDecisionRequired(gate *plan.Gate, opts *EvaluateOptions) (*Decision, error) {
	// Generate decision ID
	decisionID := fmt.Sprintf("gate_%s_%s", opts.TaskID, opts.Phase)

	// Build question and context from gate criteria
	question := "Approve phase transition?"
	context := ""
	if len(gate.Criteria) > 0 {
		question = "Please verify the following criteria:"
		context = strings.Join(gate.Criteria, "\n")
	}

	// Create pending decision
	now := time.Now()
	decision := &PendingDecision{
		DecisionID:  decisionID,
		TaskID:      opts.TaskID,
		TaskTitle:   opts.TaskTitle,
		Phase:       opts.Phase,
		GateType:    string(gate.Type),
		Question:    question,
		Context:     context,
		RequestedAt: now,
	}

	// Store pending decision
	opts.DecisionStore.Add(decision)

	// Emit event
	eventData := events.DecisionRequiredData{
		DecisionID:  decisionID,
		TaskID:      opts.TaskID,
		TaskTitle:   opts.TaskTitle,
		Phase:       opts.Phase,
		GateType:    string(gate.Type),
		Question:    question,
		Context:     context,
		RequestedAt: now,
	}

	opts.Publisher.Publish(events.Event{
		Type:   events.EventDecisionRequired,
		TaskID: opts.TaskID,
		Data:   eventData,
		Time:   now,
	})

	// Return pending decision (non-blocking)
	return &Decision{
		Pending: true,
		Reason:  "awaiting approval",
	}, nil
}
