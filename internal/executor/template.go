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
	ImplementContent string

	// Verification results from implement phase (extracted from artifact)
	VerificationResults string

	// Review phase context variables
	ReviewRound    int    // Current review round (1 or 2)
	ReviewFindings string // Previous round's findings (for Round 2)

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

	// Project detection variables (from detect.Detect())
	Language     string   // Primary language (go, typescript, python, etc.)
	HasFrontend  bool     // Whether project has a frontend
	HasTests     bool     // Whether project has existing tests
	TestCommand  string   // Command to run tests (e.g., "go test ./...")
	LintCommand  string   // Command to run linting
	BuildCommand string   // Command to build project
	Frameworks   []string // Detected frameworks

	// Constitution content (project principles)
	ConstitutionContent string

	// TDD phase artifacts
	TDDTestsContent  string // Content from tdd_write phase (tests written)
	TDDTestPlan      string // Manual UI test plan for Playwright MCP
	BreakdownContent string // Breakdown content from breakdown phase
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

	// Format booleans as strings
	hasFrontend := ""
	if vars.HasFrontend {
		hasFrontend = "true"
	}
	hasTests := ""
	if vars.HasTests {
		hasTests = "true"
	}

	// Format frameworks as comma-separated string
	frameworks := strings.Join(vars.Frameworks, ", ")

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

		// Review phase context variables
		"{{REVIEW_ROUND}}":    fmt.Sprintf("%d", vars.ReviewRound),
		"{{REVIEW_FINDINGS}}": vars.ReviewFindings,

		// Project detection variables
		"{{LANGUAGE}}":      vars.Language,
		"{{HAS_FRONTEND}}":  hasFrontend,
		"{{HAS_TESTS}}":     hasTests,
		"{{TEST_COMMAND}}":  vars.TestCommand,
		"{{LINT_COMMAND}}":  vars.LintCommand,
		"{{BUILD_COMMAND}}": vars.BuildCommand,
		"{{FRAMEWORKS}}":    frameworks,

		// Constitution content
		"{{CONSTITUTION_CONTENT}}": vars.ConstitutionContent,

		// TDD phase artifacts
		"{{TDD_TESTS_CONTENT}}": vars.TDDTestsContent,
		"{{TDD_TEST_PLAN}}":     vars.TDDTestPlan,
		"{{BREAKDOWN_CONTENT}}": vars.BreakdownContent,
	}

	result := tmpl
	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	// Process conditional blocks for review rounds
	result = processReviewConditionals(result, vars.ReviewRound)

	// Process conditional blocks for frontend/non-frontend
	result = processFrontendConditionals(result, vars.HasFrontend)

	// Process conditional blocks for TDD test plan
	result = processTDDTestPlanConditional(result, vars.TDDTestPlan)

	// Process conditional blocks for breakdown content
	result = processBreakdownContentConditional(result, vars.BreakdownContent)

	// Process conditional blocks for constitution content
	result = processConstitutionConditional(result, vars.ConstitutionContent)

	return result
}

// processReviewConditionals handles {{#if REVIEW_ROUND_1}} and {{#if REVIEW_ROUND_2}} blocks.
// If the condition is true, the block content is kept; otherwise it's removed.
func processReviewConditionals(content string, reviewRound int) string {
	// Process REVIEW_ROUND_1 blocks
	round1Pattern := regexp.MustCompile(`(?s)\{\{#if REVIEW_ROUND_1\}\}(.*?)\{\{/if\}\}`)
	if reviewRound == 1 {
		// Keep the content inside the block
		content = round1Pattern.ReplaceAllString(content, "$1")
	} else {
		// Remove the entire block
		content = round1Pattern.ReplaceAllString(content, "")
	}

	// Process REVIEW_ROUND_2 blocks
	round2Pattern := regexp.MustCompile(`(?s)\{\{#if REVIEW_ROUND_2\}\}(.*?)\{\{/if\}\}`)
	if reviewRound == 2 {
		// Keep the content inside the block
		content = round2Pattern.ReplaceAllString(content, "$1")
	} else {
		// Remove the entire block
		content = round2Pattern.ReplaceAllString(content, "")
	}

	return content
}

