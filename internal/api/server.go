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
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/diff"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// Server is the orc API server.
type Server struct {
	addr    string
	workDir string // Project directory
	mux     *http.ServeMux
	logger  *slog.Logger

	// Orc configuration
	orcConfig *config.Config

	// Event publisher for real-time updates
	publisher events.Publisher
	wsHandler *WSHandler

	// Storage backend
	backend storage.Backend

	// SSE subscribers per task (legacy, kept for compatibility)
	subscribers   map[string][]chan Event
	subscribersMu sync.RWMutex

	// Running tasks for cancellation
	runningTasks   map[string]context.CancelFunc
	runningTasksMu sync.RWMutex

	// Diff cache for computed diffs
	diffCache *diff.Cache

	// PR status poller for periodic updates
	prPoller *PRPoller
}

// Event represents an SSE event.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Config holds server configuration.
type Config struct {
	Addr    string
	WorkDir string // Project directory (defaults to ".")
	Logger  *slog.Logger
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() *Config {
	return &Config{
		Addr:    ":8080",
		WorkDir: ".",
		Logger:  slog.Default(),
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

	// Set default work directory
	workDir := cfg.WorkDir
	if workDir == "" {
		workDir = "."
	}

	// Load orc configuration from the work directory
	configPath := filepath.Join(workDir, ".orc", "config.yaml")
	orcCfg, err := config.LoadFrom(configPath)
	if err != nil {
		logger.Warn("failed to load orc config, using defaults", "error", err)
		orcCfg = config.Default()
	}

	// Create event publisher
	pub := events.NewMemoryPublisher()

	// Create storage backend (database-only mode)
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(workDir, storageCfg)
	if err != nil {
		logger.Error("failed to create storage backend", "error", err)
		return nil
	}

	s := &Server{
		addr:         cfg.Addr,
		workDir:      workDir,
		mux:          http.NewServeMux(),
		logger:       logger,
		orcConfig:    orcCfg,
		publisher:    pub,
		backend:      backend,
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
	s.mux.HandleFunc("PATCH /api/tasks/{id}", cors(s.handleUpdateTask))
	s.mux.HandleFunc("DELETE /api/tasks/{id}", cors(s.handleDeleteTask))

	// Task state and plan
	s.mux.HandleFunc("GET /api/tasks/{id}/state", cors(s.handleGetState))
	s.mux.HandleFunc("GET /api/tasks/{id}/plan", cors(s.handleGetPlan))
	s.mux.HandleFunc("GET /api/tasks/{id}/transcripts", cors(s.handleGetTranscripts))
	s.mux.HandleFunc("GET /api/tasks/{id}/session", cors(s.handleGetSession))
	s.mux.HandleFunc("GET /api/tasks/{id}/tokens", cors(s.handleGetTokens))

	// Task attachments
	s.mux.HandleFunc("GET /api/tasks/{id}/attachments", cors(s.handleListAttachments))
	s.mux.HandleFunc("POST /api/tasks/{id}/attachments", cors(s.handleUploadAttachment))
	s.mux.HandleFunc("GET /api/tasks/{id}/attachments/{filename}", cors(s.handleGetAttachment))
	s.mux.HandleFunc("DELETE /api/tasks/{id}/attachments/{filename}", cors(s.handleDeleteAttachment))

	// Task test results (Playwright)
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results", cors(s.handleGetTestResults))
	s.mux.HandleFunc("POST /api/tasks/{id}/test-results", cors(s.handleSaveTestReport))
	s.mux.HandleFunc("POST /api/tasks/{id}/test-results/init", cors(s.handleInitTestResults))
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results/screenshots", cors(s.handleListScreenshots))
	s.mux.HandleFunc("POST /api/tasks/{id}/test-results/screenshots", cors(s.handleUploadScreenshot))
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results/screenshots/{filename}", cors(s.handleGetScreenshot))
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results/report", cors(s.handleGetHTMLReport))
	s.mux.HandleFunc("GET /api/tasks/{id}/test-results/traces/{filename}", cors(s.handleGetTrace))

	// Task dependencies
	s.mux.HandleFunc("GET /api/tasks/{id}/dependencies", cors(s.handleGetDependencies))

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

	// Task finalize (sync with target branch, resolve conflicts, run tests)
	s.mux.HandleFunc("POST /api/tasks/{id}/finalize", cors(s.handleFinalizeTask))
	s.mux.HandleFunc("GET /api/tasks/{id}/finalize", cors(s.handleGetFinalizeStatus))

	// Task export (export artifacts to branch or directory)
	s.mux.HandleFunc("POST /api/tasks/{id}/export", cors(s.handleExportTask))

	// Export configuration
	s.mux.HandleFunc("GET /api/config/export", cors(s.handleGetExportConfig))
	s.mux.HandleFunc("PUT /api/config/export", cors(s.handleUpdateExportConfig))

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
	s.mux.HandleFunc("DELETE /api/initiatives/{id}/tasks/{taskId}", cors(s.handleRemoveInitiativeTask))
	s.mux.HandleFunc("POST /api/initiatives/{id}/decisions", cors(s.handleAddInitiativeDecision))
	s.mux.HandleFunc("GET /api/initiatives/{id}/ready", cors(s.handleGetReadyTasks))
	s.mux.HandleFunc("GET /api/initiatives/{id}/dependency-graph", cors(s.handleGetInitiativeDependencyGraph))

	// Task dependency graph (for arbitrary set of tasks)
	s.mux.HandleFunc("GET /api/tasks/dependency-graph", cors(s.handleGetTasksDependencyGraph))

	// Subtasks (proposed sub-tasks queue)
	s.mux.HandleFunc("GET /api/tasks/{taskId}/subtasks", cors(s.handleListSubtasks))
	s.mux.HandleFunc("GET /api/tasks/{taskId}/subtasks/pending", cors(s.handleListPendingSubtasks))
	s.mux.HandleFunc("POST /api/subtasks", cors(s.handleCreateSubtask))
	s.mux.HandleFunc("GET /api/subtasks/{id}", cors(s.handleGetSubtask))
	s.mux.HandleFunc("POST /api/subtasks/{id}/approve", cors(s.handleApproveSubtask))
	s.mux.HandleFunc("POST /api/subtasks/{id}/reject", cors(s.handleRejectSubtask))
	s.mux.HandleFunc("DELETE /api/subtasks/{id}", cors(s.handleDeleteSubtask))

	// Knowledge queue (patterns, gotchas, decisions)
	s.mux.HandleFunc("GET /api/knowledge", cors(s.handleListKnowledge))
	s.mux.HandleFunc("GET /api/knowledge/status", cors(s.handleGetKnowledgeStatus))
	s.mux.HandleFunc("GET /api/knowledge/stale", cors(s.handleListStaleKnowledge))
	s.mux.HandleFunc("POST /api/knowledge", cors(s.handleCreateKnowledge))
	s.mux.HandleFunc("POST /api/knowledge/approve-all", cors(s.handleApproveAllKnowledge))
	s.mux.HandleFunc("GET /api/knowledge/{id}", cors(s.handleGetKnowledge))
	s.mux.HandleFunc("POST /api/knowledge/{id}/approve", cors(s.handleApproveKnowledge))
	s.mux.HandleFunc("POST /api/knowledge/{id}/reject", cors(s.handleRejectKnowledge))
	s.mux.HandleFunc("POST /api/knowledge/{id}/validate", cors(s.handleValidateKnowledge))
	s.mux.HandleFunc("DELETE /api/knowledge/{id}", cors(s.handleDeleteKnowledge))

	// Review comments (code review UI)
	s.mux.HandleFunc("GET /api/tasks/{id}/review/comments", cors(s.handleListReviewComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/review/comments", cors(s.handleCreateReviewComment))
	s.mux.HandleFunc("GET /api/tasks/{id}/review/comments/{commentId}", cors(s.handleGetReviewComment))
	s.mux.HandleFunc("PATCH /api/tasks/{id}/review/comments/{commentId}", cors(s.handleUpdateReviewComment))
	s.mux.HandleFunc("DELETE /api/tasks/{id}/review/comments/{commentId}", cors(s.handleDeleteReviewComment))
	s.mux.HandleFunc("POST /api/tasks/{id}/review/retry", cors(s.handleReviewRetry))
	s.mux.HandleFunc("GET /api/tasks/{id}/review/stats", cors(s.handleGetReviewStats))

	// Task comments (general notes/discussion)
	s.mux.HandleFunc("GET /api/tasks/{id}/comments", cors(s.handleListTaskComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/comments", cors(s.handleCreateTaskComment))
	s.mux.HandleFunc("GET /api/tasks/{id}/comments/stats", cors(s.handleGetTaskCommentStats))
	s.mux.HandleFunc("GET /api/tasks/{id}/comments/{commentId}", cors(s.handleGetTaskComment))
	s.mux.HandleFunc("PATCH /api/tasks/{id}/comments/{commentId}", cors(s.handleUpdateTaskComment))
	s.mux.HandleFunc("DELETE /api/tasks/{id}/comments/{commentId}", cors(s.handleDeleteTaskComment))

	// GitHub PR integration
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr", cors(s.handleCreatePR))
	s.mux.HandleFunc("GET /api/tasks/{id}/github/pr", cors(s.handleGetPR))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/merge", cors(s.handleMergePR))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/sync", cors(s.handleSyncPRComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/import", cors(s.handleImportPRComments))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/{commentId}/autofix", cors(s.handleAutoFixComment))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/comments/{commentId}/reply", cors(s.handleReplyToPRComment))
	s.mux.HandleFunc("GET /api/tasks/{id}/github/pr/checks", cors(s.handleListPRChecks))
	s.mux.HandleFunc("POST /api/tasks/{id}/github/pr/refresh", cors(s.handleRefreshPRStatus))

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
	s.mux.HandleFunc("PUT /api/settings/global", cors(s.handleUpdateGlobalSettings))
	s.mux.HandleFunc("GET /api/settings/project", cors(s.handleGetProjectSettings))
	s.mux.HandleFunc("GET /api/settings/hierarchy", cors(s.handleGetSettingsHierarchy))
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

	// Plugins - Local (.claude/plugins/)
	s.mux.HandleFunc("GET /api/plugins", cors(s.handleListPlugins))
	s.mux.HandleFunc("GET /api/plugins/resources", cors(s.handleListPluginResources))
	s.mux.HandleFunc("GET /api/plugins/updates", cors(s.handleCheckPluginUpdates))
	s.mux.HandleFunc("GET /api/plugins/{name}", cors(s.handleGetPlugin))
	s.mux.HandleFunc("GET /api/plugins/{name}/commands", cors(s.handleListPluginCommands))
	s.mux.HandleFunc("POST /api/plugins/{name}/enable", cors(s.handleEnablePlugin))
	s.mux.HandleFunc("POST /api/plugins/{name}/disable", cors(s.handleDisablePlugin))
	s.mux.HandleFunc("POST /api/plugins/{name}/update", cors(s.handleUpdatePlugin))
	s.mux.HandleFunc("DELETE /api/plugins/{name}", cors(s.handleUninstallPlugin))

	// Plugins - Marketplace (separate prefix to avoid route conflicts)
	s.mux.HandleFunc("GET /api/marketplace/plugins", cors(s.handleBrowseMarketplace))
	s.mux.HandleFunc("GET /api/marketplace/plugins/search", cors(s.handleSearchMarketplace))
	s.mux.HandleFunc("GET /api/marketplace/plugins/{name}", cors(s.handleGetMarketplacePlugin))
	s.mux.HandleFunc("POST /api/marketplace/plugins/{name}/install", cors(s.handleInstallPlugin))

	// Config (orc configuration)
	s.mux.HandleFunc("GET /api/config", cors(s.handleGetConfig))
	s.mux.HandleFunc("PUT /api/config", cors(s.handleUpdateConfig))

	// Projects
	s.mux.HandleFunc("GET /api/projects", cors(s.handleListProjects))
	s.mux.HandleFunc("GET /api/projects/default", cors(s.handleGetDefaultProject))
	s.mux.HandleFunc("PUT /api/projects/default", cors(s.handleSetDefaultProject))
	s.mux.HandleFunc("GET /api/projects/{id}", cors(s.handleGetProject))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks", cors(s.handleListProjectTasks))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks", cors(s.handleCreateProjectTask))
	s.mux.HandleFunc("GET /api/projects/{id}/tasks/{taskId}", cors(s.handleGetProjectTask))
	s.mux.HandleFunc("DELETE /api/projects/{id}/tasks/{taskId}", cors(s.handleDeleteProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/run", cors(s.handleRunProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/pause", cors(s.handlePauseProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/resume", cors(s.handleResumeProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/rewind", cors(s.handleRewindProjectTask))
	s.mux.HandleFunc("POST /api/projects/{id}/tasks/{taskId}/escalate", cors(s.handleEscalateProjectTask))
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

	// Team (team mode infrastructure)
	s.mux.HandleFunc("GET /api/team/members", cors(s.handleListTeamMembers))
	s.mux.HandleFunc("POST /api/team/members", cors(s.handleCreateTeamMember))
	s.mux.HandleFunc("GET /api/team/members/{id}", cors(s.handleGetTeamMember))
	s.mux.HandleFunc("PUT /api/team/members/{id}", cors(s.handleUpdateTeamMember))
	s.mux.HandleFunc("DELETE /api/team/members/{id}", cors(s.handleDeleteTeamMember))
	s.mux.HandleFunc("GET /api/team/members/{id}/claims", cors(s.handleGetMemberClaims))
	s.mux.HandleFunc("POST /api/tasks/{id}/claim", cors(s.handleClaimTask))
	s.mux.HandleFunc("POST /api/tasks/{id}/release", cors(s.handleReleaseTask))
	s.mux.HandleFunc("GET /api/tasks/{id}/claim", cors(s.handleGetTaskClaim))
	s.mux.HandleFunc("GET /api/team/activity", cors(s.handleListActivity))

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

	// Create and start PR status poller
	s.prPoller = NewPRPoller(PRPollerConfig{
		WorkDir:   s.workDir,
		Interval:  60 * time.Second,
		Logger:    s.logger,
		OrcConfig: s.orcConfig,
		Backend:   s.backend,
		OnStatusChange: func(taskID string, pr *task.PRInfo) {
			// Publish task update event when PR status changes
			s.logger.Info("PR status changed", "task", taskID, "status", pr.Status)
			s.publisher.Publish(events.Event{
				Type:   events.EventTaskUpdated,
				TaskID: taskID,
				Data:   map[string]any{"pr": pr},
			})

			// Auto-trigger finalize when PR is approved (if enabled in config)
			if pr.Status == task.PRStatusApproved {
				triggered, err := s.TriggerFinalizeOnApproval(taskID)
				if err != nil {
					s.logger.Error("failed to auto-trigger finalize", "task", taskID, "error", err)
				} else if triggered {
					s.logger.Info("finalize auto-triggered on PR approval", "task", taskID)
				}
			}
		},
	})
	s.prPoller.Start(ctx)

	go func() {
		<-ctx.Done()
		// Stop PR poller
		if s.prPoller != nil {
			s.prPoller.Stop()
		}
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

// Backend returns the storage backend (for testing).
func (s *Server) Backend() storage.Backend {
	return s.backend
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
	t, err := s.backend.LoadTask(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	t.Status = task.StatusPaused
	if err := s.backend.SaveTask(t); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return map[string]any{
		"status":  "paused",
		"task_id": id,
	}, nil
}

// resumeTask resumes a paused task (called by WebSocket handler).
func (s *Server) resumeTask(id string) (map[string]any, error) {
	t, err := s.backend.LoadTask(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	// If task was paused, restart execution
	if t.Status == task.StatusPaused {
		t.Status = task.StatusRunning
		if err := s.backend.SaveTask(t); err != nil {
			return nil, fmt.Errorf("failed to update task: %w", err)
		}

		// Resume execution
		p, err := s.backend.LoadPlan(id)
		if err != nil {
			return nil, fmt.Errorf("plan not found")
		}

		st, err := s.backend.LoadState(id)
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

			execCfg := executor.ConfigFromOrc(s.orcConfig)
			execCfg.WorkDir = s.workDir
			exec := executor.NewWithConfig(execCfg, s.orcConfig)
			exec.SetBackend(s.backend)
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

	t, err := s.backend.LoadTask(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	t.Status = task.StatusFailed
	if err := s.backend.SaveTask(t); err != nil {
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
	// Use workDir if set, otherwise fall back to current working directory
	if s.workDir != "" {
		return s.workDir
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return wd
}
