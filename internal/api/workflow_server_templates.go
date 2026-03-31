package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
	"github.com/randalmurphal/orc/templates"
)

// ListPhaseTemplates returns all phase templates.
func (s *workflowServer) ListPhaseTemplates(
	ctx context.Context,
	req *connect.Request[orcv1.ListPhaseTemplatesRequest],
) (*connect.Response[orcv1.ListPhaseTemplatesResponse], error) {
	phaseTemplates, err := s.globalDB.ListPhaseTemplates()
	if err != nil {
		return connect.NewResponse(&orcv1.ListPhaseTemplatesResponse{
			Templates: []*orcv1.PhaseTemplate{},
		}), nil
	}

	if !req.Msg.IncludeBuiltin {
		var filtered []*db.PhaseTemplate
		for _, t := range phaseTemplates {
			if !t.IsBuiltin {
				filtered = append(filtered, t)
			}
		}
		phaseTemplates = filtered
	}

	var resolvedPhases []workflow.ResolvedPhase
	if s.resolver != nil {
		resolvedPhases, _ = s.resolver.ListPhases()
	}
	sourceMap := make(map[string]workflow.Source)
	for _, rp := range resolvedPhases {
		sourceMap[rp.Phase.ID] = rp.Source
	}

	protoTemplates := make([]*orcv1.PhaseTemplate, len(phaseTemplates))
	sources := make(map[string]orcv1.DefinitionSource, len(phaseTemplates))

	for i, t := range phaseTemplates {
		protoTemplates[i] = dbPhaseTemplateToProto(t)
		if src, ok := sourceMap[t.ID]; ok {
			sources[t.ID] = workflowSourceToProto(src)
		} else if t.IsBuiltin {
			sources[t.ID] = orcv1.DefinitionSource_DEFINITION_SOURCE_EMBEDDED
		} else {
			sources[t.ID] = orcv1.DefinitionSource_DEFINITION_SOURCE_PROJECT
		}
	}

	return connect.NewResponse(&orcv1.ListPhaseTemplatesResponse{
		Templates: protoTemplates,
		Sources:   sources,
	}), nil
}

// GetPhaseTemplate returns a single phase template.
func (s *workflowServer) GetPhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.GetPhaseTemplateRequest],
) (*connect.Response[orcv1.GetPhaseTemplateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	tmpl, err := s.globalDB.GetPhaseTemplate(req.Msg.Id)
	if err != nil || tmpl == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase template %s not found", req.Msg.Id))
	}

	return connect.NewResponse(&orcv1.GetPhaseTemplateResponse{
		Template: dbPhaseTemplateToProto(tmpl),
	}), nil
}

