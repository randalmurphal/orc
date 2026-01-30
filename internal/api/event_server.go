// Package api provides the Connect RPC and REST API server for orc.
// This file implements the EventService Connect RPC service, replacing WebSocket.
package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
)

const (
	// GlobalTaskID is used to subscribe to all task events.
	globalTaskID = "*"
	// heartbeatInterval is how often to send heartbeats.
	heartbeatInterval = 30 * time.Second
)

// eventServer implements the EventServiceHandler interface.
type eventServer struct {
	orcv1connect.UnimplementedEventServiceHandler
	publisher    events.Publisher
	backend      storage.Backend
	projectCache *ProjectCache
	logger       *slog.Logger
}

// NewEventServer creates a new EventService handler.
func NewEventServer(
	publisher events.Publisher,
	backend storage.Backend,
	logger *slog.Logger,
) orcv1connect.EventServiceHandler {
	return &eventServer{
		publisher: publisher,
		backend:   backend,
		logger:    logger,
	}
}

// SetProjectCache sets the project cache for multi-project support.
func (s *eventServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the storage backend for the given project ID.
func (s *eventServer) getBackend(projectID string) (storage.Backend, error) {
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

// getProjectDB returns the underlying ProjectDB for event queries.
func (s *eventServer) getProjectDB(projectID string) (*db.ProjectDB, error) {
	backend, err := s.getBackend(projectID)
	if err != nil {
		return nil, err
	}
	if dbBackend, ok := backend.(*storage.DatabaseBackend); ok {
		return dbBackend.DB(), nil
	}
	return nil, fmt.Errorf("backend is not a DatabaseBackend")
}

// Subscribe streams real-time events to the client, replacing WebSocket.
func (s *eventServer) Subscribe(
	ctx context.Context,
	req *connect.Request[orcv1.SubscribeRequest],
	stream *connect.ServerStream[orcv1.SubscribeResponse],
) error {
	// Determine subscription scope
	taskID := globalTaskID
	if req.Msg.TaskId != nil && *req.Msg.TaskId != "" {
		taskID = *req.Msg.TaskId
	}

	// Get backend for initiative filtering (use first project_id if specified)
	filterBackend := s.backend
	if len(req.Msg.ProjectIds) > 0 && s.projectCache != nil {
		if b, err := s.projectCache.GetBackend(req.Msg.ProjectIds[0]); err == nil {
			filterBackend = b
		}
	}

	// Subscribe to event channel
	eventChan := s.publisher.Subscribe(taskID)
	defer s.publisher.Unsubscribe(taskID, eventChan)

	// Build event type filter
	eventTypes := make(map[string]bool)
	for _, t := range req.Msg.EventTypes {
		eventTypes[t] = true
	}

	// Optional heartbeat ticker
	var heartbeat <-chan time.Time
	if req.Msg.IncludeHeartbeat {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		heartbeat = ticker.C
	}

	s.logger.Debug("client subscribed to events", "task_id", taskID, "event_types", req.Msg.EventTypes)

	for {
		select {
		case <-ctx.Done():
			s.logger.Debug("client disconnected", "task_id", taskID)
			return nil

		case event, ok := <-eventChan:
			if !ok {
				return nil
			}

			// Filter by event type if specified
			if len(eventTypes) > 0 && !eventTypes[string(event.Type)] {
				continue
			}

			// Filter by initiative if specified
			initFilter := ""
			if req.Msg.InitiativeId != nil {
				initFilter = *req.Msg.InitiativeId
			}
			if filterEventByInitiative(event, initFilter, filterBackend) {
				continue
			}

			// Convert to proto event
			protoEvent := internalEventToProto(event)
			if protoEvent == nil {
				continue
			}

			if err := stream.Send(&orcv1.SubscribeResponse{Event: protoEvent}); err != nil {
				s.logger.Debug("failed to send event", "error", err)
				return err
			}

		case <-heartbeat:
			hb := &orcv1.Event{
				Id:        uuid.New().String(),
				Timestamp: timestamppb.Now(),
				Payload: &orcv1.Event_Heartbeat{
					Heartbeat: &orcv1.HeartbeatEvent{
						Timestamp: timestamppb.Now(),
					},
				},
			}
			if err := stream.Send(&orcv1.SubscribeResponse{Event: hb}); err != nil {
				return err
			}
		}
	}
}

// GetEvents returns historical events with pagination.
func (s *eventServer) GetEvents(
	ctx context.Context,
	req *connect.Request[orcv1.GetEventsRequest],
) (*connect.Response[orcv1.GetEventsResponse], error) {
	pdb, err := s.getProjectDB(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Build query options
	opts := db.QueryEventsOptions{}
	if req.Msg.TaskId != nil {
		opts.TaskID = *req.Msg.TaskId
	}
	if req.Msg.InitiativeId != nil {
		opts.InitiativeID = *req.Msg.InitiativeId
	}
	if req.Msg.Since != nil {
		t := req.Msg.Since.AsTime()
		opts.Since = &t
	}
	if req.Msg.Until != nil {
		t := req.Msg.Until.AsTime()
		opts.Until = &t
	}
	opts.EventTypes = req.Msg.Types

	// Pagination
	if req.Msg.Page != nil {
		opts.Offset = int(req.Msg.Page.Page * req.Msg.Page.Limit)
		opts.Limit = int(req.Msg.Page.Limit)
	}

	dbEvents, err := pdb.QueryEvents(opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get total count for pagination
	total, _ := pdb.CountEvents(opts)

	// Convert to proto events
	protoEvents := make([]*orcv1.Event, 0, len(dbEvents))
	for _, e := range dbEvents {
		pe := dbEventToProto(&e)
		if pe != nil {
			protoEvents = append(protoEvents, pe)
		}
	}

	// Build pagination response
	limit := int32(50)
	page := int32(0)
	if req.Msg.Page != nil {
		limit = req.Msg.Page.Limit
		page = req.Msg.Page.Page
	}
	totalPages := (int32(total) + limit - 1) / limit

	return connect.NewResponse(&orcv1.GetEventsResponse{
		Events: protoEvents,
		Page: &orcv1.PageResponse{
			Page:       page,
			Limit:      limit,
			Total:      int32(total),
			TotalPages: totalPages,
			HasMore:    page < totalPages-1,
		},
	}), nil
}

// GetTimeline returns timeline events for a task.
func (s *eventServer) GetTimeline(
	ctx context.Context,
	req *connect.Request[orcv1.GetTimelineRequest],
) (*connect.Response[orcv1.GetTimelineResponse], error) {
	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	pdb, err := s.getProjectDB(req.Msg.GetProjectId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("invalid project: %w", err))
	}

	// Build query options
	opts := db.QueryEventsOptions{
		TaskID: req.Msg.TaskId,
	}

	// Filter by event types
	if len(req.Msg.Types) > 0 {
		types := make([]string, len(req.Msg.Types))
		for i, t := range req.Msg.Types {
			types[i] = protoTimelineTypeToString(t)
		}
		opts.EventTypes = types
	}

	// Pagination
	if req.Msg.Page != nil {
		opts.Offset = int(req.Msg.Page.Page * req.Msg.Page.Limit)
		opts.Limit = int(req.Msg.Page.Limit)
	}

	dbEvents, err := pdb.QueryEventsWithTitles(opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Get total count
	total, _ := pdb.CountEvents(opts)

	// Convert to timeline events
	timelineEvents := make([]*orcv1.TimelineEvent, 0, len(dbEvents))
	for _, e := range dbEvents {
		te := dbEventToTimelineEvent(&e)
		if te != nil {
			timelineEvents = append(timelineEvents, te)
		}
	}

	// Build pagination response
	limit := int32(50)
	page := int32(0)
	if req.Msg.Page != nil {
		limit = req.Msg.Page.Limit
		page = req.Msg.Page.Page
	}
	totalPages := (int32(total) + limit - 1) / limit

	return connect.NewResponse(&orcv1.GetTimelineResponse{
		Events: timelineEvents,
		Page: &orcv1.PageResponse{
			Page:       page,
			Limit:      limit,
			Total:      int32(total),
			TotalPages: totalPages,
			HasMore:    page < totalPages-1,
		},
	}), nil
}

// =============================================================================
// Conversion helpers
// =============================================================================

// internalEventToProto converts an internal event to a proto event.
func internalEventToProto(e events.Event) *orcv1.Event {
	result := &orcv1.Event{
		Id:        uuid.New().String(),
		Timestamp: timestamppb.New(e.Time),
	}
	if e.TaskID != "" {
		result.TaskId = &e.TaskID
	}

	// Convert based on event type
	switch e.Type {
	case events.EventTaskCreated:
		if task, ok := e.Data.(map[string]any); ok {
			result.Payload = &orcv1.Event_TaskCreated{
				TaskCreated: &orcv1.TaskCreatedEvent{
					TaskId: e.TaskID,
					Title:  getString(task, "title"),
				},
			}
		}

	case events.EventTaskUpdated:
		result.Payload = &orcv1.Event_TaskUpdated{
			TaskUpdated: &orcv1.TaskUpdatedEvent{
				TaskId: e.TaskID,
			},
		}

	case events.EventTaskDeleted:
		result.Payload = &orcv1.Event_TaskDeleted{
			TaskDeleted: &orcv1.TaskDeletedEvent{
				TaskId: e.TaskID,
			},
		}

	case events.EventPhase:
		if update, ok := e.Data.(*events.PhaseUpdate); ok {
			result.Payload = &orcv1.Event_PhaseChanged{
				PhaseChanged: &orcv1.PhaseChangedEvent{
					TaskId:    e.TaskID,
					PhaseName: update.Phase,
					Status:    stringToProtoPhaseStatus(update.Status),
				},
			}
		} else if update, ok := e.Data.(events.PhaseUpdate); ok {
			result.Payload = &orcv1.Event_PhaseChanged{
				PhaseChanged: &orcv1.PhaseChangedEvent{
					TaskId:    e.TaskID,
					PhaseName: update.Phase,
					Status:    stringToProtoPhaseStatus(update.Status),
				},
			}
		}

	case events.EventTokens:
		if update, ok := e.Data.(*events.TokenUpdate); ok {
			result.Payload = &orcv1.Event_TokensUpdated{
				TokensUpdated: &orcv1.TokensUpdatedEvent{
					TaskId: e.TaskID,
					Tokens: &orcv1.TokenUsage{
						InputTokens:              int32(update.InputTokens),
						OutputTokens:             int32(update.OutputTokens),
						CacheCreationInputTokens: int32(update.CacheCreationInputTokens),
						CacheReadInputTokens:     int32(update.CacheReadInputTokens),
						TotalTokens:              int32(update.TotalTokens),
					},
				},
			}
		} else if update, ok := e.Data.(events.TokenUpdate); ok {
			result.Payload = &orcv1.Event_TokensUpdated{
				TokensUpdated: &orcv1.TokensUpdatedEvent{
					TaskId: e.TaskID,
					Tokens: &orcv1.TokenUsage{
						InputTokens:              int32(update.InputTokens),
						OutputTokens:             int32(update.OutputTokens),
						CacheCreationInputTokens: int32(update.CacheCreationInputTokens),
						CacheReadInputTokens:     int32(update.CacheReadInputTokens),
						TotalTokens:              int32(update.TotalTokens),
					},
				},
			}
		}

	case events.EventDecisionRequired:
		if data, ok := e.Data.(map[string]any); ok {
			result.Payload = &orcv1.Event_DecisionRequired{
				DecisionRequired: &orcv1.DecisionRequiredEvent{
					DecisionId: getString(data, "decision_id"),
					TaskId:     e.TaskID,
					Phase:      getString(data, "phase"),
					GateType:   getString(data, "gate_type"),
					Question:   getString(data, "question"),
					Context:    getString(data, "context"),
				},
			}
		}

	case events.EventDecisionResolved:
		if data, ok := e.Data.(map[string]any); ok {
			result.Payload = &orcv1.Event_DecisionResolved{
				DecisionResolved: &orcv1.DecisionResolvedEvent{
					DecisionId: getString(data, "decision_id"),
					TaskId:     e.TaskID,
					Phase:      getString(data, "phase"),
					Approved:   getBool(data, "approved"),
					ResolvedBy: getString(data, "resolved_by"),
				},
			}
		}

	case events.EventError:
		if data, ok := e.Data.(*events.ErrorData); ok {
			result.Payload = &orcv1.Event_Error{
				Error: &orcv1.ErrorEvent{
					TaskId: e.TaskID,
					Error:  data.Message,
				},
			}
		} else if data, ok := e.Data.(events.ErrorData); ok {
			result.Payload = &orcv1.Event_Error{
				Error: &orcv1.ErrorEvent{
					TaskId: e.TaskID,
					Error:  data.Message,
				},
			}
		}

	case events.EventFilesChanged:
		if data, ok := e.Data.(map[string]any); ok {
			result.Payload = &orcv1.Event_FilesChanged{
				FilesChanged: &orcv1.FilesChangedEvent{
					TaskId:         e.TaskID,
					TotalAdditions: getInt32(data, "total_additions"),
					TotalDeletions: getInt32(data, "total_deletions"),
				},
			}
		}

	case events.EventActivity:
		if update, ok := e.Data.(*events.ActivityUpdate); ok {
			result.Payload = &orcv1.Event_Activity{
				Activity: &orcv1.ActivityEvent{
					TaskId:   e.TaskID,
					PhaseId:  update.Phase,
					Activity: stringToProtoActivityState(update.Activity),
				},
			}
		} else if update, ok := e.Data.(events.ActivityUpdate); ok {
			result.Payload = &orcv1.Event_Activity{
				Activity: &orcv1.ActivityEvent{
					TaskId:   e.TaskID,
					PhaseId:  update.Phase,
					Activity: stringToProtoActivityState(update.Activity),
				},
			}
		} else {
			return nil
		}

	case events.EventSessionUpdate:
		if update, ok := e.Data.(*events.SessionUpdate); ok {
			result.Payload = &orcv1.Event_SessionMetrics{
				SessionMetrics: &orcv1.SessionMetricsEvent{
					DurationSeconds:  update.DurationSeconds,
					TotalTokens:      int32(update.TotalTokens),
					EstimatedCostUsd: update.EstimatedCostUSD,
					InputTokens:      int32(update.InputTokens),
					OutputTokens:     int32(update.OutputTokens),
					TasksRunning:     int32(update.TasksRunning),
					IsPaused:         update.IsPaused,
				},
			}
		} else if update, ok := e.Data.(events.SessionUpdate); ok {
			result.Payload = &orcv1.Event_SessionMetrics{
				SessionMetrics: &orcv1.SessionMetricsEvent{
					DurationSeconds:  update.DurationSeconds,
					TotalTokens:      int32(update.TotalTokens),
					EstimatedCostUsd: update.EstimatedCostUSD,
					InputTokens:      int32(update.InputTokens),
					OutputTokens:     int32(update.OutputTokens),
					TasksRunning:     int32(update.TasksRunning),
					IsPaused:         update.IsPaused,
				},
			}
		} else {
			return nil
		}

	case events.EventWarning:
		if data, ok := e.Data.(*events.WarningData); ok {
			warning := &orcv1.WarningEvent{
				TaskId:  e.TaskID,
				Message: data.Message,
			}
			if data.Phase != "" {
				warning.Phase = &data.Phase
			}
			result.Payload = &orcv1.Event_Warning{Warning: warning}
		} else if data, ok := e.Data.(events.WarningData); ok {
			warning := &orcv1.WarningEvent{
				TaskId:  e.TaskID,
				Message: data.Message,
			}
			if data.Phase != "" {
				warning.Phase = &data.Phase
			}
			result.Payload = &orcv1.Event_Warning{Warning: warning}
		} else {
			return nil
		}

	case events.EventHeartbeat:
		if data, ok := e.Data.(*events.HeartbeatData); ok {
			result.Payload = &orcv1.Event_Heartbeat{
				Heartbeat: &orcv1.HeartbeatEvent{
					Timestamp: timestamppb.New(data.Timestamp),
				},
			}
		} else if data, ok := e.Data.(events.HeartbeatData); ok {
			result.Payload = &orcv1.Event_Heartbeat{
				Heartbeat: &orcv1.HeartbeatEvent{
					Timestamp: timestamppb.New(data.Timestamp),
				},
			}
		} else {
			return nil
		}

	default:
		// Unknown event type, skip
		return nil
	}

	return result
}

// dbEventToProto converts a db event to a proto event.
// Uses the database event ID to ensure stable, deterministic IDs for deduplication.
func dbEventToProto(e *db.EventLog) *orcv1.Event {
	result := &orcv1.Event{
		Id:        strconv.FormatInt(e.ID, 10),
		Timestamp: timestamppb.New(e.CreatedAt),
	}
	if e.TaskID != "" {
		result.TaskId = &e.TaskID
	}

	// Basic conversion - data is JSON stored as any
	// For now, create events based on type
	switch e.EventType {
	case "phase":
		result.Payload = &orcv1.Event_PhaseChanged{
			PhaseChanged: &orcv1.PhaseChangedEvent{
				TaskId: e.TaskID,
			},
		}
		if e.Phase != nil {
			result.GetPhaseChanged().PhaseName = *e.Phase
		}

	case "transcript":
		// Transcript events don't map directly to proto events
		return nil

	case "activity":
		result.Payload = &orcv1.Event_Activity{
			Activity: &orcv1.ActivityEvent{
				TaskId: e.TaskID,
			},
		}
		if e.Phase != nil {
			result.GetActivity().PhaseId = *e.Phase
		}

	case "error":
		result.Payload = &orcv1.Event_Error{
			Error: &orcv1.ErrorEvent{
				TaskId: e.TaskID,
			},
		}

	default:
		return nil
	}

	return result
}

// dbEventToTimelineEvent converts a db event to a timeline event.
// Uses the database event ID to ensure stable, deterministic IDs for deduplication.
func dbEventToTimelineEvent(e *db.EventLogWithTitle) *orcv1.TimelineEvent {
	result := &orcv1.TimelineEvent{
		Id:        strconv.FormatInt(e.ID, 10),
		TaskId:    e.TaskID,
		TaskTitle: e.TaskTitle,
		EventType: stringToProtoTimelineType(e.EventType),
		Source:    e.Source,
		CreatedAt: timestamppb.New(e.CreatedAt),
	}
	if e.Phase != nil {
		result.Phase = e.Phase
	}
	if e.Iteration != nil {
		v := int32(*e.Iteration)
		result.Iteration = &v
	}

	return result
}

// protoTimelineTypeToString converts proto timeline type to db event type.
func protoTimelineTypeToString(t orcv1.TimelineEventType) string {
	switch t {
	case orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_PHASE_STARTED,
		orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_PHASE_COMPLETED,
		orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_PHASE_FAILED:
		return "phase"
	case orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_TASK_CREATED,
		orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_TASK_STARTED,
		orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_TASK_COMPLETED,
		orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_TASK_FAILED:
		return "task"
	case orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_ACTIVITY:
		return "activity"
	case orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_ERROR:
		return "error"
	case orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_METRICS:
		return "metrics"
	case orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_GATE_PENDING,
		orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_GATE_APPROVED,
		orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_GATE_REJECTED:
		return "gate"
	default:
		return ""
	}
}

// stringToProtoTimelineType converts db event type to proto timeline type.
func stringToProtoTimelineType(s string) orcv1.TimelineEventType {
	switch s {
	case "phase":
		return orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_PHASE_COMPLETED
	case "task":
		return orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_TASK_COMPLETED
	case "activity":
		return orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_ACTIVITY
	case "error":
		return orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_ERROR
	case "metrics":
		return orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_METRICS
	case "gate":
		return orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_GATE_PENDING
	default:
		return orcv1.TimelineEventType_TIMELINE_EVENT_TYPE_UNSPECIFIED
	}
}

// Helper functions for map access
func getString(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getBool(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getInt32(m map[string]any, key string) int32 {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return int32(n)
		case int32:
			return n
		case int64:
			return int32(n)
		case float64:
			return int32(n)
		}
	}
	return 0
}

// filterEventByInitiative returns true if the event should be filtered out based on initiative.
// When initiativeID is empty, no filtering occurs (backward compatible).
// When set, only events from tasks belonging to that initiative pass through.
func filterEventByInitiative(event events.Event, initiativeID string, backend storage.Backend) bool {
	// No filter set - pass everything through (backward compatible)
	if initiativeID == "" {
		return false
	}

	// Global events or events without a task ID are filtered when initiative is set
	if event.TaskID == "" || event.TaskID == globalTaskID {
		return true
	}

	// Load the task to check its initiative
	task, err := backend.LoadTask(event.TaskID)
	if err != nil || task == nil {
		// Task not found - filter out
		return true
	}

	// Task has no initiative - filter out
	if task.InitiativeId == nil {
		return true
	}

	// Check if task's initiative matches the filter
	if *task.InitiativeId != initiativeID {
		return true
	}

	// Task belongs to the filtered initiative - pass through
	return false
}

// stringToProtoActivityState converts a string activity state to proto ActivityState enum.
func stringToProtoActivityState(activity string) orcv1.ActivityState {
	switch activity {
	case "idle":
		return orcv1.ActivityState_ACTIVITY_STATE_IDLE
	case "waiting_api":
		return orcv1.ActivityState_ACTIVITY_STATE_WAITING_API
	case "streaming":
		return orcv1.ActivityState_ACTIVITY_STATE_STREAMING
	case "running_tool":
		return orcv1.ActivityState_ACTIVITY_STATE_RUNNING_TOOL
	case "processing":
		return orcv1.ActivityState_ACTIVITY_STATE_PROCESSING
	case "spec_analyzing":
		return orcv1.ActivityState_ACTIVITY_STATE_SPEC_ANALYZING
	case "spec_writing":
		return orcv1.ActivityState_ACTIVITY_STATE_SPEC_WRITING
	default:
		return orcv1.ActivityState_ACTIVITY_STATE_UNSPECIFIED
	}
}
