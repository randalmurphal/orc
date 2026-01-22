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
func (r *Resolver) ResolveAll(ctx context.Context, defs []Definition, rctx *ResolutionContext) (VariableSet, error) {
	vars := make(VariableSet)

	// First, add built-in variables from the resolution context
	r.addBuiltinVariables(vars, rctx)

	// Then resolve custom variable definitions
	for _, def := range defs {
		resolved, err := r.Resolve(ctx, &def, rctx)
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
func (r *Resolver) Resolve(ctx context.Context, def *Definition, rctx *ResolutionContext) (*ResolvedVariable, error) {
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

	// Resolve based on source type
	var value string
	var err error

	switch def.SourceType {
	case SourceStatic:
		value, err = r.resolveStatic(def)
	case SourceEnv:
		value, err = r.resolveEnv(def, rctx)
	case SourceScript:
		value, err = r.resolveScript(ctx, def)
	case SourceAPI:
		value, err = r.resolveAPI(ctx, def)
	case SourcePhaseOutput:
		value, err = r.resolvePhaseOutput(def, rctx)
	case SourcePromptFragment:
		value, err = r.resolvePromptFragment(def)
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

// resolveStatic returns a static value.
func (r *Resolver) resolveStatic(def *Definition) (string, error) {
	cfg, err := ParseStaticConfig(def.SourceConfig)
	if err != nil {
		return "", fmt.Errorf("parse static config: %w", err)
	}
	return cfg.Value, nil
}

// resolveEnv reads an environment variable.
func (r *Resolver) resolveEnv(def *Definition, rctx *ResolutionContext) (string, error) {
	cfg, err := ParseEnvConfig(def.SourceConfig)
	if err != nil {
		return "", fmt.Errorf("parse env config: %w", err)
	}

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

// resolveScript executes a script and returns its output.
func (r *Resolver) resolveScript(ctx context.Context, def *Definition) (string, error) {
	cfg, err := ParseScriptConfig(def.SourceConfig)
	if err != nil {
		return "", fmt.Errorf("parse script config: %w", err)
	}

	return r.scriptExecutor.Execute(ctx, cfg, r.projectRoot)
}

// resolveAPI makes an HTTP request and returns the response.
func (r *Resolver) resolveAPI(ctx context.Context, def *Definition) (string, error) {
	cfg, err := ParseAPIConfig(def.SourceConfig)
	if err != nil {
		return "", fmt.Errorf("parse api config: %w", err)
	}

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
	defer resp.Body.Close()

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

	// Apply jq filter if specified
	if cfg.JQFilter != "" {
		// For now, we don't implement jq - that would require a dependency.
		// Instead, we'll support simple JSON path extraction in a future iteration.
		// Just return the full response for now.
		// TODO: Add jq support via gojq or similar
		_ = cfg.JQFilter
	}

	return strings.TrimSpace(result), nil
}

// resolvePhaseOutput reads the artifact or transcript from a prior phase.
func (r *Resolver) resolvePhaseOutput(def *Definition, rctx *ResolutionContext) (string, error) {
	cfg, err := ParsePhaseOutputConfig(def.SourceConfig)
	if err != nil {
		return "", fmt.Errorf("parse phase output config: %w", err)
	}

	if rctx == nil || rctx.PriorOutputs == nil {
		return "", fmt.Errorf("no prior outputs available")
	}

	value, ok := rctx.PriorOutputs[cfg.Phase]
	if !ok {
		return "", fmt.Errorf("no output from phase %s", cfg.Phase)
	}

	return value, nil
}

// resolvePromptFragment reads a prompt fragment file.
func (r *Resolver) resolvePromptFragment(def *Definition) (string, error) {
	cfg, err := ParsePromptFragmentConfig(def.SourceConfig)
	if err != nil {
		return "", fmt.Errorf("parse prompt fragment config: %w", err)
	}

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

	// Run context
	vars["RUN_ID"] = rctx.WorkflowRunID
	vars["WORKFLOW_ID"] = rctx.WorkflowID
	vars["PROMPT"] = rctx.Prompt
	vars["INSTRUCTIONS"] = rctx.Instructions

	// Phase context
	vars["PHASE"] = rctx.Phase
	vars["ITERATION"] = fmt.Sprintf("%d", rctx.Iteration)

	// Git context
	vars["WORKTREE_PATH"] = rctx.WorkingDir
	vars["TASK_BRANCH"] = rctx.TaskBranch
	vars["TARGET_BRANCH"] = rctx.TargetBranch

	// Add prior phase outputs with OUTPUT_ prefix
	for phase, content := range rctx.PriorOutputs {
		key := "OUTPUT_" + strings.ToUpper(phase)
		vars[key] = content

		// Also add common aliases
		switch phase {
		case "spec", "tiny_spec":
			vars["SPEC_CONTENT"] = content
		case "research":
			vars["RESEARCH_CONTENT"] = content
		case "design":
			vars["DESIGN_CONTENT"] = content
		case "tdd_write":
			vars["TDD_TESTS_CONTENT"] = content
		case "breakdown":
			vars["BREAKDOWN_CONTENT"] = content
		case "implement":
			vars["IMPLEMENT_CONTENT"] = content
		}
	}
}

// ClearCache clears the resolver's cache.
func (r *Resolver) ClearCache() {
	r.cache.Clear()
}

// RenderTemplate applies variable substitution to a template string.
// Variables use the {{VAR}} format. Missing variables are replaced with empty strings.
func RenderTemplate(template string, vars VariableSet) string {
	result := template

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

// RenderTemplateStrict is like RenderTemplate but returns an error for missing variables.
func RenderTemplateStrict(template string, vars VariableSet) (string, []string) {
	var missing []string
	result := template

	// Find all variables in template
	pattern := regexp.MustCompile(`\{\{([A-Z_][A-Z0-9_]*)\}\}`)
	matches := pattern.FindAllStringSubmatch(template, -1)

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
