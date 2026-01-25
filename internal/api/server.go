// Package api provides the REST API and SSE server for orc.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/diff"
	orcerrors "github.com/randalmurphal/orc/internal/errors"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// Server is the orc API server.
type Server struct {
	addr            string
	workDir         string // Project directory
	maxPortAttempts int    // Number of ports to try
	mux             *http.ServeMux
	logger          *slog.Logger

	// Orc configuration
	orcConfig *config.Config

	// Event publisher for real-time updates
	publisher events.Publisher
	wsHandler *WSHandler

	// Storage backend
	backend storage.Backend

	// Project database for workflow execution
	projectDB *db.ProjectDB

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

	// Automation service for trigger-based automation
	automationSvc *automation.Service

	// Pending gate decisions (for human approval gates in API mode)
	pendingDecisions *gate.PendingDecisionStore

	// Server context for graceful shutdown of background goroutines
	serverCtx       context.Context
	serverCtxCancel context.CancelFunc

	// Session tracking
	sessionID    string
	sessionStart time.Time
}

// Event represents an SSE event.
type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// Config holds server configuration.
type Config struct {
	Addr            string
	WorkDir         string // Project directory (defaults to ".")
	Logger          *slog.Logger
	MaxPortAttempts int // Number of ports to try if initial port is busy (default: 10)
}

