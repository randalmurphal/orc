// Package executor provides the execution engine for orc.
// This file contains the workflow execution system which replaces task-centric execution.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
	"github.com/randalmurphal/orc/internal/workflow"
)

// ContextType determines how the workflow is executed.
type ContextType string

const (
	// ContextDefault creates a new task with worktree.
	ContextDefault ContextType = "default"

	// ContextTask attaches to an existing task.
	ContextTask ContextType = "task"

	// ContextBranch operates on an existing branch without a task.
	ContextBranch ContextType = "branch"

	// ContextPR operates on a pull request branch.
	ContextPR ContextType = "pr"

	// ContextStandalone runs without task or special git setup.
	ContextStandalone ContextType = "standalone"
)

// WorkflowRunOptions configures a workflow run.
type WorkflowRunOptions struct {
	// ContextType determines how the workflow executes.
	ContextType ContextType

	// Prompt is the user-provided task description.
	Prompt string

	// Instructions are additional guidance for this run.
	Instructions string

	// TaskID is set when ContextType is ContextTask.
	TaskID string

	// Branch is set when ContextType is ContextBranch.
	Branch string

	// PRID is set when ContextType is ContextPR.
	PRID int

	// Category helps Claude understand the type of work.
	Category task.Category

	// Variables are additional variables to inject.
	Variables map[string]string

	// Stream enables real-time output streaming.
	Stream bool
}

// WorkflowExecutor runs workflows using the new database-first workflow system.
type WorkflowExecutor struct {
	backend    storage.Backend
	projectDB  *db.ProjectDB
	orcConfig  *config.Config
	resolver   *variable.Resolver
	logger     *slog.Logger
	workingDir string
	claudePath string

	// Optional components
	gitOps    *git.Git
	publisher *PublishHelper
}

// WorkflowExecutorOption configures a WorkflowExecutor.
type WorkflowExecutorOption func(*WorkflowExecutor)

// WithWorkflowGitOps sets the git operations handler.
func WithWorkflowGitOps(g *git.Git) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.gitOps = g
	}
}

// WithWorkflowPublisher sets the event publisher.
func WithWorkflowPublisher(p events.Publisher) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.publisher = NewPublishHelper(p)
	}
}

// WithWorkflowLogger sets the logger.
func WithWorkflowLogger(l *slog.Logger) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.logger = l
	}
}

// WithWorkflowClaudePath sets the path to the Claude CLI executable.
func WithWorkflowClaudePath(path string) WorkflowExecutorOption {
	return func(we *WorkflowExecutor) {
		we.claudePath = path
	}
}

// NewWorkflowExecutor creates a new workflow executor.
func NewWorkflowExecutor(
	backend storage.Backend,
	projectDB *db.ProjectDB,
	orcConfig *config.Config,
	workingDir string,
	opts ...WorkflowExecutorOption,
) *WorkflowExecutor {
	we := &WorkflowExecutor{
		backend:    backend,
		projectDB:  projectDB,
		orcConfig:  orcConfig,
		resolver:   variable.NewResolver(workingDir),
		workingDir: workingDir,
		logger:     slog.Default(),
		claudePath: "claude",
		publisher:  NewPublishHelper(nil), // Initialize with nil-safe wrapper
	}

	for _, opt := range opts {
		opt(we)
	}

	return we
}

