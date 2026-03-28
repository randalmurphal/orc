// workflow_phase.go contains phase execution logic for workflow runs.
// This includes loading prompts, executing phases with Claude, and handling timeouts.
package executor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/automation"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
	"github.com/randalmurphal/orc/templates"
)

// MaxOrcRetries is the maximum number of times orc will retry calling Claude
// when it receives invalid JSON or a "continue" status. This is NOT the Claude
// turn limit (--max-turns) which comes from config.MaxTurns.
const MaxOrcRetries = 5

// PhaseExecutionConfig holds configuration for a phase execution.
type PhaseExecutionConfig struct {
	Prompt      string
	Model       string
	Provider    string // "claude", "codex", "ollama", or empty (default: claude)
	WorkingDir  string
	TaskID      string
	PhaseID     string
	RunID       string
	Thinking    bool
	ReviewRound int // For review phase: 1 = findings, 2 = decision

	// For quality checks
	PhaseTemplate *db.PhaseTemplate
	WorkflowPhase *db.WorkflowPhase

	// Claude CLI configuration (resolved from template + override + agent + skills)
	ClaudeConfig *PhaseClaudeConfig
}

// PhaseExecutionResult holds the result of a phase execution.
type PhaseExecutionResult struct {
	Iterations          int
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	CostUSD             float64
	Content             string // Phase output content for content-producing phases
	RawOutput           string // Full JSON output (for docs phase note extraction)
	SessionID           string
}

type controlPlaneVariableUsage struct {
	PendingRecommendations    bool
	CompletionRecommendations bool
	AttentionSummary          bool
	HandoffContext            bool
	IndexedArtifacts          bool
}

type threadVariableUsage struct {
	ThreadID                   bool
	ThreadTitle                bool
	ThreadContext              bool
	ThreadHistory              bool
	ThreadLinkedContext        bool
	ThreadRecommendationDrafts bool
	ThreadDecisionDrafts       bool
}

func (u controlPlaneVariableUsage) Any() bool {
	return u.PendingRecommendations || u.CompletionRecommendations || u.AttentionSummary || u.HandoffContext || u.IndexedArtifacts
}

func (u controlPlaneVariableUsage) needsRecommendations() bool {
	return u.PendingRecommendations || u.CompletionRecommendations || u.HandoffContext
}

func (u threadVariableUsage) Any() bool {
	return u.ThreadID ||
		u.ThreadTitle ||
		u.ThreadContext ||
		u.ThreadHistory ||
		u.ThreadLinkedContext ||
		u.ThreadRecommendationDrafts ||
		u.ThreadDecisionDrafts
}

func detectControlPlaneVariableUsage(content string) controlPlaneVariableUsage {
	return controlPlaneVariableUsage{
		PendingRecommendations:    strings.Contains(content, "{{PENDING_RECOMMENDATIONS}}"),
		CompletionRecommendations: strings.Contains(content, "{{COMPLETION_RECOMMENDATIONS}}"),
		AttentionSummary:          strings.Contains(content, "{{ATTENTION_SUMMARY}}"),
		HandoffContext:            strings.Contains(content, "{{HANDOFF_CONTEXT}}"),
		IndexedArtifacts:          strings.Contains(content, "{{INDEXED_ARTIFACTS}}"),
	}
}

func detectThreadVariableUsage(content string) threadVariableUsage {
	return threadVariableUsage{
		ThreadID:                   strings.Contains(content, "{{THREAD_ID}}"),
		ThreadTitle:                strings.Contains(content, "{{THREAD_TITLE}}"),
		ThreadContext:              strings.Contains(content, "{{THREAD_CONTEXT}}"),
		ThreadHistory:              strings.Contains(content, "{{THREAD_HISTORY}}"),
		ThreadLinkedContext:        strings.Contains(content, "{{THREAD_LINKED_CONTEXT}}"),
		ThreadRecommendationDrafts: strings.Contains(content, "{{THREAD_RECOMMENDATION_DRAFTS}}"),
		ThreadDecisionDrafts:       strings.Contains(content, "{{THREAD_DECISION_DRAFTS}}"),
	}
}

func mergeControlPlaneVariableUsage(parts ...controlPlaneVariableUsage) controlPlaneVariableUsage {
	merged := controlPlaneVariableUsage{}
	for _, part := range parts {
		merged.PendingRecommendations = merged.PendingRecommendations || part.PendingRecommendations
		merged.CompletionRecommendations = merged.CompletionRecommendations || part.CompletionRecommendations
		merged.AttentionSummary = merged.AttentionSummary || part.AttentionSummary
		merged.HandoffContext = merged.HandoffContext || part.HandoffContext
		merged.IndexedArtifacts = merged.IndexedArtifacts || part.IndexedArtifacts
	}
	return merged
}

func mergeThreadVariableUsage(parts ...threadVariableUsage) threadVariableUsage {
	merged := threadVariableUsage{}
	for _, part := range parts {
		merged.ThreadID = merged.ThreadID || part.ThreadID
		merged.ThreadTitle = merged.ThreadTitle || part.ThreadTitle
		merged.ThreadContext = merged.ThreadContext || part.ThreadContext
		merged.ThreadHistory = merged.ThreadHistory || part.ThreadHistory
		merged.ThreadLinkedContext = merged.ThreadLinkedContext || part.ThreadLinkedContext
		merged.ThreadRecommendationDrafts = merged.ThreadRecommendationDrafts || part.ThreadRecommendationDrafts
		merged.ThreadDecisionDrafts = merged.ThreadDecisionDrafts || part.ThreadDecisionDrafts
	}
	return merged
}

func (we *WorkflowExecutor) phaseControlPlaneVariableUsage(
	tmpl *db.PhaseTemplate,
	phase *db.WorkflowPhase,
) (controlPlaneVariableUsage, error) {
	effectiveType := tmpl.Type
	if phase != nil && phase.TypeOverride != "" {
		effectiveType = phase.TypeOverride
	}
	if effectiveType == "" {
		effectiveType = "llm"
	}

	usage := controlPlaneVariableUsage{}
	if effectiveType == "llm" {
		promptContent, err := we.loadPhasePrompt(tmpl)
		if err != nil {
			return controlPlaneVariableUsage{}, fmt.Errorf("load phase prompt for control-plane usage: %w", err)
		}
		usage = detectControlPlaneVariableUsage(promptContent)
	} else if tmpl.PromptContent != "" {
		usage = detectControlPlaneVariableUsage(tmpl.PromptContent)
	}

	cfg := we.getEffectivePhaseClaudeConfig(tmpl, phase)
	if cfg == nil {
		return usage, nil
	}

	return mergeControlPlaneVariableUsage(
		usage,
		detectControlPlaneVariableUsage(cfg.SystemPrompt),
		detectControlPlaneVariableUsage(cfg.AppendSystemPrompt),
	), nil
}

