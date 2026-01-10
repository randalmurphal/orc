// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
	"github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/llmkit/claudeconfig"
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

// Config holds executor configuration.
type Config struct {
	// Claude CLI settings
	ClaudePath                 string
	Model                      string
	DangerouslySkipPermissions bool

	// Tool permissions (from project settings)
	AllowedTools    []string
	DisallowedTools []string

	// Execution settings
	MaxIterations int
	Timeout       time.Duration
	WorkDir       string

	// Git settings
	BranchPrefix string
	CommitPrefix string

	// Template settings
	TemplatesDir string

	// Checkpoint settings
	EnableCheckpoints bool
}

// DefaultConfig returns the default executor configuration.
func DefaultConfig() *Config {
	return &Config{
		ClaudePath:                 "claude",
		Model:                      "claude-opus-4-5-20251101",
		DangerouslySkipPermissions: true,
		MaxIterations:              30,
		Timeout:                    10 * time.Minute,
		WorkDir:                    ".",
		BranchPrefix:               "orc/",
		CommitPrefix:               "[orc]",
		TemplatesDir:               "templates",
		EnableCheckpoints:          true,
	}
}

// ConfigFromOrc creates an executor config from orc config.
func ConfigFromOrc(cfg *config.Config) *Config {
	return &Config{
		ClaudePath:                 cfg.ClaudePath,
		Model:                      cfg.Model,
		DangerouslySkipPermissions: cfg.DangerouslySkipPermissions,
		MaxIterations:              cfg.MaxIterations,
		Timeout:                    cfg.Timeout,
		WorkDir:                    ".",
		BranchPrefix:               cfg.BranchPrefix,
		CommitPrefix:               cfg.CommitPrefix,
		TemplatesDir:               cfg.TemplatesDir,
		EnableCheckpoints:          cfg.EnableCheckpoints,
	}
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
		useSessionExecution: false, // Session-based execution is opt-in for now
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

// LoadProjectToolPermissions loads tool permissions from the project's .claude/settings.json
// and applies them to the executor config if not already set.
// This allows project-level tool restrictions to be enforced during execution.
func (e *Executor) LoadProjectToolPermissions(projectRoot string) error {
	// Only load if not already configured
	if len(e.config.AllowedTools) > 0 || len(e.config.DisallowedTools) > 0 {
		return nil // Already configured, don't override
	}

	settings, err := claudeconfig.LoadProjectSettings(projectRoot)
	if err != nil {
		// No settings file is OK - no tool restrictions
		return nil
	}

	// Check for tool permissions in settings extensions
	perms, err := claudeconfig.GetToolPermissions(settings)
	if err != nil || perms == nil || perms.IsEmpty() {
		return nil
	}

	// Apply permissions to config
	if len(perms.Allow) > 0 {
		e.config.AllowedTools = perms.Allow
		e.logger.Info("loaded allowed tools from project settings", "tools", perms.Allow)
	}
	if len(perms.Deny) > 0 {
		e.config.DisallowedTools = perms.Deny
		e.logger.Info("loaded disallowed tools from project settings", "tools", perms.Deny)
	}

	// Rebuild client with new permissions
	if len(e.config.AllowedTools) > 0 || len(e.config.DisallowedTools) > 0 {
		e.rebuildClient()
	}

	return nil
}

// rebuildClient recreates the Claude client with current config settings.
func (e *Executor) rebuildClient() {
	clientOpts := []claude.ClaudeOption{
		claude.WithModel(e.config.Model),
		claude.WithWorkdir(e.config.WorkDir),
		claude.WithTimeout(e.config.Timeout),
	}

	if e.config.ClaudePath != "" {
		clientOpts = append(clientOpts, claude.WithClaudePath(e.config.ClaudePath))
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

	e.client = claude.NewClaudeCLI(clientOpts...)
}

// publish sends an event if a publisher is configured.
func (e *Executor) publish(event events.Event) {
	if e.publisher != nil {
		e.publisher.Publish(event)
	}
}

// publishPhaseStart publishes a phase start event.
func (e *Executor) publishPhaseStart(taskID, phase string) {
	e.publish(events.NewEvent(events.EventPhase, taskID, events.PhaseUpdate{
		Phase:  phase,
		Status: "started",
	}))
}

// publishPhaseComplete publishes a phase completion event.
func (e *Executor) publishPhaseComplete(taskID, phase, commitSHA string) {
	e.publish(events.NewEvent(events.EventPhase, taskID, events.PhaseUpdate{
		Phase:     phase,
		Status:    "completed",
		CommitSHA: commitSHA,
	}))
}

// publishPhaseFailed publishes a phase failure event.
func (e *Executor) publishPhaseFailed(taskID, phase string, err error) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	e.publish(events.NewEvent(events.EventPhase, taskID, events.PhaseUpdate{
		Phase:  phase,
		Status: "failed",
		Error:  errMsg,
	}))
}

// publishTranscript publishes a transcript line event.
func (e *Executor) publishTranscript(taskID, phase string, iteration int, msgType, content string) {
	e.publish(events.NewEvent(events.EventTranscript, taskID, events.TranscriptLine{
		Phase:     phase,
		Iteration: iteration,
		Type:      msgType,
		Content:   content,
		Timestamp: time.Now(),
	}))
}

// publishTokens publishes a token usage update.
func (e *Executor) publishTokens(taskID, phase string, input, output, total int) {
	e.publish(events.NewEvent(events.EventTokens, taskID, events.TokenUpdate{
		Phase:        phase,
		InputTokens:  input,
		OutputTokens: output,
		TotalTokens:  total,
	}))
}

// publishError publishes an error event.
func (e *Executor) publishError(taskID, phase, message string, fatal bool) {
	e.publish(events.NewEvent(events.EventError, taskID, events.ErrorData{
		Phase:   phase,
		Message: message,
		Fatal:   fatal,
	}))
}

// publishState publishes a full state update.
func (e *Executor) publishState(taskID string, s *state.State) {
	e.publish(events.NewEvent(events.EventState, taskID, s))
}

// ExecutePhase runs a single phase using either session-based or flowgraph execution.
func (e *Executor) ExecutePhase(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	// Use session-based execution if enabled
	if e.useSessionExecution {
		return e.executePhaseWithSession(ctx, t, p, s)
	}

	// Fall back to legacy flowgraph-based execution
	return e.executePhaseWithFlowgraph(ctx, t, p, s)
}

// executePhaseWithSession runs a phase using session-based execution.
// This provides context continuity via Claude's native session management.
func (e *Executor) executePhaseWithSession(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	// Get the appropriate executor for this task's weight
	executor := e.getPhaseExecutor(t.Weight)

	e.logger.Info("executing phase with session",
		"phase", p.ID,
		"task", t.ID,
		"weight", t.Weight,
		"executor", executor.Name(),
	)

	// Delegate to the weight-appropriate executor
	return executor.Execute(ctx, t, p, s)
}

// executePhaseWithFlowgraph runs a phase using the legacy flowgraph-based execution.
func (e *Executor) executePhaseWithFlowgraph(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: plan.PhaseRunning,
	}

	// Build phase graph
	graph := flowgraph.NewGraph[PhaseState]()

	// Add nodes
	graph.AddNode("prompt", e.buildPromptNode(p))
	graph.AddNode("execute", e.executeClaudeNode())
	graph.AddNode("check", e.checkCompletionNode(p, s))
	graph.AddNode("commit", e.commitCheckpointNode())

	// Set up edges - Ralph-style loop
	graph.SetEntry("prompt")
	graph.AddEdge("prompt", "execute")
	graph.AddEdge("execute", "check")

	maxIter := e.config.MaxIterations
	if p.Config != nil {
		if mi, ok := p.Config["max_iterations"].(int); ok {
			maxIter = mi
		}
	}

	graph.AddConditionalEdge("check", func(ctx flowgraph.Context, ps PhaseState) string {
		if ps.Complete {
			return "commit"
		}
		if ps.Iteration >= maxIter {
			return flowgraph.END // Max iterations reached
		}
		if ps.Blocked {
			return flowgraph.END // Blocked, needs intervention
		}
		return "prompt" // Loop back for another iteration
	})
	graph.AddEdge("commit", flowgraph.END)

	// Compile graph
	compiled, err := graph.Compile()
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("compile phase graph: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}

	// Create flowgraph context with LLM injected via context.WithValue
	baseCtx := WithLLM(ctx, e.client)
	fgCtx := flowgraph.NewContext(baseCtx,
		flowgraph.WithLogger(e.logger),
		flowgraph.WithContextRunID(fmt.Sprintf("%s-%s", t.ID, p.ID)),
	)

	// Initial state with retry context if applicable
	initialState := PhaseState{
		TaskID:       t.ID,
		TaskTitle:    t.Title,
		Phase:        p.ID,
		Weight:       string(t.Weight),
		Iteration:    0,
		RetryContext: e.loadRetryContextForPhase(s),
	}

	// Run with checkpointing if enabled
	var runOpts []flowgraph.RunOption
	runOpts = append(runOpts, flowgraph.WithMaxIterations(maxIter*4+10)) // Buffer for nodes per iteration

	if e.checkpointStore != nil {
		runOpts = append(runOpts,
			flowgraph.WithCheckpointing(e.checkpointStore),
			flowgraph.WithRunID(fmt.Sprintf("%s-%s", t.ID, p.ID)),
		)
	}

	// Execute
	finalState, err := compiled.Run(fgCtx, initialState, runOpts...)

	// Build result
	result.Iterations = finalState.Iteration
	result.Output = finalState.Response
	result.CommitSHA = finalState.CommitSHA
	result.Artifacts = finalState.Artifacts
	result.InputTokens = finalState.InputTokens
	result.OutputTokens = finalState.OutputTokens
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = err
		return result, err
	}

	if finalState.Complete {
		result.Status = plan.PhaseCompleted
	} else if finalState.Blocked {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("phase blocked: needs clarification")
	} else {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("max iterations (%d) reached without completion", maxIter)
	}

	return result, result.Error
}

