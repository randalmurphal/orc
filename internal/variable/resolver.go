package variable

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Resolver resolves variable definitions to their values.
// It supports caching, multiple source types, and builds the complete
// variable set needed for template rendering.
type Resolver struct {
	cache          *Cache
	scriptExecutor *ScriptExecutor
	httpClient     *http.Client
	projectRoot    string
}

// NewResolver creates a new variable resolver for the given project.
func NewResolver(projectRoot string) *Resolver {
	return &Resolver{
		cache:          NewCache(),
		scriptExecutor: NewScriptExecutor(projectRoot),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		projectRoot: projectRoot,
	}
}

// ResolveAll resolves all variable definitions and returns a VariableSet.
// Built-in variables (TASK_*, PHASE_*, etc.) are included automatically.
// Variables are resolved in order, so later variables can reference earlier ones
// via {{VAR}} interpolation in their source configs.
func (r *Resolver) ResolveAll(ctx context.Context, defs []Definition, rctx *ResolutionContext) (VariableSet, error) {
	vars := make(VariableSet)

	// First, add built-in variables from the resolution context
	r.addBuiltinVariables(vars, rctx)

	// Then resolve custom variable definitions in order.
	// Each variable can reference previously resolved variables via {{VAR}} patterns.
	for _, def := range defs {
		resolved, err := r.Resolve(ctx, &def, rctx, vars)
		if err != nil {
			if def.Required {
				return nil, fmt.Errorf("resolve required variable %s: %w", def.Name, err)
			}
			// Use default value for non-required variables on error
			if def.DefaultValue != "" {
				vars[def.Name] = def.DefaultValue
			}
			continue
		}
		vars[def.Name] = resolved.Value
	}

	return vars, nil
}

// Resolve resolves a single variable definition.
// currentVars contains already-resolved variables used for {{VAR}} interpolation in source configs.
func (r *Resolver) Resolve(ctx context.Context, def *Definition, rctx *ResolutionContext, currentVars VariableSet) (*ResolvedVariable, error) {
	// Check cache first
	cacheKey := CacheKey(def, rctx)
	if def.CacheTTL > 0 {
		if value, ok := r.cache.Get(cacheKey); ok {
			return &ResolvedVariable{
				Name:        def.Name,
				Value:       value,
				Source:      def.SourceType,
				ResolvedAt:  time.Now(),
				CachedUntil: time.Now().Add(def.CacheTTL),
			}, nil
		}
	}

	// Resolve based on source type.
	// Pattern: parse config -> interpolate with currentVars -> resolve -> extract.
	var value string
	var err error

	switch def.SourceType {
	case SourceStatic:
		cfg, parseErr := ParseStaticConfig(def.SourceConfig)
		if parseErr != nil {
			err = fmt.Errorf("parse static config: %w", parseErr)
			break
		}
		cfg.Interpolate(currentVars)
		value = cfg.Value

	case SourceEnv:
		cfg, parseErr := ParseEnvConfig(def.SourceConfig)
		if parseErr != nil {
			err = fmt.Errorf("parse env config: %w", parseErr)
			break
		}
		cfg.Interpolate(currentVars)
		value, err = r.resolveEnvWithConfig(cfg, rctx)

	case SourceScript:
		cfg, parseErr := ParseScriptConfig(def.SourceConfig)
		if parseErr != nil {
			err = fmt.Errorf("parse script config: %w", parseErr)
			break
		}
		cfg.Interpolate(currentVars)
		value, err = r.scriptExecutor.Execute(ctx, cfg, r.projectRoot)

	case SourceAPI:
		cfg, parseErr := ParseAPIConfig(def.SourceConfig)
		if parseErr != nil {
			err = fmt.Errorf("parse api config: %w", parseErr)
			break
		}
		cfg.Interpolate(currentVars)
		value, err = r.resolveAPIWithConfig(ctx, cfg)

	case SourcePhaseOutput:
		cfg, parseErr := ParsePhaseOutputConfig(def.SourceConfig)
		if parseErr != nil {
			err = fmt.Errorf("parse phase output config: %w", parseErr)
			break
		}
		cfg.Interpolate(currentVars)
		value, err = r.resolvePhaseOutputWithConfig(cfg, rctx)

	case SourcePromptFragment:
		cfg, parseErr := ParsePromptFragmentConfig(def.SourceConfig)
		if parseErr != nil {
			err = fmt.Errorf("parse prompt fragment config: %w", parseErr)
			break
		}
		cfg.Interpolate(currentVars)
		value, err = r.resolvePromptFragmentWithConfig(cfg)

	default:
		return nil, fmt.Errorf("unknown source type: %s", def.SourceType)
	}

	if err != nil {
		return &ResolvedVariable{
			Name:   def.Name,
			Source: def.SourceType,
			Error:  err,
		}, err
	}

	// Apply JSONPath extraction if configured
	if def.Extract != "" {
		value = ExtractJSONPath(value, def.Extract)
	}

	// Cache if TTL is set
	if def.CacheTTL > 0 {
		r.cache.Set(cacheKey, value, def.CacheTTL)
	}

	return &ResolvedVariable{
		Name:        def.Name,
		Value:       value,
		Source:      def.SourceType,
		ResolvedAt:  time.Now(),
		CachedUntil: time.Now().Add(def.CacheTTL),
	}, nil
}

