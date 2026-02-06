package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/gen/proto/orc/v1/orcv1connect"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/storage"
)

// threadServer implements the ThreadService Connect RPC handler.
type threadServer struct {
	orcv1connect.UnimplementedThreadServiceHandler
	backend   storage.Backend
	publisher events.Publisher
	logger    *slog.Logger

	// TurnExecutor factory creates executors for Claude conversations.
	// The factory receives the session ID (empty for new conversations).
	turnExecutorFactory func(sessionID string) executor.TurnExecutor
	factoryMu           sync.RWMutex

	// Per-thread mutex to prevent concurrent SendMessage on same thread.
	threadLocks sync.Map
}

// NewThreadServer creates a new thread service handler.
func NewThreadServer(backend storage.Backend, publisher events.Publisher, logger *slog.Logger) *threadServer {
	return &threadServer{
		backend:   backend,
		publisher: publisher,
		logger:    logger,
	}
}

// SetTurnExecutorFactory sets the factory used to create TurnExecutors for conversations.
func (s *threadServer) SetTurnExecutorFactory(factory func(sessionID string) executor.TurnExecutor) {
	s.factoryMu.Lock()
	defer s.factoryMu.Unlock()
	s.turnExecutorFactory = factory
}

func (s *threadServer) getTurnExecutorFactory() func(sessionID string) executor.TurnExecutor {
	s.factoryMu.RLock()
	defer s.factoryMu.RUnlock()
	return s.turnExecutorFactory
}

// CreateThread creates a new conversation thread.
func (s *threadServer) CreateThread(
	ctx context.Context,
	req *connect.Request[orcv1.CreateThreadRequest],
) (*connect.Response[orcv1.CreateThreadResponse], error) {
	if req.Msg.Title == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("title is required"))
	}

	thread := &db.Thread{
		Title: req.Msg.Title,
	}
	if req.Msg.TaskId != nil {
		thread.TaskID = *req.Msg.TaskId
	}
	if req.Msg.InitiativeId != nil {
		thread.InitiativeID = *req.Msg.InitiativeId
	}
	if req.Msg.FileContext != nil {
		thread.FileContext = *req.Msg.FileContext
	}

	if err := s.backend.DB().CreateThread(thread); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("create thread: %w", err))
	}

	return connect.NewResponse(&orcv1.CreateThreadResponse{
		Thread: threadToProto(thread),
	}), nil
}

// GetThread retrieves a thread with all its messages.
func (s *threadServer) GetThread(
	ctx context.Context,
	req *connect.Request[orcv1.GetThreadRequest],
) (*connect.Response[orcv1.GetThreadResponse], error) {
	thread, err := s.backend.DB().GetThread(req.Msg.ThreadId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get thread: %w", err))
	}
	if thread == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("thread %s not found", req.Msg.ThreadId))
	}

	return connect.NewResponse(&orcv1.GetThreadResponse{
		Thread: threadToProto(thread),
	}), nil
}

// ListThreads returns threads matching optional filters.
func (s *threadServer) ListThreads(
	ctx context.Context,
	req *connect.Request[orcv1.ListThreadsRequest],
) (*connect.Response[orcv1.ListThreadsResponse], error) {
	opts := db.ThreadListOpts{
		Status: req.Msg.Status,
		TaskID: req.Msg.TaskId,
	}

	threads, err := s.backend.DB().ListThreads(opts)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("list threads: %w", err))
	}

	var protoThreads []*orcv1.Thread
	for i := range threads {
		protoThreads = append(protoThreads, threadToProto(&threads[i]))
	}

	return connect.NewResponse(&orcv1.ListThreadsResponse{
		Threads: protoThreads,
	}), nil
}

