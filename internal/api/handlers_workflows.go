package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// handleListWorkflows returns all workflows.
func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	workflows, err := s.backend.ListWorkflows()
	if err != nil {
		s.jsonError(w, "failed to list workflows", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array, not null
	if workflows == nil {
		workflows = []*db.Workflow{}
	}

	// Optionally filter by builtin/custom
	builtinOnly := r.URL.Query().Get("builtin") == "true"
	customOnly := r.URL.Query().Get("custom") == "true"

	if builtinOnly || customOnly {
		var filtered []*db.Workflow
		for _, wf := range workflows {
			if builtinOnly && wf.IsBuiltin {
				filtered = append(filtered, wf)
			} else if customOnly && !wf.IsBuiltin {
				filtered = append(filtered, wf)
			}
		}
		workflows = filtered
		if workflows == nil {
			workflows = []*db.Workflow{}
		}
	}

	// Enrich with phase count
	type enrichedWorkflow struct {
		*db.Workflow
		PhaseCount int `json:"phase_count"`
	}

	result := make([]enrichedWorkflow, len(workflows))
	for i, wf := range workflows {
		phases, _ := s.backend.GetWorkflowPhases(wf.ID)
		result[i] = enrichedWorkflow{
			Workflow:   wf,
			PhaseCount: len(phases),
		}
	}

	s.jsonResponse(w, result)
}

// handleGetWorkflow returns a single workflow with its phases and variables.
func (s *Server) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	wf, err := s.backend.GetWorkflow(id)
	if err != nil {
		s.jsonError(w, "failed to get workflow", http.StatusInternalServerError)
		return
	}
	if wf == nil {
		s.jsonError(w, "workflow not found", http.StatusNotFound)
		return
	}

	// Get phases
	phases, err := s.backend.GetWorkflowPhases(id)
	if err != nil {
		s.jsonError(w, "failed to get workflow phases", http.StatusInternalServerError)
		return
	}
	if phases == nil {
		phases = []*db.WorkflowPhase{}
	}

	// Get variables
	variables, err := s.backend.GetWorkflowVariables(id)
	if err != nil {
		s.jsonError(w, "failed to get workflow variables", http.StatusInternalServerError)
		return
	}
	if variables == nil {
		variables = []*db.WorkflowVariable{}
	}

	// Return enriched workflow
	response := struct {
		*db.Workflow
		Phases    []*db.WorkflowPhase    `json:"phases"`
		Variables []*db.WorkflowVariable `json:"variables"`
	}{
		Workflow:  wf,
		Phases:    phases,
		Variables: variables,
	}

	s.jsonResponse(w, response)
}

// handleCreateWorkflow creates a new workflow.
func (s *Server) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID              string `json:"id"`
		Name            string `json:"name"`
		Description     string `json:"description,omitempty"`
		WorkflowType    string `json:"workflow_type,omitempty"`
		DefaultModel    string `json:"default_model,omitempty"`
		DefaultThinking bool   `json:"default_thinking,omitempty"`
		BasedOn         string `json:"based_on,omitempty"`
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
	if req.WorkflowType == "" {
		req.WorkflowType = "task"
	}

	// Check if workflow already exists
	existing, err := s.backend.GetWorkflow(req.ID)
	if err != nil {
		s.jsonError(w, "failed to check existing workflow", http.StatusInternalServerError)
		return
	}
	if existing != nil {
		s.jsonError(w, "workflow already exists", http.StatusConflict)
		return
	}

	now := time.Now()
	wf := &db.Workflow{
		ID:              req.ID,
		Name:            req.Name,
		Description:     req.Description,
		WorkflowType:    req.WorkflowType,
		DefaultModel:    req.DefaultModel,
		DefaultThinking: req.DefaultThinking,
		IsBuiltin:       false,
		BasedOn:         req.BasedOn,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// If based on another workflow, clone phases and variables
	if req.BasedOn != "" {
		source, err := s.backend.GetWorkflow(req.BasedOn)
		if err != nil || source == nil {
			s.jsonError(w, "base workflow not found", http.StatusBadRequest)
			return
		}

		if err := s.backend.SaveWorkflow(wf); err != nil {
			s.jsonError(w, "failed to create workflow", http.StatusInternalServerError)
			return
		}

		// Clone phases
		phases, _ := s.backend.GetWorkflowPhases(req.BasedOn)
		for _, p := range phases {
			newPhase := &db.WorkflowPhase{
				WorkflowID:            req.ID,
				PhaseTemplateID:       p.PhaseTemplateID,
				Sequence:              p.Sequence,
				DependsOn:             p.DependsOn,
				MaxIterationsOverride: p.MaxIterationsOverride,
				ModelOverride:         p.ModelOverride,
				ThinkingOverride:      p.ThinkingOverride,
				GateTypeOverride:      p.GateTypeOverride,
				Condition:             p.Condition,
			}
			if err := s.backend.SaveWorkflowPhase(newPhase); err != nil {
				s.jsonError(w, "failed to clone phase", http.StatusInternalServerError)
				return
			}
		}

		// Clone variables
		vars, _ := s.backend.GetWorkflowVariables(req.BasedOn)
		for _, v := range vars {
			newVar := &db.WorkflowVariable{
				WorkflowID:      req.ID,
				Name:            v.Name,
				Description:     v.Description,
				SourceType:      v.SourceType,
				SourceConfig:    v.SourceConfig,
				Required:        v.Required,
				DefaultValue:    v.DefaultValue,
				CacheTTLSeconds: v.CacheTTLSeconds,
			}
			if err := s.backend.SaveWorkflowVariable(newVar); err != nil {
				s.jsonError(w, "failed to clone variable", http.StatusInternalServerError)
				return
			}
		}
	} else {
		if err := s.backend.SaveWorkflow(wf); err != nil {
			s.jsonError(w, "failed to create workflow", http.StatusInternalServerError)
			return
		}
	}

	s.jsonResponse(w, wf)
}

