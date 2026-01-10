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

	"github.com/randalmurphal/llmkit/claudeconfig"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/project"
	"github.com/randalmurphal/orc/internal/prompt"
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

	// Skills (SKILL.md format)
	s.mux.HandleFunc("GET /api/skills", cors(s.handleListSkills))
	s.mux.HandleFunc("POST /api/skills", cors(s.handleCreateSkill))
	s.mux.HandleFunc("GET /api/skills/{name}", cors(s.handleGetSkill))
	s.mux.HandleFunc("PUT /api/skills/{name}", cors(s.handleUpdateSkill))
	s.mux.HandleFunc("DELETE /api/skills/{name}", cors(s.handleDeleteSkill))

	// Settings (Claude Code settings.json with inheritance)
	s.mux.HandleFunc("GET /api/settings", cors(s.handleGetSettings))
	s.mux.HandleFunc("GET /api/settings/project", cors(s.handleGetProjectSettings))
	s.mux.HandleFunc("PUT /api/settings", cors(s.handleUpdateSettings))

	// Tools (available Claude Code tools with permissions)
	s.mux.HandleFunc("GET /api/tools", cors(s.handleListTools))
	s.mux.HandleFunc("GET /api/tools/permissions", cors(s.handleGetToolPermissions))
	s.mux.HandleFunc("PUT /api/tools/permissions", cors(s.handleUpdateToolPermissions))

	// Agents (sub-agent definitions)
	s.mux.HandleFunc("GET /api/agents", cors(s.handleListAgents))
	s.mux.HandleFunc("POST /api/agents", cors(s.handleCreateAgent))
	s.mux.HandleFunc("GET /api/agents/{name}", cors(s.handleGetAgent))
	s.mux.HandleFunc("PUT /api/agents/{name}", cors(s.handleUpdateAgent))
	s.mux.HandleFunc("DELETE /api/agents/{name}", cors(s.handleDeleteAgent))

	// Scripts (project script registry)
	s.mux.HandleFunc("GET /api/scripts", cors(s.handleListScripts))
	s.mux.HandleFunc("POST /api/scripts", cors(s.handleCreateScript))
	s.mux.HandleFunc("POST /api/scripts/discover", cors(s.handleDiscoverScripts))
	s.mux.HandleFunc("GET /api/scripts/{name}", cors(s.handleGetScript))
	s.mux.HandleFunc("PUT /api/scripts/{name}", cors(s.handleUpdateScript))
	s.mux.HandleFunc("DELETE /api/scripts/{name}", cors(s.handleDeleteScript))

	// CLAUDE.md
	s.mux.HandleFunc("GET /api/claudemd", cors(s.handleGetClaudeMD))
	s.mux.HandleFunc("PUT /api/claudemd", cors(s.handleUpdateClaudeMD))
	s.mux.HandleFunc("GET /api/claudemd/hierarchy", cors(s.handleGetClaudeMDHierarchy))

	// Config (orc configuration)
	s.mux.HandleFunc("GET /api/config", cors(s.handleGetConfig))
	s.mux.HandleFunc("PUT /api/config", cors(s.handleUpdateConfig))

	// Projects
	s.mux.HandleFunc("GET /api/projects", cors(s.handleListProjects))
	s.mux.HandleFunc("GET /api/projects/{id}", cors(s.handleGetProject))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks", cors(s.handleListProjectTasks))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks", cors(s.handleCreateProjectTask))
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

// getProjectRoot returns the current project root directory.
func (s *Server) getProjectRoot() string {
	// Use current working directory as project root
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}

// === Hooks Handlers (settings.json format) ===

// handleListHooks returns all hooks from settings.json.
func (s *Server) handleListHooks(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadProjectSettings(s.getProjectRoot())
	if err != nil {
		// No settings file is OK - return empty hooks
		s.jsonResponse(w, map[string][]claudeconfig.Hook{})
		return
	}

	hooks := settings.Hooks
	if hooks == nil {
		hooks = make(map[string][]claudeconfig.Hook)
	}

	s.jsonResponse(w, hooks)
}

// handleGetHookTypes returns available hook event types.
func (s *Server) handleGetHookTypes(w http.ResponseWriter, r *http.Request) {
	events := claudeconfig.ValidHookEvents()
	s.jsonResponse(w, events)
}

