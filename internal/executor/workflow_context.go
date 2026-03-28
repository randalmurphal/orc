// workflow_context.go contains context building and variable resolution for workflow execution.
// This includes building resolution context, loading initiative data, project detection,
// and enriching context with phase-specific information.
package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/brief"
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/gate"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
)

const (
	promptContextThreadMessageLimit = 6
	promptContextThreadLinkLimit    = 8
	promptContextThreadDraftLimit   = 5
)

// buildContextData creates the context data JSON for a run.
func (we *WorkflowExecutor) buildContextData(opts WorkflowRunOptions) string {
	data := map[string]any{
		"prompt":       opts.Prompt,
		"instructions": opts.Instructions,
	}

	switch opts.ContextType {
	case ContextTask:
		data["task_id"] = opts.TaskID
	case ContextBranch:
		data["branch"] = opts.Branch
	case ContextPR:
		data["pr_id"] = opts.PRID
	}

	j, _ := json.Marshal(data)
	return string(j)
}

// buildResolutionContext creates the variable resolution context.
func (we *WorkflowExecutor) buildResolutionContext(
	opts WorkflowRunOptions,
	t *orcv1.Task,
	wf *db.Workflow,
	run *db.WorkflowRun,
) *variable.ResolutionContext {
	// Use effectiveWorkingDir() to get worktree path if one was created
	workDir := we.effectiveWorkingDir()
	rctx := &variable.ResolutionContext{
		WorkflowID:      wf.ID,
		WorkflowRunID:   run.ID,
		Prompt:          opts.Prompt,
		Instructions:    opts.Instructions,
		WorkingDir:      workDir,
		ProjectRoot:     workDir,
		PriorOutputs:    make(map[string]string),
		PhaseOutputVars: make(map[string]string),
	}

	if t != nil {
		rctx.TaskID = t.Id
		rctx.TaskTitle = t.Title
		rctx.TaskDescription = task.GetDescriptionProto(t)
		rctx.TaskCategory = t.Category.String()
		rctx.TaskWeight = task.GetWorkflowIDProto(t) // Use workflow ID for WEIGHT variable
		rctx.TaskBranch = t.Branch
		rctx.RequiresUITesting = t.RequiresUiTesting

		// Resolve target branch using 6-level priority
		rctx.TargetBranch = we.resolveTargetBranch(t)

		// Load initiative context if task belongs to an initiative
		initiativeID := task.GetInitiativeIDProto(t)
		if initiativeID != "" {
			we.loadInitiativeContext(rctx, initiativeID)
		}

		// Set up screenshot dir for UI testing tasks
		if t.RequiresUiTesting && we.workingDir != "" {
			rctx.ScreenshotDir = task.ScreenshotsPath(we.workingDir, t.Id)
			if err := os.MkdirAll(rctx.ScreenshotDir, 0755); err != nil {
				we.logger.Warn("failed to create screenshot directory", "error", err)
			}
		}

		// Load QA E2E specific context from task metadata
		if t.Metadata != nil {
			// Before images for visual comparison
			if images, ok := t.Metadata["before_images"]; ok {
				rctx.BeforeImages = images
			}
			// Max iterations override (workflow default can be overridden per-task)
			if maxIter, ok := t.Metadata["qa_max_loops"]; ok {
				if n, err := strconv.Atoi(maxIter); err == nil && n > 0 {
					rctx.QAMaxLoops = n
				}
			}
		}
	}

	// Load constitution content (project-level principles)
	if content, _, err := we.backend.LoadConstitution(); err == nil && content != "" {
		rctx.ConstitutionContent = content
	}

	// Load project detection from database
	we.loadProjectDetectionContext(rctx)

	// Set testing configuration from orc config
	if we.orcConfig != nil {
		rctx.CoverageThreshold = we.orcConfig.Testing.CoverageThreshold
	}

	// Set error patterns from orc config
	if we.orcConfig != nil && we.orcConfig.ErrorPatterns != "" {
		rctx.ErrorPatterns = we.orcConfig.ErrorPatterns
	}

	// Merge user-provided variables
	if opts.Variables != nil {
		rctx.Environment = opts.Variables
	}

	return rctx
}