// handleUpdateWorkflow updates an existing workflow.
func (s *Server) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	wf, err := s.backend.GetWorkflow(id)
	if err != nil {
		s.jsonError(w, "failed to get workflow", http.StatusInternalServerError)
		return
	}
	if wf == nil {
		s.jsonError(w, "workflow not found", http.StatusNotFound)
		return
	}

	if wf.IsBuiltin {
		s.jsonError(w, "cannot modify built-in workflow", http.StatusForbidden)
		return
	}

	var req struct {
		Name            *string `json:"name,omitempty"`
		Description     *string `json:"description,omitempty"`
		DefaultModel    *string `json:"default_model,omitempty"`
		DefaultThinking *bool   `json:"default_thinking,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		wf.Name = *req.Name
	}
	if req.Description != nil {
		wf.Description = *req.Description
	}
	if req.DefaultModel != nil {
		wf.DefaultModel = *req.DefaultModel
	}
	if req.DefaultThinking != nil {
		wf.DefaultThinking = *req.DefaultThinking
	}
	wf.UpdatedAt = time.Now()

	if err := s.backend.SaveWorkflow(wf); err != nil {
		s.jsonError(w, "failed to update workflow", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, wf)
}

// handleDeleteWorkflow deletes a workflow.
func (s *Server) handleDeleteWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	wf, err := s.backend.GetWorkflow(id)
	if err != nil {
		s.jsonError(w, "failed to get workflow", http.StatusInternalServerError)
		return
	}
	if wf == nil {
		s.jsonError(w, "workflow not found", http.StatusNotFound)
		return
	}

	if wf.IsBuiltin {
		s.jsonError(w, "cannot delete built-in workflow", http.StatusForbidden)
		return
	}

	if err := s.backend.DeleteWorkflow(id); err != nil {
		s.jsonError(w, "failed to delete workflow", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "deleted"})
}

// handleCloneWorkflow creates a copy of an existing workflow.
func (s *Server) handleCloneWorkflow(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("id")

	var req struct {
		NewID       string `json:"new_id"`
		Name        string `json:"name,omitempty"`
		Description string `json:"description,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.NewID == "" {
		s.jsonError(w, "new_id is required", http.StatusBadRequest)
		return
	}

	// Check source exists
	source, err := s.backend.GetWorkflow(sourceID)
	if err != nil || source == nil {
		s.jsonError(w, "source workflow not found", http.StatusNotFound)
		return
	}

	// Check target doesn't exist
	existing, _ := s.backend.GetWorkflow(req.NewID)
	if existing != nil {
		s.jsonError(w, "workflow with new_id already exists", http.StatusConflict)
		return
	}

	// Create clone
	now := time.Now()
	clone := &db.Workflow{
		ID:              req.NewID,
		Name:            req.Name,
		Description:     req.Description,
		WorkflowType:    source.WorkflowType,
		DefaultModel:    source.DefaultModel,
		DefaultThinking: source.DefaultThinking,
		IsBuiltin:       false,
		BasedOn:         sourceID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if clone.Name == "" {
		clone.Name = req.NewID
	}
	if clone.Description == "" {
		clone.Description = source.Description
	}

	if err := s.backend.SaveWorkflow(clone); err != nil {
		s.jsonError(w, "failed to create workflow clone", http.StatusInternalServerError)
		return
	}

	// Clone phases
	phases, _ := s.backend.GetWorkflowPhases(sourceID)
	for _, p := range phases {
		newPhase := &db.WorkflowPhase{
			WorkflowID:            req.NewID,
			PhaseTemplateID:       p.PhaseTemplateID,
			Sequence:              p.Sequence,
			DependsOn:             p.DependsOn,
			MaxIterationsOverride: p.MaxIterationsOverride,
			ModelOverride:         p.ModelOverride,
			ThinkingOverride:      p.ThinkingOverride,
			GateTypeOverride:      p.GateTypeOverride,
			Condition:             p.Condition,
		}
		if err := s.backend.SaveWorkflowPhase(newPhase); err != nil {
			s.jsonError(w, "failed to clone phase", http.StatusInternalServerError)
			return
		}
	}

	// Clone variables
	vars, _ := s.backend.GetWorkflowVariables(sourceID)
	for _, v := range vars {
		newVar := &db.WorkflowVariable{
			WorkflowID:      req.NewID,
			Name:            v.Name,
			Description:     v.Description,
			SourceType:      v.SourceType,
			SourceConfig:    v.SourceConfig,
			Required:        v.Required,
			DefaultValue:    v.DefaultValue,
			CacheTTLSeconds: v.CacheTTLSeconds,
		}
		if err := s.backend.SaveWorkflowVariable(newVar); err != nil {
			s.jsonError(w, "failed to clone variable", http.StatusInternalServerError)
			return
		}
	}

	s.jsonResponse(w, clone)
}

