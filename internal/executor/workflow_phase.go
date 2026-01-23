// workflow_phase.go contains phase execution logic for workflow runs.
// This includes loading prompts, executing phases with Claude, and handling timeouts.
package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
	"github.com/randalmurphal/orc/internal/workflow"
)

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

	// For quality checks
	PhaseTemplate *db.PhaseTemplate
	WorkflowPhase *db.WorkflowPhase
}

// PhaseExecutionResult holds the result of a phase execution.
type PhaseExecutionResult struct {
	Iterations          int
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	CostUSD             float64
	Artifact            string
	SessionID           string
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

	// Publish phase start event for real-time UI updates
	if t != nil {
		we.publisher.PhaseStart(t.ID, tmpl.ID)
	}

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
	// Use worktree path if available, otherwise fall back to original working dir
	execConfig := PhaseExecutionConfig{
		Prompt:        renderedPrompt,
		MaxIterations: maxIter,
		Model:         model,
		WorkingDir:    we.effectiveWorkingDir(),
		TaskID:        rctx.TaskID,
		PhaseID:       tmpl.ID,
		RunID:         run.ID,
		Thinking:      we.shouldUseThinking(tmpl, phase),
		PhaseTemplate: tmpl,
		WorkflowPhase: phase,
	}

	// Execute with ClaudeExecutor
	execResult, err := we.executeWithClaude(ctx, execConfig)
	if err != nil {
		result.Status = string(workflow.PhaseStatusFailed)
		result.Error = err.Error()
		runPhase.Status = string(workflow.PhaseStatusFailed)
		runPhase.Error = result.Error
		runPhase.CompletedAt = timePtr(time.Now())
		if saveErr := we.backend.SaveWorkflowRunPhase(runPhase); saveErr != nil {
			we.logger.Warn("failed to save failed phase state", "phase", tmpl.ID, "error", saveErr)
		}
		// Publish phase failed event for real-time UI updates
		if t != nil {
			we.publisher.PhaseFailed(t.ID, tmpl.ID, err)
		}
		return result, err
	}

	// Update result
	result.Status = string(workflow.PhaseStatusCompleted)
	result.Iterations = execResult.Iterations
	result.DurationMS = time.Since(startTime).Milliseconds()
	result.InputTokens = execResult.InputTokens
	result.OutputTokens = execResult.OutputTokens
	result.CacheCreationTokens = execResult.CacheCreationTokens
	result.CacheReadTokens = execResult.CacheReadTokens
	result.CostUSD = execResult.CostUSD

	// Extract artifact if phase produces one and save to phase_outputs
	if tmpl.ProducesArtifact && result.Artifact == "" {
		result.Artifact = execResult.Artifact
	}
	if result.Artifact != "" && t != nil {
		// Determine output variable name from template or infer from phase ID
		outputVarName := tmpl.OutputVarName
		if outputVarName == "" {
			// Infer standard variable names for known phase types
			switch tmpl.ID {
			case "spec", "tiny_spec":
				outputVarName = "SPEC_CONTENT"
			case "design":
				outputVarName = "DESIGN_CONTENT"
			case "tdd_write":
				outputVarName = "TDD_TESTS_CONTENT"
			case "breakdown":
				outputVarName = "BREAKDOWN_CONTENT"
			case "research":
				outputVarName = "RESEARCH_CONTENT"
			case "docs":
				outputVarName = "DOCS_CONTENT"
			default:
				outputVarName = "OUTPUT_" + strings.ToUpper(strings.ReplaceAll(tmpl.ID, "-", "_"))
			}
		}

		taskID := t.ID
		output := &storage.PhaseOutputInfo{
			WorkflowRunID:   run.ID,
			PhaseTemplateID: tmpl.ID,
			TaskID:          &taskID,
			Content:         result.Artifact,
			OutputVarName:   outputVarName,
			ArtifactType:    tmpl.ArtifactType,
			Source:          "workflow",
			Iteration:       result.Iterations,
		}
		if err := we.backend.SavePhaseOutput(output); err != nil {
			we.logger.Warn("failed to save phase output",
				"task", t.ID,
				"phase", tmpl.ID,
				"output_var", outputVarName,
				"error", err,
			)
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

	// Publish phase complete event for real-time UI updates
	if t != nil {
		we.publisher.PhaseComplete(t.ID, tmpl.ID, "")
		// Trigger automation event for phase completion
		we.triggerAutomationEvent(ctx, automation.EventPhaseCompleted, t, tmpl.ID)
	}

	// Update run totals
	run.TotalCostUSD += result.CostUSD
	run.TotalInputTokens += result.InputTokens
	run.TotalOutputTokens += result.OutputTokens
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		we.logger.Warn("failed to update run totals", "error", err)
	}

	// Record cost to global database for cross-project analytics
	phaseModel := we.resolvePhaseModel(tmpl, phase)
	we.recordCostToGlobal(t, tmpl.ID, result, phaseModel, time.Since(startTime))

	// Update execution state if available
	if we.execState != nil {
		we.execState.CompletePhase(tmpl.ID, "") // Empty commit SHA for workflow phases
		we.execState.AddCost(result.CostUSD)
		if err := we.backend.SaveState(we.execState); err != nil {
			we.logger.Warn("failed to save execution state", "error", err)
		}
	}

	return result, nil
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

	// Use injected TurnExecutor for testing, or create real ClaudeExecutor
	var turnExec TurnExecutor
	if we.turnExecutor != nil {
		turnExec = we.turnExecutor
		turnExec.UpdateSessionID(sessionID)
	} else {
		turnExec = NewClaudeExecutor(
			WithClaudePath(we.claudePath),
			WithClaudeWorkdir(cfg.WorkingDir),
			WithClaudeModel(cfg.Model),
			WithClaudeSessionID(sessionID),
			WithClaudeMaxTurns(cfg.MaxIterations),
			WithClaudeLogger(we.logger),
			WithClaudePhaseID(cfg.PhaseID),
		)
	}

	// Schema is set via phaseID, which GetSchemaForPhaseWithRound uses
	_ = schema // Mark as intentionally unused here

	// Execute turns until completion
	for i := 0; i < cfg.MaxIterations; i++ {
		// Check context
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		result.Iterations++

		// Update iteration count in database for real-time monitoring
		if cfg.RunID != "" && cfg.PhaseID != "" {
			if err := we.backend.UpdatePhaseIterations(cfg.RunID, cfg.PhaseID, result.Iterations); err != nil {
				we.logger.Warn("failed to update phase iterations", "phase", cfg.PhaseID, "error", err)
			}
		}

		// Execute turn
		turnResult, err := turnExec.ExecuteTurn(ctx, prompt)
		if err != nil {
			return result, fmt.Errorf("turn %d: %w", i+1, err)
		}

		// Store transcripts to database
		we.storeTranscripts(cfg.TaskID, cfg.PhaseID, sessionID, cfg.Model, prompt, turnResult, i+1)

		// Accumulate tokens
		result.InputTokens += turnResult.Usage.InputTokens
		result.OutputTokens += turnResult.Usage.OutputTokens
		result.CacheCreationTokens += turnResult.Usage.CacheCreationInputTokens
		result.CacheReadTokens += turnResult.Usage.CacheReadInputTokens
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
			// Run quality checks if configured for this phase
			if checkResult := we.runQualityChecks(ctx, cfg); checkResult != nil {
				if checkResult.HasBlocks {
					we.logger.Info("quality checks failed, continuing iteration",
						"phase", cfg.PhaseID,
						"failures", checkResult.FailureSummary(),
					)
					// Continue with quality check failure context
					prompt = FormatQualityChecksForPrompt(checkResult)
					continue
				}
				// Warnings only - log but continue
				if !checkResult.AllPassed {
					we.logger.Warn("quality checks had warnings",
						"phase", cfg.PhaseID,
						"failures", checkResult.FailureSummary(),
					)
				} else {
					we.logger.Info("quality checks passed", "phase", cfg.PhaseID)
				}
			}

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

// phaseTimeoutError wraps an error to indicate it was caused by PhaseMax timeout.
type phaseTimeoutError struct {
	phase   string
	timeout time.Duration
	taskID  string
	err     error
}

func (e *phaseTimeoutError) Error() string {
	return fmt.Sprintf("phase %s exceeded timeout (%v). Run 'orc resume %s' to retry.", e.phase, e.timeout, e.taskID)
}

func (e *phaseTimeoutError) Unwrap() error {
	return e.err
}

// IsPhaseTimeoutError returns true if the error is a phase timeout error.
func IsPhaseTimeoutError(err error) bool {
	var pte *phaseTimeoutError
	return errors.As(err, &pte)
}

// executePhaseWithTimeout wraps executePhase with PhaseMax timeout if configured.
// PhaseMax=0 means unlimited (no timeout).
// Returns a phaseTimeoutError if the phase times out due to PhaseMax.
// Logs warnings at 50% and 75% of the timeout duration.
func (we *WorkflowExecutor) executePhaseWithTimeout(
	ctx context.Context,
	tmpl *db.PhaseTemplate,
	phase *db.WorkflowPhase,
	vars map[string]string,
	rctx *variable.ResolutionContext,
	run *db.WorkflowRun,
	runPhase *db.WorkflowRunPhase,
	t *task.Task,
) (PhaseResult, error) {
	phaseMax := time.Duration(0)
	if we.orcConfig != nil {
		phaseMax = we.orcConfig.Timeouts.PhaseMax
	}

	if phaseMax <= 0 {
		// No timeout configured, execute directly
		return we.executePhase(ctx, tmpl, phase, vars, rctx, run, runPhase, t)
	}

	// Create timeout context for this phase
	phaseCtx, cancel := context.WithTimeout(ctx, phaseMax)
	defer cancel()

	// Get task ID for logging
	taskID := ""
	if t != nil {
		taskID = t.ID
	}

	// Start timeout monitoring goroutine for warnings at 50% and 75%
	startTime := time.Now()
	warningDone := make(chan struct{})
	go func() {
		defer close(warningDone)

		threshold50 := phaseMax / 2
		threshold75 := phaseMax * 3 / 4

		timer50 := time.NewTimer(threshold50)
		defer timer50.Stop()

		select {
		case <-phaseCtx.Done():
			return
		case <-timer50.C:
			elapsed := time.Since(startTime)
			remaining := phaseMax - elapsed
			we.logger.Warn("phase_max 50% elapsed",
				"phase", tmpl.ID,
				"task", taskID,
				"elapsed", elapsed.Round(time.Second),
				"timeout", phaseMax,
				"remaining", remaining.Round(time.Second),
			)
		}

		timer75 := time.NewTimer(threshold75 - threshold50)
		defer timer75.Stop()

		select {
		case <-phaseCtx.Done():
			return
		case <-timer75.C:
			elapsed := time.Since(startTime)
			remaining := phaseMax - elapsed
			we.logger.Warn("phase_max 75% elapsed",
				"phase", tmpl.ID,
				"task", taskID,
				"elapsed", elapsed.Round(time.Second),
				"timeout", phaseMax,
				"remaining", remaining.Round(time.Second),
			)
		}

		// Wait for context to complete
		<-phaseCtx.Done()
	}()

	result, err := we.executePhase(phaseCtx, tmpl, phase, vars, rctx, run, runPhase, t)

	// Capture the phase context error before canceling.
	// This determines if the timeout was reached (DeadlineExceeded) vs normal completion.
	phaseCtxErr := phaseCtx.Err()

	// Cancel the phase context to signal the warning goroutine to exit.
	// This must be called before waiting on warningDone to avoid deadlock.
	cancel()

	// Wait for warning goroutine to finish
	<-warningDone

	if err != nil {
		// Check if phase context timed out (but parent context is still alive)
		if phaseCtxErr == context.DeadlineExceeded && ctx.Err() == nil {
			we.logger.Error("phase timeout exceeded",
				"phase", tmpl.ID,
				"timeout", phaseMax,
				"task", taskID,
			)
			return result, &phaseTimeoutError{
				phase:   tmpl.ID,
				timeout: phaseMax,
				taskID:  taskID,
				err:     err,
			}
		}
	}
	return result, err
}

// checkSpecRequirements checks if a task has a valid spec for non-trivial weights.
// Returns an error if spec is required but missing or invalid.
// Skips check if the workflow's first phase is "spec" or "tiny_spec" (the spec will be created during execution).
func (we *WorkflowExecutor) checkSpecRequirements(t *task.Task, phases []*db.WorkflowPhase) error {
	if t == nil {
		return nil
	}

	// Trivial tasks don't require specs
	if t.Weight == task.WeightTrivial {
		return nil
	}

	// Skip if workflow starts with spec phase - it will create the spec
	if len(phases) > 0 {
		firstPhase := phases[0].PhaseTemplateID
		if firstPhase == "spec" || firstPhase == "tiny_spec" {
			we.logger.Debug("skipping spec requirement check - workflow starts with spec phase",
				"task", t.ID)
			return nil
		}
	}

	// Check if spec validation is enabled in config
	if we.orcConfig == nil || !we.orcConfig.Plan.RequireSpecForExecution {
		return nil
	}

	// Check if this weight should skip validation
	if slices.Contains(we.orcConfig.Plan.SkipValidationWeights, string(t.Weight)) {
		return nil
	}

	// Check if spec exists using backend
	specExists, err := we.backend.SpecExistsForTask(t.ID)
	if err != nil {
		we.logger.Warn("failed to check spec existence", "task", t.ID, "error", err)
		specExists = false
	}
	if !specExists {
		we.logger.Warn("task has no spec", "task", t.ID, "weight", t.Weight)
		return fmt.Errorf("task %s requires a spec for weight '%s' - run 'orc plan %s' to create one", t.ID, t.Weight, t.ID)
	}

	// Load spec content to validate
	specContent, err := we.backend.GetSpecForTask(t.ID)
	if err != nil || specContent == "" {
		we.logger.Warn("task spec is invalid", "task", t.ID, "weight", t.Weight)
		return fmt.Errorf("task %s has an incomplete spec - run 'orc plan %s' to update it", t.ID, t.ID)
	}

	return nil
}

// storeTranscripts saves user prompt and assistant response to the database.
// This enables transcript viewing via `orc log` without relying on JSONL files.
func (we *WorkflowExecutor) storeTranscripts(taskID, phaseID, sessionID, model, prompt string, result *TurnResult, iteration int) {
	if we.backend == nil || taskID == "" {
		return
	}

	now := time.Now().UnixMilli()

	// Store user prompt
	userTranscript := &storage.Transcript{
		TaskID:      taskID,
		Phase:       phaseID,
		SessionID:   sessionID,
		MessageUUID: uuid.NewString(),
		Type:        "user",
		Role:        "user",
		Content:     prompt,
		Timestamp:   now,
	}
	if err := we.backend.AddTranscript(userTranscript); err != nil {
		we.logger.Warn("failed to store user transcript", "task", taskID, "phase", phaseID, "error", err)
	}

	// Store assistant response
	assistantTranscript := &storage.Transcript{
		TaskID:              taskID,
		Phase:               phaseID,
		SessionID:           sessionID,
		MessageUUID:         uuid.NewString(),
		Type:                "assistant",
		Role:                "assistant",
		Content:             result.Content,
		Model:               model,
		InputTokens:         result.Usage.InputTokens,
		OutputTokens:        result.Usage.OutputTokens,
		CacheCreationTokens: result.Usage.CacheCreationInputTokens,
		CacheReadTokens:     result.Usage.CacheReadInputTokens,
		Timestamp:           now + 1, // Ensure ordering
	}
	if err := we.backend.AddTranscript(assistantTranscript); err != nil {
		we.logger.Warn("failed to store assistant transcript", "task", taskID, "phase", phaseID, "error", err)
	}
}

// runQualityChecks runs quality checks configured for the phase.
// Returns nil if no checks are configured.
func (we *WorkflowExecutor) runQualityChecks(ctx context.Context, cfg PhaseExecutionConfig) *QualityCheckResult {
	// Load quality checks from phase template (with workflow override)
	checks, err := LoadQualityChecksForPhase(cfg.PhaseTemplate, cfg.WorkflowPhase)
	if err != nil {
		we.logger.Warn("failed to load quality checks - continuing without checks",
			"phase", cfg.PhaseID,
			"error", err,
			"note", "check phase_templates.quality_checks JSON format",
		)
		return nil
	}

	if len(checks) == 0 {
		// No checks configured for this phase
		we.logger.Debug("no quality checks configured for phase", "phase", cfg.PhaseID)
		return nil
	}

	we.logger.Info("running quality checks", "phase", cfg.PhaseID, "check_count", len(checks))

	// Load project commands from database
	commands, err := we.projectDB.GetProjectCommandsMap()
	if err != nil {
		we.logger.Warn("failed to load project commands - code checks may not run",
			"phase", cfg.PhaseID,
			"error", err,
			"hint", "run 'orc config commands' to view/configure",
		)
		// Continue with empty commands - custom checks may still work
		commands = make(map[string]*db.ProjectCommand)
	}

	// Create and run the quality check runner
	runner := NewQualityCheckRunner(
		cfg.WorkingDir,
		checks,
		commands,
		we.logger,
	)

	return runner.Run(ctx)
}
