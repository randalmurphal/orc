// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"log/slog"
	"path/filepath"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
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

	if cfg.ClaudePath != "" {
		clientOpts = append(clientOpts, claude.WithClaudePath(cfg.ClaudePath))
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
		BranchPrefix: cfg.BranchPrefix,
		CommitPrefix: cfg.CommitPrefix,
		WorktreeDir:  orcCfg.Worktree.Dir,
	}
	gitOps, err := git.New(cfg.WorkDir, gitCfg)
	if err != nil {
		// Log warning but don't fail - git might not be initialized
		slog.Warn("failed to initialize git operations", "error", err)
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
				WithFullGitSvc(e.gitOps),
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
				WithGitSvc(e.gitOps),
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