// resolveEnvWithConfig reads an environment variable with a pre-parsed config.
func (r *Resolver) resolveEnvWithConfig(cfg *EnvConfig, rctx *ResolutionContext) (string, error) {
	// Check context environment first (for testing)
	if rctx != nil && rctx.Environment != nil {
		if value, ok := rctx.Environment[cfg.Var]; ok {
			return value, nil
		}
	}

	// Then check actual environment
	value := os.Getenv(cfg.Var)
	if value == "" && cfg.Default != "" {
		return cfg.Default, nil
	}

	return value, nil
}

// resolveAPIWithConfig makes an HTTP request with a pre-parsed config.
func (r *Resolver) resolveAPIWithConfig(ctx context.Context, cfg *APIConfig) (string, error) {
	// Validate URL
	if !strings.HasPrefix(cfg.URL, "https://") && !strings.HasPrefix(cfg.URL, "http://") {
		return "", fmt.Errorf("invalid URL scheme: must be http or https")
	}

	// Default to GET
	method := cfg.Method
	if method == "" {
		method = "GET"
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, cfg.URL, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// Add headers
	for key, value := range cfg.Headers {
		req.Header.Set(key, value)
	}

	// Use custom timeout if specified
	client := r.httpClient
	if cfg.TimeoutMS > 0 {
		client = &http.Client{
			Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond,
		}
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Read body (limited to 10MB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	result := string(body)

	// Apply JQ filter if specified (using gjson syntax, not actual jq)
	if cfg.JQFilter != "" {
		result = ExtractJSONPath(result, cfg.JQFilter)
	}

	return strings.TrimSpace(result), nil
}

// resolvePhaseOutputWithConfig reads the artifact from a prior phase with a pre-parsed config.
func (r *Resolver) resolvePhaseOutputWithConfig(cfg *PhaseOutputConfig, rctx *ResolutionContext) (string, error) {
	if rctx == nil || rctx.PriorOutputs == nil {
		return "", fmt.Errorf("no prior outputs available")
	}

	value, ok := rctx.PriorOutputs[cfg.Phase]
	if !ok {
		return "", fmt.Errorf("no output from phase %s", cfg.Phase)
	}

	return value, nil
}

// resolvePromptFragmentWithConfig reads a prompt fragment file with a pre-parsed config.
func (r *Resolver) resolvePromptFragmentWithConfig(cfg *PromptFragmentConfig) (string, error) {
	// Resolve path
	var fragmentPath string
	if filepath.IsAbs(cfg.Path) {
		fragmentPath = cfg.Path
	} else if strings.HasPrefix(cfg.Path, ".orc/") {
		fragmentPath = filepath.Join(r.projectRoot, cfg.Path)
	} else {
		// Default to .orc/prompts/fragments/
		fragmentPath = filepath.Join(r.projectRoot, ".orc", "prompts", "fragments", cfg.Path)
	}

	content, err := os.ReadFile(fragmentPath)
	if err != nil {
		return "", fmt.Errorf("read fragment %s: %w", fragmentPath, err)
	}

	return strings.TrimSpace(string(content)), nil
}

// addBuiltinVariables adds all built-in variables from the resolution context.
func (r *Resolver) addBuiltinVariables(vars VariableSet, rctx *ResolutionContext) {
	if rctx == nil {
		return
	}

	// Task context
	vars["TASK_ID"] = rctx.TaskID
	vars["TASK_TITLE"] = rctx.TaskTitle
	vars["TASK_DESCRIPTION"] = rctx.TaskDescription
	vars["TASK_CATEGORY"] = rctx.TaskCategory
	vars["WEIGHT"] = rctx.TaskWeight

	// Run context
	vars["RUN_ID"] = rctx.WorkflowRunID
	vars["WORKFLOW_ID"] = rctx.WorkflowID
	vars["PROMPT"] = rctx.Prompt
	vars["INSTRUCTIONS"] = rctx.Instructions

	// Phase context
	vars["PHASE"] = rctx.Phase
	vars["ITERATION"] = fmt.Sprintf("%d", rctx.Iteration)
	// Structured retry variables (only populated when retry is active)
	if rctx.RetryAttempt > 0 {
		vars["RETRY_ATTEMPT"] = fmt.Sprintf("%d", rctx.RetryAttempt)
		vars["RETRY_FROM_PHASE"] = rctx.RetryFromPhase
		vars["RETRY_REASON"] = rctx.RetryReason
		if rctx.RetryFeedback != "" {
			vars["RETRY_FEEDBACK"] = rctx.RetryFeedback
		}
	}

	// Git context
	vars["WORKTREE_PATH"] = rctx.WorkingDir
	vars["PROJECT_ROOT"] = rctx.ProjectRoot
	vars["TASK_BRANCH"] = rctx.TaskBranch
	vars["TARGET_BRANCH"] = rctx.TargetBranch

	// Constitution content (project-level principles)
	vars["CONSTITUTION_CONTENT"] = rctx.ConstitutionContent

	// Error patterns (language-specific error handling idioms)
	if rctx.ErrorPatterns != "" {
		vars["ERROR_PATTERNS"] = rctx.ErrorPatterns
	}

	// Initiative context
	vars["INITIATIVE_ID"] = rctx.InitiativeID
	vars["INITIATIVE_TITLE"] = rctx.InitiativeTitle
	vars["INITIATIVE_VISION"] = rctx.InitiativeVision
	vars["INITIATIVE_DECISIONS"] = rctx.InitiativeDecisions
	vars["INITIATIVE_NOTES"] = rctx.InitiativeNotes
	vars["INITIATIVE_TASKS"] = rctx.InitiativeTasks
	// Format full initiative context section if initiative is set
	if rctx.InitiativeID != "" {
		vars["INITIATIVE_CONTEXT"] = formatInitiativeContext(rctx)
	}

	// Review context
	vars["REVIEW_ROUND"] = fmt.Sprintf("%d", rctx.ReviewRound)
	vars["REVIEW_FINDINGS"] = rctx.ReviewFindings

	// Loop context
	// LOOP_ITERATION is empty when not in a loop (0), otherwise stringified
	if rctx.LoopIteration > 0 {
		vars["LOOP_ITERATION"] = fmt.Sprintf("%d", rctx.LoopIteration)
	}

	// Project detection context
	vars["LANGUAGE"] = rctx.Language
	if rctx.HasFrontend {
		vars["HAS_FRONTEND"] = "true"
	}
	if rctx.HasTests {
		vars["HAS_TESTS"] = "true"
	}
	vars["TEST_COMMAND"] = rctx.TestCommand
	vars["LINT_COMMAND"] = rctx.LintCommand
	vars["BUILD_COMMAND"] = rctx.BuildCommand
	vars["FRAMEWORKS"] = strings.Join(rctx.Frameworks, ", ")

	// Testing configuration
	if rctx.CoverageThreshold > 0 {
		vars["COVERAGE_THRESHOLD"] = fmt.Sprintf("%d", rctx.CoverageThreshold)
	} else {
		vars["COVERAGE_THRESHOLD"] = "85" // Default
	}

	// UI testing context
	if rctx.RequiresUITesting {
		vars["REQUIRES_UI_TESTING"] = "true"
	}
	vars["SCREENSHOT_DIR"] = rctx.ScreenshotDir
	vars["TEST_RESULTS"] = rctx.TestResults
	vars["TDD_TEST_PLAN"] = rctx.TDDTestPlan

	// Scratchpad context (persistent notes from phase execution)
	vars["PREV_SCRATCHPAD"] = rctx.PrevScratchpad
	vars["RETRY_SCRATCHPAD"] = rctx.RetryScratchpad

	// Project brief (auto-generated context from task history)
	vars["PROJECT_BRIEF"] = rctx.ProjectBrief

	// Control-plane context
	vars["PENDING_RECOMMENDATIONS"] = rctx.PendingRecommendations
	vars["COMPLETION_RECOMMENDATIONS"] = rctx.CompletionRecommendations
	vars["ATTENTION_SUMMARY"] = rctx.AttentionSummary
	vars["HANDOFF_CONTEXT"] = rctx.HandoffContext
	vars["INDEXED_ARTIFACTS"] = rctx.IndexedArtifacts

	// Discussion workspace context
	vars["THREAD_ID"] = rctx.ThreadID
	vars["THREAD_TITLE"] = rctx.ThreadTitle
	vars["THREAD_CONTEXT"] = rctx.ThreadContext
	vars["THREAD_HISTORY"] = rctx.ThreadHistory
	vars["THREAD_LINKED_CONTEXT"] = rctx.ThreadLinkedContext
	vars["THREAD_RECOMMENDATION_DRAFTS"] = rctx.ThreadRecommendationDrafts
	vars["THREAD_DECISION_DRAFTS"] = rctx.ThreadDecisionDrafts

	// Automation context
	vars["RECENT_COMPLETED_TASKS"] = rctx.RecentCompletedTasks
	vars["RECENT_CHANGED_FILES"] = rctx.RecentChangedFiles
	vars["CHANGELOG_CONTENT"] = rctx.ChangelogContent
	vars["CLAUDEMD_CONTENT"] = rctx.ClaudeMDContent

	// Provider context
	vars["PROVIDER"] = rctx.Provider
	vars["COMMIT_AUTHOR"] = commitAuthorForProvider(rctx.Provider)
	if providerSupportsSubagents(rctx.Provider) {
		vars["SUPPORTS_SUBAGENTS"] = "true"
	}

	// QA E2E testing context
	if rctx.QAIteration > 0 {
		vars["QA_ITERATION"] = fmt.Sprintf("%d", rctx.QAIteration)
	}
	if rctx.QAMaxLoops > 0 {
		vars["QA_MAX_LOOPS"] = fmt.Sprintf("%d", rctx.QAMaxLoops)
	}
	vars["QA_FINDINGS"] = rctx.QAFindings
	vars["BEFORE_IMAGES"] = rctx.BeforeImages
	vars["PREVIOUS_FINDINGS"] = rctx.PreviousFindings
	if rctx.TaskID != "" {
		vars["QA_OUTPUT_DIR"] = "/tmp/orc-qa-" + rctx.TaskID
	}

	// Add prior phase outputs with OUTPUT_ prefix (generic accessor)
	for phase, content := range rctx.PriorOutputs {
		key := "OUTPUT_" + strings.ToUpper(phase)
		vars[key] = content
	}

	// Add named phase output variables (e.g., SPEC_CONTENT, TDD_TESTS_CONTENT)
	// These are populated by applyPhaseContentToVars() using each phase template's
	// OutputVarName field — the single source of truth for output variable naming.
	for varName, content := range rctx.PhaseOutputVars {
		vars[varName] = content
	}

	// Allow environment overrides of builtins (e.g., bench injecting TASK_ID
	// for ContextStandalone mode). rctx.Environment is nil by default,
	// so this is a no-op for all existing callers.
	for k, v := range rctx.Environment {
		if v != "" {
			vars[k] = v
		}
	}
}

// formatInitiativeContext builds a complete initiative context section for templates.
func formatInitiativeContext(rctx *ResolutionContext) string {
	if rctx.InitiativeID == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Initiative Context\n\n")
	fmt.Fprintf(&sb, "This task is part of **%s** (%s).\n", rctx.InitiativeTitle, rctx.InitiativeID)

	if rctx.InitiativeVision != "" {
		sb.WriteString("\n### Vision\n\n")
		sb.WriteString(rctx.InitiativeVision)
		sb.WriteString("\n")
	}

	if rctx.InitiativeDecisions != "" {
		sb.WriteString("\n### Decisions\n\n")
		sb.WriteString("The following decisions have been made for this initiative:\n\n")
		sb.WriteString(rctx.InitiativeDecisions)
		sb.WriteString("\n")
	}

	if rctx.InitiativeNotes != "" {
		sb.WriteString("\n### Knowledge from Prior Tasks\n\n")
		sb.WriteString("The following learnings have been captured from completed tasks in this initiative:\n\n")
		sb.WriteString(rctx.InitiativeNotes)
		sb.WriteString("\n")
	}

	sb.WriteString("\n**Alignment**: Ensure your work aligns with the initiative vision and respects prior decisions.\n")

	return sb.String()
}

// RenderTemplate applies variable substitution to a template string.
// Variables use the {{VAR}} format. Missing variables are replaced with empty strings.
// Also handles {{#if VAR}}...{{/if}} conditional blocks.
func RenderTemplate(template string, vars VariableSet) string {
	result := template

	// Process conditional blocks first: {{#if VAR}}...{{/if}}
	result = processConditionals(result, vars)

	// Replace all {{VAR}} patterns
	pattern := regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*)\}\}`)
	result = pattern.ReplaceAllStringFunc(result, func(match string) string {
		// Extract variable name (without {{ }})
		name := match[2 : len(match)-2]
		if value, ok := vars[name]; ok {
			return value
		}
		return "" // Missing variables become empty
	})

	return result
}

// processConditionals handles {{#if VAR}}...{{/if}} and {{#if VAR}}...{{else}}...{{/if}} blocks.
// If the variable exists and is non-empty, the if-branch is kept; otherwise the else-branch
// (or empty string if no else-branch).
func processConditionals(content string, vars VariableSet) string {
	// Pattern matches {{#if VAR}}...{{/if}} with the variable name
	pattern := regexp.MustCompile(`(?s)\{\{#if ([A-Z_][A-Z0-9_]*)\}\}(.*?)\{\{/if\}\}`)

	return pattern.ReplaceAllStringFunc(content, func(match string) string {
		// Extract variable name
		submatches := pattern.FindStringSubmatch(match)
		if len(submatches) < 3 {
			return ""
		}

		varName := submatches[1]
		blockContent := submatches[2]

		// Split on {{else}} to get if-branch and else-branch
		ifBranch, elseBranch, _ := strings.Cut(blockContent, "{{else}}")

		// Check if variable exists and is non-empty
		if value, ok := vars[varName]; ok && value != "" {
			return ifBranch
		}

		return elseBranch
	})
}

// commitAuthorForProvider returns the commit co-author string for a given LLM provider.
func commitAuthorForProvider(provider string) string {
	switch provider {
	case "codex":
		return "OpenAI Codex <noreply@openai.com>"
	default:
		return "Claude Sonnet 4.5 <noreply@anthropic.com>"
	}
}

// providerSupportsSubagents returns whether a provider supports spawning sub-agents.
// Currently only Claude Code supports the Task tool for sub-agent dispatch.
func providerSupportsSubagents(provider string) bool {
	return provider == "" || provider == "claude"
}

// RenderTemplateStrict is like RenderTemplate but returns an error for missing variables.
func RenderTemplateStrict(template string, vars VariableSet) (string, []string) {
	var missing []string
	result := template

	// Process conditional blocks first
	result = processConditionals(result, vars)

	// Find all variables in template (after conditionals processed)
	pattern := regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*)\}\}`)
	matches := pattern.FindAllStringSubmatch(result, -1)

	// Track which ones are missing
	for _, match := range matches {
		name := match[1]
		if _, ok := vars[name]; !ok {
			missing = append(missing, name)
		}
	}

	// Do the replacement
	result = pattern.ReplaceAllStringFunc(result, func(match string) string {
		name := match[2 : len(match)-2]
		if value, ok := vars[name]; ok {
			return value
		}
		return ""
	})

	return result, missing
}
