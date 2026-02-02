// parallel_execution.go implements parallel phase execution for workflow runs.
// Phases are grouped by execution level based on their dependency graph.
// Phases within the same level have no dependencies on each other and can run in parallel.
//
// Key invariants:
// - Uses errgroup.WithContext for parallel execution (SC-3)
// - Cancels sibling phases on first failure (DEC-008)
// - First error in parallel group is reported (SC-6)
// - Thread-safe variable writes via safeVars (SC-8)
// - rctx cloned per goroutine (SC-9)
package executor

import (
	"context"
	"fmt"
	"maps"
	"sync"

	"golang.org/x/sync/errgroup"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

// safeVars provides thread-safe access to a map[string]string.
// Used during parallel phase execution to safely collect phase output variables.
type safeVars struct {
	mu   sync.RWMutex
	vars map[string]string
}

// newSafeVars creates a new thread-safe vars wrapper.
func newSafeVars() *safeVars {
	return &safeVars{
		vars: make(map[string]string),
	}
}

// newSafeVarsFrom creates a safeVars initialized from an existing map.
//
//nolint:unused // Prepared for parallel execution wiring
func newSafeVarsFrom(initial map[string]string) *safeVars {
	sv := &safeVars{
		vars: make(map[string]string, len(initial)),
	}
	maps.Copy(sv.vars, initial)
	return sv
}

// Set stores a value for the given key.
func (sv *safeVars) Set(key, value string) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.vars[key] = value
}

// Get retrieves a value for the given key.
func (sv *safeVars) Get(key string) string {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	return sv.vars[key]
}

// Clone returns a copy of the internal map.
func (sv *safeVars) Clone() map[string]string {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	result := make(map[string]string, len(sv.vars))
	maps.Copy(result, sv.vars)
	return result
}

// MergeFrom copies all entries from another safeVars into this one.
func (sv *safeVars) MergeFrom(other *safeVars) {
	if other == nil {
		return
	}
	other.mu.RLock()
	defer other.mu.RUnlock()
	sv.mu.Lock()
	defer sv.mu.Unlock()
	maps.Copy(sv.vars, other.vars)
}

// parallelPhaseResult holds the result of a parallel phase execution.
//
//nolint:unused // Prepared for parallel execution wiring
type parallelPhaseResult struct {
	phase       *db.WorkflowPhase
	result      PhaseResult
	err         error
	outputVars  *safeVars        // Variables produced by this phase
	rctx        *variable.ResolutionContext // Updated resolution context
}

// executeLevelParallel executes all phases in a level concurrently.
// Returns the first error encountered (if any) and cancels remaining phases.
// Phase outputs are collected via thread-safe safeVars.
//
// Parameters:
// - ctx: Context for cancellation
// - phases: Phases to execute in parallel (all must have dependencies satisfied)
// - vars: Current variable state (read-only during parallel execution)
// - baseRctx: Base resolution context to clone for each goroutine
// - run: Workflow run record
// - t: Task being executed (may be nil for non-task contexts)
// - executePhase: Callback to execute a single phase
//
// Returns:
// - results: All phase results (even from failed/cancelled phases)
// - mergedVars: Variables collected from all phases (merge into caller's vars after)
// - firstErr: First error encountered, or nil if all succeeded
//
//nolint:unused // Prepared for parallel execution wiring
func (we *WorkflowExecutor) executeLevelParallel(
	ctx context.Context,
	phases []*db.WorkflowPhase,
	vars map[string]string,
	baseRctx *variable.ResolutionContext,
	run *db.WorkflowRun,
	t *orcv1.Task,
	varDefs []variable.Definition,
) ([]parallelPhaseResult, *safeVars, error) {
	if len(phases) == 0 {
		return nil, nil, nil
	}

	// Single phase optimization: skip goroutine overhead (SC-10 behavioral equivalence)
	if len(phases) == 1 {
		return we.executeSinglePhase(ctx, phases[0], vars, baseRctx, run, t, varDefs)
	}

	// Set flag to indicate we're in parallel level - this prevents task state updates
	// during execution to avoid race conditions. Task state will be updated after
	// all phases in this level complete.
	we.inParallelLevel = true
	defer func() { we.inParallelLevel = false }()

	// Parallel execution with errgroup
	g, gctx := errgroup.WithContext(ctx)

	// Collect results from all goroutines
	results := make([]parallelPhaseResult, len(phases))
	var resultsMu sync.Mutex

	// Thread-safe collection of output variables
	mergedVars := newSafeVars()

	for i, phase := range phases {
		// Capture loop variables
		idx := i
		p := phase

		g.Go(func() (err error) {
			// Panic recovery per goroutine
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic in phase %s: %v", p.PhaseTemplateID, r)
					resultsMu.Lock()
					results[idx] = parallelPhaseResult{
						phase: p,
						err:   err,
					}
					resultsMu.Unlock()
				}
			}()

			// Clone resolution context for this goroutine (SC-9)
			rctx := cloneResolutionContext(baseRctx)

			// Execute the phase
			result, phaseErr := we.executeParallelPhase(gctx, p, vars, rctx, run, t, varDefs, mergedVars)

			resultsMu.Lock()
			results[idx] = parallelPhaseResult{
				phase:      p,
				result:     result,
				err:        phaseErr,
				rctx:       rctx,
			}
			resultsMu.Unlock()

			return phaseErr
		})
	}

	// Wait for all goroutines and get first error (SC-6)
	firstErr := g.Wait()

	return results, mergedVars, firstErr
}

