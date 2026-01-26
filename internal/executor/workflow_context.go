// workflow_context.go contains context building and variable resolution for workflow execution.
// This includes building resolution context, loading initiative data, project detection,
// and enriching context with phase-specific information.
package executor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/variable"
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
		WorkflowID:    wf.ID,
		WorkflowRunID: run.ID,
		Prompt:        opts.Prompt,
		Instructions:  opts.Instructions,
		WorkingDir:    workDir,
		ProjectRoot:   workDir,
		PriorOutputs:  make(map[string]string),
	}

	if t != nil {
		rctx.TaskID = t.Id
		rctx.TaskTitle = t.Title
		rctx.TaskDescription = task.GetDescriptionProto(t)
		rctx.TaskCategory = t.Category.String()
		rctx.TaskWeight = t.Weight.String()
		rctx.TaskBranch = t.Branch
		rctx.RequiresUITesting = t.RequiresUiTesting

		// Resolve target branch
		rctx.TargetBranch = ResolveTargetBranchForTask(t, we.backend, we.orcConfig)

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
			if maxIter, ok := t.Metadata["qa_max_iterations"]; ok {
				if n, err := strconv.Atoi(maxIter); err == nil && n > 0 {
					rctx.QAMaxIterations = n
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

	we.logger.Debug("initiative context loaded",
		"initiative_id", init.ID,
		"has_vision", init.Vision != "",
		"decision_count", len(init.Decisions),
	)
}

// loadProjectDetectionContext loads project detection data into the resolution context.
func (we *WorkflowExecutor) loadProjectDetectionContext(rctx *variable.ResolutionContext) {
	dbBackend, ok := we.backend.(*storage.DatabaseBackend)
	if !ok {
		return
	}

	detection, err := dbBackend.DB().LoadDetection()
	if err != nil || detection == nil {
		return
	}

	rctx.Language = detection.Language
	rctx.HasTests = detection.HasTests
	rctx.TestCommand = detection.TestCommand
	rctx.LintCommand = detection.LintCommand
	rctx.Frameworks = detection.Frameworks

	// Determine HasFrontend from frameworks
	for _, f := range detection.Frameworks {
		switch f {
		case "react", "vue", "angular", "svelte", "nextjs", "nuxt", "gatsby", "astro":
			rctx.HasFrontend = true
		}
	}
}

// enrichContextForPhase adds phase-specific context to the resolution context.
// Call this before executing each phase to load review findings, artifacts, etc.
// Note: Uses Task-centric approach where execution state is in task.Task.Execution.
func (we *WorkflowExecutor) enrichContextForPhase(rctx *variable.ResolutionContext, phaseID string, t *orcv1.Task) {
	if t == nil {
		return
	}

	// Load retry context from task's execution state
	rctx.RetryContext = LoadRetryContextFromExecutionProto(t.Execution)

	// Load review context for review phases
	if phaseID == "review" {
		we.loadReviewContextProto(rctx, t.Id, t.Execution)
	}

	// Load test results for review phase
	if phaseID == "review" {
		rctx.TestResults = we.loadPriorPhaseContentProto(t.Id, t.Execution, "test")
	}

	// Load TDD test plan if it exists
	rctx.TDDTestPlan = we.loadPriorPhaseContentProto(t.Id, t.Execution, "tdd_write_plan")

	// Load automation context for automation tasks
	if t.IsAutomation {
		we.loadAutomationContextProto(rctx, t)
	}
}

// formatReviewFindingsForPrompt formats review findings for template injection.
func formatReviewFindingsForPrompt(findings *orcv1.ReviewRoundFindings) string {
	if findings == nil {
		return "No findings from previous round."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "## Round %d Summary\n\n", findings.Round)
	sb.WriteString(findings.Summary)
	sb.WriteString("\n\n")

	// Count issues by severity
	highCount, mediumCount, lowCount := 0, 0, 0
	for _, issue := range findings.Issues {
		switch issue.Severity {
		case "high":
			highCount++
		case "medium":
			mediumCount++
		case "low":
			lowCount++
		}
	}

	fmt.Fprintf(&sb, "**Issues Found:** %d high, %d medium, %d low\n\n", highCount, mediumCount, lowCount)

	if len(findings.Issues) > 0 {
		sb.WriteString("### Issues to Verify\n\n")
		for i, issue := range findings.Issues {
			fmt.Fprintf(&sb, "%d. [%s] %s", i+1, strings.ToUpper(issue.Severity), issue.Description)
			if issue.File != nil && *issue.File != "" {
				fmt.Fprintf(&sb, " (in %s", *issue.File)
				if issue.Line != nil && *issue.Line > 0 {
					fmt.Fprintf(&sb, ":%d", *issue.Line)
				}
				sb.WriteString(")")
			}
			sb.WriteString("\n")
			if issue.Suggestion != nil && *issue.Suggestion != "" {
				fmt.Fprintf(&sb, "   Suggested fix: %s\n", *issue.Suggestion)
			}
		}
	}

	if len(findings.Positives) > 0 {
		sb.WriteString("\n### Positive Notes\n\n")
		for _, p := range findings.Positives {
			fmt.Fprintf(&sb, "- %s\n", p)
		}
	}

	if len(findings.Questions) > 0 {
		sb.WriteString("\n### Questions from Review\n\n")
		for _, q := range findings.Questions {
			fmt.Fprintf(&sb, "- %s\n", q)
		}
	}

	return sb.String()
}

// formatRecentCompletedTasksForPrompt formats recent completed tasks as a markdown list.
func formatRecentCompletedTasksForPrompt(tasks []*orcv1.Task, limit int) string {
	var completed []*orcv1.Task
	for _, t := range tasks {
		if t.Status == orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			completed = append(completed, t)
		}
	}

	// Sort by completion time (most recent first) - already done by LoadAllTasksProto
	if len(completed) > limit {
		completed = completed[:limit]
	}

	var sb strings.Builder
	for _, t := range completed {
		fmt.Fprintf(&sb, "- **%s**: %s", t.Id, t.Title)
		if t.Category != orcv1.TaskCategory_TASK_CATEGORY_UNSPECIFIED {
			fmt.Fprintf(&sb, " [%s]", task.CategoryFromProto(t.Category))
		}
		if t.Weight != orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED {
			fmt.Fprintf(&sb, " (%s)", task.WeightFromProto(t.Weight))
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
			Required:     wv.Required,
			DefaultValue: wv.DefaultValue,
			CacheTTL:     time.Duration(wv.CacheTTLSeconds) * time.Second,
		}
	}
	return defs
}

// loadReviewContextProto loads review-specific context into the resolution context.
func (we *WorkflowExecutor) loadReviewContextProto(rctx *variable.ResolutionContext, taskID string, e *orcv1.ExecutionState) {
	// Determine review round from state
	round := 1
	if e != nil && e.Phases != nil {
		if ps, ok := e.Phases["review"]; ok && ps.Status == orcv1.PhaseStatus_PHASE_STATUS_COMPLETED {
			round = 2
		}
	}
	rctx.ReviewRound = round

	// Load previous round's findings for round 2+
	if round > 1 {
		findings, err := we.backend.LoadReviewFindings(taskID, round-1)
		if err != nil {
			we.logger.Debug("failed to load review findings",
				"task_id", taskID,
				"round", round-1,
				"error", err,
			)
			return
		}
		if findings != nil {
			rctx.ReviewFindings = formatReviewFindingsForPrompt(findings)
		}
	}
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
	tasks, err := we.backend.LoadAllTasksProto()
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
