package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/randalmurphal/orc/internal/events"
)

func TestWSHandler_Connect(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	// Create test server
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Should be able to send a message
	msg := WSMessage{Type: "ping"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Errorf("failed to send message: %v", err)
	}

	if handler.ConnectionCount() != 1 {
		t.Errorf("expected 1 connection, got %d", handler.ConnectionCount())
	}
}

func TestWSHandler_Subscribe(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Subscribe to task
	msg := WSMessage{Type: "subscribe", TaskID: "TASK-001"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read response
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["type"] != "subscribed" {
		t.Errorf("expected type 'subscribed', got %v", resp["type"])
	}
	if resp["task_id"] != "TASK-001" {
		t.Errorf("expected task_id 'TASK-001', got %v", resp["task_id"])
	}
}

func TestWSHandler_ReceiveEvents(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Subscribe to task
	msg := WSMessage{Type: "subscribe", TaskID: "TASK-001"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read subscription confirmation
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read subscription response: %v", err)
	}

	// Publish an event
	pub.Publish(events.NewEvent(events.EventState, "TASK-001", map[string]string{"status": "running"}))

	// Give time for event to be forwarded
	time.Sleep(100 * time.Millisecond)

	// Read event
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read event: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to parse event: %v", err)
	}

	if resp["type"] != "event" {
		t.Errorf("expected type 'event', got %v", resp["type"])
	}
	if resp["event"] != "state" {
		t.Errorf("expected event 'state', got %v", resp["event"])
	}
}

func TestWSHandler_InvalidMessage(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Send invalid JSON
	if err := ws.WriteMessage(websocket.TextMessage, []byte("not json")); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Should receive error response
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["type"] != "error" {
		t.Errorf("expected type 'error', got %v", resp["type"])
	}
}

func TestWSHandler_SubscribeWithoutTaskID(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Subscribe without task ID
	msg := WSMessage{Type: "subscribe"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Should receive error
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["type"] != "error" {
		t.Errorf("expected type 'error', got %v", resp["type"])
	}
}

func TestWSHandler_UnknownMessageType(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Send unknown message type
	msg := WSMessage{Type: "unknown_type"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Should receive error
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["type"] != "error" {
		t.Errorf("expected type 'error', got %v", resp["type"])
	}
}

func TestWSHandler_MultipleConnections(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Connect multiple clients
	var conns []*websocket.Conn
	for i := 0; i < 3; i++ {
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatalf("failed to connect client %d: %v", i, err)
		}
		conns = append(conns, ws)
	}

	defer func() {
		for _, ws := range conns {
			_ = ws.Close()
		}
	}()

	// Allow connections to register
	time.Sleep(50 * time.Millisecond)

	if handler.ConnectionCount() != 3 {
		t.Errorf("expected 3 connections, got %d", handler.ConnectionCount())
	}

	// Close one connection
	_ = conns[0].Close()
	time.Sleep(100 * time.Millisecond)

	if handler.ConnectionCount() != 2 {
		t.Errorf("expected 2 connections after close, got %d", handler.ConnectionCount())
	}
}

func TestWSHandler_Broadcast(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Connect and subscribe
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	msg := WSMessage{Type: "subscribe", TaskID: "TASK-001"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read subscription confirmation
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = ws.ReadMessage()

	// Broadcast event
	event := events.NewEvent(events.EventPhase, "TASK-001", events.PhaseUpdate{
		Phase:  "test",
		Status: "completed",
	})
	handler.Broadcast("TASK-001", event)

	// Should receive broadcast
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read broadcast: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to parse broadcast: %v", err)
	}

	if resp["type"] != "event" {
		t.Errorf("expected type 'event', got %v", resp["type"])
	}
}

func TestWSHandler_Close(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Connect
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Allow connection to register
	time.Sleep(50 * time.Millisecond)

	if handler.ConnectionCount() != 1 {
		t.Errorf("expected 1 connection, got %d", handler.ConnectionCount())
	}

	// Close handler
	handler.Close()

	time.Sleep(100 * time.Millisecond)

	if handler.ConnectionCount() != 0 {
		t.Errorf("expected 0 connections after close, got %d", handler.ConnectionCount())
	}
}

func TestWSHandler_CORSUpgrader(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	// Create test server
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Try connecting with Origin header
	dialer := websocket.Dialer{}
	header := http.Header{}
	header.Set("Origin", "http://different-origin.com")

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("failed to connect with different origin: %v", err)
	}
	_ = ws.Close()
}

func TestWSHandler_GlobalSubscription(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Subscribe globally (using "*")
	msg := WSMessage{Type: "subscribe", TaskID: events.GlobalTaskID}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read subscription confirmation
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read subscription response: %v", err)
	}

	var resp map[string]any
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["type"] != "subscribed" {
		t.Errorf("expected type 'subscribed', got %v", resp["type"])
	}
	if resp["task_id"] != "*" {
		t.Errorf("expected task_id '*', got %v", resp["task_id"])
	}
}