// DefaultConfig returns the default server configuration.
func DefaultConfig() *Config {
	return &Config{
		Addr:            ":8080",
		WorkDir:         ".",
		Logger:          slog.Default(),
		MaxPortAttempts: 10,
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

	// Set default work directory - use FindProjectRoot for worktree awareness
	workDir := cfg.WorkDir
	if workDir == "" {
		var err error
		workDir, err = config.FindProjectRoot()
		if err != nil {
			// Fall back to cwd if not in a project (server may be started for init)
			workDir, err = os.Getwd()
			if err != nil {
				panic(fmt.Sprintf("cannot determine work directory: %v", err))
			}
			logger.Warn("server started outside orc project", "workDir", workDir)
		}
	}

	// Load orc configuration from the work directory
	orcCfg, err := config.LoadFrom(workDir)
	if err != nil {
		logger.Warn("failed to load orc config, using defaults", "error", err)
		orcCfg = config.Default()
	}

	// Create storage backend (database-only mode)
	storageCfg := &config.StorageConfig{Mode: "database"}
	backend, err := storage.NewDatabaseBackend(workDir, storageCfg)
	if err != nil {
		// Fatal error - server cannot function without storage backend
		panic(fmt.Sprintf("failed to create storage backend: %v", err))
	}

	// Seed built-in workflows and phase templates
	if seeded, err := workflow.SeedBuiltins(backend.DB()); err != nil {
		logger.Error("failed to seed built-in workflows", "error", err)
	} else if seeded > 0 {
		logger.Info("seeded built-in workflows", "count", seeded)
	}

	// Seed built-in agents and phase-agent associations
	if seeded, err := workflow.SeedAgents(backend.DB()); err != nil {
		logger.Error("failed to seed built-in agents", "error", err)
	} else if seeded > 0 {
		logger.Info("seeded built-in agents", "count", seeded)
	}

	// Migrate phase template model settings (updates existing templates)
	if migrated, err := workflow.MigratePhaseTemplateModels(backend.DB()); err != nil {
		logger.Error("failed to migrate phase template models", "error", err)
	} else if migrated > 0 {
		logger.Info("migrated phase template models", "count", migrated)
	}

	// Create event publisher with persistence
	pub := events.NewPersistentPublisher(backend, "executor", logger)

	// Create a background context for the server - will be replaced by StartContext
	serverCtx, serverCtxCancel := context.WithCancel(context.Background())

	// Set default max port attempts
	maxPortAttempts := cfg.MaxPortAttempts
	if maxPortAttempts <= 0 {
		maxPortAttempts = 10
	}

	// Create automation service if enabled
	var automationSvc *automation.Service
	if orcCfg.AutomationEnabled() {
		adapter := automation.NewProjectDBAdapter(backend.DB())
		automationSvc = automation.NewService(orcCfg, adapter, logger)

		// Create task creator for automation with efficient DB adapter
		taskCreator := automation.NewAutoTaskCreator(orcCfg, backend, logger,
			automation.WithDBAdapter(adapter))
		automationSvc.SetTaskCreator(taskCreator)

		logger.Info("automation service enabled")
	}

	s := &Server{
		addr:             cfg.Addr,
		workDir:          workDir,
		maxPortAttempts:  maxPortAttempts,
		mux:              http.NewServeMux(),
		logger:           logger,
		orcConfig:        orcCfg,
		publisher:        pub,
		backend:          backend,
		projectDB:        backend.DB(),
		subscribers:      make(map[string][]chan Event),
		runningTasks:     make(map[string]context.CancelFunc),
		diffCache:        diff.NewCache(100), // Cache up to 100 file diffs
		automationSvc:    automationSvc,
		pendingDecisions: gate.NewPendingDecisionStore(),
		serverCtx:        serverCtx,
		serverCtxCancel:  serverCtxCancel,
		sessionID:        uuid.New().String(),
		sessionStart:     time.Now(),
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

	// Session metrics
	s.mux.HandleFunc("GET /api/session", cors(s.handleGetSessionMetrics))

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
	s.mux.HandleFunc("POST /api/tasks/{id}/skip-block", cors(s.handleSkipBlock))

	// Gate decisions (human approval in headless mode)
	s.mux.HandleFunc("GET /api/decisions", cors(s.handleListDecisions))
	s.mux.HandleFunc("POST /api/decisions/{id}", cors(s.handlePostDecision))

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

	// Workflows
	s.mux.HandleFunc("GET /api/workflows", cors(s.handleListWorkflows))
	s.mux.HandleFunc("POST /api/workflows", cors(s.handleCreateWorkflow))
	s.mux.HandleFunc("GET /api/workflows/{id}", cors(s.handleGetWorkflow))
	s.mux.HandleFunc("PUT /api/workflows/{id}", cors(s.handleUpdateWorkflow))
	s.mux.HandleFunc("DELETE /api/workflows/{id}", cors(s.handleDeleteWorkflow))
	s.mux.HandleFunc("POST /api/workflows/{id}/clone", cors(s.handleCloneWorkflow))
	s.mux.HandleFunc("POST /api/workflows/{id}/phases", cors(s.handleAddWorkflowPhase))
	s.mux.HandleFunc("DELETE /api/workflows/{id}/phases/{phaseId}", cors(s.handleRemoveWorkflowPhase))
	s.mux.HandleFunc("PATCH /api/workflows/{id}/phases/{phaseId}", cors(s.handleUpdateWorkflowPhase))
	s.mux.HandleFunc("POST /api/workflows/{id}/variables", cors(s.handleAddWorkflowVariable))
	s.mux.HandleFunc("DELETE /api/workflows/{id}/variables/{name}", cors(s.handleRemoveWorkflowVariable))

	// Phase Templates
	s.mux.HandleFunc("GET /api/phase-templates", cors(s.handleListPhaseTemplates))
	s.mux.HandleFunc("POST /api/phase-templates", cors(s.handleCreatePhaseTemplate))
	s.mux.HandleFunc("GET /api/phase-templates/{id}", cors(s.handleGetPhaseTemplate))
	s.mux.HandleFunc("PUT /api/phase-templates/{id}", cors(s.handleUpdatePhaseTemplate))
	s.mux.HandleFunc("DELETE /api/phase-templates/{id}", cors(s.handleDeletePhaseTemplate))
	s.mux.HandleFunc("GET /api/phase-templates/{id}/prompt", cors(s.handleGetPhaseTemplatePrompt))

	// Workflow Runs
	s.mux.HandleFunc("GET /api/workflow-runs", cors(s.handleListWorkflowRuns))
	s.mux.HandleFunc("POST /api/workflow-runs", cors(s.handleTriggerWorkflowRun))
	s.mux.HandleFunc("GET /api/workflow-runs/{id}", cors(s.handleGetWorkflowRun))
	s.mux.HandleFunc("POST /api/workflow-runs/{id}/cancel", cors(s.handleCancelWorkflowRun))
	s.mux.HandleFunc("GET /api/workflow-runs/{id}/transcript", cors(s.handleGetWorkflowRunTranscript))

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
	s.mux.HandleFunc("GET /api/tasks/{id}/review/findings", cors(s.handleGetReviewFindings))

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

	// Metrics (JSONL-based analytics)
	s.mux.HandleFunc("GET /api/metrics/summary", cors(s.handleGetMetricsSummary))
	s.mux.HandleFunc("GET /api/metrics/daily", cors(s.handleGetDailyMetrics))
	s.mux.HandleFunc("GET /api/metrics/by-model", cors(s.handleGetMetricsByModel))
	s.mux.HandleFunc("GET /api/tasks/{id}/metrics", cors(s.handleGetTaskMetrics))

	// Todos (progress tracking from JSONL)
	s.mux.HandleFunc("GET /api/tasks/{id}/todos", cors(s.handleGetTaskTodos))
	s.mux.HandleFunc("GET /api/tasks/{id}/todos/history", cors(s.handleGetTaskTodoHistory))

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
	s.mux.HandleFunc("GET /api/agents/stats", cors(s.handleGetAgentStats))
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

	// Constitution (project principles/invariants)
	s.mux.HandleFunc("GET /api/constitution", cors(s.handleGetConstitution))
	s.mux.HandleFunc("PUT /api/constitution", cors(s.handleUpdateConstitution))
	s.mux.HandleFunc("DELETE /api/constitution", cors(s.handleDeleteConstitution))

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
	s.mux.HandleFunc("GET /api/config/stats", cors(s.handleGetConfigStats))

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

	// Stats (activity heatmap, outcomes donut, metrics)
	s.mux.HandleFunc("GET /api/stats/activity", cors(s.handleGetActivityStats))
	s.mux.HandleFunc("GET /api/stats/per-day", cors(s.handleGetPerDayStats))
	s.mux.HandleFunc("GET /api/stats/outcomes", cors(s.handleGetOutcomesStats))
	s.mux.HandleFunc("GET /api/stats/top-initiatives", cors(s.handleGetTopInitiatives))
	s.mux.HandleFunc("GET /api/stats/top-files", cors(s.handleGetTopFiles))
	s.mux.HandleFunc("GET /api/stats/comparison", cors(s.handleGetComparisonStats))

	// Events (timeline queries)
	s.mux.HandleFunc("GET /api/events", cors(s.handleGetEvents))

	// Automation (triggers and automation tasks)
	s.mux.HandleFunc("GET /api/automation/triggers", cors(s.handleListTriggers))
	s.mux.HandleFunc("GET /api/automation/triggers/{id}", cors(s.handleGetTrigger))
	s.mux.HandleFunc("PUT /api/automation/triggers/{id}", cors(s.handleUpdateTrigger))
	s.mux.HandleFunc("POST /api/automation/triggers/{id}/run", cors(s.handleRunTrigger))
	s.mux.HandleFunc("GET /api/automation/triggers/{id}/history", cors(s.handleGetTriggerHistory))
	s.mux.HandleFunc("POST /api/automation/triggers/{id}/reset", cors(s.handleResetTrigger))
	s.mux.HandleFunc("GET /api/automation/tasks", cors(s.handleListAutomationTasks))
	s.mux.HandleFunc("GET /api/automation/stats", cors(s.handleGetAutomationStats))

	// Notifications
	s.mux.HandleFunc("GET /api/notifications", cors(s.handleListNotifications))
	s.mux.HandleFunc("PUT /api/notifications/{id}/dismiss", cors(s.handleDismissNotification))
	s.mux.HandleFunc("PUT /api/notifications/dismiss-all", cors(s.handleDismissAllNotifications))

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

	// Branches (branch registry and lifecycle)
	s.mux.HandleFunc("GET /api/branches", cors(s.handleListBranches))
	s.mux.HandleFunc("GET /api/branches/{name}", cors(s.handleGetBranch))
	s.mux.HandleFunc("PATCH /api/branches/{name}/status", cors(s.handleUpdateBranchStatus))
	s.mux.HandleFunc("DELETE /api/branches/{name}", cors(s.handleDeleteBranch))

	// Static files (embedded frontend) - catch-all for non-API routes
	s.mux.Handle("/", staticHandler())
}

// parseAddr extracts host and port from an address string like ":8080" or "127.0.0.1:8080"
func parseAddr(addr string) (host string, port int, err error) {
	// Handle ":8080" format
	if strings.HasPrefix(addr, ":") {
		port, err = strconv.Atoi(addr[1:])
		return "", port, err
	}

	// Handle "host:port" format
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	port, err = strconv.Atoi(p)
	return h, port, err
}

// findAvailablePort tries to find an available port starting from basePort.
// Returns a listener bound to an available port, or an error if none found.
func findAvailablePort(host string, basePort, maxAttempts int) (net.Listener, int, error) {
	for i := 0; i < maxAttempts; i++ {
		port := basePort + i
		addr := net.JoinHostPort(host, strconv.Itoa(port))
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, port, nil
		}
	}
	return nil, 0, fmt.Errorf("no available port in range %d-%d", basePort, basePort+maxAttempts-1)
}

// Start starts the API server.
func (s *Server) Start() error {
	host, basePort, err := parseAddr(s.addr)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", s.addr, err)
	}

	ln, actualPort, err := findAvailablePort(host, basePort, s.maxPortAttempts)
	if err != nil {
		return err
	}

	if actualPort != basePort {
		s.logger.Info("port in use, using alternative", "requested", basePort, "actual", actualPort)
	}
	s.logger.Info("starting API server", "addr", ln.Addr().String())
	return http.Serve(ln, s.mux)
}

