package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

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
	backend      storage.Backend
	projectCache *ProjectCache
	publisher    events.Publisher
	logger       *slog.Logger

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

// SetProjectCache sets the project cache for multi-project support.
func (s *threadServer) SetProjectCache(cache *ProjectCache) {
	s.projectCache = cache
}

// getBackend returns the appropriate backend for a project ID.
func (s *threadServer) getBackend(projectID string) (storage.Backend, error) {
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

	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
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
	for _, link := range req.Msg.Links {
		thread.Links = append(thread.Links, db.ThreadLink{
			LinkType: link.LinkType,
			TargetID: link.TargetId,
			Title:    link.Title,
		})
	}

	if err := backend.DB().CreateThread(thread); err != nil {
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
	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	thread, err := backend.DB().GetThread(req.Msg.ThreadId)
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
	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	opts := db.ThreadListOpts{
		Status: req.Msg.Status,
		TaskID: req.Msg.TaskId,
	}

	threads, err := backend.DB().ListThreads(opts)
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

	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	// Acquire per-thread lock to serialize concurrent SendMessage calls
	lockVal, _ := s.threadLocks.LoadOrStore(req.Msg.ThreadId, &sync.Mutex{})
	mu := lockVal.(*sync.Mutex)
	mu.Lock()
	defer mu.Unlock()

	// Reload the thread while holding the lock so session continuity and linked
	// context reflect the latest persisted state.
	thread, err := backend.DB().GetThread(req.Msg.ThreadId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get thread: %w", err))
	}
	if thread == nil {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("thread %s not found", req.Msg.ThreadId))
	}

	// Store user message
	userMsg := &db.ThreadMessage{
		ThreadID: thread.ID,
		Role:     "user",
		Content:  req.Msg.Content,
	}
	if err := backend.DB().AddThreadMessage(userMsg); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store user message: %w", err))
	}

	// Publish user message event
	s.publisher.Publish(events.NewProjectEvent(events.EventThreadMessage, req.Msg.ProjectId, thread.ID, events.ThreadMessageData{
		ThreadID:  thread.ID,
		MessageID: userMsg.ID,
		Role:      "user",
		Content:   userMsg.Content,
	}))

	// Build system prompt with task/initiative context
	systemPrompt := s.buildSystemPrompt(backend, thread)

	// Publish typing event before Claude invocation
	s.publisher.Publish(events.NewProjectEvent(events.EventThreadTyping, req.Msg.ProjectId, thread.ID, events.ThreadTypingData{
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

	// Clear typing indicator regardless of success or failure
	s.publisher.Publish(events.NewProjectEvent(events.EventThreadTyping, req.Msg.ProjectId, thread.ID, events.ThreadTypingData{
		ThreadID: thread.ID,
		IsTyping: false,
	}))

	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("claude invocation failed: %w", err))
	}

	// Store session ID from response (for multi-turn continuity)
	if result.SessionID != "" && result.SessionID != thread.SessionID {
		if updateErr := backend.DB().UpdateThreadSessionID(thread.ID, result.SessionID); updateErr != nil {
			s.logger.Warn("failed to update thread session ID", "thread_id", thread.ID, "error", updateErr)
		}
	}

	// Store assistant message
	assistantMsg := &db.ThreadMessage{
		ThreadID: thread.ID,
		Role:     "assistant",
		Content:  result.Content,
	}
	if err := backend.DB().AddThreadMessage(assistantMsg); err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("store assistant message: %w", err))
	}

	// Publish assistant message event
	s.publisher.Publish(events.NewProjectEvent(events.EventThreadMessage, req.Msg.ProjectId, thread.ID, events.ThreadMessageData{
		ThreadID:  thread.ID,
		MessageID: assistantMsg.ID,
		Role:      "assistant",
		Content:   assistantMsg.Content,
	}))
	s.publishThreadUpdated(req.Msg.ProjectId, thread.ID, "message_added")

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
	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	if err := backend.DB().ArchiveThread(req.Msg.ThreadId); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("archive thread: %w", err))
	}

	// Publish status change event
	s.publisher.Publish(events.NewProjectEvent(events.EventThreadStatus, req.Msg.ProjectId, req.Msg.ThreadId, events.ThreadStatusData{
		ThreadID:  req.Msg.ThreadId,
		OldStatus: "active",
		NewStatus: "archived",
	}))
	s.publishThreadUpdated(req.Msg.ProjectId, req.Msg.ThreadId, "archived")

	thread, err := backend.DB().GetThread(req.Msg.ThreadId)
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
	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	if err := backend.DB().DeleteThread(req.Msg.ThreadId); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("delete thread: %w", err))
	}

	return connect.NewResponse(&orcv1.DeleteThreadResponse{}), nil
}

