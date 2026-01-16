// Package executor provides task phase execution for orc.
package executor

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/templates"
)

// truncateForLog truncates a string for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// WorktreeContext holds worktree-specific context for template rendering.
// This provides safety context to Claude about where it's working and what
// branches are protected.
type WorktreeContext struct {
	// WorktreePath is the absolute path to the worktree directory.
	WorktreePath string

	// TaskBranch is the git branch for this task (e.g., orc/TASK-001).
	TaskBranch string

	// TargetBranch is the target branch for merging (e.g., main).
	TargetBranch string
}

// TemplateVars holds all variables for template rendering.
type TemplateVars struct {
	TaskID           string
	TaskTitle        string
	TaskDescription  string
	TaskCategory     string // Task category (bug, feature, refactor, etc.)
	Phase            string
	Weight           string
	Iteration        int
	RetryContext     string
	ResearchContent  string
	SpecContent      string
	DesignContent    string
	ImplementContent string

	// Verification results from implement phase (extracted from artifact)
	VerificationResults string

	// Worktree context variables (for safety instructions)
	WorktreePath string // Absolute path to the worktree directory
	TaskBranch   string // The git branch for this task (e.g., orc/TASK-001)
	TargetBranch string // The target branch for merging (e.g., main)

	// UI Testing context variables
	RequiresUITesting bool   // Whether the task requires UI testing
	ScreenshotDir     string // Directory for saving screenshots (task attachments)
	TestResults       string // Test results from previous test phase (for validate)

	// Testing configuration
	CoverageThreshold int // Minimum test coverage percentage required (default: 85)

	// Initiative context variables (inherited from parent initiative)
	InitiativeID        string // Initiative ID (e.g., INIT-001)
	InitiativeTitle     string // Initiative title
	InitiativeVision    string // Initiative vision/goals
	InitiativeDecisions string // Formatted initiative decisions

	// Automation task context variables
	RecentCompletedTasks string // Formatted list of recently completed tasks
	RecentChangedFiles   string // List of files changed in recent tasks
	ChangelogContent     string // Current CHANGELOG.md content
	ClaudeMDContent      string // Current CLAUDE.md content
}

// UITestingContext holds UI testing-specific context for template rendering.
type UITestingContext struct {
	// RequiresUITesting indicates if the task needs UI testing.
	RequiresUITesting bool

	// ScreenshotDir is the absolute path where screenshots should be saved.
	ScreenshotDir string

	// TestResults contains the output from the test phase.
	TestResults string
}

// InitiativeDecision represents a decision from an initiative for context injection.
type InitiativeDecision struct {
	ID        string // Decision ID (e.g., DEC-001)
	Decision  string // The decision text
	Rationale string // Why this decision was made
}

// InitiativeContext holds initiative-specific context for template rendering.
// This provides shared vision and decisions to tasks within an initiative.
type InitiativeContext struct {
	// ID is the initiative identifier (e.g., INIT-001).
	ID string

	// Title is the initiative title.
	Title string

	// Vision is the strategic vision/goals for the initiative.
	Vision string

	// Decisions is a list of decisions made within the initiative.
	Decisions []InitiativeDecision
}

// AutomationContext holds automation task-specific context for template rendering.
// This provides information about recent tasks, changed files, and project state
// for automation templates like changelog generation, style normalization, etc.
type AutomationContext struct {
	// RecentCompletedTasks is a formatted list of recently completed tasks.
	// Used by templates like changelog-generation.md and knowledge-sync.md.
	RecentCompletedTasks string

	// RecentChangedFiles is a list of files changed in recent tasks.
	// Used by templates like style-normalization.md.
	RecentChangedFiles string

	// ChangelogContent is the current CHANGELOG.md content.
	// Used by changelog-generation.md template.
	ChangelogContent string

	// ClaudeMDContent is the current CLAUDE.md content.
	// Used by knowledge-sync.md template.
	ClaudeMDContent string
}