// Run executes a workflow with the given options.
// This is the main entry point for workflow execution.
func (we *WorkflowExecutor) Run(ctx context.Context, workflowID string, opts WorkflowRunOptions) (*WorkflowRunResult, error) {
	// Load workflow from database
	wf, err := we.projectDB.GetWorkflow(workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow %s: %w", workflowID, err)
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	// Load workflow phases
	phases, err := we.projectDB.GetWorkflowPhases(workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow phases: %w", err)
	}

	// Load workflow variables
	workflowVars, err := we.projectDB.GetWorkflowVariables(workflowID)
	if err != nil {
		return nil, fmt.Errorf("load workflow variables: %w", err)
	}

	// Create workflow run record
	runID, err := we.backend.GetNextWorkflowRunID()
	if err != nil {
		return nil, fmt.Errorf("get next run ID: %w", err)
	}

	// Build context data based on context type
	contextData := we.buildContextData(opts)

	run := &db.WorkflowRun{
		ID:           runID,
		WorkflowID:   workflowID,
		ContextType:  string(opts.ContextType),
		ContextData:  contextData,
		Prompt:       opts.Prompt,
		Instructions: opts.Instructions,
		Status:       string(workflow.RunStatusPending),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Handle task creation for default context
	var t *task.Task
	if opts.ContextType == ContextDefault {
		t, err = we.createTaskForRun(opts)
		if err != nil {
			return nil, fmt.Errorf("create task: %w", err)
		}
		run.TaskID = &t.ID
	} else if opts.ContextType == ContextTask {
		// Load existing task
		t, err = we.backend.LoadTask(opts.TaskID)
		if err != nil {
			return nil, fmt.Errorf("load task %s: %w", opts.TaskID, err)
		}
		run.TaskID = &t.ID
	}

	// Save run
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		return nil, fmt.Errorf("save workflow run: %w", err)
	}

	// Build resolution context
	rctx := we.buildResolutionContext(opts, t, wf, run)

	// Convert workflow variables to definitions
	varDefs := we.convertToDefinitions(workflowVars)

	// Resolve all variables
	vars, err := we.resolver.ResolveAll(ctx, varDefs, rctx)
	if err != nil {
		we.failRun(run, fmt.Errorf("resolve variables: %w", err))
		return nil, err
	}

	// Store variable snapshot
	varsJSON, _ := json.Marshal(vars)
	run.VariablesSnapshot = string(varsJSON)
	run.Status = string(workflow.RunStatusRunning)
	run.StartedAt = timePtr(time.Now())
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		return nil, fmt.Errorf("save workflow run: %w", err)
	}

	// Execute phases in order
	result := &WorkflowRunResult{
		RunID:        runID,
		WorkflowID:   workflowID,
		StartedAt:    *run.StartedAt,
		PhaseResults: make([]PhaseResult, 0, len(phases)),
	}

	for _, phase := range phases {
		// Load phase template
		tmpl, err := we.projectDB.GetPhaseTemplate(phase.PhaseTemplateID)
		if err != nil {
			we.failRun(run, fmt.Errorf("load phase template %s: %w", phase.PhaseTemplateID, err))
			return result, err
		}
		if tmpl == nil {
			we.failRun(run, fmt.Errorf("phase template not found: %s", phase.PhaseTemplateID))
			return result, fmt.Errorf("phase template not found: %s", phase.PhaseTemplateID)
		}

		// Check for context cancellation
		if ctx.Err() != nil {
			we.interruptRun(run, ctx.Err())
			return result, ctx.Err()
		}

		// Create run phase record
		runPhase := &db.WorkflowRunPhase{
			WorkflowRunID:   runID,
			PhaseTemplateID: phase.PhaseTemplateID,
			Status:          string(workflow.PhaseStatusPending),
		}
		if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
			return result, fmt.Errorf("save run phase: %w", err)
		}

		// Update run with current phase
		run.CurrentPhase = phase.PhaseTemplateID
		if err := we.backend.SaveWorkflowRun(run); err != nil {
			return result, fmt.Errorf("update run phase: %w", err)
		}

		// Execute the phase
		phaseResult, err := we.executePhase(ctx, tmpl, phase, vars, rctx, run, runPhase, t)
		result.PhaseResults = append(result.PhaseResults, phaseResult)

		if err != nil {
			we.failRun(run, err)
			return result, err
		}

		// Update variables with phase output if artifact was produced
		if phaseResult.Artifact != "" {
			vars["OUTPUT_"+phaseResult.PhaseID] = phaseResult.Artifact
			// Update common aliases
			switch phaseResult.PhaseID {
			case "spec", "tiny_spec":
				vars["SPEC_CONTENT"] = phaseResult.Artifact
			case "design":
				vars["DESIGN_CONTENT"] = phaseResult.Artifact
			case "tdd_write":
				vars["TDD_TESTS_CONTENT"] = phaseResult.Artifact
			case "breakdown":
				vars["BREAKDOWN_CONTENT"] = phaseResult.Artifact
			case "research":
				vars["RESEARCH_CONTENT"] = phaseResult.Artifact
			}
			rctx.PriorOutputs[phaseResult.PhaseID] = phaseResult.Artifact
		}
	}

	// Complete run
	run.Status = string(workflow.RunStatusCompleted)
	run.CompletedAt = timePtr(time.Now())
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		return result, fmt.Errorf("complete run: %w", err)
	}

	result.CompletedAt = run.CompletedAt
	result.Success = true

	return result, nil
}