func (we *WorkflowExecutor) phaseThreadVariableUsage(
	tmpl *db.PhaseTemplate,
	phase *db.WorkflowPhase,
) (threadVariableUsage, error) {
	effectiveType := tmpl.Type
	if phase != nil && phase.TypeOverride != "" {
		effectiveType = phase.TypeOverride
	}
	if effectiveType == "" {
		effectiveType = "llm"
	}

	usage := threadVariableUsage{}
	if effectiveType == "llm" {
		promptContent, err := we.loadPhasePrompt(tmpl)
		if err != nil {
			return threadVariableUsage{}, fmt.Errorf("load phase prompt for thread usage: %w", err)
		}
		usage = detectThreadVariableUsage(promptContent)
	} else if tmpl.PromptContent != "" {
		usage = detectThreadVariableUsage(tmpl.PromptContent)
	}

	cfg := we.getEffectivePhaseClaudeConfig(tmpl, phase)
	if cfg == nil {
		return usage, nil
	}

	return mergeThreadVariableUsage(
		usage,
		detectThreadVariableUsage(cfg.SystemPrompt),
		detectThreadVariableUsage(cfg.AppendSystemPrompt),
	), nil
}

// executePhase runs a single phase of the workflow.
func (we *WorkflowExecutor) executePhase(
	ctx context.Context,
	tmpl *db.PhaseTemplate,
	phase *db.WorkflowPhase,
	vars variable.VariableSet,
	rctx *variable.ResolutionContext,
	run *db.WorkflowRun,
	runPhase *db.WorkflowRunPhase,
	t *orcv1.Task,
) (PhaseResult, error) {
	result := PhaseResult{
		PhaseID: tmpl.ID,
		Status:  orcv1.PhaseStatus_PHASE_STATUS_PENDING.String(),
	}

	startTime := time.Now()

	// Update phase status
	runPhase.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING.String()
	runPhase.StartedAt = timePtr(startTime)
	if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
		return result, fmt.Errorf("update phase status: %w", err)
	}

	we.logger.Info("executing phase",
		"run_id", run.ID,
		"phase", tmpl.ID,
	)

	// Publish phase start event for real-time UI updates
	if t != nil {
		we.publisher.PhaseStart(t.Id, tmpl.ID)
	}

	// Use iteration-specific template if LoopTemplates is configured
	effectiveTemplate := tmpl
	if phase.LoopConfig != "" && rctx != nil {
		loopCfg, err := db.ParseLoopConfig(phase.LoopConfig)
		if err != nil {
			we.logger.Warn("failed to parse loop config, using base template",
				"phase", tmpl.ID,
				"error", err,
			)
		} else if loopCfg != nil && len(loopCfg.LoopTemplates) > 0 {
			iteration := rctx.GetEffectiveReviewRound()
			iterationTemplate := loopCfg.GetTemplateForIteration(iteration, tmpl.PromptPath)
			if iterationTemplate != tmpl.PromptPath {
				roundTemplate := *tmpl
				roundTemplate.PromptPath = iterationTemplate
				effectiveTemplate = &roundTemplate
				we.logger.Info("using iteration-specific template",
					"phase", tmpl.ID,
					"iteration", iteration,
					"path", iterationTemplate,
				)
			}
		}
	}

	// Resolve effective phase type: WorkflowPhase.TypeOverride > PhaseTemplate.Type > "llm"
	effectiveType := tmpl.Type
	if phase.TypeOverride != "" {
		effectiveType = phase.TypeOverride
	}
	if effectiveType == "" {
		effectiveType = "llm"
	}

	// For non-LLM types, dispatch to the registered executor (skip prompt loading)
	if effectiveType != "llm" {
		executor, lookupErr := we.phaseTypeRegistry.Get(effectiveType)
		if lookupErr != nil {
			result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING.String()
			result.Error = lookupErr.Error()
			return result, lookupErr
		}

		params := PhaseTypeParams{
			PhaseTemplate: tmpl,
			Task:          t,
			Vars:          vars,
			RCtx:          rctx,
		}

		// Build KnowledgePhaseConfig from template metadata if this is a knowledge phase
		if effectiveType == "knowledge" && we.knowledgeService != nil {
			params.KnowledgeConfig = &KnowledgePhaseConfig{
				OutputVar: tmpl.OutputVarName,
			}
		}

		phaseResult, execErr := executor.ExecutePhase(ctx, params)

		// Update result from executor output
		result.PhaseID = phaseResult.PhaseID
		result.Status = phaseResult.Status
		result.Content = phaseResult.Content
		result.CostUSD = phaseResult.CostUSD
		result.InputTokens = phaseResult.InputTokens
		result.OutputTokens = phaseResult.OutputTokens
		result.DurationMS = time.Since(startTime).Milliseconds()

		if execErr != nil {
			result.Error = execErr.Error()
			runPhase.Status = result.Status
			runPhase.Error = result.Error
			runPhase.CompletedAt = timePtr(time.Now())
			if saveErr := we.backend.SaveWorkflowRunPhase(runPhase); saveErr != nil {
				we.logger.Warn("failed to save failed non-LLM phase state", "phase", tmpl.ID, "error", saveErr)
			}
			if t != nil {
				we.publisher.PhaseFailed(t.Id, tmpl.ID, execErr)
			}
			return result, execErr
		}

		// Update run phase record - respect executor's returned status
		isSkipped := result.Status == orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String()
		if isSkipped {
			runPhase.Status = orcv1.PhaseStatus_PHASE_STATUS_SKIPPED.String()
		} else {
			runPhase.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String()
		}
		runPhase.CompletedAt = timePtr(time.Now())
		runPhase.CostUSD = result.CostUSD
		runPhase.InputTokens = result.InputTokens
		runPhase.OutputTokens = result.OutputTokens
		if result.Content != "" {
			runPhase.Content = result.Content
		}
		if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
			we.logger.Warn("failed to save non-LLM run phase", "error", err)
		}

		// Publish appropriate event
		if t != nil {
			if isSkipped {
				we.publisher.PhaseSkipped(t.Id, tmpl.ID)
			} else {
				we.publisher.PhaseComplete(t.Id, tmpl.ID, "")
			}
		}

		// Update run totals (non-LLM phases typically have zero cost)
		run.TotalCostUSD += result.CostUSD
		run.TotalInputTokens += result.InputTokens
		run.TotalOutputTokens += result.OutputTokens
		if err := we.backend.SaveWorkflowRun(run); err != nil {
			we.logger.Warn("failed to update run totals for non-LLM phase", "error", err)
		}

		// Update execution state if available
		if we.task != nil {
			if isSkipped {
				task.SkipPhaseProto(we.task.Execution, tmpl.ID, "non-LLM executor returned skipped")
			} else {
				task.CompletePhaseProto(we.task.Execution, tmpl.ID, "")
			}
			task.SetPhaseTokensProto(we.task.Execution, tmpl.ID, &orcv1.TokenUsage{
				InputTokens:              int32(result.InputTokens),
				OutputTokens:             int32(result.OutputTokens),
				CacheCreationInputTokens: int32(result.CacheCreationTokens),
				CacheReadInputTokens:     int32(result.CacheReadTokens),
				TotalTokens:              int32(result.InputTokens + result.OutputTokens + result.CacheCreationTokens + result.CacheReadTokens),
			})
			if err := we.backend.SaveTask(we.task); err != nil {
				we.logger.Warn("failed to save task execution state for non-LLM phase", "error", err)
			}
		}

		return result, nil
	}

	// --- LLM path below: load prompt, configure Claude, execute ---

	// Load prompt template
	promptContent, err := we.loadPhasePrompt(effectiveTemplate)
	if err != nil {
		result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING.String()
		result.Error = err.Error()
		return result, err
	}

	// Render template with variables
	renderedPrompt := variable.RenderTemplate(promptContent, vars)

	// Determine model (workflow phase override > template default > config default)
	model := we.resolvePhaseModel(tmpl, phase)

	// Resolve effective Claude configuration for this phase
	claudeConfig := we.getEffectivePhaseClaudeConfig(tmpl, phase)

	// Load phase agents from global database and add to Claude config
	if rctx.TaskWeight != "" && we.globalDB != nil {
		phaseAgents, err := LoadPhaseAgents(we.globalDB, tmpl.ID, rctx.TaskWeight, vars)
		if err != nil {
			we.logger.Warn("failed to load phase agents", "phase", tmpl.ID, "weight", rctx.TaskWeight, "error", err)
		} else if len(phaseAgents) > 0 {
			if claudeConfig == nil {
				claudeConfig = &PhaseClaudeConfig{}
			}
			if claudeConfig.InlineAgents == nil {
				claudeConfig.InlineAgents = make(map[string]InlineAgentDef)
			}
			maps.Copy(claudeConfig.InlineAgents, phaseAgents)
			we.logger.Info("loaded phase agents", "phase", tmpl.ID, "weight", rctx.TaskWeight, "count", len(phaseAgents))
		}
	}

	// Merge runtime MCP settings (headless mode, task-specific user-data-dir) into phase MCP config
	// This applies orc config settings to MCP servers defined in phase templates
	if claudeConfig != nil && len(claudeConfig.MCPServers) > 0 {
		claudeConfig.MCPServers = MergeMCPConfigSettings(claudeConfig.MCPServers, rctx.TaskID, we.orcConfig)
	}

	// Phase settings lifecycle: reset → apply → execute → reset
	// Only when running in a worktree (standalone mode has no worktree)
	if we.worktreePath != "" && we.globalDB != nil {
		// Pre-reset: restore .claude/ to clean project state from target branch
		if err := resetClaudeDir(we.worktreePath, rctx.TargetBranch); err != nil {
			we.logger.Warn("pre-reset .claude/ failed, continuing (ApplyPhaseSettings will overwrite)",
				"phase", tmpl.ID,
				"error", err,
			)
		}

		// Apply phase-specific settings (hooks, skills, MCP servers, env vars)
		baseCfg := &WorktreeBaseConfig{
			WorktreePath: we.worktreePath,
			MainRepoPath: we.workingDir,
			TaskID:       rctx.TaskID,
			AdditionalEnv: map[string]string{
				"ORC_TASK_ID": rctx.TaskID,
			},
		}
		if err := ApplyPhaseSettings(we.worktreePath, claudeConfig, baseCfg, we.globalDB, we.globalDB); err != nil {
			result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING.String()
			result.Error = err.Error()
			return result, fmt.Errorf("apply phase settings for %s: %w", tmpl.ID, err)
		}
	}

	// Check if this phase already completed but task state wasn't updated (crash recovery).
	// If runPhase.Content is populated, Claude already finished - use saved result.
	// This prevents resuming a finished Claude session which would fail with empty structured_output.
	if we.isResuming && runPhase.Content != "" {
		we.logger.Info("using saved phase content from previous run (crash recovery)",
			"phase", tmpl.ID,
			"content_length", len(runPhase.Content),
		)
		// Build result from saved runPhase data
		result.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String()
		result.Content = runPhase.Content
		result.Iterations = runPhase.Iterations
		result.InputTokens = runPhase.InputTokens
		result.OutputTokens = runPhase.OutputTokens
		result.CostUSD = runPhase.CostUSD
		result.DurationMS = 0 // Already completed, no new duration

		// Ensure runPhase is marked completed (might already be, but idempotent)
		if runPhase.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String() {
			runPhase.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String()
			runPhase.CompletedAt = timePtr(time.Now())
			if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
				we.logger.Warn("failed to update run phase status", "error", err)
			}
		}

		// Complete the task state that wasn't saved before crash
		if we.task != nil {
			task.CompletePhaseProto(we.task.Execution, tmpl.ID, runPhase.CommitSHA)
			if err := we.backend.SaveTask(we.task); err != nil {
				we.logger.Warn("failed to save task execution state", "error", err)
			}
		}

		// Publish phase complete event
		if t != nil {
			we.publisher.PhaseComplete(t.Id, tmpl.ID, "")
		}

		return result, nil
	}

	// Determine provider (workflow phase override > template > workflow default > config > "claude")
	provider := we.resolvePhaseProvider(tmpl, phase)

	// Validate provider is known (reject typos and unsupported providers)
	if err := validateProvider(provider); err != nil {
		return result, fmt.Errorf("execute phase %s: %w", tmpl.ID, err)
	}

	// Validate provider capabilities before execution
	if err := we.validateProviderCapabilities(provider, tmpl.ID, claudeConfig); err != nil {
		return result, fmt.Errorf("provider validation: %w", err)
	}

	// Build execution context for LLM provider
	// Use worktree path if available, otherwise fall back to original working dir
	execConfig := PhaseExecutionConfig{
		Prompt:        renderedPrompt,
		Model:         model,
		Provider:      provider,
		WorkingDir:    we.effectiveWorkingDir(),
		TaskID:        rctx.TaskID,
		PhaseID:       tmpl.ID,
		RunID:         run.ID,
		Thinking:      we.shouldUseThinking(tmpl, phase),
		ReviewRound:   rctx.ReviewRound, // For review phase: controls schema selection
		PhaseTemplate: tmpl,
		WorkflowPhase: phase,
		ClaudeConfig:  claudeConfig,
	}

	// Record session metadata on task for monitoring (provider:model per phase)
	if t != nil {
		setTaskSessionMetadata(t, tmpl.ID, provider, model)
	}

	// Execute with provider-specific adapter
	adapter := providerAdapterFor(provider)
	var execResult *PhaseExecutionResult
	execResult, err = we.executeWithProvider(ctx, execConfig, adapter)

	// Post-reset: restore .claude/ for next phase (non-fatal)
	if we.worktreePath != "" && we.globalDB != nil {
		if resetErr := resetClaudeDir(we.worktreePath, rctx.TargetBranch); resetErr != nil {
			we.logger.Warn("post-reset .claude/ failed",
				"phase", tmpl.ID,
				"error", resetErr,
			)
		}
	}
	if err != nil {
		result.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING.String()
		result.Error = err.Error()
		// Preserve execution metrics even on error (e.g. blocked phases still ran the LLM)
		if execResult != nil {
			result.DurationMS = time.Since(startTime).Milliseconds()
			result.InputTokens = execResult.InputTokens
			result.OutputTokens = execResult.OutputTokens
			result.CacheCreationTokens = execResult.CacheCreationTokens
			result.CacheReadTokens = execResult.CacheReadTokens
			result.CostUSD = execResult.CostUSD
			result.Provider = provider
			result.Model = model
			result.Content = execResult.Content
			result.RawOutput = execResult.RawOutput
			result.OutputVarName = tmpl.OutputVarName
			if result.CostUSD == 0 && we.tokenRates != nil && (result.InputTokens+result.OutputTokens) > 0 {
				result.CostUSD = EstimateTokenCostUSDWithRates(we.tokenRates, provider, model,
					int64(result.InputTokens), int64(result.OutputTokens),
					int64(result.CacheReadTokens), int64(result.CacheCreationTokens))
			}
		}
		runPhase.Status = orcv1.PhaseStatus_PHASE_STATUS_PENDING.String()
		runPhase.Error = result.Error
		runPhase.CompletedAt = timePtr(time.Now())
		if saveErr := we.backend.SaveWorkflowRunPhase(runPhase); saveErr != nil {
			we.logger.Warn("failed to save failed phase state", "phase", tmpl.ID, "error", saveErr)
		}
		// Publish phase failed event for real-time UI updates
		if t != nil {
			we.publisher.PhaseFailed(t.Id, tmpl.ID, err)
		}
		return result, err
	}

	// Update result
	result.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String()
	result.Iterations = execResult.Iterations
	result.DurationMS = time.Since(startTime).Milliseconds()
	result.InputTokens = execResult.InputTokens
	result.OutputTokens = execResult.OutputTokens
	result.CacheCreationTokens = execResult.CacheCreationTokens
	result.CacheReadTokens = execResult.CacheReadTokens
	result.CostUSD = execResult.CostUSD
	result.Provider = provider
	result.Model = model
	result.OutputVarName = tmpl.OutputVarName

	// Estimate cost from token rates when provider doesn't return cost natively
	if result.CostUSD == 0 && we.tokenRates != nil && (result.InputTokens+result.OutputTokens) > 0 {
		result.CostUSD = EstimateTokenCostUSDWithRates(we.tokenRates, provider, model,
			int64(result.InputTokens), int64(result.OutputTokens),
			int64(result.CacheReadTokens), int64(result.CacheCreationTokens))
	}

	// Capture phase output content for loop condition evaluation and variable propagation.
	// All phases store their output in result.Content so that applyPhaseContentToVars
	// populates rctx.PriorOutputs — required for EvaluateCondition(phase_output.*).
	result.Content = execResult.Content
	result.RawOutput = execResult.RawOutput
	if tmpl.ProducesArtifact && result.Content == "" {
		we.logger.Warn("artifact-producing phase completed with no content extracted",
			"phase", tmpl.ID,
			"output_var", tmpl.OutputVarName,
			"raw_output_length", len(execResult.Content),
		)
	}
	// Save structured phase output to phase_outputs when the template explicitly
	// declares an output variable or produces an artifact. Review phases need
	// durable structured output even though they are not artifact-producing.
	if result.Content != "" && t != nil && (tmpl.ProducesArtifact || tmpl.OutputVarName != "") {
		// Use template's output variable name, fall back to OUTPUT_<PHASE_ID>
		outputVarName := tmpl.OutputVarName
		if outputVarName == "" {
			outputVarName = "OUTPUT_" + strings.ToUpper(strings.ReplaceAll(tmpl.ID, "-", "_"))
		}

		taskID := t.Id
		output := &storage.PhaseOutputInfo{
			WorkflowRunID:   run.ID,
			PhaseTemplateID: tmpl.ID,
			TaskID:          &taskID,
			Content:         result.Content,
			OutputVarName:   outputVarName,
			ArtifactType:    tmpl.ArtifactType,
			Source:          "workflow",
			Iteration:       result.Iterations,
		}
		if err := we.backend.SavePhaseOutput(output); err != nil {
			we.logger.Warn("failed to save phase output",
				"task", t.Id,
				"phase", tmpl.ID,
				"output_var", outputVarName,
				"error", err,
			)
		}
	}

	// Persist initiative notes from docs phase (SC-5: knowledge curator integration)
	// Use execResult.RawOutput which contains the full JSON (including initiative_notes),
	// not result.Content which only has the extracted "content" field.
	if tmpl.ID == "docs" && execResult.RawOutput != "" && rctx != nil && rctx.InitiativeID != "" {
		docsResp, parseErr := ParseDocsResponse(execResult.RawOutput)
		if parseErr != nil {
			we.logger.Warn("failed to parse docs response for initiative notes",
				"task", t.Id,
				"error", parseErr,
			)
		} else if len(docsResp.InitiativeNotes) > 0 {
			taskID := ""
			if t != nil {
				taskID = t.Id
			}
			if persistErr := PersistInitiativeNotes(we.backend, docsResp.InitiativeNotes, taskID, rctx.InitiativeID); persistErr != nil {
				we.logger.Warn("failed to persist initiative notes from docs phase",
					"task", t.Id,
					"initiative", rctx.InitiativeID,
					"error", persistErr,
				)
			} else {
				we.logger.Info("persisted initiative notes from docs phase",
					"task", t.Id,
					"initiative", rctx.InitiativeID,
					"count", len(docsResp.InitiativeNotes),
				)
			}
		}
	}

	// Update phase record
	runPhase.Status = orcv1.PhaseStatus_PHASE_STATUS_COMPLETED.String()
	runPhase.Iterations = result.Iterations
	runPhase.CompletedAt = timePtr(time.Now())
	runPhase.InputTokens = result.InputTokens
	runPhase.OutputTokens = result.OutputTokens
	runPhase.CostUSD = result.CostUSD
	if result.Content != "" {
		runPhase.Content = result.Content
	}
	if err := we.backend.SaveWorkflowRunPhase(runPhase); err != nil {
		we.logger.Warn("failed to save run phase", "error", err)
	}

	// Publish phase complete event for real-time UI updates
	if t != nil {
		we.publisher.PhaseComplete(t.Id, tmpl.ID, "")
		// Trigger automation event for phase completion
		we.triggerAutomationEvent(ctx, automation.EventPhaseCompleted, t, tmpl.ID)
	}

	// Update run totals
	run.TotalCostUSD += result.CostUSD
	run.TotalInputTokens += result.InputTokens
	run.TotalOutputTokens += result.OutputTokens
	if err := we.backend.SaveWorkflowRun(run); err != nil {
		we.logger.Warn("failed to update run totals", "error", err)
	}

	// Record cost to global database for cross-project analytics
	phaseModel := we.resolvePhaseModel(tmpl, phase)
	phaseProvider := we.resolvePhaseProvider(tmpl, phase)
	we.recordCostToGlobal(ctx, t, tmpl.ID, result, phaseModel, phaseProvider, time.Since(startTime))

	// Update execution state if available (Task-centric approach)
	if we.task != nil {
		// Create checkpoint commit for this phase so `orc rewind` works
		commitSHA := ""
		if we.gitOps != nil {
			checkpoint, err := we.gitOps.CreateCheckpoint(t.Id, tmpl.ID, "completed")
			if err != nil {
				we.logger.Debug("no checkpoint created", "phase", tmpl.ID, "reason", err)
			} else if checkpoint != nil {
				commitSHA = checkpoint.CommitSHA
			}
		}
		task.CompletePhaseProto(we.task.Execution, tmpl.ID, commitSHA)
		task.SetPhaseTokensProto(we.task.Execution, tmpl.ID, &orcv1.TokenUsage{
			InputTokens:              int32(result.InputTokens),
			OutputTokens:             int32(result.OutputTokens),
			CacheCreationInputTokens: int32(result.CacheCreationTokens),
			CacheReadInputTokens:     int32(result.CacheReadTokens),
			TotalTokens:              int32(result.InputTokens + result.OutputTokens + result.CacheCreationTokens + result.CacheReadTokens),
		})
		currentPhase := ""
		if we.task.CurrentPhase != nil {
			currentPhase = *we.task.CurrentPhase
		}
		task.AddCostProto(we.task.Execution, currentPhase, result.CostUSD)
		if err := we.backend.SaveTask(we.task); err != nil {
			we.logger.Warn("failed to save task execution state", "error", err)
		}
	}

	return result, nil
}

