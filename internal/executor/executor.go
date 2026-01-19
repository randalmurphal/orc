// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/tokenpool"
)

// PhaseState holds state during phase execution.
type PhaseState struct {
	// Task context
	TaskID          string
	TaskTitle       string
	TaskDescription string
	Phase           string
	Weight          string

	// Execution state
	Iteration  int
	Prompt     string   // Rendered prompt sent to Claude
	Response   string   // Claude's response
	Complete   bool     // Phase completion detected
	Blocked    bool     // Phase blocked (needs clarification)
	Error      error    // Any error during execution
	Artifacts  []string // Files created/modified
	CommitSHA  string   // Git commit after phase completion
	TokensUsed int      // Total tokens used in this phase

	// Token tracking
	InputTokens              int
	OutputTokens             int
	CacheCreationInputTokens int
	CacheReadInputTokens     int

	// Prior phase content (for template rendering)
	ResearchContent  string
	SpecContent      string
	DesignContent    string
	ImplementContent string

	// Retry context (populated when retrying from a failed phase)
	RetryContext string

	// Worktree context (for template rendering)
	WorktreePath string
	TaskBranch   string
	TargetBranch string

	// Task category (for template rendering)
	TaskCategory string

	// Initiative context (for template rendering)
	InitiativeContext string

	// UI Testing context (for template rendering)
	RequiresUITesting string
	ScreenshotDir     string
	TestResults       string

	// Testing configuration
	CoverageThreshold int

	// Review context (for review phase)
	ReviewRound    int    // Current review round (1 or 2)
	ReviewFindings string // Previous round's findings (for Round 2)

	// Verification results from implement phase
	VerificationResults string
}

// Config, DefaultConfig, and ConfigFromOrc are defined in config.go

// commonClaudeLocations contains paths where Claude CLI is commonly installed.
// Order matters - check most common locations first.
var commonClaudeLocations = []string{
	// User-local installs (npm global, homebrew user)
	"~/.local/bin/claude",
	"~/.claude/local/claude",
	"~/.npm-global/bin/claude",
	// System installs (homebrew, apt, manual)
	"/usr/local/bin/claude",
	"/opt/homebrew/bin/claude",
	"/usr/bin/claude",
	// macOS-specific paths
	"/opt/local/bin/claude",
	// Linux snap install
	"/snap/bin/claude",
}

// resolveClaudePath resolves a Claude CLI path to an absolute path.
// This is necessary because when cmd.Dir is set (e.g., for worktrees),
// Go's exec.Command won't perform PATH lookup for relative executables.
// By resolving to absolute path upfront, execution works regardless of cmd.Dir.
//
// Resolution order:
//  1. Empty string - returned unchanged
//  2. Already absolute - returned unchanged
//  3. PATH lookup - uses exec.LookPath for relative names like "claude"
//  4. Common locations - checks well-known install paths as fallback
func resolveClaudePath(path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}

	// Resolve relative path to absolute using PATH lookup
	if absPath, err := exec.LookPath(path); err == nil {
		return absPath
	}

	// If the path is "claude" (the default), try common install locations
	if path == "claude" {
		if found := findClaudeInCommonLocations(); found != "" {
			return found
		}
	}

	return path // Fall back to original if all lookups fail
}

// findClaudeInCommonLocations checks common Claude install paths.
// Returns the first valid executable found, or empty string if none.
func findClaudeInCommonLocations() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "" // Will skip ~ paths
	}

	for _, loc := range commonClaudeLocations {
		path := loc
		// Expand ~ to home directory
		if strings.HasPrefix(path, "~/") && homeDir != "" {
			path = filepath.Join(homeDir, path[2:])
		} else if strings.HasPrefix(path, "~/") {
			continue // Skip ~ paths if we couldn't get home dir
		}

		// Check if file exists and is executable
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			// On Unix, check executable bit
			if info.Mode()&0111 != 0 {
				return path
			}
		}
	}
	return ""
}

// Result represents the result of a phase execution.
type Result struct {
	Phase        string
	Status       plan.PhaseStatus
	Iterations   int
	Duration     time.Duration
	Output       string
	Error        error
	Artifacts    []string
	CommitSHA    string
	InputTokens  int
	OutputTokens int
	CostUSD      float64 // Total cost in USD for this phase
	Model        string  // Model used for this phase (e.g., "opus", "sonnet")

	// Cache token tracking (for cost analytics)
	CacheCreationTokens int // Tokens used to create new cache entries
	CacheReadTokens     int // Tokens read from existing cache
}