// StartContext starts the API server with context for graceful shutdown.
func (s *Server) StartContext(ctx context.Context) error {
	host, basePort, err := parseAddr(s.addr)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", s.addr, err)
	}

	ln, actualPort, err := findAvailablePort(host, basePort, s.maxPortAttempts)
	if err != nil {
		return err
	}

	if actualPort != basePort {
		s.logger.Info("port in use, using alternative", "requested", basePort, "actual", actualPort)
	}

	server := &http.Server{
		Handler: s.mux,
	}

	// Cancel the default server context and replace with the provided one
	s.serverCtxCancel()
	s.serverCtx, s.serverCtxCancel = context.WithCancel(ctx)

	// Start finalize tracker cleanup (5 min retention, 1 min interval)
	finTracker.startCleanup(s.serverCtx, 1*time.Minute, 5*time.Minute)

	// Prune stale worktree entries on startup
	// This cleans up git's internal worktree tracking for directories that were
	// deleted without proper cleanup (e.g., crashed processes, manual deletion).
	s.pruneStaleWorktrees()

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
	s.prPoller.Start(s.serverCtx)

	go func() {
		<-ctx.Done()
		// Cancel server context (stops finalize goroutines, cleanup goroutine, etc.)
		s.serverCtxCancel()

		// Cancel all running finalize operations
		finTracker.cancelAll()

		// Stop PR poller
		if s.prPoller != nil {
			s.prPoller.Stop()
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			s.logger.Error("server shutdown error", "error", err)
		}
	}()

	s.logger.Info("starting API server", "addr", ln.Addr().String())
	return server.Serve(ln)
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

