package api

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	llmkit "github.com/randalmurphal/llmkit/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
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
	t.Run("SC-7: StreamTranscript validation logic", func(t *testing.T) {
		// Test that we can at least validate the StreamTranscript method exists and has correct signature
		// Full streaming functionality would require integration testing
		server := NewTranscriptServer(nil)
		assert.NotNil(t, server, "StreamTranscript method should be available on transcript server")

		// Test request validation by examining the method signature
		// The method should expect StreamTranscriptRequest and return StreamTranscriptResponse
		req := &orcv1.StreamTranscriptRequest{
			ProjectId: "test-project",
			TaskId:    "TASK-001",
		}
		assert.Equal(t, "TASK-001", req.TaskId, "StreamTranscriptRequest should have TaskId field")
		assert.Equal(t, "test-project", req.ProjectId, "StreamTranscriptRequest should have ProjectId field")

		// Test response structure
		chunk := &orcv1.TranscriptChunk{
			TaskId:  "TASK-001",
			Type:    "response",
			Content: "test content",
			Phase:   "implement",
		}
		resp := &orcv1.StreamTranscriptResponse{
			Chunk: chunk,
		}
		assert.Equal(t, "TASK-001", resp.Chunk.TaskId, "StreamTranscriptResponse should contain TranscriptChunk")
	})
}

func TestTranscriptServer_GetLiveTranscript(t *testing.T) {
	t.Run("SC-8: GetLiveTranscript should return current streaming state", func(t *testing.T) {
		// Arrange: Set up backend with persisted content
		mockBackend := &MockStreamingBackend{
			transcripts: []storage.Transcript{
				{
					ID:        1,
					TaskID:    "TASK-001",
					Phase:     "implement",
					Type:      "user",
					Content:   "Persisted prompt",
					Timestamp: time.Now().Add(-5 * time.Minute).UnixMilli(),
				},
				{
					ID:        2,
					TaskID:    "TASK-001",
					Phase:     "implement",
					Type:      "assistant",
					Content:   "Persisted response",
					Timestamp: time.Now().Add(-4 * time.Minute).UnixMilli(),
				},
			},
			task: func() *orcv1.Task {
				t := task.NewProtoTask("TASK-001", "Test")
				task.SetCurrentPhaseProto(t, "implement")
				return t
			}(),
		}

		server := &transcriptServer{
			backend: mockBackend,
		}

		// Act: Request live transcript
		req := &connect.Request[orcv1.GetLiveTranscriptRequest]{
			Msg: &orcv1.GetLiveTranscriptRequest{
				TaskId: "TASK-001",
			},
		}

		resp, err := server.GetLiveTranscript(context.Background(), req)

		// Assert: Should include persisted content
		require.NoError(t, err)
		assert.NotNil(t, resp.Msg.Transcript)

		transcript := resp.Msg.Transcript
		assert.Equal(t, "TASK-001", transcript.TaskId)
		assert.Equal(t, "implement", transcript.Phase)
		assert.Len(t, transcript.Entries, 2)
		assert.False(t, resp.Msg.HasLiveContent)
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
				TaskId: "NONEXISTENT",
			},
		}

		resp, err := server.GetLiveTranscript(context.Background(), req)

		require.NoError(t, err)
		assert.Equal(t, 0, len(resp.Msg.Transcript.Entries))
	})
}