// CreatePhaseTemplate creates a new phase template.
func (s *workflowServer) CreatePhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.CreatePhaseTemplateRequest],
) (*connect.Response[orcv1.CreatePhaseTemplateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}

	existing, _ := s.globalDB.GetPhaseTemplate(req.Msg.Id)
	if existing != nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("phase template %s already exists", req.Msg.Id))
	}

	tmpl := &db.PhaseTemplate{
		ID:               req.Msg.Id,
		Name:             req.Msg.Name,
		PromptSource:     protoPromptSourceToString(req.Msg.PromptSource),
		ProducesArtifact: req.Msg.ProducesArtifact,
		GateType:         protoGateTypeToString(req.Msg.GateType),
		Checkpoint:       req.Msg.Checkpoint,
		IsBuiltin:        false,
	}
	if req.Msg.Description != nil {
		tmpl.Description = *req.Msg.Description
	}
	if req.Msg.PromptContent != nil {
		tmpl.PromptContent = *req.Msg.PromptContent
	}
	if req.Msg.PromptPath != nil {
		tmpl.PromptPath = *req.Msg.PromptPath
	}
	if req.Msg.OutputSchema != nil {
		tmpl.OutputSchema = *req.Msg.OutputSchema
	}
	if req.Msg.ArtifactType != nil {
		tmpl.ArtifactType = *req.Msg.ArtifactType
	}
	if req.Msg.OutputVarName != nil {
		tmpl.OutputVarName = *req.Msg.OutputVarName
	}
	if len(req.Msg.InputVariables) > 0 {
		if jsonBytes, err := json.Marshal(req.Msg.InputVariables); err == nil {
			tmpl.InputVariables = string(jsonBytes)
		}
	}
	if req.Msg.ThinkingEnabled != nil {
		tmpl.ThinkingEnabled = req.Msg.ThinkingEnabled
	}
	if req.Msg.RuntimeConfig != nil {
		tmpl.RuntimeConfig = *req.Msg.RuntimeConfig
	}
	if req.Msg.Provider != nil {
		if err := validateProviderString(*req.Msg.Provider); err != nil {
			return nil, err
		}
		tmpl.Provider = *req.Msg.Provider
	}

	if tmpl.PromptSource == "" {
		tmpl.PromptSource = "db"
	}
	if tmpl.GateType == "" {
		tmpl.GateType = "auto"
	}

	if err := s.globalDB.SavePhaseTemplate(tmpl); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save phase template: %w", err))
	}

	return connect.NewResponse(&orcv1.CreatePhaseTemplateResponse{
		Template: dbPhaseTemplateToProto(tmpl),
	}), nil
}

