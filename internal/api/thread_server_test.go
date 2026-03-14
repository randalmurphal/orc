package api

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/storage"
)

// ============================================================================
// SC-9: ThreadService is registered and accessible via Connect RPC
// ============================================================================

func TestThreadServer_Registration(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewThreadServer(backend, publisher, slog.Default())

	// Call CreateThread via the server directly (simulates Connect RPC call)
	resp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title: "Test thread",
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread via Connect: %v", err)
	}

	if resp.Msg.Thread == nil {
		t.Fatal("expected non-nil thread in response")
	}
	if resp.Msg.Thread.Id == "" {
		t.Error("expected non-empty thread ID")
	}
	if resp.Msg.Thread.Title != "Test thread" {
		t.Errorf("expected title 'Test thread', got %q", resp.Msg.Thread.Title)
	}
	if resp.Msg.Thread.Status != "active" {
		t.Errorf("expected status 'active', got %q", resp.Msg.Thread.Status)
	}
}

func TestThreadServer_GetThread_UsesCanonicalTypedLinks(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewThreadServer(backend, publisher, slog.Default())

	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title: "Generic thread",
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	threadID := createResp.Msg.Thread.Id
	_, err = server.AddLink(
		context.Background(),
		connect.NewRequest(&orcv1.AddThreadLinkRequest{
			ThreadId: threadID,
			Link: &orcv1.ThreadLinkInput{
				LinkType: db.ThreadLinkTypeTask,
				TargetId: "TASK-123",
				Title:    "TASK-123",
			},
		}),
	)
	if err != nil {
		t.Fatalf("AddLink: %v", err)
	}

	getResp, err := server.GetThread(
		context.Background(),
		connect.NewRequest(&orcv1.GetThreadRequest{
			ThreadId: threadID,
		}),
	)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}

	if getResp.Msg.Thread.TaskId != "TASK-123" {
		t.Fatalf("expected canonical task ID TASK-123, got %q", getResp.Msg.Thread.TaskId)
	}
}

func TestThreadServer_ListThreads_FiltersByInitiative(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewThreadServer(backend, publisher, slog.Default())

	matchingResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title: "Matching initiative thread",
			Links: []*orcv1.ThreadLinkInput{
				{
					LinkType: db.ThreadLinkTypeInitiative,
					TargetId: "INIT-001",
					Title:    "INIT-001",
				},
			},
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread(matching): %v", err)
	}
	_, err = server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title: "Different initiative thread",
			Links: []*orcv1.ThreadLinkInput{
				{
					LinkType: db.ThreadLinkTypeInitiative,
					TargetId: "INIT-002",
					Title:    "INIT-002",
				},
			},
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread(other): %v", err)
	}

	listResp, err := server.ListThreads(
		context.Background(),
		connect.NewRequest(&orcv1.ListThreadsRequest{
			InitiativeId: "INIT-001",
		}),
	)
	if err != nil {
		t.Fatalf("ListThreads: %v", err)
	}
	if len(listResp.Msg.Threads) != 1 {
		t.Fatalf("expected 1 matching thread, got %d", len(listResp.Msg.Threads))
	}
	if listResp.Msg.Threads[0].Id != matchingResp.Msg.Thread.Id {
		t.Fatalf("expected thread %s, got %s", matchingResp.Msg.Thread.Id, listResp.Msg.Threads[0].Id)
	}
}

// ============================================================================
// SC-6: SendMessage stores user message, invokes Claude, stores response
// ============================================================================

func TestThreadServer_SendMessage(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mock := executor.NewMockTurnExecutor("This is Claude's response")

	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		if sessionID != "" {
			mock.UpdateSessionID(sessionID)
		}
		return mock
	})

	// Create a thread first
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title: "Chat thread",
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// Send a message
	sendResp, err := server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: threadID,
			Content:  "How should I implement login?",
		}),
	)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Must return user message
	if sendResp.Msg.UserMessage == nil {
		t.Fatal("expected non-nil user message")
	}
	if sendResp.Msg.UserMessage.Role != "user" {
		t.Errorf("expected user message role 'user', got %q", sendResp.Msg.UserMessage.Role)
	}
	if sendResp.Msg.UserMessage.Content != "How should I implement login?" {
		t.Errorf("expected user message content, got %q", sendResp.Msg.UserMessage.Content)
	}

	// Must return assistant message
	if sendResp.Msg.AssistantMessage == nil {
		t.Fatal("expected non-nil assistant message")
	}
	if sendResp.Msg.AssistantMessage.Role != "assistant" {
		t.Errorf("expected assistant message role 'assistant', got %q", sendResp.Msg.AssistantMessage.Role)
	}
	if sendResp.Msg.AssistantMessage.Content != "This is Claude's response" {
		t.Errorf("expected assistant content 'This is Claude's response', got %q",
			sendResp.Msg.AssistantMessage.Content)
	}

	// Verify mock was called
	if mock.CallCount() != 1 {
		t.Errorf("expected 1 TurnExecutor call, got %d", mock.CallCount())
	}

	// Verify messages are persisted (via GetThread)
	getResp, err := server.GetThread(
		context.Background(),
		connect.NewRequest(&orcv1.GetThreadRequest{
			ThreadId: threadID,
		}),
	)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if len(getResp.Msg.Thread.Messages) != 2 {
		t.Errorf("expected 2 persisted messages, got %d", len(getResp.Msg.Thread.Messages))
	}
}

