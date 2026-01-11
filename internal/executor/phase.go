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

	// Initial state with retry context if applicable
	initialState := PhaseState{
		TaskID:           t.ID,
		TaskTitle:        t.Title,
		TaskDescription:  t.Description,
		Phase:            p.ID,
		Weight:           string(t.Weight),
		Iteration:        0,
		RetryContext:     LoadRetryContextForPhase(s),
		ResearchContent:  templateVars.ResearchContent,
		SpecContent:      templateVars.SpecContent,
		DesignContent:    templateVars.DesignContent,
		ImplementContent: templateVars.ImplementContent,
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