// Executor runs phases using session-based execution with weight-adaptive strategies.
type Executor struct {
	config          *Config
	orcConfig       *config.Config
	client          claude.Client
	sessionMgr      session.SessionManager
	gateEvaluator   *gate.Evaluator
	gitOps          *git.Git
	checkpointStore checkpoint.Store
	logger          *slog.Logger
	publisher       events.Publisher
	backend         storage.Backend

	// Token pool for automatic account switching (nil if disabled)
	tokenPool *tokenpool.Pool

	// Phase executors by type (created lazily)
	trivialExecutor  *TrivialExecutor
	standardExecutor *StandardExecutor
	fullExecutor     *FullExecutor

	// Runtime state for current task
	worktreePath   string   // Path to worktree if enabled
	worktreeGit    *git.Git // Git operations for worktree
	currentTaskDir string   // Directory for current task's files

	// Use session-based execution (new) vs flowgraph (legacy)
	useSessionExecution bool

	// Resource tracker for process/memory diagnostics
	resourceTracker *ResourceTracker

	// Automation service for trigger-based automation
	automationSvc *automation.Service

	// Haiku client for validation calls (separate from main client)
	haikuClient claude.Client

	// Global database for cross-project cost tracking
	globalDB *db.GlobalDB
}

// New creates a new executor with the given configuration.
func New(cfg *Config) *Executor {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// Load orc config for gate resolution
	orcCfg, err := config.Load()
	if err != nil {
		orcCfg = config.Default()
	}

	// Create Claude CLI client with llmkit
	clientOpts := []claude.ClaudeOption{
		claude.WithModel(cfg.Model),
		claude.WithWorkdir(cfg.WorkDir),
		claude.WithTimeout(cfg.Timeout),
	}

	// Resolve Claude path to absolute to ensure it works with worktrees
	claudePath := resolveClaudePath(cfg.ClaudePath)
	if claudePath != "" {
		clientOpts = append(clientOpts, claude.WithClaudePath(claudePath))
	}

	if cfg.DangerouslySkipPermissions {
		clientOpts = append(clientOpts, claude.WithDangerouslySkipPermissions())
	}

	// Apply tool permissions
	if len(cfg.AllowedTools) > 0 {
		clientOpts = append(clientOpts, claude.WithAllowedTools(cfg.AllowedTools))
	}
	if len(cfg.DisallowedTools) > 0 {
		clientOpts = append(clientOpts, claude.WithDisallowedTools(cfg.DisallowedTools))
	}

	// Token pool injection happens after pool is loaded (see below)
	// We'll rebuild the client with the token if pool is enabled

	client := claude.NewClaudeCLI(clientOpts...)

	// Create session manager for session-based execution
	// Sessions will use the same model and workdir settings
	// Include "user" setting source to load agents from ~/.claude/agents/
	sessionMgr := session.NewManager(
		session.WithDefaultSessionOptions(
			session.WithModel(cfg.Model),
			session.WithWorkdir(cfg.WorkDir),
			session.WithClaudePath(claudePath),
			session.WithPermissions(cfg.DangerouslySkipPermissions),
			session.WithSettingSources([]string{"project", "local", "user"}),
		),
	)

	// Create checkpoint store if enabled
	var cpStore checkpoint.Store
	if cfg.EnableCheckpoints {
		cpStore = checkpoint.NewMemoryStore()
	}

	// Create git operations with orc-specific config
	gitCfg := git.Config{
		BranchPrefix:   cfg.BranchPrefix,
		CommitPrefix:   cfg.CommitPrefix,
		WorktreeDir:    orcCfg.Worktree.Dir,
		ExecutorPrefix: orcCfg.ExecutorPrefix(),
	}
	gitOps, err := git.New(cfg.WorkDir, gitCfg)
	if err != nil {
		// Log warning but don't fail - git might not be initialized
		slog.Warn("failed to initialize git operations", "error", err)
	}

	// Load token pool if enabled
	var pool *tokenpool.Pool
	if orcCfg.Pool.Enabled {
		pool, err = tokenpool.New(orcCfg.Pool.ConfigPath, tokenpool.WithLogger(slog.Default()))
		if err != nil {
			slog.Warn("failed to load token pool", "error", err)
		} else {
			slog.Info("token pool enabled", "accounts", len(pool.Accounts()))
			// Rebuild client with token from pool
			if token := pool.Token(); token != "" {
				clientOpts = append(clientOpts, claude.WithEnvVar("CLAUDE_CODE_OAUTH_TOKEN", token))
				client = claude.NewClaudeCLI(clientOpts...)
				slog.Info("using token from pool", "account", pool.Current().ID)
			}
		}
	}

	// Create resource tracker with config from orcCfg.Diagnostics
	rtConfig := ResourceTrackerConfig{
		Enabled:               orcCfg.Diagnostics.ResourceTracking.Enabled,
		MemoryThresholdMB:     orcCfg.Diagnostics.ResourceTracking.MemoryThresholdMB,
		LogOrphanedMCPOnly:    orcCfg.Diagnostics.ResourceTracking.LogOrphanedMCPOnly,
		FilterSystemProcesses: orcCfg.Diagnostics.ResourceTracking.FilterSystemProcesses,
	}
	resourceTracker := NewResourceTracker(rtConfig, slog.Default())

	// Create Haiku client for validation if enabled
	var haikuClient claude.Client
	if orcCfg.Validation.Enabled {
		haikuOpts := []claude.ClaudeOption{
			claude.WithModel(orcCfg.Validation.Model),
			claude.WithWorkdir(cfg.WorkDir),
		}
		if claudePath != "" {
			haikuOpts = append(haikuOpts, claude.WithClaudePath(claudePath))
		}
		if cfg.DangerouslySkipPermissions {
			haikuOpts = append(haikuOpts, claude.WithDangerouslySkipPermissions())
		}
		haikuClient = claude.NewClaudeCLI(haikuOpts...)
	}

	// Open global database for cross-project cost tracking
	// Cost tracking is optional - failures are logged but don't block execution
	globalDB, err := db.OpenGlobal()
	if err != nil {
		slog.Warn("failed to open global database for cost tracking", "error", err)
	}

	return &Executor{
		config:              cfg,
		orcConfig:           orcCfg,
		client:              client,
		sessionMgr:          sessionMgr,
		gateEvaluator:       gate.New(client),
		gitOps:              gitOps,
		checkpointStore:     cpStore,
		logger:              slog.Default(),
		tokenPool:           pool,
		backend:             cfg.Backend,
		useSessionExecution: orcCfg.Execution.UseSessionExecution,
		resourceTracker:     resourceTracker,
		haikuClient:         haikuClient,
		globalDB:            globalDB,
	}
}