// FormatDecisions formats the decisions as a markdown string for template injection.
func (ctx InitiativeContext) FormatDecisions() string {
	if len(ctx.Decisions) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, d := range ctx.Decisions {
		sb.WriteString(fmt.Sprintf("- **%s**: %s", d.ID, d.Decision))
		if d.Rationale != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", d.Rationale))
		}
		sb.WriteString("\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// formatInitiativeContextSection builds a complete initiative context section for templates.
// Returns empty string if no initiative context is set, otherwise returns a formatted
// markdown section with vision and decisions.
func formatInitiativeContextSection(vars TemplateVars) string {
	if vars.InitiativeID == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Initiative Context\n\n")
	sb.WriteString(fmt.Sprintf("This task is part of **%s** (%s).\n", vars.InitiativeTitle, vars.InitiativeID))

	if vars.InitiativeVision != "" {
		sb.WriteString("\n### Vision\n\n")
		sb.WriteString(vars.InitiativeVision)
		sb.WriteString("\n")
	}

	if vars.InitiativeDecisions != "" {
		sb.WriteString("\n### Decisions\n\n")
		sb.WriteString("The following decisions have been made for this initiative:\n\n")
		sb.WriteString(vars.InitiativeDecisions)
		sb.WriteString("\n")
	}

	sb.WriteString("\n**Alignment**: Ensure your work aligns with the initiative vision and respects prior decisions.\n")

	return sb.String()
}

// RenderTemplate performs variable substitution on a template string.
// Variables use the {{VAR}} format. Missing variables are replaced with
// empty strings.
func RenderTemplate(tmpl string, vars TemplateVars) string {
	// For tasks without a spec phase, use task description as spec content
	specContent := vars.SpecContent
	if specContent == "" && vars.TaskDescription != "" {
		specContent = vars.TaskDescription
	}

	// Format UI testing flag as string
	requiresUITesting := ""
	if vars.RequiresUITesting {
		requiresUITesting = "true"
	}

	replacements := map[string]string{
		"{{TASK_ID}}":                vars.TaskID,
		"{{TASK_TITLE}}":             vars.TaskTitle,
		"{{TASK_DESCRIPTION}}":       vars.TaskDescription,
		"{{TASK_CATEGORY}}":          vars.TaskCategory,
		"{{PHASE}}":                  vars.Phase,
		"{{WEIGHT}}":                 vars.Weight,
		"{{ITERATION}}":              fmt.Sprintf("%d", vars.Iteration),
		"{{RETRY_CONTEXT}}":          vars.RetryContext,
		"{{RESEARCH_CONTENT}}":       vars.ResearchContent,
		"{{SPEC_CONTENT}}":           specContent,
		"{{DESIGN_CONTENT}}":         vars.DesignContent,
		"{{IMPLEMENT_CONTENT}}":      vars.ImplementContent,
		"{{IMPLEMENTATION_SUMMARY}}": vars.ImplementContent, // Alias for template compatibility
		"{{VERIFICATION_RESULTS}}":   vars.VerificationResults,

		// Worktree context variables
		"{{WORKTREE_PATH}}": vars.WorktreePath,
		"{{TASK_BRANCH}}":   vars.TaskBranch,
		"{{TARGET_BRANCH}}": vars.TargetBranch,

		// UI Testing context variables
		"{{REQUIRES_UI_TESTING}}": requiresUITesting,
		"{{SCREENSHOT_DIR}}":      vars.ScreenshotDir,
		"{{TEST_RESULTS}}":        vars.TestResults,

		// Testing configuration
		"{{COVERAGE_THRESHOLD}}": fmt.Sprintf("%d", vars.CoverageThreshold),

		// Initiative context variables
		"{{INITIATIVE_ID}}":        vars.InitiativeID,
		"{{INITIATIVE_TITLE}}":     vars.InitiativeTitle,
		"{{INITIATIVE_VISION}}":    vars.InitiativeVision,
		"{{INITIATIVE_DECISIONS}}": vars.InitiativeDecisions,
		"{{INITIATIVE_CONTEXT}}":   formatInitiativeContextSection(vars),

		// Automation task context variables
		"{{RECENT_COMPLETED_TASKS}}": vars.RecentCompletedTasks,
		"{{RECENT_CHANGED_FILES}}":   vars.RecentChangedFiles,
		"{{CHANGELOG_CONTENT}}":      vars.ChangelogContent,
		"{{CLAUDEMD_CONTENT}}":       vars.ClaudeMDContent,
	}

	result := tmpl
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}
	return result
}

// LoadPromptTemplate loads a prompt template for a phase.
// If the phase has an inline prompt, it returns that.
// Otherwise, it loads from the embedded templates.
func LoadPromptTemplate(phase *plan.Phase) (string, error) {
	if phase == nil {
		return "", fmt.Errorf("phase is nil")
	}

	// Inline prompt takes precedence
	if phase.Prompt != "" {
		return phase.Prompt, nil
	}

	// Load from embedded templates
	tmplPath := fmt.Sprintf("prompts/%s.md", phase.ID)
	content, err := templates.Prompts.ReadFile(tmplPath)
	if err != nil {
		return "", fmt.Errorf("prompt not found for phase %s", phase.ID)
	}

	return string(content), nil
}

// BuildTemplateVars creates template variables from task context.
// If state is nil, prior content fields will be empty.
func BuildTemplateVars(
	t *task.Task,
	p *plan.Phase,
	s *state.State,
	iteration int,
	retryContext string,
) TemplateVars {
	// Debug: log task fields to trace description injection
	if t != nil {
		slog.Debug("BuildTemplateVars called",
			"task_id", t.ID,
			"title", t.Title,
			"description_len", len(t.Description),
			"description_preview", truncateForLog(t.Description, 100),
		)
	}

	vars := TemplateVars{
		TaskID:          t.ID,
		TaskTitle:       t.Title,
		TaskDescription: t.Description,
		TaskCategory:    string(t.Category),
		Phase:           p.ID,
		Weight:          string(t.Weight),
		Iteration:       iteration,
		RetryContext:    retryContext,
	}

	// Populate prior phase content from artifacts and transcripts
	taskDir := task.TaskDir(t.ID)
	vars.ResearchContent = loadPriorContent(taskDir, s, "research")
	vars.SpecContent = loadPriorContent(taskDir, s, "spec")
	vars.DesignContent = loadPriorContent(taskDir, s, "design")
	vars.ImplementContent = loadPriorContent(taskDir, s, "implement")

	// Extract verification results from implement content
	if vars.ImplementContent != "" {
		vars.VerificationResults = extractVerificationResults(vars.ImplementContent)
	}

	return vars
}

// BuildTemplateVarsWithWorktree creates template variables with worktree context.
// This is the preferred function when executing phases in a worktree.
func BuildTemplateVarsWithWorktree(
	t *task.Task,
	p *plan.Phase,
	s *state.State,
	iteration int,
	retryContext string,
	wctx WorktreeContext,
) TemplateVars {
	vars := BuildTemplateVars(t, p, s, iteration, retryContext)
	vars.WorktreePath = wctx.WorktreePath
	vars.TaskBranch = wctx.TaskBranch
	vars.TargetBranch = wctx.TargetBranch
	return vars
}

// WithWorktreeContext returns a copy of the vars with worktree context applied.
func (v TemplateVars) WithWorktreeContext(wctx WorktreeContext) TemplateVars {
	v.WorktreePath = wctx.WorktreePath
	v.TaskBranch = wctx.TaskBranch
	v.TargetBranch = wctx.TargetBranch
	return v
}

// WithUITestingContext returns a copy of the vars with UI testing context applied.
func (v TemplateVars) WithUITestingContext(ctx UITestingContext) TemplateVars {
	v.RequiresUITesting = ctx.RequiresUITesting
	v.ScreenshotDir = ctx.ScreenshotDir
	v.TestResults = ctx.TestResults
	return v
}

// WithInitiativeContext returns a copy of the vars with initiative context applied.
// This injects initiative vision and decisions into the task prompt.
func (v TemplateVars) WithInitiativeContext(ctx InitiativeContext) TemplateVars {
	v.InitiativeID = ctx.ID
	v.InitiativeTitle = ctx.Title
	v.InitiativeVision = ctx.Vision
	v.InitiativeDecisions = ctx.FormatDecisions()
	return v
}

// WithAutomationContext returns a copy of the vars with automation context applied.
// This injects recent tasks, changed files, and project file content for automation templates.
func (v TemplateVars) WithAutomationContext(ctx AutomationContext) TemplateVars {
	v.RecentCompletedTasks = ctx.RecentCompletedTasks
	v.RecentChangedFiles = ctx.RecentChangedFiles
	v.ChangelogContent = ctx.ChangelogContent
	v.ClaudeMDContent = ctx.ClaudeMDContent
	return v
}

// LoadInitiativeContext loads initiative context for a task if it belongs to an initiative.
// Returns nil if the task doesn't belong to an initiative or if the initiative can't be loaded.
func LoadInitiativeContext(t *task.Task, backend storage.Backend) *InitiativeContext {
	if t == nil || t.InitiativeID == "" || backend == nil {
		return nil
	}

	init, err := backend.LoadInitiative(t.InitiativeID)
	if err != nil {
		slog.Debug("failed to load initiative for task",
			"task_id", t.ID,
			"initiative_id", t.InitiativeID,
			"error", err,
		)
		return nil
	}

	// Convert initiative decisions to context decisions
	decisions := make([]InitiativeDecision, len(init.Decisions))
	for i, d := range init.Decisions {
		decisions[i] = InitiativeDecision{
			ID:        d.ID,
			Decision:  d.Decision,
			Rationale: d.Rationale,
		}
	}

	return &InitiativeContext{
		ID:        init.ID,
		Title:     init.Title,
		Vision:    init.Vision,
		Decisions: decisions,
	}
}

// loadPriorContent loads content from a completed prior phase.
// It reads from artifact files or falls back to extracting from transcripts.
func loadPriorContent(taskDir string, s *state.State, phaseID string) string {
	// Check if phase is completed (only load content from completed phases)
	if s != nil && s.Phases != nil {
		ps, ok := s.Phases[phaseID]
		if ok && ps.Status != state.StatusCompleted {
			return ""
		}
	}

	// Try artifact file first: {taskDir}/artifacts/{phase}.md
	artifactPath := filepath.Join(taskDir, "artifacts", phaseID+".md")
	if content, err := os.ReadFile(artifactPath); err == nil {
		return strings.TrimSpace(string(content))
	}

	// Fall back to extracting from transcripts
	return loadFromTranscript(taskDir, phaseID)
}

// loadFromTranscript reads the latest transcript for a phase and extracts artifacts.
func loadFromTranscript(taskDir string, phaseID string) string {
	transcriptsDir := filepath.Join(taskDir, "transcripts")

	// Find transcript files for this phase: {phase}-{iteration}.md
	entries, err := os.ReadDir(transcriptsDir)
	if err != nil {
		return ""
	}

	// Pattern: {phase}-{number}.md
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(phaseID) + `-(\d+)\.md$`)

	var transcriptFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if pattern.MatchString(entry.Name()) {
			transcriptFiles = append(transcriptFiles, entry.Name())
		}
	}

	if len(transcriptFiles) == 0 {
		return ""
	}

	// Sort to get the latest iteration (highest number)
	sort.Strings(transcriptFiles)
	latestFile := transcriptFiles[len(transcriptFiles)-1]

	content, err := os.ReadFile(filepath.Join(transcriptsDir, latestFile))
	if err != nil {
		return ""
	}

	return extractArtifact(string(content))
}

