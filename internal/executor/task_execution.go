// Package executor provides the flowgraph-based execution engine for orc.
// This file contains task execution methods for the Executor type.
package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/llmkit/claude/session"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ErrTaskBlocked is returned when a task completes all phases but is blocked
// due to external issues (e.g., sync conflicts with target branch).
// The task execution succeeded, but the completion action failed.
// Callers should display a blocked message, not a completion message.
var ErrTaskBlocked = errors.New("task blocked")

// ExecuteTask runs all phases of a task with gate evaluation and cross-phase retry.
func (e *Executor) ExecuteTask(ctx context.Context, t *task.Task, p *plan.Plan, s *state.State) error {
	// Set current task directory for saving files
	e.currentTaskDir = e.taskDir(t.ID)

	// Take process snapshot before task execution (for orphan detection)
	if e.resourceTracker != nil {
		if err := e.resourceTracker.SnapshotBefore(); err != nil {
			e.logger.Warn("failed to take resource snapshot before task", "error", err)
		}
		// Schedule after-snapshot and analysis on task completion (success or failure)
		defer e.runResourceAnalysis()
	}

	// Check spec requirements for non-trivial tasks
	// Skip if first phase is "spec" - the spec phase will create it
	if err := e.checkSpecRequirements(t, p); err != nil {
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
	if err := e.backend.SaveTask(t); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	// Save initial state with execution info
	if err := e.backend.SaveState(s); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	// Start heartbeat goroutine to keep orphan detection happy during long-running phases.
	// This prevents false positives where a task with a live PID is marked orphaned
	// due to stale heartbeat. While our updated CheckOrphaned logic prioritizes PID
	// over heartbeat, keeping heartbeats fresh is belt-and-suspenders for scenarios
	// where PID checks are unreliable (e.g., future cross-machine coordination).
	heartbeat := NewHeartbeatRunner(e.backend, s, e.logger)
	heartbeat.Start(ctx)
	defer heartbeat.Stop()

	// Setup worktree if enabled
	if e.orcConfig.Worktree.Enabled && e.gitOps != nil {
		if err := e.setupWorktreeForTask(t); err != nil {
			e.failSetup(t, s, err)
			return err
		}
		// Cleanup worktree on exit based on config and success
		defer e.cleanupWorktreeForTask(t)
	}

	// Sync with target branch before execution starts (catches stale worktrees)
	// This fixes race conditions when parallel tasks modify the same files:
	// - Task A and Task B start from same main commit
	// - Task A completes and merges first
	// - Task B's worktree is now stale - sync brings in Task A's changes
	// - Implement phase can now incorporate those changes
	if e.orcConfig.ShouldSyncOnStart() && e.orcConfig.ShouldSyncForWeight(string(t.Weight)) {
		if err := e.syncOnTaskStart(ctx, t); err != nil {
			// Sync failures are treated as setup failures
			e.logger.Error("sync-on-start failed", "task", t.ID, "error", err)
			e.failSetup(t, s, err)
			return err
		}
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
		if err := e.backend.SaveState(s); err != nil {
			return fmt.Errorf("save state: %w", err)
		}

		// Update task's current phase for status display
		t.CurrentPhase = phase.ID
		if err := e.backend.SaveTask(t); err != nil {
			return fmt.Errorf("save task: %w", err)
		}

		e.logger.Info("executing phase", "phase", phase.ID, "task", t.ID)

		// Sync with target branch before phase if configured
		if err := e.syncBeforePhase(ctx, t, phase.ID); err != nil {
			// Sync failures are treated as phase failures for retry handling
			e.logger.Error("pre-phase sync failed", "phase", phase.ID, "error", err)
			s.FailPhase(phase.ID, err)
			if saveErr := e.backend.SaveState(s); saveErr != nil {
				e.logger.Error("failed to save state on sync failure", "error", saveErr)
			}
			return fmt.Errorf("pre-phase sync for %s: %w", phase.ID, err)
		}

		// Publish phase start event
		e.publishPhaseStart(t.ID, phase.ID)
		e.publishState(t.ID, s)

		// Execute phase with PhaseMax timeout if configured
		// PhaseMax=0 means unlimited (no timeout)
		result, err := e.executePhaseWithTimeout(ctx, t, phase, s)
		if err != nil {
			// Check for context errors - distinguish between phase timeout and parent interrupt
			if ctx.Err() != nil {
				e.interruptTask(t, phase.ID, s, ctx.Err())
				return ctx.Err()
			}

			// Check if it's a phase timeout error (marked by our wrapper)
			if isPhaseTimeoutError(err) {
				e.interruptTask(t, phase.ID, s, err)
				return err
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

		// Save spec content to database for spec phase BEFORE marking complete.
		// This ensures we fail-fast if spec phase produces invalid output.
		// Pass worktree path so we can check for spec files if agent didn't use artifact tags.
		if phase.ID == "spec" {
			requiresSpec := t.Weight != task.WeightTrivial && t.Weight != task.WeightSmall

			// Check for empty output first
			if result.Output == "" {
				if requiresSpec {
					e.logger.Error("spec phase produced no output",
						"task", t.ID,
						"weight", t.Weight,
					)
					specErr := fmt.Errorf("spec phase failed: no output produced")
					e.failTask(t, phase, s, specErr)
					return specErr
				}
				e.logger.Warn("spec phase produced no output, continuing (trivial/small weight)")
			} else {
				saved, err := SaveSpecToDatabase(e.backend, t.ID, phase.ID, result.Output, e.worktreePath)
				if err != nil {
					// Check if it's a spec extraction error with details
					if specErr, ok := err.(*SpecExtractionError); ok {
						if requiresSpec {
							e.logger.Error("spec extraction failed",
								"task", t.ID,
								"reason", specErr.Reason,
								"output_len", specErr.OutputLen,
								"spec_path", specErr.SpecPath,
								"file_exists", specErr.FileExists,
								"file_read_err", specErr.FileReadErr,
								"hint", "Agent must output spec in <artifact> tags OR write to spec.md file",
							)
							extractionErr := fmt.Errorf("spec phase failed: %s", specErr.Reason)
							e.failTask(t, phase, s, extractionErr)
							return extractionErr
						}
						e.logger.Warn("spec extraction failed (non-critical)", "reason", specErr.Reason)
					} else {
						// Other errors (database, etc.)
						if requiresSpec {
							e.logger.Error("failed to save spec to database",
								"task", t.ID,
								"error", err,
							)
							dbErr := fmt.Errorf("spec phase failed: %w", err)
							e.failTask(t, phase, s, dbErr)
							return dbErr
						}
						e.logger.Warn("failed to save spec to database", "error", err)
					}
				} else if saved {
					e.logger.Info("saved spec to database", "task", t.ID)
				}
			}
		}

		// Extract and save review findings for review phase.
		// This enables multi-round review by persisting findings between rounds.
		if phase.ID == "review" && result.Output != "" {
			// Determine review round - same logic as template context loading.
			// Round 1: first time review runs (phase not yet completed before).
			// Round 2: review phase was previously completed.
			reviewRound := 1
			if s.Phases != nil {
				if ps, ok := s.Phases["review"]; ok && ps.Status == "completed" {
					reviewRound = 2
				}
			}
			e.tryExtractReviewFindings(ctx, t.ID, result.Output, reviewRound)
		}

		// Extract and save QA results for qa phase.
		// This enables QA result persistence for reporting and dashboard display.
		if phase.ID == "qa" && result.Output != "" {
			e.tryExtractQAResult(ctx, t.ID, result.Output)
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
		if err := e.backend.SaveState(s); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
		if err := e.backend.SavePlan(p, t.ID); err != nil {
			return fmt.Errorf("save plan: %w", err)
		}

		// Publish phase completion events
		e.publishPhaseComplete(t.ID, phase.ID, result.CommitSHA)
		e.publishTokens(t.ID, phase.ID, result.InputTokens, result.OutputTokens, 0, 0, result.InputTokens+result.OutputTokens)
		e.publishState(t.ID, s)

		// Trigger automation event for phase completion
		e.triggerAutomationEvent(ctx, automation.EventPhaseCompleted, t, phase.ID)

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
// Uses the full 5-level branch resolution hierarchy to determine the target branch,
// and auto-creates initiative/staging branches if they don't exist.
func (e *Executor) setupWorktreeForTask(t *task.Task) error {
	result, err := SetupWorktreeForTask(t, e.orcConfig, e.gitOps, e.backend)
	if err != nil {
		return fmt.Errorf("setup worktree: %w", err)
	}

	e.worktreePath = result.Path
	e.worktreeGit = e.gitOps.InWorktree(result.Path)

	logMsg := "created worktree"
	if result.Reused {
		logMsg = "reusing existing worktree"
	}
	e.logger.Info(logMsg, "task", t.ID, "path", result.Path, "target_branch", result.TargetBranch)

	// Generate per-worktree MCP config for isolated Playwright sessions
	if ShouldGenerateMCPConfig(t, e.orcConfig) {
		if err := GenerateWorktreeMCPConfig(result.Path, t.ID, t, e.orcConfig); err != nil {
			e.logger.Warn("failed to generate MCP config", "task", t.ID, "error", err)
			// Non-fatal: continue without MCP config
		} else {
			e.logger.Info("generated MCP config", "task", t.ID, "path", result.Path+"/.mcp.json")
		}
	}

	// Create a new Claude client for the worktree context
	// This ensures all Claude work happens in the isolated worktree
	worktreeClientOpts := []claude.ClaudeOption{
		claude.WithModel(e.config.Model),
		claude.WithWorkdir(result.Path),
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
	e.logger.Info("claude client configured for worktree", "path", result.Path)

	// Create new session manager for worktree context
	// Include "user" setting source to load agents from ~/.claude/agents/
	e.sessionMgr = session.NewManager(
		session.WithDefaultSessionOptions(
			session.WithModel(e.config.Model),
			session.WithWorkdir(result.Path),
			session.WithClaudePath(claudePath),
			session.WithPermissions(e.config.DangerouslySkipPermissions),
			// Disable go.work in sessions (same reason as above)
			session.WithEnv(map[string]string{"GOWORK": "off"}),
			session.WithSettingSources([]string{"project", "local", "user"}),
		),
	)

	// Reset phase executors to use new worktree context
	e.resetPhaseExecutors()

	return nil
}

// cleanupWorktreeForTask removes the worktree based on config and task status.
func (e *Executor) cleanupWorktreeForTask(t *task.Task) {
	if e.worktreePath == "" {
		return
	}

	shouldCleanup := (t.Status == task.StatusCompleted && e.orcConfig.Worktree.CleanupOnComplete) ||
		(t.Status == task.StatusFailed && e.orcConfig.Worktree.CleanupOnFail)
	if !shouldCleanup {
		return
	}

	// Cleanup Playwright user data directory (task-specific browser profile)
	if err := CleanupPlaywrightUserData(t.ID); err != nil {
		e.logger.Warn("failed to cleanup playwright user data", "task", t.ID, "error", err)
	}

	// Use stored worktree path directly instead of reconstructing from task ID.
	// This handles initiative-prefixed worktrees correctly (e.g., feature-auth-TASK-001
	// instead of orc-TASK-001).
	if err := e.gitOps.CleanupWorktreeAtPath(e.worktreePath); err != nil {
		e.logger.Warn("failed to cleanup worktree", "path", e.worktreePath, "error", err)
	} else {
		e.logger.Info("cleaned up worktree", "task", t.ID, "path", e.worktreePath)
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
				if saveErr := e.backend.SaveState(s); saveErr != nil {
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
	if saveErr := e.backend.SaveState(s); saveErr != nil {
		e.logger.Error("failed to save state on setup failure", "error", saveErr)
	}

	// Update task status
	t.Status = task.StatusFailed
	if saveErr := e.backend.SaveTask(t); saveErr != nil {
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
	if saveErr := e.backend.SaveState(s); saveErr != nil {
		e.logger.Error("failed to save state on failure", "error", saveErr)
	}
	t.Status = task.StatusFailed
	if saveErr := e.backend.SaveTask(t); saveErr != nil {
		e.logger.Error("failed to save task on failure", "error", saveErr)
	}

	// Publish failure events
	e.publishPhaseFailed(t.ID, phase.ID, err)
	e.publishError(t.ID, phase.ID, err.Error(), true)
	e.publishState(t.ID, s)

	// Trigger automation event for task failure
	e.triggerAutomationEvent(context.Background(), automation.EventTaskFailed, t, phase.ID)
}

// interruptTask handles marking a task as interrupted/paused.
// This ensures both task status and state are properly updated when execution is cancelled,
// preventing orphaned tasks that show "running" but have no active executor.
func (e *Executor) interruptTask(t *task.Task, phaseID string, s *state.State, err error) {
	e.logger.Info("task interrupted", "task", t.ID, "phase", phaseID, "reason", err.Error())

	// Update state: mark phase as interrupted and store error
	s.InterruptPhase(phaseID)
	s.Error = fmt.Sprintf("interrupted during %s: %s", phaseID, err.Error())
	if saveErr := e.backend.SaveState(s); saveErr != nil {
		e.logger.Error("failed to save state on interrupt", "error", saveErr)
	}

	// Update task status to paused (not running, not failed - can be resumed)
	t.Status = task.StatusPaused
	if saveErr := e.backend.SaveTask(t); saveErr != nil {
		e.logger.Error("failed to save task on interrupt", "error", saveErr)
	}

	// Publish events so UI is updated
	e.publishError(t.ID, phaseID, err.Error(), false) // Not fatal - can be resumed
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
					if saveErr := e.backend.SaveState(s); saveErr != nil {
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
// Completion flow:
// 1. Try completion actions (sync, PR/merge) FIRST
// 2. If sync fails with conflicts, try auto/Claude resolution
// 3. If resolution fails, set status to blocked (not completed)
// 4. Only mark completed if everything succeeds
func (e *Executor) completeTask(ctx context.Context, t *task.Task, s *state.State) error {
	// Run completion action (sync, PR) FIRST - before marking complete
	completionErr := e.runCompletion(ctx, t)

	if completionErr != nil {
		// Check if it's a conflict error that we couldn't resolve
		if errors.Is(completionErr, ErrSyncConflict) {
			e.logger.Error("completion failed due to unresolved conflicts",
				"task", t.ID,
				"error", completionErr)

			// Mark task as blocked, not completed
			t.Status = task.StatusBlocked
			if t.Metadata == nil {
				t.Metadata = make(map[string]string)
			}
			t.Metadata["blocked_reason"] = "sync_conflict"
			t.Metadata["blocked_error"] = completionErr.Error()

			s.ClearExecution()
			if saveErr := e.backend.SaveState(s); saveErr != nil {
				e.logger.Error("failed to save state on conflict block", "error", saveErr)
			}
			if saveErr := e.backend.SaveTask(t); saveErr != nil {
				e.logger.Error("failed to save task on conflict block", "error", saveErr)
			}

			// Publish blocked event
			e.publish(events.NewEvent(events.EventComplete, t.ID, events.CompleteData{
				Status: "blocked",
			}))
			e.publishState(t.ID, s)

			// Return ErrTaskBlocked so CLI can display the correct message.
			// This is NOT a fatal error - the task phases completed successfully,
			// but the post-completion sync failed. CLI should show a blocked message
			// instead of a completion celebration.
			return fmt.Errorf("%w: sync conflict - resolve conflicts then run 'orc resume %s'", ErrTaskBlocked, t.ID)
		}

		// Check if it's a merge failure (e.g., from race condition with parallel tasks)
		if errors.Is(completionErr, ErrMergeFailed) {
			e.logger.Error("completion failed due to merge failure",
				"task", t.ID,
				"error", completionErr)

			// Mark task as blocked, not completed
			t.Status = task.StatusBlocked
			if t.Metadata == nil {
				t.Metadata = make(map[string]string)
			}
			t.Metadata["blocked_reason"] = "merge_failed"
			t.Metadata["blocked_error"] = completionErr.Error()

			s.ClearExecution()
			if saveErr := e.backend.SaveState(s); saveErr != nil {
				e.logger.Error("failed to save state on merge failure block", "error", saveErr)
			}
			if saveErr := e.backend.SaveTask(t); saveErr != nil {
				e.logger.Error("failed to save task on merge failure block", "error", saveErr)
			}

			// Publish blocked event
			e.publish(events.NewEvent(events.EventComplete, t.ID, events.CompleteData{
				Status: "blocked",
			}))
			e.publishState(t.ID, s)

			// Return ErrTaskBlocked so CLI can display the correct message.
			// The PR was created but merge failed after retries.
			return fmt.Errorf("%w: merge failed - run 'orc resume %s' after resolving", ErrTaskBlocked, t.ID)
		}

		// Other completion errors (non-conflict, non-merge) - log warning but continue to complete
		e.logger.Warn("completion action failed", "error", completionErr)
	}

	// Completion succeeded (or had non-blocking errors) - mark as completed
	s.Complete()
	s.ClearExecution()
	if saveErr := e.backend.SaveState(s); saveErr != nil {
		e.logger.Error("failed to save state on completion", "error", saveErr)
	}

	t.Status = task.StatusCompleted
	completedAt := time.Now()
	t.CompletedAt = &completedAt
	if saveErr := e.backend.SaveTask(t); saveErr != nil {
		e.logger.Error("failed to save task on completion", "error", saveErr)
	}

	// Publish completion event
	e.publish(events.NewEvent(events.EventComplete, t.ID, events.CompleteData{
		Status: "completed",
	}))
	e.publishState(t.ID, s)

	// Trigger automation events
	e.triggerAutomationEvent(ctx, automation.EventTaskCompleted, t, "")

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
// Skips check if the plan's first phase is "spec" (the spec will be created during execution).
func (e *Executor) checkSpecRequirements(t *task.Task, p *plan.Plan) error {
	// Trivial tasks don't require specs
	if t.Weight == task.WeightTrivial {
		return nil
	}

	// Skip if plan starts with spec phase - it will create the spec
	if p != nil && len(p.Phases) > 0 && p.Phases[0].ID == "spec" {
		e.logger.Debug("skipping spec requirement check - plan starts with spec phase",
			"task", t.ID)
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

		// Check if spec exists using backend
		specExists, err := e.backend.SpecExists(t.ID)
		if err != nil {
			e.logger.Warn("failed to check spec existence", "task", t.ID, "error", err)
			specExists = false
		}
		if !specExists {
			e.logger.Warn("task has no spec", "task", t.ID, "weight", t.Weight)
			return fmt.Errorf("task %s requires a spec for weight '%s' - run 'orc plan %s' to create one", t.ID, t.Weight, t.ID)
		}

		// Load spec content to validate
		specContent, err := e.backend.LoadSpec(t.ID)
		if err != nil || specContent == "" {
			e.logger.Warn("task spec is invalid", "task", t.ID, "weight", t.Weight)
			return fmt.Errorf("task %s has an incomplete spec - run 'orc plan %s' to update it", t.ID, t.ID)
		}

		// Haiku validation for spec quality (if enabled)
		if e.haikuClient != nil && e.orcConfig.ShouldValidateSpec(string(t.Weight)) {
			ctx := context.Background()
			ready, suggestions, valErr := ValidateTaskReadiness(ctx, e.haikuClient, t.Description, specContent, string(t.Weight))
			if valErr != nil {
				if e.orcConfig.Validation.FailOnAPIError {
					// Fail properly - task is resumable from spec phase
					e.logger.Error("spec validation API error - failing task",
						"task", t.ID,
						"error", valErr,
						"hint", "Task can be resumed with 'orc resume'",
					)
					return fmt.Errorf("spec validation API error (resumable): %w", valErr)
				}
				// Fail open (legacy behavior for fast profile)
				e.logger.Warn("haiku spec validation error (continuing)",
					"task", t.ID,
					"error", valErr,
				)
			} else if !ready && len(suggestions) > 0 {
				// Block execution on poor spec quality
				e.logger.Error("spec quality validation failed - blocking execution",
					"task", t.ID,
					"suggestions", suggestions,
				)
				suggestionText := ""
				for i, s := range suggestions {
					suggestionText += fmt.Sprintf("\n  %d. %s", i+1, s)
				}
				return fmt.Errorf("task %s spec quality is insufficient for execution:%s\n\nRun 'orc plan %s' to improve the spec", t.ID, suggestionText, t.ID)
			}
		}
	} else if e.orcConfig.Plan.WarnOnMissingSpec {
		// Only warn for weights that semantically require specs (large, greenfield)
		// Trivial/small/medium tasks don't benefit from spec warnings - they're simple enough
		// to implement directly without upfront planning
		requiresSpec := t.Weight == task.WeightLarge || t.Weight == task.WeightGreenfield

		// Just warn, don't block
		specExists, _ := e.backend.SpecExists(t.ID)
		if requiresSpec && !specExists {
			e.logger.Warn("task has no spec (execution will continue)",
				"task", t.ID,
				"weight", t.Weight,
				"hint", "run 'orc plan "+t.ID+"' to create a spec",
			)
		}
	}

	return nil
}

// FinalizeTask executes only the finalize phase for a task.
// This is used when manually triggering finalize via CLI.
func (e *Executor) FinalizeTask(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) error {
	// Set current task directory for worktree operations
	e.currentTaskDir = e.taskDir(t.ID)

	// Record execution info for orphan detection
	hostname, _ := os.Hostname()
	s.StartExecution(os.Getpid(), hostname)

	// Update task status
	originalStatus := t.Status
	t.Status = task.StatusRunning
	now := time.Now()
	if t.StartedAt == nil {
		t.StartedAt = &now
	}
	t.CurrentPhase = "finalize"
	if err := e.backend.SaveTask(t); err != nil {
		return fmt.Errorf("save task: %w", err)
	}

	// Save initial state with execution info
	if err := e.backend.SaveState(s); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	// Start heartbeat goroutine (same as ExecuteTask)
	heartbeat := NewHeartbeatRunner(e.backend, s, e.logger)
	heartbeat.Start(ctx)
	defer heartbeat.Stop()

	// Setup worktree if enabled
	if e.orcConfig.Worktree.Enabled && e.gitOps != nil {
		if err := e.setupWorktreeForTask(t); err != nil {
			e.failSetup(t, s, err)
			return err
		}
		// Cleanup worktree on exit based on config and success
		defer e.cleanupWorktreeForTask(t)
	}

	// Start phase and update heartbeat
	s.StartPhase("finalize")
	s.UpdateHeartbeat()
	if err := e.backend.SaveState(s); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	e.logger.Info("executing finalize phase", "task", t.ID)

	// Publish phase start event
	e.publishPhaseStart(t.ID, "finalize")
	e.publishState(t.ID, s)

	// Create finalize executor
	workingDir := e.config.WorkDir
	if e.worktreePath != "" {
		workingDir = e.worktreePath
	}

	gitSvc := e.gitOps
	if e.worktreeGit != nil {
		gitSvc = e.worktreeGit
	}

	finalizeExec := NewFinalizeExecutor(
		e.sessionMgr,
		WithFinalizeGitSvc(gitSvc),
		WithFinalizePublisher(e.publisher),
		WithFinalizeLogger(e.logger),
		WithFinalizeConfig(DefaultConfigForWeight(t.Weight)),
		WithFinalizeOrcConfig(e.orcConfig),
		WithFinalizeWorkingDir(workingDir),
		WithFinalizeTaskDir(e.currentTaskDir),
		WithFinalizeBackend(e.backend),
		WithFinalizeStateUpdater(func(st *state.State) {
			if saveErr := e.backend.SaveState(st); saveErr != nil {
				e.logger.Error("failed to save state during finalize", "error", saveErr)
			}
		}),
	)

	// Execute finalize phase with PhaseMax timeout if configured
	// PhaseMax=0 means unlimited (no timeout)
	phaseMax := e.orcConfig.Timeouts.PhaseMax
	finalizeCtx := ctx
	var finalizeCancel context.CancelFunc
	if phaseMax > 0 {
		finalizeCtx, finalizeCancel = context.WithTimeout(ctx, phaseMax)
		defer finalizeCancel()
	}

	result, err := finalizeExec.Execute(finalizeCtx, t, p, s)
	if err != nil {
		// Check for context errors - distinguish between phase timeout and parent interrupt
		if ctx.Err() != nil {
			e.interruptTask(t, "finalize", s, ctx.Err())
			return ctx.Err()
		}

		// Check if finalize context timed out (PhaseMax exceeded)
		if finalizeCtx.Err() == context.DeadlineExceeded {
			timeoutErr := &phaseTimeoutError{
				phase:   "finalize",
				timeout: phaseMax,
				err:     err,
			}
			e.logger.Warn("finalize phase timeout exceeded",
				"timeout", phaseMax,
				"task", t.ID,
			)
			e.interruptTask(t, "finalize", s, timeoutErr)
			return timeoutErr
		}

		// Fail the phase
		s.FailPhase("finalize", err)
		s.ClearExecution()
		if saveErr := e.backend.SaveState(s); saveErr != nil {
			e.logger.Error("failed to save state on failure", "error", saveErr)
		}

		// Restore original status on failure
		t.Status = originalStatus
		if saveErr := e.backend.SaveTask(t); saveErr != nil {
			e.logger.Error("failed to save task on failure", "error", saveErr)
		}

		// Publish failure events
		e.publishPhaseFailed(t.ID, "finalize", err)
		e.publishError(t.ID, "finalize", err.Error(), true)
		e.publishState(t.ID, s)

		return fmt.Errorf("finalize phase failed: %w", err)
	}

	// Complete phase
	s.CompletePhase("finalize", result.CommitSHA)
	p.Status = plan.PhaseCompleted
	p.CommitSHA = result.CommitSHA

	// Save state
	if err := e.backend.SaveState(s); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	// Save plan to persist the phase status
	existingPlan, loadErr := e.backend.LoadPlan(t.ID)
	if loadErr == nil {
		// Update finalize phase status in existing plan
		for i := range existingPlan.Phases {
			if existingPlan.Phases[i].ID == "finalize" {
				existingPlan.Phases[i].Status = plan.PhaseCompleted
				existingPlan.Phases[i].CommitSHA = result.CommitSHA
				break
			}
		}
		if saveErr := e.backend.SavePlan(existingPlan, t.ID); saveErr != nil {
			e.logger.Warn("failed to save plan", "error", saveErr)
		}
	}

	// If task was previously paused/blocked/failed, restore to that state
	// Only mark complete if ALL phases are done
	allPhasesComplete := true
	if existingPlan != nil {
		for _, phase := range existingPlan.Phases {
			if phase.Status != plan.PhaseCompleted && phase.Status != plan.PhaseSkipped {
				allPhasesComplete = false
				break
			}
		}
	}

	if allPhasesComplete {
		s.Complete()
		t.Status = task.StatusCompleted
		completedAt := time.Now()
		t.CompletedAt = &completedAt
	} else {
		t.Status = originalStatus
		if t.Status == task.StatusRunning {
			t.Status = task.StatusPaused // Don't leave in running state
		}
	}
	s.ClearExecution()

	if saveErr := e.backend.SaveState(s); saveErr != nil {
		e.logger.Error("failed to save state on completion", "error", saveErr)
	}
	if saveErr := e.backend.SaveTask(t); saveErr != nil {
		e.logger.Error("failed to save task on completion", "error", saveErr)
	}

	// Publish completion events
	e.publishPhaseComplete(t.ID, "finalize", result.CommitSHA)
	e.publishTokens(t.ID, "finalize", result.InputTokens, result.OutputTokens, 0, 0, result.InputTokens+result.OutputTokens)
	e.publishState(t.ID, s)

	// Push finalize changes and wait for CI, then merge
	if t.HasPR() && e.orcConfig.ShouldWaitForCI() {
		// Push any finalize changes first
		if gitSvc != nil {
			if pushErr := gitSvc.Push("origin", t.Branch, true); pushErr != nil {
				e.logger.Warn("failed to push finalize changes", "error", pushErr)
				// Continue anyway - changes might already be pushed
			}
		}

		// Wait for CI and merge
		ciMerger := NewCIMerger(
			e.orcConfig,
			WithCIMergerPublisher(e.publisher),
			WithCIMergerLogger(e.logger),
			WithCIMergerWorkDir(workingDir),
			WithCIMergerBackend(e.backend),
		)

		if mergeErr := ciMerger.WaitForCIAndMerge(ctx, t); mergeErr != nil {
			e.logger.Warn("CI wait and merge failed", "error", mergeErr)
			// Publish CI/merge error but don't fail the finalize phase itself
			e.publishError(t.ID, "ci_merge", mergeErr.Error(), false)
		} else {
			// Task already completed - just log the successful merge
			e.logger.Info("PR merged successfully via finalize", "task", t.ID)
			e.publishState(t.ID, s)
		}
	}

	return nil
}

// triggerAutomationEvent sends an event to the automation service if configured.
// This is used to trigger automation tasks based on task/phase completion events.
func (e *Executor) triggerAutomationEvent(ctx context.Context, eventType string, t *task.Task, phase string) {
	if e.automationSvc == nil {
		return
	}

	event := &automation.Event{
		Type:     eventType,
		TaskID:   t.ID,
		Weight:   string(t.Weight),
		Category: string(t.Category),
		Phase:    phase,
	}

	if err := e.automationSvc.HandleEvent(ctx, event); err != nil {
		e.logger.Warn("automation event handling failed",
			"event", eventType,
			"task", t.ID,
			"error", err)
	}
}

// tryExtractReviewFindings attempts to extract and save review findings from phase output.
// This is a best-effort operation - extraction failures are logged but don't fail the phase.
func (e *Executor) tryExtractReviewFindings(ctx context.Context, taskID, output string, round int) {
	// Use haiku client for extraction (same as validation)
	if e.haikuClient == nil {
		e.logger.Debug("skipping review findings extraction - no haiku client configured")
		return
	}

	// Extract review findings from the output
	findings, err := ExtractReviewFindings(ctx, e.haikuClient, output)
	if err != nil {
		e.logger.Warn("failed to extract review findings",
			"task", taskID,
			"round", round,
			"error", err,
		)
		return
	}

	// Convert executor.ReviewFindings to storage.ReviewFindings
	storageFindings := &storage.ReviewFindings{
		TaskID:      taskID,
		Round:       round,
		Summary:     findings.Summary,
		Issues:      make([]storage.ReviewFinding, len(findings.Issues)),
		Questions:   findings.Questions,
		Positives:   findings.Positives,
		Perspective: string(findings.Perspective),
	}
	for i, issue := range findings.Issues {
		storageFindings.Issues[i] = storage.ReviewFinding{
			Severity:    issue.Severity,
			File:        issue.File,
			Line:        issue.Line,
			Description: issue.Description,
			Suggestion:  issue.Suggestion,
			Perspective: string(issue.Perspective),
		}
	}

	// Save to database
	if err := e.backend.SaveReviewFindings(storageFindings); err != nil {
		e.logger.Warn("failed to save review findings",
			"task", taskID,
			"round", round,
			"error", err,
		)
		return
	}

	e.logger.Info("extracted and saved review findings",
		"task", taskID,
		"round", round,
		"issues", len(findings.Issues),
		"summary_length", len(findings.Summary),
	)
}

// tryExtractQAResult attempts to extract and save QA results from phase output.
// This is a best-effort operation - extraction failures are logged but don't fail the phase.
func (e *Executor) tryExtractQAResult(ctx context.Context, taskID, output string) {
	// Use haiku client for extraction (same as validation)
	if e.haikuClient == nil {
		e.logger.Debug("skipping QA result extraction - no haiku client configured")
		return
	}

	// Extract QA results from the output
	qaResult, err := ExtractQAResult(ctx, e.haikuClient, output)
	if err != nil {
		e.logger.Warn("failed to extract QA result",
			"task", taskID,
			"error", err,
		)
		return
	}

	// Convert executor.QAResult to storage.QAResult
	storageResult := &storage.QAResult{
		TaskID:         taskID,
		Status:         string(qaResult.Status),
		Summary:        qaResult.Summary,
		Recommendation: qaResult.Recommendation,
	}

	// Convert nested types
	for _, t := range qaResult.TestsWritten {
		storageResult.TestsWritten = append(storageResult.TestsWritten, storage.QATest{
			File:        t.File,
			Description: t.Description,
			Type:        t.Type,
		})
	}

	if qaResult.TestsRun != nil {
		storageResult.TestsRun = &storage.QATestRun{
			Total:   qaResult.TestsRun.Total,
			Passed:  qaResult.TestsRun.Passed,
			Failed:  qaResult.TestsRun.Failed,
			Skipped: qaResult.TestsRun.Skipped,
		}
	}

	if qaResult.Coverage != nil {
		storageResult.Coverage = &storage.QACoverage{
			Percentage:     qaResult.Coverage.Percentage,
			UncoveredAreas: qaResult.Coverage.UncoveredAreas,
		}
	}

	for _, doc := range qaResult.Documentation {
		storageResult.Documentation = append(storageResult.Documentation, storage.QADoc{
			File: doc.File,
			Type: doc.Type,
		})
	}

	for _, issue := range qaResult.Issues {
		storageResult.Issues = append(storageResult.Issues, storage.QAIssue{
			Severity:     issue.Severity,
			Description:  issue.Description,
			Reproduction: issue.Reproduction,
		})
	}

	// Save to database
	if err := e.backend.SaveQAResult(storageResult); err != nil {
		e.logger.Warn("failed to save QA result",
			"task", taskID,
			"error", err,
		)
		return
	}

	e.logger.Info("extracted and saved QA result",
		"task", taskID,
		"status", qaResult.Status,
		"tests_written", len(qaResult.TestsWritten),
		"issues", len(qaResult.Issues),
	)
}

// phaseTimeoutError wraps an error to indicate it was caused by PhaseMax timeout
type phaseTimeoutError struct {
	phase   string
	timeout time.Duration
	err     error
}

func (e *phaseTimeoutError) Error() string {
	return fmt.Sprintf("phase %s exceeded timeout (%v)", e.phase, e.timeout)
}

func (e *phaseTimeoutError) Unwrap() error {
	return e.err
}

// isPhaseTimeoutError returns true if the error is a phase timeout error
func isPhaseTimeoutError(err error) bool {
	var pte *phaseTimeoutError
	return errors.As(err, &pte)
}

// executePhaseWithTimeout wraps ExecutePhase with PhaseMax timeout if configured.
// PhaseMax=0 means unlimited (no timeout).
// Returns a phaseTimeoutError if the phase times out due to PhaseMax.
func (e *Executor) executePhaseWithTimeout(ctx context.Context, t *task.Task, phase *plan.Phase, s *state.State) (*Result, error) {
	phaseMax := e.orcConfig.Timeouts.PhaseMax
	if phaseMax <= 0 {
		// No timeout configured, execute directly
		return e.ExecutePhase(ctx, t, phase, s)
	}

	// Create timeout context for this phase
	phaseCtx, cancel := context.WithTimeout(ctx, phaseMax)
	defer cancel()

	result, err := e.ExecutePhase(phaseCtx, t, phase, s)
	if err != nil {
		// Check if phase context timed out (but parent context is still alive)
		if phaseCtx.Err() == context.DeadlineExceeded && ctx.Err() == nil {
			e.logger.Warn("phase timeout exceeded",
				"phase", phase.ID,
				"timeout", phaseMax,
				"task", t.ID,
			)
			return result, &phaseTimeoutError{
				phase:   phase.ID,
				timeout: phaseMax,
				err:     err,
			}
		}
	}
	return result, err
}