// WorkflowRunResult contains the result of a workflow execution.
type WorkflowRunResult struct {
	RunID        string
	WorkflowID   string
	TaskID       string
	StartedAt    time.Time
	CompletedAt  *time.Time
	Success      bool
	Error        string
	PhaseResults []PhaseResult
	TotalCostUSD float64
	TotalTokens  int
}

// PhaseResult contains the result of a phase execution.
type PhaseResult struct {
	PhaseID      string
	Status       string
	Iterations   int
	DurationMS   int64
	Artifact     string
	Error        string
	InputTokens  int
	OutputTokens int
	CostUSD      float64
}

// buildContextData creates the context data JSON for a run.
func (we *WorkflowExecutor) buildContextData(opts WorkflowRunOptions) string {
	data := map[string]any{
		"prompt":       opts.Prompt,
		"instructions": opts.Instructions,
	}

	switch opts.ContextType {
	case ContextTask:
		data["task_id"] = opts.TaskID
	case ContextBranch:
		data["branch"] = opts.Branch
	case ContextPR:
		data["pr_id"] = opts.PRID
	}

	j, _ := json.Marshal(data)
	return string(j)
}

// createTaskForRun creates a task for a default context run.
func (we *WorkflowExecutor) createTaskForRun(opts WorkflowRunOptions) (*task.Task, error) {
	taskID, err := we.backend.GetNextTaskID()
	if err != nil {
		return nil, fmt.Errorf("get next task ID: %w", err)
	}

	t := &task.Task{
		ID:          taskID,
		Title:       truncateTitle(opts.Prompt),
		Description: opts.Prompt,
		Category:    opts.Category,
		Status:      task.StatusCreated,
		Queue:       task.QueueActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if t.Category == "" {
		t.Category = task.CategoryFeature
	}

	if err := we.backend.SaveTask(t); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}

	return t, nil
}

// buildResolutionContext creates the variable resolution context.
func (we *WorkflowExecutor) buildResolutionContext(
	opts WorkflowRunOptions,
	t *task.Task,
	wf *db.Workflow,
	run *db.WorkflowRun,
) *variable.ResolutionContext {
	rctx := &variable.ResolutionContext{
		WorkflowID:    wf.ID,
		WorkflowRunID: run.ID,
		Prompt:        opts.Prompt,
		Instructions:  opts.Instructions,
		WorkingDir:    we.workingDir,
		ProjectRoot:   we.workingDir,
		PriorOutputs:  make(map[string]string),
	}

	if t != nil {
		rctx.TaskID = t.ID
		rctx.TaskTitle = t.Title
		rctx.TaskDescription = t.Description
		rctx.TaskCategory = string(t.Category)
		rctx.TaskBranch = t.Branch
	}

	// Merge user-provided variables
	if opts.Variables != nil {
		rctx.Environment = opts.Variables
	}

	return rctx
}

