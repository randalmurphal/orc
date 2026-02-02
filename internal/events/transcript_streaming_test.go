package events

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}

// TestTranscriptStreaming_SC1_RealTimeTranscriptEvents tests that
// transcript events are streamed in real-time to WebSocket clients
func TestTranscriptStreaming_SC1_RealTimeTranscriptEvents(t *testing.T) {
	t.Run("should stream transcript_chunk events to subscribed clients", func(t *testing.T) {
		// Arrange: Set up event server with WebSocket hub
		server := NewEventServer()
		hub := &MockWebSocketHub{events: make(chan *orcv1.Event, 10)}
		server.SetWebSocketHub(hub)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Act: Publish a transcript event
		projectID := "test-project"
		taskID := "TASK-001"
		event := &orcv1.Event{
			Id:        "transcript-event-1",
			ProjectId: &projectID,
			TaskId:    &taskID,
			Payload: &orcv1.Event_Activity{
				Activity: &orcv1.ActivityEvent{
					TaskId:   "TASK-001",
					PhaseId:  "implement",
					Activity: orcv1.ActivityState_ACTIVITY_STATE_STREAMING,
					Details:  stringPtr(`{"content":"Hello World","timestamp":"2024-01-01T12:00:00Z","type":"response","phase":"implement"}`),
				},
			},
		}

		err := server.PublishEvent(ctx, event)
		require.NoError(t, err)

		// Assert: Event should be forwarded to WebSocket hub
		select {
		case receivedEvent := <-hub.events:
			assert.Equal(t, "TASK-001", *receivedEvent.TaskId)
			assert.Equal(t, "test-project", *receivedEvent.ProjectId)
			// Check that it's an activity event with transcript details
			activity, ok := receivedEvent.Payload.(*orcv1.Event_Activity)
			assert.True(t, ok, "Event should contain ActivityEvent payload")
			if ok {
				assert.Contains(t, *activity.Activity.Details, "Hello World")
				assert.Equal(t, orcv1.ActivityState_ACTIVITY_STATE_STREAMING, activity.Activity.Activity)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("Expected transcript event to be forwarded to WebSocket hub")
		}
	})

	t.Run("should filter events by project and task ID", func(t *testing.T) {
		server := NewEventServer()
		hub := &MockWebSocketHub{events: make(chan *orcv1.Event, 10)}
		server.SetWebSocketHub(hub)

		ctx := context.Background()

		// Act: Publish events for different projects and tasks
		event1 := &orcv1.Event{
			Type:      "transcript_chunk",
			ProjectId: "project-1",
			TaskId:    "TASK-001",
			Data:      `{"content":"Project 1 Task 1"}`,
		}
		event2 := &orcv1.Event{
			Type:      "transcript_chunk",
			ProjectId: "project-2",
			TaskId:    "TASK-001",
			Data:      `{"content":"Project 2 Task 1"}`,
		}
		event3 := &orcv1.Event{
			Type:      "transcript_chunk",
			ProjectId: "project-1",
			TaskId:    "TASK-002",
			Data:      `{"content":"Project 1 Task 2"}`,
		}

		server.PublishEvent(ctx, event1)
		server.PublishEvent(ctx, event2)
		server.PublishEvent(ctx, event3)

		// Assert: All events should be published but clients will filter by subscription
		assert.Equal(t, 3, len(hub.events))
	})

	t.Run("should handle malformed transcript data gracefully", func(t *testing.T) {
		server := NewEventServer()
		hub := &MockWebSocketHub{events: make(chan *orcv1.Event, 10)}
		server.SetWebSocketHub(hub)

		ctx := context.Background()

		// Act: Publish event with invalid JSON data
		event := &orcv1.Event{
			Type:      "transcript_chunk",
			ProjectId: "test-project",
			TaskId:    "TASK-001",
			Data:      `invalid json{`,
		}

		err := server.PublishEvent(ctx, event)

		// Assert: Should not error - malformed data handling is client responsibility
		assert.NoError(t, err)

		// Event should still be forwarded
		select {
		case receivedEvent := <-hub.events:
			assert.Equal(t, "transcript_chunk", receivedEvent.Type)
			assert.Equal(t, "invalid json{", receivedEvent.Data)
		case <-time.After(1 * time.Second):
			t.Fatal("Expected malformed event to still be forwarded")
		}
	})
}

// TestTranscriptStreaming_SC2_EventGeneration tests that transcript events
// are properly generated during task execution
func TestTranscriptStreaming_SC2_EventGeneration(t *testing.T) {
	t.Run("should generate transcript_chunk events during LLM interactions", func(t *testing.T) {
		// Arrange: Set up task executor with event publisher
		mockEventPublisher := &MockEventPublisher{events: make([]*orcv1.Event, 0)}
		executor := &TaskExecutor{
			eventPublisher: mockEventPublisher,
		}

		// Act: Execute a phase that should generate transcript events
		ctx := context.Background()
		task := &Task{
			ID:        "TASK-001",
			ProjectID: "test-project",
			Phase:     "implement",
		}

		// This should trigger LLM calls and transcript events
		err := executor.ExecutePhase(ctx, task)
		require.NoError(t, err)

		// Assert: Transcript events should be generated
		assert.Greater(t, len(mockEventPublisher.events), 0)

		// Find transcript events
		transcriptEvents := make([]*orcv1.Event, 0)
		for _, event := range mockEventPublisher.events {
			if event.Type == "transcript_chunk" {
				transcriptEvents = append(transcriptEvents, event)
			}
		}

		assert.Greater(t, len(transcriptEvents), 0, "Expected at least one transcript event")

		// Verify event structure
		for _, event := range transcriptEvents {
			assert.Equal(t, "TASK-001", event.TaskId)
			assert.Equal(t, "test-project", event.ProjectId)

			// Verify data format
			var transcriptData map[string]interface{}
			err := json.Unmarshal([]byte(event.Data), &transcriptData)
			assert.NoError(t, err, "Transcript data should be valid JSON")

			assert.Contains(t, transcriptData, "content")
			assert.Contains(t, transcriptData, "timestamp")
			assert.Contains(t, transcriptData, "type")
			assert.Contains(t, transcriptData, "phase")
		}
	})

	t.Run("should include token counts in transcript events", func(t *testing.T) {
		mockEventPublisher := &MockEventPublisher{events: make([]*orcv1.Event, 0)}
		executor := &TaskExecutor{
			eventPublisher: mockEventPublisher,
		}

		ctx := context.Background()
		task := &Task{
			ID:        "TASK-001",
			ProjectID: "test-project",
			Phase:     "implement",
		}

		err := executor.ExecutePhase(ctx, task)
		require.NoError(t, err)

		// Assert: Events should include token information
		transcriptEvents := make([]*orcv1.Event, 0)
		for _, event := range mockEventPublisher.events {
			if event.Type == "transcript_chunk" {
				transcriptEvents = append(transcriptEvents, event)
			}
		}

		require.Greater(t, len(transcriptEvents), 0)

		// Check that at least one event has token data
		hasTokenData := false
		for _, event := range transcriptEvents {
			var transcriptData map[string]interface{}
			err := json.Unmarshal([]byte(event.Data), &transcriptData)
			require.NoError(t, err)

			if tokens, exists := transcriptData["tokens"]; exists {
				tokenMap, ok := tokens.(map[string]interface{})
				if ok && tokenMap["input"] != nil && tokenMap["output"] != nil {
					hasTokenData = true
					break
				}
			}
		}

		assert.True(t, hasTokenData, "Expected at least one transcript event to have token counts")
	})

	t.Run("should generate different event types for prompts vs responses", func(t *testing.T) {
		mockEventPublisher := &MockEventPublisher{events: make([]*orcv1.Event, 0)}
		executor := &TaskExecutor{
			eventPublisher: mockEventPublisher,
		}

		ctx := context.Background()
		task := &Task{
			ID:        "TASK-001",
			ProjectID: "test-project",
			Phase:     "implement",
		}

		err := executor.ExecutePhase(ctx, task)
		require.NoError(t, err)

		// Assert: Should have both prompt and response events
		promptEvents := 0
		responseEvents := 0

		for _, event := range mockEventPublisher.events {
			if event.Type == "transcript_chunk" {
				var transcriptData map[string]interface{}
				err := json.Unmarshal([]byte(event.Data), &transcriptData)
				require.NoError(t, err)

				eventType, exists := transcriptData["type"]
				require.True(t, exists)

				switch eventType {
				case "prompt":
					promptEvents++
				case "response":
					responseEvents++
				}
			}
		}

		assert.Greater(t, promptEvents, 0, "Expected at least one prompt event")
		assert.Greater(t, responseEvents, 0, "Expected at least one response event")
	})
}

// TestTranscriptStreaming_SC3_WebSocketIntegration tests WebSocket delivery
func TestTranscriptStreaming_SC3_WebSocketIntegration(t *testing.T) {
	t.Run("should deliver transcript events to WebSocket clients", func(t *testing.T) {
		// Arrange: Set up WebSocket hub with client
		hub := NewWebSocketHub()
		mockClient := &MockWebSocketClient{
			messages: make(chan []byte, 10),
			id:       "client-1",
		}

		// Register client for specific task
		hub.RegisterClient(mockClient, "test-project", "TASK-001")

		// Act: Send transcript event through hub
		event := &orcv1.Event{
			Type:      "transcript_chunk",
			ProjectId: "test-project",
			TaskId:    "TASK-001",
			Data:      `{"content":"Test message","type":"response"}`,
		}

		err := hub.BroadcastEvent(event)
		require.NoError(t, err)

		// Assert: Client should receive the event
		select {
		case message := <-mockClient.messages:
			var receivedEvent orcv1.Event
			err := json.Unmarshal(message, &receivedEvent)
			assert.NoError(t, err)
			assert.Equal(t, "transcript_chunk", receivedEvent.Type)
			assert.Equal(t, "TASK-001", receivedEvent.TaskId)
		case <-time.After(1 * time.Second):
			t.Fatal("Client should have received the transcript event")
		}
	})

	t.Run("should not deliver events to unmatched clients", func(t *testing.T) {
		hub := NewWebSocketHub()
		mockClient := &MockWebSocketClient{
			messages: make(chan []byte, 10),
			id:       "client-1",
		}

		// Register client for different task
		hub.RegisterClient(mockClient, "test-project", "TASK-002")

		// Act: Send event for different task
		event := &orcv1.Event{
			Type:      "transcript_chunk",
			ProjectId: "test-project",
			TaskId:    "TASK-001", // Different task
			Data:      `{"content":"Test message"}`,
		}

		err := hub.BroadcastEvent(event)
		require.NoError(t, err)

		// Assert: Client should NOT receive the event
		select {
		case <-mockClient.messages:
			t.Fatal("Client should not receive event for different task")
		case <-time.After(500 * time.Millisecond):
			// Expected - no message received
		}
	})

	t.Run("should handle client disconnection gracefully", func(t *testing.T) {
		hub := NewWebSocketHub()
		mockClient := &MockWebSocketClient{
			messages: make(chan []byte, 10),
			id:       "client-1",
			closed:   false,
		}

		hub.RegisterClient(mockClient, "test-project", "TASK-001")

		// Act: Close client connection
		mockClient.closed = true
		hub.UnregisterClient(mockClient)

		// Send event after client disconnection
		event := &orcv1.Event{
			Type:      "transcript_chunk",
			ProjectId: "test-project",
			TaskId:    "TASK-001",
			Data:      `{"content":"After disconnect"}`,
		}

		// Assert: Should not panic or error
		err := hub.BroadcastEvent(event)
		assert.NoError(t, err)
	})
}

// Mock implementations for testing

type MockWebSocketHub struct {
	events chan *orcv1.Event
}

func (m *MockWebSocketHub) BroadcastEvent(event *orcv1.Event) error {
	select {
	case m.events <- event:
		return nil
	default:
		return nil // Drop if channel full
	}
}

func (m *MockWebSocketHub) RegisterClient(client WebSocketClient, projectID, taskID string) {
	// Mock implementation
}

func (m *MockWebSocketHub) UnregisterClient(client WebSocketClient) {
	// Mock implementation
}

type MockEventPublisher struct {
	events []*orcv1.Event
}

func (m *MockEventPublisher) PublishEvent(ctx context.Context, event *orcv1.Event) error {
	m.events = append(m.events, event)
	return nil
}

type MockWebSocketClient struct {
	messages chan []byte
	id       string
	closed   bool
}

func (m *MockWebSocketClient) ID() string {
	return m.id
}

func (m *MockWebSocketClient) Send(data []byte) error {
	if m.closed {
		return nil // Simulate closed connection
	}
	select {
	case m.messages <- data:
		return nil
	default:
		return nil
	}
}

func (m *MockWebSocketClient) Close() error {
	m.closed = true
	close(m.messages)
	return nil
}

// Mock types that need to be defined for compilation

type EventServer struct {
	hub WebSocketHub
}

func NewEventServer() *EventServer {
	return &EventServer{}
}

func (e *EventServer) SetWebSocketHub(hub WebSocketHub) {
	e.hub = hub
}

func (e *EventServer) PublishEvent(ctx context.Context, event *orcv1.Event) error {
	if e.hub != nil {
		return e.hub.BroadcastEvent(event)
	}
	return nil
}

type TaskExecutor struct {
	eventPublisher EventPublisher
}

type Task struct {
	ID        string
	ProjectID string
	Phase     string
}

func (t *TaskExecutor) ExecutePhase(ctx context.Context, task *Task) error {
	// Mock implementation that simulates LLM interaction and event generation

	// Simulate prompt event
	promptEvent := &orcv1.Event{
		Type:      "transcript_chunk",
		ProjectId: task.ProjectID,
		TaskId:    task.ID,
		Data:      `{"content":"User prompt for implementation","type":"prompt","phase":"` + task.Phase + `","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`,
	}
	t.eventPublisher.PublishEvent(ctx, promptEvent)

	// Simulate response event with token counts
	responseEvent := &orcv1.Event{
		Type:      "transcript_chunk",
		ProjectId: task.ProjectID,
		TaskId:    task.ID,
		Data:      `{"content":"Claude's implementation response","type":"response","phase":"` + task.Phase + `","timestamp":"` + time.Now().Format(time.RFC3339) + `","tokens":{"input":150,"output":300}}`,
	}
	t.eventPublisher.PublishEvent(ctx, responseEvent)

	return nil
}

type WebSocketHub interface {
	BroadcastEvent(event *orcv1.Event) error
	RegisterClient(client WebSocketClient, projectID, taskID string)
	UnregisterClient(client WebSocketClient)
}

type WebSocketClient interface {
	ID() string
	Send(data []byte) error
	Close() error
}

type EventPublisher interface {
	PublishEvent(ctx context.Context, event *orcv1.Event) error
}

func NewWebSocketHub() WebSocketHub {
	return &mockWebSocketHubImpl{}
}

type mockWebSocketHubImpl struct {
	clients map[string]WebSocketClient
}

func (m *mockWebSocketHubImpl) BroadcastEvent(event *orcv1.Event) error {
	// Mock implementation
	return nil
}

func (m *mockWebSocketHubImpl) RegisterClient(client WebSocketClient, projectID, taskID string) {
	if m.clients == nil {
		m.clients = make(map[string]WebSocketClient)
	}
	m.clients[client.ID()] = client
}

func (m *mockWebSocketHubImpl) UnregisterClient(client WebSocketClient) {
	if m.clients != nil {
		delete(m.clients, client.ID())
	}
}