// NewWithConfig creates an executor with explicit orc config.
func NewWithConfig(cfg *Config, orcCfg *config.Config) *Executor {
	e := New(cfg)
	if orcCfg != nil {
		e.orcConfig = orcCfg
	}
	return e
}

// SetPublisher sets the event publisher for real-time updates.
func (e *Executor) SetPublisher(p events.Publisher) {
	e.publisher = p
}

// SetBackend sets the storage backend for task/state persistence.
func (e *Executor) SetBackend(b storage.Backend) {
	e.backend = b
}

// SetAutomationService sets the automation service for trigger-based automation.
func (e *Executor) SetAutomationService(svc *automation.Service) {
	e.automationSvc = svc
}

// taskDir returns the directory for a task's files.
func (e *Executor) taskDir(taskID string) string {
	return filepath.Join(e.config.WorkDir, ".orc", "tasks", taskID)
}

// SetClient sets the Claude client (for testing).
func (e *Executor) SetClient(c claude.Client) {
	e.client = c
}

// SetHaikuClient sets the Haiku client for validation (for testing).
func (e *Executor) SetHaikuClient(c claude.Client) {
	e.haikuClient = c
}

// HaikuClient returns the Haiku client for validation calls.
func (e *Executor) HaikuClient() claude.Client {
	return e.haikuClient
}

// SetUseSessionExecution enables or disables session-based execution.
// When disabled, falls back to the legacy flowgraph-based execution.
func (e *Executor) SetUseSessionExecution(use bool) {
	e.useSessionExecution = use
}

