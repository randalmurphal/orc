// Package api provides the REST API and SSE server for orc.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/diff"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/events"
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

	// Event publisher for real-time updates
	publisher events.Publisher
	wsHandler *WSHandler

	// SSE subscribers per task (legacy, kept for compatibility)
	subscribers   map[string][]chan Event
	subscribersMu sync.RWMutex

	// Running tasks for cancellation
	runningTasks   map[string]context.CancelFunc
	runningTasksMu sync.RWMutex

	// Diff cache for computed diffs
	diffCache *diff.Cache
}

// Event represents an SSE event.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
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
		diffCache:    diff.NewCache(100), // Cache up to 100 file diffs
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
	s.mux.HandleFunc("GET /api/tasks/{id}/session", cors(s.handleGetSession))
	s.mux.HandleFunc("GET /api/tasks/{id}/tokens", cors(s.handleGetTokens))

	// Task diff (git changes visualization)
	s.mux.HandleFunc("GET /api/tasks/{id}/diff", cors(s.handleGetDiff))
	s.mux.HandleFunc("GET /api/tasks/{id}/diff/stats", cors(s.handleGetDiffStats))
	s.mux.HandleFunc("GET /api/tasks/{id}/diff/file/{path...}", cors(s.handleGetDiffFile))

	// Task control
	s.mux.HandleFunc("POST /api/tasks/{id}/run", cors(s.handleRunTask))
	s.mux.HandleFunc("POST /api/tasks/{id}/pause", cors(s.handlePauseTask))
	s.mux.HandleFunc("POST /api/tasks/{id}/resume", cors(s.handleResumeTask))

	// Task retry (fresh session with context injection)
	s.mux.HandleFunc("POST /api/tasks/{id}/retry", cors(s.handleRetryTask))
	s.mux.HandleFunc("GET /api/tasks/{id}/retry/preview", cors(s.handleGetRetryPreview))
	s.mux.HandleFunc("POST /api/tasks/{id}/retry/feedback", cors(s.handleRetryWithFeedback))

	// SSE streaming (legacy)
	s.mux.HandleFunc("GET /api/tasks/{id}/stream", s.handleStream)

	// Initiatives
	s.mux.HandleFunc("GET /api/initiatives", cors(s.handleListInitiatives))
	s.mux.HandleFunc("POST /api/initiatives", cors(s.handleCreateInitiative))
	s.mux.HandleFunc("GET /api/initiatives/{id}", cors(s.handleGetInitiative))
	s.mux.HandleFunc("PUT /api/initiatives/{id}", cors(s.handleUpdateInitiative))
	s.mux.HandleFunc("DELETE /api/initiatives/{id}", cors(s.handleDeleteInitiative))
	s.mux.HandleFunc("GET /api/initiatives/{id}/tasks", cors(s.handleListInitiativeTasks))
	s.mux.HandleFunc("POST /api/initiatives/{id}/tasks", cors(s.handleAddInitiativeTask))
	s.mux.HandleFunc("POST /api/initiatives/{id}/decisions", cors(s.handleAddInitiativeDecision))
	s.mux.HandleFunc("GET /api/initiatives/{id}/ready", cors(s.handleGetReadyTasks))

	// Subtasks (proposed sub-tasks queue)
	s.mux.HandleFunc("GET /api/tasks/{taskId}/subtasks", cors(s.handleListSubtasks))
	s.mux.HandleFunc("GET /api/tasks/{taskId}/subtasks/pending", cors(s.handleListPendingSubtasks))
	s.mux.HandleFunc("POST /api/subtasks", cors(s.handleCreateSubtask))
	s.mux.HandleFunc("GET /api/subtasks/{id}", cors(s.handleGetSubtask))
	s.mux.HandleFunc("POST /api/subtasks/{id}/approve", cors(s.handleApproveSubtask))
	s.mux.HandleFunc("POST /api/subtasks/{id}/reject", cors(s.handleRejectSubtask))
	s.mux.HandleFunc("DELETE /api/subtasks/{id}", cors(s.handleDeleteSubtask))

	// Review comments (code review UI)
	s.mux.HandleFunc("GET /api/tasks/{id}/review/comments", cors(s.handleListReviewComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/review/comments", cors(s.handleCreateReviewComment))
	s.mux.HandleFunc("GET /api/tasks/{id}/review/comments/{commentId}", cors(s.handleGetReviewComment))
	s.mux.HandleFunc("PATCH /api/tasks/{id}/review/comments/{commentId}", cors(s.handleUpdateReviewComment))
	s.mux.HandleFunc("DELETE /api/tasks/{id}/review/comments/{commentId}", cors(s.handleDeleteReviewComment))
	s.mux.HandleFunc("POST /api/tasks/{id}/review/retry", cors(s.handleReviewRetry))
	s.mux.HandleFunc("GET /api/tasks/{id}/review/stats", cors(s.handleGetReviewStats))

	// GitHub PR integration
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr", cors(s.handleCreatePR))
	s.mux.HandleFunc("GET /api/tasks/{id}/github/pr", cors(s.handleGetPR))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/merge", cors(s.handleMergePR))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/sync", cors(s.handleSyncPRComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/{commentId}/autofix", cors(s.handleAutoFixComment))
	s.mux.HandleFunc("GET /api/tasks/{id}/github/pr/checks", cors(s.handleListPRChecks))

	// WebSocket for real-time updates
	s.mux.Handle("GET /api/ws", s.wsHandler)

	// Cost aggregation
	s.mux.HandleFunc("GET /api/cost/summary", cors(s.handleGetCostSummary))

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
	s.mux.HandleFunc("GET /api/settings/global", cors(s.handleGetGlobalSettings))
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

	// MCP Servers (.mcp.json)
	s.mux.HandleFunc("GET /api/mcp", cors(s.handleListMCPServers))
	s.mux.HandleFunc("POST /api/mcp", cors(s.handleCreateMCPServer))
	s.mux.HandleFunc("GET /api/mcp/{name}", cors(s.handleGetMCPServer))
	s.mux.HandleFunc("PUT /api/mcp/{name}", cors(s.handleUpdateMCPServer))
	s.mux.HandleFunc("DELETE /api/mcp/{name}", cors(s.handleDeleteMCPServer))

	// Config (orc configuration)
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

	// Templates
	s.mux.HandleFunc("GET /api/templates", cors(s.handleListTemplates))
	s.mux.HandleFunc("POST /api/templates", cors(s.handleCreateTemplate))
	s.mux.HandleFunc("GET /api/templates/{name}", cors(s.handleGetTemplate))
	s.mux.HandleFunc("DELETE /api/templates/{name}", cors(s.handleDeleteTemplate))

	// Dashboard
	s.mux.HandleFunc("GET /api/dashboard/stats", cors(s.handleGetDashboardStats))

	// Static files (embedded frontend) - catch-all for non-API routes
	s.mux.Handle("/", staticHandler())
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

// jsonResponse writes a JSON response.
func (s *Server) jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// jsonError writes a JSON error response.
func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// handleOrcError writes a structured JSON error response for OrcErrors.
func (s *Server) handleOrcError(w http.ResponseWriter, err *orcerrors.OrcError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus())
	json.NewEncoder(w).Encode(err.ToAPIError())
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


// getProjectRoot returns the current project root directory.
func (s *Server) getProjectRoot() string {
	// Use current working directory as project root
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
