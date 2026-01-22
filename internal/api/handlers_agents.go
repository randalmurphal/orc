package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/randalmurphal/llmkit/claudeconfig"
	"github.com/randalmurphal/orc/internal/storage"
)

// === Agents Handlers ===

// handleListAgents returns all sub-agent definitions.
// Supports ?scope=global to list from ~/.claude/agents/ .md files.
// For project scope, uses AgentService which reads from settings.json.
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")

	if scope == "global" {
		// Global agents are stored as .md files in ~/.claude/agents/
		homeDir, err := os.UserHomeDir()
		if err != nil {
			s.jsonError(w, "failed to get home directory", http.StatusInternalServerError)
			return
		}
		agents, err := claudeconfig.DiscoverAgents(filepath.Join(homeDir, ".claude"))
		if err != nil {
			// No agents directory is OK - return empty list
			s.jsonResponse(w, []claudeconfig.AgentInfo{})
			return
		}
		// Convert to AgentInfo for response
		infos := make([]claudeconfig.AgentInfo, 0, len(agents))
		for _, a := range agents {
			infos = append(infos, a.Info())
		}
		s.jsonResponse(w, infos)
		return
	}

	// Project agents from settings.json
	svc := claudeconfig.NewAgentService(s.getProjectRoot())
	agents, err := svc.List()
	if err != nil {
		// No agents configured is OK - return empty list
		s.jsonResponse(w, []claudeconfig.SubAgent{})
		return
	}

	s.jsonResponse(w, agents)
}

// handleGetAgent returns a specific agent by name.
func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := claudeconfig.NewAgentService(s.getProjectRoot())

	agent, err := svc.Get(name)
	if err != nil {
		s.jsonError(w, "agent not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, agent)
}

// handleCreateAgent creates a new sub-agent.
func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var agent claudeconfig.SubAgent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := claudeconfig.NewAgentService(s.getProjectRoot())
	if err := svc.Create(agent); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, agent)
}

// handleUpdateAgent updates an existing agent.
func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var agent claudeconfig.SubAgent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := claudeconfig.NewAgentService(s.getProjectRoot())
	if err := svc.Update(name, agent); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return updated agent
	updated, _ := svc.Get(agent.Name)
	if updated == nil {
		updated, _ = svc.Get(name)
	}
	s.jsonResponse(w, updated)
}

// handleDeleteAgent deletes an agent.
func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := claudeconfig.NewAgentService(s.getProjectRoot())

	if err := svc.Delete(name); err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// === Agent Stats Types ===

// AgentStatsEntry represents stats for a single agent.
type AgentStatsEntry struct {
	Name         string            `json:"name"`
	Model        string            `json:"model"`
	Status       string            `json:"status"` // "active" or "idle"
	Stats        AgentStatsMetrics `json:"stats"`
	Tools        []string          `json:"tools"`
	LastActivity *time.Time        `json:"last_activity,omitempty"`
}

// AgentStatsMetrics contains the numeric stats for an agent.
type AgentStatsMetrics struct {
	TokensToday        int     `json:"tokens_today"`
	TasksDoneToday     int     `json:"tasks_done_today"`
	TasksDoneTotal     int     `json:"tasks_done_total"`
	SuccessRate        float64 `json:"success_rate"`
	AvgTaskTimeSeconds int     `json:"avg_task_time_seconds"`
}

// AgentStatsSummary contains aggregate stats across all agents.
type AgentStatsSummary struct {
	TotalAgents      int `json:"total_agents"`
	ActiveAgents     int `json:"active_agents"`
	TotalTokensToday int `json:"total_tokens_today"`
	TotalTasksToday  int `json:"total_tasks_today"`
}

// AgentStatsResponse is the response for GET /api/agents/stats.
type AgentStatsResponse struct {
	Agents  []AgentStatsEntry `json:"agents"`
	Summary AgentStatsSummary `json:"summary"`
}