func TestThreadServer_SendMessage_EmptyContent(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewThreadServer(backend, publisher, slog.Default())

	// Create a thread
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title: "Test",
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// SendMessage with empty content
	_, err = server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: createResp.Msg.Thread.Id,
			Content:  "",
		}),
	)
	if err == nil {
		t.Fatal("expected error for empty content, got nil")
	}

	connectErr := new(connect.Error)
	if !threadErrorAs(err, &connectErr) || connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected InvalidArgument error, got: %v", err)
	}
}

func TestThreadServer_SendMessage_NotFound(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewThreadServer(backend, publisher, slog.Default())

	_, err := server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: "THR-999",
			Content:  "Hello",
		}),
	)
	if err == nil {
		t.Fatal("expected error for non-existent thread, got nil")
	}

	connectErr := new(connect.Error)
	if !threadErrorAs(err, &connectErr) || connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

// ============================================================================
// SC-6 Failure Mode: Claude CLI invocation failure
// ============================================================================

func TestThreadServer_SendMessage_ClaudeError(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mock := executor.NewMockTurnExecutor("")
	mock.Error = fmt.Errorf("claude CLI unavailable")

	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	// Create thread
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "Error test"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// SendMessage should fail
	_, err = server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: threadID,
			Content:  "Hello",
		}),
	)
	if err == nil {
		t.Fatal("expected error when Claude CLI fails, got nil")
	}

	// User message should be stored, but no assistant message
	getResp, err := server.GetThread(
		context.Background(),
		connect.NewRequest(&orcv1.GetThreadRequest{ThreadId: threadID}),
	)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}

	// Only user message should be stored (not the assistant)
	if len(getResp.Msg.Thread.Messages) != 1 {
		t.Errorf("expected 1 message (user only), got %d", len(getResp.Msg.Thread.Messages))
	}
	if len(getResp.Msg.Thread.Messages) > 0 && getResp.Msg.Thread.Messages[0].Role != "user" {
		t.Errorf("expected surviving message to be 'user', got %q",
			getResp.Msg.Thread.Messages[0].Role)
	}
}

func TestThreadServer_SendMessage_Timeout(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mock := executor.NewMockTurnExecutor("")
	mock.Delay = 5 * time.Second // Long delay

	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	// Create thread
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "Timeout test"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// Use a short context timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = server.SendMessage(
		ctx,
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: createResp.Msg.Thread.Id,
			Content:  "This should timeout",
		}),
	)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

// ============================================================================
// SC-7: Thread maintains session ID for multi-turn continuity
// ============================================================================

func TestThreadServer_SessionContinuity(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	var sessionIDsReceived []string
	mock := executor.NewMockTurnExecutor("Response 1")
	mock.Responses = []string{"Response 1", "Response 2"}

	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		sessionIDsReceived = append(sessionIDsReceived, sessionID)
		if sessionID != "" {
			mock.UpdateSessionID(sessionID)
		}
		return mock
	})

	// Create thread
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "Session test"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// First message — creates a new session
	_, err = server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: threadID,
			Content:  "First message",
		}),
	)
	if err != nil {
		t.Fatalf("SendMessage 1: %v", err)
	}

	// Second message — should reuse the same session
	_, err = server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: threadID,
			Content:  "Second message",
		}),
	)
	if err != nil {
		t.Fatalf("SendMessage 2: %v", err)
	}

	// First call should receive empty session ID (new session)
	if len(sessionIDsReceived) < 2 {
		t.Fatalf("expected at least 2 factory calls, got %d", len(sessionIDsReceived))
	}
	if sessionIDsReceived[0] != "" {
		t.Errorf("first call should have empty session ID, got %q", sessionIDsReceived[0])
	}

	// Second call should receive the session ID from the first response
	if sessionIDsReceived[1] == "" {
		t.Error("second call should have non-empty session ID (reuse)")
	}
	if sessionIDsReceived[1] != mock.SessionIDValue {
		t.Errorf("second call session ID %q should match mock session ID %q",
			sessionIDsReceived[1], mock.SessionIDValue)
	}
}

func TestThreadServer_FirstMessage_CreatesSession(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mock := executor.NewMockTurnExecutor("Hello")
	mock.SessionIDValue = "new-session-abc"

	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	// Create thread
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "New session test"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// Send first message
	_, err = server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: threadID,
			Content:  "Hello",
		}),
	)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Verify session ID was stored on the thread
	getResp, err := server.GetThread(
		context.Background(),
		connect.NewRequest(&orcv1.GetThreadRequest{ThreadId: threadID}),
	)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if getResp.Msg.Thread.SessionId != "new-session-abc" {
		t.Errorf("expected session_id 'new-session-abc', got %q",
			getResp.Msg.Thread.SessionId)
	}
}

// ============================================================================
// SC-8: System prompt includes task description and initiative context
// ============================================================================

