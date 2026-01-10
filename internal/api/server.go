// Package api provides the REST API and SSE server for orc.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// Server is the orc API server.
type Server struct {
	addr   string
	mux    *http.ServeMux
	logger *slog.Logger

	// SSE subscribers per task
	subscribers   map[string][]chan Event
	subscribersMu sync.RWMutex
}

// Event represents an SSE event.
type Event struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Config holds server configuration.
type Config struct {
	Addr   string
	Logger *slog.Logger
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() *Config {
	return &Config{
		Addr:   ":8080",
		Logger: slog.Default(),
	}
}

// New creates a new API server.
func New(cfg *Config) *Server {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Ensure logger is never nil
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		addr:        cfg.Addr,
		mux:         http.NewServeMux(),
		logger:      logger,
		subscribers: make(map[string][]chan Event),
	}

	s.registerRoutes()
	return s
}

// registerRoutes sets up all API routes.
func (s *Server) registerRoutes() {
	// CORS middleware wrapper
	cors := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			h(w, r)
		}
	}

	// Health check
	s.mux.HandleFunc("GET /api/health", cors(s.handleHealth))

	// Tasks
	s.mux.HandleFunc("GET /api/tasks", cors(s.handleListTasks))
	s.mux.HandleFunc("POST /api/tasks", cors(s.handleCreateTask))
	s.mux.HandleFunc("GET /api/tasks/{id}", cors(s.handleGetTask))
	s.mux.HandleFunc("DELETE /api/tasks/{id}", cors(s.handleDeleteTask))

	// Task state and plan
	s.mux.HandleFunc("GET /api/tasks/{id}/state", cors(s.handleGetState))
	s.mux.HandleFunc("GET /api/tasks/{id}/plan", cors(s.handleGetPlan))
	s.mux.HandleFunc("GET /api/tasks/{id}/transcripts", cors(s.handleGetTranscripts))

	// Task control
	s.mux.HandleFunc("POST /api/tasks/{id}/run", cors(s.handleRunTask))
	s.mux.HandleFunc("POST /api/tasks/{id}/pause", cors(s.handlePauseTask))
	s.mux.HandleFunc("POST /api/tasks/{id}/resume", cors(s.handleResumeTask))

	// SSE streaming
	s.mux.HandleFunc("GET /api/tasks/{id}/stream", s.handleStream)

	// Config
	s.mux.HandleFunc("GET /api/config", cors(s.handleGetConfig))
}