// executeWithProvider runs a phase using the given provider adapter.
// This is the shared orchestration loop for all LLM providers. Provider-specific
// behavior (session management, executor config, post-turn persistence) is
// encapsulated in the ProviderAdapter. The loop handles retry, quality checks,
// verification gates, and token accumulation uniformly.
func (we *WorkflowExecutor) executeWithProvider(ctx context.Context, cfg PhaseExecutionConfig, adapter ProviderAdapter) (*PhaseExecutionResult, error) {
	result := &PhaseExecutionResult{}

	// 1. Provider-specific preparation (session, model, phase settings)
	pctx, err := adapter.PrepareExecution(&cfg, we)
	if err != nil {
		return result, fmt.Errorf("%s prepare: %w", adapter.Name(), err)
	}
	we.clearRetryStateForFreshPhaseStart(cfg.PhaseID, pctx.ShouldResume)
	result.SessionID = pctx.SessionID

	// 2. Create or inject TurnExecutor
	var turnExec TurnExecutor
	if we.turnExecutor != nil {
		turnExec = we.turnExecutor
		turnExec.UpdateSessionID(pctx.SessionID)
	} else {
		teCfg := adapter.BuildTurnExecutorConfig(&cfg, pctx, we)
		turnExec = NewTurnExecutor(teCfg)
	}

	// 3. Shared orchestration loop
	for i := 0; i < MaxOrcRetries; i++ {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		result.Iterations++
		we.updatePhaseIterations(cfg, result.Iterations)

		var (
			turnResult  *TurnResult
			turnResults []*TurnResult
		)

		if we.turnExecutor == nil && shouldUseClaudeStructuredFinalize(cfg, adapter) {
			turnResult, turnResults, err = executeClaudeStructuredFinalize(ctx, turnExec, cfg, pctx.Prompt)
		} else {
			turnResult, err = turnExec.ExecuteTurn(ctx, pctx.Prompt)
			if turnResult != nil {
				turnResults = []*TurnResult{turnResult}
			}
		}

		for _, currentTurn := range turnResults {
			if currentTurn == nil {
				continue
			}

			// Session ID update (shared — both providers)
			if currentTurn.SessionID != "" {
				turnExec.UpdateSessionID(currentTurn.SessionID)
				result.SessionID = currentTurn.SessionID
			}

			// Provider-specific post-turn (Codex: persist thread_id)
			if postErr := adapter.PostTurn(currentTurn, pctx, &cfg, we); postErr != nil {
				we.logger.Warn("post-turn failed", "provider", adapter.Name(), "error", postErr)
			}

			// Token accumulation — uniform, all fields, all providers
			if currentTurn.Usage != nil {
				result.InputTokens += int(currentTurn.Usage.InputTokens)
				result.OutputTokens += int(currentTurn.Usage.OutputTokens)
				result.CacheCreationTokens += int(currentTurn.Usage.CacheCreationInputTokens)
				result.CacheReadTokens += int(currentTurn.Usage.CacheReadInputTokens)
			}
			result.CostUSD += currentTurn.CostUSD
		}

		if err != nil {
			return result, fmt.Errorf("%s turn %d: %w", adapter.Name(), i+1, err)
		}

		// Parse status at orchestration level (authoritative for all providers + mocks).
		// Both real executors also parse in ExecuteTurn() for internal retry logic,
		// but orchestration-level parse is the single source of truth for completion.
		reviewRound := cfg.ReviewRound
		if reviewRound == 0 {
			reviewRound = 1
		}
		status, reason, parseErr := ParsePhaseSpecificResponse(cfg.PhaseID, reviewRound, turnResult.Content)
		if parseErr != nil {
			we.logger.Debug("parse phase response failed",
				"phase", cfg.PhaseID,
				"error", parseErr,
			)
			pctx.Prompt = fmt.Sprintf("Continue. Previous output was not valid JSON. Iteration %d/%d.",
				i+2, MaxOrcRetries)
			continue
		}

		switch status {
		case PhaseStatusComplete:
			// Verification gate (implementation phases only, skip in test mode)
			if isImplementationPhase(cfg.PhaseID) && we.turnExecutor == nil {
				if verifyErr := ValidateImplementCompletion(turnResult.Content); verifyErr != nil {
					we.logger.Info("implement verification gate failed, continuing iteration",
						"phase", cfg.PhaseID,
						"error", verifyErr.Error(),
					)
					pctx.Prompt = FormatVerificationFeedback(verifyErr)
					continue
				}
				we.logger.Info("implement verification gate passed", "phase", cfg.PhaseID)
			}

			// Quality checks
			if checkResult := we.runQualityChecks(ctx, cfg); checkResult != nil {
				if checkResult.HasBlocks {
					we.logger.Info("quality checks failed, continuing iteration",
						"phase", cfg.PhaseID,
						"failures", checkResult.FailureSummary(),
					)
					pctx.Prompt = FormatQualityChecksForPrompt(checkResult)
					continue
				}
				if !checkResult.AllPassed {
					we.logger.Warn("quality checks had warnings",
						"phase", cfg.PhaseID,
						"failures", checkResult.FailureSummary(),
					)
				} else {
					we.logger.Info("quality checks passed", "phase", cfg.PhaseID)
				}
			}

			result.RawOutput = turnResult.Content
			result.Content = extractPhaseOutput(turnResult.Content)
			return result, nil

		case PhaseStatusBlocked:
			result.RawOutput = turnResult.Content
			result.Content = extractPhaseOutput(turnResult.Content)
			return result, &PhaseBlockedError{
				Phase:  cfg.PhaseID,
				Reason: reason,
				Output: turnResult.Content,
			}

		case PhaseStatusContinue:
			pctx.Prompt = fmt.Sprintf("Continue working. Iteration %d/%d. %s",
				i+2, MaxOrcRetries, reason)
		}
	}

	return result, fmt.Errorf("max orc retries (%d) reached without completion (%s)", MaxOrcRetries, adapter.Name())
}

