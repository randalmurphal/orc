package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// handleListPhaseTemplates returns all phase templates.
func (s *Server) handleListPhaseTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := s.backend.ListPhaseTemplates()
	if err != nil {
		s.jsonError(w, "failed to list phase templates", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array, not null
	if templates == nil {
		templates = []*db.PhaseTemplate{}
	}

	// Optionally filter by builtin/custom
	builtinOnly := r.URL.Query().Get("builtin") == "true"
	customOnly := r.URL.Query().Get("custom") == "true"

	if builtinOnly || customOnly {
		var filtered []*db.PhaseTemplate
		for _, t := range templates {
			if builtinOnly && t.IsBuiltin {
				filtered = append(filtered, t)
			} else if customOnly && !t.IsBuiltin {
				filtered = append(filtered, t)
			}
		}
		templates = filtered
		if templates == nil {
			templates = []*db.PhaseTemplate{}
		}
	}

	s.jsonResponse(w, templates)
}

// handleGetPhaseTemplate returns a single phase template.
func (s *Server) handleGetPhaseTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	tmpl, err := s.backend.GetPhaseTemplate(id)
	if err != nil {
		s.jsonError(w, "failed to get phase template", http.StatusInternalServerError)
		return
	}
	if tmpl == nil {
		s.jsonError(w, "phase template not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, tmpl)
}

// handleCreatePhaseTemplate creates a new phase template.
func (s *Server) handleCreatePhaseTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID               string `json:"id"`
		Name             string `json:"name"`
		Description      string `json:"description,omitempty"`
		PromptSource     string `json:"prompt_source,omitempty"`
		PromptContent    string `json:"prompt_content,omitempty"`
		PromptPath       string `json:"prompt_path,omitempty"`
		InputVariables   string `json:"input_variables,omitempty"`
		OutputSchema     string `json:"output_schema,omitempty"`
		ProducesArtifact bool   `json:"produces_artifact,omitempty"`
		ArtifactType     string `json:"artifact_type,omitempty"`
		MaxIterations    int    `json:"max_iterations,omitempty"`
		ModelOverride    string `json:"model_override,omitempty"`
		ThinkingEnabled  *bool  `json:"thinking_enabled,omitempty"`
		GateType         string `json:"gate_type,omitempty"`
		Checkpoint       bool   `json:"checkpoint,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		s.jsonError(w, "id is required", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		req.Name = req.ID
	}
	if req.PromptSource == "" {
		req.PromptSource = "db"
	}
	if req.MaxIterations == 0 {
		req.MaxIterations = 20
	}
	if req.GateType == "" {
		req.GateType = "auto"
	}

	// Check if template already exists
	existing, err := s.backend.GetPhaseTemplate(req.ID)
	if err != nil {
		s.jsonError(w, "failed to check existing template", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		s.jsonError(w, "phase template already exists", http.StatusConflict)
		return
	}

	now := time.Now()
	tmpl := &db.PhaseTemplate{
		ID:               req.ID,
		Name:             req.Name,
		Description:      req.Description,
		PromptSource:     req.PromptSource,
		PromptContent:    req.PromptContent,
		PromptPath:       req.PromptPath,
		InputVariables:   req.InputVariables,
		OutputSchema:     req.OutputSchema,
		ProducesArtifact: req.ProducesArtifact,
		ArtifactType:     req.ArtifactType,
		MaxIterations:    req.MaxIterations,
		ModelOverride:    req.ModelOverride,
		ThinkingEnabled:  req.ThinkingEnabled,
		GateType:         req.GateType,
		Checkpoint:       req.Checkpoint,
		IsBuiltin:        false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.backend.SavePhaseTemplate(tmpl); err != nil {
		s.jsonError(w, "failed to create phase template", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, tmpl)
}

// handleUpdatePhaseTemplate updates an existing phase template.
func (s *Server) handleUpdatePhaseTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	tmpl, err := s.backend.GetPhaseTemplate(id)
	if err != nil {
		s.jsonError(w, "failed to get phase template", http.StatusInternalServerError)
		return
	}
	if tmpl == nil {
		s.jsonError(w, "phase template not found", http.StatusNotFound)
		return
	}

	if tmpl.IsBuiltin {
		s.jsonError(w, "cannot modify built-in phase template", http.StatusForbidden)
		return
	}

	var req struct {
		Name             *string `json:"name,omitempty"`
		Description      *string `json:"description,omitempty"`
		PromptContent    *string `json:"prompt_content,omitempty"`
		MaxIterations    *int    `json:"max_iterations,omitempty"`
		ModelOverride    *string `json:"model_override,omitempty"`
		ThinkingEnabled  *bool   `json:"thinking_enabled,omitempty"`
		GateType         *string `json:"gate_type,omitempty"`
		ProducesArtifact *bool   `json:"produces_artifact,omitempty"`
		ArtifactType     *string `json:"artifact_type,omitempty"`
		Checkpoint       *bool   `json:"checkpoint,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		tmpl.Name = *req.Name
	}
	if req.Description != nil {
		tmpl.Description = *req.Description
	}
	if req.PromptContent != nil {
		tmpl.PromptContent = *req.PromptContent
	}
	if req.MaxIterations != nil {
		tmpl.MaxIterations = *req.MaxIterations
	}
	if req.ModelOverride != nil {
		tmpl.ModelOverride = *req.ModelOverride
	}
	if req.ThinkingEnabled != nil {
		tmpl.ThinkingEnabled = req.ThinkingEnabled
	}
	if req.GateType != nil {
		tmpl.GateType = *req.GateType
	}
	if req.ProducesArtifact != nil {
		tmpl.ProducesArtifact = *req.ProducesArtifact
	}
	if req.ArtifactType != nil {
		tmpl.ArtifactType = *req.ArtifactType
	}
	if req.Checkpoint != nil {
		tmpl.Checkpoint = *req.Checkpoint
	}
	tmpl.UpdatedAt = time.Now()

	if err := s.backend.SavePhaseTemplate(tmpl); err != nil {
		s.jsonError(w, "failed to update phase template", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, tmpl)
}

// handleDeletePhaseTemplate deletes a phase template.
func (s *Server) handleDeletePhaseTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	tmpl, err := s.backend.GetPhaseTemplate(id)
	if err != nil {
		s.jsonError(w, "failed to get phase template", http.StatusInternalServerError)
		return
	}
	if tmpl == nil {
		s.jsonError(w, "phase template not found", http.StatusNotFound)
		return
	}

	if tmpl.IsBuiltin {
		s.jsonError(w, "cannot delete built-in phase template", http.StatusForbidden)
		return
	}

	// Check if template is used by any workflows
	workflows, err := s.backend.ListWorkflows()
	if err == nil {
		for _, wf := range workflows {
			phases, _ := s.backend.GetWorkflowPhases(wf.ID)
			for _, p := range phases {
				if p.PhaseTemplateID == id {
					s.jsonError(w, "phase template is used by workflow: "+wf.ID, http.StatusConflict)
					return
				}
			}
		}
	}

	if err := s.backend.DeletePhaseTemplate(id); err != nil {
		s.jsonError(w, "failed to delete phase template", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "deleted"})
}

// handleGetPhaseTemplatePrompt returns the prompt content for a phase template.
func (s *Server) handleGetPhaseTemplatePrompt(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	tmpl, err := s.backend.GetPhaseTemplate(id)
	if err != nil {
		s.jsonError(w, "failed to get phase template", http.StatusInternalServerError)
		return
	}
	if tmpl == nil {
		s.jsonError(w, "phase template not found", http.StatusNotFound)
		return
	}

	// Return prompt content based on source
	var content string
	switch tmpl.PromptSource {
	case "db":
		content = tmpl.PromptContent
	case "embedded":
		// Would need to load from embedded templates
		// For now, return the path as a hint
		content = "<!-- Embedded prompt at: " + tmpl.PromptPath + " -->"
	case "file":
		// Would need to load from file
		content = "<!-- File prompt at: " + tmpl.PromptPath + " -->"
	}

	s.jsonResponse(w, map[string]string{
		"source":  tmpl.PromptSource,
		"path":    tmpl.PromptPath,
		"content": content,
	})
}