// executeSinglePhase handles the single-phase case without goroutine overhead.
//
//nolint:unused // Prepared for parallel execution wiring
func (we *WorkflowExecutor) executeSinglePhase(
	ctx context.Context,
	phase *db.WorkflowPhase,
	vars map[string]string,
	baseRctx *variable.ResolutionContext,
	run *db.WorkflowRun,
	t *orcv1.Task,
	varDefs []variable.Definition,
) ([]parallelPhaseResult, *safeVars, error) {
	outputVars := newSafeVars()

	// Clone context for consistency with parallel path
	rctx := cloneResolutionContext(baseRctx)

	result, err := we.executeParallelPhase(ctx, phase, vars, rctx, run, t, varDefs, outputVars)

	results := []parallelPhaseResult{{
		phase:      phase,
		result:     result,
		err:        err,
		outputVars: outputVars,
		rctx:       rctx,
	}}

	return results, outputVars, err
}

// executeParallelPhase executes a single phase within parallel execution.
// This is the core execution logic extracted for use by both single and parallel paths.
//
//nolint:unused // Prepared for parallel execution wiring
func (we *WorkflowExecutor) executeParallelPhase(
	ctx context.Context,
	phase *db.WorkflowPhase,
	vars map[string]string,
	rctx *variable.ResolutionContext,
	run *db.WorkflowRun,
	t *orcv1.Task,
	varDefs []variable.Definition,
	outputVars *safeVars,
) (PhaseResult, error) {
	// Check context before starting
	if ctx.Err() != nil {
		return PhaseResult{
			PhaseID: phase.PhaseTemplateID,
			Status:  orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
			Error:   ctx.Err().Error(),
		}, ctx.Err()
	}

	// Load phase template
	tmpl, err := we.projectDB.GetPhaseTemplate(phase.PhaseTemplateID)
	if err != nil {
		return PhaseResult{
			PhaseID: phase.PhaseTemplateID,
			Status:  orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
			Error:   err.Error(),
		}, fmt.Errorf("load phase template %s: %w", phase.PhaseTemplateID, err)
	}
	if tmpl == nil {
		return PhaseResult{
			PhaseID: phase.PhaseTemplateID,
			Status:  orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
			Error:   "template not found",
		}, fmt.Errorf("phase template not found: %s", phase.PhaseTemplateID)
	}

	// Check phase condition (skip if condition not met)
	if phase.Condition != "" {
		condCtx := &ConditionContext{
			Task: t,
			Vars: vars,
			RCtx: rctx,
		}
		condResult, condErr := EvaluateCondition(phase.Condition, condCtx)
		if condErr != nil {
			we.logger.Warn("condition evaluation failed, executing phase",
				"phase", phase.PhaseTemplateID, "error", condErr)
		} else if !condResult {
			// Condition not met - skip this phase
			we.logger.Info("phase skipped by condition",
				"phase", phase.PhaseTemplateID,
				"condition", phase.Condition)

			// Mark phase as skipped in task execution state
			if t != nil {
				task.EnsureExecutionProto(t)
				task.SkipPhaseProto(t.Execution, phase.PhaseTemplateID, "condition not met")
				_ = we.backend.SaveTask(t) // Best-effort
			}

			return PhaseResult{
				PhaseID: phase.PhaseTemplateID,
				Status:  orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String(),
			}, nil
		}
	}

	// Create run phase record
	runPhase := &db.WorkflowRunPhase{
		WorkflowRunID:   run.ID,
		PhaseTemplateID: phase.PhaseTemplateID,
		Status:          orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}
	if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
		return PhaseResult{
			PhaseID: phase.PhaseTemplateID,
			Status:  orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
			Error:   err.Error(),
		}, fmt.Errorf("save run phase: %w", err)
	}

	// Update run with current phase (for monitoring - last writer wins in parallel)
	run.CurrentPhase = phase.PhaseTemplateID
	_ = we.backend.SaveWorkflowRun(run) // Best-effort, don't fail on this

	// Update phase in resolution context
	rctx.Phase = tmpl.ID

	// Enrich context with phase-specific data
	we.enrichContextForPhase(rctx, tmpl.ID, t)

	// Re-resolve variables with updated context
	// Create a read-only copy of vars for this goroutine
	localVars := make(map[string]string, len(vars))
	maps.Copy(localVars, vars)

	resolvedVars, err := we.resolver.ResolveAll(ctx, varDefs, rctx)
	if err != nil {
		return PhaseResult{
			PhaseID: phase.PhaseTemplateID,
			Status:  orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
			Error:   err.Error(),
		}, fmt.Errorf("resolve variables for phase %s: %w", tmpl.ID, err)
	}

	// Merge resolved vars into localVars
	for k, v := range resolvedVars {
		localVars[k] = v
	}

	// Execute phase with timeout
	// Note: Task state updates are skipped when inParallelLevel is true (checked in executePhase)
	// This avoids race conditions when multiple phases run concurrently.
	phaseResult, err := we.executePhaseWithTimeout(ctx, tmpl, phase, localVars, rctx, run, runPhase, t)

	// Collect output variables (thread-safe)
	if phaseResult.Content != "" && outputVars != nil {
		// Store using the same logic as applyPhaseContentToVars
		outputVars.Set("OUTPUT_"+phase.PhaseTemplateID, phaseResult.Content)
		varName := tmpl.OutputVarName
		if varName == "" {
			varName = "OUTPUT_" + normalizeVarName(phase.PhaseTemplateID)
		}
		outputVars.Set(varName, phaseResult.Content)
	}

	return phaseResult, err
}

