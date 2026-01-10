// Package api provides the REST API and SSE server for orc.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/hooks"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/project"
	"github.com/randalmurphal/orc/internal/prompt"
	"github.com/randalmurphal/orc/internal/skills"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// Server is the orc API server.
type Server struct {
	addr   string
	mux    *http.ServeMux
	logger *slog.Logger

	// Event publisher for real-time updates
	publisher events.Publisher
	wsHandler *WSHandler

	// SSE subscribers per task (legacy, kept for compatibility)
	subscribers   map[string][]chan Event
	subscribersMu sync.RWMutex

	// Running tasks for cancellation
	runningTasks   map[string]context.CancelFunc
	runningTasksMu sync.RWMutex
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

	// Create event publisher
	pub := events.NewMemoryPublisher()

	s := &Server{
		addr:         cfg.Addr,
		mux:          http.NewServeMux(),
		logger:       logger,
		publisher:    pub,
		subscribers:  make(map[string][]chan Event),
		runningTasks: make(map[string]context.CancelFunc),
	}

	// Create WebSocket handler
	s.wsHandler = NewWSHandler(pub, s, logger)

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

	// SSE streaming (legacy)
	s.mux.HandleFunc("GET /api/tasks/{id}/stream", s.handleStream)

	// WebSocket for real-time updates
	s.mux.Handle("GET /api/ws", s.wsHandler)

	// Prompts
	s.mux.HandleFunc("GET /api/prompts", cors(s.handleListPrompts))
	s.mux.HandleFunc("GET /api/prompts/variables", cors(s.handleGetPromptVariables))
	s.mux.HandleFunc("GET /api/prompts/{phase}", cors(s.handleGetPrompt))
	s.mux.HandleFunc("GET /api/prompts/{phase}/default", cors(s.handleGetPromptDefault))
	s.mux.HandleFunc("PUT /api/prompts/{phase}", cors(s.handleSavePrompt))
	s.mux.HandleFunc("DELETE /api/prompts/{phase}", cors(s.handleDeletePrompt))

	// Hooks
	s.mux.HandleFunc("GET /api/hooks", cors(s.handleListHooks))
	s.mux.HandleFunc("GET /api/hooks/types", cors(s.handleGetHookTypes))
	s.mux.HandleFunc("POST /api/hooks", cors(s.handleCreateHook))
	s.mux.HandleFunc("GET /api/hooks/{name}", cors(s.handleGetHook))
	s.mux.HandleFunc("PUT /api/hooks/{name}", cors(s.handleUpdateHook))
	s.mux.HandleFunc("DELETE /api/hooks/{name}", cors(s.handleDeleteHook))

	// Skills
	s.mux.HandleFunc("GET /api/skills", cors(s.handleListSkills))
	s.mux.HandleFunc("POST /api/skills", cors(s.handleCreateSkill))
	s.mux.HandleFunc("GET /api/skills/{name}", cors(s.handleGetSkill))
	s.mux.HandleFunc("PUT /api/skills/{name}", cors(s.handleUpdateSkill))
	s.mux.HandleFunc("DELETE /api/skills/{name}", cors(s.handleDeleteSkill))

	// Config
	s.mux.HandleFunc("GET /api/config", cors(s.handleGetConfig))
	s.mux.HandleFunc("PUT /api/config", cors(s.handleUpdateConfig))

	// Projects
	s.mux.HandleFunc("GET /api/projects", cors(s.handleListProjects))
	s.mux.HandleFunc("GET /api/projects/{id}", cors(s.handleGetProject))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks", cors(s.handleListProjectTasks))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks", cors(s.handleCreateProjectTask))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks/{taskId}", cors(s.handleGetProjectTask))
	s.mux.HandleFunc("DELETE /api/projects/{id}/tasks/{taskId}", cors(s.handleDeleteProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/run", cors(s.handleRunProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/pause", cors(s.handlePauseProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/resume", cors(s.handleResumeProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/rewind", cors(s.handleRewindProjectTask))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks/{taskId}/state", cors(s.handleGetProjectTaskState))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks/{taskId}/plan", cors(s.handleGetProjectTaskPlan))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks/{taskId}/transcripts", cors(s.handleGetProjectTaskTranscripts))
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

// handleListTasks returns all tasks with optional pagination.
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

	// Check for pagination params
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	// If no pagination requested, return all tasks (backward compatible)
	if pageStr == "" && limitStr == "" {
		s.jsonResponse(w, tasks)
		return
	}

	// Parse pagination params
	page := 1
	limit := 20 // default limit
	if pageStr != "" {
		if p, err := parsePositiveInt(pageStr); err == nil && p > 0 {
			page = p
		}
	}
	if limitStr != "" {
		if l, err := parsePositiveInt(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	// Calculate pagination
	total := len(tasks)
	totalPages := (total + limit - 1) / limit
	start := (page - 1) * limit
	end := start + limit

	// Bounds checking
	if start >= total {
		start = total
		end = total
	}
	if end > total {
		end = total
	}

	pagedTasks := tasks[start:end]

	s.jsonResponse(w, map[string]any{
		"tasks":       pagedTasks,
		"total":       total,
		"page":        page,
		"limit":       limit,
		"total_pages": totalPages,
	})
}

// parsePositiveInt parses a string to a positive integer using strconv.
func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid number")
	}
	return n, nil
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
	id := r.PathValue("id")

	// Check if task is running
	t, err := task.Load(id)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if t.Status == task.StatusRunning {
		s.jsonError(w, "cannot delete running task", http.StatusConflict)
		return
	}

	// Delete task
	if err := task.Delete(id); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to delete task: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Store cancel function for later cancellation
	s.runningTasksMu.Lock()
	s.runningTasks[id] = cancel
	s.runningTasksMu.Unlock()

	// Start execution in background goroutine
	go func() {
		defer func() {
			s.runningTasksMu.Lock()
			delete(s.runningTasks, id)
			s.runningTasksMu.Unlock()
		}()

		exec := executor.New(executor.DefaultConfig())
		exec.SetPublisher(s.publisher)

		// Execute with event publishing
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

// ConfigUpdateRequest represents a config update request.
type ConfigUpdateRequest struct {
	Profile    string `json:"profile,omitempty"`
	Automation *struct {
		GatesDefault string `json:"gates_default,omitempty"`
		RetryEnabled *bool  `json:"retry_enabled,omitempty"`
		RetryMax     *int   `json:"retry_max,omitempty"`
	} `json:"automation,omitempty"`
	Execution *struct {
		Model         string `json:"model,omitempty"`
		MaxIterations *int   `json:"max_iterations,omitempty"`
		Timeout       string `json:"timeout,omitempty"`
	} `json:"execution,omitempty"`
	Git *struct {
		BranchPrefix string `json:"branch_prefix,omitempty"`
		CommitPrefix string `json:"commit_prefix,omitempty"`
	} `json:"git,omitempty"`
}

// handleUpdateConfig updates orc configuration.
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var req ConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Load existing config
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	// Apply profile if specified
	if req.Profile != "" {
		profile := config.AutomationProfile(req.Profile)
		cfg.ApplyProfile(profile)
	}

	// Apply automation settings
	if req.Automation != nil {
		if req.Automation.GatesDefault != "" {
			cfg.Gates.DefaultType = req.Automation.GatesDefault
		}
		if req.Automation.RetryEnabled != nil {
			cfg.Retry.Enabled = *req.Automation.RetryEnabled
		}
		if req.Automation.RetryMax != nil {
			cfg.Retry.MaxRetries = *req.Automation.RetryMax
		}
	}

	// Apply execution settings
	if req.Execution != nil {
		if req.Execution.Model != "" {
			cfg.Model = req.Execution.Model
		}
		if req.Execution.MaxIterations != nil {
			cfg.MaxIterations = *req.Execution.MaxIterations
		}
		if req.Execution.Timeout != "" {
			if d, err := time.ParseDuration(req.Execution.Timeout); err == nil {
				cfg.Timeout = d
			}
		}
	}

	// Apply git settings
	if req.Git != nil {
		if req.Git.BranchPrefix != "" {
			cfg.BranchPrefix = req.Git.BranchPrefix
		}
		if req.Git.CommitPrefix != "" {
			cfg.CommitPrefix = req.Git.CommitPrefix
		}
	}

	// Save config
	if err := cfg.Save(); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save config: %v", err), http.StatusInternalServerError)
		return
	}

	// Return updated config
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

// handleListProjects returns all registered projects.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := project.ListProjects()
	if err != nil {
		s.jsonError(w, "failed to list projects", http.StatusInternalServerError)
		return
	}

	// Ensure we return an empty array, not null
	if projects == nil {
		projects = []project.Project{}
	}

	s.jsonResponse(w, projects)
}