func TestThreadServer_SystemPrompt(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	// Create a task in the backend
	taskDesc := "Build a REST endpoint for user authentication with JWT tokens"
	taskProto := &orcv1.Task{
		Id:          "TASK-001",
		Title:       "Implement login endpoint",
		Description: &taskDesc,
		Status:      orcv1.TaskStatus_TASK_STATUS_PLANNED,
	}
	if err := backend.SaveTask(taskProto); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}

	// Create an initiative with vision and decisions
	initiative := &db.Initiative{
		ID:     "INIT-001",
		Title:  "User Auth",
		Status: "active",
		Vision: "JWT-based auth with refresh tokens for secure stateless authentication",
	}
	if err := backend.DB().SaveInitiative(initiative); err != nil {
		t.Fatalf("SaveInitiative: %v", err)
	}
	decision := &db.InitiativeDecision{
		ID:           "DEC-001",
		InitiativeID: "INIT-001",
		Decision:     "Use bcrypt for passwords",
		Rationale:    "Industry standard",
		DecidedAt:    time.Now(),
	}
	if err := backend.DB().AddInitiativeDecision(decision); err != nil {
		t.Fatalf("AddInitiativeDecision: %v", err)
	}

	mock := executor.NewMockTurnExecutor("Sure, I'll help with login")
	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	// Create thread linked to task and initiative
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title:        "Login discussion",
			TaskId:       threadStringPtr("TASK-001"),
			InitiativeId: threadStringPtr("INIT-001"),
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// Send message to trigger system prompt construction
	_, err = server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: createResp.Msg.Thread.Id,
			Content:  "How should I implement this?",
		}),
	)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Check that the prompt sent to Claude contains task context
	if len(mock.Prompts) == 0 {
		t.Fatal("expected at least 1 prompt sent to Claude")
	}
	prompt := mock.Prompts[0]

	// Prompt should contain task description
	if !strings.Contains(prompt, "Implement login endpoint") &&
		!strings.Contains(prompt, "REST endpoint for user authentication") {
		t.Error("expected prompt to contain task description")
	}

	// Prompt should contain initiative vision
	if !strings.Contains(prompt, "JWT-based auth with refresh tokens") {
		t.Error("expected prompt to contain initiative vision")
	}

	// Prompt should contain initiative decision
	if !strings.Contains(prompt, "Use bcrypt for passwords") {
		t.Error("expected prompt to contain initiative decision")
	}
}

func TestThreadServer_SystemPrompt_NoLinks(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mock := executor.NewMockTurnExecutor("Hello!")
	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	// Thread with no task or initiative links
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title: "General discussion",
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	// Should not error — minimal system prompt is fine
	_, err = server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: createResp.Msg.Thread.Id,
			Content:  "Hello",
		}),
	)
	if err != nil {
		t.Fatalf("SendMessage with no links should succeed: %v", err)
	}

	if mock.CallCount() != 1 {
		t.Errorf("expected 1 Claude call, got %d", mock.CallCount())
	}
}

func TestThreadServer_SystemPrompt_IncludesPersistedThreadContext(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mustCreateThreadServerFixtures(t, backend)
	if err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     "INIT-001",
		Title:  "Operator Control Plane",
		Status: "active",
	}); err != nil {
		t.Fatalf("SaveInitiative: %v", err)
	}

	thread := &db.Thread{
		Title:        "Workspace context",
		TaskID:       "TASK-001",
		InitiativeID: "INIT-001",
		Links: []db.ThreadLink{
			{
				LinkType: db.ThreadLinkTypeDiff,
				TargetID: "TASK-001:web/src/components/layout/DiscussionPanel.tsx",
				Title:    "DiscussionPanel diff",
			},
		},
	}
	if err := backend.DB().CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	if err := backend.DB().AddThreadMessage(&db.ThreadMessage{
		ThreadID: thread.ID,
		Role:     "user",
		Content:  "Remember the prior discussion.",
	}); err != nil {
		t.Fatalf("AddThreadMessage: %v", err)
	}
	if err := backend.DB().CreateThreadRecommendationDraft(&db.ThreadRecommendationDraft{
		ThreadID:       thread.ID,
		Kind:           db.RecommendationKindFollowUp,
		Title:          "Add promotion coverage",
		Summary:        "The promotion path needs an API regression test.",
		ProposedAction: "Add a thread promotion regression test.",
		Evidence:       "No current test covers this flow.",
	}); err != nil {
		t.Fatalf("CreateThreadRecommendationDraft: %v", err)
	}
	if err := backend.DB().CreateThreadDecisionDraft(&db.ThreadDecisionDraft{
		ThreadID:     thread.ID,
		InitiativeID: "INIT-001",
		Decision:     "Keep thread context persisted",
		Rationale:    "Reopening a thread should preserve the real workspace state.",
	}); err != nil {
		t.Fatalf("CreateThreadDecisionDraft: %v", err)
	}

	mock := executor.NewMockTurnExecutor("Context captured")
	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	_, err := server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: thread.ID,
			Content:  "What still matters here?",
		}),
	)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	if len(mock.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(mock.Prompts))
	}
	prompt := mock.Prompts[0]
	if !strings.Contains(prompt, "Linked context:") || !strings.Contains(prompt, "DiscussionPanel diff") {
		t.Fatalf("expected prompt to include linked context, got %q", prompt)
	}
	if !strings.Contains(prompt, "Recommendation drafts:") || !strings.Contains(prompt, "Add promotion coverage") {
		t.Fatalf("expected prompt to include recommendation drafts, got %q", prompt)
	}
	if !strings.Contains(prompt, "Decision drafts:") || !strings.Contains(prompt, "Keep thread context persisted") {
		t.Fatalf("expected prompt to include decision drafts, got %q", prompt)
	}
	if !strings.Contains(prompt, "Recent thread history:") || !strings.Contains(prompt, "Remember the prior discussion.") {
		t.Fatalf("expected prompt to include recent thread history, got %q", prompt)
	}
}

// ============================================================================
// SC-10: thread_message events are published on message add
// ============================================================================

