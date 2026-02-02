package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/randalmurphal/orc/internal/storage"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

// TestTranscriptStreaming_API tests the new streaming API endpoints for live output panel
//
// These tests cover the backend API layer for TASK-737: Implement live output panel for Task Detail
//
// Success Criteria Coverage:
// - SC-7: StreamTranscript RPC method for real-time transcript delivery
// - SC-8: WebSocket event integration with transcript server
// - SC-9: Proper error handling for streaming edge cases
// - SC-10: Integration with existing transcript storage

func TestTranscriptServer_StreamTranscript(t *testing.T) {
	t.Run("SC-7: StreamTranscript should deliver real-time transcript chunks", func(t *testing.T) {
		// Arrange: Set up transcript server with mock backend
		mockBackend := &MockStreamingBackend{
			transcripts: []storage.Transcript{},
			streamEvents: make(chan TranscriptStreamEvent, 10),
		}

		server := &transcriptServer{
			backend: mockBackend,
		}

		// Create mock stream
		mockStream := &MockTranscriptStream{
			sent: make([]*orcv1.StreamTranscriptResponse, 0),
		}

		// Act: Start streaming for a task
		req := &connect.Request[orcv1.StreamTranscriptRequest]{
			Msg: &orcv1.StreamTranscriptRequest{
				ProjectId: "test-project",
				TaskId:    "TASK-001",
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Start streaming in background
		streamErr := make(chan error, 1)
		go func() {
			err := server.StreamTranscript(ctx, req, mockStream)
			streamErr <- err
		}()

		// Emit some transcript events
		mockBackend.EmitTranscriptEvent(TranscriptStreamEvent{
			TaskID:    "TASK-001",
			ProjectID: "test-project",
			Content:   "Starting implementation...",
			Type:      "prompt",
			Phase:     "implement",
			Timestamp: time.Now(),
		})

		mockBackend.EmitTranscriptEvent(TranscriptStreamEvent{
			TaskID:    "TASK-001",
			ProjectID: "test-project",
			Content:   "I'll implement the feature...",
			Type:      "response",
			Phase:     "implement",
			Timestamp: time.Now(),
			Tokens: &TokenCount{
				Input:  150,
				Output: 300,
			},
		})

		// Wait a bit for events to be processed
		time.Sleep(500 * time.Millisecond)
		cancel() // Stop streaming

		// Wait for stream to complete
		err := <-streamErr
		assert.NoError(t, err, "StreamTranscript should complete without error")

		// Assert: Should have received transcript chunks
		assert.GreaterOrEqual(t, len(mockStream.sent), 2, "Should receive transcript events")

		// Verify first chunk (prompt)
		firstChunk := mockStream.sent[0]
		assert.Equal(t, "TASK-001", firstChunk.Chunk.TaskId)
		assert.Equal(t, "prompt", firstChunk.Chunk.Type)
		assert.Equal(t, "implement", firstChunk.Chunk.Phase)
		assert.Contains(t, firstChunk.Chunk.Content, "Starting implementation")

		// Verify second chunk (response with tokens)
		secondChunk := mockStream.sent[1]
		assert.Equal(t, "response", secondChunk.Chunk.Type)
		assert.Contains(t, secondChunk.Chunk.Content, "I'll implement")
		assert.NotNil(t, secondChunk.Chunk.Tokens)
		assert.Equal(t, int32(150), secondChunk.Chunk.Tokens.InputTokens)
		assert.Equal(t, int32(300), secondChunk.Chunk.Tokens.OutputTokens)
	})

	t.Run("should filter events by task ID", func(t *testing.T) {
		mockBackend := &MockStreamingBackend{
			transcripts: []storage.Transcript{},
			streamEvents: make(chan TranscriptStreamEvent, 10),
		}

		server := &transcriptServer{
			backend: mockBackend,
		}

		mockStream := &MockTranscriptStream{
			sent: make([]*orcv1.StreamTranscriptResponse, 0),
		}

		req := &connect.Request[orcv1.StreamTranscriptRequest]{
			Msg: &orcv1.StreamTranscriptRequest{
				ProjectId: "test-project",
				TaskId:    "TASK-001", // Subscribing to TASK-001
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		go func() {
			server.StreamTranscript(ctx, req, mockStream)
		}()

		// Emit events for different tasks
		mockBackend.EmitTranscriptEvent(TranscriptStreamEvent{
			TaskID:    "TASK-001",
			ProjectID: "test-project",
			Content:   "For task 1",
			Type:      "response",
		})

		mockBackend.EmitTranscriptEvent(TranscriptStreamEvent{
			TaskID:    "TASK-002", // Different task
			ProjectID: "test-project",
			Content:   "For task 2",
			Type:      "response",
		})

		time.Sleep(500 * time.Millisecond)
		cancel()

		// Assert: Should only receive events for subscribed task
		assert.Equal(t, 1, len(mockStream.sent))
		assert.Contains(t, mockStream.sent[0].Chunk.Content, "For task 1")
	})

	t.Run("should require valid task ID", func(t *testing.T) {
		server := &transcriptServer{
			backend: &MockStreamingBackend{},
		}

		mockStream := &MockTranscriptStream{}

		req := &connect.Request[orcv1.StreamTranscriptRequest]{
			Msg: &orcv1.StreamTranscriptRequest{
				ProjectId: "test-project",
				TaskId:    "", // Missing task ID
			},
		}

		err := server.StreamTranscript(context.Background(), req, mockStream)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "task_id is required")
	})

	t.Run("should handle stream context cancellation gracefully", func(t *testing.T) {
		mockBackend := &MockStreamingBackend{
			streamEvents: make(chan TranscriptStreamEvent, 10),
		}

		server := &transcriptServer{
			backend: mockBackend,
		}

		mockStream := &MockTranscriptStream{
			sent: make([]*orcv1.StreamTranscriptResponse, 0),
		}

		req := &connect.Request[orcv1.StreamTranscriptRequest]{
			Msg: &orcv1.StreamTranscriptRequest{
				ProjectId: "test-project",
				TaskId:    "TASK-001",
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		// Stream should terminate when context is cancelled
		err := server.StreamTranscript(ctx, req, mockStream)

		// Should either be no error (clean shutdown) or context.DeadlineExceeded
		if err != nil {
			assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
		}
	})
}

func TestTranscriptServer_GetLiveTranscript(t *testing.T) {
	t.Run("SC-8: GetLiveTranscript should return current streaming state", func(t *testing.T) {
		// Arrange: Set up backend with some persisted and streaming content
		mockBackend := &MockStreamingBackend{
			transcripts: []storage.Transcript{
				{
					ID:        "1",
					TaskID:    "TASK-001",
					Phase:     "implement",
					Type:      "user",
					Content:   "Persisted prompt",
					Timestamp: time.Now().Add(-5 * time.Minute).UnixMilli(),
				},
				{
					ID:        "2",
					TaskID:    "TASK-001",
					Phase:     "implement",
					Type:      "assistant",
					Content:   "Persisted response",
					Timestamp: time.Now().Add(-4 * time.Minute).UnixMilli(),
				},
			},
			liveTranscript: []TranscriptStreamEvent{
				{
					TaskID:    "TASK-001",
					ProjectID: "test-project",
					Content:   "Live streaming content",
					Type:      "response",
					Phase:     "implement",
					Timestamp: time.Now(),
				},
			},
		}

		server := &transcriptServer{
			backend: mockBackend,
		}

		// Act: Request live transcript
		req := &connect.Request[orcv1.GetLiveTranscriptRequest]{
			Msg: &orcv1.GetLiveTranscriptRequest{
				ProjectId: "test-project",
				TaskId:    "TASK-001",
			},
		}

		resp, err := server.GetLiveTranscript(context.Background(), req)

		// Assert: Should include both persisted and live content
		require.NoError(t, err)
		assert.NotNil(t, resp.Msg.Transcript)

		transcript := resp.Msg.Transcript
		assert.Equal(t, "TASK-001", transcript.TaskId)
		assert.Equal(t, "implement", transcript.Phase)

		// Should have persisted entries plus live entries
		assert.GreaterOrEqual(t, len(transcript.Entries), 3)

		// Check that live content is included
		hasLiveContent := false
		for _, entry := range transcript.Entries {
			if entry.Content == "Live streaming content" {
				hasLiveContent = true
				break
			}
		}
		assert.True(t, hasLiveContent, "Response should include live streaming content")
	})

	t.Run("should return empty transcript for non-existent task", func(t *testing.T) {
		mockBackend := &MockStreamingBackend{
			transcripts: []storage.Transcript{},
		}

		server := &transcriptServer{
			backend: mockBackend,
		}

		req := &connect.Request[orcv1.GetLiveTranscriptRequest]{
			Msg: &orcv1.GetLiveTranscriptRequest{
				ProjectId: "test-project",
				TaskId:    "NONEXISTENT",
			},
		}

		resp, err := server.GetLiveTranscript(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, 0, len(resp.Msg.Transcript.Entries))
	})
}

func TestTranscriptServer_EventIntegration(t *testing.T) {
	t.Run("SC-9: Should handle event publisher integration", func(t *testing.T) {
		// Arrange: Server with event publisher
		mockBackend := &MockStreamingBackend{}
		mockPublisher := &MockEventPublisher{
			events: make([]*orcv1.Event, 0),
		}

		server := &transcriptServer{
			backend: mockBackend,
		}

		// Act: Set event publisher (this would happen during server setup)
		server.SetEventPublisher(mockPublisher)

		// Simulate storing a new transcript entry (which should publish event)
		transcript := storage.Transcript{
			ID:        "new-1",
			TaskID:    "TASK-001",
			Phase:     "implement",
			Type:      "assistant",
			Content:   "New response content",
			Timestamp: time.Now().UnixMilli(),
		}

		err := server.StoreTranscriptEntry(context.Background(), "test-project", transcript)
		require.NoError(t, err)

		// Assert: Event should be published
		assert.Equal(t, 1, len(mockPublisher.events))

		event := mockPublisher.events[0]
		assert.Equal(t, "transcript_chunk", event.Type)
		assert.Equal(t, "TASK-001", event.TaskId)
		assert.Equal(t, "test-project", event.ProjectId)

		// Verify event data structure
		var eventData map[string]interface{}
		err = json.Unmarshal([]byte(event.Data), &eventData)
		require.NoError(t, err)

		assert.Equal(t, "New response content", eventData["content"])
		assert.Equal(t, "assistant", eventData["type"])
		assert.Equal(t, "implement", eventData["phase"])
		assert.NotNil(t, eventData["timestamp"])
	})

	t.Run("should not publish events for transcript queries", func(t *testing.T) {
		mockBackend := &MockStreamingBackend{
			transcripts: []storage.Transcript{
				{ID: "1", TaskID: "TASK-001", Type: "user", Content: "Test"},
			},
		}

		mockPublisher := &MockEventPublisher{
			events: make([]*orcv1.Event, 0),
		}

		server := &transcriptServer{
			backend: mockBackend,
		}
		server.SetEventPublisher(mockPublisher)

		// Act: Query existing transcripts (read operation)
		req := &connect.Request[orcv1.GetTranscriptRequest]{
			Msg: &orcv1.GetTranscriptRequest{
				ProjectId: "test-project",
				TaskId:    "TASK-001",
				Phase:     "implement",
				Iteration: 1,
			},
		}

		_, err := server.GetTranscript(context.Background(), req)
		require.NoError(t, err)

		// Assert: No events should be published for read operations
		assert.Equal(t, 0, len(mockPublisher.events))
	})
}

// Mock implementations for testing

type MockStreamingBackend struct {
	storage.Backend
	transcripts    []storage.Transcript
	streamEvents   chan TranscriptStreamEvent
	liveTranscript []TranscriptStreamEvent
}

func (m *MockStreamingBackend) GetTranscripts(taskID string) ([]storage.Transcript, error) {
	result := make([]storage.Transcript, 0)
	for _, t := range m.transcripts {
		if t.TaskID == taskID {
			result = append(result, t)
		}
	}
	return result, nil
}

func (m *MockStreamingBackend) EmitTranscriptEvent(event TranscriptStreamEvent) {
	select {
	case m.streamEvents <- event:
	default:
		// Drop if channel full
	}
}

func (m *MockStreamingBackend) GetLiveTranscript(taskID string) []TranscriptStreamEvent {
	result := make([]TranscriptStreamEvent, 0)
	for _, event := range m.liveTranscript {
		if event.TaskID == taskID {
			result = append(result, event)
		}
	}
	return result
}

func (m *MockStreamingBackend) SubscribeToTranscriptEvents(taskID string) <-chan TranscriptStreamEvent {
	return m.streamEvents
}

type MockTranscriptStream struct {
	sent []*orcv1.StreamTranscriptResponse
}

func (m *MockTranscriptStream) Send(resp *orcv1.StreamTranscriptResponse) error {
	m.sent = append(m.sent, resp)
	return nil
}

type MockEventPublisher struct {
	events []*orcv1.Event
}

func (m *MockEventPublisher) PublishEvent(ctx context.Context, event *orcv1.Event) error {
	m.events = append(m.events, event)
	return nil
}

// New types that need to be defined for the streaming functionality

type TranscriptStreamEvent struct {
	TaskID    string
	ProjectID string
	Content   string
	Type      string // "prompt", "response", "tool", "error"
	Phase     string
	Timestamp time.Time
	Tokens    *TokenCount
}

type TokenCount struct {
	Input  int32
	Output int32
}

// Methods that need to be added to transcriptServer

func (s *transcriptServer) StreamTranscript(
	ctx context.Context,
	req *connect.Request[orcv1.StreamTranscriptRequest],
	stream TranscriptStreamer,
) error {
	// This method needs to be implemented to satisfy SC-7
	// It should:
	// 1. Validate request parameters
	// 2. Subscribe to transcript events for the given task
	// 3. Stream transcript chunks to the client
	// 4. Handle context cancellation gracefully

	if req.Msg.TaskId == "" {
		return connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	// Implementation would go here...
	return errors.New("StreamTranscript not yet implemented")
}

func (s *transcriptServer) GetLiveTranscript(
	ctx context.Context,
	req *connect.Request[orcv1.GetLiveTranscriptRequest],
) (*connect.Response[orcv1.GetLiveTranscriptResponse], error) {
	// This method needs to be implemented to satisfy SC-8
	// It should:
	// 1. Get persisted transcript entries
	// 2. Get current live transcript events
	// 3. Merge them into a single response
	// 4. Return the combined transcript

	if req.Msg.TaskId == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("task_id is required"))
	}

	// Implementation would go here...
	return nil, errors.New("GetLiveTranscript not yet implemented")
}

func (s *transcriptServer) SetEventPublisher(publisher EventPublisher) {
	// This method needs to be implemented to satisfy SC-9
	// It should allow the server to publish transcript events

	// Implementation would go here...
}

func (s *transcriptServer) StoreTranscriptEntry(
	ctx context.Context,
	projectID string,
	transcript storage.Transcript,
) error {
	// This method needs to be implemented to satisfy SC-9
	// It should:
	// 1. Store the transcript entry in the backend
	// 2. Publish a transcript_chunk event for real-time subscribers

	// Implementation would go here...
	return errors.New("StoreTranscriptEntry not yet implemented")
}

// Interfaces that need to be defined

type TranscriptStreamer interface {
	Send(*orcv1.StreamTranscriptResponse) error
}

type EventPublisher interface {
	PublishEvent(ctx context.Context, event *orcv1.Event) error
}

// Proto message types that need to be defined in the .proto files

// These would need to be added to the transcript.proto file:
/*
message StreamTranscriptRequest {
	string project_id = 1;
	string task_id = 2;
	optional string phase = 3;
}

message StreamTranscriptResponse {
	TranscriptChunk chunk = 1;
}

message TranscriptChunk {
	string task_id = 1;
	string type = 2;  // "prompt", "response", "tool", "error"
	string content = 3;
	string phase = 4;
	google.protobuf.Timestamp timestamp = 5;
	optional TokenUsage tokens = 6;
}

message GetLiveTranscriptRequest {
	string project_id = 1;
	string task_id = 2;
	optional string phase = 3;
}

message GetLiveTranscriptResponse {
	Transcript transcript = 1;
	bool has_live_content = 2;
}
*/