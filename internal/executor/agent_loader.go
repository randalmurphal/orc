// agent_loader.go contains logic for loading agents from the database
// and converting them to Claude CLI inline agent format.
package executor

import (
	"fmt"

	"github.com/randalmurphal/orc/internal/db"
)

// AgentWithAssignment combines agent definition with phase assignment info.
// Used when loading agents for phase execution.
type AgentWithAssignment struct {
	Agent      *db.Agent
	PhaseAgent *db.PhaseAgent
}

// ToInlineAgentDef converts a database Agent to an InlineAgentDef for Claude CLI.
func ToInlineAgentDef(a *db.Agent) InlineAgentDef {
	return InlineAgentDef{
		Description: a.Description,
		Prompt:      a.Prompt,
		Tools:       a.Tools,
		Model:       a.Model,
	}
}

// LoadPhaseAgents loads agents for a phase template, filtered by task weight.
// Returns a map of agent ID to InlineAgentDef suitable for Claude CLI --agents flag.
func LoadPhaseAgents(gdb *db.GlobalDB, phaseTemplateID string, weight string) (map[string]InlineAgentDef, error) {
	// Load agents with their definitions from global DB
	agentsWithDefs, err := gdb.GetPhaseAgentsWithDefinitions(phaseTemplateID, weight)
	if err != nil {
		return nil, fmt.Errorf("get phase agents for %s: %w", phaseTemplateID, err)
	}

	if len(agentsWithDefs) == 0 {
		return nil, nil
	}

	// Convert to inline agent format
	result := make(map[string]InlineAgentDef, len(agentsWithDefs))
	for _, awa := range agentsWithDefs {
		result[awa.Agent.ID] = ToInlineAgentDef(awa.Agent)
	}

	return result, nil
}

// GroupAgentsBySequence groups agents by their execution sequence.
// Agents with the same sequence run in parallel.
func GroupAgentsBySequence(agents []*db.AgentWithAssignment) [][]*db.AgentWithAssignment {
	return db.GroupBySequence(agents)
}