// buildPromptNode creates the prompt building node.
func (e *Executor) buildPromptNode(p *plan.Phase) flowgraph.NodeFunc[PhaseState] {
	return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
		// Load template from templates/prompts/{phase}.md
		templatePath := filepath.Join(e.config.TemplatesDir, "prompts", p.Name+".md")
		tmplContent, err := os.ReadFile(templatePath)
		if err != nil {
			// Try with ID if name doesn't exist
			templatePath = filepath.Join(e.config.TemplatesDir, "prompts", p.ID+".md")
			tmplContent, err = os.ReadFile(templatePath)
			if err != nil {
				// Use inline prompt from plan if template doesn't exist
				if p.Prompt != "" {
					s.Prompt = e.renderTemplate(p.Prompt, s)
				} else {
					return s, fmt.Errorf("no prompt template found for phase %s", p.ID)
				}
				s.Iteration++
				return s, nil
			}
		}

		// Render template with task context
		s.Prompt = e.renderTemplate(string(tmplContent), s)
		s.Iteration++
		return s, nil
	}
}

// renderTemplate does simple template variable substitution.
func (e *Executor) renderTemplate(tmpl string, s PhaseState) string {
	// Simple variable replacement
	replacements := map[string]string{
		"{{TASK_ID}}":          s.TaskID,
		"{{TASK_TITLE}}":       s.TaskTitle,
		"{{TASK_DESCRIPTION}}": s.TaskDescription,
		"{{PHASE}}":            s.Phase,
		"{{WEIGHT}}":           s.Weight,
		"{{ITERATION}}":        fmt.Sprintf("%d", s.Iteration),
		"{{RESEARCH_CONTENT}}": s.ResearchContent,
		"{{SPEC_CONTENT}}":     s.SpecContent,
		"{{DESIGN_CONTENT}}":   s.DesignContent,
		"{{RETRY_CONTEXT}}":    s.RetryContext,
	}

	result := tmpl
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	return result
}

