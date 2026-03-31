// Package executor provides the finalize phase executor for orc.
// The finalize phase syncs the task branch with the target branch,
// resolves conflicts, runs tests, and prepares for merge.
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
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
	globalDB         *db.GlobalDB // For loading workflows during target branch resolution

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

// WithFinalizeGlobalDB sets the global database for workflow loading during target branch resolution.
func WithFinalizeGlobalDB(gdb *db.GlobalDB) FinalizeExecutorOption {
	return func(e *FinalizeExecutor) { e.globalDB = gdb }
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
			MaxTurns:           10, // Lower for finalize - most work is git ops
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

	targetBranch := ResolveTargetBranchWithGlobalDB(t, e.backend, e.globalDB, e.orcConfig)

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