// convertToDefinitions converts database workflow variables to variable definitions.
func (we *WorkflowExecutor) convertToDefinitions(wvs []*db.WorkflowVariable) []variable.Definition {
	defs := make([]variable.Definition, len(wvs))
	for i, wv := range wvs {
		defs[i] = variable.Definition{
			Name:         wv.Name,
			Description:  wv.Description,
			SourceType:   variable.SourceType(wv.SourceType),
			SourceConfig: json.RawMessage(wv.SourceConfig),
			Required:     wv.Required,
			DefaultValue: wv.DefaultValue,
			CacheTTL:     time.Duration(wv.CacheTTLSeconds) * time.Second,
		}
	}
	return defs
}

// executePhase runs a single phase of the workflow.
func (we *WorkflowExecutor) executePhase(
	ctx context.Context,
	tmpl *db.PhaseTemplate,
	phase *db.WorkflowPhase,
	vars variable.VariableSet,
	rctx *variable.ResolutionContext,
	run *db.WorkflowRun,
	runPhase *db.WorkflowRunPhase,
	t *task.Task,
) (PhaseResult, error) {
	result := PhaseResult{
		PhaseID: tmpl.ID,
		Status:  string(workflow.PhaseStatusRunning),
	}

	startTime := time.Now()

	// Update phase status
	runPhase.Status = string(workflow.PhaseStatusRunning)
	runPhase.StartedAt = timePtr(startTime)
	if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
		return result, fmt.Errorf("update phase status: %w", err)
	}

	we.logger.Info("executing phase",
		"run_id", run.ID,
		"phase", tmpl.ID,
		"max_iterations", tmpl.MaxIterations,
	)

	// Load prompt template
	promptContent, err := we.loadPhasePrompt(tmpl)
	if err != nil {
		result.Status = string(workflow.PhaseStatusFailed)
		result.Error = err.Error()
		return result, err
	}

	// Render template with variables
	renderedPrompt := variable.RenderTemplate(promptContent, vars)

	// Determine max iterations (phase override or template default)
	maxIter := tmpl.MaxIterations
	if phase.MaxIterationsOverride != nil {
		maxIter = *phase.MaxIterationsOverride
	}

	// Determine model (phase override or template default or global)
	model := we.resolvePhaseModel(tmpl, phase)

	// Build execution context for ClaudeExecutor
	execConfig := PhaseExecutionConfig{
		Prompt:        renderedPrompt,
		MaxIterations: maxIter,
		Model:         model,
		WorkingDir:    we.workingDir,
		TaskID:        rctx.TaskID,
		PhaseID:       tmpl.ID,
		RunID:         run.ID,
		Thinking:      we.shouldUseThinking(tmpl, phase),
	}

	// Execute with ClaudeExecutor
	execResult, err := we.executeWithClaude(ctx, execConfig)
	if err != nil {
		result.Status = string(workflow.PhaseStatusFailed)
		result.Error = err.Error()
		runPhase.Status = string(workflow.PhaseStatusFailed)
		runPhase.Error = result.Error
		runPhase.CompletedAt = timePtr(time.Now())
		we.backend.SaveWorkflowRunPhase(runPhase)
		return result, err
	}

	// Update result
	result.Status = string(workflow.PhaseStatusCompleted)
	result.Iterations = execResult.Iterations
	result.DurationMS = time.Since(startTime).Milliseconds()
	result.InputTokens = execResult.InputTokens
	result.OutputTokens = execResult.OutputTokens
	result.CostUSD = execResult.CostUSD

	// Extract artifact if phase produces one
	if tmpl.ProducesArtifact {
		result.Artifact = execResult.Artifact
		// Save artifact to database
		if result.Artifact != "" && t != nil {
			if err := we.backend.SaveArtifact(t.ID, tmpl.ID, result.Artifact, "workflow"); err != nil {
				we.logger.Warn("failed to save artifact",
					"task", t.ID,
					"phase", tmpl.ID,
					"error", err,
				)
			}
		}
	}

	// Update phase record
	runPhase.Status = string(workflow.PhaseStatusCompleted)
	runPhase.Iterations = result.Iterations
	runPhase.CompletedAt = timePtr(time.Now())
	runPhase.InputTokens = result.InputTokens
	runPhase.OutputTokens = result.OutputTokens
	runPhase.CostUSD = result.CostUSD
	if result.Artifact != "" {
		runPhase.Artifact = result.Artifact
	}
	if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
		we.logger.Warn("failed to save run phase", "error", err)
	}

	// Update run totals
	run.TotalCostUSD += result.CostUSD
	run.TotalInputTokens += result.InputTokens
	run.TotalOutputTokens += result.OutputTokens
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		we.logger.Warn("failed to update run totals", "error", err)
	}

	return result, nil
}