// UpdatePhaseTemplate updates an existing phase template.
func (s *workflowServer) UpdatePhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.UpdatePhaseTemplateRequest],
) (*connect.Response[orcv1.UpdatePhaseTemplateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	resolved, resolveErr := s.resolver.ResolvePhase(req.Msg.Id)
	if resolveErr != nil {
		dbTmpl, dbErr := s.globalDB.GetPhaseTemplate(req.Msg.Id)
		if dbErr != nil || dbTmpl == nil {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase template %s not found", req.Msg.Id))
		}
		if dbTmpl.IsBuiltin {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in phase template"))
		}
		return s.updateDBOnlyPhaseTemplate(dbTmpl, req.Msg)
	}

	if resolved.Source == workflow.SourceEmbedded {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in phase template"))
	}

	pt := resolved.Phase
	if req.Msg.Name != nil {
		pt.Name = *req.Msg.Name
	}
	if req.Msg.Description != nil {
		pt.Description = *req.Msg.Description
	}
	if req.Msg.PromptSource != nil {
		pt.PromptSource = workflow.PromptSource(protoPromptSourceToString(*req.Msg.PromptSource))
	}
	if req.Msg.PromptContent != nil {
		pt.PromptContent = *req.Msg.PromptContent
	}
	if req.Msg.PromptPath != nil {
		pt.PromptPath = *req.Msg.PromptPath
	}
	if req.Msg.OutputSchema != nil {
		pt.OutputSchema = *req.Msg.OutputSchema
	}
	if req.Msg.ProducesArtifact != nil {
		pt.ProducesArtifact = *req.Msg.ProducesArtifact
	}
	if req.Msg.ArtifactType != nil {
		pt.ArtifactType = *req.Msg.ArtifactType
	}
	if req.Msg.OutputVarName != nil {
		pt.OutputVarName = *req.Msg.OutputVarName
	}
	if req.Msg.InputVariables != nil {
		pt.InputVariables = req.Msg.InputVariables
	}
	if req.Msg.ThinkingEnabled != nil {
		pt.ThinkingEnabled = req.Msg.ThinkingEnabled
	}
	if req.Msg.RuntimeConfig != nil {
		pt.RuntimeConfig = *req.Msg.RuntimeConfig
	}
	if req.Msg.GateType != nil {
		pt.GateType = workflow.GateType(protoGateTypeToString(*req.Msg.GateType))
	}
	if req.Msg.Checkpoint != nil {
		pt.Checkpoint = *req.Msg.Checkpoint
	}
	if req.Msg.Provider != nil {
		if err := validateProviderString(*req.Msg.Provider); err != nil {
			return nil, err
		}
		pt.Provider = *req.Msg.Provider
	}

	writeLevel := workflow.SourceToWriteLevel(resolved.Source)
	if writeLevel != "" {
		writer := workflow.NewWriterFromOrcDir(s.resolver.OrcDir())
		if _, writeErr := writer.WritePhase(pt, writeLevel); writeErr != nil {
			s.logger.Warn("failed to write phase file", "id", req.Msg.Id, "error", writeErr)
		}
	}

	if _, err := s.cache.SyncAll(); err != nil {
		s.logger.Warn("failed to sync cache after phase update", "error", err)
	}

	tmpl, err := s.globalDB.GetPhaseTemplate(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get updated phase template: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdatePhaseTemplateResponse{
		Template: dbPhaseTemplateToProto(tmpl),
	}), nil
}

// updateDBOnlyPhaseTemplate handles updates to templates created directly in DB (not from files).
func (s *workflowServer) updateDBOnlyPhaseTemplate(
	tmpl *db.PhaseTemplate,
	req *orcv1.UpdatePhaseTemplateRequest,
) (*connect.Response[orcv1.UpdatePhaseTemplateResponse], error) {
	if req.Name != nil {
		tmpl.Name = *req.Name
	}
	if req.Description != nil {
		tmpl.Description = *req.Description
	}
	if req.PromptSource != nil {
		tmpl.PromptSource = protoPromptSourceToString(*req.PromptSource)
	}
	if req.PromptContent != nil {
		tmpl.PromptContent = *req.PromptContent
	}
	if req.PromptPath != nil {
		tmpl.PromptPath = *req.PromptPath
	}
	if req.OutputSchema != nil {
		tmpl.OutputSchema = *req.OutputSchema
	}
	if req.ProducesArtifact != nil {
		tmpl.ProducesArtifact = *req.ProducesArtifact
	}
	if req.ArtifactType != nil {
		tmpl.ArtifactType = *req.ArtifactType
	}
	if req.OutputVarName != nil {
		tmpl.OutputVarName = *req.OutputVarName
	}
	if req.InputVariables != nil {
		if len(req.InputVariables) > 0 {
			data, _ := json.Marshal(req.InputVariables)
			tmpl.InputVariables = string(data)
		} else {
			tmpl.InputVariables = "[]"
		}
	}
	if req.ThinkingEnabled != nil {
		tmpl.ThinkingEnabled = req.ThinkingEnabled
	}
	if req.RuntimeConfig != nil {
		tmpl.RuntimeConfig = *req.RuntimeConfig
	}
	if req.GateType != nil {
		tmpl.GateType = protoGateTypeToString(*req.GateType)
	}
	if req.Checkpoint != nil {
		tmpl.Checkpoint = *req.Checkpoint
	}
	if req.Provider != nil {
		if err := validateProviderString(*req.Provider); err != nil {
			return nil, err
		}
		tmpl.Provider = *req.Provider
	}

	if err := s.globalDB.SavePhaseTemplate(tmpl); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save phase template: %w", err))
	}

	updated, err := s.globalDB.GetPhaseTemplate(tmpl.ID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get updated phase template: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdatePhaseTemplateResponse{
		Template: dbPhaseTemplateToProto(updated),
	}), nil
}

// DeletePhaseTemplate deletes a phase template.
func (s *workflowServer) DeletePhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.DeletePhaseTemplateRequest],
) (*connect.Response[orcv1.DeletePhaseTemplateResponse], error) {
	if req.Msg.Id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("id is required"))
	}

	tmpl, err := s.globalDB.GetPhaseTemplate(req.Msg.Id)
	if err != nil || tmpl == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase template %s not found", req.Msg.Id))
	}
	if tmpl.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot delete built-in phase template"))
	}

	workflows, err := s.globalDB.ListWorkflows()
	if err == nil {
		for _, wf := range workflows {
			phases, _ := s.globalDB.GetWorkflowPhases(wf.ID)
			for _, p := range phases {
				if p.PhaseTemplateID == req.Msg.Id {
					return nil, connect.NewError(connect.CodeFailedPrecondition,
						fmt.Errorf("phase template is used by workflow: %s", wf.ID))
				}
			}
		}
	}

	if err := s.globalDB.DeletePhaseTemplate(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete phase template: %w", err))
	}

	return connect.NewResponse(&orcv1.DeletePhaseTemplateResponse{
		Message: "deleted",
	}), nil
}

