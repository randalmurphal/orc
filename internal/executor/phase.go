// Package executor provides the flowgraph-based execution engine for orc.
// This file contains phase execution methods for the Executor type.
package executor

import (
	"context"
	"fmt"
	"time"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// ExecutePhase runs a single phase using either session-based or flowgraph execution.
func (e *Executor) ExecutePhase(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	// Use session-based execution if enabled
	if e.useSessionExecution {
		return e.executePhaseWithSession(ctx, t, p, s)
	}

	// Fall back to legacy flowgraph-based execution
	return e.executePhaseWithFlowgraph(ctx, t, p, s)
}

// executePhaseWithSession runs a phase using session-based execution.
// This provides context continuity via Claude's native session management.
func (e *Executor) executePhaseWithSession(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	// Get the appropriate executor for this task's weight
	executor := e.getPhaseExecutor(t.Weight)

	e.logger.Info("executing phase with session",
		"phase", p.ID,
		"task", t.ID,
		"weight", t.Weight,
		"executor", executor.Name(),
	)

	// Delegate to the weight-appropriate executor
	return executor.Execute(ctx, t, p, s)
}

// executePhaseWithFlowgraph runs a phase using the legacy flowgraph-based execution.
func (e *Executor) executePhaseWithFlowgraph(ctx context.Context, t *task.Task, p *plan.Phase, s *state.State) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: plan.PhaseRunning,
	}

	// Build phase graph
	graph := flowgraph.NewGraph[PhaseState]()

	// Add nodes
	graph.AddNode("prompt", e.buildPromptNode(p))
	graph.AddNode("execute", e.executeClaudeNode())
	graph.AddNode("check", e.checkCompletionNode(p, s))
	graph.AddNode("commit", e.commitCheckpointNode())

	// Set up edges - Ralph-style loop
	graph.SetEntry("prompt")
	graph.AddEdge("prompt", "execute")
	graph.AddEdge("execute", "check")

	maxIter := e.config.MaxIterations
	if p.Config != nil {
		if mi, ok := p.Config["max_iterations"].(int); ok {
			maxIter = mi
		}
	}

	graph.AddConditionalEdge("check", func(ctx flowgraph.Context, ps PhaseState) string {
		if ps.Complete {
			return "commit"
		}
		if ps.Iteration >= maxIter {
			return flowgraph.END // Max iterations reached
		}
		if ps.Blocked {
			return flowgraph.END // Blocked, needs intervention
		}
		return "prompt" // Loop back for another iteration
	})
	graph.AddEdge("commit", flowgraph.END)

	// Compile graph
	compiled, err := graph.Compile()
	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("compile phase graph: %w", err)
		result.Duration = time.Since(start)
		return result, result.Error
	}

	// Create flowgraph context with LLM injected via context.WithValue
	baseCtx := WithLLM(ctx, e.client)
	fgCtx := flowgraph.NewContext(baseCtx,
		flowgraph.WithLogger(e.logger),
		flowgraph.WithContextRunID(fmt.Sprintf("%s-%s", t.ID, p.ID)),
	)

	// Build template vars to get prior phase content
	templateVars := BuildTemplateVars(t, p, s, 0, "")

	// Load spec content from database (specs are not stored as file artifacts)
	templateVars = templateVars.WithSpecFromDatabase(e.backend, t.ID)

	// Load review context for review phases (round 2+ needs prior findings)
	if p.ID == "review" {
		round := 1
		if s != nil && s.Phases != nil {
			if ps, ok := s.Phases["review"]; ok && ps.Status == "completed" {
				round = 2
			}
		}
		templateVars = templateVars.WithReviewContext(e.backend, t.ID, round)
	}

	// Load and apply initiative context if task belongs to an initiative
	if initCtx := LoadInitiativeContext(t, e.backend); initCtx != nil {
		templateVars = templateVars.WithInitiativeContext(*initCtx)
		e.logger.Info("initiative context injected (flowgraph)",
			"task", t.ID,
			"initiative", initCtx.ID,
		)
	}

	// Apply UI testing context if task requires it
	if t.RequiresUITesting {
		projectDir := e.config.WorkDir
		if e.worktreePath != "" {
			projectDir = e.worktreePath
		}
		screenshotDir := task.ScreenshotsPath(projectDir, t.ID)
		templateVars = templateVars.WithUITestingContext(UITestingContext{
			RequiresUITesting: true,
			ScreenshotDir:     screenshotDir,
			TestResults:       loadPriorContent(task.TaskDir(t.ID), s, "test"),
		})
		e.logger.Info("UI testing context injected (flowgraph)",
			"task", t.ID,
			"screenshot_dir", screenshotDir,
		)
	}

	// Build worktree context
	worktreePath := e.worktreePath
	taskBranch := t.Branch
	targetBranch := ResolveTargetBranchForTask(t, e.backend, e.orcConfig)

	// Format requires UI testing as a string (empty string for false, "true" for true)
	requiresUITesting := ""
	if templateVars.RequiresUITesting {
		requiresUITesting = "true"
	}

	// Initial state with retry context if applicable
	initialState := PhaseState{
		TaskID:           t.ID,
		TaskTitle:        t.Title,
		TaskDescription:  t.Description,
		TaskCategory:     string(t.Category),
		Phase:            p.ID,
		Weight:           string(t.Weight),
		Iteration:        0,
		RetryContext:     LoadRetryContextForPhase(s),
		ResearchContent:  templateVars.ResearchContent,
		SpecContent:      templateVars.SpecContent,
		DesignContent:    templateVars.DesignContent,
		ImplementContent: templateVars.ImplementContent,
		WorktreePath:     worktreePath,
		TaskBranch:       taskBranch,
		TargetBranch:     targetBranch,

		// Initiative context (formatted section for {{INITIATIVE_CONTEXT}})
		InitiativeContext: formatInitiativeContextSection(templateVars),

		// UI Testing context
		RequiresUITesting: requiresUITesting,
		ScreenshotDir:     templateVars.ScreenshotDir,
		TestResults:       templateVars.TestResults,

		// Testing configuration
		CoverageThreshold: templateVars.CoverageThreshold,

		// Review phase context (populated by WithReviewContext)
		ReviewRound:         templateVars.ReviewRound,
		ReviewFindings:      templateVars.ReviewFindings,
		VerificationResults: templateVars.VerificationResults,
	}

	// Run with checkpointing if enabled
	var runOpts []flowgraph.RunOption
	runOpts = append(runOpts, flowgraph.WithMaxIterations(maxIter*4+10)) // Buffer for nodes per iteration

	if e.checkpointStore != nil {
		runOpts = append(runOpts,
			flowgraph.WithCheckpointing(e.checkpointStore),
			flowgraph.WithRunID(fmt.Sprintf("%s-%s", t.ID, p.ID)),
		)
	}

	// Execute
	finalState, err := compiled.Run(fgCtx, initialState, runOpts...)

	// Build result
	result.Iterations = finalState.Iteration
	result.Output = finalState.Response
	result.CommitSHA = finalState.CommitSHA
	result.Artifacts = finalState.Artifacts
	result.InputTokens = finalState.InputTokens
	result.OutputTokens = finalState.OutputTokens
	result.Duration = time.Since(start)

	if err != nil {
		result.Status = plan.PhaseFailed
		result.Error = err
		return result, err
	}

	if finalState.Complete {
		result.Status = plan.PhaseCompleted
	} else if finalState.Blocked {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("phase blocked: needs clarification")
	} else {
		result.Status = plan.PhaseFailed
		result.Error = fmt.Errorf("max iterations (%d) reached without completion", maxIter)
	}

	return result, result.Error
}
