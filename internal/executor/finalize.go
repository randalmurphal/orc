// Package executor provides the finalize phase executor for orc.
// The finalize phase syncs the task branch with the target branch,
// resolves conflicts, runs tests, and prepares for merge.
package executor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
)

// FinalizeExecutor executes the finalize phase which prepares the task branch
// for merge by syncing with target branch, resolving conflicts, running tests,
// and performing risk assessment.
//
// Finalize Phase Steps:
// 1. Fetch latest target branch
// 2. Check divergence (commits ahead/behind)
// 3. Sync via merge or rebase (per config)
// 4. Detect and resolve conflicts (with Claude assistance)
// 5. Run full test suite
// 6. Perform risk assessment
// 7. Create finalization commit
//
// This executor supports retry/escalation back to the implement phase if
// issues persist beyond configured thresholds.
type FinalizeExecutor struct {
	claudePath       string // Path to claude binary
	gitSvc           *git.Git
	publisher        *events.PublishHelper
	logger           *slog.Logger
	config           ExecutorConfig
	orcConfig        *config.Config
	workingDir       string
	executionUpdater func(*orcv1.ExecutionState)
	backend          storage.Backend

	// turnExecutor allows injection of a mock for testing
	turnExecutor TurnExecutor
}

// FinalizeExecutorOption configures a FinalizeExecutor.
type FinalizeExecutorOption func(*FinalizeExecutor)

// WithFinalizeGitSvc sets the git service.
func WithFinalizeGitSvc(svc *git.Git) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.gitSvc = svc }
}

// WithFinalizePublisher sets the event publisher.
func WithFinalizePublisher(p events.Publisher) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.publisher = events.NewPublishHelper(p) }
}

// WithFinalizeLogger sets the logger.
func WithFinalizeLogger(l *slog.Logger) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.logger = l }
}

// WithFinalizeConfig sets the execution config.
func WithFinalizeConfig(cfg ExecutorConfig) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.config = cfg }
}

// WithFinalizeOrcConfig sets the orc configuration.
// Also connects to ExecutorConfig for model resolution.
func WithFinalizeOrcConfig(cfg *config.Config) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) {
		e.orcConfig = cfg
		e.config.OrcConfig = cfg // Connect for model resolution
	}
}

// WithFinalizeWorkingDir sets the working directory.
func WithFinalizeWorkingDir(dir string) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.workingDir = dir }
}

// WithFinalizeExecutionUpdater sets the execution state updater callback.
func WithFinalizeExecutionUpdater(fn func(*orcv1.ExecutionState)) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.executionUpdater = fn }
}

// WithFinalizeBackend sets the storage backend for initiative loading.
func WithFinalizeBackend(b storage.Backend) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.backend = b }
}

// WithFinalizeClaudePath sets the path to the claude binary.
func WithFinalizeClaudePath(path string) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.claudePath = path }
}

// WithFinalizeTurnExecutor sets a TurnExecutor for testing.
func WithFinalizeTurnExecutor(te TurnExecutor) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.turnExecutor = te }
}

// NewFinalizeExecutor creates a new finalize executor.
func NewFinalizeExecutor(opts ...FinalizeExecutorOption) *FinalizeExecutor {
	e := &FinalizeExecutor{
		claudePath: "claude",
		logger:     slog.Default(),
		publisher:  events.NewPublishHelper(nil),
		config: ExecutorConfig{
			MaxIterations:      10, // Lower for finalize - most work is git ops
			CheckpointInterval: 1,
			SessionPersistence: true,
		},
	}

	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}

	return e
}

// Name returns the executor type name.
func (e *FinalizeExecutor) Name() string {
	return "finalize"
}

// FinalizeResult contains the outcome of the finalize phase.
type FinalizeResult struct {
	// Synced indicates the branch was successfully synced with target
	Synced bool

	// ConflictsResolved is the number of conflicts that were resolved
	ConflictsResolved int

	// ConflictFiles lists files that had conflicts
	ConflictFiles []string

	// TestsPassed indicates all tests passed after sync
	TestsPassed bool

	// TestFailures contains test failure details if tests failed
	TestFailures []TestFailure

	// RiskLevel is the assessed merge risk (low, medium, high, critical)
	RiskLevel string

	// FilesChanged is the number of files changed vs target
	FilesChanged int

	// LinesChanged is the total lines added/removed vs target
	LinesChanged int

	// NeedsReview indicates if the changes require additional review
	NeedsReview bool

	// CommitSHA is the final commit SHA after finalization
	CommitSHA string
}

