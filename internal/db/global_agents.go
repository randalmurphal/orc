package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SaveAgent saves or updates an agent definition in global DB.
func (g *GlobalDB) SaveAgent(a *Agent) error {
	toolsJSON, err := json.Marshal(a.Tools)
	if err != nil {
		return fmt.Errorf("marshal agent tools: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if a.CreatedAt == "" {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	_, err = g.Exec(`
		INSERT INTO agents (id, name, description, prompt, tools, model, provider, system_prompt, runtime_config, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			prompt = excluded.prompt,
			tools = excluded.tools,
			model = excluded.model,
			provider = excluded.provider,
			system_prompt = excluded.system_prompt,
			runtime_config = excluded.runtime_config,
			is_builtin = excluded.is_builtin,
			updated_at = excluded.updated_at
	`, a.ID, a.Name, a.Description, a.Prompt, string(toolsJSON),
		a.Model, a.Provider, a.SystemPrompt, a.RuntimeConfig, a.IsBuiltin, a.CreatedAt, a.UpdatedAt)
	if err != nil {
		return fmt.Errorf("save agent %s: %w", a.ID, err)
	}

	return nil
}

// GetAgent retrieves an agent by ID from global DB.
func (g *GlobalDB) GetAgent(id string) (*Agent, error) {
	var a Agent
	var toolsJSON string
	var model, provider, systemPrompt, runtimeConfig sql.NullString

	err := g.QueryRow(`
		SELECT id, name, description, prompt, tools, model, provider, system_prompt, runtime_config, is_builtin, created_at, updated_at
		FROM agents WHERE id = ?
	`, id).Scan(
		&a.ID, &a.Name, &a.Description, &a.Prompt, &toolsJSON,
		&model, &provider, &systemPrompt, &runtimeConfig, &a.IsBuiltin, &a.CreatedAt, &a.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get agent %s: %w", id, err)
	}

	if model.Valid {
		a.Model = model.String
	}
	if provider.Valid {
		a.Provider = provider.String
	}
	if systemPrompt.Valid {
		a.SystemPrompt = systemPrompt.String
	}
	if runtimeConfig.Valid {
		a.RuntimeConfig = runtimeConfig.String
	}

	if toolsJSON != "" {
		if err := json.Unmarshal([]byte(toolsJSON), &a.Tools); err != nil {
			return nil, fmt.Errorf("unmarshal agent tools: %w", err)
		}
	}

	return &a, nil
}

// ListAgents returns all agents from global DB.
func (g *GlobalDB) ListAgents() ([]*Agent, error) {
	rows, err := g.Query(`
		SELECT id, name, description, prompt, tools, model, provider, system_prompt, runtime_config, is_builtin, created_at, updated_at
		FROM agents
		ORDER BY is_builtin DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var agents []*Agent
	for rows.Next() {
		var a Agent
		var toolsJSON string
		var model, provider, systemPrompt, runtimeConfig sql.NullString

		if err := rows.Scan(
			&a.ID, &a.Name, &a.Description, &a.Prompt, &toolsJSON,
			&model, &provider, &systemPrompt, &runtimeConfig, &a.IsBuiltin, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}

		if model.Valid {
			a.Model = model.String
		}
		if provider.Valid {
			a.Provider = provider.String
		}
		if systemPrompt.Valid {
			a.SystemPrompt = systemPrompt.String
		}
		if runtimeConfig.Valid {
			a.RuntimeConfig = runtimeConfig.String
		}

		if toolsJSON != "" {
			if err := json.Unmarshal([]byte(toolsJSON), &a.Tools); err != nil {
				return nil, fmt.Errorf("unmarshal agent tools: %w", err)
			}
		}

		agents = append(agents, &a)
	}

	return agents, nil
}

// DeleteAgent deletes a non-builtin agent by ID from global DB.
func (g *GlobalDB) DeleteAgent(id string) error {
	agent, err := g.GetAgent(id)
	if err != nil {
		return err
	}
	if agent == nil {
		return nil
	}
	if agent.IsBuiltin {
		return fmt.Errorf("cannot delete builtin agent %s", id)
	}

	_, err = g.Exec("DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent %s: %w", id, err)
	}

	return nil
}

// CountAgents returns the number of agents in global DB.
func (g *GlobalDB) CountAgents() (int, error) {
	var count int
	err := g.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count agents: %w", err)
	}
	return count, nil
}

// SavePhaseAgent creates or updates a phase-agent association in global DB.
func (g *GlobalDB) SavePhaseAgent(pa *PhaseAgent) error {
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

	res, err := g.Exec(query,
		pa.PhaseTemplateID, pa.AgentID, pa.Sequence, pa.Role,
		weightFilterJSON, pa.IsBuiltin, pa.CreatedAt, pa.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save phase agent %s/%s: %w", pa.PhaseTemplateID, pa.AgentID, err)
	}

	if pa.ID == 0 {
		id, _ := res.LastInsertId()
		pa.ID = id
	}
	return nil
}

// GetPhaseAgents returns all agent associations for a phase template from global DB.
func (g *GlobalDB) GetPhaseAgents(phaseTemplateID string) ([]*PhaseAgent, error) {
	query := `
		SELECT id, phase_template_id, agent_id, sequence, role, weight_filter, is_builtin, created_at, updated_at
		FROM phase_agents
		WHERE phase_template_id = ?
		ORDER BY sequence ASC, agent_id ASC
	`

	rows, err := g.Query(query, phaseTemplateID)
	if err != nil {
		return nil, fmt.Errorf("get phase agents for %s: %w", phaseTemplateID, err)
	}
	defer func() { _ = rows.Close() }()

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

	return agents, rows.Err()
}

// GetPhaseAgentsForWeight returns agent associations for a phase template.
func (g *GlobalDB) GetPhaseAgentsForWeight(phaseTemplateID, weight string) ([]*PhaseAgent, error) {
	agents, err := g.GetPhaseAgents(phaseTemplateID)
	if err != nil {
		return nil, err
	}

	var filtered []*PhaseAgent
	for _, pa := range agents {
		if len(pa.WeightFilter) == 0 {
			filtered = append(filtered, pa)
			continue
		}
		for _, w := range pa.WeightFilter {
			if w == weight {
				filtered = append(filtered, pa)
				break
			}
		}
	}

	return filtered, nil
}

// GetPhaseAgentsWithDefinitions returns agent associations with full definitions from global DB.
func (g *GlobalDB) GetPhaseAgentsWithDefinitions(phaseTemplateID, weight string) ([]*AgentWithAssignment, error) {
	phaseAgents, err := g.GetPhaseAgentsForWeight(phaseTemplateID, weight)
	if err != nil {
		return nil, err
	}

	if len(phaseAgents) == 0 {
		return nil, nil
	}

	var result []*AgentWithAssignment
	for _, pa := range phaseAgents {
		agent, err := g.GetAgent(pa.AgentID)
		if err != nil {
			return nil, fmt.Errorf("get agent %s: %w", pa.AgentID, err)
		}
		if agent == nil {
			continue
		}

		result = append(result, &AgentWithAssignment{
			Agent:      agent,
			PhaseAgent: pa,
		})
	}

	return result, nil
}