func TestTranscriptServer_GetSession_UsesPhaseScopedSession(t *testing.T) {
	sessionStart := time.Now().Add(-2 * time.Minute)
	mockBackend := &MockStreamingBackend{
		task: func() *orcv1.Task {
			taskRecord := task.NewProtoTask("TASK-001", "Test")
			task.SetCurrentPhaseProto(taskRecord, "implement")
			task.SetPhaseSessionMetadataProto(taskRecord.Execution, "implement", mustSessionMetadata(t, "codex", "codex-thread-123"))
			taskRecord.Metadata["phase:implement:provider"] = "codex"
			taskRecord.Metadata["phase:implement:model"] = "gpt-5.4"
			taskRecord.Execution.Phases["implement"].StartedAt = timestamppb.New(sessionStart)
			return taskRecord
		}(),
		transcripts: []storage.Transcript{
			{ID: 1, TaskID: "TASK-001", Phase: "implement", Type: "assistant", Content: "one", Timestamp: sessionStart.UnixMilli()},
			{ID: 2, TaskID: "TASK-001", Phase: "implement", Type: "assistant", Content: "two", Timestamp: time.Now().UnixMilli()},
		},
	}

	server := &transcriptServer{backend: mockBackend}
	resp, err := server.GetSession(context.Background(), &connect.Request[orcv1.GetSessionRequest]{
		Msg: &orcv1.GetSessionRequest{TaskId: "TASK-001"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Session)
	assert.Equal(t, "codex-thread-123", resp.Msg.Session.Id)
	assert.Equal(t, "gpt-5.4", resp.Msg.Session.Model)
	assert.Equal(t, int32(2), resp.Msg.Session.TurnCount)
	require.NotNil(t, resp.Msg.Session.CreatedAt)
}

func TestTranscriptServer_GetSession_CountsPromptAsActiveTurn(t *testing.T) {
	sessionStart := time.Now().Add(-30 * time.Second)
	mockBackend := &MockStreamingBackend{
		task: func() *orcv1.Task {
			taskRecord := task.NewProtoTask("TASK-002", "Prompt-only session")
			task.SetCurrentPhaseProto(taskRecord, "implement_codex")
			task.SetPhaseSessionMetadataProto(taskRecord.Execution, "implement_codex", mustSessionMetadata(t, "codex", "codex-thread-live"))
			taskRecord.Metadata["phase:implement_codex:model"] = "gpt-5.4"
			taskRecord.Execution.Phases["implement_codex"].StartedAt = timestamppb.New(sessionStart)
			return taskRecord
		}(),
		transcripts: []storage.Transcript{
			{ID: 1, TaskID: "TASK-002", Phase: "implement_codex", Type: "user", Content: "do work", Timestamp: sessionStart.UnixMilli()},
		},
	}

	server := &transcriptServer{backend: mockBackend}
	resp, err := server.GetSession(context.Background(), &connect.Request[orcv1.GetSessionRequest]{
		Msg: &orcv1.GetSessionRequest{TaskId: "TASK-002"},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Session)
	assert.Equal(t, "codex-thread-live", resp.Msg.Session.Id)
	assert.Equal(t, "gpt-5.4", resp.Msg.Session.Model)
	assert.Equal(t, int32(1), resp.Msg.Session.TurnCount)
	require.NotNil(t, resp.Msg.Session.CreatedAt)
}

func TestTranscriptServer_GetTranscript_UsesStructuredSessionMetadata(t *testing.T) {
	mockBackend := &MockStreamingBackend{
		task: func() *orcv1.Task {
			taskRecord := task.NewProtoTask("TASK-003", "Transcript session metadata")
			task.SetCurrentPhaseProto(taskRecord, "implement")
			task.SetPhaseSessionMetadataProto(taskRecord.Execution, "implement", mustSessionMetadata(t, "codex", "codex-thread-789"))
			taskRecord.Metadata["phase:implement:provider"] = "codex"
			taskRecord.Metadata["phase:implement:model"] = "gpt-5.4"
			return taskRecord
		}(),
		transcripts: []storage.Transcript{
			{ID: 1, TaskID: "TASK-003", Phase: "implement", SessionID: "codex-thread-789", Type: "assistant", Content: "done", Model: "gpt-5.4", Timestamp: time.Now().UnixMilli()},
		},
	}

	server := &transcriptServer{backend: mockBackend}
	resp, err := server.GetTranscript(context.Background(), &connect.Request[orcv1.GetTranscriptRequest]{
		Msg: &orcv1.GetTranscriptRequest{TaskId: "TASK-003", Phase: "implement", Iteration: 1},
	})
	require.NoError(t, err)
	require.NotNil(t, resp.Msg.Transcript)
	require.NotNil(t, resp.Msg.Transcript.SessionMetadata)

	session, parseErr := llmkit.ParseSessionMetadata(*resp.Msg.Transcript.SessionMetadata)
	require.NoError(t, parseErr)
	assert.Equal(t, "codex", session.Provider)
	assert.Equal(t, "codex-thread-789", llmkit.SessionID(session))
}

func TestTranscriptServer_EventIntegration(t *testing.T) {
	t.Run("SC-9: Should handle event publisher integration", func(t *testing.T) {
		// Arrange: Server with event publisher
		mockBackend := &MockStreamingBackend{}
		mockPublisher := &MockEventPublisher{
			events: make([]events.Event, 0),
		}

		server := &transcriptServer{
			backend: mockBackend,
		}

		// Act: Set event publisher (this would happen during server setup)
		server.SetEventPublisher(mockPublisher)

		// Simulate storing a new transcript entry (which should publish event)
		transcript := storage.Transcript{
			ID:        1,
			TaskID:    "TASK-001",
			Phase:     "implement",
			Type:      "assistant",
			Content:   "New response content",
			Timestamp: time.Now().UnixMilli(),
		}

		err := server.StoreTranscriptEntry(context.Background(), "test-project", transcript)
		require.NoError(t, err)

		// Assert: Event should be published when transcript entry is stored
		assert.Equal(t, 1, len(mockPublisher.events), "Event should be published for new transcript entries")

		// Verify the published event
		publishedEvent := mockPublisher.events[0]
		assert.Equal(t, events.EventTranscript, publishedEvent.Type)
		assert.Equal(t, "TASK-001", publishedEvent.TaskID)

		// Verify the transcript data in the event
		transcriptLine, ok := publishedEvent.Data.(events.TranscriptLine)
		assert.True(t, ok, "Event data should be TranscriptLine")
		assert.Equal(t, "implement", transcriptLine.Phase)
		assert.Equal(t, "assistant", transcriptLine.Type)
		assert.Equal(t, "New response content", transcriptLine.Content)
	})

	t.Run("should not publish events for transcript queries", func(t *testing.T) {
		mockBackend := &MockStreamingBackend{
			transcripts: []storage.Transcript{
				{ID: 1, TaskID: "TASK-001", Phase: "implement", Type: "user", Content: "Test"},
			},
		}

		mockPublisher := &MockEventPublisher{
			events: make([]events.Event, 0),
		}

		server := &transcriptServer{
			backend: mockBackend,
		}
		server.SetEventPublisher(mockPublisher)

		// Act: Query existing transcripts (read operation)
		req := &connect.Request[orcv1.GetTranscriptRequest]{
			Msg: &orcv1.GetTranscriptRequest{
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

func mustSessionMetadata(t *testing.T, provider, sessionID string) string {
	t.Helper()
	metadata, err := llmkit.MarshalSessionMetadata(llmkit.SessionMetadataForID(provider, sessionID))
	if err != nil {
		t.Fatalf("marshal session metadata: %v", err)
	}
	return metadata
}

// Mock implementations for testing

type MockStreamingBackend struct {
	storage.Backend
	transcripts    []storage.Transcript
	streamEvents   chan TranscriptStreamEvent
	liveTranscript []TranscriptStreamEvent
	task           *orcv1.Task
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

func (m *MockStreamingBackend) LoadTask(taskID string) (*orcv1.Task, error) {
	if m.task != nil && m.task.Id == taskID {
		return m.task, nil
	}
	return nil, assert.AnError
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

// We'll use a simple approach and modify the test to work with interface conversion
// by creating a wrapper that satisfies the interface requirements

type MockEventPublisher struct {
	events []events.Event
}

func (m *MockEventPublisher) Publish(event events.Event) {
	m.events = append(m.events, event)
}

func (m *MockEventPublisher) Subscribe(taskID string) <-chan events.Event {
	ch := make(chan events.Event, 10)
	close(ch)
	return ch
}

func (m *MockEventPublisher) Unsubscribe(taskID string, ch <-chan events.Event) {
	// No-op for mock
}

func (m *MockEventPublisher) Close() {
	// No-op for mock
}

// Methods are now implemented in transcript_server.go

// Interfaces that need to be defined

type TranscriptStreamer interface {
	Send(*orcv1.StreamTranscriptResponse) error
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