// Execute runs the finalize phase.
func (e *FinalizeExecutor) Execute(ctx context.Context, t *orcv1.Task, p *PhaseDisplay, exec *orcv1.ExecutionState) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: orcv1.PhaseStatus_PHASE_STATUS_PENDING,
	}

	e.publisher.PhaseStart(t.Id, p.ID)

	// Get finalize configuration
	finalizeCfg := e.getFinalizeConfig()
	if !finalizeCfg.Enabled {
		e.logger.Info("finalize phase disabled, skipping", "task", t.Id)
		result.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED
		result.Duration = time.Since(start)
		return result, nil
	}

	targetBranch := ResolveTargetBranchForTask(t, e.backend, e.orcConfig)

	e.logger.Info("starting finalize phase",
		"task", t.Id,
		"target_branch", targetBranch,
		"sync_strategy", finalizeCfg.Sync.Strategy,
	)

	// Step 1: Fetch latest target branch
	e.publishProgress(t.Id, p.ID, "Fetching latest changes from remote...")
	if err := e.fetchTarget(); err != nil {
		result.Error = fmt.Errorf("fetch target: %w", err)
		result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
		result.Duration = time.Since(start)
		return result, result.Error
	}

	// Step 2: Check divergence
	e.publishProgress(t.Id, p.ID, "Checking branch divergence...")
	ahead, behind, err := e.checkDivergence(targetBranch)
	if err != nil {
		result.Error = fmt.Errorf("check divergence: %w", err)
		result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
		result.Duration = time.Since(start)
		return result, result.Error
	}

	e.logger.Info("branch divergence",
		"commits_ahead", ahead,
		"commits_behind", behind,
	)

	// If already up-to-date, skip sync
	if behind == 0 {
		e.logger.Info("branch already up-to-date with target")
		e.publishProgress(t.Id, p.ID, "Branch already up-to-date with target branch")
	} else {
		// Step 3: Sync with target branch
		e.publishProgress(t.Id, p.ID, fmt.Sprintf("Syncing with %s (%d commits behind)...", targetBranch, behind))
		finalizeResult, err := e.syncWithTarget(ctx, t, p, exec, targetBranch, finalizeCfg)
		if err != nil {
			// Check if we should escalate to implement phase
			if e.shouldEscalate(finalizeResult, finalizeCfg) {
				result.Error = fmt.Errorf("finalize failed, needs escalation to implement phase: %w", err)
				result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
				result.Output = buildEscalationContext(finalizeResult)
			} else {
				result.Error = fmt.Errorf("sync with target: %w", err)
				result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
			}
			result.Duration = time.Since(start)
			return result, result.Error
		}

		// Step 4: Run tests after sync
		e.publishProgress(t.Id, p.ID, "Running test suite after sync...")
		testResult, err := e.runTests(ctx, t, finalizeCfg)
		if err != nil {
			finalizeResult.TestsPassed = false
			finalizeResult.TestFailures = testResult.Failures
			e.logger.Error("tests failed after sync",
				"failed", testResult.Failed,
				"passed", testResult.Passed,
			)

			// Try to fix tests using Claude
			if finalizeCfg.ConflictResolution.Enabled {
				e.publishProgress(t.Id, p.ID, "Tests failed, attempting to fix...")
				fixed, fixErr := e.tryFixTests(ctx, t, p, exec, testResult)
				if fixErr != nil || !fixed {
					result.Error = fmt.Errorf("tests failed after sync and fix attempt: %v failures", len(testResult.Failures))
					result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
					result.Output = buildTestFailureContext(testResult)
					result.Duration = time.Since(start)
					return result, result.Error
				}
			} else {
				result.Error = fmt.Errorf("tests failed after sync: %v failures", len(testResult.Failures))
				result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING
				result.Output = buildTestFailureContext(testResult)
				result.Duration = time.Since(start)
				return result, result.Error
			}
		}
		finalizeResult.TestsPassed = true

		// Step 5: Risk assessment
		e.publishProgress(t.Id, p.ID, "Performing risk assessment...")
		if err := e.assessRisk(finalizeResult, targetBranch, finalizeCfg); err != nil {
			e.logger.Warn("risk assessment failed", "error", err)
		}

		// Check if re-review is needed
		if finalizeResult.NeedsReview {
			e.logger.Warn("changes require additional review",
				"risk_level", finalizeResult.RiskLevel,
				"files_changed", finalizeResult.FilesChanged,
				"conflicts_resolved", finalizeResult.ConflictsResolved,
			)
		}

		// Step 6: Create finalization commit
		e.publishProgress(t.Id, p.ID, "Creating finalization commit...")
		commitSHA, err := e.createFinalizeCommit(t, finalizeResult)
		if err != nil {
			e.logger.Warn("failed to create finalize commit", "error", err)
		} else {
			finalizeResult.CommitSHA = commitSHA
			result.CommitSHA = commitSHA
		}

		// Build result output
		result.Output = buildFinalizeReport(t.Id, targetBranch, finalizeResult)
	}

	result.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED
	result.Duration = time.Since(start)

	e.logger.Info("finalize phase complete",
		"task", t.Id,
		"duration", result.Duration,
	)

	// Note: Finalize report is logged but not saved to phase_outputs
	// since finalize runs outside the normal workflow run context

	return result, nil
}