// AddLink adds typed context to an existing thread.
func (s *threadServer) AddLink(
	ctx context.Context,
	req *connect.Request[orcv1.AddThreadLinkRequest],
) (*connect.Response[orcv1.AddThreadLinkResponse], error) {
	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}
	if req.Msg.Link == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("link is required"))
	}

	link := &db.ThreadLink{
		ThreadID: req.Msg.ThreadId,
		LinkType: req.Msg.Link.LinkType,
		TargetID: req.Msg.Link.TargetId,
		Title:    req.Msg.Link.Title,
	}
	if err := backend.DB().CreateThreadLink(link); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("create thread link: %w", err))
	}

	thread, err := backend.DB().GetThread(req.Msg.ThreadId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("reload thread: %w", err))
	}
	s.publishThreadUpdated(req.Msg.ProjectId, req.Msg.ThreadId, "link_added")
	return connect.NewResponse(&orcv1.AddThreadLinkResponse{Thread: threadToProto(thread)}), nil
}

// CreateRecommendationDraft persists a recommendation draft in a thread.
func (s *threadServer) CreateRecommendationDraft(
	ctx context.Context,
	req *connect.Request[orcv1.CreateThreadRecommendationDraftRequest],
) (*connect.Response[orcv1.CreateThreadRecommendationDraftResponse], error) {
	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}
	if req.Msg.Draft == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("draft is required"))
	}

	draft, err := threadRecommendationDraftFromProto(req.Msg.ThreadId, req.Msg.Draft)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := backend.DB().CreateThreadRecommendationDraft(draft); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("create recommendation draft: %w", err))
	}

	thread, err := backend.DB().GetThread(req.Msg.ThreadId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("reload thread: %w", err))
	}
	s.publishThreadUpdated(req.Msg.ProjectId, req.Msg.ThreadId, "recommendation_draft_created")
	return connect.NewResponse(&orcv1.CreateThreadRecommendationDraftResponse{
		Draft:  threadRecommendationDraftToProto(draft),
		Thread: threadToProto(thread),
	}), nil
}

// PromoteRecommendationDraft promotes a draft into the recommendation inbox.
func (s *threadServer) PromoteRecommendationDraft(
	ctx context.Context,
	req *connect.Request[orcv1.PromoteThreadRecommendationDraftRequest],
) (*connect.Response[orcv1.PromoteThreadRecommendationDraftResponse], error) {
	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}

	draft, rec, err := backend.DB().PromoteThreadRecommendationDraft(ctx, req.Msg.ThreadId, req.Msg.DraftId, req.Msg.PromotedBy)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("promote recommendation draft: %w", err))
	}

	recommendation, err := storageRecommendationToProto(rec)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("convert recommendation: %w", err))
	}
	s.publishThreadRecommendationCreated(req.Msg.ProjectId, recommendation)

	thread, err := backend.DB().GetThread(req.Msg.ThreadId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("reload thread: %w", err))
	}
	s.publishThreadUpdated(req.Msg.ProjectId, req.Msg.ThreadId, "recommendation_draft_promoted")
	return connect.NewResponse(&orcv1.PromoteThreadRecommendationDraftResponse{
		Draft:          threadRecommendationDraftToProto(draft),
		Recommendation: recommendation,
		Thread:         threadToProto(thread),
	}), nil
}