// handleGetHook returns hooks for a specific event type.
func (s *Server) handleGetHook(w http.ResponseWriter, r *http.Request) {
	eventName := r.PathValue("name")

	settings, err := claudeconfig.LoadProjectSettings(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "settings not found", http.StatusNotFound)
		return
	}

	hooks, exists := settings.Hooks[eventName]
	if !exists {
		s.jsonError(w, "no hooks for this event", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, hooks)
}

// handleCreateHook adds a hook to settings.json.
func (s *Server) handleCreateHook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Event string           `json:"event"`
		Hook  claudeconfig.Hook `json:"hook"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		settings = &claudeconfig.Settings{}
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]claudeconfig.Hook)
	}

	settings.Hooks[req.Event] = append(settings.Hooks[req.Event], req.Hook)

	if err := claudeconfig.SaveProjectSettings(projectRoot, settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, req.Hook)
}

// handleUpdateHook updates hooks for a specific event.
func (s *Server) handleUpdateHook(w http.ResponseWriter, r *http.Request) {
	eventName := r.PathValue("name")

	var hooks []claudeconfig.Hook
	if err := json.NewDecoder(r.Body).Decode(&hooks); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		settings = &claudeconfig.Settings{}
	}

	if settings.Hooks == nil {
		settings.Hooks = make(map[string][]claudeconfig.Hook)
	}

	settings.Hooks[eventName] = hooks

	if err := claudeconfig.SaveProjectSettings(projectRoot, settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, hooks)
}

// handleDeleteHook removes all hooks for an event type.
func (s *Server) handleDeleteHook(w http.ResponseWriter, r *http.Request) {
	eventName := r.PathValue("name")

	projectRoot := s.getProjectRoot()
	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		s.jsonError(w, "settings not found", http.StatusNotFound)
		return
	}

	if settings.Hooks == nil {
		s.jsonError(w, "no hooks configured", http.StatusNotFound)
		return
	}

	delete(settings.Hooks, eventName)

	if err := claudeconfig.SaveProjectSettings(projectRoot, settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// === Skills Handlers (SKILL.md format) ===

// handleListSkills returns all skills from .claude/skills/.
func (s *Server) handleListSkills(w http.ResponseWriter, r *http.Request) {
	claudeDir := filepath.Join(s.getProjectRoot(), ".claude")
	skills, err := claudeconfig.DiscoverSkills(claudeDir)
	if err != nil {
		// No skills directory is OK - return empty list
		s.jsonResponse(w, []claudeconfig.SkillInfo{})
		return
	}

	// Convert to SkillInfo for listing
	infos := make([]claudeconfig.SkillInfo, 0, len(skills))
	for _, skill := range skills {
		infos = append(infos, claudeconfig.SkillInfo{
			Name:        skill.Name,
			Description: skill.Description,
			Path:        skill.Path,
		})
	}

	s.jsonResponse(w, infos)
}

// handleGetSkill returns a specific skill by name.
func (s *Server) handleGetSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	skillPath := filepath.Join(s.getProjectRoot(), ".claude", "skills", name, "SKILL.md")

	skill, err := claudeconfig.ParseSkillMD(skillPath)
	if err != nil {
		s.jsonError(w, "skill not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, skill)
}

// handleCreateSkill creates a new skill in SKILL.md format.
func (s *Server) handleCreateSkill(w http.ResponseWriter, r *http.Request) {
	var skill claudeconfig.Skill
	if err := json.NewDecoder(r.Body).Decode(&skill); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if skill.Name == "" {
		s.jsonError(w, "name is required", http.StatusBadRequest)
		return
	}

	skillsDir := filepath.Join(s.getProjectRoot(), ".claude", "skills")
	if err := claudeconfig.WriteSkillMD(&skill, skillsDir); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to create skill: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, skill)
}

// handleUpdateSkill updates an existing skill.
func (s *Server) handleUpdateSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	skillDir := filepath.Join(s.getProjectRoot(), ".claude", "skills", name)

	// Check if skill exists
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); os.IsNotExist(err) {
		s.jsonError(w, "skill not found", http.StatusNotFound)
		return
	}

	var skill claudeconfig.Skill
	if err := json.NewDecoder(r.Body).Decode(&skill); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// If name changed, we need to rename the directory
	if skill.Name != "" && skill.Name != name {
		newDir := filepath.Join(s.getProjectRoot(), ".claude", "skills", skill.Name)
		if err := os.Rename(skillDir, newDir); err != nil {
			s.jsonError(w, fmt.Sprintf("failed to rename skill: %v", err), http.StatusInternalServerError)
			return
		}
		skillDir = newDir
	} else {
		skill.Name = name
	}

	// Write the updated skill
	skillsDir := filepath.Join(s.getProjectRoot(), ".claude", "skills")
	if err := claudeconfig.WriteSkillMD(&skill, skillsDir); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to update skill: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, skill)
}

// handleDeleteSkill deletes a skill directory.
func (s *Server) handleDeleteSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	skillDir := filepath.Join(s.getProjectRoot(), ".claude", "skills", name)

	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		s.jsonError(w, "skill not found", http.StatusNotFound)
		return
	}

	if err := os.RemoveAll(skillDir); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to delete skill: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// === Settings Handlers ===

// handleGetSettings returns merged settings (global + project).
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadSettings(s.getProjectRoot())
	if err != nil {
		// Return empty settings on error
		s.jsonResponse(w, &claudeconfig.Settings{})
		return
	}

	s.jsonResponse(w, settings)
}

// handleGetProjectSettings returns project-only settings.
func (s *Server) handleGetProjectSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadProjectSettings(s.getProjectRoot())
	if err != nil {
		s.jsonResponse(w, &claudeconfig.Settings{})
		return
	}

	s.jsonResponse(w, settings)
}

// handleUpdateSettings saves project settings.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings claudeconfig.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := claudeconfig.SaveProjectSettings(s.getProjectRoot(), &settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, settings)
}

// === Tools Handlers ===

// handleListTools returns all available Claude Code tools.
func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	// Check if grouping by category is requested
	if r.URL.Query().Get("by_category") == "true" {
		byCategory := claudeconfig.ToolsByCategory()
		s.jsonResponse(w, byCategory)
		return
	}

	tools := claudeconfig.AvailableTools()
	s.jsonResponse(w, tools)
}

// handleGetToolPermissions returns tool permission settings.
func (s *Server) handleGetToolPermissions(w http.ResponseWriter, r *http.Request) {
	settings, err := claudeconfig.LoadProjectSettings(s.getProjectRoot())
	if err != nil {
		// No settings = no permissions configured
		s.jsonResponse(w, &claudeconfig.ToolPermissions{})
		return
	}

	var perms *claudeconfig.ToolPermissions
	if err := settings.GetExtension("tool_permissions", &perms); err != nil || perms == nil {
		s.jsonResponse(w, &claudeconfig.ToolPermissions{})
		return
	}

	s.jsonResponse(w, perms)
}

// handleUpdateToolPermissions saves tool permission settings.
func (s *Server) handleUpdateToolPermissions(w http.ResponseWriter, r *http.Request) {
	var perms claudeconfig.ToolPermissions
	if err := json.NewDecoder(r.Body).Decode(&perms); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		settings = &claudeconfig.Settings{}
	}

	settings.SetExtension("tool_permissions", perms)

	if err := claudeconfig.SaveProjectSettings(projectRoot, settings); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save settings: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, perms)
}

// === Agents Handlers ===

// handleListAgents returns all sub-agent definitions.
func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	svc := claudeconfig.NewAgentService(s.getProjectRoot())
	agents, err := svc.List()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to list agents: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, agents)
}

// handleGetAgent returns a specific agent by name.
func (s *Server) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := claudeconfig.NewAgentService(s.getProjectRoot())

	agent, err := svc.Get(name)
	if err != nil {
		s.jsonError(w, "agent not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, agent)
}

// handleCreateAgent creates a new sub-agent.
func (s *Server) handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	var agent claudeconfig.SubAgent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := claudeconfig.NewAgentService(s.getProjectRoot())
	if err := svc.Create(agent); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, agent)
}

// handleUpdateAgent updates an existing agent.
func (s *Server) handleUpdateAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var agent claudeconfig.SubAgent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := claudeconfig.NewAgentService(s.getProjectRoot())
	if err := svc.Update(name, agent); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return updated agent
	updated, _ := svc.Get(agent.Name)
	if updated == nil {
		updated, _ = svc.Get(name)
	}
	s.jsonResponse(w, updated)
}

// handleDeleteAgent deletes an agent.
func (s *Server) handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := claudeconfig.NewAgentService(s.getProjectRoot())

	if err := svc.Delete(name); err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// === Scripts Handlers ===

// handleListScripts returns all registered scripts.
func (s *Server) handleListScripts(w http.ResponseWriter, r *http.Request) {
	svc := claudeconfig.NewScriptService(s.getProjectRoot())
	scripts, err := svc.List()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to list scripts: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, scripts)
}

// handleGetScript returns a specific script by name.
func (s *Server) handleGetScript(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := claudeconfig.NewScriptService(s.getProjectRoot())

	script, err := svc.Get(name)
	if err != nil {
		s.jsonError(w, "script not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, script)
}

// handleCreateScript registers a new script.
func (s *Server) handleCreateScript(w http.ResponseWriter, r *http.Request) {
	var script claudeconfig.ProjectScript
	if err := json.NewDecoder(r.Body).Decode(&script); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := claudeconfig.NewScriptService(s.getProjectRoot())
	if err := svc.Create(script); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	s.jsonResponse(w, script)
}

// handleUpdateScript updates an existing script registration.
func (s *Server) handleUpdateScript(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	var script claudeconfig.ProjectScript
	if err := json.NewDecoder(r.Body).Decode(&script); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	svc := claudeconfig.NewScriptService(s.getProjectRoot())
	if err := svc.Update(name, script); err != nil {
		s.jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return updated script
	updated, _ := svc.Get(script.Name)
	if updated == nil {
		updated, _ = svc.Get(name)
	}
	s.jsonResponse(w, updated)
}

// handleDeleteScript removes a script registration.
func (s *Server) handleDeleteScript(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	svc := claudeconfig.NewScriptService(s.getProjectRoot())

	if err := svc.Delete(name); err != nil {
		s.jsonError(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDiscoverScripts auto-discovers scripts in .claude/scripts/.
func (s *Server) handleDiscoverScripts(w http.ResponseWriter, r *http.Request) {
	svc := claudeconfig.NewScriptService(s.getProjectRoot())
	discovered, err := svc.Discover()
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to discover scripts: %v", err), http.StatusInternalServerError)
		return
	}

	// Return discovered scripts (not yet registered)
	s.jsonResponse(w, discovered)
}

// === CLAUDE.md Handlers ===

// handleGetClaudeMD returns the project CLAUDE.md content.
func (s *Server) handleGetClaudeMD(w http.ResponseWriter, r *http.Request) {
	claudeMD, err := claudeconfig.LoadProjectClaudeMD(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, "CLAUDE.md not found", http.StatusNotFound)
		return
	}

	s.jsonResponse(w, claudeMD)
}

// handleUpdateClaudeMD saves the project CLAUDE.md.
func (s *Server) handleUpdateClaudeMD(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	projectRoot := s.getProjectRoot()
	if err := claudeconfig.SaveProjectClaudeMD(projectRoot, req.Content); err != nil {
		s.jsonError(w, fmt.Sprintf("failed to save CLAUDE.md: %v", err), http.StatusInternalServerError)
		return
	}

	// Return the saved content as a ClaudeMD response
	claudeMD := &claudeconfig.ClaudeMD{
		Path:    filepath.Join(projectRoot, "CLAUDE.md"),
		Content: req.Content,
		Source:  "project",
	}

	s.jsonResponse(w, claudeMD)
}

// handleGetClaudeMDHierarchy returns the full CLAUDE.md inheritance chain.
func (s *Server) handleGetClaudeMDHierarchy(w http.ResponseWriter, r *http.Request) {
	hierarchy, err := claudeconfig.LoadClaudeMDHierarchy(s.getProjectRoot())
	if err != nil {
		s.jsonError(w, fmt.Sprintf("failed to load hierarchy: %v", err), http.StatusInternalServerError)
		return
	}

	s.jsonResponse(w, hierarchy)
}
