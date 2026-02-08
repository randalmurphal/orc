package bench

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/variable"
	"github.com/randalmurphal/orc/templates"
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

	// For testing: override executor creation
	executorFactory func(cfg executor.TurnExecutorConfig) executor.TurnExecutor
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

// WithExecutorFactory overrides executor creation (for testing).
func WithExecutorFactory(f func(cfg executor.TurnExecutorConfig) executor.TurnExecutor) RunnerOption {
	return func(r *Runner) { r.executorFactory = f }
}

// NewRunner creates a benchmark runner.
func NewRunner(store *Store, globalDB *db.GlobalDB, workspace *Workspace, opts ...RunnerOption) *Runner {
	r := &Runner{
		store:     store,
		globalDB:  globalDB,
		workspace: workspace,
		evaluator: NewEvaluator(),
		logger:    slog.Default(),
		claudePath: "claude",
		codexPath:  "codex",
		executorFactory: executor.NewTurnExecutor,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
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
// Uses frozen outputs from the baseline for phases not being tested.
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

	// Load workflow phases from GlobalDB — matched to task tier
	workflowID := r.resolveWorkflow(task, variant)
	phases, err := r.loadWorkflowPhases(workflowID)
	if err != nil {
		return r.failRun(ctx, run, fmt.Errorf("load workflow %s phases: %w", workflowID, err))
	}
	r.logger.Debug("resolved workflow", "task", task.ID, "tier", task.Tier, "workflow", workflowID)

	// Execute each phase
	accumulatedVars := r.buildBaseVars(project, task, workDir)

	for _, phase := range phases {
		phaseID := phase.PhaseTemplateID

		// Check if this phase has an override (should be executed, not frozen)
		_, hasOverride := variant.PhaseOverrides[phaseID]
		hasFrozen := frozenOutputs[phaseID] != nil

		if !variant.IsBaseline && !hasOverride && hasFrozen {
			// Replay frozen output
			fo := frozenOutputs[phaseID]
			BuildVarsFromFrozen(accumulatedVars, FrozenOutputMap{phaseID: fo})

			if err := r.store.SavePhaseResult(ctx, &PhaseResult{
				RunID:          runID,
				PhaseID:        phaseID,
				WasFrozen:      true,
				FrozenOutputID: fo.ID,
				OutputContent:  fo.OutputContent,
			}); err != nil {
				return r.failRun(ctx, run, fmt.Errorf("save frozen phase result %s: %w", phaseID, err))
			}

			r.logger.Debug("replayed frozen output", "phase", phaseID, "var", fo.OutputVarName)
			continue
		}

		// Execute this phase for real
		phaseResult, err := r.executePhase(ctx, runID, phaseID, phase, variant, project, task, workDir, accumulatedVars)
		if err != nil {
			r.logger.Error("phase execution failed", "phase", phaseID, "error", err)
			if saveErr := r.store.SavePhaseResult(ctx, &PhaseResult{
				RunID:   runID,
				PhaseID: phaseID,
			}); saveErr != nil {
				r.logger.Error("save error phase result failed", "error", saveErr)
			}
			continue
		}

		// Save phase result
		if err := r.store.SavePhaseResult(ctx, phaseResult); err != nil {
			r.logger.Error("save phase result failed", "error", err)
		}

		// Save as frozen output for future variant runs
		if phaseResult.OutputContent != "" {
			outputVarName := r.resolveOutputVarName(phase)
			if err := SaveFrozenFromResult(ctx, r.store, task.ID, phaseID, variant.ID, outputVarName, phaseResult.OutputContent, trial); err != nil {
				r.logger.Error("save frozen output failed", "error", err)
			}

			// Add to accumulated vars for next phases
			if outputVarName != "" {
				accumulatedVars[outputVarName] = phaseResult.OutputContent
			}
		}
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

// executePhase runs a single phase with the appropriate model configuration.
func (r *Runner) executePhase(
	ctx context.Context,
	runID, phaseID string,
	phase *db.WorkflowPhase,
	variant *Variant,
	project *Project,
	task *Task,
	workDir string,
	vars variable.VariableSet,
) (*PhaseResult, error) {
	start := time.Now()

	// Load phase template
	tmpl, err := r.globalDB.GetPhaseTemplate(phaseID)
	if err != nil {
		return nil, fmt.Errorf("get phase template %s: %w", phaseID, err)
	}

	// Resolve model/provider for this phase
	provider, model, reasoningEffort, thinking := r.resolvePhaseConfig(phaseID, variant, phase)

	// Load and render prompt
	prompt, err := r.loadAndRenderPrompt(phaseID, tmpl, vars)
	if err != nil {
		return nil, fmt.Errorf("render prompt for %s: %w", phaseID, err)
	}

	// Create executor
	cfg := executor.TurnExecutorConfig{
		Provider:        provider,
		Model:           model,
		WorkingDir:      workDir,
		PhaseID:         phaseID,
		TaskID:          task.ID,
		RunID:           runID,
		MaxTurns:        50, // Generous limit for benchmarks
		ClaudePath:      r.claudePath,
		CodexPath:       r.codexPath,
		ReasoningEffort: reasoningEffort,
		ProducesArtifact: tmpl.ProducesArtifact,
		BypassApprovalsAndSandbox: true,
	}

	turnExec := r.executorFactory(cfg)

	r.logger.Info("executing phase", "phase", phaseID, "provider", provider, "model", model)

	// Execute
	var result *executor.TurnResult
	if tmpl.ProducesArtifact {
		result, err = turnExec.ExecuteTurn(ctx, prompt)
	} else {
		result, err = turnExec.ExecuteTurnWithoutSchema(ctx, prompt)
	}
	if err != nil {
		return nil, fmt.Errorf("execute phase %s: %w", phaseID, err)
	}

	duration := time.Since(start)

	// Build phase result
	pr := &PhaseResult{
		RunID:           runID,
		PhaseID:         phaseID,
		Provider:        provider,
		Model:           model,
		ReasoningEffort: reasoningEffort,
		ThinkingEnabled: thinking,
		DurationMs:      int(duration.Milliseconds()),
		OutputContent:   result.Content,
	}

	// Token usage
	if result.Usage != nil {
		pr.InputTokens = int(result.Usage.InputTokens)
		pr.OutputTokens = int(result.Usage.OutputTokens)
		pr.CacheReadTokens = int(result.Usage.CacheReadInputTokens)
		pr.CacheCreationTokens = int(result.Usage.CacheCreationInputTokens)
	}
	pr.CostUSD = result.CostUSD

	return pr, nil
}

// resolvePhaseConfig determines the model configuration for a phase.
// Priority: runner model override > variant override > workflow default (opus + thinking)
func (r *Runner) resolvePhaseConfig(phaseID string, variant *Variant, phase *db.WorkflowPhase) (provider, model, reasoningEffort string, thinking bool) {
	// Global override takes precedence over everything
	if r.overrideProvider != "" && r.overrideModel != "" {
		return r.overrideProvider, r.overrideModel, r.overrideReasoningEffort, false
	}

	// Defaults: opus with thinking
	provider = "claude"
	model = "opus"
	thinking = true

	// Check variant override
	if override, ok := variant.PhaseOverrides[phaseID]; ok {
		if override.Provider != "" {
			provider = override.Provider
		}
		if override.Model != "" {
			model = override.Model
		}
		if override.ReasoningEffort != "" {
			reasoningEffort = override.ReasoningEffort
		}
		if override.Thinking != nil {
			thinking = *override.Thinking
		}
	}

	return provider, model, reasoningEffort, thinking
}

// loadAndRenderPrompt loads the phase prompt template and renders it with variables.
func (r *Runner) loadAndRenderPrompt(phaseID string, tmpl *db.PhaseTemplate, vars variable.VariableSet) (string, error) {
	var promptContent string

	switch tmpl.PromptSource {
	case "embedded", "":
		// Load from embedded templates
		data, err := templates.Prompts.ReadFile(fmt.Sprintf("prompts/%s.md", phaseID))
		if err != nil {
			return "", fmt.Errorf("read embedded prompt %s: %w", phaseID, err)
		}
		promptContent = string(data)

	case "db":
		// Template content is stored in the database
		promptContent = tmpl.PromptContent

	case "file":
		// Load from file path
		data, err := os.ReadFile(tmpl.PromptPath)
		if err != nil {
			return "", fmt.Errorf("read prompt file %s: %w", tmpl.PromptPath, err)
		}
		promptContent = string(data)

	default:
		return "", fmt.Errorf("unknown prompt source %q for phase %s", tmpl.PromptSource, phaseID)
	}

	// Render variables
	rendered := variable.RenderTemplate(promptContent, vars)
	return rendered, nil
}

// buildBaseVars creates the initial variable set for a benchmark task.
func (r *Runner) buildBaseVars(project *Project, task *Task, workDir string) variable.VariableSet {
	vars := variable.VariableSet{
		"TASK_ID":          task.ID,
		"TASK_TITLE":       task.Title,
		"TASK_DESCRIPTION": task.Description,
		"TASK_CATEGORY":    task.Category,
		"LANGUAGE":         project.Language,
		"TEST_COMMAND":     project.TestCmd,
		"BUILD_COMMAND":    project.BuildCmd,
		"LINT_COMMAND":     project.LintCmd,
		"PROJECT_ROOT":     workDir,
		"WORKTREE_PATH":    workDir,
	}
	return vars
}

// loadWorkflowPhases loads the ordered phases for a workflow from GlobalDB.
func (r *Runner) loadWorkflowPhases(workflowID string) ([]*db.WorkflowPhase, error) {
	wf, err := r.globalDB.GetWorkflow(workflowID)
	if err != nil {
		return nil, fmt.Errorf("get workflow %s: %w", workflowID, err)
	}
	if wf == nil {
		return nil, fmt.Errorf("workflow %s not found", workflowID)
	}

	phases, err := r.globalDB.GetWorkflowPhases(workflowID)
	if err != nil {
		return nil, fmt.Errorf("get phases for workflow %s: %w", workflowID, err)
	}

	return phases, nil
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

var tierTimeout = map[Tier]time.Duration{
	TierTrivial: 40 * time.Minute,
	TierSmall:   45 * time.Minute,
	TierMedium:  60 * time.Minute,
	TierLarge:   90 * time.Minute,
}

// resolveWorkflow picks the workflow based on task tier, falling back
// to the variant's base_workflow if the tier isn't recognized.
func (r *Runner) resolveWorkflow(task *Task, variant *Variant) string {
	if wf, ok := tierToWorkflow[task.Tier]; ok {
		return wf
	}
	return variant.BaseWorkflow
}

// resolveOutputVarName determines the variable name for a phase's output.
func (r *Runner) resolveOutputVarName(phase *db.WorkflowPhase) string {
	// Try to get from phase template
	tmpl, err := r.globalDB.GetPhaseTemplate(phase.PhaseTemplateID)
	if err != nil {
		return "OUTPUT_" + strings.ToUpper(strings.ReplaceAll(phase.PhaseTemplateID, "-", "_"))
	}
	if tmpl.OutputVarName != "" {
		return tmpl.OutputVarName
	}
	return "OUTPUT_" + strings.ToUpper(strings.ReplaceAll(phase.PhaseTemplateID, "-", "_"))
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
func (r *Runner) failRun(ctx context.Context, run *Run, err error) error {
	run.Status = RunStatusError
	run.ErrorMessage = err.Error()
	run.CompletedAt = time.Now()
	if saveErr := r.store.SaveRun(ctx, run); saveErr != nil {
		r.logger.Error("failed to save error status for run", "run", run.ID, "error", saveErr)
	}
	return err
}

// captureModelDiff returns the git diff of all changes the model made
// relative to the pre-fix commit. Called before evaluation (which modifies
// test files) so we get the model's pure output.
func captureModelDiff(workDir, preFixCommit string) string {
	cmd := exec.Command("git", "diff", preFixCommit)
	cmd.Dir = workDir
	out, err := cmd.Output()
	if err != nil {
		return fmt.Sprintf("error capturing diff: %v", err)
	}
	return string(out)
}