// CreateDecisionDraft persists an initiative decision draft in a thread.
func (s *threadServer) CreateDecisionDraft(
	ctx context.Context,
	req *connect.Request[orcv1.CreateThreadDecisionDraftRequest],
) (*connect.Response[orcv1.CreateThreadDecisionDraftResponse], error) {
	backend, err := s.getBackend(req.Msg.ProjectId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("get backend: %w", err))
	}
	if req.Msg.Draft == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("draft is required"))
	}

	draft := &db.ThreadDecisionDraft{
		ThreadID:     req.Msg.ThreadId,
		InitiativeID: req.Msg.Draft.InitiativeId,
		Decision:     req.Msg.Draft.Decision,
		Rationale:    req.Msg.Draft.Rationale,
	}
	if err := backend.DB().CreateThreadDecisionDraft(draft); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("create decision draft: %w", err))
	}

	thread, err := backend.DB().GetThread(req.Msg.ThreadId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("reload thread: %w", err))
	}
	s.publishThreadUpdated(req.Msg.ProjectId, req.Msg.ThreadId, "decision_draft_created")
	return connect.NewResponse(&orcv1.CreateThreadDecisionDraftResponse{
		Draft:  threadDecisionDraftToProto(draft),
		Thread: threadToProto(thread),
	}), nil
}

// PromoteDecisionDraft promotes a draft into a real initiative decision.
func (s *threadServer) PromoteDecisionDraft(
	ctx context.Context,
	req *connect.Request[orcv1.PromoteThreadDecisionDraftRequest],
) (*connect.Response[orcv1.PromoteThreadDecisionDraftResponse], error) {
	return nil, connect.NewError(
		connect.CodeFailedPrecondition,
		fmt.Errorf("decision drafts cannot be promoted directly; use the human acceptance flow"),
	)
}

// RecordDecision records a decision from a thread discussion to its linked initiative.
func (s *threadServer) RecordDecision(
	ctx context.Context,
	req *connect.Request[orcv1.RecordThreadDecisionRequest],
) (*connect.Response[orcv1.RecordThreadDecisionResponse], error) {
	return nil, connect.NewError(
		connect.CodeFailedPrecondition,
		fmt.Errorf("thread decisions must stay as drafts until a human accepts them"),
	)
}

