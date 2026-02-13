package api

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
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
// SC-12: RecordDecision writes to initiative_decisions
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

	// Record a decision
	_, err = server.RecordDecision(
		context.Background(),
		connect.NewRequest(&orcv1.RecordThreadDecisionRequest{
			ThreadId:  threadID,
			Decision:  "Use JWT auth",
			Rationale: "Industry standard for stateless auth",
		}),
	)
	if err != nil {
		t.Fatalf("RecordDecision: %v", err)
	}

	// Verify decision is in the initiative_decisions table
	decisions, err := backend.DB().GetInitiativeDecisions("INIT-001")
	if err != nil {
		t.Fatalf("GetInitiativeDecisions: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0].Decision != "Use JWT auth" {
		t.Errorf("expected decision 'Use JWT auth', got %q", decisions[0].Decision)
	}
	if decisions[0].Rationale != "Industry standard for stateless auth" {
		t.Errorf("expected rationale 'Industry standard for stateless auth', got %q",
			decisions[0].Rationale)
	}
}

// ============================================================================
// SC-13: RecordDecision errors if thread has no linked initiative
// ============================================================================

func TestThreadServer_RecordDecision_NoInitiative(t *testing.T) {
	t.Parallel()
	backend := storage.NewTestBackend(t)
	publisher := events.NewMemoryPublisher()
	defer publisher.Close()

	server := NewThreadServer(backend, publisher, slog.Default())

	// Create thread WITHOUT initiative link
	createResp, err := server.CreateThread(
		context.Background(),
		connect.NewRequest(&orcv1.CreateThreadRequest{
			Title: "No initiative thread",
		}),
	)
	if err != nil {
		t.Fatalf("CreateThread: %v", err)
	}

	_, err = server.RecordDecision(
		context.Background(),
		connect.NewRequest(&orcv1.RecordThreadDecisionRequest{
			ThreadId:  createResp.Msg.Thread.Id,
			Decision:  "Some decision",
			Rationale: "Some reason",
		}),
	)
	if err == nil {
		t.Fatal("expected error when thread has no initiative, got nil")
	}

	connectErr := new(connect.Error)
	if !threadErrorAs(err, &connectErr) || connectErr.Code() != connect.CodeFailedPrecondition {
		t.Errorf("expected FailedPrecondition error, got: %v", err)
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

// ============================================================================
// Helpers
// ============================================================================

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