// getPhaseExecutor returns the appropriate phase executor for the given weight.
// Executors are created lazily and cached for reuse.
func (e *Executor) getPhaseExecutor(weight task.Weight) PhaseExecutor {
	execType := ExecutorTypeForWeight(weight)
	workingDir := e.config.WorkDir
	if e.worktreePath != "" {
		workingDir = e.worktreePath
	}

	// Use worktree git if available, otherwise fall back to main repo git
	gitSvc := e.gitOps
	if e.worktreeGit != nil {
		gitSvc = e.worktreeGit
	}

	// Create executor config with OrcConfig for model resolution
	execCfg := DefaultConfigForWeight(weight)
	execCfg.OrcConfig = e.orcConfig

	switch execType {
	case ExecutorTypeTrivial:
		if e.trivialExecutor == nil {
			e.trivialExecutor = NewTrivialExecutor(
				WithTrivialClient(e.client),
				WithTrivialPublisher(e.publisher),
				WithTrivialLogger(e.logger),
				WithTrivialConfig(execCfg),
				WithTrivialBackend(e.backend),
			)
		}
		return e.trivialExecutor

	case ExecutorTypeFull:
		if e.fullExecutor == nil {
			opts := []FullExecutorOption{
				WithFullGitSvc(gitSvc),
				WithFullPublisher(e.publisher),
				WithFullLogger(e.logger),
				WithFullConfig(execCfg),
				WithFullWorkingDir(workingDir),
				WithTaskDir(e.currentTaskDir),
				WithFullBackend(e.backend),
			}

			// Create backpressure runner with the correct working directory
			if e.orcConfig.Validation.Enabled && e.orcConfig.ShouldRunBackpressure(string(weight)) {
				bp := NewBackpressureRunner(
					workingDir,
					&e.orcConfig.Validation,
					&e.orcConfig.Testing,
					e.logger,
				)
				opts = append(opts, WithFullBackpressure(bp))
			}

			// Pass haiku client and config for progress validation
			if e.haikuClient != nil {
				opts = append(opts, WithFullHaikuClient(e.haikuClient))
			}
			if e.orcConfig != nil {
				opts = append(opts, WithFullOrcConfig(e.orcConfig))
			}

			e.fullExecutor = NewFullExecutor(e.sessionMgr, opts...)
		}
		return e.fullExecutor

	default: // ExecutorTypeStandard
		if e.standardExecutor == nil {
			opts := []StandardExecutorOption{
				WithGitSvc(gitSvc),
				WithPublisher(e.publisher),
				WithExecutorLogger(e.logger),
				WithExecutorConfig(execCfg),
				WithWorkingDir(workingDir),
				WithStandardBackend(e.backend),
			}

			// Create backpressure runner with the correct working directory
			if e.orcConfig.Validation.Enabled && e.orcConfig.ShouldRunBackpressure(string(weight)) {
				bp := NewBackpressureRunner(
					workingDir,
					&e.orcConfig.Validation,
					&e.orcConfig.Testing,
					e.logger,
				)
				opts = append(opts, WithStandardBackpressure(bp))
			}

			// Pass haiku client and config for progress validation
			if e.haikuClient != nil {
				opts = append(opts, WithStandardHaikuClient(e.haikuClient))
			}
			if e.orcConfig != nil {
				opts = append(opts, WithStandardOrcConfig(e.orcConfig))
			}

			e.standardExecutor = NewStandardExecutor(e.sessionMgr, opts...)
		}
		return e.standardExecutor
	}
}

// resetPhaseExecutors clears cached executors (called when context changes, e.g., worktree).
func (e *Executor) resetPhaseExecutors() {
	e.trivialExecutor = nil
	e.standardExecutor = nil
	e.fullExecutor = nil
}

// LoadProjectToolPermissions and rebuildClient are defined in permissions.go

// Event publishing convenience methods - thin wrappers around EventPublisher.
// These provide backwards-compatible method signatures on Executor.

func (e *Executor) eventPublisher() *EventPublisher {
	return NewEventPublisher(e.publisher)
}

func (e *Executor) publish(ev events.Event) {
	e.eventPublisher().Publish(ev)
}

func (e *Executor) publishPhaseStart(taskID, phase string) {
	e.eventPublisher().PhaseStart(taskID, phase)
}

func (e *Executor) publishPhaseComplete(taskID, phase, commitSHA string) {
	e.eventPublisher().PhaseComplete(taskID, phase, commitSHA)
}

func (e *Executor) publishPhaseFailed(taskID, phase string, err error) {
	e.eventPublisher().PhaseFailed(taskID, phase, err)
}

func (e *Executor) publishTranscript(taskID, phase string, iteration int, msgType, content string) {
	e.eventPublisher().Transcript(taskID, phase, iteration, msgType, content)
}