// handleAddWorkflowPhase adds a phase to a workflow.
func (s *Server) handleAddWorkflowPhase(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")

	wf, err := s.backend.GetWorkflow(workflowID)
	if err != nil || wf == nil {
		s.jsonError(w, "workflow not found", http.StatusNotFound)
		return
	}

	if wf.IsBuiltin {
		s.jsonError(w, "cannot modify built-in workflow", http.StatusForbidden)
		return
	}

	var req struct {
		PhaseTemplateID       string `json:"phase_template_id"`
		Sequence              int    `json:"sequence"`
		MaxIterationsOverride *int   `json:"max_iterations_override,omitempty"`
		ModelOverride         string `json:"model_override,omitempty"`
		GateTypeOverride      string `json:"gate_type_override,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.PhaseTemplateID == "" {
		s.jsonError(w, "phase_template_id is required", http.StatusBadRequest)
		return
	}

	// Verify phase template exists
	tmpl, err := s.backend.GetPhaseTemplate(req.PhaseTemplateID)
	if err != nil || tmpl == nil {
		s.jsonError(w, "phase template not found", http.StatusBadRequest)
		return
	}

	phase := &db.WorkflowPhase{
		WorkflowID:            workflowID,
		PhaseTemplateID:       req.PhaseTemplateID,
		Sequence:              req.Sequence,
		MaxIterationsOverride: req.MaxIterationsOverride,
		ModelOverride:         req.ModelOverride,
		GateTypeOverride:      req.GateTypeOverride,
	}

	if err := s.backend.SaveWorkflowPhase(phase); err != nil {
		s.jsonError(w, "failed to add phase", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, phase)
}

// handleRemoveWorkflowPhase removes a phase from a workflow.
func (s *Server) handleRemoveWorkflowPhase(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	phaseTemplateID := r.PathValue("phaseId")

	wf, err := s.backend.GetWorkflow(workflowID)
	if err != nil || wf == nil {
		s.jsonError(w, "workflow not found", http.StatusNotFound)
		return
	}

	if wf.IsBuiltin {
		s.jsonError(w, "cannot modify built-in workflow", http.StatusForbidden)
		return
	}

	if err := s.backend.DeleteWorkflowPhase(workflowID, phaseTemplateID); err != nil {
		s.jsonError(w, "failed to remove phase", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "removed"})
}

// handleAddWorkflowVariable adds a variable to a workflow.
func (s *Server) handleAddWorkflowVariable(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")

	wf, err := s.backend.GetWorkflow(workflowID)
	if err != nil || wf == nil {
		s.jsonError(w, "workflow not found", http.StatusNotFound)
		return
	}

	if wf.IsBuiltin {
		s.jsonError(w, "cannot modify built-in workflow", http.StatusForbidden)
		return
	}

	var req struct {
		Name            string          `json:"name"`
		Description     string          `json:"description,omitempty"`
		SourceType      string          `json:"source_type"`
		SourceConfig    json.RawMessage `json:"source_config"`
		Required        bool            `json:"required,omitempty"`
		DefaultValue    string          `json:"default_value,omitempty"`
		CacheTTLSeconds int             `json:"cache_ttl_seconds,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		s.jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	if req.SourceType == "" {
		s.jsonError(w, "source_type is required", http.StatusBadRequest)
		return
	}

	variable := &db.WorkflowVariable{
		WorkflowID:      workflowID,
		Name:            req.Name,
		Description:     req.Description,
		SourceType:      req.SourceType,
		SourceConfig:    string(req.SourceConfig),
		Required:        req.Required,
		DefaultValue:    req.DefaultValue,
		CacheTTLSeconds: req.CacheTTLSeconds,
	}

	if err := s.backend.SaveWorkflowVariable(variable); err != nil {
		s.jsonError(w, "failed to add variable", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, variable)
}

// handleRemoveWorkflowVariable removes a variable from a workflow.
func (s *Server) handleRemoveWorkflowVariable(w http.ResponseWriter, r *http.Request) {
	workflowID := r.PathValue("id")
	name := r.PathValue("name")

	wf, err := s.backend.GetWorkflow(workflowID)
	if err != nil || wf == nil {
		s.jsonError(w, "workflow not found", http.StatusNotFound)
		return
	}

	if wf.IsBuiltin {
		s.jsonError(w, "cannot modify built-in workflow", http.StatusForbidden)
		return
	}

	if err := s.backend.DeleteWorkflowVariable(workflowID, name); err != nil {
		s.jsonError(w, "failed to remove variable", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "removed"})
}