// cloneResolutionContext creates a deep copy of a ResolutionContext.
// This ensures each goroutine has its own context to modify.
//
//nolint:unused // Prepared for parallel execution wiring
func cloneResolutionContext(rctx *variable.ResolutionContext) *variable.ResolutionContext {
	if rctx == nil {
		return nil
	}

	// Create a new context with copied values
	clone := &variable.ResolutionContext{
		// Task context
		TaskID:          rctx.TaskID,
		TaskTitle:       rctx.TaskTitle,
		TaskDescription: rctx.TaskDescription,
		TaskWeight:      rctx.TaskWeight,
		TaskCategory:    rctx.TaskCategory,

		// Workflow context
		WorkflowID:    rctx.WorkflowID,
		WorkflowRunID: rctx.WorkflowRunID,
		Phase:         rctx.Phase,
		Iteration:     rctx.Iteration,

		// Retry context
		RetryAttempt:   rctx.RetryAttempt,
		RetryFromPhase: rctx.RetryFromPhase,
		RetryReason:    rctx.RetryReason,

		// Path context
		WorkingDir:  rctx.WorkingDir,
		ProjectRoot: rctx.ProjectRoot,

		// Prompt context
		Prompt:       rctx.Prompt,
		Instructions: rctx.Instructions,

		// Git context
		TargetBranch: rctx.TargetBranch,
		TaskBranch:   rctx.TaskBranch,

		// Constitution and patterns
		ConstitutionContent: rctx.ConstitutionContent,
		ErrorPatterns:       rctx.ErrorPatterns,

		// Initiative context
		InitiativeID:        rctx.InitiativeID,
		InitiativeTitle:     rctx.InitiativeTitle,
		InitiativeVision:    rctx.InitiativeVision,
		InitiativeDecisions: rctx.InitiativeDecisions,
		InitiativeTasks:     rctx.InitiativeTasks,

		// Review context
		ReviewRound:    rctx.ReviewRound,
		ReviewFindings: rctx.ReviewFindings,
		LoopIteration:  rctx.LoopIteration,

		// Project detection
		Language:     rctx.Language,
		HasFrontend:  rctx.HasFrontend,
		HasTests:     rctx.HasTests,
		TestCommand:  rctx.TestCommand,
		LintCommand:  rctx.LintCommand,
		BuildCommand: rctx.BuildCommand,

		// Testing configuration
		CoverageThreshold: rctx.CoverageThreshold,

		// UI testing context
		RequiresUITesting: rctx.RequiresUITesting,
		ScreenshotDir:     rctx.ScreenshotDir,
		TestResults:       rctx.TestResults,
		TDDTestPlan:       rctx.TDDTestPlan,

		// Automation context
		RecentCompletedTasks: rctx.RecentCompletedTasks,
		RecentChangedFiles:   rctx.RecentChangedFiles,
		ChangelogContent:     rctx.ChangelogContent,
		ClaudeMDContent:      rctx.ClaudeMDContent,

		// QA E2E testing context
		QAIteration:      rctx.QAIteration,
		QAMaxIterations:  rctx.QAMaxIterations,
		QAFindings:       rctx.QAFindings,
		BeforeImages:     rctx.BeforeImages,
		PreviousFindings: rctx.PreviousFindings,
	}

	// Clone slices
	if len(rctx.Frameworks) > 0 {
		clone.Frameworks = make([]string, len(rctx.Frameworks))
		copy(clone.Frameworks, rctx.Frameworks)
	}

	// Clone maps
	if rctx.PhaseOutputVars != nil {
		clone.PhaseOutputVars = make(map[string]string, len(rctx.PhaseOutputVars))
		maps.Copy(clone.PhaseOutputVars, rctx.PhaseOutputVars)
	}

	if rctx.PriorOutputs != nil {
		clone.PriorOutputs = make(map[string]string, len(rctx.PriorOutputs))
		maps.Copy(clone.PriorOutputs, rctx.PriorOutputs)
	}

	if rctx.Environment != nil {
		clone.Environment = make(map[string]string, len(rctx.Environment))
		maps.Copy(clone.Environment, rctx.Environment)
	}

	return clone
}