func (e *Executor) publishTokens(taskID, phase string, input, output, cacheCreation, cacheRead, total int) {
	e.eventPublisher().Tokens(taskID, phase, input, output, cacheCreation, cacheRead, total)
}

func (e *Executor) publishError(taskID, phase, message string, fatal bool) {
	e.eventPublisher().Error(taskID, phase, message, fatal)
}

func (e *Executor) publishState(taskID string, s *state.State) {
	e.eventPublisher().State(taskID, s)
}

// Phase execution methods (ExecutePhase, executePhaseWithSession, executePhaseWithFlowgraph)
// are defined in phase.go

// Task execution methods (ExecuteTask, ResumeFromPhase, evaluateGate, etc.)
// are defined in task_execution.go

// PR and completion methods (runCompletion, syncWithTarget, directMerge, createPR, buildPRBody, runGH)
// are defined in pr.go

// TokenPool returns the token pool if configured.
func (e *Executor) TokenPool() *tokenpool.Pool {
	return e.tokenPool
}

// SetTokenPool sets the token pool (for testing).
func (e *Executor) SetTokenPool(pool *tokenpool.Pool) {
	e.tokenPool = pool
}

// SwitchToNextAccount switches to the next available account in the pool.
// Returns an error if all accounts are exhausted.
func (e *Executor) SwitchToNextAccount() error {
	if e.tokenPool == nil {
		return tokenpool.ErrPoolDisabled
	}

	next, err := e.tokenPool.Next()
	if err != nil {
		return err
	}

	// Rebuild client with new token
	e.rebuildClientWithToken(next.Token())
	e.logger.Info("switched to next account",
		"account_id", next.ID,
		"account_name", next.Name)

	return nil
}

// rebuildClientWithToken rebuilds the Claude client with a new OAuth token.
func (e *Executor) rebuildClientWithToken(token string) {
	workdir := e.config.WorkDir
	// Use worktree path if we're in a worktree context
	if e.worktreePath != "" {
		workdir = e.worktreePath
	}

	clientOpts := []claude.ClaudeOption{
		claude.WithModel(e.config.Model),
		claude.WithWorkdir(workdir),
		claude.WithTimeout(e.config.Timeout),
	}

	// Disable go.work in worktree context to avoid path resolution issues
	if e.worktreePath != "" {
		clientOpts = append(clientOpts, claude.WithEnvVar("GOWORK", "off"))
	}

	// Resolve Claude path to absolute to ensure it works with worktrees
	claudePath := resolveClaudePath(e.config.ClaudePath)
	if claudePath != "" {
		clientOpts = append(clientOpts, claude.WithClaudePath(claudePath))
	}

	if e.config.DangerouslySkipPermissions {
		clientOpts = append(clientOpts, claude.WithDangerouslySkipPermissions())
	}

	if len(e.config.AllowedTools) > 0 {
		clientOpts = append(clientOpts, claude.WithAllowedTools(e.config.AllowedTools))
	}
	if len(e.config.DisallowedTools) > 0 {
		clientOpts = append(clientOpts, claude.WithDisallowedTools(e.config.DisallowedTools))
	}

	// Inject the new token
	if token != "" {
		clientOpts = append(clientOpts, claude.WithEnvVar("CLAUDE_CODE_OAUTH_TOKEN", token))
	}

	e.client = claude.NewClaudeCLI(clientOpts...)

	// Reset phase executors so they pick up the new client
	e.resetPhaseExecutors()
}

// MarkCurrentAccountExhausted marks the current account as exhausted due to rate limiting.
func (e *Executor) MarkCurrentAccountExhausted(reason string) {
	if e.tokenPool != nil {
		e.tokenPool.MarkExhausted(reason)
	}
}

// runResourceAnalysis takes the after-snapshot and analyzes resource usage.
// Called via defer in ExecuteTask to run regardless of success or failure.
func (e *Executor) runResourceAnalysis() {
	if e.resourceTracker == nil {
		return
	}

	// Take after snapshot
	if err := e.resourceTracker.SnapshotAfter(); err != nil {
		e.logger.Warn("failed to take resource snapshot after task", "error", err)
		return
	}

	// Detect orphaned processes (logs warnings for any found)
	e.resourceTracker.DetectOrphans()

	// Check memory growth against threshold (logs warning if exceeded)
	e.resourceTracker.CheckMemoryGrowth()

	// Reset tracker for next task
	e.resourceTracker.Reset()
}