// extractArtifact extracts content between <artifact>...</artifact> tags.
// If no artifact tags are found, returns the entire content (trimmed).
func extractArtifact(content string) string {
	// Try to extract content between <artifact> tags
	artifactPattern := regexp.MustCompile(`(?s)<artifact>(.*?)</artifact>`)
	matches := artifactPattern.FindStringSubmatch(content)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}

	// If no artifact tags, look for structured output markers
	// e.g., spec_complete:, ## Specification, etc.
	structuredPatterns := []string{
		`(?s)## Specification\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Research Results\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Design\s*\n(.*?)(?:\n##|$)`,
		`(?s)## Implementation Summary\s*\n(.*?)(?:\n##|$)`,
	}

	for _, p := range structuredPatterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(content); len(m) >= 2 {
			return strings.TrimSpace(m[1])
		}
	}

	// If no structured content found, return empty
	// We don't want to return raw transcript content as it's too noisy
	return ""
}

// extractVerificationResults extracts the verification results table from
// implement phase output. This table contains the pass/fail status of each
// success criterion.
func extractVerificationResults(content string) string {
	// Look for "### Verification Results" section with a table
	// Pattern: ### Verification Results followed by table until next ### or end
	verificationPattern := regexp.MustCompile(
		`(?s)###\s*Verification Results\s*\n+(.*?)(?:\n###|\n##|$)`,
	)
	matches := verificationPattern.FindStringSubmatch(content)
	if len(matches) >= 2 {
		result := strings.TrimSpace(matches[1])
		// Only return if it looks like a table (contains | characters)
		if strings.Contains(result, "|") {
			return result
		}
	}

	// Try alternate format without ### prefix
	altPattern := regexp.MustCompile(
		`(?s)##\s*Verification Results\s*\n+(.*?)(?:\n##|$)`,
	)
	matches = altPattern.FindStringSubmatch(content)
	if len(matches) >= 2 {
		result := strings.TrimSpace(matches[1])
		if strings.Contains(result, "|") {
			return result
		}
	}

	return ""
}