// handleGetProject returns a specific project.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	reg, err := project.LoadRegistry()
	if err != nil {
		s.jsonError(w, "failed to load registry", http.StatusInternalServerError)
		return
	}

	proj, err := reg.Get(id)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, proj)
}

// handleListProjectTasks returns all tasks for a project.
func (s *Server) handleListProjectTasks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	reg, err := project.LoadRegistry()
	if err != nil {
		s.jsonError(w, "failed to load registry", http.StatusInternalServerError)
		return
	}

	proj, err := reg.Get(id)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	// Load tasks from project directory
	tasksDir := filepath.Join(proj.Path, ".orc", "tasks")
	tasks, err := task.LoadAllFrom(tasksDir)
	if err != nil {
		// No tasks dir is OK - return empty list
		s.jsonResponse(w, []*task.Task{})
		return
	}

	if tasks == nil {
		tasks = []*task.Task{}
	}

	s.jsonResponse(w, tasks)
}

// handleCreateProjectTask creates a new task in a project.
func (s *Server) handleCreateProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")

	reg, err := project.LoadRegistry()
	if err != nil {
		s.jsonError(w, "failed to load registry", http.StatusInternalServerError)
		return
	}

	proj, err := reg.Get(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

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

	// Generate ID in project context
	id, err := task.NextIDIn(filepath.Join(proj.Path, ".orc", "tasks"))
	if err != nil {
		s.jsonError(w, "failed to generate task ID", http.StatusInternalServerError)
		return
	}

	t := task.New(id, req.Title)
	t.Description = req.Description
	if req.Weight != "" {
		t.Weight = task.Weight(req.Weight)
	} else {
		t.Weight = task.WeightMedium
	}

	// Save in project directory
	if err := t.SaveTo(filepath.Join(proj.Path, ".orc", "tasks", id)); err != nil {
		s.jsonError(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	// Create plan from template
	p, err := plan.CreateFromTemplate(t)
	if err != nil {
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

	// Save plan in project directory
	if err := p.SaveTo(filepath.Join(proj.Path, ".orc", "tasks", id)); err != nil {
		s.jsonError(w, "failed to save plan", http.StatusInternalServerError)
		return
	}

	t.Status = task.StatusPlanned
	if err := t.SaveTo(filepath.Join(proj.Path, ".orc", "tasks", id)); err != nil {
		s.jsonError(w, "failed to update task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, t)
}

// handleGetProjectTask returns a specific task from a project.
func (s *Server) handleGetProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, t)
}

// handleDeleteProjectTask deletes a task from a project.
func (s *Server) handleDeleteProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if t.Status == task.StatusRunning {
		s.jsonError(w, "cannot delete running task", http.StatusConflict)
		return
	}

	taskDir := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	if err := os.RemoveAll(taskDir); err != nil {
		s.jsonError(w, "failed to delete task", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleRunProjectTask starts task execution for a project task.
func (s *Server) handleRunProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	s.logger.Info("handleRunProjectTask", "projectID", projectID, "taskID", taskID)

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	s.logger.Info("resolved project", "name", proj.Name, "path", proj.Path)

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	s.logger.Info("loaded task", "id", t.ID, "title", t.Title)

	if !t.CanRun() {
		s.jsonError(w, "task cannot be run in current state", http.StatusBadRequest)
		return
	}

	// Load plan
	planPath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "plan.yaml")
	planData, err := os.ReadFile(planPath)
	if err != nil {
		s.jsonError(w, "failed to load plan", http.StatusInternalServerError)
		return
	}
	var p plan.Plan
	if err := yaml.Unmarshal(planData, &p); err != nil {
		s.jsonError(w, "failed to parse plan", http.StatusInternalServerError)
		return
	}

	// Load or create state
	statePath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "state.yaml")
	var st state.State
	if stateData, err := os.ReadFile(statePath); err == nil {
		yaml.Unmarshal(stateData, &st)
	} else {
		st = state.State{
			TaskID:           taskID,
			CurrentPhase:     p.Phases[0].ID,
			Status:           state.StatusRunning,
			CurrentIteration: 1,
			StartedAt:        time.Now(),
			Phases:           make(map[string]*state.PhaseState),
		}
	}

	// Mark task as running
	t.Status = task.StatusRunning
	now := time.Now()
	t.StartedAt = &now
	savePath := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	s.logger.Info("saving task", "path", savePath)
	if err := t.SaveTo(savePath); err != nil {
		s.jsonError(w, "failed to update task status", http.StatusInternalServerError)
		return
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	s.runningTasksMu.Lock()
	s.runningTasks[taskID] = cancel
	s.runningTasksMu.Unlock()

	// Capture project path for goroutine
	projectPath := proj.Path

	// Start execution in background
	go func() {
		defer func() {
			s.runningTasksMu.Lock()
			delete(s.runningTasks, taskID)
			s.runningTasksMu.Unlock()
		}()

		cfg := executor.DefaultConfig()
		cfg.WorkDir = projectPath
		exec := executor.New(cfg)
		exec.SetPublisher(s.publisher)

		if err := exec.ExecuteTask(ctx, t, &p, &st); err != nil {
			s.logger.Error("task execution failed", "task", taskID, "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("task execution completed", "task", taskID)
			s.Publish(taskID, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}
	}()

	s.jsonResponse(w, map[string]any{
		"status":  "started",
		"task_id": taskID,
	})
}

// handlePauseProjectTask pauses a running project task.
func (s *Server) handlePauseProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	if t.Status != task.StatusRunning {
		s.jsonError(w, "task is not running", http.StatusBadRequest)
		return
	}

	// Cancel the running executor
	s.runningTasksMu.Lock()
	cancel, running := s.runningTasks[taskID]
	s.runningTasksMu.Unlock()
	if running {
		s.logger.Info("cancelling running executor", "task", taskID)
		cancel()
	}

	// Update task status
	t.Status = task.StatusPaused
	taskDir := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	if err := t.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to update task status", http.StatusInternalServerError)
		return
	}

	// Update state status
	statePath := filepath.Join(taskDir, "state.yaml")
	if stateData, err := os.ReadFile(statePath); err == nil {
		var st state.State
		if err := yaml.Unmarshal(stateData, &st); err == nil {
			st.Status = state.StatusPaused
			if err := st.SaveTo(taskDir); err != nil {
				s.logger.Error("failed to save state", "error", err)
			}
		}
	}

	s.jsonResponse(w, map[string]any{
		"status":  "paused",
		"task_id": taskID,
	})
}

// handleResumeProjectTask resumes a paused project task.
func (s *Server) handleResumeProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Task must be paused to resume
	if t.Status != task.StatusPaused {
		s.jsonError(w, "task is not paused", http.StatusBadRequest)
		return
	}

	// Load plan
	planPath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "plan.yaml")
	planData, err := os.ReadFile(planPath)
	if err != nil {
		s.jsonError(w, "failed to load plan", http.StatusInternalServerError)
		return
	}
	var p plan.Plan
	if err := yaml.Unmarshal(planData, &p); err != nil {
		s.jsonError(w, "failed to parse plan", http.StatusInternalServerError)
		return
	}

	// Load state
	taskDir := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	statePath := filepath.Join(taskDir, "state.yaml")
	stateData, err := os.ReadFile(statePath)
	if err != nil {
		s.jsonError(w, "failed to load state", http.StatusInternalServerError)
		return
	}
	var st state.State
	if err := yaml.Unmarshal(stateData, &st); err != nil {
		s.jsonError(w, "failed to parse state", http.StatusInternalServerError)
		return
	}

	// Update task status
	t.Status = task.StatusRunning
	if err := t.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to update task status", http.StatusInternalServerError)
		return
	}

	// Update state status
	st.Status = state.StatusRunning
	if st.Phases[st.CurrentPhase] != nil {
		st.Phases[st.CurrentPhase].Status = state.StatusRunning
		st.Phases[st.CurrentPhase].InterruptedAt = nil
	}
	if err := st.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to update state", http.StatusInternalServerError)
		return
	}

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	s.runningTasksMu.Lock()
	s.runningTasks[taskID] = cancel
	s.runningTasksMu.Unlock()

	// Capture project path for goroutine
	projectPath := proj.Path

	s.logger.Info("resuming task execution", "task", taskID, "phase", st.CurrentPhase)

	// Start execution in background
	go func() {
		defer func() {
			s.runningTasksMu.Lock()
			delete(s.runningTasks, taskID)
			s.runningTasksMu.Unlock()
		}()

		cfg := executor.DefaultConfig()
		cfg.WorkDir = projectPath
		exec := executor.New(cfg)
		exec.SetPublisher(s.publisher)

		if err := exec.ExecuteTask(ctx, t, &p, &st); err != nil {
			s.logger.Error("task execution failed", "task", taskID, "error", err)
			s.Publish(taskID, Event{Type: "error", Data: map[string]string{"error": err.Error()}})
		} else {
			s.logger.Info("task execution completed", "task", taskID)
			s.Publish(taskID, Event{Type: "complete", Data: map[string]string{"status": "completed"}})
		}
	}()

	s.jsonResponse(w, map[string]any{
		"status":  "resumed",
		"task_id": taskID,
	})
}

