package bench

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/storage"
)

// Runner executes benchmark runs.
type Runner struct {
	store     *Store
	globalDB  *db.GlobalDB
	workspace *Workspace
	evaluator *Evaluator
	logger    *slog.Logger

	// Executor paths (resolved at construction)
	claudePath string
	codexPath  string

	// Model override: when set, ALL phases use this provider/model
	// regardless of variant config. Used for cheap smoke testing (e.g. haiku).
	overrideProvider        string
	overrideModel           string
	overrideReasoningEffort string

	// Task filter: when non-empty, only run tasks with matching IDs.
	taskFilter map[string]bool

	// In-memory infrastructure for WorkflowExecutor (created once in NewRunner).
	// The executor needs a backend + projectDB for workflow run records, but bench
	// doesn't use that data — it lives in bench.db. So we give it an ephemeral
	// in-memory store that gets discarded.
	benchBackend *storage.DatabaseBackend
	benchPDB     *db.ProjectDB

	// For testing: override turn executor (injected into WorkflowExecutor).
	turnExecutor executor.TurnExecutor
}

// RunnerOption configures a Runner.
type RunnerOption func(*Runner)

// WithRunnerLogger sets the logger.
func WithRunnerLogger(l *slog.Logger) RunnerOption {
	return func(r *Runner) { r.logger = l }
}

// WithClaudePath sets the Claude CLI path.
func WithClaudePath(path string) RunnerOption {
	return func(r *Runner) { r.claudePath = path }
}

// WithCodexPath sets the Codex CLI path.
func WithCodexPath(path string) RunnerOption {
	return func(r *Runner) { r.codexPath = path }
}

// WithModelOverride forces all phases to use a specific provider/model,
// ignoring variant phase_overrides and defaults. Useful for cheap smoke
// testing with haiku before spending on opus.
// Optional reasoningEffort (e.g. "high", "medium", "low") for Codex models.
func WithModelOverride(provider, model, reasoningEffort string) RunnerOption {
	return func(r *Runner) {
		r.overrideProvider = provider
		r.overrideModel = model
		r.overrideReasoningEffort = reasoningEffort
	}
}

// WithTaskFilter limits execution to specific task IDs.
func WithTaskFilter(taskIDs []string) RunnerOption {
	return func(r *Runner) {
		r.taskFilter = make(map[string]bool, len(taskIDs))
		for _, id := range taskIDs {
			r.taskFilter[id] = true
		}
	}
}

// WithTurnExecutor overrides turn executor creation (for testing).
// The executor is injected into WorkflowExecutor via WithWorkflowTurnExecutor.
func WithTurnExecutor(te executor.TurnExecutor) RunnerOption {
	return func(r *Runner) { r.turnExecutor = te }
}

// NewRunner creates a benchmark runner.
func NewRunner(store *Store, globalDB *db.GlobalDB, workspace *Workspace, opts ...RunnerOption) *Runner {
	r := &Runner{
		store:      store,
		globalDB:   globalDB,
		workspace:  workspace,
		evaluator:  NewEvaluator(),
		logger:     slog.Default(),
		claudePath: "claude",
		codexPath:  "codex",
	}
	for _, opt := range opts {
		opt(r)
	}

	// Create in-memory infrastructure for WorkflowExecutor.
	// This is cheap (SQLite in-memory) and reusable across runs.
	backend, pdb, err := initBenchInfra()
	if err != nil {
		r.logger.Error("failed to create bench infrastructure", "error", err)
		// Non-fatal: RunSingle will fail when it tries to use nil backend
	} else {
		r.benchBackend = backend
		r.benchPDB = pdb
	}

	return r
}