// getFinalizeConfig returns the finalize configuration with defaults.
func (e *FinalizeExecutor) getFinalizeConfig() config.FinalizeConfig {
	if e.orcConfig == nil {
		return config.FinalizeConfig{
			Enabled:     true,
			AutoTrigger: true,
			Sync: config.FinalizeSyncConfig{
				Strategy: config.FinalizeSyncMerge,
			},
			ConflictResolution: config.ConflictResolutionConfig{
				Enabled: true,
			},
			RiskAssessment: config.RiskAssessmentConfig{
				Enabled:           true,
				ReReviewThreshold: "high",
			},
			Gates: config.FinalizeGatesConfig{
				PreMerge: "auto",
			},
		}
	}
	return e.orcConfig.Completion.Finalize
}

// getTargetBranch returns the target branch for merging.
func (e *FinalizeExecutor) getTargetBranch() string {
	if e.orcConfig != nil && e.orcConfig.Completion.TargetBranch != "" {
		return e.orcConfig.Completion.TargetBranch
	}
	if e.config.TargetBranch != "" {
		return e.config.TargetBranch
	}
	return "main"
}

// fetchTarget fetches the latest changes from remote.
func (e *FinalizeExecutor) fetchTarget() error {
	if e.gitSvc == nil {
		return fmt.Errorf("git service not available")
	}
	return e.gitSvc.Fetch("origin")
}

// checkDivergence returns the number of commits ahead and behind target.
func (e *FinalizeExecutor) checkDivergence(targetBranch string) (ahead int, behind int, err error) {
	if e.gitSvc == nil {
		return 0, 0, fmt.Errorf("git service not available")
	}

	target := "origin/" + targetBranch
	result, err := e.gitSvc.DetectConflicts(target)
	if err != nil {
		return 0, 0, err
	}

	return result.CommitsAhead, result.CommitsBehind, nil
}

// syncWithTarget syncs the task branch with the target branch.
func (e *FinalizeExecutor) syncWithTarget(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	targetBranch string,
	cfg config.FinalizeConfig,
) (*FinalizeResult, error) {
	result := &FinalizeResult{}

	if e.gitSvc == nil {
		return result, fmt.Errorf("git service not available")
	}

	target := "origin/" + targetBranch

	// Choose sync strategy
	var syncResult *FinalizeResult
	var syncErr error
	switch cfg.Sync.Strategy {
	case config.FinalizeSyncRebase:
		syncResult, syncErr = e.syncViaRebase(ctx, t, p, exec, target, cfg, result)
	case config.FinalizeSyncMerge:
		syncResult, syncErr = e.syncViaMerge(ctx, t, p, exec, target, cfg, result)
	default:
		// Default to merge
		syncResult, syncErr = e.syncViaMerge(ctx, t, p, exec, target, cfg, result)
	}

	// If sync failed, return the error
	if syncErr != nil {
		return syncResult, syncErr
	}

	// Restore .orc/ from target branch to prevent worktree contamination
	// This ensures any modifications to .orc/ during task execution don't get merged
	if syncResult.Synced {
		restored, restoreErr := e.gitSvc.RestoreOrcDir(target, t.Id)
		if restoreErr != nil {
			e.logger.Warn("failed to restore .orc/ directory", "error", restoreErr)
			// Don't fail the sync - restoration is defense-in-depth
		} else if restored {
			e.logger.Info("restored .orc/ from target branch",
				"target", targetBranch,
				"reason", "prevent worktree contamination")
		}

		// Restore .claude/settings.json to prevent worktree isolation hooks from being merged
		// Worktrees inject hooks with machine-specific paths that shouldn't be shared
		restoredSettings, restoreErr := e.gitSvc.RestoreClaudeSettings(target, t.Id)
		if restoreErr != nil {
			e.logger.Warn("failed to restore .claude/settings.json", "error", restoreErr)
		} else if restoredSettings {
			e.logger.Info("restored .claude/settings.json from target branch",
				"target", targetBranch,
				"reason", "prevent worktree hooks from being merged")
		}
	}

	return syncResult, nil
}

