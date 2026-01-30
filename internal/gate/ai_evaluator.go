package gate

import (
	"context"
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/llmutil"
)

// GateAgentResponse is the standard schema for AI gate agent output.
type GateAgentResponse struct {
	Status    string         `json:"status"`               // "approved", "rejected", "blocked"
	Reason    string         `json:"reason"`               // Human-readable explanation
	RetryFrom string        `json:"retry_from,omitempty"` // Phase to retry from (optional)
	Context   string         `json:"context,omitempty"`    // Additional context (optional)
	Data      map[string]any `json:"data,omitempty"`       // Structured output for variable pipeline
}

// gateAgentResponseSchema is the JSON schema for GateAgentResponse.
const gateAgentResponseSchema = `{
  "type": "object",
  "properties": {
    "status": {
      "type": "string",
      "enum": ["approved", "rejected", "blocked"],
      "description": "Gate evaluation result"
    },
    "reason": {
      "type": "string",
      "description": "Human-readable explanation of the decision"
    },
    "retry_from": {
      "type": "string",
      "description": "Phase to retry from if rejected (optional)"
    },
    "context": {
      "type": "string",
      "description": "Additional context for the decision (optional)"
    },
    "data": {
      "type": "object",
      "description": "Structured output data for downstream variable pipeline (optional)"
    }
  },
  "required": ["status", "reason"]
}`

// evaluateAI handles AI gate evaluation using an agent and LLM call.
func (e *Evaluator) evaluateAI(ctx context.Context, phaseOutput string, opts *EvaluateOptions) (*Decision, error) {
	if opts == nil || opts.AgentID == "" {
		return nil, fmt.Errorf("ai gate: no agent configured")
	}
	if e.agentLookup == nil {
		return nil, fmt.Errorf("ai gate: agent lookup required")
	}
	if e.clientCreator == nil {
		return nil, fmt.Errorf("ai gate: client creator required")
	}

	// Look up agent
	agent, err := e.agentLookup.GetAgent(opts.AgentID)
	if err != nil {
		return nil, fmt.Errorf("ai gate: agent lookup: %w", err)
	}
	if agent == nil {
		return nil, fmt.Errorf("ai gate: agent %q not found", opts.AgentID)
	}

	// Build prompt
	prompt := e.buildAIGatePrompt(agent, phaseOutput, opts)

	// Create LLM client with agent's model
	client := e.clientCreator.NewSchemaClient(agent.Model)

	// Execute schema-constrained LLM call
	result, err := llmutil.ExecuteWithSchema[GateAgentResponse](ctx, client, prompt, gateAgentResponseSchema)
	if err != nil {
		return nil, fmt.Errorf("ai gate evaluate: %w", err)
	}

	resp := result.Data

	// Record cost (best-effort)
	if e.costRecorder != nil {
		e.costRecorder.RecordCost(db.CostEntry{
			TaskID:       opts.TaskID,
			Phase:        "gate:" + opts.Phase,
			Model:        result.Response.Model,
			InputTokens:  result.Response.Usage.InputTokens,
			OutputTokens: result.Response.Usage.OutputTokens,
			TotalTokens:  result.Response.Usage.TotalTokens,
		})
	}

	// Map status to decision
	return e.mapResponseToDecision(resp, opts)
}

// mapResponseToDecision converts a GateAgentResponse to a Decision.
func (e *Evaluator) mapResponseToDecision(resp GateAgentResponse, opts *EvaluateOptions) (*Decision, error) {
	decision := &Decision{
		Reason:     resp.Reason,
		OutputData: resp.Data,
	}

	// Set variable name from output config
	if opts.OutputConfig != nil && opts.OutputConfig.VariableName != "" {
		decision.OutputVar = opts.OutputConfig.VariableName
	}

	switch resp.Status {
	case "approved":
		decision.Approved = true
	case "rejected", "blocked":
		decision.Approved = false
		// Resolve retry phase: config overrides LLM response
		if opts.OutputConfig != nil && opts.OutputConfig.RetryFrom != "" {
			decision.RetryPhase = opts.OutputConfig.RetryFrom
		} else {
			decision.RetryPhase = resp.RetryFrom
		}
	default:
		return nil, fmt.Errorf("ai gate: unknown status %q", resp.Status)
	}

	return decision, nil
}

// buildAIGatePrompt constructs the prompt for AI gate evaluation.
func (e *Evaluator) buildAIGatePrompt(agent *db.Agent, phaseOutput string, opts *EvaluateOptions) string {
	var b strings.Builder

	// Agent context (system-level instructions go in user message for ExecuteWithSchema)
	agentContext := agent.Prompt
	if agentContext == "" {
		agentContext = agent.Description
	}
	if agentContext != "" {
		b.WriteString("## Agent Instructions\n\n")
		b.WriteString(agentContext)
		b.WriteString("\n\n")
	}

	// Current phase output
	b.WriteString("## Current Phase Output\n\n")
	b.WriteString(phaseOutput)
	b.WriteString("\n\n")

	if opts.InputConfig == nil {
		return b.String()
	}

	// Include requested phase outputs
	if len(opts.InputConfig.IncludePhaseOutput) > 0 {
		b.WriteString("## Previous Phase Outputs\n\n")
		for _, phaseID := range opts.InputConfig.IncludePhaseOutput {
			content, ok := opts.PhaseOutputs[phaseID]
			if ok && content != "" {
				fmt.Fprintf(&b, "### %s\n\n%s\n\n", phaseID, content)
			} else {
				fmt.Fprintf(&b, "### %s\n\n(unavailable)\n\n", phaseID)
			}
		}
	}

	// Include task context
	if opts.InputConfig.IncludeTask {
		b.WriteString("## Task Context\n\n")
		if opts.TaskTitle != "" {
			fmt.Fprintf(&b, "**Title:** %s\n", opts.TaskTitle)
		}
		if opts.TaskDesc != "" {
			fmt.Fprintf(&b, "**Description:** %s\n", opts.TaskDesc)
		}
		if opts.TaskCategory != "" {
			fmt.Fprintf(&b, "**Category:** %s\n", opts.TaskCategory)
		}
		if opts.TaskWeight != "" {
			fmt.Fprintf(&b, "**Weight:** %s\n", opts.TaskWeight)
		}
		b.WriteString("\n")
	}

	// Include extra variables
	if len(opts.InputConfig.ExtraVars) > 0 {
		b.WriteString("## Extra Variables\n\n")
		for _, v := range opts.InputConfig.ExtraVars {
			b.WriteString(v)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	return b.String()
}