// initBenchInfra creates an in-memory storage backend and projectDB for
// WorkflowExecutor. The executor needs these for workflow run records,
// but bench doesn't consume that data — it's discarded.
// Seeds phase templates so FK constraints on workflow_run_phases work.
func initBenchInfra() (*storage.DatabaseBackend, *db.ProjectDB, error) {
	// FK constraints disabled — bench discards executor run data anyway.
	// Avoids needing to seed every referenced table (workflows, tasks, phase_templates).
	backend, err := storage.NewInMemoryBackendNoFK()
	if err != nil {
		return nil, nil, fmt.Errorf("create in-memory backend: %w", err)
	}
	return backend, backend.DB(), nil
}

// RunBaseline executes the baseline variant against all applicable tasks.
// All phases execute (no frozen outputs). Phase outputs are saved as frozen
// for use by variant runs.
func (r *Runner) RunBaseline(ctx context.Context, trials int) error {
	baseline, err := r.store.GetBaselineVariant(ctx)
	if err != nil {
		return fmt.Errorf("get baseline variant: %w", err)
	}

	tasks, err := r.store.TasksForVariant(ctx, baseline)
	if err != nil {
		return fmt.Errorf("get tasks for baseline: %w", err)
	}

	tasks = r.filterTasks(tasks)
	r.logger.Info("starting baseline run", "variant", baseline.ID, "tasks", len(tasks), "trials", trials)

	for _, task := range tasks {
		for trial := 1; trial <= trials; trial++ {
			if err := r.RunSingle(ctx, baseline, task, trial); err != nil {
				r.logger.Error("baseline run failed", "task", task.ID, "trial", trial, "error", err)
				// Continue with next task on failure
			}
		}
	}

	return nil
}

// RunVariant executes a specific variant against all applicable tasks.
// Uses cascade mode: data-only phases (spec, tdd, breakdown) before the first
// overridden phase are frozen from baseline. Everything from the override onwards
// runs live — the variant's model change cascades through downstream phases.
func (r *Runner) RunVariant(ctx context.Context, variantID string, trials int) error {
	variant, err := r.store.GetVariant(ctx, variantID)
	if err != nil {
		return fmt.Errorf("get variant %s: %w", variantID, err)
	}

	tasks, err := r.store.TasksForVariant(ctx, variant)
	if err != nil {
		return fmt.Errorf("get tasks for variant: %w", err)
	}

	tasks = r.filterTasks(tasks)
	r.logger.Info("starting variant run", "variant", variant.ID, "tasks", len(tasks), "trials", trials)

	for _, task := range tasks {
		for trial := 1; trial <= trials; trial++ {
			if err := r.RunSingle(ctx, variant, task, trial); err != nil {
				r.logger.Error("variant run failed", "task", task.ID, "variant", variant.ID, "trial", trial, "error", err)
			}
		}
	}

	return nil
}

