// Package api provides the Connect RPC and REST API server for orc.
// This file implements the ProjectService and BranchService Connect RPC services.
package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/project"
	"github.com/randalmurphal/orc/internal/storage"
)

// projectServer implements the ProjectServiceHandler interface.
type projectServer struct {
	orcv1connect.UnimplementedProjectServiceHandler
	backend storage.Backend
	logger  *slog.Logger
}

// NewProjectServer creates a new ProjectService handler.
func NewProjectServer(
	backend storage.Backend,
	logger *slog.Logger,
) orcv1connect.ProjectServiceHandler {
	return &projectServer{
		backend: backend,
		logger:  logger,
	}
}

// ListProjects returns all registered projects.
func (s *projectServer) ListProjects(
	ctx context.Context,
	req *connect.Request[orcv1.ListProjectsRequest],
) (*connect.Response[orcv1.ListProjectsResponse], error) {
	projects, err := project.ListProjects()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list projects: %w", err))
	}

	// Get default project ID
	defaultID, _ := project.GetDefaultProject()

	protoProjects := make([]*orcv1.Project, len(projects))
	for i, p := range projects {
		protoProjects[i] = projectToProto(&p, p.ID == defaultID)
	}

	return connect.NewResponse(&orcv1.ListProjectsResponse{
		Projects: protoProjects,
	}), nil
}

// GetProject returns a specific project by ID.
func (s *projectServer) GetProject(
	ctx context.Context,
	req *connect.Request[orcv1.GetProjectRequest],
) (*connect.Response[orcv1.GetProjectResponse], error) {
	reg, err := project.LoadRegistry()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load registry: %w", err))
	}

	proj, err := reg.Get(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("project not found: %s", req.Msg.Id))
	}

	// Check if this is the default project
	defaultID, _ := project.GetDefaultProject()
	isDefault := proj.ID == defaultID

	return connect.NewResponse(&orcv1.GetProjectResponse{
		Project: projectToProto(proj, isDefault),
	}), nil
}

// GetDefaultProject returns the default project.
func (s *projectServer) GetDefaultProject(
	ctx context.Context,
	req *connect.Request[orcv1.GetDefaultProjectRequest],
) (*connect.Response[orcv1.GetDefaultProjectResponse], error) {
	defaultID, err := project.GetDefaultProject()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get default project: %w", err))
	}

	resp := &orcv1.GetDefaultProjectResponse{}
	if defaultID != "" {
		resp.DefaultProjectId = &defaultID

		// Also load the full project
		reg, err := project.LoadRegistry()
		if err == nil {
			if proj, err := reg.Get(defaultID); err == nil {
				resp.Project = projectToProto(proj, true)
			}
		}
	}

	return connect.NewResponse(resp), nil
}

// SetDefaultProject sets the default project.
func (s *projectServer) SetDefaultProject(
	ctx context.Context,
	req *connect.Request[orcv1.SetDefaultProjectRequest],
) (*connect.Response[orcv1.SetDefaultProjectResponse], error) {
	if err := project.SetDefaultProject(req.Msg.ProjectId); err != nil {
		if err.Error() == fmt.Sprintf("project not found: %s", req.Msg.ProjectId) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to set default project: %w", err))
	}

	// Load the project for response
	reg, err := project.LoadRegistry()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load registry: %w", err))
	}

	proj, err := reg.Get(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("project not found: %s", req.Msg.ProjectId))
	}

	return connect.NewResponse(&orcv1.SetDefaultProjectResponse{
		Project: projectToProto(proj, true),
	}), nil
}

// AddProject registers a new project.
func (s *projectServer) AddProject(
	ctx context.Context,
	req *connect.Request[orcv1.AddProjectRequest],
) (*connect.Response[orcv1.AddProjectResponse], error) {
	proj, err := project.RegisterProject(req.Msg.Path)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("failed to register project: %w", err))
	}

	// Optionally update name if provided
	if req.Msg.Name != "" && req.Msg.Name != proj.Name {
		reg, err := project.LoadRegistry()
		if err == nil {
			for i := range reg.Projects {
				if reg.Projects[i].ID == proj.ID {
					reg.Projects[i].Name = req.Msg.Name
					proj.Name = req.Msg.Name
					_ = reg.Save()
					break
				}
			}
		}
	}

	// Check if it became the default (first project)
	defaultID, _ := project.GetDefaultProject()
	isDefault := proj.ID == defaultID

	return connect.NewResponse(&orcv1.AddProjectResponse{
		Project: projectToProto(proj, isDefault),
	}), nil
}

// RemoveProject unregisters a project.
func (s *projectServer) RemoveProject(
	ctx context.Context,
	req *connect.Request[orcv1.RemoveProjectRequest],
) (*connect.Response[orcv1.RemoveProjectResponse], error) {
	reg, err := project.LoadRegistry()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load registry: %w", err))
	}

	if err := reg.Unregister(req.Msg.Id); err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	if err := reg.Save(); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to save registry: %w", err))
	}

	return connect.NewResponse(&orcv1.RemoveProjectResponse{
		Message: fmt.Sprintf("Project %s removed", req.Msg.Id),
	}), nil
}

