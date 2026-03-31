package api

import (
	"context"
	"fmt"
	"path/filepath"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ExportTask exports task artifacts to the filesystem or a git branch.
func (s *taskServer) ExportTask(
	ctx context.Context,
	req *connect.Request[orcv1.ExportTaskRequest],
) (*connect.Response[orcv1.ExportTaskResponse], error) {
	taskID := req.Msg.TaskId
	if taskID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("task ID required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	exists, err := backend.TaskExists(taskID)
	if err != nil || !exists {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("task not found: %s", taskID))
	}

	cfg, err := config.Load()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load config: %w", err))
	}

	resolved := cfg.Storage.ResolveExportConfig()
	opts := &storage.ExportOptions{
		TaskDefinition: resolved.TaskDefinition,
		FinalState:     resolved.FinalState,
		Transcripts:    resolved.Transcripts,
		ContextSummary: resolved.ContextSummary,
	}

	if req.Msg.TaskDefinition != nil {
		opts.TaskDefinition = *req.Msg.TaskDefinition
	}
	if req.Msg.FinalState != nil {
		opts.FinalState = *req.Msg.FinalState
	}
	if req.Msg.Transcripts != nil {
		opts.Transcripts = *req.Msg.Transcripts
	}
	if req.Msg.ContextSummary != nil {
		opts.ContextSummary = *req.Msg.ContextSummary
	}

	exportBackend, err := storage.NewBackend(s.projectRoot, &cfg.Storage)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to create storage backend: %w", err))
	}
	defer func() { _ = exportBackend.Close() }()

	exportSvc := storage.NewExportService(exportBackend, &cfg.Storage)

	if req.Msg.ToBranch {
		t, err := backend.LoadTask(taskID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to load task: %w", err))
		}

		if err := exportSvc.ExportToBranch(taskID, t.GetBranch(), opts); err != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to export to branch: %w", err))
		}

		return connect.NewResponse(&orcv1.ExportTaskResponse{
			Success:    true,
			TaskId:     taskID,
			ExportedTo: t.GetBranch(),
		}), nil
	}

	if err := exportSvc.Export(taskID, opts); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("failed to export: %w", err))
	}

	return connect.NewResponse(&orcv1.ExportTaskResponse{
		Success:    true,
		TaskId:     taskID,
		ExportedTo: filepath.Join(task.ExportPath(s.projectRoot), taskID),
	}), nil
}