// RunSingle executes one variant against one task for one trial.
func (r *Runner) RunSingle(ctx context.Context, variant *Variant, task *Task, trial int) error {
	if r.benchBackend == nil {
		return fmt.Errorf("bench infrastructure not initialized")
	}

	// Apply tier-based timeout so Codex/Claude CLI don't hit their short defaults
	if timeout, ok := tierTimeout[task.Tier]; ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	runID := uuid.New().String()

	// Clean up any stale run from previous attempts (e.g. error/fail)
	if err := r.store.DeleteRunByCombo(ctx, variant.ID, task.ID, trial); err != nil {
		r.logger.Warn("failed to clean stale run", "error", err)
	}

	// Create run record
	run := &Run{
		ID:          runID,
		VariantID:   variant.ID,
		TaskID:      task.ID,
		TrialNumber: trial,
		Status:      RunStatusRunning,
		StartedAt:   time.Now(),
	}
	if err := r.store.SaveRun(ctx, run); err != nil {
		return fmt.Errorf("create run: %w", err)
	}

	r.logger.Info("executing run", "run", runID, "variant", variant.ID, "task", task.ID, "trial", trial)

	// Get the project
	project, err := r.store.GetProject(ctx, task.ProjectID)
	if err != nil {
		return r.failRun(ctx, run, fmt.Errorf("get project %s: %w", task.ProjectID, err))
	}

	// Setup workspace
	workDir, err := r.workspace.SetupRun(runID, project, task)
	if err != nil {
		return r.failRun(ctx, run, fmt.Errorf("setup workspace: %w", err))
	}
	defer r.workspace.CleanupRun(runID, filepath.Join(r.workspace.ReposDir, project.ID))

	// Load frozen outputs if this isn't the baseline
	var frozenOutputs FrozenOutputMap
	if !variant.IsBaseline {
		baseline, err := r.store.GetBaselineVariant(ctx)
		if err != nil {
			return r.failRun(ctx, run, fmt.Errorf("get baseline: %w", err))
		}
		frozenOutputs, err = LoadFrozenOutputs(ctx, r.store, task.ID, baseline.ID, trial)
		if err != nil {
			return r.failRun(ctx, run, fmt.Errorf("load frozen outputs: %w", err))
		}
		if len(frozenOutputs) == 0 {
			return r.failRun(ctx, run, fmt.Errorf("no frozen outputs for task %s baseline trial %d — run baseline first", task.ID, trial))
		}
	}

	// Load workflow phases from GlobalDB for cascade decisions
	workflowID := r.resolveWorkflow(task, variant)
	phases, err := r.globalDB.GetWorkflowPhases(workflowID)
	if err != nil {
		return r.failRun(ctx, run, fmt.Errorf("load workflow %s phases: %w", workflowID, err))
	}
	r.logger.Debug("resolved workflow", "task", task.ID, "tier", task.Tier, "workflow", workflowID)

	// --- Cascade mode: compute which phases get frozen outputs ---
	//
	// Find the first overridden phase. Data-only phases before this point
	// get frozen from baseline (consistent inputs). Everything from the
	// override onwards runs live — the model change cascades through all
	// downstream phases, including implement, review, and docs.
	firstOverrideIdx := len(phases)
	if !variant.IsBaseline {
		for i, p := range phases {
			if _, ok := variant.PhaseOverrides[p.PhaseTemplateID]; ok {
				firstOverrideIdx = i
				break
			}
		}
	}

	prePopulated := make(map[string]string)
	for i, phase := range phases {
		phaseID := phase.PhaseTemplateID

		// Freeze decision — ALL conditions must hold:
		// 1. Not baseline (baseline always runs everything live)
		// 2. Not overridden by this variant
		// 3. Frozen output exists from baseline
		// 4. Phase is data-only (produces text artifacts, not filesystem changes)
		// 5. Phase appears BEFORE the first override (cascade: live from override onwards)
		_, hasOverride := variant.PhaseOverrides[phaseID]
		hasFrozen := frozenOutputs[phaseID] != nil
		shouldFreeze := !variant.IsBaseline && !hasOverride && hasFrozen &&
			phasesAllowFreezing[phaseID] && i < firstOverrideIdx

		if shouldFreeze {
			prePopulated[phaseID] = frozenOutputs[phaseID].OutputContent
			r.logger.Debug("will freeze phase", "phase", phaseID)
		}
	}

	// --- Build executor inputs ---

	phaseOverrides := r.buildPhaseOverrides(variant, phases)
	taskVars := r.buildTaskVariables(task, project, runID, workDir)
	benchCfg := r.benchConfig()

	// Build executor options
	execOpts := []executor.WorkflowExecutorOption{
		executor.WithPrePopulatedPhaseOutputs(prePopulated),
		executor.WithPhaseModelOverrides(phaseOverrides),
		executor.WithMaxLoopOverride(1),
		executor.WithMaxTurnsOverride(0), // 0 = unlimited turns
		executor.WithSkipGates(true),
		executor.WithWorkflowClaudePath(r.claudePath),
		executor.WithWorkflowCodexPath(r.codexPath),
		executor.WithWorkflowLogger(r.logger),
	}
	if r.turnExecutor != nil {
		execOpts = append(execOpts, executor.WithWorkflowTurnExecutor(r.turnExecutor))
	}

	we := executor.NewWorkflowExecutor(
		r.benchBackend, r.benchPDB, r.globalDB, benchCfg, workDir, execOpts...,
	)

	// --- Execute workflow ---

	result, execErr := we.Run(ctx, workflowID, executor.WorkflowRunOptions{
		ContextType: executor.ContextStandalone,
		Variables:   taskVars,
	})

	// Save per-phase results to bench store (even on partial failure).
	// The executor accumulates PhaseResults for all phases that ran.
	if result != nil {
		r.savePhaseResults(ctx, runID, result, variant, frozenOutputs, trial, task.ID)
	}

	if execErr != nil {
		return r.failRun(ctx, run, fmt.Errorf("workflow execution: %w", execErr))
	}

	// Capture model's diff before evaluation (evaluation may modify test files)
	run.ModelDiff = captureModelDiff(workDir, task.PreFixCommit)

	// Run automated evaluation
	evalResult, err := r.evaluator.RunAll(workDir, project, task)
	if err != nil {
		r.logger.Warn("evaluation failed", "error", err)
	}

	// Populate run with eval metrics
	if evalResult != nil {
		run.TestPass = evalResult.TestPass
		run.BuildSuccess = evalResult.BuildSuccess
		run.TestOutput = evalResult.TestOutput
		run.BuildOutput = evalResult.BuildOutput
	}

	// Update run status
	run.Status = RunStatusFail
	if evalResult != nil && evalResult.TestPass && evalResult.BuildSuccess {
		run.Status = RunStatusPass
	}
	run.CompletedAt = time.Now()

	// Use a detached context for the final save — the tier timeout governs
	// phase execution, not persistence. If we've done all the expensive work
	// (LLM calls, evaluation), losing the results to a deadline is wasteful.
	saveCtx, saveCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer saveCancel()
	if err := r.store.SaveRun(saveCtx, run); err != nil {
		return fmt.Errorf("update run: %w", err)
	}

	r.logger.Info("run completed", "run", runID, "status", run.Status,
		"duration", run.CompletedAt.Sub(run.StartedAt).Round(time.Second))

	return nil
}

