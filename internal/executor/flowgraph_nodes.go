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
		// Check inline prompt first (matches LoadPromptTemplate behavior)
		if p.Prompt != "" {
			s.Prompt = e.renderTemplate(p.Prompt, s)
			s.Iteration++
			return s, nil
		}

		// Fall back to template file: templates/prompts/{phase}.md
		templatePath := filepath.Join(e.config.TemplatesDir, "prompts", p.Name+".md")
		tmplContent, err := os.ReadFile(templatePath)
		if err != nil {
			// Try with ID if name doesn't exist
			templatePath = filepath.Join(e.config.TemplatesDir, "prompts", p.ID+".md")
			tmplContent, err = os.ReadFile(templatePath)
			if err != nil {
				return s, fmt.Errorf("no prompt template found for phase %s", p.ID)
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
	// For tasks without a spec phase, use task description as spec content
	specContent := s.SpecContent
	if specContent == "" && s.TaskDescription != "" {
		specContent = s.TaskDescription
	}

	// Default coverage threshold if not set
	coverageThreshold := s.CoverageThreshold
	if coverageThreshold == 0 {
		coverageThreshold = 85 // Default value
	}

	// Simple variable replacement
	replacements := map[string]string{
		"{{TASK_ID}}":                s.TaskID,
		"{{TASK_TITLE}}":             s.TaskTitle,
		"{{TASK_DESCRIPTION}}":       s.TaskDescription,
		"{{TASK_CATEGORY}}":          s.TaskCategory,
		"{{PHASE}}":                  s.Phase,
		"{{WEIGHT}}":                 s.Weight,
		"{{ITERATION}}":              fmt.Sprintf("%d", s.Iteration),
		"{{RESEARCH_CONTENT}}":       s.ResearchContent,
		"{{SPEC_CONTENT}}":           specContent,
		"{{DESIGN_CONTENT}}":         s.DesignContent,
		"{{IMPLEMENT_CONTENT}}":      s.ImplementContent,
		"{{IMPLEMENTATION_SUMMARY}}": s.ImplementContent, // Alias for template compatibility
		"{{RETRY_CONTEXT}}":          s.RetryContext,

		// Worktree context variables
		"{{WORKTREE_PATH}}": s.WorktreePath,
		"{{TASK_BRANCH}}":   s.TaskBranch,
		"{{TARGET_BRANCH}}": s.TargetBranch,

		// Initiative context (formatted section)
		"{{INITIATIVE_CONTEXT}}": s.InitiativeContext,

		// UI Testing context variables
		"{{REQUIRES_UI_TESTING}}": s.RequiresUITesting,
		"{{SCREENSHOT_DIR}}":      s.ScreenshotDir,
		"{{TEST_RESULTS}}":        s.TestResults,

		// Testing configuration
		"{{COVERAGE_THRESHOLD}}": fmt.Sprintf("%d", coverageThreshold),

		// Review phase context variables
		"{{REVIEW_ROUND}}":       fmt.Sprintf("%d", s.ReviewRound),
		"{{REVIEW_FINDINGS}}":    s.ReviewFindings,
		"{{VERIFICATION_RESULTS}}": s.VerificationResults,
	}

	result := tmpl
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	// Process conditional blocks for review rounds
	result = processReviewConditionals(result, s.ReviewRound)

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
		// Use effective input tokens (includes cache) to show actual context size
		// Note: claude.TokenUsage doesn't have EffectiveInputTokens method, so compute directly
		effectiveInput := resp.Usage.InputTokens + resp.Usage.CacheCreationInputTokens + resp.Usage.CacheReadInputTokens
		s.InputTokens += effectiveInput
		s.OutputTokens += resp.Usage.OutputTokens
		s.CacheCreationInputTokens += resp.Usage.CacheCreationInputTokens
		s.CacheReadInputTokens += resp.Usage.CacheReadInputTokens
		s.TokensUsed += effectiveInput + resp.Usage.OutputTokens

		// Publish response transcript and token update
		e.publishTranscript(s.TaskID, s.Phase, s.Iteration, "response", s.Response)
		e.publishTokens(s.TaskID, s.Phase, effectiveInput, resp.Usage.OutputTokens, resp.Usage.CacheCreationInputTokens, resp.Usage.CacheReadInputTokens, effectiveInput+resp.Usage.OutputTokens)

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
			st.AddTokens(s.InputTokens, s.OutputTokens, s.CacheCreationInputTokens, s.CacheReadInputTokens)
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
		// Use worktree git if available, otherwise fall back to main repo git
		gitSvc := e.gitOps
		if e.worktreeGit != nil {
			gitSvc = e.worktreeGit
		}

		// Skip if git operations not available
		if gitSvc == nil {
			return s, nil
		}

		// Create git checkpoint
		msg := fmt.Sprintf("%s: %s - completed", s.Phase, s.TaskTitle)
		cp, err := gitSvc.CreateCheckpoint(s.TaskID, s.Phase, msg)
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
// Writes to both database (for search/export) and files (for debugging).
func (e *Executor) saveTranscript(s PhaseState) error {
	// Build the full transcript content
	content := fmt.Sprintf(`# %s - Iteration %d

## Prompt

%s

## Response

%s

---
Tokens: %d input, %d output, %d cache_creation, %d cache_read
Complete: %v
Blocked: %v
`,
		s.Phase, s.Iteration, s.Prompt, s.Response,
		s.InputTokens, s.OutputTokens, s.CacheCreationInputTokens, s.CacheReadInputTokens, s.Complete, s.Blocked)

	// NOTE: Database transcript persistence is handled via JSONL sync from Claude Code's
	// session files (see jsonl_sync.go). Flowgraph execution still writes file backups
	// for debugging purposes.

	// Write to files for debugging/backup
	dir := filepath.Join(e.config.WorkDir, ".orc", "tasks", s.TaskID, "transcripts")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("%s-%03d.md", s.Phase, s.Iteration)
	path := filepath.Join(dir, filename)

	return os.WriteFile(path, []byte(content), 0644)
}