// executeClaudeNode creates the Claude execution node.
func (e *Executor) executeClaudeNode() flowgraph.NodeFunc[PhaseState] {
	return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
		// Use LLM client from context (injected via WithLLM)
		client := LLM(ctx)
		if client == nil {
			return s, fmt.Errorf("no LLM client available")
		}

		// Publish prompt transcript
		e.publishTranscript(s.TaskID, s.Phase, s.Iteration, "prompt", s.Prompt)

		// Execute completion
		resp, err := client.Complete(ctx, claude.CompletionRequest{
			Messages: []claude.Message{
				{Role: claude.RoleUser, Content: s.Prompt},
			},
			Model: e.config.Model,
		})
		if err != nil {
			s.Error = err
			e.publishError(s.TaskID, s.Phase, err.Error(), false)
			return s, fmt.Errorf("claude completion: %w", err)
		}

		s.Response = resp.Content
		s.InputTokens += resp.Usage.InputTokens
		s.OutputTokens += resp.Usage.OutputTokens
		s.TokensUsed += resp.Usage.TotalTokens

		// Publish response transcript and token update
		e.publishTranscript(s.TaskID, s.Phase, s.Iteration, "response", s.Response)
		e.publishTokens(s.TaskID, s.Phase, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.TotalTokens)

		return s, nil
	}
}