// handleGetAgentStats returns per-agent statistics.
// GET /api/agents/stats
func (s *Server) handleGetAgentStats(w http.ResponseWriter, r *http.Request) {
	// Get the project database
	dbBackend, ok := s.backend.(*storage.DatabaseBackend)
	if !ok {
		s.jsonError(w, "stats not available", http.StatusServiceUnavailable)
		return
	}
	pdb := dbBackend.DB()

	// Calculate midnight local time for "today"
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Get stats from database grouped by model
	modelStats, err := pdb.GetAgentStats(today)
	if err != nil {
		s.jsonError(w, fmt.Sprintf("get agent stats: %v", err), http.StatusInternalServerError)
		return
	}

	// Get configured agents from settings.json
	svc := claudeconfig.NewAgentService(s.getProjectRoot())
	configuredAgents, _ := svc.List() // Ignore error, may not have any configured

	// Build response: merge configured agents with their stats
	response := AgentStatsResponse{
		Agents: make([]AgentStatsEntry, 0),
	}

	// Track which models have been assigned to named agents
	assignedModels := make(map[string]bool)

	// First, add all configured agents with their stats
	for _, agent := range configuredAgents {
		model := agent.Model
		if model == "" {
			model = "default" // Agents without explicit model use default
		}

		entry := AgentStatsEntry{
			Name:   agent.Name,
			Model:  model,
			Status: "idle",
			Tools:  extractToolNames(agent.Tools),
		}

		// Get stats for this agent's model
		if stats, ok := modelStats[model]; ok {
			entry.Stats = AgentStatsMetrics{
				TokensToday:        stats.TokensToday,
				TasksDoneToday:     stats.TasksDoneToday,
				TasksDoneTotal:     stats.TasksDoneTotal,
				SuccessRate:        stats.SuccessRate,
				AvgTaskTimeSeconds: stats.AvgTaskTimeSeconds,
			}
			entry.LastActivity = stats.LastActivity
			if stats.IsActive {
				entry.Status = "active"
			}
			assignedModels[model] = true
		}

		response.Agents = append(response.Agents, entry)
	}

	// Add entries for models that have stats but no configured agent
	// This handles the case where agents aren't configured but tasks have been run
	for model, stats := range modelStats {
		if assignedModels[model] {
			continue
		}

		entry := AgentStatsEntry{
			Name:   deriveAgentName(model), // Create a display name from model
			Model:  model,
			Status: "idle",
			Stats: AgentStatsMetrics{
				TokensToday:        stats.TokensToday,
				TasksDoneToday:     stats.TasksDoneToday,
				TasksDoneTotal:     stats.TasksDoneTotal,
				SuccessRate:        stats.SuccessRate,
				AvgTaskTimeSeconds: stats.AvgTaskTimeSeconds,
			},
			Tools:        []string{}, // Unknown tools for unconfigured agents
			LastActivity: stats.LastActivity,
		}
		if stats.IsActive {
			entry.Status = "active"
		}

		response.Agents = append(response.Agents, entry)
	}

	// Sort agents: active first, then by name
	sort.Slice(response.Agents, func(i, j int) bool {
		if response.Agents[i].Status != response.Agents[j].Status {
			return response.Agents[i].Status == "active" // active before idle
		}
		return response.Agents[i].Name < response.Agents[j].Name
	})

	// Calculate summary
	response.Summary.TotalAgents = len(response.Agents)
	for _, agent := range response.Agents {
		if agent.Status == "active" {
			response.Summary.ActiveAgents++
		}
		response.Summary.TotalTokensToday += agent.Stats.TokensToday
		response.Summary.TotalTasksToday += agent.Stats.TasksDoneToday
	}

	s.jsonResponse(w, response)
}

// extractToolNames converts tool permissions to a list of tool names.
func extractToolNames(tools *claudeconfig.ToolPermissions) []string {
	if tools == nil {
		return []string{}
	}

	var names []string
	if len(tools.Allow) > 0 {
		names = append(names, tools.Allow...)
	}
	return names
}

// deriveAgentName creates a human-readable name from a model identifier.
func deriveAgentName(model string) string {
	// Map common model patterns to friendly names
	switch {
	case stringContainsIgnoreCase(model, "opus"):
		return "Opus Agent"
	case stringContainsIgnoreCase(model, "sonnet"):
		return "Sonnet Agent"
	case stringContainsIgnoreCase(model, "haiku"):
		return "Haiku Agent"
	default:
		// Fallback: use the model name with some cleanup
		return model
	}
}

// stringContainsIgnoreCase checks if s contains substr (case-insensitive).
func stringContainsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
