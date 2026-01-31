// Package api provides the REST API and WebSocket server for orc.
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
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/diff"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Server is the orc API server.
type Server struct {
	addr            string
	workDir         string // Project directory (legacy - used for backwards compat)
	maxPortAttempts int    // Number of ports to try
	mux             *http.ServeMux
	logger          *slog.Logger

	// Orc configuration
	orcConfig *config.Config

	// Event publisher for real-time updates
	publisher events.Publisher
	wsHandler *WSHandler

	// Storage backend (legacy - used for backwards compat)
	backend storage.Backend

	// Project database for workflow execution (legacy - used for backwards compat)
	projectDB *db.ProjectDB

	// Global database for cross-project resources
	globalDB *db.GlobalDB

	// Project database cache for multi-tenant access
	projectCache *ProjectCache

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

	// Session broadcaster for real-time metrics
	sessionBroadcaster *executor.SessionBroadcaster
}

// Event represents a WebSocket event.
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

	// Open global DB for cross-project resources and cost tracking
	globalDB, err := db.OpenGlobal()
	if err != nil {
		logger.Warn("failed to open global database", "error", err)
	}
	s.globalDB = globalDB

	// Seed built-in workflows and phase templates (into global DB)
	if globalDB != nil {
		if seeded, err := workflow.SeedBuiltins(globalDB); err != nil {
			logger.Error("failed to seed built-in workflows", "error", err)
		} else if seeded > 0 {
			logger.Info("seeded built-in workflows", "count", seeded)
		}

		// Seed built-in agents and phase-agent associations
		if seeded, err := workflow.SeedAgents(globalDB); err != nil {
			logger.Error("failed to seed built-in agents", "error", err)
		} else if seeded > 0 {
			logger.Info("seeded built-in agents", "count", seeded)
		}
	}

	// Create project cache for multi-tenant database access
	s.projectCache = NewProjectCache(10) // Max 10 projects open simultaneously

	// Create session broadcaster for real-time metrics
	s.sessionBroadcaster = executor.NewSessionBroadcaster(
		events.NewPublishHelper(pub),
		backend,
		globalDB,
		workDir,
		logger,
	)

	s.registerFileRoutes()
	s.registerConnectHandlers()
	return s
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
		OnStatusChange: func(taskID string, pr *orcv1.PRInfo) {
			// Publish task update event when PR status changes
			s.logger.Info("PR status changed", "task", taskID, "status", pr.Status)
			s.publisher.Publish(events.Event{
				Type:   events.EventTaskUpdated,
				TaskID: taskID,
				Data:   map[string]any{"pr": pr},
			})

			// Auto-trigger finalize when PR is approved (if enabled in config)
			if pr.Status == orcv1.PRStatus_PR_STATUS_APPROVED {
				triggered, err := s.TriggerFinalizeOnApproval(taskID, "")
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

		// Stop session broadcaster
		if s.sessionBroadcaster != nil {
			s.sessionBroadcaster.Stop()
		}

		// Close project cache (closes all cached databases)
		if s.projectCache != nil {
			if err := s.projectCache.Close(); err != nil {
				s.logger.Error("project cache close error", "error", err)
			}
		}

		// Close global database
		if s.globalDB != nil {
			if err := s.globalDB.Close(); err != nil {
				s.logger.Error("global database close error", "error", err)
			}
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

// Publish sends an event to the event publisher for WebSocket broadcast.
// This converts legacy Event types to the events.Event format.
func (s *Server) Publish(taskID string, event Event) {
	var eventType events.EventType
	switch event.Type {
	case "error":
		eventType = events.EventError
	case "complete":
		eventType = events.EventComplete
	case "state":
		eventType = events.EventState
	default:
		eventType = events.EventType(event.Type)
	}
	s.publisher.Publish(events.NewEvent(eventType, taskID, event.Data))
}

// Backend returns the storage backend (for testing).
func (s *Server) Backend() storage.Backend {
	return s.backend
}

// ProjectCache returns the project database cache for multi-tenant access.
func (s *Server) ProjectCache() *ProjectCache {
	return s.projectCache
}

// GlobalDB returns the global database for cross-project resources.
func (s *Server) GlobalDB() *db.GlobalDB {
	return s.globalDB
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

// protoJSONMarshaler is configured to produce frontend-compatible JSON.
var protoJSONMarshaler = protojson.MarshalOptions{
	UseEnumNumbers:  false, // Use string enum names for backward compatibility
	EmitUnpopulated: false, // Don't emit zero values
}

// jsonResponse writes a JSON response.
// For proto messages and slices of proto messages, uses protojson for proper serialization.
// For maps containing proto messages, converts proto fields using protojson first.
func (s *Server) jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")

	// Check if data is a proto message
	if msg, ok := data.(proto.Message); ok {
		bytes, err := protoJSONMarshaler.Marshal(msg)
		if err != nil {
			s.logger.Error("failed to marshal proto message", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":"internal server error"}`))
			return
		}
		_, _ = w.Write(bytes)
		return
	}

	// Check if data is a slice of proto tasks (common case)
	if tasks, ok := data.([]*orcv1.Task); ok {
		s.writeProtoSlice(w, tasks)
		return
	}

	// Check if data is a map that might contain proto messages
	if m, ok := data.(map[string]any); ok {
		data = s.convertMapProtos(m)
	}

	_ = json.NewEncoder(w).Encode(data)
}

// convertMapProtos recursively converts proto messages in a map to JSON-compatible maps.
func (s *Server) convertMapProtos(m map[string]any) map[string]any {
	result := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case proto.Message:
			// Convert proto to map via protojson
			bytes, err := protoJSONMarshaler.Marshal(val)
			if err != nil {
				s.logger.Error("failed to marshal proto in map", "key", k, "error", err)
				result[k] = nil
				continue
			}
			var converted map[string]any
			if err := json.Unmarshal(bytes, &converted); err != nil {
				s.logger.Error("failed to unmarshal proto json", "key", k, "error", err)
				result[k] = nil
				continue
			}
			result[k] = converted
		case []*orcv1.Task:
			// Handle slice of proto tasks
			result[k] = s.convertProtoTaskSlice(val)
		case map[string]any:
			result[k] = s.convertMapProtos(val)
		case []any:
			result[k] = s.convertSliceProtos(val)
		default:
			result[k] = v
		}
	}
	return result
}

// convertSliceProtos recursively converts proto messages in a slice to JSON-compatible values.
func (s *Server) convertSliceProtos(slice []any) []any {
	result := make([]any, len(slice))
	for i, v := range slice {
		switch val := v.(type) {
		case proto.Message:
			bytes, err := protoJSONMarshaler.Marshal(val)
			if err != nil {
				s.logger.Error("failed to marshal proto in slice", "index", i, "error", err)
				result[i] = nil
				continue
			}
			var converted map[string]any
			if err := json.Unmarshal(bytes, &converted); err != nil {
				s.logger.Error("failed to unmarshal proto json in slice", "index", i, "error", err)
				result[i] = nil
				continue
			}
			result[i] = converted
		case map[string]any:
			result[i] = s.convertMapProtos(val)
		case []any:
			result[i] = s.convertSliceProtos(val)
		default:
			result[i] = v
		}
	}
	return result
}

// convertProtoTaskSlice converts a slice of proto tasks to JSON-compatible maps.
func (s *Server) convertProtoTaskSlice(tasks []*orcv1.Task) []any {
	result := make([]any, len(tasks))
	for i, t := range tasks {
		bytes, err := protoJSONMarshaler.Marshal(t)
		if err != nil {
			s.logger.Error("failed to marshal proto task in slice", "index", i, "error", err)
			result[i] = nil
			continue
		}
		var converted map[string]any
		if err := json.Unmarshal(bytes, &converted); err != nil {
			s.logger.Error("failed to unmarshal proto task json", "index", i, "error", err)
			result[i] = nil
			continue
		}
		result[i] = converted
	}
	return result
}

// writeProtoSlice marshals a slice of proto messages to JSON array.
func (s *Server) writeProtoSlice(w http.ResponseWriter, tasks []*orcv1.Task) {
	if len(tasks) == 0 {
		_, _ = w.Write([]byte("[]"))
		return
	}

	_, _ = w.Write([]byte("["))
	for i, msg := range tasks {
		if i > 0 {
			_, _ = w.Write([]byte(","))
		}
		bytes, err := protoJSONMarshaler.Marshal(msg)
		if err != nil {
			s.logger.Error("failed to marshal proto message in slice", "error", err, "index", i)
			continue
		}
		_, _ = w.Write(bytes)
	}
	_, _ = w.Write([]byte("]"))
}

// jsonError writes a JSON error response.
func (s *Server) jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// pauseTask pauses a running task (called by WebSocket handler).
func (s *Server) pauseTask(id string, projectID string) (map[string]any, error) {
	backend := s.backend
	if projectID != "" && s.projectCache != nil {
		var err error
		backend, err = s.projectCache.GetBackend(projectID)
		if err != nil {
			return nil, fmt.Errorf("resolve project backend: %w", err)
		}
	}

	t, err := backend.LoadTask(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	t.Status = orcv1.TaskStatus_TASK_STATUS_PAUSED
	if err := backend.SaveTask(t); err != nil {
		return nil, fmt.Errorf("failed to update task: %w", err)
	}

	return map[string]any{
		"status":  "paused",
		"task_id": id,
	}, nil
}

// resumeTask resumes a paused, blocked, or failed task (called by WebSocket handler).
func (s *Server) resumeTask(id string, projectID string) (map[string]any, error) {
	backend := s.backend
	workDir := s.workDir
	if projectID != "" && s.projectCache != nil {
		var err error
		backend, err = s.projectCache.GetBackend(projectID)
		if err != nil {
			return nil, fmt.Errorf("resolve project backend: %w", err)
		}
		workDir, err = s.projectCache.GetProjectPath(projectID)
		if err != nil {
			return nil, fmt.Errorf("resolve project path: %w", err)
		}
	}

	t, err := backend.LoadTask(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	// Check if task is resumable
	switch t.Status {
	case orcv1.TaskStatus_TASK_STATUS_PAUSED, orcv1.TaskStatus_TASK_STATUS_BLOCKED, orcv1.TaskStatus_TASK_STATUS_FAILED:
		// These are resumable
	default:
		return nil, fmt.Errorf("task cannot be resumed (status: %s)", t.Status)
	}

	// Get execution state from task
	exec := t.GetExecution()

	// Find resume phase with smart retry handling (mirrors CLI logic)
	resumePhase := task.GetResumePhaseProto(exec)

	// If no interrupted/running phase, check retry context
	if resumePhase == "" {
		if rc := exec.GetRetryContext(); rc != nil && rc.ToPhase != "" {
			resumePhase = rc.ToPhase
			s.logger.Info("resuming from retry target", "task", id, "from", rc.FromPhase, "to", rc.ToPhase)
		}
	}

	// For failed phases (e.g., review), use retry map to go back to earlier phase
	// This prevents the review-resume loop where failed reviews keep restarting from review
	// Check task status and phase error since phases no longer track FAILED status
	currentPhase := task.GetCurrentPhaseProto(t)
	if resumePhase == "" && currentPhase != "" {
		// If task failed and current phase has an error, use retry map
		taskFailed := t.Status == orcv1.TaskStatus_TASK_STATUS_FAILED
		ps := exec.GetPhases()[currentPhase]
		phaseHasError := ps != nil && ps.Error != nil && *ps.Error != ""
		if taskFailed || phaseHasError {
			if retryFrom := s.orcConfig.ShouldRetryFrom(currentPhase); retryFrom != "" {
				resumePhase = retryFrom
				s.logger.Info("using retry map for failed phase", "task", id, "from", currentPhase, "to", retryFrom)
			}
		}
	}

	// Final fallback to current phase
	if resumePhase == "" {
		resumePhase = currentPhase
	}

	if resumePhase == "" {
		return nil, fmt.Errorf("no resume point found")
	}

	// Update task status
	t.Status = orcv1.TaskStatus_TASK_STATUS_RUNNING
	if err := backend.SaveTask(t); err != nil {
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
		workflowID := t.GetWorkflowId()
		if workflowID == "" {
			s.logger.Error("task has no workflow_id set", "task", id)
			return
		}

		// Create WorkflowExecutor
		we := executor.NewWorkflowExecutor(
			backend,
			backend.DB(),
			s.orcConfig,
			workDir,
			executor.WithWorkflowPublisher(s.publisher),
			executor.WithWorkflowLogger(s.logger),
			executor.WithWorkflowAutomationService(s.automationSvc),
			executor.WithWorkflowSessionBroadcaster(s.sessionBroadcaster),
		)

		opts := executor.WorkflowRunOptions{
			ContextType: executor.ContextTask,
			TaskID:      id,
			Prompt:      task.GetDescriptionProto(t),
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

// startTask starts a task execution (called by taskServer.RunTask).
// This spawns a WorkflowExecutor goroutine similar to resumeTask.
func (s *Server) startTask(id string, projectID string) error {
	backend := s.backend
	workDir := s.workDir
	if projectID != "" && s.projectCache != nil {
		var err error
		backend, err = s.projectCache.GetBackend(projectID)
		if err != nil {
			return fmt.Errorf("resolve project backend: %w", err)
		}
		workDir, err = s.projectCache.GetProjectPath(projectID)
		if err != nil {
			return fmt.Errorf("resolve project path: %w", err)
		}
	}

	t, err := backend.LoadTask(id)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Get workflow ID from task - should already be validated by RunTask
	workflowID := t.GetWorkflowId()
	if workflowID == "" {
		return fmt.Errorf("task has no workflow_id set")
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

		// Create WorkflowExecutor
		we := executor.NewWorkflowExecutor(
			backend,
			backend.DB(),
			s.orcConfig,
			workDir,
			executor.WithWorkflowPublisher(s.publisher),
			executor.WithWorkflowLogger(s.logger),
			executor.WithWorkflowAutomationService(s.automationSvc),
			executor.WithWorkflowSessionBroadcaster(s.sessionBroadcaster),
		)

		opts := executor.WorkflowRunOptions{
			ContextType: executor.ContextTask,
			TaskID:      id,
			Prompt:      task.GetDescriptionProto(t),
			Category:    t.Category,
		}

		// Run workflow
		_, err := we.Run(ctx, workflowID, opts)
		if err != nil {
			s.logger.Error("task execution failed", "task", id, "error", err)
		}
	}()

	return nil
}

// cancelTask cancels a running task (called by WebSocket handler).
func (s *Server) cancelTask(id string, projectID string) (map[string]any, error) {
	s.runningTasksMu.RLock()
	cancel, exists := s.runningTasks[id]
	s.runningTasksMu.RUnlock()

	if exists {
		cancel()
	}

	backend := s.backend
	if projectID != "" && s.projectCache != nil {
		var err error
		backend, err = s.projectCache.GetBackend(projectID)
		if err != nil {
			return nil, fmt.Errorf("resolve project backend: %w", err)
		}
	}

	t, err := backend.LoadTask(id)
	if err != nil {
		return nil, fmt.Errorf("task not found")
	}

	t.Status = orcv1.TaskStatus_TASK_STATUS_FAILED
	if err := backend.SaveTask(t); err != nil {
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

// SessionMetricsResponse represents session metrics for the TopBar.
type SessionMetricsResponse struct {
	SessionID        string    `json:"session_id"`
	StartedAt        time.Time `json:"started_at"`
	DurationSeconds  int64     `json:"duration_seconds"`
	TotalTokens      int       `json:"total_tokens"`
	InputTokens      int       `json:"input_tokens"`
	OutputTokens     int       `json:"output_tokens"`
	EstimatedCostUSD float64   `json:"estimated_cost_usd"`
	TasksCompleted   int       `json:"tasks_completed"`
	TasksRunning     int       `json:"tasks_running"`
	IsPaused         bool      `json:"is_paused"`
}

// GetSessionMetrics returns current session metrics (used by WebSocket handler).
func (s *Server) GetSessionMetrics(projectID string) SessionMetricsResponse {
	duration := int64(time.Since(s.sessionStart).Seconds())

	backend := s.backend
	if projectID != "" && s.projectCache != nil {
		var err error
		backend, err = s.projectCache.GetBackend(projectID)
		if err != nil {
			s.logger.Error("failed to resolve project backend for metrics", "error", err)
			return SessionMetricsResponse{
				SessionID:       s.sessionID,
				StartedAt:       s.sessionStart,
				DurationSeconds: duration,
			}
		}
	}

	// Handle nil backend (can happen in tests)
	if backend == nil {
		return SessionMetricsResponse{
			SessionID:       s.sessionID,
			StartedAt:       s.sessionStart,
			DurationSeconds: duration,
		}
	}

	tasks, err := backend.LoadAllTasks()
	if err != nil {
		s.logger.Error("failed to load tasks for session metrics", "error", err)
		return SessionMetricsResponse{
			SessionID:       s.sessionID,
			StartedAt:       s.sessionStart,
			DurationSeconds: duration,
		}
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	var running, completed int
	var totalInput, totalOutput int
	var totalCost float64

	for _, t := range tasks {
		switch t.Status {
		case orcv1.TaskStatus_TASK_STATUS_RUNNING:
			running++
		case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
			completed++
		}

		if exec := t.GetExecution(); exec != nil {
			for _, ps := range exec.GetPhases() {
				if ps != nil && ps.GetStartedAt() != nil {
					startedAt := ps.GetStartedAt().AsTime()
					if startedAt.After(today) || startedAt.Equal(today) {
						if tokens := ps.GetTokens(); tokens != nil {
							totalInput += int(tokens.InputTokens)
							totalOutput += int(tokens.OutputTokens)
						}
					}
				}
			}
			if t.GetStartedAt() != nil {
				startedAt := t.GetStartedAt().AsTime()
				if startedAt.After(today) || startedAt.Equal(today) {
					if cost := exec.GetCost(); cost != nil {
						totalCost += cost.TotalCostUsd
					}
				}
			}
		}
	}

	return SessionMetricsResponse{
		SessionID:        s.sessionID,
		StartedAt:        s.sessionStart,
		DurationSeconds:  duration,
		TotalTokens:      totalInput + totalOutput,
		InputTokens:      totalInput,
		OutputTokens:     totalOutput,
		EstimatedCostUSD: totalCost,
		TasksCompleted:   completed,
		TasksRunning:     running,
		IsPaused:         false,
	}
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
		WorktreeDir:    config.ResolveWorktreeDir(s.orcConfig.Worktree.Dir, s.workDir),
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