// LoadAutomationContext loads automation context for an automation task.
// This populates recent completed tasks, changed files, and project file content
// for automation templates like changelog generation and style normalization.
func LoadAutomationContext(t *task.Task, backend storage.Backend, projectRoot string) *AutomationContext {
	if t == nil || !t.IsAutomation || backend == nil {
		return nil
	}

	ctx := &AutomationContext{}

	// Load recent completed tasks (last 20)
	tasks, err := backend.LoadAllTasks()
	if err == nil {
		ctx.RecentCompletedTasks = formatRecentCompletedTasks(tasks, 20)
		ctx.RecentChangedFiles = collectRecentChangedFiles(tasks, 10)
	}

	// Load CHANGELOG.md content
	changelogPath := filepath.Join(projectRoot, "CHANGELOG.md")
	if content, err := os.ReadFile(changelogPath); err == nil {
		ctx.ChangelogContent = string(content)
	}

	// Load CLAUDE.md content
	claudeMDPath := filepath.Join(projectRoot, "CLAUDE.md")
	if content, err := os.ReadFile(claudeMDPath); err == nil {
		ctx.ClaudeMDContent = string(content)
	}

	return ctx
}

// formatRecentCompletedTasks formats recent completed tasks as a markdown list.
func formatRecentCompletedTasks(tasks []*task.Task, limit int) string {
	var completed []*task.Task
	for _, t := range tasks {
		if t.Status == task.StatusCompleted || t.Status == task.StatusFinished {
			completed = append(completed, t)
		}
	}

	// Sort by completion time (most recent first)
	sort.Slice(completed, func(i, j int) bool {
		if completed[i].CompletedAt == nil {
			return false
		}
		if completed[j].CompletedAt == nil {
			return true
		}
		return completed[i].CompletedAt.After(*completed[j].CompletedAt)
	})

	// Limit to requested number
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

// collectRecentChangedFiles collects files changed in recent tasks.
// This extracts file paths from task descriptions and metadata.
func collectRecentChangedFiles(tasks []*task.Task, limit int) string {
	// Get recent completed tasks
	var recent []*task.Task
	for _, t := range tasks {
		if t.Status == task.StatusCompleted || t.Status == task.StatusFinished {
			recent = append(recent, t)
		}
	}

	// Sort by completion time (most recent first)
	sort.Slice(recent, func(i, j int) bool {
		if recent[i].CompletedAt == nil {
			return false
		}
		if recent[j].CompletedAt == nil {
			return true
		}
		return recent[i].CompletedAt.After(*recent[j].CompletedAt)
	})

	// Limit tasks to check
	if len(recent) > limit {
		recent = recent[:limit]
	}

	// Collect unique file paths from task metadata
	seen := make(map[string]bool)
	var files []string

	for _, t := range recent {
		// Check metadata for changed_files key
		// Skip tasks with nil metadata to avoid nil pointer dereference
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