// syncViaMerge syncs by merging target into the task branch.
func (e *FinalizeExecutor) syncViaMerge(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	target string,
	cfg config.FinalizeConfig,
	result *FinalizeResult,
) (*FinalizeResult, error) {
	// Attempt merge
	err := e.gitSvc.Merge(target, true) // --no-ff for clear merge commits
	if err == nil {
		result.Synced = true
		return result, nil
	}

	// Check for merge conflicts
	if !strings.Contains(err.Error(), "CONFLICT") && !strings.Contains(err.Error(), "conflict") {
		return result, fmt.Errorf("merge failed: %w", err)
	}

	// Detect conflict files
	syncResult, detectErr := e.gitSvc.DetectConflicts(target)
	if detectErr == nil && syncResult.ConflictsDetected {
		result.ConflictFiles = syncResult.ConflictFiles
	}

	// If conflict resolution is enabled, try to resolve
	if cfg.ConflictResolution.Enabled && len(result.ConflictFiles) > 0 {
		e.logger.Info("conflicts detected, attempting resolution",
			"files", result.ConflictFiles,
		)

		// First, try auto-resolution for known patterns (CLAUDE.md knowledge tables)
		autoResolved, remaining, autoLogs := e.gitSvc.AutoResolveConflicts(result.ConflictFiles, e.logger)
		for _, log := range autoLogs {
			e.logger.Debug("auto-resolve", "msg", log)
		}

		if len(autoResolved) > 0 {
			e.logger.Info("auto-resolved conflicts",
				"files", autoResolved,
				"remaining", remaining,
			)
		}

		// If all conflicts were auto-resolved, we're done
		if len(remaining) == 0 {
			// Verify no unmerged files remain
			unmerged, _ := e.gitSvc.Context().RunGit("diff", "--name-only", "--diff-filter=U")
			if strings.TrimSpace(unmerged) == "" {
				// Commit the merge
				_, commitErr := e.gitSvc.Context().RunGit("commit", "--no-edit")
				if commitErr == nil {
					result.ConflictsResolved = len(result.ConflictFiles)
					result.Synced = true
					e.logger.Info("all conflicts auto-resolved successfully")
					return result, nil
				}
			}
		}

		// Fall back to Claude for remaining conflicts
		if len(remaining) > 0 {
			resolved, resolveErr := e.resolveConflicts(ctx, t, p, exec, remaining, cfg)
			if resolveErr != nil {
				// Abort merge on failure
				_, _ = e.gitSvc.Context().RunGit("merge", "--abort")
				return result, fmt.Errorf("conflict resolution failed: %w", resolveErr)
			}

			if resolved {
				result.ConflictsResolved = len(result.ConflictFiles)
				result.Synced = true
				return result, nil
			}
		}
	}

	// Abort merge if we couldn't resolve
	_, _ = e.gitSvc.Context().RunGit("merge", "--abort")
	return result, fmt.Errorf("merge conflicts could not be resolved: %v", result.ConflictFiles)
}

// syncViaRebase syncs by rebasing onto the target branch.
func (e *FinalizeExecutor) syncViaRebase(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	target string,
	cfg config.FinalizeConfig,
	result *FinalizeResult,
) (*FinalizeResult, error) {
	// Attempt rebase with conflict check
	syncResult, err := e.gitSvc.RebaseWithConflictCheck(target)
	if err == nil {
		result.Synced = true
		return result, nil
	}

	// If conflicts detected and resolution enabled
	if errors.Is(err, git.ErrMergeConflict) && cfg.ConflictResolution.Enabled {
		result.ConflictFiles = syncResult.ConflictFiles

		e.logger.Info("rebase conflicts detected, attempting resolution",
			"files", result.ConflictFiles,
		)

		// Note: For rebase, each commit may have conflicts
		// We'll try to resolve them one by one
		resolved, resolveErr := e.resolveRebaseConflicts(ctx, t, p, exec, result.ConflictFiles, cfg)
		if resolveErr != nil {
			return result, fmt.Errorf("rebase conflict resolution failed: %w", resolveErr)
		}

		if resolved {
			result.ConflictsResolved = len(result.ConflictFiles)
			result.Synced = true
			return result, nil
		}
	}

	return result, fmt.Errorf("rebase failed: %w", err)
}

// resolveConflicts uses Claude to resolve merge conflicts.
func (e *FinalizeExecutor) resolveConflicts(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	conflictFiles []string,
	cfg config.FinalizeConfig,
) (bool, error) {
	// Build conflict resolution prompt
	prompt := buildConflictResolutionPrompt(t, conflictFiles, cfg)

	// Use config default model (finalize doesn't have a phase template)
	model := e.config.Model
	if model == "" {
		model = "opus"
	}

	// Use injected turnExecutor if available, otherwise create ClaudeExecutor
	// Transcript storage is handled internally by ClaudeExecutor when backend is provided
	var turnExec TurnExecutor
	sessionID := fmt.Sprintf("%s-conflict-resolution", t.Id)
	if e.turnExecutor != nil {
		turnExec = e.turnExecutor
	} else {
		claudeOpts := []ClaudeExecutorOption{
			WithClaudePath(e.claudePath),
			WithClaudeWorkdir(e.workingDir),
			WithClaudeModel(model),
			WithClaudeSessionID(sessionID),
			WithClaudeMaxTurns(5), // Limited turns for conflict resolution
			WithClaudeLogger(e.logger),
			WithClaudePhaseID(p.ID),
			// Transcript storage options - handled internally
			WithClaudeBackend(e.backend),
			WithClaudeTaskID(t.Id),
		}
		turnExec = NewClaudeExecutor(claudeOpts...)
	}

	// Execute conflict resolution (without JSON schema - freeform response)
	_, execErr := turnExec.ExecuteTurn(ctx, prompt)
	if execErr != nil {
		return false, fmt.Errorf("conflict resolution turn: %w", execErr)
	}

	// Verify no unmerged files remain (Claude should have resolved them)
	unmerged, _ := e.gitSvc.Context().RunGit("diff", "--name-only", "--diff-filter=U")
	if strings.TrimSpace(unmerged) == "" {
		// All conflicts resolved, commit the merge
		_, commitErr := e.gitSvc.Context().RunGit("commit", "--no-edit")
		return commitErr == nil, commitErr
	}

	return false, fmt.Errorf("conflict resolution incomplete: unmerged files remain")
}