func TestThreadServer_MessageEvents(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mock := executor.NewMockTurnExecutor("Claude response")
	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	// Create thread
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "Events test"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// Subscribe to events for this thread
	eventCh := publisher.Subscribe(threadID)

	// Send message
	_, err = server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: threadID,
			Content:  "Test message",
		}),
	)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Collect events (with timeout)
	var receivedEvents []events.Event
	timeout := time.After(2 * time.Second)
	for {
		select {
		case evt := <-eventCh:
			receivedEvents = append(receivedEvents, evt)
			// We expect at least 2 thread_message events (user + assistant)
			// plus 2 thread_typing events (typing=true + typing=false)
			if len(receivedEvents) >= 4 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}
done:

	// Must have at least 2 thread_message events
	messageEvents := threadFilterEvents(receivedEvents, events.EventThreadMessage)
	if len(messageEvents) < 2 {
		t.Errorf("expected at least 2 thread_message events, got %d", len(messageEvents))
	}

	// First message event should be for user message
	if len(messageEvents) >= 1 {
		data, ok := messageEvents[0].Data.(events.ThreadMessageData)
		if !ok {
			t.Error("expected ThreadMessageData for first event")
		} else {
			if data.ThreadID != threadID {
				t.Errorf("expected thread_id %q, got %q", threadID, data.ThreadID)
			}
			if data.Role != "user" {
				t.Errorf("expected role 'user' for first event, got %q", data.Role)
			}
		}
	}

	// Second message event should be for assistant message
	if len(messageEvents) >= 2 {
		data, ok := messageEvents[1].Data.(events.ThreadMessageData)
		if !ok {
			t.Error("expected ThreadMessageData for second event")
		} else {
			if data.Role != "assistant" {
				t.Errorf("expected role 'assistant' for second event, got %q", data.Role)
			}
		}
	}
}

// ============================================================================
// SC-11: thread_typing event published while Claude is generating
// ============================================================================

func TestThreadServer_TypingEvent(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mock := executor.NewMockTurnExecutor("Response")
	// Small delay so we can observe the typing event before response
	mock.Delay = 50 * time.Millisecond

	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	// Create thread
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "Typing test"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// Subscribe before sending
	eventCh := publisher.Subscribe(threadID)

	// Send message
	_, err = server.SendMessage(
		context.Background(),
		connect.NewRequest(&orcv1.SendThreadMessageRequest{
			ThreadId: threadID,
			Content:  "Hello",
		}),
	)
	if err != nil {
		t.Fatalf("SendMessage: %v", err)
	}

	// Collect events
	var receivedEvents []events.Event
	timeout := time.After(2 * time.Second)
	for {
		select {
		case evt := <-eventCh:
			receivedEvents = append(receivedEvents, evt)
			if len(receivedEvents) >= 4 {
				goto done
			}
		case <-timeout:
			goto done
		}
	}
done:

	// Must have a thread_typing event
	typingEvents := threadFilterEvents(receivedEvents, events.EventThreadTyping)
	if len(typingEvents) == 0 {
		t.Error("expected at least 1 thread_typing event")
	}

	// Typing event should contain the thread ID
	if len(typingEvents) > 0 {
		data, ok := typingEvents[0].Data.(events.ThreadTypingData)
		if !ok {
			t.Error("expected ThreadTypingData for typing event")
		} else if data.ThreadID != threadID {
			t.Errorf("expected thread_id %q in typing event, got %q",
				threadID, data.ThreadID)
		}
	}

	// Typing event must come before the assistant message event
	var typingIdx, assistantIdx int
	typingIdx = -1
	assistantIdx = -1
	for i, evt := range receivedEvents {
		if evt.Type == events.EventThreadTyping {
			typingIdx = i
		}
		if evt.Type == events.EventThreadMessage {
			if data, ok := evt.Data.(events.ThreadMessageData); ok && data.Role == "assistant" {
				assistantIdx = i
			}
		}
	}
	if typingIdx >= 0 && assistantIdx >= 0 && typingIdx >= assistantIdx {
		t.Error("typing event should come before assistant message event")
	}
}

// ============================================================================
// SC-4: ArchiveThread publishes thread_status event
// ============================================================================

func TestThreadServer_Archive(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewThreadServer(backend, publisher, slog.Default())

	// Create thread
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "Archive test"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// Subscribe to events
	eventCh := publisher.Subscribe(threadID)

	// Archive thread
	_, err = server.ArchiveThread(
		context.Background(),
		connect.NewRequest(&orcv1.ArchiveThreadRequest{
			ThreadId: threadID,
		}),
	)
	if err != nil {
		t.Fatalf("ArchiveThread: %v", err)
	}

	// Verify status changed
	getResp, err := server.GetThread(
		context.Background(),
		connect.NewRequest(&orcv1.GetThreadRequest{ThreadId: threadID}),
	)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if getResp.Msg.Thread.Status != "archived" {
		t.Errorf("expected status 'archived', got %q", getResp.Msg.Thread.Status)
	}

	// Verify thread_status event published
	var statusEvent *events.Event
	timeout := time.After(1 * time.Second)
	for {
		select {
		case evt := <-eventCh:
			if evt.Type == events.EventThreadStatus {
				statusEvent = &evt
				goto gotEvent
			}
		case <-timeout:
			goto gotEvent
		}
	}
gotEvent:
	if statusEvent == nil {
		t.Error("expected thread_status event to be published on archive")
	}
}

