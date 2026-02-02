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
	"sync"

	"golang.org/x/sync/errgroup"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
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
func newSafeVarsFrom(initial map[string]string) *safeVars {
	sv := &safeVars{
		vars: make(map[string]string, len(initial)),
	}
	for k, v := range initial {
		sv.vars[k] = v
	}
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
	for k, v := range sv.vars {
		result[k] = v
	}
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
	for k, v := range other.vars {
		sv.vars[k] = v
	}
}

// parallelPhaseResult holds the result of a parallel phase execution.
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
	for k, v := range vars {
		localVars[k] = v
	}

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
func cloneResolutionContext(rctx *variable.ResolutionContext) *variable.ResolutionContext {
	if rctx == nil {
		return nil
	}

	// Create a new context with copied values
	clone := &variable.ResolutionContext{
		TaskID:           rctx.TaskID,
		TaskTitle:        rctx.TaskTitle,
		TaskDescription:  rctx.TaskDescription,
		TaskWeight:       rctx.TaskWeight,
		TaskCategory:     rctx.TaskCategory,
		WorkflowID:       rctx.WorkflowID,
		WorkflowName:     rctx.WorkflowName,
		Phase:            rctx.Phase,
		ReviewRound:      rctx.ReviewRound,
		LoopIteration:    rctx.LoopIteration,
		QAIteration:      rctx.QAIteration,
		QAMaxIterations:  rctx.QAMaxIterations,
		QAFindings:       rctx.QAFindings,
		PreviousFindings: rctx.PreviousFindings,
		WorktreePath:     rctx.WorktreePath,
		ProjectRoot:      rctx.ProjectRoot,
		TargetBranch:     rctx.TargetBranch,
		InitiativeID:     rctx.InitiativeID,
		InitiativeVision: rctx.InitiativeVision,
	}

	// Clone maps
	if rctx.PhaseOutputVars != nil {
		clone.PhaseOutputVars = make(map[string]string, len(rctx.PhaseOutputVars))
		for k, v := range rctx.PhaseOutputVars {
			clone.PhaseOutputVars[k] = v
		}
	}

	if rctx.PriorOutputs != nil {
		clone.PriorOutputs = make(map[string]string, len(rctx.PriorOutputs))
		for k, v := range rctx.PriorOutputs {
			clone.PriorOutputs[k] = v
		}
	}

	if rctx.Decisions != nil {
		clone.Decisions = make([]variable.Decision, len(rctx.Decisions))
		copy(clone.Decisions, rctx.Decisions)
	}

	return clone
}

// normalizeVarName converts a phase ID to a variable name.
// e.g., "qa-e2e-test" -> "QA_E2E_TEST"
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

// isLevelFullyCompleted checks if all phases in a level are in a terminal state.
// Used for resume logic to determine if a level can be skipped.
func isLevelFullyCompleted(t *orcv1.Task, phases []*db.WorkflowPhase) bool {
	if t == nil || t.Execution == nil {
		return false
	}
	for _, p := range phases {
		if ps, ok := t.Execution.Phases[p.PhaseTemplateID]; ok {
			if !IsPhaseTerminalForResume(ps.Status) {
				return false
			}
		} else {
			return false
		}
	}
	return true
}

// findPhaseInLevels locates a phase by ID across all levels and returns its level index.
// Returns -1 if not found.
func findPhaseInLevels(levels [][]*db.WorkflowPhase, phaseID string) int {
	for i, level := range levels {
		for _, p := range level {
			if p.PhaseTemplateID == phaseID {
				return i
			}
		}
	}
	return -1
}