// CancelAllRunningTasks cancels all running tasks and waits briefly for cleanup.
// Used in tests to ensure background goroutines release file handles before temp
// directory cleanup.
func (s *Server) CancelAllRunningTasks() {
	s.runningTasksMu.Lock()
	for taskID, cancel := range s.runningTasks {
		s.logger.Debug("cancelling running task", "task", taskID)
		cancel()
	}
	s.runningTasksMu.Unlock()

	// Give goroutines time to clean up and close database connections
	time.Sleep(50 * time.Millisecond)
}

// handleHealth returns server health status.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// jsonResponse writes a JSON response.
func (s *Server) jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(data)
}

// jsonError writes a JSON error response.
func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// handleOrcError writes a structured JSON error response for OrcErrors.
func (s *Server) handleOrcError(w http.ResponseWriter, err *orcerrors.OrcError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPStatus())
	_ = json.NewEncoder(w).Encode(err.ToAPIError())
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

// resumeTask resumes a paused, blocked, or failed task (called by WebSocket handler).
func (s *Server) resumeTask(id string) (map[string]any, error) {
	t, err := s.backend.LoadTask(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	// Check if task is resumable
	switch t.Status {
	case task.StatusPaused, task.StatusBlocked, task.StatusFailed:
		// These are resumable
	default:
		return nil, fmt.Errorf("task cannot be resumed (status: %s)", t.Status)
	}

	// Get execution state from task
	exec := &t.Execution

	// Find resume phase with smart retry handling (mirrors CLI logic)
	resumePhase := exec.GetResumePhase()

	// If no interrupted/running phase, check retry context
	if resumePhase == "" {
		if rc := exec.GetRetryContext(); rc != nil && rc.ToPhase != "" {
			resumePhase = rc.ToPhase
			s.logger.Info("resuming from retry target", "task", id, "from", rc.FromPhase, "to", rc.ToPhase)
		}
	}

	// For failed phases (e.g., review), use retry map to go back to earlier phase
	// This prevents the review-resume loop where failed reviews keep restarting from review
	if resumePhase == "" && t.CurrentPhase != "" {
		if ps, ok := exec.Phases[t.CurrentPhase]; ok && ps.Status == task.PhaseStatusFailed {
			if retryFrom := s.orcConfig.ShouldRetryFrom(t.CurrentPhase); retryFrom != "" {
				resumePhase = retryFrom
				s.logger.Info("using retry map for failed phase", "task", id, "from", t.CurrentPhase, "to", retryFrom)
			}
		}
	}

	// Final fallback to current phase
	if resumePhase == "" {
		resumePhase = t.CurrentPhase
	}

	if resumePhase == "" {
		return nil, fmt.Errorf("no resume point found")
	}

	// Update task status
	t.Status = task.StatusRunning
	if err := s.backend.SaveTask(t); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
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

		// Get workflow ID from task - MUST be set
		workflowID := t.WorkflowID
		if workflowID == "" {
			s.logger.Error("task has no workflow_id set", "task", id)
			return
		}

		// Create WorkflowExecutor
		we := executor.NewWorkflowExecutor(
			s.backend,
			s.backend.DB(),
			s.orcConfig,
			s.workDir,
			executor.WithWorkflowPublisher(s.publisher),
			executor.WithWorkflowLogger(s.logger),
			executor.WithWorkflowAutomationService(s.automationSvc),
		)

		opts := executor.WorkflowRunOptions{
			ContextType: executor.ContextTask,
			TaskID:      id,
			Prompt:      t.Description,
			Category:    t.Category,
		}

		// Run workflow (WorkflowExecutor handles resume internally via state)
		_, err := we.Run(ctx, workflowID, opts)
		if err != nil {
			s.logger.Error("task resume failed", "task", id, "error", err)
		}
	}()

	return map[string]any{
		"status":     "resumed",
		"task_id":    id,
		"from_phase": resumePhase,
	}, nil
}