func TestThreadServer_Archive_NotFound(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewThreadServer(backend, publisher, slog.Default())

	_, err := server.ArchiveThread(
		context.Background(),
		connect.NewRequest(&orcv1.ArchiveThreadRequest{
			ThreadId: "THR-999",
		}),
	)
	if err == nil {
		t.Fatal("expected error archiving non-existent thread")
	}

	connectErr := new(connect.Error)
	if !threadErrorAs(err, &connectErr) || connectErr.Code() != connect.CodeNotFound {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}

func TestThreadServer_Archive_Idempotent(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewThreadServer(backend, publisher, slog.Default())

	// Create and archive
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "Idempotent archive"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// Archive twice — second should not error
	_, err = server.ArchiveThread(
		context.Background(),
		connect.NewRequest(&orcv1.ArchiveThreadRequest{ThreadId: threadID}),
	)
	if err != nil {
		t.Fatalf("ArchiveThread (first): %v", err)
	}

	_, err = server.ArchiveThread(
		context.Background(),
		connect.NewRequest(&orcv1.ArchiveThreadRequest{ThreadId: threadID}),
	)
	if err != nil {
		t.Fatalf("ArchiveThread (second): %v", err)
	}
}

// ============================================================================
// SC-12: RecordDecision rejects direct promotion and preserves draft-only flow
// ============================================================================

func TestThreadServer_RecordDecision(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	// Create initiative
	initiative := &db.Initiative{
		ID:     "INIT-001",
		Title:  "Auth System",
		Status: "active",
	}
	if err := backend.DB().SaveInitiative(initiative); err != nil {
		t.Fatalf("SaveInitiative: %v", err)
	}

	server := NewThreadServer(backend, publisher, slog.Default())

	// Create thread linked to initiative
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title:        "Design discussion",
			InitiativeId: threadStringPtr("INIT-001"),
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// RecordDecision should be rejected because thread decisions stay as drafts.
	_, err = server.RecordDecision(
		context.Background(),
		connect.NewRequest(&orcv1.RecordThreadDecisionRequest{
			ThreadId:  threadID,
			Decision:  "Use JWT auth",
			Rationale: "Industry standard for stateless auth",
		}),
	)
	if err != nil {
		connectErr := new(connect.Error)
		if !threadErrorAs(err, &connectErr) || connectErr.Code() != connect.CodeFailedPrecondition {
			t.Fatalf("expected FailedPrecondition, got %v", err)
		}
	}

	decisions, err := backend.DB().GetInitiativeDecisions("INIT-001")
	if err != nil {
		t.Fatalf("GetInitiativeDecisions: %v", err)
	}
	if len(decisions) != 0 {
		t.Fatalf("expected no initiative decisions, got %d", len(decisions))
	}
}

// ============================================================================
// SC-13: PromoteDecisionDraft rejects direct promotion
// ============================================================================

func TestThreadServer_PromoteDecisionDraft_Rejected(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	if err := backend.DB().SaveInitiative(&db.Initiative{
		ID:     "INIT-001",
		Title:  "Control Plane",
		Status: "active",
	}); err != nil {
		t.Fatalf("SaveInitiative: %v", err)
	}

	server := NewThreadServer(backend, publisher, slog.Default())

	thread := &db.Thread{
		Title:        "Decision draft thread",
		InitiativeID: "INIT-001",
	}
	if err := backend.DB().CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	draft := &db.ThreadDecisionDraft{
		ThreadID:     thread.ID,
		InitiativeID: "INIT-001",
		Decision:     "Keep recommendations human-gated",
		Rationale:    "Thread drafts should not create initiative history by themselves.",
	}
	if err := backend.DB().CreateThreadDecisionDraft(draft); err != nil {
		t.Fatalf("CreateThreadDecisionDraft: %v", err)
	}

	_, err := server.PromoteDecisionDraft(
		context.Background(),
		connect.NewRequest(&orcv1.PromoteThreadDecisionDraftRequest{
			ThreadId:   thread.ID,
			DraftId:    draft.ID,
			PromotedBy: "operator",
		}),
	)
	if err != nil {
		connectErr := new(connect.Error)
		if !threadErrorAs(err, &connectErr) || connectErr.Code() != connect.CodeFailedPrecondition {
			t.Fatalf("expected FailedPrecondition error, got: %v", err)
		}
	} else {
		t.Fatal("expected direct decision promotion to be rejected")
	}
}

func TestThreadServer_AddLinkAndPromoteRecommendationDraft(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mustCreateThreadServerFixtures(t, backend)

	server := NewThreadServer(backend, publisher, slog.Default())
	cache := NewProjectCache(1)
	cache.entries["proj-001"] = &cacheEntry{db: backend.DB(), backend: backend}
	cache.order = append(cache.order, "proj-001")
	server.SetProjectCache(cache)

	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			ProjectId: "proj-001",
			Title:     "Workspace thread",
			TaskId:    threadStringPtr("TASK-001"),
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id
	eventCh := publisher.Subscribe(threadID)

	_, err = server.AddLink(
		context.Background(),
		connect.NewRequest(&orcv1.AddThreadLinkRequest{
			ProjectId: "proj-001",
			ThreadId:  threadID,
			Link: &orcv1.ThreadLinkInput{
				LinkType: "diff",
				TargetId: "TASK-001:web/src/components/layout/DiscussionPanel.tsx",
				Title:    "DiscussionPanel diff",
			},
		}),
	)
	if err != nil {
		t.Fatalf("AddLink: %v", err)
	}

	createDraftResp, err := server.CreateRecommendationDraft(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRecommendationDraftRequest{
			ProjectId: "proj-001",
			ThreadId:  threadID,
			Draft: &orcv1.ThreadRecommendationDraft{
				Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP,
				Title:          "Follow up on workspace promotion",
				Summary:        "The thread workspace promotion path needs a regression test.",
				ProposedAction: "Add an API test for draft promotion.",
				Evidence:       "No API test covered this flow before.",
			},
		}),
	)
	if err != nil {
		t.Fatalf("CreateRecommendationDraft: %v", err)
	}
	if createDraftResp.Msg.Thread == nil || len(createDraftResp.Msg.Thread.RecommendationDrafts) != 1 {
		t.Fatalf("expected thread response with 1 recommendation draft")
	}

	promoteResp, err := server.PromoteRecommendationDraft(
		context.Background(),
		connect.NewRequest(&orcv1.PromoteThreadRecommendationDraftRequest{
			ProjectId:  "proj-001",
			ThreadId:   threadID,
			DraftId:    createDraftResp.Msg.Draft.Id,
			PromotedBy: "operator",
		}),
	)
	if err != nil {
		t.Fatalf("PromoteRecommendationDraft: %v", err)
	}
	if promoteResp.Msg.Recommendation == nil {
		t.Fatal("expected created recommendation")
	}
	if promoteResp.Msg.Recommendation.SourceThreadId != threadID {
		t.Fatalf("expected source thread %s, got %s", threadID, promoteResp.Msg.Recommendation.SourceThreadId)
	}
	if promoteResp.Msg.Draft.Status != db.ThreadDraftStatusPromoted {
		t.Fatalf("expected promoted draft status, got %s", promoteResp.Msg.Draft.Status)
	}

	getResp, err := server.GetThread(
		context.Background(),
		connect.NewRequest(&orcv1.GetThreadRequest{ThreadId: threadID}),
	)
	if err != nil {
		t.Fatalf("GetThread: %v", err)
	}
	if len(getResp.Msg.Thread.Links) != 3 {
		t.Fatalf("expected task, diff, and recommendation links; got %d", len(getResp.Msg.Thread.Links))
	}

	var updateTypes []string
	timeout := time.After(2 * time.Second)
	for len(updateTypes) < 3 {
		select {
		case evt := <-eventCh:
			if evt.Type != events.EventThreadUpdated {
				continue
			}
			data, ok := evt.Data.(events.ThreadUpdatedData)
			if !ok {
				t.Fatalf("expected ThreadUpdatedData, got %T", evt.Data)
			}
			if evt.ProjectID != "proj-001" {
				t.Fatalf("thread update project_id = %q, want proj-001", evt.ProjectID)
			}
			updateTypes = append(updateTypes, data.UpdateType)
		case <-timeout:
			t.Fatalf("timed out waiting for thread update events, got %v", updateTypes)
		}
	}

	expectedTypes := []string{
		"link_added",
		"recommendation_draft_created",
		"recommendation_draft_promoted",
	}
	if !reflect.DeepEqual(updateTypes, expectedTypes) {
		t.Fatalf("thread update types = %v, want %v", updateTypes, expectedTypes)
	}
}

