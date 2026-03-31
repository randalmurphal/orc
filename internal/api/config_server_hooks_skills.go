package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// ListHooks returns all hooks from GlobalDB hook_scripts table.
func (s *configServer) ListHooks(
	ctx context.Context,
	req *connect.Request[orcv1.ListHooksRequest],
) (*connect.Response[orcv1.ListHooksResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	scripts, err := s.globalDB.ListHookScripts()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list hook scripts: %w", err))
	}

	hooks := make([]*orcv1.Hook, len(scripts))
	for i, hs := range scripts {
		hooks[i] = hookScriptToProto(hs)
	}

	return connect.NewResponse(&orcv1.ListHooksResponse{
		Hooks: hooks,
	}), nil
}

// CreateHook creates a new hook in GlobalDB.
func (s *configServer) CreateHook(
	ctx context.Context,
	req *connect.Request[orcv1.CreateHookRequest],
) (*connect.Response[orcv1.CreateHookResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content is required"))
	}
	if req.Msg.EventType == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("event_type is required"))
	}

	existing, err := s.globalDB.ListHookScripts()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list hook scripts: %w", err))
	}
	for _, hs := range existing {
		if hs.Name == req.Msg.Name {
			return nil, connect.NewError(connect.CodeAlreadyExists,
				fmt.Errorf("hook with name %q already exists", req.Msg.Name))
		}
	}

	hs := &db.HookScript{
		ID:          fmt.Sprintf("hook-%d", time.Now().UnixNano()),
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		Content:     req.Msg.Content,
		EventType:   req.Msg.EventType,
		IsBuiltin:   false,
	}

	if err := s.globalDB.SaveHookScript(hs); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save hook script: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateHookResponse{
		Hook: hookScriptToProto(hs),
	}), nil
}

// UpdateHook updates an existing hook in GlobalDB.
func (s *configServer) UpdateHook(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateHookRequest],
) (*connect.Response[orcv1.UpdateHookResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	hs, err := s.globalDB.GetHookScript(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get hook script: %w", err))
	}
	if hs == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("hook %s not found", req.Msg.Id))
	}
	if hs.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in hook"))
	}

	if req.Msg.Name != nil {
		hs.Name = *req.Msg.Name
	}
	if req.Msg.Description != nil {
		hs.Description = *req.Msg.Description
	}
	if req.Msg.Content != nil {
		hs.Content = *req.Msg.Content
	}
	if req.Msg.EventType != nil {
		hs.EventType = *req.Msg.EventType
	}

	if err := s.globalDB.SaveHookScript(hs); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save hook script: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateHookResponse{
		Hook: hookScriptToProto(hs),
	}), nil
}

// DeleteHook deletes a hook from GlobalDB.
func (s *configServer) DeleteHook(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteHookRequest],
) (*connect.Response[orcv1.DeleteHookResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	hs, err := s.globalDB.GetHookScript(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get hook script: %w", err))
	}
	if hs == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("hook %s not found", req.Msg.Id))
	}
	if hs.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot delete built-in hook"))
	}

	if err := s.globalDB.DeleteHookScript(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete hook script: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteHookResponse{
		Message: "hook deleted",
	}), nil
}

// ListSkills returns all skills from GlobalDB skills table.
func (s *configServer) ListSkills(
	ctx context.Context,
	req *connect.Request[orcv1.ListSkillsRequest],
) (*connect.Response[orcv1.ListSkillsResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	dbSkills, err := s.globalDB.ListSkills()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list skills: %w", err))
	}

	protoSkills := make([]*orcv1.Skill, len(dbSkills))
	for i, sk := range dbSkills {
		protoSkills[i] = dbSkillToProto(sk)
	}

	return connect.NewResponse(&orcv1.ListSkillsResponse{
		Skills: protoSkills,
	}), nil
}

// CreateSkill creates a new skill in GlobalDB.
func (s *configServer) CreateSkill(
	ctx context.Context,
	req *connect.Request[orcv1.CreateSkillRequest],
) (*connect.Response[orcv1.CreateSkillResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}
	if req.Msg.Name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	if req.Msg.Content == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("content is required"))
	}

	existing, err := s.globalDB.ListSkills()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list skills: %w", err))
	}
	for _, sk := range existing {
		if sk.Name == req.Msg.Name {
			return nil, connect.NewError(connect.CodeAlreadyExists,
				fmt.Errorf("skill with name %q already exists", req.Msg.Name))
		}
	}

	sk := &db.Skill{
		ID:          fmt.Sprintf("skill-%d", time.Now().UnixNano()),
		Name:        req.Msg.Name,
		Description: req.Msg.Description,
		Content:     req.Msg.Content,
		IsBuiltin:   false,
	}

	if err := s.globalDB.SaveSkill(sk); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save skill: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateSkillResponse{
		Skill: dbSkillToProto(sk),
	}), nil
}

// UpdateSkill updates an existing skill in GlobalDB.
func (s *configServer) UpdateSkill(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateSkillRequest],
) (*connect.Response[orcv1.UpdateSkillResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	sk, err := s.globalDB.GetSkill(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get skill: %w", err))
	}
	if sk == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("skill %s not found", req.Msg.Id))
	}
	if sk.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot modify built-in skill"))
	}

	if req.Msg.Name != nil {
		sk.Name = *req.Msg.Name
	}
	if req.Msg.Description != nil {
		sk.Description = *req.Msg.Description
	}
	if req.Msg.Content != nil {
		sk.Content = *req.Msg.Content
	}

	if err := s.globalDB.SaveSkill(sk); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("save skill: %w", err))
	}

	return connect.NewResponse(&orcv1.UpdateSkillResponse{
		Skill: dbSkillToProto(sk),
	}), nil
}

// DeleteSkill deletes a skill from GlobalDB.
func (s *configServer) DeleteSkill(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteSkillRequest],
) (*connect.Response[orcv1.DeleteSkillResponse], error) {
	if s.globalDB == nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("globalDB not configured"))
	}

	sk, err := s.globalDB.GetSkill(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get skill: %w", err))
	}
	if sk == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("skill %s not found", req.Msg.Id))
	}
	if sk.IsBuiltin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("cannot delete built-in skill"))
	}

	if err := s.globalDB.DeleteSkill(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete skill: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteSkillResponse{
		Message: "skill deleted",
	}), nil
}