// projectToProto converts a project.Project to proto.
func projectToProto(p *project.Project, isDefault bool) *orcv1.Project {
	return &orcv1.Project{
		Id:        p.ID,
		Name:      p.Name,
		Path:      p.Path,
		CreatedAt: timestamppb.New(p.CreatedAt),
		IsDefault: isDefault,
	}
}

// branchServer implements the BranchServiceHandler interface.
type branchServer struct {
	orcv1connect.UnimplementedBranchServiceHandler
	backend      storage.Backend
	logger       *slog.Logger
	projectCache *ProjectCache
}

// SetProjectCache sets the project cache for multi-project support.
func (s *branchServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
func (s *branchServer) getBackend(projectID string) (storage.Backend, error) {
	if projectID != "" && s.projectCache != nil {
		return s.projectCache.GetBackend(projectID)
	}
	if projectID != "" && s.projectCache == nil {
		return nil, fmt.Errorf("project_id specified but no project cache configured")
	}
	if s.backend == nil {
		return nil, fmt.Errorf("no backend available")
	}
	return s.backend, nil
}

// NewBranchServer creates a new BranchService handler.
func NewBranchServer(
	backend storage.Backend,
	logger *slog.Logger,
) orcv1connect.BranchServiceHandler {
	return &branchServer{
		backend: backend,
		logger:  logger,
	}
}

// ListBranches returns all branches with optional filtering.
func (s *branchServer) ListBranches(
	ctx context.Context,
	req *connect.Request[orcv1.ListBranchesRequest],
) (*connect.Response[orcv1.ListBranchesResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	dbBackend, ok := backend.(*storage.DatabaseBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("branches require database backend"))
	}

	opts := db.BranchListOpts{}
	if req.Msg.Type != nil && *req.Msg.Type != orcv1.BranchType_BRANCH_TYPE_UNSPECIFIED {
		opts.Type = protoBranchTypeToDB(*req.Msg.Type)
	}
	if req.Msg.Status != nil && *req.Msg.Status != orcv1.BranchStatus_BRANCH_STATUS_UNSPECIFIED {
		opts.Status = protoBranchStatusToDB(*req.Msg.Status)
	}

	branches, err := dbBackend.DB().ListBranches(opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to list branches: %w", err))
	}

	// Filter orphaned if not requested
	var filtered []*db.Branch
	for _, b := range branches {
		if !req.Msg.IncludeOrphaned && b.Status == db.BranchStatusOrphaned {
			continue
		}
		filtered = append(filtered, b)
	}

	protoBranches := make([]*orcv1.Branch, len(filtered))
	for i, b := range filtered {
		protoBranches[i] = dbBranchToProto(b)
	}

	return connect.NewResponse(&orcv1.ListBranchesResponse{
		Branches: protoBranches,
	}), nil
}

// GetBranch returns a specific branch by name.
func (s *branchServer) GetBranch(
	ctx context.Context,
	req *connect.Request[orcv1.GetBranchRequest],
) (*connect.Response[orcv1.GetBranchResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	dbBackend, ok := backend.(*storage.DatabaseBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("branches require database backend"))
	}

	branch, err := dbBackend.DB().GetBranch(req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get branch: %w", err))
	}
	if branch == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("branch not found: %s", req.Msg.Name))
	}

	return connect.NewResponse(&orcv1.GetBranchResponse{
		Branch: dbBranchToProto(branch),
	}), nil
}

// UpdateBranchStatus updates a branch's status.
func (s *branchServer) UpdateBranchStatus(
	ctx context.Context,
	req *connect.Request[orcv1.UpdateBranchStatusRequest],
) (*connect.Response[orcv1.UpdateBranchStatusResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	dbBackend, ok := backend.(*storage.DatabaseBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("branches require database backend"))
	}

	// Check if branch exists
	branch, err := dbBackend.DB().GetBranch(req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get branch: %w", err))
	}
	if branch == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("branch not found: %s", req.Msg.Name))
	}

	// Update status
	newStatus := protoBranchStatusToDB(req.Msg.Status)
	if err := dbBackend.DB().UpdateBranchStatus(req.Msg.Name, newStatus); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to update branch status: %w", err))
	}

	// Reload to return updated branch
	branch, _ = dbBackend.DB().GetBranch(req.Msg.Name)

	return connect.NewResponse(&orcv1.UpdateBranchStatusResponse{
		Branch: dbBranchToProto(branch),
	}), nil
}

// DeleteBranch removes a branch from the registry.
func (s *branchServer) DeleteBranch(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteBranchRequest],
) (*connect.Response[orcv1.DeleteBranchResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	dbBackend, ok := backend.(*storage.DatabaseBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("branches require database backend"))
	}

	// Check if branch exists
	branch, err := dbBackend.DB().GetBranch(req.Msg.Name)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get branch: %w", err))
	}
	if branch == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("branch not found: %s", req.Msg.Name))
	}

	// Check if force delete is required
	if branch.Status == db.BranchStatusActive && !req.Msg.Force {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("cannot delete active branch without force flag"))
	}

	if err := dbBackend.DB().DeleteBranch(req.Msg.Name); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to delete branch: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteBranchResponse{
		Message: fmt.Sprintf("Branch %s deleted", req.Msg.Name),
	}), nil
}