// resolveRebaseConflicts resolves conflicts during rebase.
func (e *FinalizeExecutor) resolveRebaseConflicts(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	conflictFiles []string,
	cfg config.FinalizeConfig,
) (bool, error) {
	// For rebase, we need to handle conflicts commit by commit
	maxAttempts := 10 // Maximum rebase continue attempts

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Check for conflicts
		unmerged, _ := e.gitSvc.Context().RunGit("diff", "--name-only", "--diff-filter=U")
		unmergedFiles := strings.Split(strings.TrimSpace(unmerged), "\n")
		if len(unmergedFiles) == 0 || (len(unmergedFiles) == 1 && unmergedFiles[0] == "") {
			// No more conflicts, continue rebase
			_, err := e.gitSvc.Context().RunGit("rebase", "--continue")
			if err == nil {
				return true, nil
			}
			// Check if rebase is complete
			if strings.Contains(err.Error(), "No rebase in progress") {
				return true, nil
			}
			continue
		}

		// First, try auto-resolution for known patterns (CLAUDE.md knowledge tables)
		autoResolved, remaining, autoLogs := e.gitSvc.AutoResolveConflicts(unmergedFiles, e.logger)
		for _, log := range autoLogs {
			e.logger.Debug("auto-resolve during rebase", "msg", log)
		}

		if len(autoResolved) > 0 {
			e.logger.Info("auto-resolved rebase conflicts",
				"files", autoResolved,
				"remaining", remaining,
			)
		}

		// Resolve remaining conflicts with Claude
		if len(remaining) > 0 {
			resolved, err := e.resolveConflicts(ctx, t, p, exec, remaining, cfg)
			if err != nil || !resolved {
				// Abort rebase
				_ = e.gitSvc.AbortRebase()
				return false, fmt.Errorf("failed to resolve rebase conflict at attempt %d: %w", attempt, err)
			}
		}

		// Stage all resolved files and continue
		for _, f := range unmergedFiles {
			_, _ = e.gitSvc.Context().RunGit("add", f)
		}

		_, continueErr := e.gitSvc.Context().RunGit("rebase", "--continue")
		if continueErr == nil || strings.Contains(continueErr.Error(), "No rebase in progress") {
			return true, nil
		}
	}

	// Max attempts reached
	_ = e.gitSvc.AbortRebase()
	return false, fmt.Errorf("rebase conflict resolution exceeded max attempts")
}

// runTests runs the test suite after sync.
func (e *FinalizeExecutor) runTests(ctx context.Context, t *orcv1.Task, cfg config.FinalizeConfig) (*ParsedTestResult, error) {
	// Get test command from config
	testCmd := "go test ./... -v -race"
	if e.orcConfig != nil && e.orcConfig.Testing.Commands.Unit != "" {
		testCmd = e.orcConfig.Testing.Commands.Unit
	}

	e.logger.Info("running tests", "command", testCmd)

	// Run tests
	workDir := e.workingDir
	if workDir == "" {
		return nil, fmt.Errorf("executor workingDir not set: cannot run tests safely")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", testCmd)
	cmd.Dir = workDir
	// Set GOWORK=off to avoid go.work issues in worktrees
	cmd.Env = append(os.Environ(), "GOWORK=off")

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Parse test output
	result, parseErr := ParseTestOutput(outputStr)
	if parseErr != nil {
		e.logger.Warn("failed to parse test output", "error", parseErr)
		result = &ParsedTestResult{
			Framework: "unknown",
		}
	}

	// Determine if tests passed
	if err != nil || result.Failed > 0 {
		return result, fmt.Errorf("tests failed: %d failures", result.Failed)
	}

	e.logger.Info("tests passed",
		"passed", result.Passed,
		"coverage", result.Coverage,
	)

	return result, nil
}

// tryFixTests attempts to fix test failures using Claude.
func (e *FinalizeExecutor) tryFixTests(
	ctx context.Context,
	t *orcv1.Task,
	p *PhaseDisplay,
	exec *orcv1.ExecutionState,
	testResult *ParsedTestResult,
) (bool, error) {
	// Build fix prompt
	prompt := buildTestFixPrompt(t, testResult)

	// Use config default model (finalize doesn't have a phase template)
	model := e.config.Model
	if model == "" {
		model = "opus"
	}

	// Use injected turnExecutor if available, otherwise create ClaudeExecutor
	// Transcript storage is handled internally by ClaudeExecutor when backend is provided
	var turnExec TurnExecutor
	sessionID := fmt.Sprintf("%s-test-fix", t.Id)
	if e.turnExecutor != nil {
		turnExec = e.turnExecutor
	} else {
		claudeOpts := []ClaudeExecutorOption{
			WithClaudePath(e.claudePath),
			WithClaudeWorkdir(e.workingDir),
			WithClaudeModel(model),
			WithClaudeSessionID(sessionID),
			WithClaudeMaxTurns(5),
			WithClaudeLogger(e.logger),
			WithClaudePhaseID(p.ID),
			// Transcript storage options - handled internally
			WithClaudeBackend(e.backend),
			WithClaudeTaskID(t.Id),
		}
		turnExec = NewClaudeExecutor(claudeOpts...)
	}

	// Execute test fix (without JSON schema - freeform response)
	_, err := turnExec.ExecuteTurn(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("test fix turn: %w", err)
	}

	// Re-run tests to verify fix
	cfg := e.getFinalizeConfig()
	newResult, testErr := e.runTests(ctx, t, cfg)
	if testErr != nil || newResult.Failed > 0 {
		return false, fmt.Errorf("tests still failing after fix: %d failures", newResult.Failed)
	}

	return true, nil
}

// assessRisk performs risk assessment for the changes.
func (e *FinalizeExecutor) assessRisk(result *FinalizeResult, targetBranch string, cfg config.FinalizeConfig) error {
	if !cfg.RiskAssessment.Enabled {
		result.RiskLevel = "unknown"
		return nil
	}

	if e.gitSvc == nil {
		return fmt.Errorf("git service not available")
	}

	target := "origin/" + targetBranch

	// Get diff stats
	diffStat, err := e.gitSvc.Context().RunGit("diff", "--stat", target+"...HEAD")
	if err != nil {
		return fmt.Errorf("get diff stat: %w", err)
	}

	// Parse file count from last line
	lines := strings.Split(strings.TrimSpace(diffStat), "\n")
	if len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		// Parse "X files changed, Y insertions(+), Z deletions(-)"
		result.FilesChanged = parseFileCount(lastLine)
	}

	// Get line count
	numstat, err := e.gitSvc.Context().RunGit("diff", "--numstat", target+"...HEAD")
	if err == nil {
		result.LinesChanged = parseTotalLines(numstat)
	}

	// Classify risk level
	result.RiskLevel = classifyRisk(result.FilesChanged, result.LinesChanged, result.ConflictsResolved)

	// Check if re-review is needed
	threshold := cfg.RiskAssessment.ReReviewThreshold
	if threshold == "" {
		threshold = "high"
	}

	result.NeedsReview = shouldTriggerReview(result.RiskLevel, threshold)

	e.logger.Info("risk assessment complete",
		"risk_level", result.RiskLevel,
		"files_changed", result.FilesChanged,
		"lines_changed", result.LinesChanged,
		"needs_review", result.NeedsReview,
	)

	return nil
}

