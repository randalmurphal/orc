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

// loadAndFormatInitiativeNotes loads initiative notes and returns a formatted markdown string
// grouped by note type (patterns, warnings, learnings, handoffs). Returns an empty string if
// no applicable notes exist or if loading fails.
//
// Filtering: Human notes are always included. Agent notes are only included when graduated
// (i.e., they met the strict quality bar for injection into future task prompts).
// Per DEC-003 and DEC-004 in the knowledge sharing initiative design.
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
func (we *WorkflowExecutor) enrichContextForPhase(rctx *variable.ResolutionContext, phaseID string, t *orcv1.Task) {
	if t == nil {
		return
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

	// Load automation context for automation tasks
	if t.IsAutomation {
		we.loadAutomationContextProto(rctx, t)
	}
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
