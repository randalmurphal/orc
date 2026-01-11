// Package executor provides task phase execution for orc.
//
// This file contains flowgraph node builders for phase execution.
// These functions construct the nodes used in the flowgraph-based execution path.
//
// NOTE: The following methods are defined in executor.go and will be moved here
// during the refactoring process:
//
// Phase execution entry points:
//   - ExecutePhase(ctx, task, phase, state) (*Result, error) - lines 403-412
//   - executePhaseWithSession(ctx, task, phase, state) (*Result, error) - lines 415-429
//   - executePhaseWithFlowgraph(ctx, task, phase, state) (*Result, error) - lines 432-540
//
// Flowgraph node builders:
//   - buildPromptNode(phase) flowgraph.NodeFunc[PhaseState] - lines 543-569
//   - executeClaudeNode() flowgraph.NodeFunc[PhaseState] - lines 596-631
//   - checkCompletionNode(phase, state) flowgraph.NodeFunc[PhaseState] - lines 634-662
//   - commitCheckpointNode() flowgraph.NodeFunc[PhaseState] - lines 665-685
//
// Utilities:
//   - saveTranscript(state) error - lines 688-716
//   - renderTemplate(tmpl, state) string - lines 572-593
//     (Note: renderTemplate is already in template.go as RenderTemplate)
//
// The methods remain in executor.go to avoid breaking imports during the
// incremental refactoring process. Tests in flowgraph_nodes_test.go verify
// the behavior of these methods.
package executor