// checkCompletionNode creates the completion check node.
func (e *Executor) checkCompletionNode(p *plan.Phase, st *state.State) flowgraph.NodeFunc[PhaseState] {
	return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
		// Detect completion marker in response
		s.Complete = strings.Contains(s.Response, "<phase_complete>true</phase_complete>")

		// Also check for specific phase completion tag
		phaseCompleteTag := fmt.Sprintf("<%s_complete>true</%s_complete>", p.ID, p.ID)
		if strings.Contains(s.Response, phaseCompleteTag) {
			s.Complete = true
		}

		// Check for blocked state
		if strings.Contains(s.Response, "<phase_blocked>") {
			s.Blocked = true
		}

		// Update state tracking
		if st != nil {
			st.IncrementIteration()
			st.AddTokens(s.InputTokens, s.OutputTokens)
		}

		// Save transcript for this iteration
		if err := e.saveTranscript(s); err != nil {
			ctx.Logger().Warn("failed to save transcript", "error", err)
		}

		return s, nil
	}
}

// commitCheckpointNode creates the git commit checkpoint node.
func (e *Executor) commitCheckpointNode() flowgraph.NodeFunc[PhaseState] {
	return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
		// Skip if git operations not available
		if e.gitOps == nil {
			return s, nil
		}

		// Create git checkpoint
		msg := fmt.Sprintf("%s: %s - completed", s.Phase, s.TaskTitle)
		cp, err := e.gitOps.CreateCheckpoint(s.TaskID, s.Phase, msg)
		if err != nil {
			ctx.Logger().Warn("failed to create git checkpoint", "error", err)
			// Don't fail the phase for git errors
			return s, nil
		}

		s.CommitSHA = cp.CommitSHA
		return s, nil
	}
}

// saveTranscript saves the prompt/response for this iteration.
func (e *Executor) saveTranscript(s PhaseState) error {
	dir := filepath.Join(e.config.WorkDir, ".orc", "tasks", s.TaskID, "transcripts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-%03d.md", s.Phase, s.Iteration)
	path := filepath.Join(dir, filename)

	content := fmt.Sprintf(`# %s - Iteration %d

## Prompt

%s

## Response

%s

---
Tokens: %d input, %d output
Complete: %v
Blocked: %v
`,
		s.Phase, s.Iteration, s.Prompt, s.Response,
		s.InputTokens, s.OutputTokens, s.Complete, s.Blocked)

	return os.WriteFile(path, []byte(content), 0644)
}

