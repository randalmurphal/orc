// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"log/slog"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/lock"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
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
	InputTokens  int
	OutputTokens int

	// Prior phase content (for template rendering)
	ResearchContent string
	SpecContent     string
	DesignContent   string

	// Retry context (populated when retrying from a failed phase)
	RetryContext string
}

// Config, DefaultConfig, and ConfigFromOrc are defined in config.go

// resolveClaudePath resolves a Claude CLI path to an absolute path.
// This is necessary because when cmd.Dir is set (e.g., for worktrees),
// Go's exec.Command won't perform PATH lookup for relative executables.
// By resolving to absolute path upfront, execution works regardless of cmd.Dir.
func resolveClaudePath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	// Resolve relative path to absolute using PATH lookup
	if absPath, err := exec.LookPath(path); err == nil {
		return absPath
	}
	return path // Fall back to original if lookup fails
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

	// Token pool for automatic account switching (nil if disabled)
	tokenPool *tokenpool.Pool

	// Phase executors by type (created lazily)
	trivialExecutor  *TrivialExecutor
	standardExecutor *StandardExecutor
	fullExecutor     *FullExecutor

	// Runtime state for current task
	worktreePath   string          // Path to worktree if enabled
	worktreeGit    *git.Git        // Git operations for worktree
	currentTaskDir string          // Directory for current task's files
	pidGuard       *lock.PIDGuard  // PID guard for same-user protection

	// Use session-based execution (new) vs flowgraph (legacy)
	useSessionExecution bool
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
	sessionMgr := session.NewManager(
		session.WithDefaultSessionOptions(
			session.WithModel(cfg.Model),
			session.WithWorkdir(cfg.WorkDir),
			session.WithPermissions(cfg.DangerouslySkipPermissions),
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
		useSessionExecution: orcCfg.Execution.UseSessionExecution,
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

// taskDir returns the directory for a task's files.
func (e *Executor) taskDir(taskID string) string {
	return filepath.Join(e.config.WorkDir, ".orc", "tasks", taskID)
}

// SetClient sets the Claude client (for testing).
func (e *Executor) SetClient(c claude.Client) {
	e.client = c
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

	switch execType {
	case ExecutorTypeTrivial:
		if e.trivialExecutor == nil {
			e.trivialExecutor = NewTrivialExecutor(
				WithTrivialClient(e.client),
				WithTrivialPublisher(e.publisher),
				WithTrivialLogger(e.logger),
				WithTrivialConfig(DefaultConfigForWeight(weight)),
			)
		}
		return e.trivialExecutor

	case ExecutorTypeFull:
		if e.fullExecutor == nil {
			e.fullExecutor = NewFullExecutor(
				e.sessionMgr,
				WithFullGitSvc(gitSvc),
				WithFullPublisher(e.publisher),
				WithFullLogger(e.logger),
				WithFullConfig(DefaultConfigForWeight(weight)),
				WithFullWorkingDir(workingDir),
				WithTaskDir(e.currentTaskDir),
			)
		}
		return e.fullExecutor

	default: // ExecutorTypeStandard
		if e.standardExecutor == nil {
			e.standardExecutor = NewStandardExecutor(
				e.sessionMgr,
				WithGitSvc(gitSvc),
				WithPublisher(e.publisher),
				WithExecutorLogger(e.logger),
				WithExecutorConfig(DefaultConfigForWeight(weight)),
				WithWorkingDir(workingDir),
			)
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

func (e *Executor) publishTokens(taskID, phase string, input, output, total int) {
	e.eventPublisher().Tokens(taskID, phase, input, output, total)
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

// setupWorktree creates or reuses an isolated worktree for the task.
func (e *Executor) setupWorktree(taskID string) (string, error) {
	result, err := SetupWorktree(taskID, e.orcConfig, e.gitOps)
	if err != nil {
		return "", err
	}

	if result.Reused {
		e.logger.Info("reusing existing worktree", "task", taskID, "path", result.Path)
	}

	return result.Path, nil
}

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
	clientOpts := []claude.ClaudeOption{
		claude.WithModel(e.config.Model),
		claude.WithWorkdir(e.config.WorkDir),
		claude.WithTimeout(e.config.Timeout),
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
