// Package gate provides gate evaluation for orc phase transitions.
package gate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/randalmurphal/orc/internal/plan"
)

// Decision represents a gate evaluation result.
type Decision struct {
	Approved  bool
	Reason    string
	Questions []string // For NEEDS_CLARIFICATION
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
	switch gate.Type {
	case plan.GateAuto:
		return e.evaluateAuto(gate, phaseOutput)
	case plan.GateHuman:
		return e.requestHumanApproval(gate)
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
func (e *Evaluator) requestHumanApproval(gate *plan.Gate) (*Decision, error) {
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
