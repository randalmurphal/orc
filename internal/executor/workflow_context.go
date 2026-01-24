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

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/state"
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

// createTaskForRun creates a task for a default context run.
func (we *WorkflowExecutor) createTaskForRun(opts WorkflowRunOptions) (*task.Task, error) {
	taskID, err := we.backend.GetNextTaskID()
	if err != nil {
		return nil, fmt.Errorf("get next task ID: %w", err)
	}

	t := &task.Task{
		ID:          taskID,
		Title:       truncateTitle(opts.Prompt),
		Description: opts.Prompt,
		Category:    opts.Category,
		Status:      task.StatusCreated,
		Queue:       task.QueueActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if t.Category == "" {
		t.Category = task.CategoryFeature
	}

	if err := we.backend.SaveTask(t); err != nil {
		return nil, fmt.Errorf("save task: %w", err)
	}

	return t, nil
}

// buildResolutionContext creates the variable resolution context.
func (we *WorkflowExecutor) buildResolutionContext(
	opts WorkflowRunOptions,
	t *task.Task,
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
		rctx.TaskID = t.ID
		rctx.TaskTitle = t.Title
		rctx.TaskDescription = t.Description
		rctx.TaskCategory = string(t.Category)
		rctx.TaskWeight = string(t.Weight)
		rctx.TaskBranch = t.Branch
		rctx.RequiresUITesting = t.RequiresUITesting

		// Resolve target branch
		rctx.TargetBranch = ResolveTargetBranchForTask(t, we.backend, we.orcConfig)

		// Load initiative context if task belongs to an initiative
		if t.InitiativeID != "" {
			we.loadInitiativeContext(rctx, t.InitiativeID)
		}

		// Set up screenshot dir for UI testing tasks
		if t.RequiresUITesting && we.workingDir != "" {
			rctx.ScreenshotDir = task.ScreenshotsPath(we.workingDir, t.ID)
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
func (we *WorkflowExecutor) enrichContextForPhase(rctx *variable.ResolutionContext, phaseID string, t *task.Task, s *state.State) {
	// Load retry context from state
	if s != nil {
		rctx.RetryContext = LoadRetryContextForPhase(s)
	}

	// Load review context for review phases
	if phaseID == "review" && t != nil {
		we.loadReviewContext(rctx, t.ID, s)
	}

	// Load test results for review phase
	if phaseID == "review" && t != nil {
		rctx.TestResults = we.loadPriorPhaseContent(t.ID, s, "test")
	}

	// Load TDD test plan if it exists
	if t != nil {
		rctx.TDDTestPlan = we.loadPriorPhaseContent(t.ID, s, "tdd_write_plan")
	}

	// Load automation context for automation tasks
	if t != nil && t.IsAutomation {
		we.loadAutomationContext(rctx, t)
	}
}

// loadReviewContext loads review-specific context into the resolution context.
func (we *WorkflowExecutor) loadReviewContext(rctx *variable.ResolutionContext, taskID string, s *state.State) {
	// Determine review round from state
	round := 1
	if s != nil && s.Phases != nil {
		if ps, ok := s.Phases["review"]; ok && ps.Status == state.StatusCompleted {
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

// loadAutomationContext loads automation task context.
func (we *WorkflowExecutor) loadAutomationContext(rctx *variable.ResolutionContext, t *task.Task) {
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

// loadPriorPhaseContent loads content from a completed prior phase.
func (we *WorkflowExecutor) loadPriorPhaseContent(taskID string, s *state.State, phaseID string) string {
	// Check if phase is completed
	if s != nil && s.Phases != nil {
		ps, ok := s.Phases[phaseID]
		if ok && ps.Status != state.StatusCompleted {
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

// formatReviewFindingsForPrompt formats review findings for template injection.
func formatReviewFindingsForPrompt(findings *storage.ReviewFindings) string {
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
			if issue.File != "" {
				fmt.Fprintf(&sb, " (in %s", issue.File)
				if issue.Line > 0 {
					fmt.Fprintf(&sb, ":%d", issue.Line)
				}
				sb.WriteString(")")
			}
			sb.WriteString("\n")
			if issue.Suggestion != "" {
				fmt.Fprintf(&sb, "   Suggested fix: %s\n", issue.Suggestion)
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
func formatRecentCompletedTasksForPrompt(tasks []*task.Task, limit int) string {
	var completed []*task.Task
	for _, t := range tasks {
		if t.Status == task.StatusCompleted {
			completed = append(completed, t)
		}
	}

	// Sort by completion time (most recent first) - already done by LoadAllTasks
	if len(completed) > limit {
		completed = completed[:limit]
	}

	var sb strings.Builder
	for _, t := range completed {
		fmt.Fprintf(&sb, "- **%s**: %s", t.ID, t.Title)
		if t.Category != "" {
			fmt.Fprintf(&sb, " [%s]", t.Category)
		}
		if t.Weight != "" {
			fmt.Fprintf(&sb, " (%s)", t.Weight)
		}
		sb.WriteString("\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// collectRecentChangedFilesForPrompt collects files changed in recent tasks.
func collectRecentChangedFilesForPrompt(tasks []*task.Task, limit int) string {
	var recent []*task.Task
	for _, t := range tasks {
		if t.Status == task.StatusCompleted {
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
