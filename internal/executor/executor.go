// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
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

// ResolveClaudePath resolves a Claude CLI path to an absolute path.
// This is necessary because when cmd.Dir is set (e.g., for worktrees),
// Go's exec.Command won't perform PATH lookup for relative executables.
// By resolving to absolute path upfront, execution works regardless of cmd.Dir.
//
// Resolution order:
//  1. Empty string - returned unchanged
//  2. Already absolute - returned unchanged
//  3. PATH lookup - uses exec.LookPath for relative names like "claude"
//  4. Common locations - checks well-known install paths as fallback
func ResolveClaudePath(path string) string {
	if path == "" {
		return path
	}

	// Expand tilde to home directory first
	if strings.HasPrefix(path, "~/") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(homeDir, path[2:])
		}
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
	Status       orcv1.PhaseStatus
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
	config        *Config
	orcConfig     *config.Config
	client        claude.Client
	gateEvaluator *gate.Evaluator
	gitOps        *git.Git
	logger        *slog.Logger
	publisher     events.Publisher
	backend       storage.Backend

	// Pending gate decisions (for headless mode)
	pendingDecisions *gate.PendingDecisionStore
	headless         bool // True if running in API/headless mode

	// Token pool for automatic account switching (nil if disabled)
	tokenPool *tokenpool.Pool

	// Runtime state for current task
	worktreePath  string // Path to worktree if enabled
	currentTaskID string // Task ID for hooks (e.g., TDD enforcement)

	// Resource tracker for process/memory diagnostics
	resourceTracker *ResourceTracker

	// Resume session ID for continuing paused tasks with Claude's --resume flag
	resumeSessionID string

	// Automation service for trigger-based automation
	automationSvc *automation.Service

	// Global database for cross-project cost tracking
	globalDB *db.GlobalDB

	// Session broadcaster for real-time session metrics updates
	sessionBroadcaster *SessionBroadcaster

	// ClaudeCLI path (resolved absolute path)
	claudePath string

	// turnExecutor is injected for testing to avoid spawning real Claude CLI.
	// When set, passed to sub-executors (StandardExecutor, FullExecutor, etc.)
	turnExecutor TurnExecutor
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
	claudePath := ResolveClaudePath(cfg.ClaudePath)
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

	// Create git operations with orc-specific config
	gitCfg := git.Config{
		BranchPrefix:   cfg.BranchPrefix,
		CommitPrefix:   cfg.CommitPrefix,
		WorktreeDir:    config.ResolveWorktreeDir(orcCfg.Worktree.Dir, cfg.WorkDir),
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
		FilterSystemProcesses: orcCfg.Diagnostics.ResourceTracking.FilterSystemProcesses,
	}
	resourceTracker := NewResourceTracker(rtConfig, slog.Default())

	// Open global database for cross-project cost tracking
	// Cost tracking is optional - failures are logged but don't block execution
	globalDB, err := db.OpenGlobal()
	if err != nil {
		slog.Warn("failed to open global database for cost tracking", "error", err)
	}

	return &Executor{
		config:          cfg,
		orcConfig:       orcCfg,
		client:          client,
		gateEvaluator:   gate.New(),
		gitOps:          gitOps,
		logger:          slog.Default(),
		tokenPool:       pool,
		backend:         cfg.Backend,
		resourceTracker: resourceTracker,
		globalDB:        globalDB,
		claudePath:      claudePath,
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
// Also initializes the session broadcaster if a publisher is provided.
func (e *Executor) SetPublisher(p events.Publisher) {
	e.publisher = p

	// Initialize session broadcaster when publisher is set
	if p != nil {
		e.sessionBroadcaster = NewSessionBroadcaster(
			events.NewPublishHelper(p),
			e.backend,
			e.globalDB,
			e.config.WorkDir,
			e.logger,
		)
	}
}

// SetClient sets the Claude client (for testing).
func (e *Executor) SetClient(c claude.Client) {
	e.client = c
}

// Event publishing convenience methods - thin wrappers around PublishHelper.
// These provide backwards-compatible method signatures on Executor.

func (e *Executor) eventPublisher() *events.PublishHelper {
	return events.NewPublishHelper(e.publisher)
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

func (e *Executor) publishState(taskID string, s *orcv1.ExecutionState) {
	e.eventPublisher().State(taskID, s)
}

// Phase execution methods (ExecutePhase, executePhaseWithSession, executePhaseWithFlowgraph)
// are defined in phase.go

// Task execution methods (ExecuteTask, ResumeFromPhase, evaluateGate, etc.)
// are defined in task_execution.go

// PR and completion methods (runCompletion, syncWithTarget, directMerge, createPR, buildPRBody)
// are defined in pr.go

