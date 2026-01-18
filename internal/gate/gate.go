// Package gate provides gate evaluation for orc phase transitions.
package gate

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/plan"
)

// Decision represents a gate evaluation result.
type Decision struct {
	Approved  bool
	Reason    string
	Questions []string // For NEEDS_CLARIFICATION
}

// Evaluator evaluates gates between phases.
type Evaluator struct {
	client claude.Client
}

// New creates a new gate evaluator.
func New(client claude.Client) *Evaluator {
	return &Evaluator{client: client}
}

// JSON schema for AI gate evaluation response.
const gateDecisionSchema = `{
	"type": "object",
	"properties": {
		"decision": {
			"type": "string",
			"enum": ["APPROVED", "REJECTED", "NEEDS_CLARIFICATION"],
			"description": "The evaluation decision"
		},
		"reason": {
			"type": "string",
			"description": "Explanation for the decision"
		},
		"questions": {
			"type": "array",
			"items": {"type": "string"},
			"description": "Questions if clarification is needed (empty otherwise)"
		}
	},
	"required": ["decision", "reason", "questions"]
}`

// gateResponse is the JSON structure for AI gate evaluation.
type gateResponse struct {
	Decision  string   `json:"decision"`
	Reason    string   `json:"reason"`
	Questions []string `json:"questions"`
}

// Evaluate determines if a gate passes.
func (e *Evaluator) Evaluate(ctx context.Context, gate *plan.Gate, phaseOutput string) (*Decision, error) {
	switch gate.Type {
	case plan.GateAuto:
		return e.evaluateAuto(gate, phaseOutput)
	case plan.GateAI:
		return e.evaluateAI(ctx, gate, phaseOutput)
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
			if !strings.Contains(phaseOutput, "<phase_complete>true</phase_complete>") {
				return &Decision{
					Approved: false,
					Reason:   "no completion marker found",
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

// evaluateAI uses Claude to evaluate the gate.
func (e *Evaluator) evaluateAI(ctx context.Context, gate *plan.Gate, phaseOutput string) (*Decision, error) {
	if e.client == nil {
		return nil, fmt.Errorf("AI gate requires LLM client")
	}

	prompt := fmt.Sprintf(`Review the phase output and determine if it meets the quality criteria.

Criteria:
%s

Phase Output:
---
%s
---

Evaluate whether the output meets the criteria. If you need more information, specify what questions need answering.`,
		formatCriteria(gate.Criteria),
		truncateOutput(phaseOutput, 4000),
	)

	resp, err := e.client.Complete(ctx, claude.CompletionRequest{
		Messages: []claude.Message{
			{Role: claude.RoleUser, Content: prompt},
		},
		JSONSchema: gateDecisionSchema,
	})
	if err != nil {
		return nil, fmt.Errorf("AI evaluation failed: %w", err)
	}

	// Parse JSON response
	var result gateResponse
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, fmt.Errorf("parse AI response: %w", err)
	}

	decision := strings.ToUpper(result.Decision)
	switch decision {
	case "APPROVED":
		return &Decision{
			Approved: true,
			Reason:   result.Reason,
		}, nil
	case "REJECTED":
		return &Decision{
			Approved: false,
			Reason:   result.Reason,
		}, nil
	case "NEEDS_CLARIFICATION":
		return &Decision{
			Approved:  false,
			Reason:    result.Reason,
			Questions: result.Questions,
		}, nil
	default:
		return nil, fmt.Errorf("unexpected decision: %s", result.Decision)
	}
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

// formatCriteria formats criteria for the AI prompt.
func formatCriteria(criteria []string) string {
	if len(criteria) == 0 {
		return "- General quality and completeness"
	}

	var lines []string
	for _, c := range criteria {
		lines = append(lines, "- "+c)
	}
	return strings.Join(lines, "\n")
}

// truncateOutput truncates output to a maximum length.
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return output[:maxLen] + "\n... (truncated)"
}
