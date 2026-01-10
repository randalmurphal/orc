// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
	"github.com/randalmurphal/flowgraph/pkg/flowgraph/checkpoint"
	"github.com/randalmurphal/llmkit/claude"
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

// Executor runs phases using flowgraph and llmkit.
type Executor struct {
	config          *Config
	orcConfig       *config.Config
	client          claude.Client
	gateEvaluator   *gate.Evaluator
	gitOps          *git.Git
	checkpointStore checkpoint.Store
	logger          *slog.Logger
	publisher       events.Publisher
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

	client := claude.NewClaudeCLI(clientOpts...)

	// Create checkpoint store if enabled
	var cpStore checkpoint.Store
	if cfg.EnableCheckpoints {
		cpStore = checkpoint.NewMemoryStore()
	}

	return &Executor{
		config:          cfg,
		orcConfig:       orcCfg,
		client:          client,
		gateEvaluator:   gate.New(client),
		gitOps:          git.New(cfg.WorkDir),
		checkpointStore: cpStore,
		logger:          slog.Default(),
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

// SetClient sets the Claude client (for testing).
func (e *Executor) SetClient(c claude.Client) {
	e.client = c
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

// ExecutePhase runs a single phase using flowgraph.
func (e *Executor) ExecutePhase(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
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

	// Create flowgraph context
	fgCtx := flowgraph.NewContext(ctx,
		flowgraph.WithLogger(e.logger),
		flowgraph.WithLLM(e.client),
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
		// Use LLM client from context or fallback to executor's client
		client := ctx.LLM()
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
	dir := filepath.Join(".orc", "tasks", s.TaskID, "transcripts")
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
	// Update task status
	t.Status = task.StatusRunning
	now := time.Now()
	t.StartedAt = &now
	if err := t.Save(); err != nil {
		return fmt.Errorf("save task: %w", err)
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
		if err := s.Save(); err != nil {
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
				s.Save()
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
				s.Save()
				continue
			}

			// No retry available, fail the task
			s.FailPhase(phase.ID, err)
			s.Save()
			t.Status = task.StatusFailed
			t.Save()

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
		if err := s.Save(); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
		if err := p.Save(t.ID); err != nil {
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
					s.Save()
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
	s.Save()

	t.Status = task.StatusCompleted
	completedAt := time.Now()
	t.CompletedAt = &completedAt
	t.Save()

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
	dir := filepath.Join(".orc", "tasks", taskID)
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
