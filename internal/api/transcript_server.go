// Package api provides the Connect RPC and REST API server for orc.
// This file implements the TranscriptService Connect RPC service.
package api

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
)

// TranscriptStreamEvent represents a real-time transcript chunk.
type TranscriptStreamEvent struct {
	TaskID    string
	ProjectID string
	Content   string
	Type      string // "prompt", "response", "tool", "error"
	Phase     string
	Timestamp time.Time
	Tokens    *TokenCount
}

// TokenCount tracks token usage for a transcript chunk.
type TokenCount struct {
	Input  int32
	Output int32
}

// timestampToTime converts unix milliseconds to time.Time.
func timestampToTime(ts int64) time.Time {
	return time.UnixMilli(ts)
}

// transcriptServer implements the TranscriptServiceHandler interface.
type transcriptServer struct {
	orcv1connect.UnimplementedTranscriptServiceHandler
	backend        storage.Backend
	projectCache   *ProjectCache
	eventPublisher events.Publisher
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
				Iteration: 1,                 // Default iteration since storage doesn't track it
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
			TotalTokens:              totalInput + totalOutput + totalCacheRead + totalCacheCreation,
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

			_ = end == len(content) && i == len(transcripts)-1 // isLast not used in new TranscriptChunk
			if err := stream.Send(&orcv1.GetTranscriptContentResponse{
				Chunk: &orcv1.TranscriptChunk{
					TaskId:    req.Msg.TaskId,
					Type:      "content",
					Content:   string(content[j:end]),
					Phase:     req.Msg.Phase,
					Timestamp: timestamppb.New(timestampToTime(t.Timestamp)),
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
			TotalTokens:              int32(usage.TotalInput + usage.TotalOutput + usage.TotalCacheRead + usage.TotalCacheCreation),
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

	currentPhase := task.GetCurrentPhaseProto(t)
	session := &orcv1.SessionInfo{
		Id:     task.GetPhaseSessionIDProto(t, currentPhase),
		Model:  task.GetPhaseModelProto(t, currentPhase),
		Status: t.Status.String(),
	}
	if session.Id == "" {
		session.Id = req.Msg.TaskId
	}

	if currentPhase != "" && t.Execution != nil && t.Execution.Phases != nil {
		if phaseState := t.Execution.Phases[currentPhase]; phaseState != nil {
			if phaseState.StartedAt != nil {
				session.CreatedAt = phaseState.StartedAt
			}
			if phaseState.CompletedAt != nil {
				session.LastActivity = phaseState.CompletedAt
			}
		}
	}

	transcripts, err := backend.GetTranscripts(req.Msg.TaskId)
	if err == nil {
		assistantCount := int32(0)
		var lastActivity time.Time
		for _, transcript := range transcripts {
			if currentPhase != "" && transcript.Phase != currentPhase {
				continue
			}
			if transcript.Type == "assistant" {
				assistantCount++
			}
			ts := timestampToTime(transcript.Timestamp)
			if ts.After(lastActivity) {
				lastActivity = ts
			}
		}
		session.TurnCount = assistantCount
		if !lastActivity.IsZero() {
			session.LastActivity = timestamppb.New(lastActivity)
		}
	}

	return connect.NewResponse(&orcv1.GetSessionResponse{Session: session}), nil
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

// SetEventPublisher sets the event publisher for real-time transcript streaming.
func (s *transcriptServer) SetEventPublisher(publisher events.Publisher) {
	s.eventPublisher = publisher
}

// StreamTranscript streams real-time transcript chunks for a task.
func (s *transcriptServer) StreamTranscript(
	ctx context.Context,
	req *connect.Request[orcv1.StreamTranscriptRequest],
	stream *connect.ServerStream[orcv1.StreamTranscriptResponse],
) error {
	if req.Msg.TaskId == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	lastSeenID := int64(0)
	if existing, err := backend.GetTranscripts(req.Msg.TaskId); err == nil {
		for _, transcript := range existing {
			if transcript.ID > lastSeenID {
				lastSeenID = transcript.ID
			}
		}
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			transcripts, err := backend.GetTranscripts(req.Msg.TaskId)
			if err != nil {
				return connect.NewError(connect.CodeInternal, fmt.Errorf("get transcripts: %w", err))
			}
			sort.Slice(transcripts, func(i, j int) bool { return transcripts[i].ID < transcripts[j].ID })
			for _, transcript := range transcripts {
				if transcript.ID <= lastSeenID {
					continue
				}
				if req.Msg.Phase != nil && transcript.Phase != *req.Msg.Phase {
					continue
				}
				if err := stream.Send(&orcv1.StreamTranscriptResponse{
					Chunk: transcriptToChunk(transcript),
				}); err != nil {
					return err
				}
				lastSeenID = transcript.ID
			}
		}
	}
}

// GetLiveTranscript returns both persisted and live transcript content.
func (s *transcriptServer) GetLiveTranscript(
	ctx context.Context,
	req *connect.Request[orcv1.GetLiveTranscriptRequest],
) (*connect.Response[orcv1.GetLiveTranscriptResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	backend, err := s.getBackend(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Get persisted transcript entries
	transcripts, err := backend.GetTranscripts(req.Msg.TaskId)
	if err != nil {
		transcripts = []storage.Transcript{} // Start with empty if error
	}

	// Filter to specific phase if requested
	var entries []*orcv1.TranscriptEntry
	var totalInput, totalOutput, totalCacheRead, totalCacheCreation int32

	for _, t := range transcripts {
		if req.Msg.Phase != nil && t.Phase != *req.Msg.Phase {
			continue
		}

		entry := &orcv1.TranscriptEntry{
			Timestamp: timestamppb.New(timestampToTime(t.Timestamp)),
			Type:      t.Type,
			Content:   t.Content,
		}
		entries = append(entries, entry)

		totalInput += int32(t.InputTokens)
		totalOutput += int32(t.OutputTokens)
		totalCacheRead += int32(t.CacheReadTokens)
		totalCacheCreation += int32(t.CacheCreationTokens)
	}

	// Determine phase from the transcript data or request
	phase := req.Msg.GetPhase()
	if phase == "" && len(transcripts) > 0 {
		phase = transcripts[0].Phase // Use phase from first transcript entry
	}

	// Build the response
	transcript := &orcv1.Transcript{
		TaskId:  req.Msg.TaskId,
		Phase:   phase,
		Entries: entries,
		TotalTokens: &orcv1.TokenUsage{
			InputTokens:              totalInput,
			OutputTokens:             totalOutput,
			CacheReadInputTokens:     totalCacheRead,
			CacheCreationInputTokens: totalCacheCreation,
			TotalTokens:              totalInput + totalOutput + totalCacheRead + totalCacheCreation,
		},
		StartedAt: timestamppb.New(time.Now()),
	}

	return connect.NewResponse(&orcv1.GetLiveTranscriptResponse{
		Transcript:     transcript,
		HasLiveContent: false,
	}), nil
}

// StoreTranscriptEntry stores a transcript entry and publishes a real-time event.
func (s *transcriptServer) StoreTranscriptEntry(
	ctx context.Context,
	projectID string,
	transcript storage.Transcript,
) error {
	// Store the transcript entry (this would typically be handled by the backend)
	// For now, we'll just publish the event

	// Publish real-time event if publisher is configured
	if s.eventPublisher != nil {
		// Create transcript line for event data
		transcriptLine := events.TranscriptLine{
			Phase:     transcript.Phase,
			Iteration: 1, // Default iteration
			Type:      transcript.Type,
			Content:   transcript.Content,
			Timestamp: time.UnixMilli(transcript.Timestamp),
		}

		// Create event using the events.Event type
		event := events.NewEvent(events.EventTranscript, transcript.TaskID, transcriptLine)

		// Publish the event
		s.eventPublisher.Publish(event)
	}

	return nil
}

func transcriptToChunk(transcript storage.Transcript) *orcv1.TranscriptChunk {
	chunk := &orcv1.TranscriptChunk{
		TaskId:    transcript.TaskID,
		Type:      transcript.Type,
		Content:   transcript.Content,
		Phase:     transcript.Phase,
		Timestamp: timestamppb.New(timestampToTime(transcript.Timestamp)),
	}
	if transcript.InputTokens > 0 || transcript.OutputTokens > 0 || transcript.CacheCreationTokens > 0 || transcript.CacheReadTokens > 0 {
		chunk.Tokens = &orcv1.TokenUsage{
			InputTokens:              int32(transcript.InputTokens),
			OutputTokens:             int32(transcript.OutputTokens),
			CacheCreationInputTokens: int32(transcript.CacheCreationTokens),
			CacheReadInputTokens:     int32(transcript.CacheReadTokens),
			TotalTokens:              int32(transcript.InputTokens + transcript.OutputTokens + transcript.CacheCreationTokens + transcript.CacheReadTokens),
		}
	}
	return chunk
}