// Start starts the API server.
func (s *Server) Start() error {
	s.logger.Info("starting API server", "addr", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}

// StartContext starts the API server with context for graceful shutdown.
func (s *Server) StartContext(ctx context.Context) error {
	server := &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	s.logger.Info("starting API server", "addr", s.addr)
	return server.ListenAndServe()
}

// Publish sends an event to all subscribers of a task.
func (s *Server) Publish(taskID string, event Event) {
	s.subscribersMu.RLock()
	defer s.subscribersMu.RUnlock()

	for _, ch := range s.subscribers[taskID] {
		select {
		case ch <- event:
		default:
			// Skip if channel is full
		}
	}
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleListTasks returns all tasks.
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := task.LoadAll()
	if err != nil {
		s.jsonError(w, "failed to load tasks", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array, not null
	if tasks == nil {
		tasks = []*task.Task{}
	}

	s.jsonResponse(w, tasks)
}

// handleCreateTask creates a new task.
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title       string `json:"title"`
		Description string `json:"description,omitempty"`
		Weight      string `json:"weight,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Title == "" {
		s.jsonError(w, "title is required", http.StatusBadRequest)
		return
	}

	id, err := task.NextID()
	if err != nil {
		s.jsonError(w, "failed to generate task ID", http.StatusInternalServerError)
		return
	}

	t := task.New(id, req.Title)
	t.Description = req.Description
	if req.Weight != "" {
		t.Weight = task.Weight(req.Weight)
	} else {
		// Default to medium if not specified
		t.Weight = task.WeightMedium
	}

	if err := t.Save(); err != nil {
		s.jsonError(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	// Create plan from template
	p, err := plan.CreateFromTemplate(t)
	if err != nil {
		// If template not found, use default plan
		p = &plan.Plan{
			Version:     1,
			TaskID:      id,
			Weight:      t.Weight,
			Description: "Default plan",
			Phases: []plan.Phase{
				{ID: "implement", Name: "implement", Gate: plan.Gate{Type: plan.GateAuto}, Status: plan.PhasePending},
			},
		}
	}

	// Save plan
	if err := p.Save(id); err != nil {
		s.jsonError(w, "failed to save plan", http.StatusInternalServerError)
		return
	}

	// Update task status to planned
	t.Status = task.StatusPlanned
	if err := t.Save(); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, t)
}

// handleGetTask returns a specific task.
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.Load(id)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, t)
}

// handleDeleteTask deletes a task.
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement task deletion
	s.jsonError(w, "not implemented", http.StatusNotImplemented)
}

// handleGetState returns task execution state.
func (s *Server) handleGetState(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	st, err := state.Load(id)
	if err != nil {
		s.jsonError(w, "state not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, st)
}

// handleGetPlan returns task plan.
func (s *Server) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := plan.Load(id)
	if err != nil {
		s.jsonError(w, "plan not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, p)
}

// handleGetTranscripts returns task transcript files.
func (s *Server) handleGetTranscripts(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify task exists
	if !task.Exists(id) {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Read transcript files
	transcriptsDir := task.TaskDir(id) + "/transcripts"
	entries, err := os.ReadDir(transcriptsDir)
	if err != nil {
		// No transcripts yet is OK
		s.jsonResponse(w, []map[string]interface{}{})
		return
	}

	var transcripts []map[string]interface{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(transcriptsDir + "/" + entry.Name())
		if err != nil {
			continue
		}

		info, _ := entry.Info()
		transcripts = append(transcripts, map[string]interface{}{
			"filename":   entry.Name(),
			"content":    string(content),
			"created_at": info.ModTime(),
		})
	}

	// Ensure we return an empty array, not null
	if transcripts == nil {
		transcripts = []map[string]interface{}{}
	}

	s.jsonResponse(w, transcripts)
}

// handleRunTask starts task execution.
func (s *Server) handleRunTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.Load(id)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if !t.CanRun() {
		s.jsonError(w, fmt.Sprintf("task cannot run in status: %s", t.Status), http.StatusBadRequest)
		return
	}

	// Load plan and state
	p, err := plan.Load(id)
	if err != nil {
		s.jsonError(w, "plan not found", http.StatusNotFound)
		return
	}

	st, err := state.Load(id)
	if err != nil {
		// Create new state if it doesn't exist
		st = state.New(id)
	}

	// Start execution in background goroutine
	go func() {
		ctx := context.Background()
		exec := executor.New(executor.DefaultConfig())

		// Execute with SSE publishing
		err := exec.ExecuteTask(ctx, t, p, st)
		if err != nil {
			s.logger.Error("task execution failed", "task", id, "error", err)
			s.Publish(id, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("task execution completed", "task", id)
			s.Publish(id, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}

		// Reload and publish final state
		if finalState, err := state.Load(id); err == nil {
			s.Publish(id, Event{Type: "state", Data: finalState})
		}
	}()

	s.jsonResponse(w, map[string]string{"status": "started", "task_id": id})
}

// handlePauseTask pauses task execution.
func (s *Server) handlePauseTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.Load(id)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	t.Status = task.StatusPaused
	if err := t.Save(); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "paused", "task_id": id})
}

// handleResumeTask resumes task execution.
func (s *Server) handleResumeTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, err := task.Load(id)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	t.Status = task.StatusRunning
	if err := t.Save(); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, map[string]string{"status": "resumed", "task_id": id})
}

// handleStream handles SSE streaming for a task.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify task exists
	if !task.Exists(id) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create subscriber channel
	ch := make(chan Event, 100)

	s.subscribersMu.Lock()
	s.subscribers[id] = append(s.subscribers[id], ch)
	s.subscribersMu.Unlock()

	// Cleanup on disconnect
	defer func() {
		s.subscribersMu.Lock()
		subs := s.subscribers[id]
		for i, sub := range subs {
			if sub == ch {
				s.subscribers[id] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		s.subscribersMu.Unlock()
		close(ch)
	}()

	// Send initial state
	if st, err := state.Load(id); err == nil {
		data, _ := json.Marshal(st)
		fmt.Fprintf(w, "event: state\ndata: %s\n\n", data)
		w.(http.Flusher).Flush()
	}

	// Stream events
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-ch:
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			w.(http.Flusher).Flush()
		}
	}
}

// handleGetConfig returns orc configuration.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	s.jsonResponse(w, map[string]any{
		"version": "1.0.0",
		"profile": cfg.Profile,
		"automation": map[string]any{
			"profile":       cfg.Profile,
			"gates_default": cfg.Gates.DefaultType,
			"retry_enabled": cfg.Retry.Enabled,
			"retry_max":     cfg.Retry.MaxRetries,
		},
		"execution": map[string]any{
			"model":          cfg.Model,
			"max_iterations": cfg.MaxIterations,
			"timeout":        cfg.Timeout.String(),
		},
		"git": map[string]any{
			"branch_prefix": cfg.BranchPrefix,
			"commit_prefix": cfg.CommitPrefix,
		},
	})
}

// jsonResponse writes a JSON response.
func (s *Server) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// jsonError writes a JSON error response.
func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