// loadInitiativeContext loads initiative data into the resolution context.
func (we *WorkflowExecutor) loadInitiativeContext(rctx *variable.ResolutionContext, initiativeID string) {
	init, err := we.backend.LoadInitiative(initiativeID)
	if err != nil {
		we.logger.Debug("failed to load initiative",
			"initiative_id", initiativeID,
			"error", err,
		)
		return
	}

	rctx.InitiativeID = init.ID
	rctx.InitiativeTitle = init.Title
	rctx.InitiativeVision = init.Vision

	// Format decisions as markdown
	if len(init.Decisions) > 0 {
		var sb strings.Builder
		for _, d := range init.Decisions {
			fmt.Fprintf(&sb, "- **%s**: %s", d.ID, d.Decision)
			if d.Rationale != "" {
				fmt.Fprintf(&sb, " (%s)", d.Rationale)
			}
			sb.WriteString("\n")
		}
		rctx.InitiativeDecisions = strings.TrimSuffix(sb.String(), "\n")
	}

	// Load initiative notes and format grouped by type
	// Per DEC-003: Human notes always inject, agent notes only if graduated
	rctx.InitiativeNotes = we.loadAndFormatInitiativeNotes(initiativeID)

	we.logger.Debug("initiative context loaded",
		"initiative_id", init.ID,
		"has_vision", init.Vision != "",
		"decision_count", len(init.Decisions),
		"has_notes", rctx.InitiativeNotes != "",
	)
}