// ExecuteTask runs all phases of a task with gate evaluation and cross-phase retry.
func (e *Executor) ExecuteTask(ctx context.Context, t *task.Task, p *plan.Plan, s *state.State) error {
	// Set current task directory for saving files
	e.currentTaskDir = e.taskDir(t.ID)

	// Update task status
	t.Status = task.StatusRunning
	now := time.Now()
	t.StartedAt = &now
	if err := t.SaveTo(e.currentTaskDir); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	// Setup worktree if enabled
	if e.orcConfig.Worktree.Enabled && e.gitOps != nil {
		worktreePath, err := e.setupWorktree(t.ID)
		if err != nil {
			return fmt.Errorf("setup worktree: %w", err)
		}
		e.worktreePath = worktreePath
		e.worktreeGit = e.gitOps.InWorktree(worktreePath)
		e.logger.Info("created worktree", "task", t.ID, "path", worktreePath)

		// Create a new Claude client for the worktree context
		// This ensures all Claude work happens in the isolated worktree
		worktreeClientOpts := []claude.ClaudeOption{
			claude.WithModel(e.config.Model),
			claude.WithWorkdir(worktreePath),
			claude.WithTimeout(e.config.Timeout),
		}
		if e.config.ClaudePath != "" {
			worktreeClientOpts = append(worktreeClientOpts, claude.WithClaudePath(e.config.ClaudePath))
		}
		if e.config.DangerouslySkipPermissions {
			worktreeClientOpts = append(worktreeClientOpts, claude.WithDangerouslySkipPermissions())
		}
		// Apply tool permissions to worktree client
		if len(e.config.AllowedTools) > 0 {
			worktreeClientOpts = append(worktreeClientOpts, claude.WithAllowedTools(e.config.AllowedTools))
		}
		if len(e.config.DisallowedTools) > 0 {
			worktreeClientOpts = append(worktreeClientOpts, claude.WithDisallowedTools(e.config.DisallowedTools))
		}
		e.client = claude.NewClaudeCLI(worktreeClientOpts...)
		e.logger.Info("claude client configured for worktree", "path", worktreePath)

		// Create new session manager for worktree context
		e.sessionMgr = session.NewManager(
			session.WithDefaultSessionOptions(
				session.WithModel(e.config.Model),
				session.WithWorkdir(worktreePath),
				session.WithPermissions(e.config.DangerouslySkipPermissions),
			),
		)

		// Reset phase executors to use new worktree context
		e.resetPhaseExecutors()

		// Cleanup worktree on exit based on config and success
		defer func() {
			if e.worktreePath != "" {
				shouldCleanup := (t.Status == task.StatusCompleted && e.orcConfig.Worktree.CleanupOnComplete) ||
					(t.Status == task.StatusFailed && e.orcConfig.Worktree.CleanupOnFail)
				if shouldCleanup {
					if err := e.gitOps.CleanupWorktree(t.ID); err != nil {
						e.logger.Warn("failed to cleanup worktree", "error", err)
					} else {
						e.logger.Info("cleaned up worktree", "task", t.ID)
					}
				}
			}
		}()
	}

	// Track retry counts per phase
	retryCounts := make(map[string]int)

	// Execute phases with potential retry loop
	i := 0
	for i < len(p.Phases) {
		phase := &p.Phases[i]

		// Skip completed phases
		if s.IsPhaseCompleted(phase.ID) {
			i++
			continue
		}

		// Start phase
		s.StartPhase(phase.ID)
		if err := s.SaveTo(e.currentTaskDir); err != nil {
			return fmt.Errorf("save state: %w", err)
		}

		e.logger.Info("executing phase", "phase", phase.ID, "task", t.ID)

		// Publish phase start event
		e.publishPhaseStart(t.ID, phase.ID)
		e.publishState(t.ID, s)

		// Execute phase
		result, err := e.ExecutePhase(ctx, t, phase, s)
		if err != nil {
			// Check for context cancellation (interrupt)
			if ctx.Err() != nil {
				s.InterruptPhase(phase.ID)
				s.SaveTo(e.currentTaskDir)
				return ctx.Err()
			}

			// Check if we should retry from an earlier phase
			retryFrom := e.orcConfig.ShouldRetryFrom(phase.ID)
			if retryFrom != "" && retryCounts[phase.ID] < e.orcConfig.Retry.MaxRetries {
				retryCounts[phase.ID]++
				e.logger.Info("phase failed, retrying from earlier phase",
					"failed_phase", phase.ID,
					"retry_from", retryFrom,
					"attempt", retryCounts[phase.ID],
				)

				// Save retry context with failure details
				failureOutput := result.Output
				if failureOutput == "" && err != nil {
					failureOutput = err.Error()
				}
				reason := fmt.Sprintf("Phase %s failed: %v", phase.ID, err)
				s.SetRetryContext(phase.ID, retryFrom, reason, failureOutput, retryCounts[phase.ID])

				// Save detailed context to file
				contextFile, saveErr := e.saveRetryContextFile(t.ID, phase.ID, retryFrom, reason, failureOutput, retryCounts[phase.ID])
				if saveErr != nil {
					e.logger.Warn("failed to save retry context file", "error", saveErr)
				} else {
					s.SetRetryContextFile(contextFile)
				}

				// Find the retry phase index and reset phases from there
				for j, ph := range p.Phases {
					if ph.ID == retryFrom {
						// Reset phases from retry point onwards
						for k := j; k <= i; k++ {
							s.ResetPhase(p.Phases[k].ID)
						}
						i = j // Jump back to retry phase
						break
					}
				}
				s.SaveTo(e.currentTaskDir)
				continue
			}

			// No retry available, fail the task
			s.FailPhase(phase.ID, err)
			s.SaveTo(e.currentTaskDir)
			t.Status = task.StatusFailed
			t.SaveTo(e.currentTaskDir)

			// Publish failure events
			e.publishPhaseFailed(t.ID, phase.ID, err)
			e.publishError(t.ID, phase.ID, err.Error(), true)
			e.publishState(t.ID, s)

			return fmt.Errorf("phase %s failed: %w", phase.ID, err)
		}

		// Complete phase
		s.CompletePhase(phase.ID, result.CommitSHA)
		phase.Status = plan.PhaseCompleted
		phase.CommitSHA = result.CommitSHA

		// Clear retry context on successful completion
		if s.HasRetryContext() {
			s.ClearRetryContext()
		}

		// Save state and plan
		if err := s.SaveTo(e.currentTaskDir); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
		if err := p.SaveTo(e.currentTaskDir); err != nil {
			return fmt.Errorf("save plan: %w", err)
		}

		// Publish phase completion events
		e.publishPhaseComplete(t.ID, phase.ID, result.CommitSHA)
		e.publishTokens(t.ID, phase.ID, result.InputTokens, result.OutputTokens, result.InputTokens+result.OutputTokens)
		e.publishState(t.ID, s)

		// Evaluate gate if present (gate.Type != "" means gate is configured)
		if phase.Gate.Type != "" {
			decision, gateErr := e.evaluateGate(ctx, phase, result.Output, string(t.Weight))
			if gateErr != nil {
				e.logger.Warn("gate evaluation failed", "error", gateErr)
				// Continue on gate error - don't block automation
			} else if !decision.Approved {
				// Gate rejected - check if we should retry
				retryFrom := e.orcConfig.ShouldRetryFrom(phase.ID)
				if retryFrom != "" && retryCounts[phase.ID] < e.orcConfig.Retry.MaxRetries {
					retryCounts[phase.ID]++
					e.logger.Info("gate rejected, retrying from earlier phase",
						"failed_phase", phase.ID,
						"reason", decision.Reason,
						"retry_from", retryFrom,
					)

					// Save retry context with gate rejection details
					reason := fmt.Sprintf("Gate rejected for phase %s: %s", phase.ID, decision.Reason)
					s.SetRetryContext(phase.ID, retryFrom, reason, result.Output, retryCounts[phase.ID])

					// Save detailed context to file
					contextFile, saveErr := e.saveRetryContextFile(t.ID, phase.ID, retryFrom, reason, result.Output, retryCounts[phase.ID])
					if saveErr != nil {
						e.logger.Warn("failed to save retry context file", "error", saveErr)
					} else {
						s.SetRetryContextFile(contextFile)
					}

					// Find and reset to retry phase
					for j, ph := range p.Phases {
						if ph.ID == retryFrom {
							for k := j; k <= i; k++ {
								s.ResetPhase(p.Phases[k].ID)
							}
							i = j
							break
						}
					}
					s.SaveTo(e.currentTaskDir)
					continue
				}

				// No retry - record rejection and continue (automation-first)
				e.logger.Warn("gate rejected, continuing anyway (automation mode)",
					"phase", phase.ID,
					"reason", decision.Reason,
				)
				s.RecordGateDecision(phase.ID, string(phase.Gate.Type), decision.Approved, decision.Reason)
			} else {
				s.RecordGateDecision(phase.ID, string(phase.Gate.Type), decision.Approved, decision.Reason)
			}
		}

		i++ // Move to next phase
	}

	// Complete task
	s.Complete()
	s.SaveTo(e.currentTaskDir)

	t.Status = task.StatusCompleted
	completedAt := time.Now()
	t.CompletedAt = &completedAt
	t.SaveTo(e.currentTaskDir)

	// Run completion action (merge/PR)
	if err := e.runCompletion(ctx, t); err != nil {
		e.logger.Warn("completion action failed", "error", err)
		// Don't fail the task for completion errors
	}

	// Publish completion event
	e.publish(events.NewEvent(events.EventComplete, t.ID, events.CompleteData{
		Status: "completed",
	}))
	e.publishState(t.ID, s)

	return nil
}