func (we *WorkflowExecutor) clearRetryStateForFreshPhaseStart(phaseID string, resumed bool) {
	if resumed || we.task == nil || !shouldStartFreshRetryPhase(we.task, phaseID) {
		return
	}

	task.ClearRetryState(we.task)
	if err := we.backend.SaveTask(we.task); err != nil {
		we.logger.Warn("failed to clear retry state for fresh phase start",
			"phase", phaseID,
			"error", err,
		)
		return
	}

	we.logger.Info("cleared retry state after starting fresh retry phase",
		"task", we.task.Id,
		"phase", phaseID,
	)
}

// updatePhaseIterations persists the current iteration count for real-time monitoring.
func (we *WorkflowExecutor) updatePhaseIterations(cfg PhaseExecutionConfig, iterations int) {
	if cfg.RunID != "" && cfg.PhaseID != "" {
		if err := we.backend.UpdatePhaseIterations(cfg.RunID, cfg.PhaseID, iterations); err != nil {
			we.logger.Warn("failed to update phase iterations", "phase", cfg.PhaseID, "error", err)
		}
	}
}

// loadPhasePrompt loads the prompt content for a phase template.
func (we *WorkflowExecutor) loadPhasePrompt(tmpl *db.PhaseTemplate) (string, error) {
	switch tmpl.PromptSource {
	case "embedded":
		// Load from embedded templates
		return we.loadEmbeddedPrompt(tmpl.PromptPath)

	case "db":
		// Use inline prompt content
		if tmpl.PromptContent == "" {
			return "", fmt.Errorf("phase %s has no prompt content", tmpl.ID)
		}
		return tmpl.PromptContent, nil

	case "file":
		// Load from file system
		return we.loadFilePrompt(tmpl.PromptPath)

	default:
		return "", fmt.Errorf("unknown prompt source: %s", tmpl.PromptSource)
	}
}

