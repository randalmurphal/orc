package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// PhaseAgent represents an association between a phase template and an agent.
// Agents with the same sequence number run in parallel.
type PhaseAgent struct {
	ID              int64    `json:"id"`
	PhaseTemplateID string   `json:"phase_template_id"` // References phase_templates.id
	AgentID         string   `json:"agent_id"`          // References agents.id
	Sequence        int      `json:"sequence"`          // Execution order (same = parallel)
	Role            string   `json:"role"`              // 'correctness', 'architecture', etc.
	WeightFilter    []string `json:"weight_filter"`     // ["medium", "large"] or nil for all
	IsBuiltin       bool     `json:"is_builtin"`        // True for built-in associations
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

// AgentWithAssignment combines an agent definition with its phase assignment.
// Used when loading agents for a specific phase execution.
type AgentWithAssignment struct {
	Agent      *Agent
	PhaseAgent *PhaseAgent
}

// SavePhaseAgent saves or updates a phase-agent association.
func (pdb *ProjectDB) SavePhaseAgent(pa *PhaseAgent) error {
	var weightFilterJSON string
	if len(pa.WeightFilter) > 0 {
		b, err := json.Marshal(pa.WeightFilter)
		if err != nil {
			return fmt.Errorf("marshal weight filter: %w", err)
		}
		weightFilterJSON = string(b)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if pa.CreatedAt == "" {
		pa.CreatedAt = now
	}
	pa.UpdatedAt = now

	query := `
		INSERT INTO phase_agents (phase_template_id, agent_id, sequence, role, weight_filter, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(phase_template_id, agent_id) DO UPDATE SET
			sequence = excluded.sequence,
			role = excluded.role,
			weight_filter = excluded.weight_filter,
			is_builtin = excluded.is_builtin,
			updated_at = excluded.updated_at
	`

	result, err := pdb.Exec(query,
		pa.PhaseTemplateID, pa.AgentID, pa.Sequence, pa.Role,
		weightFilterJSON, pa.IsBuiltin, pa.CreatedAt, pa.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save phase agent: %w", err)
	}

	// Set ID if this was an insert
	if pa.ID == 0 {
		id, err := result.LastInsertId()
		if err == nil {
			pa.ID = id
		}
	}

	return nil
}

// GetPhaseAgents returns all agent associations for a phase template.
func (pdb *ProjectDB) GetPhaseAgents(phaseTemplateID string) ([]*PhaseAgent, error) {
	query := `
		SELECT id, phase_template_id, agent_id, sequence, role, weight_filter, is_builtin, created_at, updated_at
		FROM phase_agents
		WHERE phase_template_id = ?
		ORDER BY sequence ASC, agent_id ASC
	`

	rows, err := pdb.Query(query, phaseTemplateID)
	if err != nil {
		return nil, fmt.Errorf("get phase agents for %s: %w", phaseTemplateID, err)
	}
	defer rows.Close()

	return scanPhaseAgents(rows)
}

// GetPhaseAgentsForWeight returns agent associations for a phase template,
// filtered to only agents that apply to the given task weight.
func (pdb *ProjectDB) GetPhaseAgentsForWeight(phaseTemplateID, weight string) ([]*PhaseAgent, error) {
	agents, err := pdb.GetPhaseAgents(phaseTemplateID)
	if err != nil {
		return nil, err
	}

	// Filter by weight
	var filtered []*PhaseAgent
	for _, pa := range agents {
		// No weight filter = applies to all weights
		if len(pa.WeightFilter) == 0 {
			filtered = append(filtered, pa)
			continue
		}

		// Check if weight is in filter
		for _, w := range pa.WeightFilter {
			if w == weight {
				filtered = append(filtered, pa)
				break
			}
		}
	}

	return filtered, nil
}

// GetPhaseAgentsWithDefinitions loads agents with their full definitions for a phase.
// Filtered by task weight.
func (pdb *ProjectDB) GetPhaseAgentsWithDefinitions(phaseTemplateID, weight string) ([]*AgentWithAssignment, error) {
	phaseAgents, err := pdb.GetPhaseAgentsForWeight(phaseTemplateID, weight)
	if err != nil {
		return nil, err
	}

	if len(phaseAgents) == 0 {
		return nil, nil
	}

	var result []*AgentWithAssignment
	for _, pa := range phaseAgents {
		agent, err := pdb.GetAgent(pa.AgentID)
		if err != nil {
			return nil, fmt.Errorf("get agent %s: %w", pa.AgentID, err)
		}
		if agent == nil {
			// Agent doesn't exist - skip
			continue
		}

		result = append(result, &AgentWithAssignment{
			Agent:      agent,
			PhaseAgent: pa,
		})
	}

	return result, nil
}

// DeletePhaseAgent removes an agent from a phase.
// Returns error if association is builtin.
func (pdb *ProjectDB) DeletePhaseAgent(phaseTemplateID, agentID string) error {
	// Check if builtin
	query := `SELECT is_builtin FROM phase_agents WHERE phase_template_id = ? AND agent_id = ?`
	var isBuiltin bool
	err := pdb.QueryRow(query, phaseTemplateID, agentID).Scan(&isBuiltin)
	if err == sql.ErrNoRows {
		return nil // Already doesn't exist
	}
	if err != nil {
		return fmt.Errorf("check phase agent builtin: %w", err)
	}
	if isBuiltin {
		return fmt.Errorf("cannot delete builtin phase agent association")
	}

	_, err = pdb.Exec(
		"DELETE FROM phase_agents WHERE phase_template_id = ? AND agent_id = ?",
		phaseTemplateID, agentID,
	)
	if err != nil {
		return fmt.Errorf("delete phase agent: %w", err)
	}

	return nil
}

// CountPhaseAgents returns the number of agent associations for a phase.
func (pdb *ProjectDB) CountPhaseAgents(phaseTemplateID string) (int, error) {
	var count int
	err := pdb.QueryRow(
		"SELECT COUNT(*) FROM phase_agents WHERE phase_template_id = ?",
		phaseTemplateID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count phase agents: %w", err)
	}
	return count, nil
}

// scanPhaseAgents scans rows into PhaseAgent structs.
func scanPhaseAgents(rows *sql.Rows) ([]*PhaseAgent, error) {
	var agents []*PhaseAgent

	for rows.Next() {
		var pa PhaseAgent
		var role sql.NullString
		var weightFilterJSON sql.NullString

		if err := rows.Scan(
			&pa.ID, &pa.PhaseTemplateID, &pa.AgentID, &pa.Sequence,
			&role, &weightFilterJSON, &pa.IsBuiltin, &pa.CreatedAt, &pa.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan phase agent: %w", err)
		}

		if role.Valid {
			pa.Role = role.String
		}

		if weightFilterJSON.Valid && weightFilterJSON.String != "" {
			if err := json.Unmarshal([]byte(weightFilterJSON.String), &pa.WeightFilter); err != nil {
				return nil, fmt.Errorf("unmarshal weight filter: %w", err)
			}
		}

		agents = append(agents, &pa)
	}

	return agents, nil
}

// GroupBySequence groups agents by their sequence number.
// Agents with the same sequence should be executed in parallel.
func GroupBySequence(agents []*AgentWithAssignment) [][]*AgentWithAssignment {
	if len(agents) == 0 {
		return nil
	}

	// Group by sequence
	groups := make(map[int][]*AgentWithAssignment)
	var sequences []int

	for _, a := range agents {
		seq := a.PhaseAgent.Sequence
		if _, exists := groups[seq]; !exists {
			sequences = append(sequences, seq)
		}
		groups[seq] = append(groups[seq], a)
	}

	// Sort sequences and build result
	// sequences are already in order from the ORDER BY in the query
	var result [][]*AgentWithAssignment
	for _, seq := range sequences {
		result = append(result, groups[seq])
	}

	return result
}