// buildTaskVariables creates a map of task metadata to inject as variable
// overrides in ContextStandalone mode. These flow through opts.Variables →
// rctx.Environment → addBuiltinVariables override (resolver.go).
func (r *Runner) buildTaskVariables(task *Task, project *Project, runID, workDir string) map[string]string {
	weight := string(task.Tier)
	return map[string]string{
		// Task metadata
		"TASK_ID":          task.ID,
		"TASK_TITLE":       task.Title,
		"TASK_DESCRIPTION": task.Description,
		"TASK_CATEGORY":    task.Category,
		"CATEGORY":         task.Category,
		"WEIGHT":           weight,

		// Project / build
		"LANGUAGE":      project.Language,
		"TEST_COMMAND":  project.TestCmd,
		"BUILD_COMMAND": project.BuildCmd,
		"LINT_COMMAND":  project.LintCmd,
		"WORKTREE_PATH": workDir,
		"PROJECT_ROOT":  workDir,

		// Git context (synthesized for bench — no real PR)
		"TASK_BRANCH":   fmt.Sprintf("bench/%s/%s", task.ID, runID[:8]),
		"TARGET_BRANCH": "main",
		"COMMIT_AUTHOR": "Benchmark Runner <bench@orc>",

		// These are empty for bench but templates reference them
		"HAS_FRONTEND":         "false",
		"HAS_TESTS":            "true",
		"FRAMEWORKS":           "",
		"INITIATIVE_CONTEXT":   "",
		"INITIATIVE_ID":        "",
		"INITIATIVE_NOTES":     "",
		"CONSTITUTION_CONTENT": "",
		"COVERAGE_THRESHOLD":   "",
	}
}