// loadEmbeddedPrompt loads a prompt from embedded templates.
// Tries the embed.FS first (works in production binary), falls back to filesystem (dev).
func (we *WorkflowExecutor) loadEmbeddedPrompt(path string) (string, error) {
	// Try embedded templates first (production path)
	content, err := templates.Prompts.ReadFile(path)
	if err == nil {
		return string(content), nil
	}

	// Fallback to filesystem for development (worktree has templates/ directory)
	fullPath := filepath.Join(we.workingDir, "templates", path)
	content, fsErr := os.ReadFile(fullPath)
	if fsErr != nil {
		return "", fmt.Errorf("load embedded prompt %s: embed: %w, file: %w", path, err, fsErr)
	}
	return string(content), nil
}

// loadFilePrompt loads a prompt from the file system.
func (we *WorkflowExecutor) loadFilePrompt(path string) (string, error) {
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		fullPath = filepath.Join(we.workingDir, ".orc", "prompts", path)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("load prompt file %s: %w", fullPath, err)
	}
	return string(content), nil
}

// loadSystemPromptFile loads a system prompt from embedded templates or user files.
// Path resolution:
//   - Paths starting with "system_prompts/" → load from embedded templates.SystemPrompts
//   - Absolute paths → load directly from filesystem
//   - Relative paths → load from .orc/system_prompts/ in working directory
func (we *WorkflowExecutor) loadSystemPromptFile(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Check if it's an embedded system prompt (built-in)
	if strings.HasPrefix(path, "system_prompts/") {
		content, err := templates.SystemPrompts.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("load embedded system prompt %s: %w", path, err)
		}
		return string(content), nil
	}

	// Otherwise, load from filesystem (user-configured)
	var fullPath string
	if filepath.IsAbs(path) {
		fullPath = path
	} else {
		// User system prompts in .orc/system_prompts/
		fullPath = filepath.Join(we.workingDir, ".orc", "system_prompts", path)
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("load system prompt file %s: %w", fullPath, err)
	}
	return string(content), nil
}