// loadAndFormatInitiativeNotes loads initiative notes and formats them grouped by type.
// Per DEC-003: Human notes always inject, agent notes only if graduated (met strict bar).
// Per DEC-004: Group notes by type for easy scanning.
func (we *WorkflowExecutor) loadAndFormatInitiativeNotes(initiativeID string) string {
	notes, err := we.backend.GetInitiativeNotes(initiativeID)
	if err != nil {
		we.logger.Debug("failed to load initiative notes",
			"initiative_id", initiativeID,
			"error", err,
		)
		return ""
	}

	if len(notes) == 0 {
		return ""
	}

	// Filter notes: human notes always, agent notes only if graduated
	var filtered []db.InitiativeNote
	for _, n := range notes {
		if n.AuthorType == db.NoteAuthorHuman || (n.AuthorType == db.NoteAuthorAgent && n.Graduated) {
			filtered = append(filtered, n)
		}
	}

	if len(filtered) == 0 {
		return ""
	}

	// Group notes by type (patterns, warnings, learnings, handoffs)
	byType := make(map[string][]db.InitiativeNote)
	for _, n := range filtered {
		byType[n.NoteType] = append(byType[n.NoteType], n)
	}

	// Format notes grouped by type in a consistent order
	var sb strings.Builder
	typeOrder := []struct {
		noteType string
		label    string
		emoji    string
	}{
		{db.NoteTypePattern, "Patterns", "📋"},
		{db.NoteTypeWarning, "Warnings", "⚠️"},
		{db.NoteTypeLearning, "Learnings", "💡"},
		{db.NoteTypeHandoff, "Handoffs", "🤝"},
	}

	for _, t := range typeOrder {
		notes := byType[t.noteType]
		if len(notes) == 0 {
			continue
		}

		fmt.Fprintf(&sb, "**%s %s:**\n", t.emoji, t.label)
		for _, n := range notes {
			fmt.Fprintf(&sb, "- %s", n.Content)
			if n.SourceTask != "" {
				fmt.Fprintf(&sb, " *(from %s)*", n.SourceTask)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// loadProjectDetectionContext loads project detection data into the resolution context.
// It reads from both the legacy detection table and the newer project_languages table.
// project_languages is the authoritative source for build_command and per-language data.
func (we *WorkflowExecutor) loadProjectDetectionContext(rctx *variable.ResolutionContext) {
	dbBackend, ok := we.backend.(*storage.DatabaseBackend)
	if !ok {
		return
	}

	pdb := dbBackend.DB()

	// Load from legacy detection table (language, frameworks, test/lint commands)
	detection, err := pdb.LoadDetection()
	if err != nil || detection == nil {
		return
	}

	rctx.Language = detection.Language
	rctx.HasTests = detection.HasTests
	rctx.TestCommand = detection.TestCommand
	rctx.LintCommand = detection.LintCommand
	rctx.Frameworks = detection.Frameworks

	// Supplement with project_languages data (has build_command, per-language overrides)
	primaryLang, err := pdb.GetPrimaryLanguage()
	if err == nil && primaryLang != nil {
		rctx.BuildCommand = primaryLang.BuildCommand

		// project_languages may have more accurate per-language commands
		if primaryLang.TestCommand != "" {
			rctx.TestCommand = primaryLang.TestCommand
		}
		if primaryLang.LintCommand != "" {
			rctx.LintCommand = primaryLang.LintCommand
		}
	}

	// HasFrontend: prefer DB query (checks language, root_path, frameworks) over framework switch
	if hasFE, feErr := pdb.HasFrontend(); feErr == nil {
		rctx.HasFrontend = hasFE
	} else {
		// Fallback to framework-based detection from legacy detection table
		for _, f := range detection.Frameworks {
			switch f {
			case "react", "vue", "angular", "svelte", "nextjs", "nuxt", "gatsby", "astro":
				rctx.HasFrontend = true
			}
		}
	}
}

// enrichContextForPhase adds phase-specific context to the resolution context.
// Call this before executing each phase to load review findings, artifacts, etc.
func (we *WorkflowExecutor) enrichContextForPhase(
	rctx *variable.ResolutionContext,
	phaseID string,
	t *orcv1.Task,
	threadUsage threadVariableUsage,
) error {
	if t == nil {
		clearThreadContext(rctx)
		return nil
	}

	// Load structured retry fields from task metadata
	PopulateRetryFields(rctx, t)

	// Note: Review round detection now uses LoopIteration from the loop system.
	// ReviewFindings is populated via output_transform "format_findings" in workflow phase config.

	// Load test results for review phase
	if phaseID == "review" {
		rctx.TestResults = we.loadPriorPhaseContentProto(t.Id, t.Execution, "test")
	}

	// Load TDD test plan if it exists
	rctx.TDDTestPlan = we.loadPriorPhaseContentProto(t.Id, t.Execution, "tdd_write_plan")

	// Generate project brief from task history
	we.populateProjectBrief(rctx)

	// Load scratchpad entries from prior phases for PREV_SCRATCHPAD
	we.populateScratchpadContext(rctx, t.Id, phaseID)

	// Load automation context for automation tasks
	if t.IsAutomation {
		we.loadAutomationContextProto(rctx, t)
	}

	if threadUsage.Any() {
		if err := we.populateThreadContext(rctx, t); err != nil {
			return err
		}
	} else {
		clearThreadContext(rctx)
	}

	return nil
}

func clearThreadContext(rctx *variable.ResolutionContext) {
	rctx.ThreadID = ""
	rctx.ThreadTitle = ""
	rctx.ThreadContext = ""
	rctx.ThreadHistory = ""
	rctx.ThreadLinkedContext = ""
	rctx.ThreadRecommendationDrafts = ""
	rctx.ThreadDecisionDrafts = ""
}

func (we *WorkflowExecutor) populateThreadContext(rctx *variable.ResolutionContext, t *orcv1.Task) error {
	clearThreadContext(rctx)

	thread, err := we.loadPromptContextThread(t)
	if err != nil {
		return err
	}
	if thread == nil {
		return nil
	}

	rctx.ThreadID = thread.ID
	rctx.ThreadTitle = thread.Title
	rctx.ThreadHistory = db.FormatThreadMessagesForPrompt(thread.Messages, promptContextThreadMessageLimit)
	rctx.ThreadLinkedContext = db.FormatThreadLinksForPrompt(thread.Links, promptContextThreadLinkLimit)
	rctx.ThreadRecommendationDrafts = db.FormatThreadRecommendationDraftsForPrompt(thread.RecommendationDrafts, promptContextThreadDraftLimit)
	rctx.ThreadDecisionDrafts = db.FormatThreadDecisionDraftsForPrompt(thread.DecisionDrafts, promptContextThreadDraftLimit)
	rctx.ThreadContext = joinPromptSections(
		sectionIfPresent("Thread", fmt.Sprintf("- %s (%s)", thread.Title, thread.ID)),
		sectionIfPresent("Linked context", rctx.ThreadLinkedContext),
		sectionIfPresent("Recommendation drafts", rctx.ThreadRecommendationDrafts),
		sectionIfPresent("Decision drafts", rctx.ThreadDecisionDrafts),
		sectionIfPresent("Recent thread history", rctx.ThreadHistory),
	)
	return nil
}

func (we *WorkflowExecutor) loadPromptContextThread(t *orcv1.Task) (*db.Thread, error) {
	if t.Id != "" {
		thread, err := we.loadPromptContextThreadForList(db.ThreadListOpts{
			TaskID: t.Id,
			Status: db.ThreadStatusActive,
			Limit:  1,
		}, "task discussion threads for "+t.Id)
		if err != nil {
			return nil, err
		}
		if thread != nil {
			return thread, nil
		}

		thread, err = we.loadPromptContextThreadForList(db.ThreadListOpts{
			TaskID: t.Id,
			Limit:  1,
		}, "task discussion threads for "+t.Id)
		if err != nil {
			return nil, err
		}
		if thread != nil {
			return thread, nil
		}
	}

	if initiativeID := task.GetInitiativeIDProto(t); initiativeID != "" {
		thread, err := we.loadPromptContextThreadForList(db.ThreadListOpts{
			InitiativeID: initiativeID,
			Status:       db.ThreadStatusActive,
			Limit:        1,
		}, "initiative discussion threads for "+initiativeID)
		if err != nil {
			return nil, err
		}
		if thread != nil {
			return thread, nil
		}

		thread, err = we.loadPromptContextThreadForList(db.ThreadListOpts{
			InitiativeID: initiativeID,
			Limit:        1,
		}, "initiative discussion threads for "+initiativeID)
		if err != nil {
			return nil, err
		}
		if thread != nil {
			return thread, nil
		}
	}

	return nil, nil
}

func (we *WorkflowExecutor) loadPromptContextThreadForList(opts db.ThreadListOpts, description string) (*db.Thread, error) {
	threads, err := we.backend.DB().ListThreads(opts)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", description, err)
	}
	if len(threads) == 0 {
		return nil, nil
	}

	thread, err := we.backend.DB().GetThread(threads[0].ID)
	if err != nil {
		return nil, fmt.Errorf("load discussion thread %s: %w", threads[0].ID, err)
	}
	return thread, nil
}

func sectionIfPresent(title, content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	return fmt.Sprintf("## %s\n%s", title, content)
}

func joinPromptSections(sections ...string) string {
	parts := make([]string, 0, len(sections))
	for _, section := range sections {
		section = strings.TrimSpace(section)
		if section == "" {
			continue
		}
		parts = append(parts, section)
	}
	return strings.Join(parts, "\n\n")
}

func (we *WorkflowExecutor) populateControlPlaneContext(
	rctx *variable.ResolutionContext,
	phaseID string,
	currentTask *orcv1.Task,
	usage controlPlaneVariableUsage,
) error {
	rctx.PendingRecommendations = ""
	rctx.CompletionRecommendations = ""
	rctx.HandoffContext = ""
	rctx.AttentionSummary = ""

	if usage.needsRecommendations() {
		recommendations, err := we.backend.LoadAllRecommendations()
		if err != nil {
			return fmt.Errorf("load recommendations for control-plane context: %w", err)
		}

		if usage.PendingRecommendations {
			rctx.PendingRecommendations = formatPendingRecommendations(recommendations)
		}
		if usage.CompletionRecommendations {
			rctx.CompletionRecommendations = formatCompletionRecommendations(currentTask, recommendations)
		}
		if usage.HandoffContext {
			rctx.HandoffContext = formatHandoffContext(currentTask, phaseID, recommendations)
		}
	}

	if !usage.AttentionSummary {
		return nil
	}

	signals, err := we.backend.LoadActiveAttentionSignals()
	if err != nil {
		return fmt.Errorf("load attention signals for control-plane context: %w", err)
	}
	tasks, err := we.backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("load tasks for control-plane context: %w", err)
	}
	signals = controlplane.MergeTaskAttentionSignals("", tasks, signals)
	promptSignals, err := buildPromptAttentionSignals(we.backend, signals)
	if err != nil {
		return fmt.Errorf("format attention signals for control-plane context: %w", err)
	}
	promptSignals = append(promptSignals, promptPendingDecisionSignals(we.projectIDForEvents(), we.pendingDecisions)...)
	sort.Slice(promptSignals, func(i, j int) bool {
		if promptSignals[i].TaskID == promptSignals[j].TaskID {
			return promptSignals[i].Kind < promptSignals[j].Kind
		}
		return promptSignals[i].TaskID < promptSignals[j].TaskID
	})
	rctx.AttentionSummary = controlplane.FormatAttentionSummary(promptSignals)
	return nil
}

func formatPendingRecommendations(recommendations []*orcv1.Recommendation) string {
	candidates := make([]controlplane.RecommendationCandidate, 0, len(recommendations))
	for _, recommendation := range recommendations {
		if recommendation.GetStatus() != orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING {
			continue
		}

		candidates = append(candidates, controlplane.RecommendationCandidate{
			Kind:           recommendationKindName(recommendation.GetKind()),
			Title:          recommendation.GetTitle(),
			Summary:        recommendation.GetSummary(),
			ProposedAction: recommendation.GetProposedAction(),
			Evidence:       recommendation.GetEvidence(),
			DedupeKey:      recommendation.GetDedupeKey(),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].DedupeKey == candidates[j].DedupeKey {
			return candidates[i].Title < candidates[j].Title
		}
		return candidates[i].DedupeKey < candidates[j].DedupeKey
	})

	return controlplane.FormatRecommendationSummary(candidates)
}

func formatCompletionRecommendations(
	currentTask *orcv1.Task,
	recommendations []*orcv1.Recommendation,
) string {
	if currentTask == nil {
		return ""
	}

	candidates := make([]controlplane.RecommendationCandidate, 0, len(recommendations))
	for _, recommendation := range recommendations {
		if recommendation.GetSourceTaskId() != currentTask.GetId() {
			continue
		}

		candidates = append(candidates, controlplane.RecommendationCandidate{
			Kind:           recommendationKindName(recommendation.GetKind()),
			Title:          recommendation.GetTitle(),
			Summary:        recommendation.GetSummary(),
			ProposedAction: recommendation.GetProposedAction(),
			Evidence:       recommendation.GetEvidence(),
			DedupeKey:      recommendation.GetDedupeKey(),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].DedupeKey == candidates[j].DedupeKey {
			return candidates[i].Title < candidates[j].Title
		}
		return candidates[i].DedupeKey < candidates[j].DedupeKey
	})

	return controlplane.FormatRecommendationSummary(candidates)
}

func formatHandoffContext(
	currentTask *orcv1.Task,
	phaseID string,
	recommendations []*orcv1.Recommendation,
) string {
	if currentTask == nil {
		return ""
	}

	pack := controlplane.HandoffPack{
		TaskID:       currentTask.GetId(),
		TaskTitle:    currentTask.GetTitle(),
		CurrentPhase: phaseID,
		Summary:      task.GetDescriptionProto(currentTask),
		NextSteps:    handoffNextSteps(currentTask.GetId(), recommendations),
		Risks:        handoffRisks(currentTask.GetId(), recommendations),
	}

	return controlplane.FormatHandoffPack(pack)
}

func buildPromptAttentionSignals(
	backend storage.Backend,
	signals []*controlplane.PersistedAttentionSignal,
) ([]controlplane.AttentionSignal, error) {
	promptSignals := make([]controlplane.AttentionSignal, 0, len(signals))
	for _, persistedSignal := range signals {
		if persistedSignal == nil {
			continue
		}

		promptSignal, err := promptAttentionSignal(backend, persistedSignal)
		if err != nil {
			return nil, err
		}
		promptSignals = append(promptSignals, promptSignal)
	}

	return promptSignals, nil
}

func promptPendingDecisionSignals(
	projectID string,
	store *gate.PendingDecisionStore,
) []controlplane.AttentionSignal {
	if projectID == "" || store == nil {
		return nil
	}

	decisions := store.List(projectID)
	signals := make([]controlplane.AttentionSignal, 0, len(decisions))
	for _, decision := range decisions {
		if decision == nil {
			continue
		}

		summary := decision.Question
		if decision.Context != "" {
			summary = strings.TrimSpace(summary + "\n" + decision.Context)
		}
		signals = append(signals, controlplane.AttentionSignal{
			Kind:    string(controlplane.AttentionSignalKindDecisionRequest),
			TaskID:  decision.TaskID,
			Title:   decision.TaskTitle,
			Status:  "pending_decision",
			Phase:   decision.Phase,
			Summary: summary,
		})
	}

	return signals
}

func recommendationKindName(kind orcv1.RecommendationKind) string {
	return strings.TrimPrefix(strings.ToLower(kind.String()), "recommendation_kind_")
}

func taskStatusName(status orcv1.TaskStatus) string {
	return strings.TrimPrefix(strings.ToLower(status.String()), "task_status_")
}

func attentionSummaryForTask(taskItem *orcv1.Task) string {
	return controlplane.TaskAttentionSummary(taskItem)
}

func promptAttentionSignal(
	backend storage.Backend,
	persistedSignal *controlplane.PersistedAttentionSignal,
) (controlplane.AttentionSignal, error) {
	if persistedSignal == nil {
		return controlplane.AttentionSignal{}, fmt.Errorf("attention signal is required")
	}

	promptSignal := controlplane.AttentionSignal{
		Kind:    string(persistedSignal.Kind),
		TaskID:  persistedSignal.ReferenceID,
		Title:   persistedSignal.Title,
		Status:  persistedSignal.Status,
		Summary: persistedSignal.Summary,
	}

	switch persistedSignal.ReferenceType {
	case controlplane.AttentionSignalReferenceTypeTask:
		taskItem, err := backend.LoadTask(persistedSignal.ReferenceID)
		if err != nil {
			return controlplane.AttentionSignal{}, fmt.Errorf(
				"load task %s for attention signal %s: %w",
				persistedSignal.ReferenceID,
				persistedSignal.ID,
				err,
			)
		}
		if taskItem == nil {
			return controlplane.AttentionSignal{}, fmt.Errorf(
				"task %s for attention signal %s not found",
				persistedSignal.ReferenceID,
				persistedSignal.ID,
			)
		}

		promptSignal.TaskID = taskItem.GetId()
		if promptSignal.Title == "" {
			promptSignal.Title = taskItem.GetTitle()
		}
		if promptSignal.Status == "" {
			promptSignal.Status = taskStatusName(taskItem.GetStatus())
		}
		promptSignal.Phase = task.GetCurrentPhaseProto(taskItem)
		if promptSignal.Summary == "" {
			promptSignal.Summary = attentionSummaryForTask(taskItem)
		}

	case controlplane.AttentionSignalReferenceTypeRun:
		run, err := backend.GetWorkflowRun(persistedSignal.ReferenceID)
		if err != nil {
			return controlplane.AttentionSignal{}, fmt.Errorf(
				"load run %s for attention signal %s: %w",
				persistedSignal.ReferenceID,
				persistedSignal.ID,
				err,
			)
		}
		if run == nil {
			return controlplane.AttentionSignal{}, fmt.Errorf(
				"run %s for attention signal %s not found",
				persistedSignal.ReferenceID,
				persistedSignal.ID,
			)
		}
		if run.TaskID != nil && *run.TaskID != "" {
			taskItem, err := backend.LoadTask(*run.TaskID)
			if err != nil {
				return controlplane.AttentionSignal{}, fmt.Errorf(
					"load task %s for attention signal %s: %w",
					*run.TaskID,
					persistedSignal.ID,
					err,
				)
			}
			if taskItem == nil {
				return controlplane.AttentionSignal{}, fmt.Errorf(
					"task %s for attention signal %s not found",
					*run.TaskID,
					persistedSignal.ID,
				)
			}
			promptSignal.TaskID = taskItem.GetId()
			if promptSignal.Title == "" {
				promptSignal.Title = taskItem.GetTitle()
			}
			if promptSignal.Status == "" {
				promptSignal.Status = taskStatusName(taskItem.GetStatus())
			}
			promptSignal.Phase = task.GetCurrentPhaseProto(taskItem)
			if promptSignal.Summary == "" {
				promptSignal.Summary = attentionSummaryForTask(taskItem)
			}
		}
	}

	return promptSignal, nil
}

func handoffNextSteps(taskID string, recommendations []*orcv1.Recommendation) []string {
	steps := make([]string, 0)
	seen := make(map[string]struct{})

	for _, recommendation := range recommendations {
		if recommendation.GetSourceTaskId() != taskID {
			continue
		}
		if recommendation.GetStatus() != orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING {
			continue
		}
		if recommendation.GetKind() == orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK {
			continue
		}

		step := strings.TrimSpace(recommendation.GetProposedAction())
		if step == "" {
			step = strings.TrimSpace(recommendation.GetSummary())
		}
		if step == "" {
			step = strings.TrimSpace(recommendation.GetTitle())
		}
		if step == "" {
			continue
		}
		if _, exists := seen[step]; exists {
			continue
		}

		seen[step] = struct{}{}
		steps = append(steps, step)
	}

	sort.Strings(steps)
	return steps
}

func handoffRisks(taskID string, recommendations []*orcv1.Recommendation) []string {
	risks := make([]string, 0)
	seen := make(map[string]struct{})

	for _, recommendation := range recommendations {
		if recommendation.GetSourceTaskId() != taskID {
			continue
		}
		if recommendation.GetStatus() != orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING {
			continue
		}
		if recommendation.GetKind() != orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK {
			continue
		}

		risk := strings.TrimSpace(recommendation.GetTitle())
		summary := strings.TrimSpace(recommendation.GetSummary())
		if risk == "" {
			risk = summary
		} else if summary != "" {
			risk = risk + ": " + summary
		}
		if risk == "" {
			continue
		}
		if _, exists := seen[risk]; exists {
			continue
		}

		seen[risk] = struct{}{}
		risks = append(risks, risk)
	}

	sort.Strings(risks)
	return risks
}

// populateProjectBrief generates a project brief and populates rctx.ProjectBrief.
// Requires *storage.DatabaseBackend — silently skips for other backend types.
func (we *WorkflowExecutor) populateProjectBrief(rctx *variable.ResolutionContext) {
	dbBackend, ok := we.backend.(*storage.DatabaseBackend)
	if !ok {
		return
	}

	// Lazily create brief generator (persists across phases for caching)
	if we.briefGenerator == nil {
		cfg := brief.DefaultConfig()
		if we.orcConfig != nil {
			cfg.MaxTokens = we.orcConfig.Brief.MaxTokens
			cfg.StaleThreshold = we.orcConfig.Brief.StaleThreshold
		}
		cfg.CachePath = filepath.Join(we.workingDir, ".orc", "brief-cache.json")
		we.briefGenerator = brief.NewGenerator(dbBackend, cfg)
	}

	b, err := we.briefGenerator.Generate(context.Background())
	if err != nil {
		we.logger.Warn("failed to generate project brief", "error", err)
		return
	}

	rctx.ProjectBrief = brief.FormatBrief(b)
}

// formatRecentCompletedTasksForPrompt formats recent completed tasks as a markdown list.
func formatRecentCompletedTasksForPrompt(tasks []*orcv1.Task, limit int) string {
	var completed []*orcv1.Task
	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			completed = append(completed, t)
		}
	}

	// Sort by completion time (most recent first) - already done by LoadAllTasks
	if len(completed) > limit {
		completed = completed[:limit]
	}

	var sb strings.Builder
	for _, t := range completed {
		fmt.Fprintf(&sb, "- **%s**: %s", t.Id, t.Title)
		if t.Category != orcv1.TaskCategory_TASK_CATEGORY_UNSPECIFIED {
			fmt.Fprintf(&sb, " [%s]", task.CategoryFromProto(t.Category))
		}
		if workflowID := task.GetWorkflowIDProto(t); workflowID != "" {
			fmt.Fprintf(&sb, " (%s)", workflowID)
		}
		sb.WriteString("\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// collectRecentChangedFilesForPrompt collects files changed in recent tasks.
func collectRecentChangedFilesForPrompt(tasks []*orcv1.Task, limit int) string {
	var recent []*orcv1.Task
	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			recent = append(recent, t)
		}
	}

	if len(recent) > limit {
		recent = recent[:limit]
	}

	seen := make(map[string]bool)
	var files []string

	for _, t := range recent {
		if t.Metadata == nil {
			continue
		}
		if changedFiles, ok := t.Metadata["changed_files"]; ok {
			for f := range strings.SplitSeq(changedFiles, ",") {
				f = strings.TrimSpace(f)
				if f != "" && !seen[f] {
					seen[f] = true
					files = append(files, f)
				}
			}
		}
	}

	return strings.Join(files, "\n")
}

// convertToDefinitions converts database workflow variables to variable definitions.
func (we *WorkflowExecutor) convertToDefinitions(wvs []*db.WorkflowVariable) []variable.Definition {
	defs := make([]variable.Definition, len(wvs))
	for i, wv := range wvs {
		defs[i] = variable.Definition{
			Name:         wv.Name,
			Description:  wv.Description,
			SourceType:   variable.SourceType(wv.SourceType),
			SourceConfig: json.RawMessage(wv.SourceConfig),
			Extract:      wv.Extract,
			Required:     wv.Required,
			DefaultValue: wv.DefaultValue,
			CacheTTL:     time.Duration(wv.CacheTTLSeconds) * time.Second,
		}
	}
	return defs
}

// loadPriorPhaseContentProto loads content from a completed prior phase using proto types.
func (we *WorkflowExecutor) loadPriorPhaseContentProto(taskID string, e *orcv1.ExecutionState, phaseID string) string {
	// Check if phase is completed
	if e != nil && e.Phases != nil {
		ps, ok := e.Phases[phaseID]
		if ok && ps.Status != orcv1.PhaseStatus_PHASE_STATUS_COMPLETED {
			return ""
		}
	}

	// Load from database - phase outputs are stored there, not as files
	outputs, err := we.backend.GetPhaseOutputsForTask(taskID)
	if err != nil {
		we.logger.Debug("failed to load phase outputs for task",
			"task_id", taskID,
			"phase_id", phaseID,
			"error", err,
		)
		return ""
	}

	// Find the output for this phase
	for _, output := range outputs {
		if output.PhaseTemplateID == phaseID {
			return strings.TrimSpace(output.Content)
		}
	}

	return ""
}

// loadAutomationContextProto loads automation task context using proto types.
func (we *WorkflowExecutor) loadAutomationContextProto(rctx *variable.ResolutionContext, t *orcv1.Task) {
	// Load recent completed tasks
	tasks, err := we.backend.LoadAllTasks()
	if err == nil {
		rctx.RecentCompletedTasks = formatRecentCompletedTasksForPrompt(tasks, 20)
		rctx.RecentChangedFiles = collectRecentChangedFilesForPrompt(tasks, 10)
	}

	// Load CHANGELOG.md content
	changelogPath := filepath.Join(we.workingDir, "CHANGELOG.md")
	if content, err := os.ReadFile(changelogPath); err == nil {
		rctx.ChangelogContent = string(content)
	}

	// Load CLAUDE.md content
	claudeMDPath := filepath.Join(we.workingDir, "CLAUDE.md")
	if content, err := os.ReadFile(claudeMDPath); err == nil {
		rctx.ClaudeMDContent = string(content)
	}
}