// handleRewindProjectTask rewinds a task to a previous phase.
func (s *Server) handleRewindProjectTask(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	// Parse request body
	var req struct {
		Phase string `json:"phase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Phase == "" {
		s.jsonError(w, "phase is required", http.StatusBadRequest)
		return
	}

	t, err := s.loadProjectTask(proj.Path, taskID)
	if err != nil {
		s.jsonError(w, "task not found", http.StatusNotFound)
		return
	}

	// Load plan
	taskDir := filepath.Join(proj.Path, ".orc", "tasks", taskID)
	planPath := filepath.Join(taskDir, "plan.yaml")
	planData, err := os.ReadFile(planPath)
	if err != nil {
		s.jsonError(w, "failed to load plan", http.StatusInternalServerError)
		return
	}
	var p plan.Plan
	if err := yaml.Unmarshal(planData, &p); err != nil {
		s.jsonError(w, "failed to parse plan", http.StatusInternalServerError)
		return
	}

	// Find target phase
	targetPhase := p.GetPhase(req.Phase)
	if targetPhase == nil {
		s.jsonError(w, "phase not found", http.StatusBadRequest)
		return
	}

	// Load state
	statePath := filepath.Join(taskDir, "state.yaml")
	stateData, err := os.ReadFile(statePath)
	if err != nil && !os.IsNotExist(err) {
		s.jsonError(w, "failed to load state", http.StatusInternalServerError)
		return
	}
	var st state.State
	if err == nil {
		yaml.Unmarshal(stateData, &st)
	}

	// Mark target and all later phases as pending
	foundTarget := false
	for i := range p.Phases {
		if p.Phases[i].ID == req.Phase {
			foundTarget = true
		}
		if foundTarget {
			p.Phases[i].Status = plan.PhasePending
			p.Phases[i].CommitSHA = ""
			if st.Phases[p.Phases[i].ID] != nil {
				st.Phases[p.Phases[i].ID].Status = state.StatusPending
				st.Phases[p.Phases[i].ID].CompletedAt = nil
			}
		}
	}

	// Update state to point to target phase
	st.Status = state.StatusPending
	st.CurrentPhase = req.Phase
	st.CurrentIteration = 1
	st.CompletedAt = nil

	// Update task status to allow re-running
	t.Status = task.StatusPlanned
	t.CompletedAt = nil

	// Save all updates
	if err := p.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to save plan", http.StatusInternalServerError)
		return
	}
	if err := st.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to save state", http.StatusInternalServerError)
		return
	}
	if err := t.SaveTo(taskDir); err != nil {
		s.jsonError(w, "failed to save task", http.StatusInternalServerError)
		return
	}

	s.logger.Info("rewound task", "task", taskID, "toPhase", req.Phase)

	s.jsonResponse(w, map[string]any{
		"status":  "rewound",
		"task_id": taskID,
		"phase":   req.Phase,
	})
}

// handleGetProjectTaskState returns the state for a project task.
func (s *Server) handleGetProjectTaskState(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	statePath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "state.yaml")
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			s.jsonError(w, "state not found", http.StatusNotFound)
			return
		}
		s.jsonError(w, "failed to read state", http.StatusInternalServerError)
		return
	}

	var st state.State
	if err := yaml.Unmarshal(data, &st); err != nil {
		s.jsonError(w, "failed to parse state", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, st)
}

// handleGetProjectTaskPlan returns the plan for a project task.
func (s *Server) handleGetProjectTaskPlan(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	planPath := filepath.Join(proj.Path, ".orc", "tasks", taskID, "plan.yaml")
	data, err := os.ReadFile(planPath)
	if err != nil {
		if os.IsNotExist(err) {
			s.jsonError(w, "plan not found", http.StatusNotFound)
			return
		}
		s.jsonError(w, "failed to read plan", http.StatusInternalServerError)
		return
	}

	var p plan.Plan
	if err := yaml.Unmarshal(data, &p); err != nil {
		s.jsonError(w, "failed to parse plan", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, p)
}

// handleGetProjectTaskTranscripts returns transcripts for a project task.
func (s *Server) handleGetProjectTaskTranscripts(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	taskID := r.PathValue("taskId")

	proj, err := s.getProject(projectID)
	if err != nil {
		s.jsonError(w, "project not found", http.StatusNotFound)
		return
	}

	transcriptsDir := filepath.Join(proj.Path, ".orc", "tasks", taskID, "transcripts")
	entries, err := os.ReadDir(transcriptsDir)
	if err != nil {
		if os.IsNotExist(err) {
			s.jsonResponse(w, []any{})
			return
		}
		s.jsonError(w, "failed to read transcripts", http.StatusInternalServerError)
		return
	}

	var transcripts []map[string]any
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(transcriptsDir, entry.Name()))
		if err != nil {
			continue
		}

		info, _ := entry.Info()
		transcripts = append(transcripts, map[string]any{
			"filename":   entry.Name(),
			"content":    string(content),
			"created_at": info.ModTime().Format(time.RFC3339),
		})
	}

	if transcripts == nil {
		transcripts = []map[string]any{}
	}

	s.jsonResponse(w, transcripts)
}

// getProject loads a project by ID.
func (s *Server) getProject(projectID string) (*project.Project, error) {
	reg, err := project.LoadRegistry()
	if err != nil {
		return nil, err
	}
	return reg.Get(projectID)
}

// loadProjectTask loads a task from a specific project path.
func (s *Server) loadProjectTask(projectPath, taskID string) (*task.Task, error) {
	taskPath := filepath.Join(projectPath, ".orc", "tasks", taskID, "task.yaml")
	data, err := os.ReadFile(taskPath)
	if err != nil {
		return nil, err
	}

	var t task.Task
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, err
	}

	return &t, nil
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

// pauseTask pauses a running task (called by WebSocket handler).
func (s *Server) pauseTask(id string) (map[string]any, error) {
	t, err := task.Load(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	t.Status = task.StatusPaused
	if err := t.Save(); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return map[string]any{
		"status":  "paused",
		"task_id": id,
	}, nil
}

// resumeTask resumes a paused task (called by WebSocket handler).
func (s *Server) resumeTask(id string) (map[string]any, error) {
	t, err := task.Load(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	// If task was paused, restart execution
	if t.Status == task.StatusPaused {
		t.Status = task.StatusRunning
		if err := t.Save(); err != nil {
			return nil, fmt.Errorf("failed to update task: %w", err)
		}

		// Resume execution
		p, err := plan.Load(id)
		if err != nil {
			return nil, fmt.Errorf("plan not found")
		}

		st, err := state.Load(id)
		if err != nil {
			return nil, fmt.Errorf("state not found")
		}

		// Find resume point
		resumePhase := st.GetResumePhase()
		if resumePhase == "" {
			return nil, fmt.Errorf("no resume point found")
		}

		// Create cancellable context
		ctx, cancel := context.WithCancel(context.Background())

		s.runningTasksMu.Lock()
		s.runningTasks[id] = cancel
		s.runningTasksMu.Unlock()

		go func() {
			defer func() {
				s.runningTasksMu.Lock()
				delete(s.runningTasks, id)
				s.runningTasksMu.Unlock()
			}()

			exec := executor.New(executor.DefaultConfig())
			exec.SetPublisher(s.publisher)
			err := exec.ResumeFromPhase(ctx, t, p, st, resumePhase)
			if err != nil {
				s.logger.Error("task resume failed", "task", id, "error", err)
			}
		}()
	}

	return map[string]any{
		"status":  "resumed",
		"task_id": id,
	}, nil
}

// cancelTask cancels a running task (called by WebSocket handler).
func (s *Server) cancelTask(id string) (map[string]any, error) {
	s.runningTasksMu.RLock()
	cancel, exists := s.runningTasks[id]
	s.runningTasksMu.RUnlock()

	if exists {
		cancel()
	}

	t, err := task.Load(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	t.Status = task.StatusFailed
	if err := t.Save(); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return map[string]any{
		"status":  "cancelled",
		"task_id": id,
	}, nil
}

// Publisher returns the event publisher for external use.
func (s *Server) Publisher() events.Publisher {
	return s.publisher
}

// handleListPrompts returns all available prompts.
func (s *Server) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	svc := prompt.DefaultService()
	prompts, err := svc.List()
	if err != nil {
		s.jsonError(w, "failed to list prompts", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, prompts)
}

// handleGetPromptVariables returns template variable documentation.
func (s *Server) handleGetPromptVariables(w http.ResponseWriter, r *http.Request) {
	vars := prompt.GetVariableReference()
	s.jsonResponse(w, vars)
}

// handleGetPrompt returns a specific prompt by phase.
func (s *Server) handleGetPrompt(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")
	svc := prompt.DefaultService()

	p, err := svc.Get(phase)
	if err != nil {
		s.jsonError(w, "prompt not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, p)
}

// handleGetPromptDefault returns the embedded default prompt for a phase.
func (s *Server) handleGetPromptDefault(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")
	svc := prompt.DefaultService()

	p, err := svc.GetDefault(phase)
	if err != nil {
		s.jsonError(w, "default prompt not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, p)
}

// handleSavePrompt saves a project prompt override.
func (s *Server) handleSavePrompt(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")

	var req struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		s.jsonError(w, "content is required", http.StatusBadRequest)
		return
	}

	svc := prompt.DefaultService()
	if err := svc.Save(phase, req.Content); err != nil {
		s.jsonError(w, "failed to save prompt", http.StatusInternalServerError)
		return
	}

	// Return updated prompt
	p, err := svc.Get(phase)
	if err != nil {
		s.jsonError(w, "failed to reload prompt", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, p)
}

// handleDeletePrompt deletes a project prompt override.
func (s *Server) handleDeletePrompt(w http.ResponseWriter, r *http.Request) {
	phase := r.PathValue("phase")
	svc := prompt.DefaultService()

	// Check if override exists
	if !svc.HasOverride(phase) {
		s.jsonError(w, "no override exists for this phase", http.StatusNotFound)
		return
	}

	if err := svc.Delete(phase); err != nil {
		s.jsonError(w, "failed to delete prompt", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// === Hooks Handlers ===

// handleListHooks returns all hooks.
func (s *Server) handleListHooks(w http.ResponseWriter, r *http.Request) {
	svc := hooks.DefaultService()
	hookList, err := svc.List()
	if err != nil {
		s.jsonError(w, "failed to list hooks", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, hookList)
}

// handleGetHookTypes returns available hook types.
func (s *Server) handleGetHookTypes(w http.ResponseWriter, r *http.Request) {
	types := hooks.GetHookTypes()
	s.jsonResponse(w, types)
}

// handleGetHook returns a specific hook.
func (s *Server) handleGetHook(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := hooks.DefaultService()

	hook, err := svc.Get(name)
	if err != nil {
		s.jsonError(w, "hook not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, hook)
}

// handleCreateHook creates a new hook.
func (s *Server) handleCreateHook(w http.ResponseWriter, r *http.Request) {
	var hook hooks.Hook
	if err := json.NewDecoder(r.Body).Decode(&hook); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := hooks.DefaultService()
	if err := svc.Create(hook); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, hook)
}

// handleUpdateHook updates an existing hook.
func (s *Server) handleUpdateHook(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var hook hooks.Hook
	if err := json.NewDecoder(r.Body).Decode(&hook); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := hooks.DefaultService()
	if err := svc.Update(name, hook); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return updated hook
	updated, _ := svc.Get(hook.Name)
	if updated == nil {
		updated, _ = svc.Get(name)
	}
	s.jsonResponse(w, updated)
}

// handleDeleteHook deletes a hook.
func (s *Server) handleDeleteHook(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := hooks.DefaultService()

	if err := svc.Delete(name); err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// === Skills Handlers ===

// handleListSkills returns all skills.
func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	svc := skills.DefaultService()
	skillList, err := svc.List()
	if err != nil {
		s.jsonError(w, "failed to list skills", http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, skillList)
}

// handleGetSkill returns a specific skill.
func (s *Server) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := skills.DefaultService()

	skill, err := svc.Get(name)
	if err != nil {
		s.jsonError(w, "skill not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, skill)
}

// handleCreateSkill creates a new skill.
func (s *Server) handleCreateSkill(w http.ResponseWriter, r *http.Request) {
	var skill skills.Skill
	if err := json.NewDecoder(r.Body).Decode(&skill); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := skills.DefaultService()
	if err := svc.Create(skill); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, skill)
}

// handleUpdateSkill updates an existing skill.
func (s *Server) handleUpdateSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var skill skills.Skill
	if err := json.NewDecoder(r.Body).Decode(&skill); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := skills.DefaultService()
	if err := svc.Update(name, skill); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return updated skill
	updated, _ := svc.Get(skill.Name)
	if updated == nil {
		updated, _ = svc.Get(name)
	}
	s.jsonResponse(w, updated)
}

// handleDeleteSkill deletes a skill.
func (s *Server) handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := skills.DefaultService()

	if err := svc.Delete(name); err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