// resolveExecutorAgent resolves the executor agent for a phase.
// Priority: workflow_phases.agent_override > phase_templates.agent_id
// Returns nil if no agent is configured (agents are optional during transition period).
func (we *WorkflowExecutor) resolveExecutorAgent(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) *db.Agent {
	// 1. Check workflow_phases.agent_override (per-workflow override)
	if phase.AgentOverride != "" {
		agent, err := we.projectDB.GetAgent(phase.AgentOverride)
		if err != nil {
			we.logger.Warn("failed to load agent override",
				"agent_id", phase.AgentOverride,
				"phase", tmpl.ID,
				"error", err,
			)
			return nil
		}
		return agent
	}

	// 2. Use phase_templates.agent_id (phase template default)
	if tmpl.AgentID != "" {
		agent, err := we.projectDB.GetAgent(tmpl.AgentID)
		if err != nil {
			we.logger.Warn("failed to load phase agent",
				"agent_id", tmpl.AgentID,
				"phase", tmpl.ID,
				"error", err,
			)
			return nil
		}
		return agent
	}

	// No agent configured - caller should use fallback defaults
	return nil
}

// resolvePhaseModel determines which model to use for a phase.
// Priority: workflow phase override > workflow default_model > executor agent model > config default
// If the model string contains a provider prefix (e.g., "codex:gpt-5"), the prefix is stripped
// and only the model name is returned (e.g., "gpt-5"). Provider extraction is handled separately
// by resolvePhaseProvider.
func (we *WorkflowExecutor) resolvePhaseModel(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) string {
	// Workflow phase override takes precedence (per-workflow customization)
	if phase.ModelOverride != "" {
		if _, m := ParseProviderModel(phase.ModelOverride); m != "" {
			return m
		}
		return phase.ModelOverride
	}

	// Workflow default_model (workflow-level default)
	if we.wf != nil && we.wf.DefaultModel != "" {
		if _, m := ParseProviderModel(we.wf.DefaultModel); m != "" {
			return m
		}
		return we.wf.DefaultModel
	}

	// Executor agent model
	if agent := we.resolveExecutorAgent(tmpl, phase); agent != nil && agent.Model != "" {
		if _, m := ParseProviderModel(agent.Model); m != "" {
			return m
		}
		return agent.Model
	}

	// Provider-aware model resolution.
	// Resolve provider first — codex-family providers should NOT inherit the global
	// config model (which is typically a Claude model like "opus").
	provider := we.resolvePhaseProvider(tmpl, phase)
	if isCodexFamilyProvider(provider) {
		if we.orcConfig != nil && (provider == ProviderOllama || provider == ProviderLMStudio) {
			if m := we.orcConfig.Providers.Ollama.DefaultModel; m != "" {
				return m
			}
		}
		if m := providerDefaultModel(provider); m != "" {
			return m
		}
	}

	// Config default model (applies to primary provider, typically Claude)
	if we.orcConfig != nil && we.orcConfig.Model != "" {
		if _, m := ParseProviderModel(we.orcConfig.Model); m != "" {
			return m
		}
		return we.orcConfig.Model
	}

	// Ultimate fallback
	if m := providerDefaultModel(provider); m != "" {
		return m
	}
	return "opus"
}