// evaluateGate evaluates a phase gate using configured gate type.
func (e *Executor) evaluateGate(ctx context.Context, phase *plan.Phase, output string, weight string) (*gate.Decision, error) {
	// Resolve effective gate type from config
	gateType := e.orcConfig.ResolveGateType(phase.ID, weight)

	// For auto gates with AutoApproveOnSuccess, just approve
	if gateType == "auto" && e.orcConfig.Gates.AutoApproveOnSuccess {
		return &gate.Decision{
			Approved: true,
			Reason:   "auto-approved on success",
		}, nil
	}

	// Override the gate type from config
	effectiveGate := &plan.Gate{
		Type:     plan.GateType(gateType),
		Criteria: phase.Gate.Criteria,
	}

	return e.gateEvaluator.Evaluate(ctx, effectiveGate, output)
}

// ResumeFromPhase resumes execution from a specific phase.
func (e *Executor) ResumeFromPhase(ctx context.Context, t *task.Task, p *plan.Plan, s *state.State, phaseID string) error {
	// Find the phase index
	startIdx := -1
	for i, phase := range p.Phases {
		if phase.ID == phaseID {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return fmt.Errorf("phase %s not found in plan", phaseID)
	}

	// Reset the interrupted phase
	s.ResetPhase(phaseID)

	// Create a sub-plan starting from the resume point
	resumePlan := &plan.Plan{
		Version:     p.Version,
		Weight:      p.Weight,
		Description: p.Description,
		Phases:      p.Phases[startIdx:],
	}

	// Use ExecuteTask which handles gates and retry
	return e.ExecuteTask(ctx, t, resumePlan, s)
}

// saveRetryContextFile saves detailed retry context to a markdown file.
func (e *Executor) saveRetryContextFile(taskID, fromPhase, toPhase, reason, output string, attempt int) (string, error) {
	dir := filepath.Join(e.config.WorkDir, ".orc", "tasks", taskID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	filename := fmt.Sprintf("retry-context-%s-%d.md", fromPhase, attempt)
	path := filepath.Join(dir, filename)

	content := fmt.Sprintf(`# Retry Context

## Summary
- **From Phase**: %s
- **To Phase**: %s
- **Attempt**: %d
- **Timestamp**: %s

## Reason
%s

## Output from Failed Phase

%s
`, fromPhase, toPhase, attempt, time.Now().Format(time.RFC3339), reason, output)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	return path, nil
}

// loadRetryContextForPhase loads retry context for prompt injection.
func (e *Executor) loadRetryContextForPhase(s *state.State) string {
	rc := s.GetRetryContext()
	if rc == nil {
		return ""
	}

	// Build a summary for prompt injection
	context := fmt.Sprintf(`## Retry Context

This phase is being re-executed due to a failure in a later phase.

**What happened:**
- Phase "%s" failed/was rejected
- Reason: %s
- This is retry attempt #%d

**What to fix:**
Please address the issues that caused the later phase to fail. The failure output is below:

---
%s
---

Focus on fixing the root cause of these issues in this phase.
`, rc.FromPhase, rc.Reason, rc.Attempt, rc.FailureOutput)

	// If there's a context file with more details, reference it
	if rc.ContextFile != "" {
		context += fmt.Sprintf("\nDetailed context saved to: %s\n", rc.ContextFile)
	}

	return context
}

// setupWorktree creates or reuses an isolated worktree for the task.
func (e *Executor) setupWorktree(taskID string) (string, error) {
	if e.gitOps == nil {
		return "", fmt.Errorf("git operations not available")
	}

	targetBranch := e.orcConfig.Completion.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	// Check if worktree already exists
	worktreePath := e.gitOps.WorktreePath(taskID)
	if _, err := os.Stat(worktreePath); err == nil {
		// Worktree exists, reuse it
		e.logger.Info("reusing existing worktree", "task", taskID, "path", worktreePath)
		return worktreePath, nil
	}

	return e.gitOps.CreateWorktree(taskID, targetBranch)
}

// runCompletion executes the completion action (merge/PR/none).
func (e *Executor) runCompletion(ctx context.Context, t *task.Task) error {
	action := e.orcConfig.Completion.Action
	if action == "" || action == "none" {
		return nil
	}

	if e.gitOps == nil {
		return fmt.Errorf("git operations not available")
	}

	// Sync with target branch before completion
	if err := e.syncWithTarget(ctx, t); err != nil {
		return fmt.Errorf("sync with target: %w", err)
	}

	switch action {
	case "merge":
		return e.directMerge(ctx, t)
	case "pr":
		return e.createPR(ctx, t)
	default:
		e.logger.Warn("unknown completion action", "action", action)
		return nil
	}
}

// syncWithTarget rebases the task branch onto the target branch.
func (e *Executor) syncWithTarget(ctx context.Context, t *task.Task) error {
	cfg := e.orcConfig.Completion
	targetBranch := cfg.TargetBranch
	if targetBranch == "" {
		targetBranch = "main"
	}

	// Use worktree git if available
	gitOps := e.gitOps
	if e.worktreeGit != nil {
		gitOps = e.worktreeGit
	}

	e.logger.Info("syncing with target branch", "target", targetBranch)

	// Fetch latest from remote
	if err := gitOps.Fetch("origin"); err != nil {
		e.logger.Warn("fetch failed, continuing anyway", "error", err)
	}

	// Rebase onto target
	target := "origin/" + targetBranch
	if err := gitOps.Rebase(target); err != nil {
		return fmt.Errorf("rebase onto %s: %w", target, err)
	}

	e.logger.Info("synced with target branch", "target", targetBranch)
	return nil
}

// directMerge merges the task branch directly into the target branch.
func (e *Executor) directMerge(ctx context.Context, t *task.Task) error {
	cfg := e.orcConfig.Completion
	taskBranch := e.gitOps.BranchName(t.ID)

	// Use worktree git if available, otherwise main repo
	gitOps := e.gitOps
	if e.worktreeGit != nil {
		gitOps = e.worktreeGit
	}

	// Checkout target branch
	if err := gitOps.Context().Checkout(cfg.TargetBranch); err != nil {
		return fmt.Errorf("checkout %s: %w", cfg.TargetBranch, err)
	}

	// Merge task branch
	if err := gitOps.Merge(taskBranch, true); err != nil {
		return fmt.Errorf("merge %s: %w", taskBranch, err)
	}

	// Push to remote
	if err := gitOps.Push("origin", cfg.TargetBranch, false); err != nil {
		e.logger.Warn("failed to push after merge", "error", err)
	}

	// Delete task branch if configured
	if cfg.DeleteBranch {
		if err := gitOps.DeleteBranch(taskBranch, false); err != nil {
			e.logger.Warn("failed to delete task branch", "error", err)
		}
	}

	e.logger.Info("merged task branch", "task", t.ID, "branch", taskBranch, "target", cfg.TargetBranch)
	return nil
}

// createPR creates a pull request for the task branch.
func (e *Executor) createPR(ctx context.Context, t *task.Task) error {
	cfg := e.orcConfig.Completion
	taskBranch := e.gitOps.BranchName(t.ID)

	// Use worktree git if available
	gitOps := e.gitOps
	if e.worktreeGit != nil {
		gitOps = e.worktreeGit
	}

	// Push task branch to remote
	if err := gitOps.Push("origin", taskBranch, true); err != nil {
		return fmt.Errorf("push branch: %w", err)
	}

	// Build PR title
	title := cfg.PR.Title
	if title == "" {
		title = "[orc] {{TASK_TITLE}}"
	}
	title = strings.ReplaceAll(title, "{{TASK_TITLE}}", t.Title)
	title = strings.ReplaceAll(title, "{{TASK_ID}}", t.ID)

	// Build PR body
	body := e.buildPRBody(t)

	// Create PR using gh CLI
	args := []string{"pr", "create",
		"--title", title,
		"--body", body,
		"--base", cfg.TargetBranch,
		"--head", taskBranch,
	}

	// Add labels
	for _, label := range cfg.PR.Labels {
		args = append(args, "--label", label)
	}

	// Add reviewers
	for _, reviewer := range cfg.PR.Reviewers {
		args = append(args, "--reviewer", reviewer)
	}

	// Add draft flag
	if cfg.PR.Draft {
		args = append(args, "--draft")
	}

	// Run gh CLI
	output, err := e.runGH(ctx, args...)
	if err != nil {
		return fmt.Errorf("create PR: %w", err)
	}

	// Extract PR URL from output
	prURL := strings.TrimSpace(output)
	if prURL != "" {
		if t.Metadata == nil {
			t.Metadata = make(map[string]string)
		}
		t.Metadata["pr_url"] = prURL
		t.SaveTo(e.currentTaskDir)
	}

	e.logger.Info("created pull request", "task", t.ID, "url", prURL)

	// Enable auto-merge if configured
	if cfg.PR.AutoMerge && prURL != "" {
		if _, err := e.runGH(ctx, "pr", "merge", prURL, "--auto", "--squash"); err != nil {
			e.logger.Warn("failed to enable auto-merge", "error", err)
		} else {
			e.logger.Info("enabled auto-merge", "task", t.ID)
		}
	}

	return nil
}

// buildPRBody constructs the PR body from task information.
func (e *Executor) buildPRBody(t *task.Task) string {
	var sb strings.Builder

	sb.WriteString("## Summary\n\n")
	if t.Description != "" {
		sb.WriteString(t.Description)
	} else {
		sb.WriteString(t.Title)
	}
	sb.WriteString("\n\n")

	sb.WriteString("## Task Details\n\n")
	sb.WriteString(fmt.Sprintf("- **Task ID**: %s\n", t.ID))
	sb.WriteString(fmt.Sprintf("- **Weight**: %s\n", t.Weight))
	sb.WriteString("\n")

	sb.WriteString("## Test Plan\n\n")
	sb.WriteString("- [ ] Automated tests passed\n")
	sb.WriteString("- [ ] Manual verification completed\n")
	sb.WriteString("\n")

	sb.WriteString("---\n")
	sb.WriteString("*Created by [orc](https://github.com/randalmurphal/orc)*\n")

	return sb.String()
}

// runGH executes a gh CLI command.
func (e *Executor) runGH(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)

	// Use worktree path if available
	if e.worktreePath != "" {
		cmd.Dir = e.worktreePath
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, output)
	}

	return string(output), nil
}