func TestWSHandler_GlobalSubscription_InitialSessionUpdate(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Subscribe globally
	msg := WSMessage{Type: "subscribe", TaskID: events.GlobalTaskID}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read subscription confirmation
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read subscription response: %v", err)
	}

	// Read initial session_update event
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial session_update: %v", err)
	}

	var sessionEvent map[string]any
	if err := json.Unmarshal(data, &sessionEvent); err != nil {
		t.Fatalf("failed to parse session_update: %v", err)
	}

	// Verify it's a session_update event
	if sessionEvent["type"] != "event" {
		t.Errorf("expected type 'event', got %v", sessionEvent["type"])
	}
	if sessionEvent["event"] != string(events.EventSessionUpdate) {
		t.Errorf("expected event '%s', got %v", events.EventSessionUpdate, sessionEvent["event"])
	}
	if sessionEvent["task_id"] != events.GlobalTaskID {
		t.Errorf("expected task_id '%s', got %v", events.GlobalTaskID, sessionEvent["task_id"])
	}

	// Verify session_update data contains expected fields
	sessionData, ok := sessionEvent["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data to be map, got %T", sessionEvent["data"])
	}

	// Check that all required fields are present
	requiredFields := []string{"duration_seconds", "total_tokens", "estimated_cost_usd",
		"input_tokens", "output_tokens", "tasks_running", "is_paused"}
	for _, field := range requiredFields {
		if _, exists := sessionData[field]; !exists {
			t.Errorf("session_update missing required field: %s", field)
		}
	}
}

func TestWSHandler_GlobalSubscription_ReceivesAllTaskEvents(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Subscribe globally
	msg := WSMessage{Type: "subscribe", TaskID: events.GlobalTaskID}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read subscription confirmation
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read subscription response: %v", err)
	}

	// Read initial session_update event (sent on global subscription)
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial session_update: %v", err)
	}

	// Publish events for different tasks
	pub.Publish(events.NewEvent(events.EventState, "TASK-001", map[string]string{"status": "running"}))
	pub.Publish(events.NewEvent(events.EventState, "TASK-002", map[string]string{"status": "completed"}))

	// Give time for events to be forwarded
	time.Sleep(100 * time.Millisecond)

	// Should receive both events
	receivedTasks := make(map[string]bool)
	for i := 0; i < 2; i++ {
		_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read event %d: %v", i+1, err)
		}

		var resp map[string]any
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("failed to parse event %d: %v", i+1, err)
		}

		if resp["type"] != "event" {
			t.Errorf("expected type 'event', got %v", resp["type"])
		}
		taskID := resp["task_id"].(string)
		receivedTasks[taskID] = true
	}

	if !receivedTasks["TASK-001"] {
		t.Error("expected to receive event for TASK-001")
	}
	if !receivedTasks["TASK-002"] {
		t.Error("expected to receive event for TASK-002")
	}
}

func TestWSHandler_GlobalSubscription_FileWatcherEvents(t *testing.T) {
	pub := events.NewMemoryPublisher()
	server := &Server{runningTasks: make(map[string]context.CancelFunc)}
	handler := NewWSHandler(pub, server, nil)

	ts := httptest.NewServer(handler)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer func() { _ = ws.Close() }()

	// Subscribe globally
	msg := WSMessage{Type: "subscribe", TaskID: events.GlobalTaskID}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read subscription confirmation
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read subscription response: %v", err)
	}

	// Read initial session_update event (sent on global subscription)
	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial session_update: %v", err)
	}

	// Simulate file watcher events (task_created, task_updated, task_deleted)
	testCases := []struct {
		eventType events.EventType
		taskID    string
		data      map[string]any
	}{
		{
			eventType: events.EventTaskCreated,
			taskID:    "TASK-NEW",
			data:      map[string]any{"task": map[string]string{"id": "TASK-NEW", "title": "New Task"}},
		},
		{
			eventType: events.EventTaskUpdated,
			taskID:    "TASK-001",
			data:      map[string]any{"task": map[string]string{"id": "TASK-001", "status": "running"}},
		},
		{
			eventType: events.EventTaskDeleted,
			taskID:    "TASK-OLD",
			data:      map[string]any{"task_id": "TASK-OLD"},
		},
	}

	// Publish all events
	for _, tc := range testCases {
		pub.Publish(events.NewEvent(tc.eventType, tc.taskID, tc.data))
	}

	// Give time for events to be forwarded
	time.Sleep(100 * time.Millisecond)

	// Should receive all file watcher events
	for i, tc := range testCases {
		_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read event %d (%s): %v", i+1, tc.eventType, err)
		}

		var resp map[string]any
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("failed to parse event %d: %v", i+1, err)
		}

		if resp["type"] != "event" {
			t.Errorf("event %d: expected type 'event', got %v", i+1, resp["type"])
		}
		if resp["event"] != string(tc.eventType) {
			t.Errorf("event %d: expected event '%s', got %v", i+1, tc.eventType, resp["event"])
		}
		if resp["task_id"] != tc.taskID {
			t.Errorf("event %d: expected task_id '%s', got %v", i+1, tc.taskID, resp["task_id"])
		}
	}
}
