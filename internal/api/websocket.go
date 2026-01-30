package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/randalmurphal/orc/internal/events"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024 // 512KB
)

// WSMessage represents a WebSocket message.
type WSMessage struct {
	Type   string          `json:"type"` // subscribe, unsubscribe, command, event
	TaskID string          `json:"task_id,omitempty"`
	Action string          `json:"action,omitempty"` // pause, resume, cancel
	Data   json.RawMessage `json:"data,omitempty"`
}

// WSHandler manages WebSocket connections.
type WSHandler struct {
	upgrader    websocket.Upgrader
	publisher   events.Publisher
	connections map[*websocket.Conn]*wsConnection
	mu          sync.RWMutex
	logger      *slog.Logger
	server      *Server // Reference to main server for task operations
}

// wsConnection tracks a single WebSocket connection.
type wsConnection struct {
	conn         *websocket.Conn
	mu           sync.Mutex // protects taskID, eventChan, unsubscribed
	taskID       string
	eventChan    <-chan events.Event
	send         chan []byte
	done         chan struct{}
	unsubscribed bool
}

// NewWSHandler creates a new WebSocket handler.
func NewWSHandler(pub events.Publisher, server *Server, logger *slog.Logger) *WSHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &WSHandler{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
		publisher:   pub,
		connections: make(map[*websocket.Conn]*wsConnection),
		logger:      logger,
		server:      server,
	}
}

// ServeHTTP handles WebSocket upgrade requests.
func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	wsConn := &wsConnection{
		conn: conn,
		send: make(chan []byte, 256),
		done: make(chan struct{}),
	}

	h.mu.Lock()
	h.connections[conn] = wsConn
	h.mu.Unlock()

	// Start goroutines for reading and writing
	go h.readPump(wsConn)
	go h.writePump(wsConn)
}

// readPump reads messages from the WebSocket connection.
func (h *WSHandler) readPump(c *wsConnection) {
	defer func() {
		h.closeConnection(c)
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				h.logger.Error("websocket read error", "error", err)
			}
			return
		}

		h.handleMessage(c, message)
	}
}

// writePump writes messages to the WebSocket connection.
func (h *WSHandler) writePump(c *wsConnection) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case <-c.done:
			return
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Channel closed
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Send message as individual WebSocket frame (not batched to avoid invalid JSON)
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

			// Send any queued messages as separate frames
			n := len(c.send)
			for i := 0; i < n; i++ {
				_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := c.conn.WriteMessage(websocket.TextMessage, <-c.send); err != nil {
					return
				}
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming WebSocket messages.
func (h *WSHandler) handleMessage(c *wsConnection, data []byte) {
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		h.sendError(c, "invalid message format")
		return
	}

	switch msg.Type {
	case "subscribe":
		h.handleSubscribe(c, msg.TaskID)
	case "unsubscribe":
		h.handleUnsubscribe(c)
	case "command":
		h.handleCommand(c, msg)
	case "ping":
		// Respond to application-level ping with pong
		h.sendJSON(c, map[string]any{"type": "pong"})
	default:
		h.sendError(c, "unknown message type: "+msg.Type)
	}
}

// handleSubscribe subscribes the connection to a task's events.
// Use taskID "*" to subscribe to all task events (global subscription).
func (h *WSHandler) handleSubscribe(c *wsConnection, taskID string) {
	if taskID == "" {
		h.sendError(c, "task_id required for subscribe (use \"*\" for all tasks)")
		return
	}

	// Unsubscribe from previous task if any
	h.handleUnsubscribe(c)

	// Subscribe to new task (or global if taskID is "*")
	c.mu.Lock()
	c.taskID = taskID
	c.eventChan = h.publisher.Subscribe(taskID)
	c.unsubscribed = false
	c.mu.Unlock()

	// Start event forwarding goroutine
	go h.forwardEvents(c)

	// Send acknowledgment
	h.sendJSON(c, map[string]any{
		"type":    "subscribed",
		"task_id": taskID,
	})

	// For global subscriptions, send initial session_update so reconnecting
	// clients have current metrics immediately
	if taskID == events.GlobalTaskID && h.server != nil {
		h.logger.Debug("websocket subscribed to all tasks (global)")
		// Send initial session_update event
		sessionMetrics := h.server.GetSessionMetrics("")
		h.sendJSON(c, map[string]any{
			"type":    "event",
			"event":   string(events.EventSessionUpdate),
			"task_id": events.GlobalTaskID,
			"data":    sessionMetrics,
			"time":    time.Now(),
		})
	} else {
		h.logger.Debug("websocket subscribed", "task_id", taskID)
	}
}