// createFinalizeCommit creates a commit documenting the finalization.
func (e *FinalizeExecutor) createFinalizeCommit(t *orcv1.Task, result *FinalizeResult) (string, error) {
	if e.gitSvc == nil {
		return "", fmt.Errorf("git service not available")
	}

	// Check if there are changes to commit
	clean, err := e.gitSvc.IsClean()
	if err != nil {
		return "", fmt.Errorf("check clean: %w", err)
	}

	if clean {
		// No changes, get current HEAD as commit SHA
		sha, err := e.gitSvc.Context().HeadCommit()
		return sha, err
	}

	// Build commit message
	msg := fmt.Sprintf("[orc] %s: finalize - completed\n\nPhase: finalize\nStatus: completed\nConflicts resolved: %d\nRisk level: %s\nReady for merge: YES",
		t.Id,
		result.ConflictsResolved,
		result.RiskLevel,
	)

	checkpoint, err := e.gitSvc.CreateCheckpoint(t.Id, "finalize", "completed")
	if err != nil {
		// Try direct commit as fallback
		if err := e.gitSvc.Context().StageAll(); err != nil {
			return "", fmt.Errorf("stage all: %w", err)
		}
		if err := e.gitSvc.Context().Commit(msg); err != nil {
			return "", fmt.Errorf("commit: %w", err)
		}
		sha, _ := e.gitSvc.Context().HeadCommit()
		return sha, nil
	}

	return checkpoint.CommitSHA, nil
}

// shouldEscalate determines if the finalize failure should trigger escalation.
func (e *FinalizeExecutor) shouldEscalate(result *FinalizeResult, cfg config.FinalizeConfig) bool {
	if result == nil {
		return false
	}

	// Escalate if too many conflicts couldn't be resolved
	if len(result.ConflictFiles) > 10 {
		return true
	}

	// Escalate if tests consistently fail
	if !result.TestsPassed && len(result.TestFailures) > 5 {
		return true
	}

	return false
}

// publishProgress publishes a progress message for the finalize phase.
func (e *FinalizeExecutor) publishProgress(taskID, phaseID, message string) {
	e.publisher.Transcript(taskID, phaseID, 0, "progress", message)
}

// Helper functions

