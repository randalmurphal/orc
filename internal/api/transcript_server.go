// Package api provides the Connect RPC and REST API server for orc.
// This file implements the TranscriptService Connect RPC service.
package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
)

// timestampToTime converts unix milliseconds to time.Time.
func timestampToTime(ts int64) time.Time {
	return time.UnixMilli(ts)
}

// transcriptServer implements the TranscriptServiceHandler interface.
type transcriptServer struct {
	orcv1connect.UnimplementedTranscriptServiceHandler
	backend      storage.Backend
	projectCache *ProjectCache
}

// NewTranscriptServer creates a new TranscriptService handler.
func NewTranscriptServer(backend storage.Backend) orcv1connect.TranscriptServiceHandler {
	return &transcriptServer{
		backend: backend,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *transcriptServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the storage backend for the given project ID.
func (s *transcriptServer) getBackend(projectID string) (storage.Backend, error) {
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

// getProjectDB returns the underlying ProjectDB for transcript queries.
func (s *transcriptServer) getProjectDB(projectID string) (*db.ProjectDB, error) {
	backend, err := s.getBackend(projectID)
	if err != nil {
		return nil, err
	}
	if dbBackend, ok := backend.(*storage.DatabaseBackend); ok {
		return dbBackend.DB(), nil
	}
	return nil, fmt.Errorf("backend is not a DatabaseBackend")
}

// ListTranscripts returns transcript files for a task.
func (s *transcriptServer) ListTranscripts(
	ctx context.Context,
	req *connect.Request[orcv1.ListTranscriptsRequest],
) (*connect.Response[orcv1.ListTranscriptsResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	transcripts, err := backend.GetTranscripts(req.Msg.TaskId)
	if err != nil {
		return connect.NewResponse(&orcv1.ListTranscriptsResponse{
			Transcripts: []*orcv1.TranscriptFile{},
		}), nil
	}

	// Group transcripts by phase (no iteration in storage.Transcript)
	groups := make(map[string][]storage.Transcript)
	for _, t := range transcripts {
		// Filter by phase if specified
		if req.Msg.Phase != nil && t.Phase != *req.Msg.Phase {
			continue
		}
		groups[t.Phase] = append(groups[t.Phase], t)
	}

	// Convert to transcript files
	result := make([]*orcv1.TranscriptFile, 0, len(groups))
	for phase, group := range groups {
		if len(group) > 0 {
			result = append(result, &orcv1.TranscriptFile{
				Path:      "", // Path not stored in DB-backed transcripts
				Phase:     phase,
				Iteration: 1, // Default iteration since storage doesn't track it
				Size:      int64(len(group)), // Number of entries
				CreatedAt: timestamppb.New(timestampToTime(group[0].Timestamp)),
			})
		}
	}

	return connect.NewResponse(&orcv1.ListTranscriptsResponse{
		Transcripts: result,
	}), nil
}

// GetTranscript returns a specific transcript.
func (s *transcriptServer) GetTranscript(
	ctx context.Context,
	req *connect.Request[orcv1.GetTranscriptRequest],
) (*connect.Response[orcv1.GetTranscriptResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}
	if req.Msg.Phase == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("phase is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	transcripts, err := backend.GetTranscripts(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("transcript not found"))
	}

	// Filter to specific phase (iteration not tracked in storage)
	var entries []*orcv1.TranscriptEntry
	var totalInput, totalOutput, totalCacheRead, totalCacheCreation int32
	var sessionID, model string

	for _, t := range transcripts {
		if t.Phase != req.Msg.Phase {
			continue
		}

		entry := &orcv1.TranscriptEntry{
			Timestamp: timestamppb.New(timestampToTime(t.Timestamp)),
			Type:      t.Type,
			Content:   t.Content,
		}

		// Tool information is in Content as JSON
		entries = append(entries, entry)

		// Track token usage
		totalInput += int32(t.InputTokens)
		totalOutput += int32(t.OutputTokens)
		totalCacheRead += int32(t.CacheReadTokens)
		totalCacheCreation += int32(t.CacheCreationTokens)

		// Get session/model from first assistant message
		if t.SessionID != "" {
			sessionID = t.SessionID
		}
		if t.Model != "" {
			model = t.Model
		}
	}

	if len(entries) == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("transcript not found"))
	}

	transcript := &orcv1.Transcript{
		TaskId:    req.Msg.TaskId,
		Phase:     req.Msg.Phase,
		Iteration: req.Msg.Iteration,
		Entries:   entries,
		TotalTokens: &orcv1.TokenUsage{
			InputTokens:              totalInput,
			OutputTokens:             totalOutput,
			CacheReadInputTokens:     totalCacheRead,
			CacheCreationInputTokens: totalCacheCreation,
			TotalTokens:              totalInput + totalOutput,
		},
		StartedAt: entries[0].Timestamp,
	}
	if sessionID != "" {
		transcript.SessionId = &sessionID
	}
	if model != "" {
		transcript.Model = &model
	}
	if len(entries) > 0 {
		transcript.EndedAt = entries[len(entries)-1].Timestamp
	}

	return connect.NewResponse(&orcv1.GetTranscriptResponse{
		Transcript: transcript,
	}), nil
}

// GetTranscriptContent streams transcript content for large transcripts.
func (s *transcriptServer) GetTranscriptContent(
	ctx context.Context,
	req *connect.Request[orcv1.GetTranscriptContentRequest],
	stream *connect.ServerStream[orcv1.GetTranscriptContentResponse],
) error {
	if req.Msg.TaskId == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	transcripts, err := backend.GetTranscripts(req.Msg.TaskId)
	if err != nil {
		return connect.NewError(connect.CodeNotFound, errors.New("transcript not found"))
	}

	// Filter to specific phase, stream content
	chunkSize := 8192
	for i, t := range transcripts {
		if t.Phase != req.Msg.Phase {
			continue
		}

		content := []byte(t.Content)
		for j := 0; j < len(content); j += chunkSize {
			end := j + chunkSize
			if end > len(content) {
				end = len(content)
			}

			isLast := end == len(content) && i == len(transcripts)-1
			if err := stream.Send(&orcv1.GetTranscriptContentResponse{
				Chunk: &orcv1.TranscriptChunk{
					Data:   content[j:end],
					IsLast: isLast,
				},
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetTokens returns token usage for a task.
func (s *transcriptServer) GetTokens(
	ctx context.Context,
	req *connect.Request[orcv1.GetTokensRequest],
) (*connect.Response[orcv1.GetTokensResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	pdb, err := s.getProjectDB(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	usage, err := pdb.GetTaskTokenUsage(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&orcv1.GetTokensResponse{
		Tokens: &orcv1.TokenUsage{
			InputTokens:              int32(usage.TotalInput),
			OutputTokens:             int32(usage.TotalOutput),
			CacheReadInputTokens:     int32(usage.TotalCacheRead),
			CacheCreationInputTokens: int32(usage.TotalCacheCreation),
			TotalTokens:              int32(usage.TotalInput + usage.TotalOutput),
		},
	}), nil
}

// GetSession returns session information for a task.
func (s *transcriptServer) GetSession(
	ctx context.Context,
	req *connect.Request[orcv1.GetSessionRequest],
) (*connect.Response[orcv1.GetSessionResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Load task to get session info
	t, err := backend.LoadTask(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("task not found"))
	}

	return connect.NewResponse(&orcv1.GetSessionResponse{
		Session: &orcv1.SessionInfo{
			Id:     req.Msg.TaskId,
			Model:  "", // Would need transcript scan to get model
			Status: t.Status.String(),
		},
	}), nil
}

// GetTodos returns the current todo list for a task.
func (s *transcriptServer) GetTodos(
	ctx context.Context,
	req *connect.Request[orcv1.GetTodosRequest],
) (*connect.Response[orcv1.GetTodosResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	pdb, err := s.getProjectDB(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	snapshot, err := pdb.GetLatestTodos(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if snapshot == nil {
		return connect.NewResponse(&orcv1.GetTodosResponse{
			Items: []*orcv1.TodoItem{},
		}), nil
	}

	items := make([]*orcv1.TodoItem, len(snapshot.Items))
	for i, item := range snapshot.Items {
		items[i] = &orcv1.TodoItem{
			Content: item.Content,
			Status:  item.Status,
		}
		if item.ActiveForm != "" {
			items[i].ActiveForm = &item.ActiveForm
		}
	}

	resp := &orcv1.GetTodosResponse{
		Items: items,
	}
	if snapshot.Phase != "" {
		resp.Phase = &snapshot.Phase
	}

	return connect.NewResponse(resp), nil
}

// GetTodoHistory returns all todo snapshots for a task.
func (s *transcriptServer) GetTodoHistory(
	ctx context.Context,
	req *connect.Request[orcv1.GetTodoHistoryRequest],
) (*connect.Response[orcv1.GetTodoHistoryResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	pdb, err := s.getProjectDB(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	history, err := pdb.GetTodoHistory(req.Msg.TaskId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if history == nil {
		return connect.NewResponse(&orcv1.GetTodoHistoryResponse{
			Snapshots: []*orcv1.TodoSnapshot{},
		}), nil
	}

	snapshots := make([]*orcv1.TodoSnapshot, len(history))
	for i, snap := range history {
		items := make([]*orcv1.TodoItem, len(snap.Items))
		for j, item := range snap.Items {
			items[j] = &orcv1.TodoItem{
				Content: item.Content,
				Status:  item.Status,
			}
			if item.ActiveForm != "" {
				items[j].ActiveForm = &item.ActiveForm
			}
		}

		snapshots[i] = &orcv1.TodoSnapshot{
			Timestamp: timestamppb.New(snap.Timestamp),
			Phase:     snap.Phase,
			Items:     items,
		}
	}

	return connect.NewResponse(&orcv1.GetTodoHistoryResponse{
		Snapshots: snapshots,
	}), nil
}
