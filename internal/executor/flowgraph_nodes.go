// Package executor provides the flowgraph-based execution engine for orc.
package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/randalmurphal/flowgraph/pkg/flowgraph"
	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
)

// buildPromptNode creates the prompt building node.
func (e *Executor) buildPromptNode(p *plan.Phase) flowgraph.NodeFunc[PhaseState] {
	return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
		// Load template from templates/prompts/{phase}.md
		templatePath := filepath.Join(e.config.TemplatesDir, "prompts", p.Name+".md")
		tmplContent, err := os.ReadFile(templatePath)
		if err != nil {
			// Try with ID if name doesn't exist
			templatePath = filepath.Join(e.config.TemplatesDir, "prompts", p.ID+".md")
			tmplContent, err = os.ReadFile(templatePath)
			if err != nil {
				// Use inline prompt from plan if template doesn't exist
				if p.Prompt != "" {
					s.Prompt = e.renderTemplate(p.Prompt, s)
				} else {
					return s, fmt.Errorf("no prompt template found for phase %s", p.ID)
				}
				s.Iteration++
				return s, nil
			}
		}

		// Render template with task context
		s.Prompt = e.renderTemplate(string(tmplContent), s)
		s.Iteration++
		return s, nil
	}
}

// renderTemplate does simple template variable substitution.
func (e *Executor) renderTemplate(tmpl string, s PhaseState) string {
	// Simple variable replacement
	replacements := map[string]string{
		"{{TASK_ID}}":          s.TaskID,
		"{{TASK_TITLE}}":       s.TaskTitle,
		"{{TASK_DESCRIPTION}}": s.TaskDescription,
		"{{PHASE}}":            s.Phase,
		"{{WEIGHT}}":           s.Weight,
		"{{ITERATION}}":        fmt.Sprintf("%d", s.Iteration),
		"{{RESEARCH_CONTENT}}": s.ResearchContent,
		"{{SPEC_CONTENT}}":     s.SpecContent,
		"{{DESIGN_CONTENT}}":   s.DesignContent,
		"{{RETRY_CONTEXT}}":    s.RetryContext,
	}

	result := tmpl
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	return result
}

// executeClaudeNode creates the Claude execution node.
func (e *Executor) executeClaudeNode() flowgraph.NodeFunc[PhaseState] {
	return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
		// Use LLM client from context (injected via WithLLM)
		client := LLM(ctx)
		if client == nil {
			return s, fmt.Errorf("no LLM client available")
		}

		// Publish prompt transcript
		e.publishTranscript(s.TaskID, s.Phase, s.Iteration, "prompt", s.Prompt)

		// Execute completion
		resp, err := client.Complete(ctx, claude.CompletionRequest{
			Messages: []claude.Message{
				{Role: claude.RoleUser, Content: s.Prompt},
			},
			Model: e.config.Model,
		})
		if err != nil {
			s.Error = err
			e.publishError(s.TaskID, s.Phase, err.Error(), false)
			return s, fmt.Errorf("claude completion: %w", err)
		}

		s.Response = resp.Content
		s.InputTokens += resp.Usage.InputTokens
		s.OutputTokens += resp.Usage.OutputTokens
		s.TokensUsed += resp.Usage.TotalTokens

		// Publish response transcript and token update
		e.publishTranscript(s.TaskID, s.Phase, s.Iteration, "response", s.Response)
		e.publishTokens(s.TaskID, s.Phase, resp.Usage.InputTokens, resp.Usage.OutputTokens, resp.Usage.TotalTokens)

		return s, nil
	}
}

// checkCompletionNode creates the completion check node.
func (e *Executor) checkCompletionNode(p *plan.Phase, st *state.State) flowgraph.NodeFunc[PhaseState] {
	return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
		// Detect completion marker in response
		s.Complete = strings.Contains(s.Response, "<phase_complete>true</phase_complete>")

		// Also check for specific phase completion tag
		phaseCompleteTag := fmt.Sprintf("<%s_complete>true</%s_complete>", p.ID, p.ID)
		if strings.Contains(s.Response, phaseCompleteTag) {
			s.Complete = true
		}

		// Check for blocked state
		if strings.Contains(s.Response, "<phase_blocked>") {
			s.Blocked = true
		}

		// Update state tracking
		if st != nil {
			st.IncrementIteration()
			st.AddTokens(s.InputTokens, s.OutputTokens)
		}

		// Save transcript for this iteration
		if err := e.saveTranscript(s); err != nil {
			ctx.Logger().Warn("failed to save transcript", "error", err)
		}

		return s, nil
	}
}

// commitCheckpointNode creates the git commit checkpoint node.
func (e *Executor) commitCheckpointNode() flowgraph.NodeFunc[PhaseState] {
	return func(ctx flowgraph.Context, s PhaseState) (PhaseState, error) {
		// Skip if git operations not available
		if e.gitOps == nil {
			return s, nil
		}

		// Create git checkpoint
		msg := fmt.Sprintf("%s: %s - completed", s.Phase, s.TaskTitle)
		cp, err := e.gitOps.CreateCheckpoint(s.TaskID, s.Phase, msg)
		if err != nil {
			ctx.Logger().Warn("failed to create git checkpoint", "error", err)
			// Don't fail the phase for git errors
			return s, nil
		}

		s.CommitSHA = cp.CommitSHA
		return s, nil
	}
}

// saveTranscript saves the prompt/response for this iteration.
func (e *Executor) saveTranscript(s PhaseState) error {
	dir := filepath.Join(e.config.WorkDir, ".orc", "tasks", s.TaskID, "transcripts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-%03d.md", s.Phase, s.Iteration)
	path := filepath.Join(dir, filename)

	content := fmt.Sprintf(`# %s - Iteration %d

## Prompt

%s

## Response

%s

---
Tokens: %d input, %d output
Complete: %v
Blocked: %v
`,
		s.Phase, s.Iteration, s.Prompt, s.Response,
		s.InputTokens, s.OutputTokens, s.Complete, s.Blocked)

	return os.WriteFile(path, []byte(content), 0644)
}