func TestThreadServer_PromoteRecommendationDraft_DefaultsActorWhenClientOmitsIt(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mustCreateThreadServerFixtures(t, backend)
	server := NewThreadServer(backend, publisher, slog.Default())

	thread := &db.Thread{
		Title:  "Promotion without explicit actor",
		TaskID: "TASK-001",
	}
	if err := backend.DB().CreateThread(thread); err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	draft := &db.ThreadRecommendationDraft{
		ThreadID:       thread.ID,
		Kind:           db.RecommendationKindFollowUp,
		Title:          "Promote without actor",
		Summary:        "The server should attribute the promotion even when the browser omits a name.",
		ProposedAction: "Resolve the actor server-side.",
		Evidence:       "Frontend discussion flows do not carry an authenticated user name today.",
	}
	if err := backend.DB().CreateThreadRecommendationDraft(draft); err != nil {
		t.Fatalf("CreateThreadRecommendationDraft: %v", err)
	}

	resp, err := server.PromoteRecommendationDraft(
		context.Background(),
		connect.NewRequest(&orcv1.PromoteThreadRecommendationDraftRequest{
			ThreadId: thread.ID,
			DraftId:  draft.ID,
		}),
	)
	if err != nil {
		t.Fatalf("PromoteRecommendationDraft: %v", err)
	}
	if resp.Msg.Draft.GetPromotedBy() == "" {
		t.Fatal("expected server to fill promoted_by when omitted")
	}
}