// shouldUseThinking determines if extended thinking should be enabled.
// Resolution chain:
// 1. Phase ThinkingOverride (highest priority)
// 2. Workflow DefaultThinking (only when true; false falls through)
// 3. Template ThinkingEnabled
// 4. Phase-specific defaults (spec/review -> true, others -> false)
func (we *WorkflowExecutor) shouldUseThinking(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) bool {
	// Phase override takes precedence
	if phase.ThinkingOverride != nil {
		return *phase.ThinkingOverride
	}

	// Workflow default_thinking (only when explicitly true)
	if we.wf != nil && we.wf.DefaultThinking {
		return true
	}

	// Template default
	if tmpl.ThinkingEnabled != nil {
		return *tmpl.ThinkingEnabled
	}

	// Decision phases default to thinking
	switch tmpl.ID {
	case "spec", "review":
		return true
	}

	return false
}

// phaseTimeoutError wraps an error to indicate it was caused by PhaseMax timeout.
type phaseTimeoutError struct {
	phase   string
	timeout time.Duration
	taskID  string
	err     error
}

func (e *phaseTimeoutError) Error() string {
	return fmt.Sprintf("phase %s exceeded timeout (%v). Run 'orc resume %s' to retry.", e.phase, e.timeout, e.taskID)
}

func (e *phaseTimeoutError) Unwrap() error {
	return e.err
}

// IsPhaseTimeoutError returns true if the error is a phase timeout error.
func IsPhaseTimeoutError(err error) bool {
	var pte *phaseTimeoutError
	return errors.As(err, &pte)
}

// PhaseBlockedError signals a phase blocked but should proceed to gate evaluation.
// This is NOT a failure - gates decide whether to retry or fail the task.
// Used when phases output {"status": "blocked", "reason": "..."} to indicate
// the phase cannot complete without intervention (e.g., review found issues).
type PhaseBlockedError struct {
	Phase  string // Phase that blocked
	Reason string // Why it blocked
	Output string // Full phase output for storage/retry context
}

func (e *PhaseBlockedError) Error() string {
	return fmt.Sprintf("phase %s blocked: %s", e.Phase, e.Reason)
}

// IsPhaseBlockedError returns true if the error is a PhaseBlockedError.
func IsPhaseBlockedError(err error) bool {
	var pbe *PhaseBlockedError
	return errors.As(err, &pbe)
}

// executePhaseWithTimeout wraps executePhase with PhaseMax timeout if configured.
// PhaseMax=0 means unlimited (no timeout).
// Returns a phaseTimeoutError if the phase times out due to PhaseMax.
// Logs warnings at 50% and 75% of the timeout duration.
func (we *WorkflowExecutor) executePhaseWithTimeout(
	ctx context.Context,
	tmpl *db.PhaseTemplate,
	phase *db.WorkflowPhase,
	vars map[string]string,
	rctx *variable.ResolutionContext,
	run *db.WorkflowRun,
	runPhase *db.WorkflowRunPhase,
	t *orcv1.Task,
) (PhaseResult, error) {
	// Update task.CurrentPhase BEFORE phase execution begins (SC-1, SC-3).
	// This ensures `orc status` can read the current phase directly from the task record.
	if t != nil {
		provider := we.resolvePhaseProvider(tmpl, phase)
		model := we.resolvePhaseModel(tmpl, phase)
		task.SetCurrentPhaseProto(t, tmpl.ID)
		task.StartPhaseProto(t.Execution, tmpl.ID)
		setTaskSessionMetadata(t, tmpl.ID, provider, model)
		if err := we.backend.SaveTask(t); err != nil {
			// Non-fatal: workflow run still tracks the phase. Log and continue.
			we.logger.Warn("failed to save task phase start", "task", t.Id, "phase", tmpl.ID, "error", err)
		}
	}

	phaseMax := time.Duration(0)
	if we.orcConfig != nil {
		phaseMax = we.orcConfig.Timeouts.PhaseMax
	}

	if phaseMax <= 0 {
		// No timeout configured, execute directly
		return we.executePhase(ctx, tmpl, phase, vars, rctx, run, runPhase, t)
	}

	// Create timeout context for this phase
	phaseCtx, cancel := context.WithTimeout(ctx, phaseMax)
	defer cancel()

	// Get task ID for logging
	taskID := ""
	if t != nil {
		taskID = t.Id
	}

	// Start timeout monitoring goroutine for warnings at 50% and 75%
	startTime := time.Now()
	warningDone := make(chan struct{})
	go func() {
		defer close(warningDone)

		threshold50 := phaseMax / 2
		threshold75 := phaseMax * 3 / 4

		timer50 := time.NewTimer(threshold50)
		defer timer50.Stop()

		select {
		case <-phaseCtx.Done():
			return
		case <-timer50.C:
			elapsed := time.Since(startTime)
			remaining := phaseMax - elapsed
			we.logger.Warn("phase_max 50% elapsed",
				"phase", tmpl.ID,
				"task", taskID,
				"elapsed", elapsed.Round(time.Second),
				"timeout", phaseMax,
				"remaining", remaining.Round(time.Second),
			)
		}

		timer75 := time.NewTimer(threshold75 - threshold50)
		defer timer75.Stop()

		select {
		case <-phaseCtx.Done():
			return
		case <-timer75.C:
			elapsed := time.Since(startTime)
			remaining := phaseMax - elapsed
			we.logger.Warn("phase_max 75% elapsed",
				"phase", tmpl.ID,
				"task", taskID,
				"elapsed", elapsed.Round(time.Second),
				"timeout", phaseMax,
				"remaining", remaining.Round(time.Second),
			)
		}

		// Wait for context to complete
		<-phaseCtx.Done()
	}()

	result, err := we.executePhase(phaseCtx, tmpl, phase, vars, rctx, run, runPhase, t)

	// Capture the phase context error before canceling.
	// This determines if the timeout was reached (DeadlineExceeded) vs normal completion.
	phaseCtxErr := phaseCtx.Err()

	// Cancel the phase context to signal the warning goroutine to exit.
	// This must be called before waiting on warningDone to avoid deadlock.
	cancel()

	// Wait for warning goroutine to finish
	<-warningDone

	if err != nil {
		// Check if phase context timed out (but parent context is still alive)
		if phaseCtxErr == context.DeadlineExceeded && ctx.Err() == nil {
			we.logger.Error("phase timeout exceeded",
				"phase", tmpl.ID,
				"timeout", phaseMax,
				"task", taskID,
			)
			return result, &phaseTimeoutError{
				phase:   tmpl.ID,
				timeout: phaseMax,
				taskID:  taskID,
				err:     err,
			}
		}
	}
	return result, err
}