// SendMessage stores a user message, invokes Claude, and stores the response.
func (s *threadServer) SendMessage(
	ctx context.Context,
	req *connect.Request[orcv1.SendThreadMessageRequest],
) (*connect.Response[orcv1.SendThreadMessageResponse], error) {
	if strings.TrimSpace(req.Msg.Content) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("content is required"))
	}

	// Verify thread exists
	thread, err := s.backend.DB().GetThread(req.Msg.ThreadId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get thread: %w", err))
	}
	if thread == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("thread %s not found", req.Msg.ThreadId))
	}

	// Acquire per-thread lock to serialize concurrent SendMessage calls
	lockVal, _ := s.threadLocks.LoadOrStore(req.Msg.ThreadId, &sync.Mutex{})
	mu := lockVal.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Store user message
	userMsg := &db.ThreadMessage{
		ThreadID: thread.ID,
		Role:     "user",
		Content:  req.Msg.Content,
	}
	if err := s.backend.DB().AddThreadMessage(userMsg); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store user message: %w", err))
	}

	// Publish user message event
	s.publisher.Publish(events.NewEvent(events.EventThreadMessage, thread.ID, events.ThreadMessageData{
		ThreadID:  thread.ID,
		MessageID: userMsg.ID,
		Role:      "user",
		Content:   userMsg.Content,
	}))

	// Build system prompt with task/initiative context
	systemPrompt := s.buildSystemPrompt(thread)

	// Publish typing event before Claude invocation
	s.publisher.Publish(events.NewEvent(events.EventThreadTyping, thread.ID, events.ThreadTypingData{
		ThreadID: thread.ID,
		IsTyping: true,
	}))

	// Invoke Claude via TurnExecutor
	factory := s.getTurnExecutorFactory()
	if factory == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("no turn executor configured"))
	}

	te := factory(thread.SessionID)
	prompt := systemPrompt + "\n\nUser message: " + req.Msg.Content
	result, err := te.ExecuteTurnWithoutSchema(ctx, prompt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("claude invocation failed: %w", err))
	}

	// Store session ID from response (for multi-turn continuity)
	if result.SessionID != "" && result.SessionID != thread.SessionID {
		if updateErr := s.backend.DB().UpdateThreadSessionID(thread.ID, result.SessionID); updateErr != nil {
			s.logger.Warn("failed to update thread session ID", "thread_id", thread.ID, "error", updateErr)
		}
	}

	// Store assistant message
	assistantMsg := &db.ThreadMessage{
		ThreadID: thread.ID,
		Role:     "assistant",
		Content:  result.Content,
	}
	if err := s.backend.DB().AddThreadMessage(assistantMsg); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store assistant message: %w", err))
	}

	// Publish assistant message event
	s.publisher.Publish(events.NewEvent(events.EventThreadMessage, thread.ID, events.ThreadMessageData{
		ThreadID:  thread.ID,
		MessageID: assistantMsg.ID,
		Role:      "assistant",
		Content:   assistantMsg.Content,
	}))

	return connect.NewResponse(&orcv1.SendThreadMessageResponse{
		UserMessage:      threadMessageToProto(userMsg),
		AssistantMessage: threadMessageToProto(assistantMsg),
	}), nil
}

// ArchiveThread sets a thread's status to "archived".
func (s *threadServer) ArchiveThread(
	ctx context.Context,
	req *connect.Request[orcv1.ArchiveThreadRequest],
) (*connect.Response[orcv1.ArchiveThreadResponse], error) {
	if err := s.backend.DB().ArchiveThread(req.Msg.ThreadId); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("archive thread: %w", err))
	}

	// Publish status change event
	s.publisher.Publish(events.NewEvent(events.EventThreadStatus, req.Msg.ThreadId, events.ThreadStatusData{
		ThreadID:  req.Msg.ThreadId,
		OldStatus: "active",
		NewStatus: "archived",
	}))

	thread, err := s.backend.DB().GetThread(req.Msg.ThreadId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get archived thread: %w", err))
	}

	return connect.NewResponse(&orcv1.ArchiveThreadResponse{
		Thread: threadToProto(thread),
	}), nil
}

// DeleteThread removes a thread and all its messages.
func (s *threadServer) DeleteThread(
	ctx context.Context,
	req *connect.Request[orcv1.DeleteThreadRequest],
) (*connect.Response[orcv1.DeleteThreadResponse], error) {
	if err := s.backend.DB().DeleteThread(req.Msg.ThreadId); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete thread: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteThreadResponse{}), nil
}