// ClonePhaseTemplate clones a phase template to a new ID.
func (s *workflowServer) ClonePhaseTemplate(
	ctx context.Context,
	req *connect.Request[orcv1.ClonePhaseTemplateRequest],
) (*connect.Response[orcv1.ClonePhaseTemplateResponse], error) {
	if req.Msg.SourceId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source_id is required"))
	}
	if req.Msg.NewId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("new_id is required"))
	}

	result, err := s.cloner.ClonePhase(req.Msg.SourceId, req.Msg.NewId, workflow.WriteLevelProject, false)
	if err != nil {
		if errors.Is(err, workflow.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("source phase template %s not found", req.Msg.SourceId))
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("clone phase template: %w", err))
	}

	if req.Msg.NewName != nil && *req.Msg.NewName != "" {
		resolved, err := s.resolver.ResolvePhase(req.Msg.NewId)
		if err == nil && resolved != nil {
			resolved.Phase.Name = *req.Msg.NewName
			writer := workflow.NewWriterFromOrcDir(s.resolver.OrcDir())
			if _, writeErr := writer.WritePhase(resolved.Phase, workflow.WriteLevelProject); writeErr != nil {
				s.logger.Warn("failed to update cloned phase template name", "error", writeErr)
			}
		}
	}

	if _, err := s.cache.SyncAll(); err != nil {
		s.logger.Warn("failed to sync cache after clone", "error", err)
	}

	clone, err := s.globalDB.GetPhaseTemplate(req.Msg.NewId)
	if err != nil {
		s.logger.Warn("failed to get cloned phase template from DB", "id", req.Msg.NewId, "error", err)
		return connect.NewResponse(&orcv1.ClonePhaseTemplateResponse{
			Template: &orcv1.PhaseTemplate{
				Id:   result.DestID,
				Name: result.DestID,
			},
		}), nil
	}

	return connect.NewResponse(&orcv1.ClonePhaseTemplateResponse{
		Template: dbPhaseTemplateToProto(clone),
	}), nil
}

// GetPromptContent returns the prompt content for a phase template.
func (s *workflowServer) GetPromptContent(
	ctx context.Context,
	req *connect.Request[orcv1.GetPromptContentRequest],
) (*connect.Response[orcv1.GetPromptContentResponse], error) {
	if req.Msg.PhaseTemplateId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("phase_template_id is required"))
	}

	tmpl, err := s.globalDB.GetPhaseTemplate(req.Msg.PhaseTemplateId)
	if err != nil || tmpl == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("phase template %s not found", req.Msg.PhaseTemplateId))
	}

	var content string
	switch tmpl.PromptSource {
	case "db":
		content = tmpl.PromptContent
	case "embedded":
		if tmpl.PromptPath != "" {
			data, err := templates.Prompts.ReadFile(tmpl.PromptPath)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("read embedded prompt %s: %w", tmpl.PromptPath, err))
			}
			content = string(data)
		}
	case "file":
		content = "<!-- File prompt at: " + tmpl.PromptPath + " -->"
	}

	resp := &orcv1.GetPromptContentResponse{
		Content: content,
		Source:  stringToProtoPromptSource(tmpl.PromptSource),
	}
	if tmpl.PromptPath != "" {
		resp.Path = &tmpl.PromptPath
	}

	return connect.NewResponse(resp), nil
}