// processFrontendConditionals handles {{#if HAS_FRONTEND}} and {{#if NOT_HAS_FRONTEND}} blocks.
// Used for UI-aware TDD test generation.
func processFrontendConditionals(content string, hasFrontend bool) string {
	// Process HAS_FRONTEND blocks
	frontendPattern := regexp.MustCompile(`(?s)\{\{#if HAS_FRONTEND\}\}(.*?)\{\{/if\}\}`)
	if hasFrontend {
		// Keep the content inside the block
		content = frontendPattern.ReplaceAllString(content, "$1")
	} else {
		// Remove the entire block
		content = frontendPattern.ReplaceAllString(content, "")
	}

	// Process NOT_HAS_FRONTEND blocks
	noFrontendPattern := regexp.MustCompile(`(?s)\{\{#if NOT_HAS_FRONTEND\}\}(.*?)\{\{/if\}\}`)
	if !hasFrontend {
		// Keep the content inside the block
		content = noFrontendPattern.ReplaceAllString(content, "$1")
	} else {
		// Remove the entire block
		content = noFrontendPattern.ReplaceAllString(content, "")
	}

	return content
}

// processTDDTestPlanConditional handles {{#if TDD_TEST_PLAN}} blocks.
// Used to include manual UI testing instructions when a test plan exists.
func processTDDTestPlanConditional(content string, testPlan string) string {
	pattern := regexp.MustCompile(`(?s)\{\{#if TDD_TEST_PLAN\}\}(.*?)\{\{/if\}\}`)
	if testPlan != "" {
		// Keep the content inside the block
		content = pattern.ReplaceAllString(content, "$1")
	} else {
		// Remove the entire block
		content = pattern.ReplaceAllString(content, "")
	}
	return content
}

// processBreakdownContentConditional handles {{#if BREAKDOWN_CONTENT}} blocks.
// Used to include breakdown-specific instructions when breakdown content exists.
func processBreakdownContentConditional(content string, breakdownContent string) string {
	pattern := regexp.MustCompile(`(?s)\{\{#if BREAKDOWN_CONTENT\}\}(.*?)\{\{/if\}\}`)
	if breakdownContent != "" {
		// Keep the content inside the block
		content = pattern.ReplaceAllString(content, "$1")
	} else {
		// Remove the entire block
		content = pattern.ReplaceAllString(content, "")
	}
	return content
}

// processConstitutionConditional handles {{#if CONSTITUTION_CONTENT}} blocks.
// Used to include constitution-specific instructions and checks when a constitution is configured.
func processConstitutionConditional(content string, constitutionContent string) string {
	pattern := regexp.MustCompile(`(?s)\{\{#if CONSTITUTION_CONTENT\}\}(.*?)\{\{/if\}\}`)
	if constitutionContent != "" {
		// Keep the content inside the block
		content = pattern.ReplaceAllString(content, "$1")
	} else {
		// Remove the entire block
		content = pattern.ReplaceAllString(content, "")
	}
	return content
}