func TestThreadServer_PromoteRecommendationDraft_FromGenericThread(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mustCreateThreadServerFixtures(t, backend)

	server := NewThreadServer(backend, publisher, slog.Default())
	cache := NewProjectCache(1)
	cache.entries["proj-001"] = &cacheEntry{db: backend.DB(), backend: backend}
	cache.order = append(cache.order, "proj-001")
	server.SetProjectCache(cache)

	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			ProjectId: "proj-001",
			Title:     "Default workspace thread",
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	createDraftResp, err := server.CreateRecommendationDraft(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRecommendationDraftRequest{
			ProjectId: "proj-001",
			ThreadId:  threadID,
			Draft: &orcv1.ThreadRecommendationDraft{
				Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP,
				Title:          "Promote from generic thread",
				Summary:        "Default threads should not need task provenance.",
				ProposedAction: "Persist thread-only recommendations.",
				Evidence:       "The thread came from the sidebar create flow.",
			},
		}),
	)
	if err != nil {
		t.Fatalf("CreateRecommendationDraft: %v", err)
	}

	promoteResp, err := server.PromoteRecommendationDraft(
		context.Background(),
		connect.NewRequest(&orcv1.PromoteThreadRecommendationDraftRequest{
			ProjectId:  "proj-001",
			ThreadId:   threadID,
			DraftId:    createDraftResp.Msg.Draft.Id,
			PromotedBy: "operator",
		}),
	)
	if err != nil {
		t.Fatalf("PromoteRecommendationDraft: %v", err)
	}
	if promoteResp.Msg.Recommendation == nil {
		t.Fatal("expected created recommendation")
	}
	if promoteResp.Msg.Recommendation.SourceThreadId != threadID {
		t.Fatalf("expected source thread %s, got %s", threadID, promoteResp.Msg.Recommendation.SourceThreadId)
	}
	if promoteResp.Msg.Recommendation.SourceTaskId != "" {
		t.Fatalf("expected empty source task, got %s", promoteResp.Msg.Recommendation.SourceTaskId)
	}
	if promoteResp.Msg.Recommendation.SourceRunId != "" {
		t.Fatalf("expected empty source run, got %s", promoteResp.Msg.Recommendation.SourceRunId)
	}
	if len(promoteResp.Msg.Thread.Links) != 1 {
		t.Fatalf("expected only recommendation link, got %d", len(promoteResp.Msg.Thread.Links))
	}
}

func TestThreadServer_PromoteRecommendationDraft_RejectsMismatchedThread(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mustCreateThreadServerFixtures(t, backend)
	server := NewThreadServer(backend, publisher, slog.Default())

	threadA := &db.Thread{Title: "Thread A", TaskID: "TASK-001"}
	threadB := &db.Thread{Title: "Thread B", TaskID: "TASK-001"}
	if err := backend.DB().CreateThread(threadA); err != nil {
		t.Fatalf("CreateThread(threadA): %v", err)
	}
	if err := backend.DB().CreateThread(threadB); err != nil {
		t.Fatalf("CreateThread(threadB): %v", err)
	}

	draft := &db.ThreadRecommendationDraft{
		ThreadID:       threadA.ID,
		Kind:           db.RecommendationKindFollowUp,
		Title:          "Thread A draft",
		Summary:        "Should stay attached to thread A.",
		ProposedAction: "Promote only from the owning thread.",
		Evidence:       "A mismatched request should fail loudly.",
	}
	if err := backend.DB().CreateThreadRecommendationDraft(draft); err != nil {
		t.Fatalf("CreateThreadRecommendationDraft: %v", err)
	}

	_, err := server.PromoteRecommendationDraft(
		context.Background(),
		connect.NewRequest(&orcv1.PromoteThreadRecommendationDraftRequest{
			ThreadId:   threadB.ID,
			DraftId:    draft.ID,
			PromotedBy: "operator",
		}),
	)
	if err == nil {
		t.Fatal("expected mismatched thread/draft promotion to fail")
	}

	connectErr := new(connect.Error)
	if !threadErrorAs(err, &connectErr) || connectErr.Code() != connect.CodeInvalidArgument {
		t.Fatalf("expected InvalidArgument error, got: %v", err)
	}

	recommendations, loadErr := backend.LoadAllRecommendations()
	if loadErr != nil {
		t.Fatalf("LoadAllRecommendations: %v", loadErr)
	}
	if len(recommendations) != 0 {
		t.Fatalf("expected no promoted recommendations, got %d", len(recommendations))
	}
}

func mustCreateThreadServerFixtures(t *testing.T, backend *storage.DatabaseBackend) {
	t.Helper()

	if err := backend.DB().SaveWorkflow(&db.Workflow{
		ID:   "wf-thread",
		Name: "Thread Workflow",
	}); err != nil {
		t.Fatalf("SaveWorkflow: %v", err)
	}
	if err := backend.DB().SaveTask(&db.Task{
		ID:         "TASK-001",
		Title:      "Thread Source Task",
		WorkflowID: "wf-thread",
		Status:     "running",
	}); err != nil {
		t.Fatalf("SaveTask: %v", err)
	}
	taskID := "TASK-001"
	if err := backend.DB().SaveWorkflowRun(&db.WorkflowRun{
		ID:          "RUN-001",
		WorkflowID:  "wf-thread",
		ContextType: "task",
		TaskID:      &taskID,
		Status:      "running",
	}); err != nil {
		t.Fatalf("SaveWorkflowRun: %v", err)
	}
}

// ============================================================================
// Failure mode: Concurrent SendMessage
// ============================================================================

func TestThreadServer_ConcurrentSend(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	mock := executor.NewMockTurnExecutor("Response")
	mock.Delay = 100 * time.Millisecond // Simulate some work

	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return mock
	})

	// Create thread
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "Concurrent test"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	// Send two messages concurrently
	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(n int) {
			_, err := server.SendMessage(
				context.Background(),
				connect.NewRequest(&orcv1.SendThreadMessageRequest{
					ThreadId: threadID,
					Content:  fmt.Sprintf("Message %d", n),
				}),
			)
			errCh <- err
		}(i)
	}

	// One should succeed, the other should get ResourceExhausted or both succeed sequentially
	var errs []error
	for i := 0; i < 2; i++ {
		errs = append(errs, <-errCh)
	}

	// At least one must succeed
	successCount := 0
	for _, err := range errs {
		if err == nil {
			successCount++
		}
	}
	if successCount == 0 {
		t.Error("expected at least one concurrent SendMessage to succeed")
	}
}

