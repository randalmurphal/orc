package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/randalmurphal/llmkit/claudeconfig"
)

// === Agents Handlers ===

// handleListAgents returns all sub-agent definitions.
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	svc := claudeconfig.NewAgentService(s.getProjectRoot())
	agents, err := svc.List()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to list agents: %v", err), http.StatusInternalServerError)
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