// buildConflictResolutionPrompt creates the prompt for conflict resolution.
func buildConflictResolutionPrompt(t *orcv1.Task, conflictFiles []string, cfg config.FinalizeConfig) string {
	conflictCfg := cfg.ConflictResolution
	var sb strings.Builder

	sb.WriteString("# Conflict Resolution Task\n\n")
	sb.WriteString("You are resolving merge conflicts for task: ")
	sb.WriteString(t.Id)
	sb.WriteString(" - ")
	sb.WriteString(t.Title)
	sb.WriteString("\n\n")

	sb.WriteString("## Conflicted Files\n\n")
	for _, f := range conflictFiles {
		sb.WriteString("- `")
		sb.WriteString(f)
		sb.WriteString("`\n")
	}

	sb.WriteString("\n## Conflict Resolution Rules\n\n")
	sb.WriteString("**CRITICAL - You MUST follow these rules:**\n\n")
	sb.WriteString("1. **NEVER remove features** - Both your changes AND upstream changes must be preserved\n")
	sb.WriteString("2. **Merge intentions, not text** - Understand what each side was trying to accomplish\n")
	sb.WriteString("3. **Prefer additive resolution** - If in doubt, keep both implementations\n")
	sb.WriteString("4. **Test after every file** - Don't batch conflict resolutions\n\n")

	sb.WriteString("## Prohibited Resolutions\n\n")
	sb.WriteString("- **NEVER**: Just take \"ours\" or \"theirs\" without understanding\n")
	sb.WriteString("- **NEVER**: Remove upstream features to fix conflicts\n")
	sb.WriteString("- **NEVER**: Remove your features to fix conflicts\n")
	sb.WriteString("- **NEVER**: Comment out conflicting code\n\n")

	// Add custom instructions if provided
	if conflictCfg.Instructions != "" {
		sb.WriteString("## Additional Instructions\n\n")
		sb.WriteString(conflictCfg.Instructions)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. For each conflicted file, read and understand both sides of the conflict\n")
	sb.WriteString("2. Resolve the conflict by merging both changes appropriately\n")
	sb.WriteString("3. Stage the resolved file with `git add <file>`\n")
	sb.WriteString("4. After all files are resolved, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "complete", "summary": "Resolved X conflicts in files A, B, C"}`)
	sb.WriteString("\n\nIf you cannot resolve a conflict, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "blocked", "reason": "[explanation]"}`)
	sb.WriteString("\n")

	return sb.String()
}

// buildTestFixPrompt creates the prompt for fixing test failures.
func buildTestFixPrompt(t *orcv1.Task, testResult *ParsedTestResult) string {
	var sb strings.Builder

	sb.WriteString("# Test Failure Fix Task\n\n")
	sb.WriteString("You are fixing test failures for task: ")
	sb.WriteString(t.Id)
	sb.WriteString(" - ")
	sb.WriteString(t.Title)
	sb.WriteString("\n\n")

	sb.WriteString("## Test Failures\n\n")
	for i, f := range testResult.Failures {
		if i >= 5 {
			sb.WriteString(fmt.Sprintf("... and %d more failures\n", len(testResult.Failures)-5))
			break
		}
		sb.WriteString(fmt.Sprintf("### %s\n", f.Test))
		if f.File != "" {
			sb.WriteString(fmt.Sprintf("**File**: `%s:%d`\n", f.File, f.Line))
		}
		if f.Message != "" {
			sb.WriteString(fmt.Sprintf("**Error**: %s\n", f.Message))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("1. Analyze each failing test\n")
	sb.WriteString("2. Fix the code or test as appropriate\n")
	sb.WriteString("3. The fix should preserve all intended functionality\n")
	sb.WriteString("4. Do NOT remove tests to fix failures\n")
	sb.WriteString("5. When done, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "complete", "summary": "Fixed X test failures"}`)
	sb.WriteString("\n\nIf you cannot fix the tests, output ONLY this JSON:\n")
	sb.WriteString(`{"status": "blocked", "reason": "[explanation]"}`)
	sb.WriteString("\n")

	return sb.String()
}

// buildTestFailureContext creates context for test failure escalation.
func buildTestFailureContext(testResult *ParsedTestResult) string {
	if testResult == nil {
		return "Tests failed with unknown results"
	}
	return BuildTestRetryContext("finalize", testResult)
}

// buildEscalationContext creates context for escalation to implement phase.
func buildEscalationContext(result *FinalizeResult) string {
	if result == nil {
		return "Finalize phase failed and requires escalation to implement phase"
	}

	var sb strings.Builder
	sb.WriteString("## Finalize Escalation Required\n\n")
	sb.WriteString("The finalize phase encountered issues that require revisiting implementation:\n\n")

	if len(result.ConflictFiles) > 0 {
		sb.WriteString("### Unresolved Conflicts\n\n")
		for _, f := range result.ConflictFiles {
			sb.WriteString("- `")
			sb.WriteString(f)
			sb.WriteString("`\n")
		}
		sb.WriteString("\n")
	}

	if !result.TestsPassed && len(result.TestFailures) > 0 {
		sb.WriteString("### Test Failures\n\n")
		for i, f := range result.TestFailures {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("... and %d more failures\n", len(result.TestFailures)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s: %s\n", f.Test, f.Message))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Please review and fix these issues in the implement phase, then retry finalize.\n")

	return sb.String()
}

// buildFinalizeReport creates the finalization report output.
func buildFinalizeReport(taskID, targetBranch string, result *FinalizeResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Finalization Report: %s\n\n", taskID))

	sb.WriteString("## Sync Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Target Branch | %s |\n", targetBranch))
	sb.WriteString(fmt.Sprintf("| Conflicts Resolved | %d |\n", result.ConflictsResolved))
	sb.WriteString(fmt.Sprintf("| Files Changed (total) | %d |\n", result.FilesChanged))
	sb.WriteString(fmt.Sprintf("| Lines Changed (total) | %d |\n", result.LinesChanged))
	sb.WriteString("\n")

	if len(result.ConflictFiles) > 0 {
		sb.WriteString("## Conflict Resolution\n\n")
		sb.WriteString("| File | Status |\n")
		sb.WriteString("|------|--------|\n")
		for _, f := range result.ConflictFiles {
			sb.WriteString(fmt.Sprintf("| `%s` | ✓ Resolved |\n", f))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Test Results\n\n")
	if result.TestsPassed {
		sb.WriteString("✓ All tests passed\n\n")
	} else {
		sb.WriteString("✗ Tests failed\n\n")
	}

	sb.WriteString("## Risk Assessment\n\n")
	sb.WriteString("| Factor | Value | Risk |\n")
	sb.WriteString("|--------|-------|------|\n")
	sb.WriteString(fmt.Sprintf("| Files Changed | %d | %s |\n",
		result.FilesChanged, classifyFileRisk(result.FilesChanged)))
	sb.WriteString(fmt.Sprintf("| Lines Changed | %d | %s |\n",
		result.LinesChanged, classifyLineRisk(result.LinesChanged)))
	sb.WriteString(fmt.Sprintf("| Conflicts Resolved | %d | %s |\n",
		result.ConflictsResolved, classifyConflictRisk(result.ConflictsResolved)))
	sb.WriteString(fmt.Sprintf("| **Overall Risk** | | **%s** |\n", result.RiskLevel))
	sb.WriteString("\n")

	sb.WriteString("## Merge Decision\n\n")
	if result.NeedsReview {
		sb.WriteString("**Ready for Merge**: NO - Review Required\n")
		sb.WriteString("**Recommended Action**: review-then-merge\n")
	} else if result.RiskLevel == "critical" {
		sb.WriteString("**Ready for Merge**: NO - Senior Review Required\n")
		sb.WriteString("**Recommended Action**: senior-review-required\n")
	} else {
		sb.WriteString("**Ready for Merge**: YES\n")
		sb.WriteString("**Recommended Action**: auto-merge\n")
	}

	if result.CommitSHA != "" {
		sb.WriteString(fmt.Sprintf("\n**Commit**: %s\n", result.CommitSHA))
	}

	sb.WriteString("\n")
	sb.WriteString(`{"status": "complete", "summary": "Finalization complete"}`)
	sb.WriteString("\n")

	return sb.String()
}

// parseFileCount extracts file count from git diff --stat last line.
func parseFileCount(line string) int {
	// Format: "X files changed, Y insertions(+), Z deletions(-)"
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		count, _ := strconv.Atoi(parts[0])
		return count
	}
	return 0
}

// parseTotalLines calculates total lines from git diff --numstat.
func parseTotalLines(numstat string) int {
	total := 0
	for _, line := range strings.Split(numstat, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			added, _ := strconv.Atoi(parts[0])
			removed, _ := strconv.Atoi(parts[1])
			total += added + removed
		}
	}
	return total
}

// classifyRisk determines the overall risk level.
func classifyRisk(files, lines, conflicts int) string {
	// Critical: >30 files OR >1000 lines OR >10 conflicts
	if files > 30 || lines > 1000 || conflicts > 10 {
		return "critical"
	}

	// High: 16-30 files OR 500-1000 lines OR 4-10 conflicts
	if files > 15 || lines > 500 || conflicts > 3 {
		return "high"
	}

	// Medium: 6-15 files OR 100-500 lines OR 1-3 conflicts
	if files > 5 || lines > 100 || conflicts > 0 {
		return "medium"
	}

	// Low: 1-5 files AND <100 lines AND 0 conflicts
	return "low"
}

// classifyFileRisk returns risk level based on file count.
func classifyFileRisk(files int) string {
	if files > 30 {
		return "Critical"
	}
	if files > 15 {
		return "High"
	}
	if files > 5 {
		return "Medium"
	}
	return "Low"
}

// classifyLineRisk returns risk level based on line count.
func classifyLineRisk(lines int) string {
	if lines > 1000 {
		return "Critical"
	}
	if lines > 500 {
		return "High"
	}
	if lines > 100 {
		return "Medium"
	}
	return "Low"
}

// classifyConflictRisk returns risk level based on conflict count.
func classifyConflictRisk(conflicts int) string {
	if conflicts > 10 {
		return "High"
	}
	if conflicts > 3 {
		return "Medium"
	}
	if conflicts > 0 {
		return "Low"
	}
	return "None"
}

// shouldTriggerReview determines if review should be triggered based on risk.
func shouldTriggerReview(riskLevel, threshold string) bool {
	riskOrder := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	riskVal := riskOrder[riskLevel]
	thresholdVal := riskOrder[threshold]

	return riskVal >= thresholdVal
}