// handleUnsubscribe unsubscribes the connection from current task.
func (h *WSHandler) handleUnsubscribe(c *wsConnection) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.taskID != "" && c.eventChan != nil && !c.unsubscribed {
		h.publisher.Unsubscribe(c.taskID, c.eventChan)
		c.unsubscribed = true
		c.taskID = ""
		c.eventChan = nil
	}
}

// handleCommand handles control commands (pause, resume, cancel).
func (h *WSHandler) handleCommand(c *wsConnection, msg WSMessage) {
	if msg.TaskID == "" {
		h.sendError(c, "task_id required for command")
		return
	}

	var result map[string]any
	var err error

	switch msg.Action {
	case "pause":
		result, err = h.server.pauseTask(msg.TaskID, "")
	case "resume":
		result, err = h.server.resumeTask(msg.TaskID, "")
	case "cancel":
		result, err = h.server.cancelTask(msg.TaskID, "")
	default:
		h.sendError(c, "unknown action: "+msg.Action)
		return
	}

	if err != nil {
		h.sendError(c, err.Error())
		return
	}

	result["type"] = "command_result"
	result["action"] = msg.Action
	h.sendJSON(c, result)
}

// forwardEvents forwards events from the publisher to the WebSocket.
func (h *WSHandler) forwardEvents(c *wsConnection) {
	// Get a local reference to eventChan under lock
	c.mu.Lock()
	eventChan := c.eventChan
	c.mu.Unlock()

	if eventChan == nil {
		return
	}

	for {
		select {
		case <-c.done:
			return
		case event, ok := <-eventChan:
			if !ok {
				return
			}

			// Check if unsubscribed before sending
			c.mu.Lock()
			unsubscribed := c.unsubscribed
			c.mu.Unlock()
			if unsubscribed {
				return
			}

			wsEvent := map[string]any{
				"type":    "event",
				"event":   string(event.Type),
				"task_id": event.TaskID,
				"data":    event.Data,
				"time":    event.Time,
			}
			h.sendJSON(c, wsEvent)
		}
	}
}

// closeConnection cleans up a WebSocket connection.
func (h *WSHandler) closeConnection(c *wsConnection) {
	h.mu.Lock()
	_, exists := h.connections[c.conn]
	if !exists {
		h.mu.Unlock()
		return // Already cleaned up
	}
	delete(h.connections, c.conn)
	h.mu.Unlock()

	h.handleUnsubscribe(c)

	// Safely close done channel (only once)
	select {
	case <-c.done:
		// Already closed
	default:
		close(c.done)
	}

	_ = c.conn.Close()
}

// sendJSON sends a JSON message to a connection.
func (h *WSHandler) sendJSON(c *wsConnection, data any) {
	msg, err := json.Marshal(data)
	if err != nil {
		h.logger.Error("failed to marshal JSON", "error", err)
		return
	}

	select {
	case c.send <- msg:
	default:
		// Buffer full, skip message
		h.logger.Warn("websocket send buffer full, dropping message")
	}
}

// sendError sends an error message to a connection.
func (h *WSHandler) sendError(c *wsConnection, message string) {
	h.sendJSON(c, map[string]any{
		"type":  "error",
		"error": message,
	})
}

// Broadcast sends an event to all connections subscribed to a task.
func (h *WSHandler) Broadcast(taskID string, event events.Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, c := range h.connections {
		if c.taskID == taskID {
			wsEvent := map[string]any{
				"type":    "event",
				"event":   string(event.Type),
				"task_id": event.TaskID,
				"data":    event.Data,
				"time":    event.Time,
			}
			h.sendJSON(c, wsEvent)
		}
	}
}

// ConnectionCount returns the number of active connections.
func (h *WSHandler) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// Close closes all connections.
func (h *WSHandler) Close() {
	h.mu.Lock()
	conns := make([]*wsConnection, 0, len(h.connections))
	for _, c := range h.connections {
		conns = append(conns, c)
	}
	h.mu.Unlock()

	for _, c := range conns {
		h.closeConnection(c)
	}
}