// CleanupStaleBranches removes branches that haven't had activity recently.
func (s *branchServer) CleanupStaleBranches(
	ctx context.Context,
	req *connect.Request[orcv1.CleanupStaleBranchesRequest],
) (*connect.Response[orcv1.CleanupStaleBranchesResponse], error) {
	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	dbBackend, ok := backend.(*storage.DatabaseBackend)
	if !ok {
		return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("branches require database backend"))
	}

	staleDays := int(req.Msg.StaleDays)
	if staleDays <= 0 {
		staleDays = 30 // Default 30 days
	}

	cutoff := time.Now().AddDate(0, 0, -staleDays)
	staleBranches, err := dbBackend.DB().GetStaleBranches(cutoff)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to get stale branches: %w", err))
	}

	var deleted, skipped []string
	for _, branch := range staleBranches {
		if req.Msg.DryRun {
			deleted = append(deleted, branch.Name) // Would be deleted
		} else {
			if err := dbBackend.DB().DeleteBranch(branch.Name); err != nil {
				s.logger.Warn("failed to delete stale branch", "name", branch.Name, "error", err)
				skipped = append(skipped, branch.Name)
			} else {
				deleted = append(deleted, branch.Name)
			}
		}
	}

	return connect.NewResponse(&orcv1.CleanupStaleBranchesResponse{
		DeletedBranches: deleted,
		SkippedBranches: skipped,
	}), nil
}

// dbBranchToProto converts a db.Branch to proto.
func dbBranchToProto(b *db.Branch) *orcv1.Branch {
	result := &orcv1.Branch{
		Name:      b.Name,
		Type:      dbBranchTypeToProto(b.Type),
		CreatedAt: timestamppb.New(b.CreatedAt),
		Status:    dbBranchStatusToProto(b.Status),
	}
	if !b.LastActivity.IsZero() {
		result.LastActivity = timestamppb.New(b.LastActivity)
	}
	if b.OwnerID != "" {
		result.OwnerId = &b.OwnerID
	}
	if b.BaseBranch != "" {
		result.TargetBranch = &b.BaseBranch
	}
	// Note: commits_ahead and commits_behind would require git operations
	// to compute, so they're left at 0 for now
	return result
}

// dbBranchTypeToProto converts db.BranchType to proto enum.
func dbBranchTypeToProto(t db.BranchType) orcv1.BranchType {
	switch t {
	case db.BranchTypeInitiative:
		return orcv1.BranchType_BRANCH_TYPE_INITIATIVE
	case db.BranchTypeStaging:
		return orcv1.BranchType_BRANCH_TYPE_STAGING
	case db.BranchTypeTask:
		return orcv1.BranchType_BRANCH_TYPE_TASK
	default:
		return orcv1.BranchType_BRANCH_TYPE_UNSPECIFIED
	}
}

// protoBranchTypeToDB converts proto enum to db.BranchType.
func protoBranchTypeToDB(t orcv1.BranchType) db.BranchType {
	switch t {
	case orcv1.BranchType_BRANCH_TYPE_INITIATIVE:
		return db.BranchTypeInitiative
	case orcv1.BranchType_BRANCH_TYPE_STAGING:
		return db.BranchTypeStaging
	case orcv1.BranchType_BRANCH_TYPE_TASK:
		return db.BranchTypeTask
	default:
		return ""
	}
}

// dbBranchStatusToProto converts db.BranchStatus to proto enum.
func dbBranchStatusToProto(s db.BranchStatus) orcv1.BranchStatus {
	switch s {
	case db.BranchStatusActive:
		return orcv1.BranchStatus_BRANCH_STATUS_ACTIVE
	case db.BranchStatusMerged:
		return orcv1.BranchStatus_BRANCH_STATUS_MERGED
	case db.BranchStatusStale:
		return orcv1.BranchStatus_BRANCH_STATUS_STALE
	case db.BranchStatusOrphaned:
		return orcv1.BranchStatus_BRANCH_STATUS_ORPHANED
	default:
		return orcv1.BranchStatus_BRANCH_STATUS_UNSPECIFIED
	}
}

// protoBranchStatusToDB converts proto enum to db.BranchStatus.
func protoBranchStatusToDB(s orcv1.BranchStatus) db.BranchStatus {
	switch s {
	case orcv1.BranchStatus_BRANCH_STATUS_ACTIVE:
		return db.BranchStatusActive
	case orcv1.BranchStatus_BRANCH_STATUS_MERGED:
		return db.BranchStatusMerged
	case orcv1.BranchStatus_BRANCH_STATUS_STALE:
		return db.BranchStatusStale
	case orcv1.BranchStatus_BRANCH_STATUS_ORPHANED:
		return db.BranchStatusOrphaned
	default:
		return ""
	}
}