// buildPhaseOverrides converts variant config + global model override into
// executor PhaseModelOverride maps.
// Priority: global model override > variant phase override > default (opus + thinking).
func (r *Runner) buildPhaseOverrides(variant *Variant, phases []*db.WorkflowPhase) map[string]executor.PhaseModelOverride {
	overrides := make(map[string]executor.PhaseModelOverride, len(phases))
	thinkingTrue := true

	for _, phase := range phases {
		phaseID := phase.PhaseTemplateID

		// Global override takes precedence over everything (smoke testing)
		if r.overrideProvider != "" && r.overrideModel != "" {
			overrides[phaseID] = executor.PhaseModelOverride{
				Provider:        r.overrideProvider,
				Model:           r.overrideModel,
				ReasoningEffort: r.overrideReasoningEffort,
				Thinking:        nil, // Don't force thinking for global override
			}
			continue
		}

		// Start with defaults: opus + thinking
		override := executor.PhaseModelOverride{
			Provider: "claude",
			Model:    "opus",
			Thinking: &thinkingTrue,
		}

		// Apply variant-specific overrides
		if vo, ok := variant.PhaseOverrides[phaseID]; ok {
			if vo.Provider != "" {
				override.Provider = vo.Provider
			}
			if vo.Model != "" {
				override.Model = vo.Model
			}
			if vo.ReasoningEffort != "" {
				override.ReasoningEffort = vo.ReasoningEffort
			}
			if vo.Thinking != nil {
				override.Thinking = vo.Thinking
			}
		}

		overrides[phaseID] = override
	}

	return overrides
}

// benchConfig creates a minimal config.Config for bench execution.
// No worktree management, no PR creation, no completion actions.
func (r *Runner) benchConfig() *config.Config {
	return &config.Config{
		Worktree: config.WorktreeConfig{
			Enabled: false,
		},
		Completion: config.CompletionConfig{
			Action: "none",
		},
	}
}

// savePhaseResults maps executor WorkflowRunResult to bench PhaseResults and
// saves them to bench.db. Also saves frozen outputs for future variant runs.
func (r *Runner) savePhaseResults(
	_ context.Context,
	runID string,
	result *executor.WorkflowRunResult,
	variant *Variant,
	frozenOutputs FrozenOutputMap,
	trial int,
	taskID string,
) {
	// Use detached context — the caller's may be canceled (timeout/interrupt),
	// but phase results should still be persisted since the expensive work is done.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, pr := range result.PhaseResults {
		benchPR := &PhaseResult{
			RunID:   runID,
			PhaseID: pr.PhaseID,
		}

		if pr.WasPrePopulated {
			// This phase was frozen from baseline
			benchPR.WasFrozen = true
			benchPR.OutputContent = pr.Content
			if fo, ok := frozenOutputs[pr.PhaseID]; ok {
				benchPR.FrozenOutputID = fo.ID
			}
		} else {
			// This phase ran live — capture full metrics
			benchPR.Provider = pr.Provider
			benchPR.Model = pr.Model
			benchPR.InputTokens = pr.InputTokens
			benchPR.OutputTokens = pr.OutputTokens
			benchPR.CacheReadTokens = pr.CacheReadTokens
			benchPR.CacheCreationTokens = pr.CacheCreationTokens
			benchPR.CostUSD = pr.CostUSD
			benchPR.DurationMs = int(pr.DurationMS)
			benchPR.OutputContent = pr.Content
		}

		if err := r.store.SavePhaseResult(ctx, benchPR); err != nil {
			r.logger.Error("save phase result failed", "phase", pr.PhaseID, "error", err)
		}

		// Save as frozen output for future variant runs (non-frozen phases with content)
		if !pr.WasPrePopulated && pr.Content != "" {
			if err := SaveFrozenFromResult(ctx, r.store, taskID, pr.PhaseID, variant.ID, pr.OutputVarName, pr.Content, trial); err != nil {
				r.logger.Error("save frozen output failed", "phase", pr.PhaseID, "error", err)
			}
		}
	}
}