// normalizeVarName converts a phase ID to a variable name.
// e.g., "qa-e2e-test" -> "QA_E2E_TEST"
//
//nolint:unused // Prepared for parallel execution wiring
func normalizeVarName(phaseID string) string {
	result := make([]byte, 0, len(phaseID))
	for i := 0; i < len(phaseID); i++ {
		c := phaseID[i]
		if c == '-' || c == '_' {
			result = append(result, '_')
		} else if c >= 'a' && c <= 'z' {
			result = append(result, c-32) // to upper
		} else {
			result = append(result, c)
		}
	}
	return string(result)
}

// runPhasesParallel executes all phases using level-based parallel execution.
// Phases at the same level (no dependencies between them) run concurrently.
// Levels are executed sequentially, waiting for all phases in a level to complete
// before starting the next level.
//
// This is the parallel execution entry point, called from Run when parallelExecution is enabled.
//
//nolint:unused // Prepared for parallel execution wiring
func (we *WorkflowExecutor) runPhasesParallel(
	ctx context.Context,
	phases []*db.WorkflowPhase,
	vars map[string]string,
	rctx *variable.ResolutionContext,
	run *db.WorkflowRun,
	t *orcv1.Task,
	varDefs []variable.Definition,
) ([]PhaseResult, error) {
	// Compute execution levels from dependency graph
	levels, err := computeExecutionLevels(phases)
	if err != nil {
		return nil, fmt.Errorf("compute execution levels: %w", err)
	}

	if len(levels) == 0 {
		return nil, nil
	}

	results := make([]PhaseResult, 0, len(phases))
	mergedVars := newSafeVarsFrom(vars)

	// Process levels sequentially, phases within levels in parallel
	for levelIdx, level := range levels {
		// Filter out already-completed phases for resume support
		var phasesToRun []*db.WorkflowPhase
		for _, phase := range level {
			if t != nil {
				if ps, ok := t.Execution.Phases[phase.PhaseTemplateID]; ok {
					if IsPhaseTerminalForResume(ps.Status) {
						we.logger.Info("skipping terminal phase (parallel)", "phase", phase.PhaseTemplateID, "status", ps.Status)
						// Load content from completed phase for variable chaining
						if output, err := we.backend.GetPhaseOutput(run.ID, phase.PhaseTemplateID); err == nil && output != nil {
							mergedVars.Set("OUTPUT_"+phase.PhaseTemplateID, output.Content)
							if output.OutputVarName != "" {
								mergedVars.Set(output.OutputVarName, output.Content)
							}
						}
						continue
					}
				}
			}
			phasesToRun = append(phasesToRun, phase)
		}

		if len(phasesToRun) == 0 {
			continue
		}

		we.logger.Info("executing level",
			"level", levelIdx,
			"phases", len(phasesToRun),
			"run_id", run.ID,
		)

		// Execute phases in this level in parallel
		levelResults, outputVars, levelErr := we.executeLevelParallel(
			ctx,
			phasesToRun,
			mergedVars.Clone(),
			rctx,
			run,
			t,
			varDefs,
		)

		// Collect results
		for _, pr := range levelResults {
			results = append(results, pr.result)
		}

		// Merge output variables from this level
		if outputVars != nil {
			mergedVars.MergeFrom(outputVars)
		}

		// Update task state for all completed phases in this level (deferred from parallel execution)
		// This is done sequentially after all parallel phases complete to avoid race conditions.
		if t != nil && len(phasesToRun) > 1 {
			we.updateTaskStateAfterLevel(levelResults, t, run)
		}

		// On error, stop execution (DEC-008: cancel siblings already done by errgroup)
		if levelErr != nil {
			we.logger.Error("level execution failed",
				"level", levelIdx,
				"error", levelErr,
			)
			return results, levelErr
		}
	}

	return results, nil
}

