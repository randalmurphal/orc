package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Agent represents an agent definition stored in the database.
// Agents can be used as:
// - Sub-agents: passed to Claude CLI via --agents JSON (uses Prompt field)
// - Main executors: used as the phase executor (uses SystemPrompt field)
type Agent struct {
	ID          string   `json:"id"`          // 'code-reviewer', 'silent-failure-hunter', etc.
	Name        string   `json:"name"`        // Display name
	Description string   `json:"description"` // When to use (required by Claude CLI for sub-agents)
	Prompt      string   `json:"prompt"`      // Context prompt for sub-agent role (what this agent does)
	Tools       []string `json:"tools"`       // Allowed tools: ["Read", "Grep", "Edit"]
	Model       string   `json:"model"`       // 'opus', 'sonnet', 'haiku' (optional override)

	// Executor role fields (used when agent is the main executor for a phase)
	SystemPrompt string `json:"system_prompt,omitempty"` // Role framing for executor
	ClaudeConfig string `json:"claude_config,omitempty"` // JSON: additional claude settings

	IsBuiltin bool   `json:"is_builtin"` // True for built-in agents
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// SaveAgent saves or updates an agent definition.
func (pdb *ProjectDB) SaveAgent(a *Agent) error {
	toolsJSON, err := json.Marshal(a.Tools)
	if err != nil {
		return fmt.Errorf("marshal agent tools: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	if a.CreatedAt == "" {
		a.CreatedAt = now
	}
	a.UpdatedAt = now

	query := `
		INSERT INTO agents (id, name, description, prompt, tools, model, system_prompt, claude_config, is_builtin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			prompt = excluded.prompt,
			tools = excluded.tools,
			model = excluded.model,
			system_prompt = excluded.system_prompt,
			claude_config = excluded.claude_config,
			is_builtin = excluded.is_builtin,
			updated_at = excluded.updated_at
	`

	_, err = pdb.Exec(query,
		a.ID, a.Name, a.Description, a.Prompt, string(toolsJSON),
		a.Model, a.SystemPrompt, a.ClaudeConfig, a.IsBuiltin, a.CreatedAt, a.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("save agent %s: %w", a.ID, err)
	}

	return nil
}

// GetAgent retrieves an agent by ID.
func (pdb *ProjectDB) GetAgent(id string) (*Agent, error) {
	query := `
		SELECT id, name, description, prompt, tools, model, system_prompt, claude_config, is_builtin, created_at, updated_at
		FROM agents
		WHERE id = ?
	`

	var a Agent
	var toolsJSON string
	var model, systemPrompt, claudeConfig sql.NullString

	err := pdb.QueryRow(query, id).Scan(
		&a.ID, &a.Name, &a.Description, &a.Prompt, &toolsJSON,
		&model, &systemPrompt, &claudeConfig, &a.IsBuiltin, &a.CreatedAt, &a.UpdatedAt,
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
	if systemPrompt.Valid {
		a.SystemPrompt = systemPrompt.String
	}
	if claudeConfig.Valid {
		a.ClaudeConfig = claudeConfig.String
	}

	if toolsJSON != "" {
		if err := json.Unmarshal([]byte(toolsJSON), &a.Tools); err != nil {
			return nil, fmt.Errorf("unmarshal agent tools: %w", err)
		}
	}

	return &a, nil
}

// ListAgents returns all agents.
func (pdb *ProjectDB) ListAgents() ([]*Agent, error) {
	query := `
		SELECT id, name, description, prompt, tools, model, system_prompt, claude_config, is_builtin, created_at, updated_at
		FROM agents
		ORDER BY is_builtin DESC, name ASC
	`

	rows, err := pdb.Query(query)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var agents []*Agent
	for rows.Next() {
		var a Agent
		var toolsJSON string
		var model, systemPrompt, claudeConfig sql.NullString

		if err := rows.Scan(
			&a.ID, &a.Name, &a.Description, &a.Prompt, &toolsJSON,
			&model, &systemPrompt, &claudeConfig, &a.IsBuiltin, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}

		if model.Valid {
			a.Model = model.String
		}
		if systemPrompt.Valid {
			a.SystemPrompt = systemPrompt.String
		}
		if claudeConfig.Valid {
			a.ClaudeConfig = claudeConfig.String
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

// DeleteAgent deletes an agent by ID.
// Returns error if agent is builtin.
func (pdb *ProjectDB) DeleteAgent(id string) error {
	// Check if builtin
	agent, err := pdb.GetAgent(id)
	if err != nil {
		return err
	}
	if agent == nil {
		return nil // Already doesn't exist
	}
	if agent.IsBuiltin {
		return fmt.Errorf("cannot delete builtin agent %s", id)
	}

	_, err = pdb.Exec("DELETE FROM agents WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete agent %s: %w", id, err)
	}

	return nil
}

// CountAgents returns the number of agents.
func (pdb *ProjectDB) CountAgents() (int, error) {
	var count int
	err := pdb.QueryRow("SELECT COUNT(*) FROM agents").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count agents: %w", err)
	}
	return count, nil
}