// tierToWorkflow maps benchmark task tiers to orc workflow IDs.
// Each tier gets the workflow that matches how orc would actually execute
// a task of that complexity — trivial tasks don't need spec/tdd phases.
var tierToWorkflow = map[Tier]string{
	TierTrivial: "implement-trivial",
	TierSmall:   "implement-small",
	TierMedium:  "implement-medium",
	TierLarge:   "implement-large",
}

// PhaseApplicableTiers maps each phase to the tiers whose workflows contain it.
// Used by TasksForVariant to only run variants against tasks where the
// overridden phase actually exists — no point running a spec-override variant
// against trivial tasks that have no spec phase.
var PhaseApplicableTiers = map[string][]Tier{
	"implement":     {TierTrivial, TierSmall, TierMedium, TierLarge},
	"tiny_spec":     {TierSmall},
	"spec":          {TierMedium, TierLarge},
	"tdd_write":     {TierMedium, TierLarge},
	"tdd_integrate": {TierMedium, TierLarge},
	"breakdown":     {TierLarge},
	"review":        {TierSmall, TierMedium, TierLarge},
	"docs":          {TierSmall, TierMedium, TierLarge},
}

// phasesAllowFreezing lists phases whose outputs are pure data artifacts
// (text consumed by template variables). These CAN be frozen from baseline
// when not overridden — they provide consistent inputs to downstream phases.
//
// Phases NOT in this set produce filesystem changes or evaluate state and
// ALWAYS run live, even when not overridden by the variant. This ensures
// every variant run produces actual code changes for evaluation.
var phasesAllowFreezing = map[string]bool{
	"spec":          true,
	"tiny_spec":     true,
	"tdd_write":     true,
	"tdd_integrate": true,
	"breakdown":     true,
}

var tierTimeout = map[Tier]time.Duration{
	TierTrivial: 45 * time.Minute,
	TierSmall:   90 * time.Minute,
	TierMedium:  105 * time.Minute,
	TierLarge:   120 * time.Minute,
}

// resolveWorkflow picks the workflow based on task tier, falling back
// to the variant's base_workflow if the tier isn't recognized.
func (r *Runner) resolveWorkflow(task *Task, variant *Variant) string {
	if wf, ok := tierToWorkflow[task.Tier]; ok {
		return wf
	}
	return variant.BaseWorkflow
}

// filterTasks applies the task filter if set.
func (r *Runner) filterTasks(tasks []*Task) []*Task {
	if len(r.taskFilter) == 0 {
		return tasks
	}
	var filtered []*Task
	for _, t := range tasks {
		if r.taskFilter[t.ID] {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// failRun marks a run as errored.
func (r *Runner) failRun(_ context.Context, run *Run, err error) error {
	run.Status = RunStatusError
	run.ErrorMessage = err.Error()
	run.CompletedAt = time.Now()
	// Use detached context — the caller's context may be canceled (timeout/interrupt),
	// but we still want to persist the error status.
	saveCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if saveErr := r.store.SaveRun(saveCtx, run); saveErr != nil {
		r.logger.Error("failed to save error status for run", "run", run.ID, "error", saveErr)
	}
	return err
}

// captureModelDiff returns the git diff of all changes the model made
// relative to the pre-fix commit. Called before evaluation (which modifies
// test files) so we get the model's pure output.
//
// Uses a detached context and its own process group to avoid inheriting
// signals from parent cleanup.
func captureModelDiff(workDir, preFixCommit string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// git diff defaults to text mode — binary files show as
	// "Binary files differ" without outputting their content.
	cmd := exec.CommandContext(ctx, "git", "diff", preFixCommit)
	cmd.Dir = workDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		slog.Warn("failed to capture model diff",
			"error", err,
			"workDir", workDir,
			"preFixCommit", preFixCommit,
			"stderr", stderr.String(),
		)
		return "" // Empty diff — judge will skip this run
	}
	return string(out)
}