// LoadPromptTemplate loads a prompt template for a phase.
// If the phase has an inline prompt, it returns that.
// Otherwise, it loads from the embedded templates.
func LoadPromptTemplate(phase *Phase) (string, error) {
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
	p *Phase,
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
	// Also check for tiny_spec (combined spec+TDD for trivial/small tasks)
	if vars.SpecContent == "" {
		vars.SpecContent = loadPriorContent(taskDir, s, "tiny_spec")
	}
	vars.ImplementContent = loadPriorContent(taskDir, s, "implement")

	// Load TDD phase content for implement phase
	vars.TDDTestsContent = loadPriorContent(taskDir, s, "tdd_write")
	// Also check tiny_spec for TDD content (combined spec+TDD)
	if vars.TDDTestsContent == "" {
		vars.TDDTestsContent = loadPriorContent(taskDir, s, "tiny_spec")
	}
	vars.BreakdownContent = loadPriorContent(taskDir, s, "breakdown")

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
	p *Phase,
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

// ProjectDetectionContext holds project detection results for template rendering.
type ProjectDetectionContext struct {
	Language     string
	HasFrontend  bool
	HasTests     bool
	TestCommand  string
	LintCommand  string
	BuildCommand string
	Frameworks   []string
}

// WithProjectDetection returns a copy of the vars with project detection context applied.
func (v TemplateVars) WithProjectDetection(ctx ProjectDetectionContext) TemplateVars {
	v.Language = ctx.Language
	v.HasFrontend = ctx.HasFrontend
	v.HasTests = ctx.HasTests
	v.TestCommand = ctx.TestCommand
	v.LintCommand = ctx.LintCommand
	v.BuildCommand = ctx.BuildCommand
	v.Frameworks = ctx.Frameworks
	return v
}

// WithProjectDetectionFromDatabase returns a copy of the vars with project detection loaded
// from the database. This loads the detection results stored during project initialization.
func (v TemplateVars) WithProjectDetectionFromDatabase(backend storage.Backend) TemplateVars {
	if backend == nil {
		return v
	}

	// Type assert to get access to the underlying database
	dbBackend, ok := backend.(*storage.DatabaseBackend)
	if !ok {
		slog.Debug("backend is not DatabaseBackend, skipping project detection")
		return v
	}

	detection, err := dbBackend.DB().LoadDetection()
	if err != nil {
		slog.Debug("failed to load project detection from database", "error", err)
		return v
	}
	if detection == nil {
		slog.Debug("no project detection found in database")
		return v
	}

	// Determine HasFrontend from frameworks (look for frontend-related frameworks)
	hasFrontend := false
	for _, f := range detection.Frameworks {
		switch f {
		case "react", "vue", "angular", "svelte", "nextjs", "nuxt", "gatsby", "astro":
			hasFrontend = true
		}
	}

	return v.WithProjectDetection(ProjectDetectionContext{
		Language:    detection.Language,
		HasFrontend: hasFrontend,
		HasTests:    detection.HasTests,
		TestCommand: detection.TestCommand,
		LintCommand: detection.LintCommand,
		Frameworks:  detection.Frameworks,
	})
}

// WithSpecFromDatabase returns a copy of the vars with spec content loaded from the database.
// This is the preferred method for loading spec content, as specs are stored exclusively
// in the database (not as file artifacts) to avoid merge conflicts in worktrees.
// If the backend is nil or loading fails, the original SpecContent is preserved.
func (v TemplateVars) WithSpecFromDatabase(backend storage.Backend, taskID string) TemplateVars {
	if backend == nil {
		return v
	}
	specContent, err := backend.LoadSpec(taskID)
	if err != nil {
		slog.Debug("failed to load spec from database",
			"task_id", taskID,
			"error", err,
		)
		return v
	}
	if specContent != "" {
		v.SpecContent = specContent
	}
	return v
}

// WithArtifactsFromDatabase returns a copy of the vars with artifact content loaded
// from the database. This includes: ResearchContent, TDDTestsContent, BreakdownContent.
// SpecContent is loaded separately via WithSpecFromDatabase.
// If the backend is nil or loading fails, the original content is preserved.
func (v TemplateVars) WithArtifactsFromDatabase(backend storage.Backend, taskID string) TemplateVars {
	if backend == nil {
		return v
	}

	// Load all artifacts for this task from the database
	artifacts, err := backend.LoadAllArtifacts(taskID)
	if err != nil {
		slog.Debug("failed to load artifacts from database",
			"task_id", taskID,
			"error", err,
		)
		return v
	}

	// Apply each artifact if present (DB takes precedence over file-based)
	if content, ok := artifacts["research"]; ok && content != "" {
		v.ResearchContent = content
	}
	if content, ok := artifacts["tdd_write"]; ok && content != "" {
		v.TDDTestsContent = content
	}
	if content, ok := artifacts["breakdown"]; ok && content != "" {
		v.BreakdownContent = content
	}
	// tiny_spec can also contain TDD content (combined spec+TDD for trivial/small)
	if v.TDDTestsContent == "" {
		if content, ok := artifacts["tiny_spec"]; ok && content != "" {
			v.TDDTestsContent = content
		}
	}

	return v
}

// WithConstitutionFromDatabase returns a copy of the vars with constitution content loaded
// from the database. The constitution contains project-level principles that guide all tasks.
// If no constitution is configured or loading fails, the original content is preserved.
func (v TemplateVars) WithConstitutionFromDatabase(backend storage.Backend) TemplateVars {
	if backend == nil {
		return v
	}

	content, _, err := backend.LoadConstitution()
	if err != nil {
		// Don't log for ErrNoConstitution - that's expected when not configured
		return v
	}
	if content != "" {
		v.ConstitutionContent = content
	}
	return v
}

// WithReviewContext returns a copy of the vars with review context applied.
// For round 2+, it loads the previous round's findings from the database and formats
// them for injection into the prompt template via {{REVIEW_FINDINGS}}.
func (v TemplateVars) WithReviewContext(backend storage.Backend, taskID string, round int) TemplateVars {
	v.ReviewRound = round

	// For round 1, no prior findings to load
	if round <= 1 || backend == nil {
		return v
	}

	// Load previous round's findings
	findings, err := backend.LoadReviewFindings(taskID, round-1)
	if err != nil {
		slog.Debug("failed to load review findings from database",
			"task_id", taskID,
			"round", round-1,
			"error", err,
		)
		return v
	}

	if findings != nil {
		// Format findings for the template
		v.ReviewFindings = formatReviewFindingsForTemplate(findings)
	}

	return v
}

// formatReviewFindingsForTemplate formats storage.ReviewFindings for template injection.
func formatReviewFindingsForTemplate(findings *storage.ReviewFindings) string {
	if findings == nil {
		return "No findings from previous round."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Round %d Summary\n\n", findings.Round))
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

	sb.WriteString(fmt.Sprintf("**Issues Found:** %d high, %d medium, %d low\n\n", highCount, mediumCount, lowCount))

	if len(findings.Issues) > 0 {
		sb.WriteString("### Issues to Verify\n\n")
		for i, issue := range findings.Issues {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, strings.ToUpper(issue.Severity), issue.Description))
			if issue.File != "" {
				sb.WriteString(fmt.Sprintf(" (in %s", issue.File))
				if issue.Line > 0 {
					sb.WriteString(fmt.Sprintf(":%d", issue.Line))
				}
				sb.WriteString(")")
			}
			sb.WriteString("\n")
			if issue.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("   Suggested fix: %s\n", issue.Suggestion))
			}
		}
	}

	if len(findings.Positives) > 0 {
		sb.WriteString("\n### Positive Notes\n\n")
		for _, p := range findings.Positives {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	if len(findings.Questions) > 0 {
		sb.WriteString("\n### Questions from Review\n\n")
		for _, q := range findings.Questions {
			sb.WriteString(fmt.Sprintf("- %s\n", q))
		}
	}

	return sb.String()
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

// loadFromTranscript is a no-op fallback.
// Artifact content should be written directly to artifact files by agents.
// No extraction from transcripts is performed - if the artifact file doesn't
// exist in {taskDir}/artifacts/{phase}.md, there's no artifact for that phase.
func loadFromTranscript(_, _ string) string {
	// No transcript extraction - agents write files directly
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
		if t.Status == task.StatusCompleted {
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
		if t.Status == task.StatusCompleted {
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

// BuildContinuationPrompt creates a short prompt for resuming a paused session.
// When using Claude's --resume flag, the session already has full context,
// so we just need to signal that we're continuing from where we left off.
func BuildContinuationPrompt(s *state.State, phaseID string) string {
	iteration := 1
	if s != nil {
		iteration = s.CurrentIteration
	}

	return fmt.Sprintf(`Resuming from where we paused.

Work has been committed to git. Continue from iteration %d of the %s phase.

Continue working on the task.`,
		iteration, phaseID)
}