// PhaseExecutionConfig holds configuration for a phase execution.
type PhaseExecutionConfig struct {
	Prompt        string
	MaxIterations int
	Model         string
	WorkingDir    string
	TaskID        string
	PhaseID       string
	RunID         string
	Thinking      bool
}

// PhaseExecutionResult holds the result of a phase execution.
type PhaseExecutionResult struct {
	Iterations   int
	InputTokens  int
	OutputTokens int
	CostUSD      float64
	Artifact     string
	SessionID    string
}

// executeWithClaude runs the phase using Claude CLI.
func (we *WorkflowExecutor) executeWithClaude(ctx context.Context, cfg PhaseExecutionConfig) (*PhaseExecutionResult, error) {
	result := &PhaseExecutionResult{}

	// Inject ultrathink prefix if thinking is enabled
	prompt := cfg.Prompt
	if cfg.Thinking {
		prompt = "ultrathink\n\n" + prompt
	}

	// Generate session ID
	sessionID := fmt.Sprintf("%s-%s-%s", cfg.RunID, cfg.TaskID, cfg.PhaseID)
	result.SessionID = sessionID

	// Get schema for this phase
	schema := GetSchemaForPhase(cfg.PhaseID)

	// Create Claude executor using functional options
	turnExec := NewClaudeExecutor(
		WithClaudePath(we.claudePath),
		WithClaudeWorkdir(cfg.WorkingDir),
		WithClaudeModel(cfg.Model),
		WithClaudeSessionID(sessionID),
		WithClaudeMaxTurns(cfg.MaxIterations),
		WithClaudeLogger(we.logger),
		WithClaudePhaseID(cfg.PhaseID),
	)

	// Set the schema
	if schema != "" {
		// Schema is set via phaseID, which GetSchemaForPhaseWithRound uses
	}

	// Execute turns until completion
	for i := 0; i < cfg.MaxIterations; i++ {
		// Check context
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		result.Iterations++

		// Execute turn
		turnResult, err := turnExec.ExecuteTurn(ctx, prompt)
		if err != nil {
			return result, fmt.Errorf("turn %d: %w", i+1, err)
		}

		// Accumulate tokens
		result.InputTokens += turnResult.Usage.InputTokens
		result.OutputTokens += turnResult.Usage.OutputTokens
		result.CostUSD += turnResult.CostUSD

		// Check for completion
		status, reason, err := ParsePhaseSpecificResponse(cfg.PhaseID, 1, turnResult.Content)
		if err != nil {
			we.logger.Debug("parse phase response failed",
				"phase", cfg.PhaseID,
				"error", err,
			)
			// Continue iteration
			prompt = fmt.Sprintf("Continue. Previous output was not valid JSON. Iteration %d/%d.",
				i+2, cfg.MaxIterations)
			continue
		}

		switch status {
		case PhaseStatusComplete:
			// Extract artifact if present
			result.Artifact = extractArtifactFromJSON(turnResult.Content)
			return result, nil

		case PhaseStatusBlocked:
			return result, fmt.Errorf("phase blocked: %s", reason)

		case PhaseStatusContinue:
			// Continue to next iteration
			prompt = fmt.Sprintf("Continue working. Iteration %d/%d. %s",
				i+2, cfg.MaxIterations, reason)
		}
	}

	return result, fmt.Errorf("max iterations (%d) reached without completion", cfg.MaxIterations)
}