// updateTaskStateAfterLevel updates the task state for all phases that completed
// in a parallel level. This is called after all phases in the level have finished
// to avoid race conditions during concurrent execution.
//
//nolint:unused,unparam // Prepared for parallel execution wiring
func (we *WorkflowExecutor) updateTaskStateAfterLevel(results []parallelPhaseResult, t *orcv1.Task, _ *db.WorkflowRun) {
	if t == nil || t.Execution == nil {
		return
	}

	for _, pr := range results {
		if pr.phase == nil {
			continue
		}

		phaseID := pr.phase.PhaseTemplateID

		// Only update state for successful phases
		if pr.err != nil {
			continue
		}

		// Create checkpoint commit for this phase so `orc rewind` works
		commitSHA := ""
		if we.gitOps != nil {
			checkpoint, err := we.gitOps.CreateCheckpoint(t.Id, phaseID, "completed")
			if err != nil {
				we.logger.Debug("no checkpoint created (parallel)", "phase", phaseID, "reason", err)
			} else if checkpoint != nil {
				commitSHA = checkpoint.CommitSHA
			}
		}

		// Update phase state
		task.CompletePhaseProto(t.Execution, phaseID, commitSHA)

		// Add cost from this phase
		task.AddCostProto(t.Execution, phaseID, pr.result.CostUSD)
	}

	// Set current phase to the last completed phase in this level
	// (for status display - during parallel execution this is somewhat arbitrary)
	if len(results) > 0 {
		lastPhase := results[len(results)-1]
		if lastPhase.phase != nil && lastPhase.err == nil {
			task.SetCurrentPhaseProto(t, lastPhase.phase.PhaseTemplateID)
		}
	}

	// Save task state once after all phases updated
	if err := we.backend.SaveTask(t); err != nil {
		we.logger.Warn("failed to save task state after parallel level", "error", err)
	}
}