func TestThreadServer_ConcurrentSend_UsesFreshSessionState(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	var sessionIDsReceived []string
	mock := executor.NewMockTurnExecutor("Response")
	mock.Delay = 50 * time.Millisecond

	server := NewThreadServer(backend, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		sessionIDsReceived = append(sessionIDsReceived, sessionID)
		if sessionID != "" {
			mock.UpdateSessionID(sessionID)
		}
		return mock
	})

	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{Title: "Concurrent session test"}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}
	threadID := createResp.Msg.Thread.Id

	errCh := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(n int) {
			_, sendErr := server.SendMessage(
				context.Background(),
				connect.NewRequest(&orcv1.SendThreadMessageRequest{
					ThreadId: threadID,
					Content:  fmt.Sprintf("Message %d", n),
				}),
			)
			errCh <- sendErr
		}(i)
	}

	for i := 0; i < 2; i++ {
		if sendErr := <-errCh; sendErr != nil {
			t.Fatalf("SendMessage[%d]: %v", i, sendErr)
		}
	}

	if len(sessionIDsReceived) != 2 {
		t.Fatalf("expected 2 factory calls, got %d", len(sessionIDsReceived))
	}
	if sessionIDsReceived[0] != "" {
		t.Fatalf("expected first send to start a new session, got %q", sessionIDsReceived[0])
	}
	if sessionIDsReceived[1] == "" {
		t.Fatal("expected second send to observe the session ID written by the first send")
	}
}

func TestThreadServer_ConcurrentSend_IsScopedByProjectAndThread(t *testing.T) {
	t.Parallel()

	backendA := storage.NewTestBackend(t)
	backendB := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	exec := &parallelSendMockExecutor{delay: 100 * time.Millisecond}
	server := NewThreadServer(backendA, publisher, slog.Default())
	server.SetTurnExecutorFactory(func(sessionID string) executor.TurnExecutor {
		return exec
	})

	cache := NewProjectCache(2)
	cache.entries["proj-a"] = &cacheEntry{db: backendA.DB(), backend: backendA}
	cache.entries["proj-b"] = &cacheEntry{db: backendB.DB(), backend: backendB}
	cache.order = append(cache.order, "proj-a", "proj-b")
	server.SetProjectCache(cache)

	createThread := func(projectID string) string {
		resp, err := server.CreateThread(
			context.Background(),
			connect.NewRequest(&orcv1.CreateThreadRequest{
				ProjectId: projectID,
				Title:     "Shared thread id",
			}),
		)
		if err != nil {
			t.Fatalf("CreateThread(%s): %v", projectID, err)
		}
		return resp.Msg.Thread.Id
	}

	threadIDA := createThread("proj-a")
	threadIDB := createThread("proj-b")
	if threadIDA != threadIDB {
		t.Fatalf("expected matching local thread IDs for isolation test, got %s and %s", threadIDA, threadIDB)
	}

	errCh := make(chan error, 2)
	for _, projectID := range []string{"proj-a", "proj-b"} {
		go func(projectID string) {
			_, err := server.SendMessage(
				context.Background(),
				connect.NewRequest(&orcv1.SendThreadMessageRequest{
					ProjectId: projectID,
					ThreadId:  threadIDA,
					Content:   "hello",
				}),
			)
			errCh <- err
		}(projectID)
	}

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("SendMessage[%d]: %v", i, err)
		}
	}
	if exec.maxConcurrent() != 2 {
		t.Fatalf("expected project-scoped sends to execute concurrently, max concurrency was %d", exec.maxConcurrent())
	}
}

// ============================================================================
// Helpers
// ============================================================================

type parallelSendMockExecutor struct {
	delay     time.Duration
	mu        sync.Mutex
	active    int
	maxActive int
	sessionID string
}

func (m *parallelSendMockExecutor) ExecuteTurn(ctx context.Context, prompt string) (*executor.TurnResult, error) {
	m.mu.Lock()
	m.active++
	if m.active > m.maxActive {
		m.maxActive = m.active
	}
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.active--
		m.mu.Unlock()
	}()

	select {
	case <-time.After(m.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return &executor.TurnResult{
		Content:   "parallel response",
		NumTurns:  1,
		SessionID: "parallel-session",
	}, nil
}

func (m *parallelSendMockExecutor) ExecuteTurnWithoutSchema(ctx context.Context, prompt string) (*executor.TurnResult, error) {
	return m.ExecuteTurn(ctx, prompt)
}

func (m *parallelSendMockExecutor) UpdateSessionID(id string) {
	m.sessionID = id
}

func (m *parallelSendMockExecutor) SessionID() string {
	return m.sessionID
}

func (m *parallelSendMockExecutor) maxConcurrent() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.maxActive
}

// threadStringPtr avoids collision with stringPtr in other test files.
func threadStringPtr(s string) *string {
	return &s
}

// threadErrorAs checks if err is a *connect.Error and sets target.
func threadErrorAs(err error, target **connect.Error) bool {
	if err == nil {
		return false
	}
	connectErr, ok := err.(*connect.Error)
	if ok {
		*target = connectErr
		return true
	}
	return false
}

func threadFilterEvents(evts []events.Event, eventType events.EventType) []events.Event {
	var result []events.Event
	for _, evt := range evts {
		if evt.Type == eventType {
			result = append(result, evt)
		}
	}
	return result
}