// buildSystemPrompt constructs context-rich system prompt for Claude.
func (s *threadServer) buildSystemPrompt(backend storage.Backend, thread *db.Thread) string {
	var parts []string
	parts = append(parts, "You are a helpful assistant in a conversation thread.")

	if thread.TaskID != "" {
		task, err := backend.LoadTask(thread.TaskID)
		if err != nil {
			s.logger.Warn("failed to load thread task context", "thread_id", thread.ID, "task_id", thread.TaskID, "error", err)
		} else if task != nil {
			parts = append(parts, fmt.Sprintf("\nTask: %s", task.Title))
			if task.Description != nil && *task.Description != "" {
				parts = append(parts, fmt.Sprintf("Task Description: %s", *task.Description))
			}
		}
	}

	if thread.InitiativeID != "" {
		initiative, err := backend.DB().GetInitiative(thread.InitiativeID)
		if err != nil {
			s.logger.Warn("failed to load thread initiative context", "thread_id", thread.ID, "initiative_id", thread.InitiativeID, "error", err)
		} else if initiative != nil {
			parts = append(parts, fmt.Sprintf("\nInitiative: %s", initiative.Title))
			if initiative.Vision != "" {
				parts = append(parts, fmt.Sprintf("Vision: %s", initiative.Vision))
			}

			decisions, decErr := backend.DB().GetInitiativeDecisions(thread.InitiativeID)
			if decErr != nil {
				s.logger.Warn("failed to load thread initiative decisions", "thread_id", thread.ID, "initiative_id", thread.InitiativeID, "error", decErr)
			} else if len(decisions) > 0 {
				parts = append(parts, "\nDecisions:")
				for _, d := range decisions {
					parts = append(parts, fmt.Sprintf("- %s", d.Decision))
				}
			}
		}
	}

	if links := db.FormatThreadLinksForPrompt(thread.Links, 8); links != "" {
		parts = append(parts, "\nLinked context:")
		parts = append(parts, links)
	}

	if drafts := db.FormatThreadRecommendationDraftsForPrompt(thread.RecommendationDrafts, 5); drafts != "" {
		parts = append(parts, "\nRecommendation drafts:")
		parts = append(parts, drafts)
	}

	if drafts := db.FormatThreadDecisionDraftsForPrompt(thread.DecisionDrafts, 5); drafts != "" {
		parts = append(parts, "\nDecision drafts:")
		parts = append(parts, drafts)
	}

	if history := db.FormatThreadMessagesForPrompt(thread.Messages, 6); thread.SessionID == "" && history != "" {
		parts = append(parts, "\nRecent thread history:")
		parts = append(parts, history)
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
	for i := range t.Links {
		proto.Links = append(proto.Links, threadLinkToProto(&t.Links[i]))
	}
	for i := range t.RecommendationDrafts {
		proto.RecommendationDrafts = append(proto.RecommendationDrafts, threadRecommendationDraftToProto(&t.RecommendationDrafts[i]))
	}
	for i := range t.DecisionDrafts {
		proto.DecisionDrafts = append(proto.DecisionDrafts, threadDecisionDraftToProto(&t.DecisionDrafts[i]))
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

func threadLinkToProto(link *db.ThreadLink) *orcv1.ThreadLink {
	if link == nil {
		return nil
	}

	proto := &orcv1.ThreadLink{
		Id:       link.ID,
		ThreadId: link.ThreadID,
		LinkType: link.LinkType,
		TargetId: link.TargetID,
		Title:    link.Title,
	}
	if !link.CreatedAt.IsZero() {
		proto.CreatedAt = timestamppb.New(link.CreatedAt)
	}
	return proto
}

func threadRecommendationDraftFromProto(threadID string, protoDraft *orcv1.ThreadRecommendationDraft) (*db.ThreadRecommendationDraft, error) {
	if protoDraft == nil {
		return nil, fmt.Errorf("recommendation draft is required")
	}
	kind := recommendationKindProtoToString(protoDraft.Kind)
	if kind == "" {
		return nil, fmt.Errorf("recommendation kind is required")
	}
	return &db.ThreadRecommendationDraft{
		ThreadID:       threadID,
		Kind:           kind,
		Title:          protoDraft.Title,
		Summary:        protoDraft.Summary,
		ProposedAction: protoDraft.ProposedAction,
		Evidence:       protoDraft.Evidence,
		DedupeKey:      protoDraft.DedupeKey,
		SourceTaskID:   protoDraft.SourceTaskId,
		SourceRunID:    protoDraft.SourceRunId,
	}, nil
}

func threadRecommendationDraftToProto(draft *db.ThreadRecommendationDraft) *orcv1.ThreadRecommendationDraft {
	if draft == nil {
		return nil
	}
	kind, err := recommendationKindStringToProto(draft.Kind)
	if err != nil {
		kind = orcv1.RecommendationKind_RECOMMENDATION_KIND_UNSPECIFIED
	}
	proto := &orcv1.ThreadRecommendationDraft{
		Id:                       draft.ID,
		ThreadId:                 draft.ThreadID,
		Kind:                     kind,
		Title:                    draft.Title,
		Summary:                  draft.Summary,
		ProposedAction:           draft.ProposedAction,
		Evidence:                 draft.Evidence,
		DedupeKey:                draft.DedupeKey,
		SourceTaskId:             draft.SourceTaskID,
		SourceRunId:              draft.SourceRunID,
		Status:                   draft.Status,
		PromotedRecommendationId: draft.PromotedRecommendationID,
		PromotedBy:               draft.PromotedBy,
	}
	if draft.PromotedAt != nil {
		proto.PromotedAt = timestamppb.New(*draft.PromotedAt)
	}
	if !draft.CreatedAt.IsZero() {
		proto.CreatedAt = timestamppb.New(draft.CreatedAt)
	}
	if !draft.UpdatedAt.IsZero() {
		proto.UpdatedAt = timestamppb.New(draft.UpdatedAt)
	}
	return proto
}

func threadDecisionDraftToProto(draft *db.ThreadDecisionDraft) *orcv1.ThreadDecisionDraft {
	if draft == nil {
		return nil
	}
	proto := &orcv1.ThreadDecisionDraft{
		Id:                 draft.ID,
		ThreadId:           draft.ThreadID,
		InitiativeId:       draft.InitiativeID,
		Decision:           draft.Decision,
		Rationale:          draft.Rationale,
		Status:             draft.Status,
		PromotedDecisionId: draft.PromotedDecisionID,
		PromotedBy:         draft.PromotedBy,
	}
	if draft.PromotedAt != nil {
		proto.PromotedAt = timestamppb.New(*draft.PromotedAt)
	}
	if !draft.CreatedAt.IsZero() {
		proto.CreatedAt = timestamppb.New(draft.CreatedAt)
	}
	if !draft.UpdatedAt.IsZero() {
		proto.UpdatedAt = timestamppb.New(draft.UpdatedAt)
	}
	return proto
}

func storageRecommendationToProto(rec *db.Recommendation) (*orcv1.Recommendation, error) {
	if rec == nil {
		return nil, fmt.Errorf("recommendation is required")
	}

	kind, err := recommendationKindStringToProto(rec.Kind)
	if err != nil {
		return nil, err
	}
	status, err := recommendationStatusStringToProto(rec.Status)
	if err != nil {
		return nil, err
	}

	proto := &orcv1.Recommendation{
		Id:             rec.ID,
		Kind:           kind,
		Status:         status,
		Title:          rec.Title,
		Summary:        rec.Summary,
		ProposedAction: rec.ProposedAction,
		Evidence:       rec.Evidence,
		SourceTaskId:   rec.SourceTaskID,
		SourceRunId:    rec.SourceRunID,
		SourceThreadId: rec.SourceThreadID,
		DedupeKey:      rec.DedupeKey,
		PromotedToType: rec.PromotedToType,
		PromotedToId:   rec.PromotedToID,
		PromotedBy:     rec.PromotedBy,
	}
	if rec.DecidedBy != "" {
		proto.DecidedBy = &rec.DecidedBy
	}
	if rec.DecisionReason != "" {
		proto.DecisionReason = &rec.DecisionReason
	}
	if rec.DecidedAt != nil {
		proto.DecidedAt = timestamppb.New(*rec.DecidedAt)
	}
	if rec.PromotedAt != nil {
		proto.PromotedAt = timestamppb.New(*rec.PromotedAt)
	}
	if !rec.CreatedAt.IsZero() {
		proto.CreatedAt = timestamppb.New(rec.CreatedAt)
	}
	if !rec.UpdatedAt.IsZero() {
		proto.UpdatedAt = timestamppb.New(rec.UpdatedAt)
	}
	return proto, nil
}

func (s *threadServer) publishThreadRecommendationCreated(projectID string, rec *orcv1.Recommendation) {
	if s.publisher == nil || rec == nil {
		return
	}
	s.publisher.Publish(events.NewProjectEvent(events.EventRecommendationCreated, projectID, rec.SourceTaskId, events.RecommendationCreatedData{
		RecommendationID: rec.Id,
		Kind:             recommendationKindProtoToString(rec.Kind),
		Status:           recommendationStatusProtoToString(rec.Status),
		Title:            rec.Title,
		Summary:          rec.Summary,
		SourceTaskID:     rec.SourceTaskId,
		SourceRunID:      rec.SourceRunId,
		SourceThreadID:   rec.SourceThreadId,
		PromotedToType:   rec.PromotedToType,
		PromotedToID:     rec.PromotedToId,
		PromotedBy:       rec.PromotedBy,
		PromotedAt:       recommendationTimestampString(rec.PromotedAt),
	}))
}

func (s *threadServer) publishThreadUpdated(projectID, threadID, updateType string) {
	if s.publisher == nil || threadID == "" {
		return
	}
	s.publisher.Publish(events.NewProjectEvent(events.EventThreadUpdated, projectID, threadID, events.ThreadUpdatedData{
		ThreadID:   threadID,
		UpdateType: updateType,
	}))
}