// loadPhasePrompt loads the prompt content for a phase template.
func (we *WorkflowExecutor) loadPhasePrompt(tmpl *db.PhaseTemplate) (string, error) {
	switch tmpl.PromptSource {
	case "embedded":
		// Load from embedded templates
		return we.loadEmbeddedPrompt(tmpl.PromptPath)

	case "db":
		// Use inline prompt content
		if tmpl.PromptContent == "" {
			return "", fmt.Errorf("phase %s has no prompt content", tmpl.ID)
		}
		return tmpl.PromptContent, nil

	case "file":
		// Load from file system
		return we.loadFilePrompt(tmpl.PromptPath)

	default:
		return "", fmt.Errorf("unknown prompt source: %s", tmpl.PromptSource)
	}
}

// loadEmbeddedPrompt loads a prompt from embedded templates.
func (we *WorkflowExecutor) loadEmbeddedPrompt(path string) (string, error) {
	// Import from templates package - fallback to file for now
	fullPath := filepath.Join(we.workingDir, "templates", path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		// Try embedded
		return "", fmt.Errorf("load embedded prompt %s: %w", path, err)
	}
	return string(content), nil
}

// loadFilePrompt loads a prompt from the file system.
func (we *WorkflowExecutor) loadFilePrompt(path string) (string, error) {
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		fullPath = filepath.Join(we.workingDir, ".orc", "prompts", path)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("load prompt file %s: %w", fullPath, err)
	}
	return string(content), nil
}

// resolvePhaseModel determines which model to use for a phase.
func (we *WorkflowExecutor) resolvePhaseModel(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) string {
	// Phase override takes precedence
	if phase.ModelOverride != "" {
		return phase.ModelOverride
	}

	// Template override
	if tmpl.ModelOverride != "" {
		return tmpl.ModelOverride
	}

	// Default to sonnet
	return "sonnet"
}

// shouldUseThinking determines if extended thinking should be enabled.
func (we *WorkflowExecutor) shouldUseThinking(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) bool {
	// Phase override takes precedence
	if phase.ThinkingOverride != nil {
		return *phase.ThinkingOverride
	}

	// Template default
	if tmpl.ThinkingEnabled != nil {
		return *tmpl.ThinkingEnabled
	}

	// Decision phases default to thinking
	switch tmpl.ID {
	case "spec", "design", "review", "validate":
		return true
	}

	return false
}

// failRun marks a run as failed.
func (we *WorkflowExecutor) failRun(run *db.WorkflowRun, err error) {
	run.Status = string(workflow.RunStatusFailed)
	run.Error = err.Error()
	run.CompletedAt = timePtr(time.Now())
	if saveErr := we.backend.SaveWorkflowRun(run); saveErr != nil {
		we.logger.Error("failed to save run failure", "error", saveErr)
	}
}

// interruptRun marks a run as cancelled (interrupted by context cancellation).
func (we *WorkflowExecutor) interruptRun(run *db.WorkflowRun, err error) {
	run.Status = string(workflow.RunStatusCancelled)
	run.Error = err.Error()
	run.CompletedAt = timePtr(time.Now())
	if saveErr := we.backend.SaveWorkflowRun(run); saveErr != nil {
		we.logger.Error("failed to save run interruption", "error", saveErr)
	}
}

// Helper functions

func timePtr(t time.Time) *time.Time {
	return &t
}

func truncateTitle(s string) string {
	const maxLen = 80
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// extractArtifactFromJSON extracts the artifact field from phase JSON output.
func extractArtifactFromJSON(output string) string {
	var data struct {
		Artifact string `json:"artifact"`
	}
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		return ""
	}
	return data.Artifact
}
