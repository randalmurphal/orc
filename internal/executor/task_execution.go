// Package executor provides the flowgraph-based execution engine for orc.
// This file contains task execution methods for the Executor type.
package executor

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// ExecuteTask runs all phases of a task with gate evaluation and cross-phase retry.
func (e *Executor) ExecuteTask(ctx context.Context, t *task.Task, p *plan.Plan, s *state.State) error {
	// Set current task directory for saving files
	e.currentTaskDir = e.taskDir(t.ID)

	// Check spec requirements for non-trivial tasks
	if err := e.checkSpecRequirements(t); err != nil {
		return err
	}

	// Record execution info for orphan detection
	hostname, _ := os.Hostname()
	s.StartExecution(os.Getpid(), hostname)

	// Update task status and initial phase atomically
	// Setting CurrentPhase before saving ensures the UI shows the task
	// in the correct column (e.g., "implement") rather than "queued"
	t.Status = task.StatusRunning
	now := time.Now()
	t.StartedAt = &now
	if len(p.Phases) > 0 {
		t.CurrentPhase = p.Phases[0].ID
	}
	if err := t.SaveTo(e.currentTaskDir); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	// Save initial state with execution info
	if err := s.SaveTo(e.currentTaskDir); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	// Setup worktree if enabled
	if e.orcConfig.Worktree.Enabled && e.gitOps != nil {
		if err := e.setupWorktreeForTask(t); err != nil {
			e.failSetup(t, s, err)
			return err
		}
		// Cleanup worktree on exit based on config and success
		defer e.cleanupWorktreeForTask(t)
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

		// Start phase and update heartbeat
		s.StartPhase(phase.ID)
		s.UpdateHeartbeat()
		if err := s.SaveTo(e.currentTaskDir); err != nil {
			return fmt.Errorf("save state: %w", err)
		}

		// Update task's current phase for status display
		t.CurrentPhase = phase.ID
		if err := t.SaveTo(e.currentTaskDir); err != nil {
			return fmt.Errorf("save task: %w", err)
		}

		e.logger.Info("executing phase", "phase", phase.ID, "task", t.ID)

		// Sync with target branch before phase if configured
		if err := e.syncBeforePhase(ctx, t, phase.ID); err != nil {
			// Sync failures are treated as phase failures for retry handling
			e.logger.Error("pre-phase sync failed", "phase", phase.ID, "error", err)
			s.FailPhase(phase.ID, err)
			if saveErr := s.SaveTo(e.currentTaskDir); saveErr != nil {
				e.logger.Error("failed to save state on sync failure", "error", saveErr)
			}
			return fmt.Errorf("pre-phase sync for %s: %w", phase.ID, err)
		}

		// Publish phase start event
		e.publishPhaseStart(t.ID, phase.ID)
		e.publishState(t.ID, s)

		// Execute phase
		result, err := e.ExecutePhase(ctx, t, phase, s)
		if err != nil {
			// Check for context cancellation (interrupt)
			if ctx.Err() != nil {
				s.InterruptPhase(phase.ID)
				if saveErr := s.SaveTo(e.currentTaskDir); saveErr != nil {
					e.logger.Error("failed to save state on interrupt", "error", saveErr)
				}
				return ctx.Err()
			}

			// Handle phase failure with potential retry
			shouldRetry, retryIdx := e.handlePhaseFailure(phase.ID, err, result, p, s, retryCounts, i)
			if shouldRetry {
				i = retryIdx
				continue
			}

			// No retry available, fail the task
			e.failTask(t, phase, s, err)
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

		// Post-phase knowledge extraction for docs phase (fallback mechanism)
		if phase.ID == "docs" {
			e.tryKnowledgeExtraction(t.ID)
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
		e.publishTokens(t.ID, phase.ID, result.InputTokens, result.OutputTokens, 0, 0, result.InputTokens+result.OutputTokens)
		e.publishState(t.ID, s)

		// Evaluate gate if present (gate.Type != "" means gate is configured)
		if phase.Gate.Type != "" {
			shouldRetry, retryIdx := e.handleGateEvaluation(ctx, phase, result, t, p, s, retryCounts, i)
			if shouldRetry {
				i = retryIdx
				continue
			}
		}

		i++ // Move to next phase
	}

	// Complete task
	return e.completeTask(ctx, t, s)
}

// setupWorktreeForTask creates or reuses an isolated worktree for the task.
func (e *Executor) setupWorktreeForTask(t *task.Task) error {
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
		// Disable go.work to avoid "directory prefix does not contain modules listed in go.work"
		// error when running go commands in worktrees. The parent repo's go.work has relative
		// paths that don't work from the worktree location.
		claude.WithEnvVar("GOWORK", "off"),
	}
	// Resolve Claude path to absolute to ensure it works with worktree cmd.Dir
	claudePath := resolveClaudePath(e.config.ClaudePath)
	if claudePath != "" {
		worktreeClientOpts = append(worktreeClientOpts, claude.WithClaudePath(claudePath))
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
	// Inject token from pool if configured
	if e.tokenPool != nil {
		if token := e.tokenPool.Token(); token != "" {
			worktreeClientOpts = append(worktreeClientOpts, claude.WithEnvVar("CLAUDE_CODE_OAUTH_TOKEN", token))
		}
	}
	e.client = claude.NewClaudeCLI(worktreeClientOpts...)
	e.logger.Info("claude client configured for worktree", "path", worktreePath)

	// Create new session manager for worktree context
	e.sessionMgr = session.NewManager(
		session.WithDefaultSessionOptions(
			session.WithModel(e.config.Model),
			session.WithWorkdir(worktreePath),
			session.WithClaudePath(claudePath),
			session.WithPermissions(e.config.DangerouslySkipPermissions),
			// Disable go.work in sessions (same reason as above)
			session.WithEnv(map[string]string{"GOWORK": "off"}),
		),
	)

	// Reset phase executors to use new worktree context
	e.resetPhaseExecutors()

	return nil
}

// cleanupWorktreeForTask removes the worktree based on config and task status.
func (e *Executor) cleanupWorktreeForTask(t *task.Task) {
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
}

// handlePhaseFailure handles a phase execution failure, potentially setting up a retry.
// Returns (shouldRetry, retryIndex) where retryIndex is the phase index to jump to.
func (e *Executor) handlePhaseFailure(phaseID string, err error, result *Result, p *plan.Plan, s *state.State, retryCounts map[string]int, currentIdx int) (bool, int) {
	// Check if we should retry from an earlier phase
	retryFrom := e.orcConfig.ShouldRetryFrom(phaseID)
	if retryFrom != "" && retryCounts[phaseID] < e.orcConfig.EffectiveMaxRetries() {
		retryCounts[phaseID]++
		e.logger.Info("phase failed, retrying from earlier phase",
			"failed_phase", phaseID,
			"retry_from", retryFrom,
			"attempt", retryCounts[phaseID],
		)

		// Save retry context with failure details
		failureOutput := result.Output
		if failureOutput == "" && err != nil {
			failureOutput = err.Error()
		}
		reason := fmt.Sprintf("Phase %s failed: %v", phaseID, err)
		s.SetRetryContext(phaseID, retryFrom, reason, failureOutput, retryCounts[phaseID])

		// Save detailed context to file
		contextFile, saveErr := SaveRetryContextFile(e.config.WorkDir, "", phaseID, retryFrom, reason, failureOutput, retryCounts[phaseID])
		if saveErr != nil {
			e.logger.Warn("failed to save retry context file", "error", saveErr)
		} else {
			s.SetRetryContextFile(contextFile)
		}

		// Find the retry phase index and reset phases from there
		for j, ph := range p.Phases {
			if ph.ID == retryFrom {
				// Reset phases from retry point onwards
				for k := j; k <= currentIdx; k++ {
					s.ResetPhase(p.Phases[k].ID)
				}
				if saveErr := s.SaveTo(e.currentTaskDir); saveErr != nil {
					e.logger.Error("failed to save state on retry", "error", saveErr)
				}
				return true, j
			}
		}
	}

	return false, 0
}

// failSetup handles marking a task as failed during setup (before any phase runs).
// This is called when setup operations like worktree creation fail.
func (e *Executor) failSetup(t *task.Task, s *state.State, err error) {
	e.logger.Error("task setup failed", "task", t.ID, "error", err)

	// Clear execution tracking and set error
	s.ClearExecution()
	s.Error = err.Error()
	if saveErr := s.SaveTo(e.currentTaskDir); saveErr != nil {
		e.logger.Error("failed to save state on setup failure", "error", saveErr)
	}

	// Update task status
	t.Status = task.StatusFailed
	if saveErr := t.SaveTo(e.currentTaskDir); saveErr != nil {
		e.logger.Error("failed to save task on setup failure", "error", saveErr)
	}

	// Publish failure events - use "setup" as the phase identifier
	e.publishError(t.ID, "setup", err.Error(), true)
	e.publishState(t.ID, s)
}

// failTask handles marking a task as failed.
func (e *Executor) failTask(t *task.Task, phase *plan.Phase, s *state.State, err error) {
	s.FailPhase(phase.ID, err)
	s.ClearExecution() // Clear execution tracking on failure
	if saveErr := s.SaveTo(e.currentTaskDir); saveErr != nil {
		e.logger.Error("failed to save state on failure", "error", saveErr)
	}
	t.Status = task.StatusFailed
	if saveErr := t.SaveTo(e.currentTaskDir); saveErr != nil {
		e.logger.Error("failed to save task on failure", "error", saveErr)
	}

	// Publish failure events
	e.publishPhaseFailed(t.ID, phase.ID, err)
	e.publishError(t.ID, phase.ID, err.Error(), true)
	e.publishState(t.ID, s)
}

// handleGateEvaluation evaluates a phase gate and handles potential retry.
// Returns (shouldRetry, retryIndex) where retryIndex is the phase index to jump to.
func (e *Executor) handleGateEvaluation(ctx context.Context, phase *plan.Phase, result *Result, t *task.Task, p *plan.Plan, s *state.State, retryCounts map[string]int, currentIdx int) (bool, int) {
	decision, gateErr := e.evaluateGate(ctx, phase, result.Output, string(t.Weight))
	if gateErr != nil {
		e.logger.Warn("gate evaluation failed", "error", gateErr)
		// Continue on gate error - don't block automation
		return false, 0
	}

	if !decision.Approved {
		// Gate rejected - check if we should retry
		retryFrom := e.orcConfig.ShouldRetryFrom(phase.ID)
		if retryFrom != "" && retryCounts[phase.ID] < e.orcConfig.EffectiveMaxRetries() {
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
			contextFile, saveErr := SaveRetryContextFile(e.config.WorkDir, t.ID, phase.ID, retryFrom, reason, result.Output, retryCounts[phase.ID])
			if saveErr != nil {
				e.logger.Warn("failed to save retry context file", "error", saveErr)
			} else {
				s.SetRetryContextFile(contextFile)
			}

			// Find and reset to retry phase
			for j, ph := range p.Phases {
				if ph.ID == retryFrom {
					for k := j; k <= currentIdx; k++ {
						s.ResetPhase(p.Phases[k].ID)
					}
					if saveErr := s.SaveTo(e.currentTaskDir); saveErr != nil {
						e.logger.Error("failed to save state after retry reset", "error", saveErr)
					}
					return true, j
				}
			}
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

	return false, 0
}

// completeTask finalizes the task after all phases are done.
func (e *Executor) completeTask(ctx context.Context, t *task.Task, s *state.State) error {
	s.Complete()
	s.ClearExecution() // Clear execution tracking on completion
	if saveErr := s.SaveTo(e.currentTaskDir); saveErr != nil {
		e.logger.Error("failed to save state on completion", "error", saveErr)
	}

	t.Status = task.StatusCompleted
	completedAt := time.Now()
	t.CompletedAt = &completedAt
	if saveErr := t.SaveTo(e.currentTaskDir); saveErr != nil {
		e.logger.Error("failed to save task on completion", "error", saveErr)
	}

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

// checkSpecRequirements checks if a task has a valid spec for non-trivial weights.
// Returns an error if spec is required but missing or invalid.
func (e *Executor) checkSpecRequirements(t *task.Task) error {
	// Trivial tasks don't require specs
	if t.Weight == task.WeightTrivial {
		return nil
	}

	// Check if spec validation is enabled in config
	if e.orcConfig.Plan.RequireSpecForExecution {
		// Check if this weight should skip validation
		for _, skipWeight := range e.orcConfig.Plan.SkipValidationWeights {
			if string(t.Weight) == skipWeight {
				return nil
			}
		}

		// Check if spec exists and is valid
		if !task.SpecExists(t.ID) {
			e.logger.Warn("task has no spec", "task", t.ID, "weight", t.Weight)
			return fmt.Errorf("task %s requires a spec for weight '%s' - run 'orc plan %s' to create one", t.ID, t.Weight, t.ID)
		}

		// Validate spec content
		if !task.HasValidSpec(t.ID, t.Weight) {
			e.logger.Warn("task spec is invalid", "task", t.ID, "weight", t.Weight)
			return fmt.Errorf("task %s has an incomplete spec - run 'orc plan %s' to update it", t.ID, t.ID)
		}
	} else if e.orcConfig.Plan.WarnOnMissingSpec {
		// Just warn, don't block
		if !task.SpecExists(t.ID) {
			e.logger.Warn("task has no spec (execution will continue)",
				"task", t.ID,
				"weight", t.Weight,
				"hint", "run 'orc plan "+t.ID+"' to create a spec",
			)
		}
	}

	return nil
}