// RecordDecision records a decision from a thread discussion to its linked initiative.
func (s *threadServer) RecordDecision(
	ctx context.Context,
	req *connect.Request[orcv1.RecordThreadDecisionRequest],
) (*connect.Response[orcv1.RecordThreadDecisionResponse], error) {
	thread, err := s.backend.DB().GetThread(req.Msg.ThreadId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get thread: %w", err))
	}
	if thread == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("thread %s not found", req.Msg.ThreadId))
	}

	if thread.InitiativeID == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("thread %s has no linked initiative", thread.ID))
	}

	// Generate decision ID
	decisionID := fmt.Sprintf("DEC-%s-%d", thread.ID, time.Now().UnixMilli())

	decision := &db.InitiativeDecision{
		ID:           decisionID,
		InitiativeID: thread.InitiativeID,
		Decision:     req.Msg.Decision,
		Rationale:    req.Msg.Rationale,
		DecidedBy:    "thread:" + thread.ID,
		DecidedAt:    time.Now(),
	}

	if err := s.backend.DB().AddInitiativeDecision(decision); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("record decision: %w", err))
	}

	return connect.NewResponse(&orcv1.RecordThreadDecisionResponse{
		DecisionId: decisionID,
	}), nil
}

// buildSystemPrompt constructs context-rich system prompt for Claude.
func (s *threadServer) buildSystemPrompt(thread *db.Thread) string {
	var parts []string
	parts = append(parts, "You are a helpful assistant in a conversation thread.")

	if thread.TaskID != "" {
		task, err := s.backend.LoadTask(thread.TaskID)
		if err == nil && task != nil {
			parts = append(parts, fmt.Sprintf("\nTask: %s", task.Title))
			if task.Description != nil && *task.Description != "" {
				parts = append(parts, fmt.Sprintf("Task Description: %s", *task.Description))
			}
		}
	}

	if thread.InitiativeID != "" {
		initiative, err := s.backend.DB().GetInitiative(thread.InitiativeID)
		if err == nil && initiative != nil {
			parts = append(parts, fmt.Sprintf("\nInitiative: %s", initiative.Title))
			if initiative.Vision != "" {
				parts = append(parts, fmt.Sprintf("Vision: %s", initiative.Vision))
			}

			decisions, decErr := s.backend.DB().GetInitiativeDecisions(thread.InitiativeID)
			if decErr == nil && len(decisions) > 0 {
				parts = append(parts, "\nDecisions:")
				for _, d := range decisions {
					parts = append(parts, fmt.Sprintf("- %s", d.Decision))
				}
			}
		}
	}

	return strings.Join(parts, "\n")
}

// threadToProto converts a db.Thread to the proto Thread message.
func threadToProto(t *db.Thread) *orcv1.Thread {
	if t == nil {
		return nil
	}

	proto := &orcv1.Thread{
		Id:           t.ID,
		Title:        t.Title,
		Status:       t.Status,
		TaskId:       t.TaskID,
		InitiativeId: t.InitiativeID,
		SessionId:    t.SessionID,
		FileContext:  t.FileContext,
	}

	if !t.CreatedAt.IsZero() {
		proto.CreatedAt = timestamppb.New(t.CreatedAt)
	}
	if !t.UpdatedAt.IsZero() {
		proto.UpdatedAt = timestamppb.New(t.UpdatedAt)
	}

	for i := range t.Messages {
		proto.Messages = append(proto.Messages, threadMessageToProto(&t.Messages[i]))
	}

	return proto
}

// threadMessageToProto converts a db.ThreadMessage to the proto ThreadMessage.
func threadMessageToProto(m *db.ThreadMessage) *orcv1.ThreadMessage {
	if m == nil {
		return nil
	}

	proto := &orcv1.ThreadMessage{
		Id:       m.ID,
		ThreadId: m.ThreadID,
		Role:     m.Role,
		Content:  m.Content,
	}

	if !m.CreatedAt.IsZero() {
		proto.CreatedAt = timestamppb.New(m.CreatedAt)
	}

	return proto
}