// runQualityChecks runs quality checks configured for the phase.
// Returns nil if no checks are configured.
func (we *WorkflowExecutor) runQualityChecks(ctx context.Context, cfg PhaseExecutionConfig) *QualityCheckResult {
	// Load quality checks from phase template (with workflow override)
	checks, err := LoadQualityChecksForPhase(cfg.PhaseTemplate, cfg.WorkflowPhase)
	if err != nil {
		we.logger.Warn("failed to load quality checks - continuing without checks",
			"phase", cfg.PhaseID,
			"error", err,
			"note", "check phase_templates.quality_checks JSON format",
		)
		return nil
	}

	commands := make(map[string]*db.ProjectCommand)
	if we.projectDB != nil {
		// Load project commands from database
		loadedCommands, err := we.projectDB.GetProjectCommandsMap()
		if err != nil {
			we.logger.Warn("failed to load project commands - code checks may not run",
				"phase", cfg.PhaseID,
				"error", err,
				"hint", "run 'orc config commands' to view/configure",
			)
		} else {
			commands = loadedCommands
		}
	}

	if isImplementationPhase(cfg.PhaseID) {
		if we.projectDB == nil {
			we.logger.Debug("skipping hard implement verification checks without project database",
				"phase", cfg.PhaseID,
			)
		}
		requiredChecks := buildRequiredImplementChecks(commands)
		if we.projectDB != nil && len(requiredChecks) == 0 {
			we.logger.Warn("no enabled project verification commands configured for implementation hard gate",
				"phase", cfg.PhaseID,
				"hint", "configure project commands with 'orc config commands' or re-run 'orc init'",
			)
		}
		checks = mergeRequiredImplementationChecks(checks, requiredChecks)
	}

	if len(checks) == 0 {
		// No checks configured for this phase
		we.logger.Debug("no quality checks configured for phase", "phase", cfg.PhaseID)
		return nil
	}

	we.logger.Info("running quality checks", "phase", cfg.PhaseID, "check_count", len(checks))

	// Create and run the quality check runner
	runner := NewQualityCheckRunner(
		cfg.WorkingDir,
		checks,
		commands,
		we.logger,
	)

	return runner.Run(ctx)
}

// getEffectivePhaseClaudeConfig resolves the effective Claude configuration for a phase.
// Priority order:
//  1. workflow_phases.claude_config_override (per-workflow override)
//  2. executor agent claude_config (from resolved agent)
//
// After merging, it also:
//  3. Resolves agent_ref to merge agent configuration
//  4. Loads skill_refs to inject skill content into AppendSystemPrompt
//  5. Sets system prompt from executor agent (DB-first approach)
func (we *WorkflowExecutor) getEffectivePhaseClaudeConfig(tmpl *db.PhaseTemplate, phase *db.WorkflowPhase) *PhaseClaudeConfig {
	var cfg *PhaseClaudeConfig

	// 1. Load executor agent's ClaudeConfig as base
	if agent := we.resolveExecutorAgent(tmpl, phase); agent != nil && agent.ClaudeConfig != "" {
		base, err := ParsePhaseClaudeConfig(agent.ClaudeConfig)
		if err != nil {
			we.logger.Warn("failed to parse agent claude_config",
				"agent_id", agent.ID,
				"phase", tmpl.ID,
				"error", err,
			)
		} else if base != nil {
			cfg = base
		}
	}

	// 2. Merge template's claude_config (between agent and workflow override)
	if tmpl != nil && tmpl.ClaudeConfig != "" {
		tmplCfg, err := ParsePhaseClaudeConfig(tmpl.ClaudeConfig)
		if err != nil {
			we.logger.Warn("failed to parse template claude_config",
				"phase", tmpl.ID,
				"error", err,
			)
		} else if tmplCfg != nil {
			if cfg == nil {
				cfg = tmplCfg
			} else {
				cfg = cfg.Merge(tmplCfg)
			}
		}
	}

	// 3. Merge workflow phase override (can override agent + template config)
	if phase != nil && phase.ClaudeConfigOverride != "" {
		override, err := ParsePhaseClaudeConfig(phase.ClaudeConfigOverride)
		if err != nil {
			we.logger.Warn("failed to parse workflow phase claude_config_override",
				"phase", phase.PhaseTemplateID,
				"error", err,
			)
		} else if override != nil {
			if cfg == nil {
				cfg = override
			} else {
				cfg = cfg.Merge(override)
			}
		}
	}

	// Ensure we have a config to set system prompt
	if cfg == nil {
		cfg = &PhaseClaudeConfig{}
	}

	// 4. Resolve agent reference (from claude_config JSON)
	if cfg.AgentRef != "" {
		claudeDir := filepath.Join(we.workingDir, ".claude")
		resolver := NewAgentResolver(we.workingDir, claudeDir)
		if err := resolver.ResolveAgentConfig(cfg); err != nil {
			we.logger.Warn("failed to resolve agent reference",
				"agent_ref", cfg.AgentRef,
				"error", err,
			)
			// Continue without agent config - it's not fatal
		}
	}

	// 5. Resolve system prompt (DB-first approach)
	// Priority: executor_agent.system_prompt (DB) > claude_config.system_prompt_file > claude_config.system_prompt
	if agent := we.resolveExecutorAgent(tmpl, phase); agent != nil && agent.SystemPrompt != "" {
		// DB-stored system prompt takes precedence (editable via UI)
		cfg.SystemPrompt = agent.SystemPrompt
	} else if cfg.SystemPromptFile != "" {
		// Fallback: file reference (for backward compatibility)
		content, err := we.loadSystemPromptFile(cfg.SystemPromptFile)
		if err != nil {
			we.logger.Warn("failed to load system prompt file",
				"file", cfg.SystemPromptFile,
				"error", err,
			)
			// Continue without system prompt - it's not fatal
		} else if content != "" {
			cfg.SystemPrompt = content
		}
	}
	// cfg.SystemPrompt (inline in JSON) is already set if present

	// 6. Resolve AppendSystemPromptFile to content
	if cfg.AppendSystemPromptFile != "" {
		content, err := we.loadSystemPromptFile(cfg.AppendSystemPromptFile)
		if err != nil {
			we.logger.Warn("failed to load append system prompt file",
				"file", cfg.AppendSystemPromptFile,
				"error", err,
			)
			// Continue without append system prompt - it's not fatal
		} else if content != "" {
			// Append to existing AppendSystemPrompt if any
			if cfg.AppendSystemPrompt != "" {
				cfg.AppendSystemPrompt = content + "\n\n" + cfg.AppendSystemPrompt
			} else {
				cfg.AppendSystemPrompt = content
			}
		}
	}

	// Return nil if config is empty (no special configuration)
	if cfg.IsEmpty() {
		return nil
	}

	return cfg
}

// normalizeCodexExecutionModel strips the provider prefix from model names
// when routing through Codex CLI (e.g., "ollama/qwen2.5" -> "qwen2.5").
func normalizeCodexExecutionModel(provider, model string) string {
	// Strip "provider/" prefix if the model string includes it
	if idx := strings.Index(model, "/"); idx >= 0 {
		prefix := model[:idx]
		if normalizeProvider(prefix) == normalizeProvider(provider) {
			return model[idx+1:]
		}
	}
	return model
}

// setTaskSessionMetadata records session metadata on the task for monitoring.
// Stores provider and model as task-level metadata keyed by phase ID since
// PhaseState does not have a metadata field.
func setTaskSessionMetadata(t *orcv1.Task, phaseID, provider, model string) {
	if t == nil {
		return
	}
	if t.Metadata == nil {
		t.Metadata = make(map[string]string)
	}
	t.Metadata["phase:"+phaseID+":provider"] = provider
	t.Metadata["phase:"+phaseID+":model"] = model
}
