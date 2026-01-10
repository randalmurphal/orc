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
	defer ws.Close()

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
	defer ws.Close()

	// Subscribe to task
	msg := WSMessage{Type: "subscribe", TaskID: "TASK-001"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read response
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
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
	defer ws.Close()

	// Subscribe to task
	msg := WSMessage{Type: "subscribe", TaskID: "TASK-001"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read subscription confirmation
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = ws.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read subscription response: %v", err)
	}

	// Publish an event
	pub.Publish(events.NewEvent(events.EventState, "TASK-001", map[string]string{"status": "running"}))

	// Give time for event to be forwarded
	time.Sleep(100 * time.Millisecond)

	// Read event
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
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
	defer ws.Close()

	// Send invalid JSON
	if err := ws.WriteMessage(websocket.TextMessage, []byte("not json")); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Should receive error response
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
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
	defer ws.Close()

	// Subscribe without task ID
	msg := WSMessage{Type: "subscribe"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Should receive error
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
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
	defer ws.Close()

	// Send unknown message type
	msg := WSMessage{Type: "unknown_type"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	// Should receive error
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
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
			ws.Close()
		}
	}()

	// Allow connections to register
	time.Sleep(50 * time.Millisecond)

	if handler.ConnectionCount() != 3 {
		t.Errorf("expected 3 connections, got %d", handler.ConnectionCount())
	}

	// Close one connection
	conns[0].Close()
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
	defer ws.Close()

	msg := WSMessage{Type: "subscribe", TaskID: "TASK-001"}
	if err := ws.WriteJSON(msg); err != nil {
		t.Fatalf("failed to send subscribe: %v", err)
	}

	// Read subscription confirmation
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, _ = ws.ReadMessage()

	// Broadcast event
	event := events.NewEvent(events.EventPhase, "TASK-001", events.PhaseUpdate{
		Phase:  "test",
		Status: "completed",
	})
	handler.Broadcast("TASK-001", event)

	// Should receive broadcast
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
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
	defer ws.Close()

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
	ws.Close()
}