// ensureTaskStatusConsistent verifies task status matches execution outcome.
// This is a safety net to prevent orphaned "running" tasks when the executor
// fails to update task status (e.g., due to panic, unexpected error path).
func (s *Server) ensureTaskStatusConsistent(id string, execErr error) {
	// Reload task to get current values (task now contains execution state)
	t, err := s.backend.LoadTask(id)
	if err != nil {
		s.logger.Warn("failed to reload task for status check", "task", id, "error", err)
		return
	}

	// Get execution state from task
	exec := &t.Execution

	// If task is still "running" but execution finished, fix it
	if t.Status == task.StatusRunning {
		var newStatus task.Status
		var reason string

		if execErr != nil {
			// Execution failed - check if current phase was interrupted
			if ps := exec.Phases[t.CurrentPhase]; ps != nil && ps.Status == task.PhaseStatusInterrupted {
				newStatus = task.StatusPaused
				reason = "interrupted"
			} else {
				newStatus = task.StatusFailed
				reason = "execution error"
			}
		} else {
			// Execution succeeded - this shouldn't happen if executor worked correctly
			// but handle it anyway
			newStatus = task.StatusCompleted
			reason = "execution completed"
		}

		s.logger.Warn("fixing stale task status",
			"task", id,
			"old_status", t.Status,
			"new_status", newStatus,
			"reason", reason,
		)

		t.Status = newStatus
		if err := s.backend.SaveTask(t); err != nil {
			s.logger.Error("failed to fix task status", "task", id, "error", err)
		}
	}

	// Always publish final execution state
	if finalTask, err := s.backend.LoadTask(id); err == nil {
		s.Publish(id, Event{Type: "state", Data: &finalTask.Execution})
	}
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
// Prefers workDir (set at server startup), falls back to FindProjectRoot.
func (s *Server) getProjectRoot() string {
	if s.workDir != "" {
		return s.workDir
	}
	// workDir should always be set at server startup, but handle edge case
	root, err := config.FindProjectRoot()
	if err != nil {
		s.logger.Warn("getProjectRoot: workDir not set and FindProjectRoot failed",
			"error", err)
		// Last resort: use cwd (better than "." which is ambiguous)
		wd, wdErr := os.Getwd()
		if wdErr != nil {
			s.logger.Error("getProjectRoot: cannot determine directory", "error", wdErr)
			return "."
		}
		return wd
	}
	return root
}

// pruneStaleWorktrees removes stale worktree entries from git's tracking.
// Stale entries occur when a worktree directory is deleted without using
// `git worktree remove` (e.g., crashed processes, manual deletion).
// This runs on server startup to ensure git's worktree list stays clean.
func (s *Server) pruneStaleWorktrees() {
	// Initialize git operations
	gitCfg := git.Config{
		BranchPrefix:   s.orcConfig.BranchPrefix,
		CommitPrefix:   s.orcConfig.CommitPrefix,
		WorktreeDir:    s.orcConfig.Worktree.Dir,
		ExecutorPrefix: s.orcConfig.ExecutorPrefix(),
	}
	gitOps, err := git.New(s.workDir, gitCfg)
	if err != nil {
		s.logger.Warn("failed to init git for worktree pruning", "error", err)
		return
	}

	if err := gitOps.PruneWorktrees(); err != nil {
		s.logger.Warn("failed to prune stale worktrees", "error", err)
	} else {
		s.logger.Debug("pruned stale worktree entries")
	}
}
