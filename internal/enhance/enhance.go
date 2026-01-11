// Package enhance provides task description enhancement using Claude.
package enhance

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/randalmurphal/llmkit/claude"
	"github.com/randalmurphal/orc/internal/task"
)


// Mode determines how task enhancement is performed.
type Mode string

const (
	// ModeQuick skips enhancement entirely, uses --weight flag directly
	ModeQuick Mode = "quick"
	// ModeStandard uses AI to analyze and enhance the task
	ModeStandard Mode = "standard"
	// ModeInteractive allows user to review and edit enhancement
	ModeInteractive Mode = "interactive"
)

// Options configures task enhancement.
type Options struct {
	Mode    Mode
	Weight  string // For ModeQuick
	Timeout time.Duration
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		Mode:    ModeStandard,
		Timeout: 2 * time.Minute,
	}
}

// Result contains the enhancement output.
type Result struct {
	OriginalTitle string
	EnhancedTitle string
	Description   string
	Weight        string
	Analysis      *Analysis
	SessionID     string
	TokensUsed    int
	CostUSD       float64
}

// Analysis contains the AI's analysis of the task.
type Analysis struct {
	Scope         string   `yaml:"scope" json:"scope"`
	AffectedFiles []string `yaml:"affected_files,omitempty" json:"affected_files,omitempty"`
	Risks         []string `yaml:"risks,omitempty" json:"risks,omitempty"`
	Dependencies  []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	TestStrategy  string   `yaml:"test_strategy,omitempty" json:"test_strategy,omitempty"`
}

// Enhance analyzes and enhances a task description.
func Enhance(ctx context.Context, title string, opts Options) (*Result, error) {
	if opts.Mode == ModeQuick {
		return quickEnhance(title, opts.Weight)
	}

	return standardEnhance(ctx, title, opts)
}

// quickEnhance creates a result without calling Claude.
func quickEnhance(title string, weight string) (*Result, error) {
	if weight == "" {
		weight = "medium" // Default weight
	}

	// Validate weight
	validWeights := map[string]bool{
		"trivial":    true,
		"small":      true,
		"medium":     true,
		"large":      true,
		"greenfield": true,
	}
	if !validWeights[weight] {
		return nil, fmt.Errorf("invalid weight: %s (must be trivial, small, medium, large, or greenfield)", weight)
	}

	return &Result{
		OriginalTitle: title,
		EnhancedTitle: title,
		Description:   title,
		Weight:        weight,
	}, nil
}

// standardEnhance uses Claude to analyze and enhance the task.
func standardEnhance(ctx context.Context, title string, opts Options) (*Result, error) {
	client := claude.GetDefaultClient()

	prompt := buildEnhancePrompt(title)

	req := claude.CompletionRequest{
		Messages: []claude.Message{
			{Role: claude.RoleUser, Content: prompt},
		},
		Model: "claude-sonnet-4-20250514", // Use Sonnet for faster responses
	}

	var response strings.Builder
	var totalTokens int

	streamCh, err := client.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("enhance request: %w", err)
	}

	for chunk := range streamCh {
		if chunk.Error != nil {
			return nil, fmt.Errorf("stream error: %w", chunk.Error)
		}
		if chunk.Content != "" {
			response.WriteString(chunk.Content)
		}
		if chunk.Done && chunk.Usage != nil {
			totalTokens = chunk.Usage.TotalTokens
		}
	}

	// Parse the response
	enhanceResult, err := parseEnhanceResponse(title, response.String())
	if err != nil {
		// Fallback to basic result
		enhanceResult = &Result{
			OriginalTitle: title,
			EnhancedTitle: title,
			Description:   title,
			Weight:        "medium",
		}
	}

	// Add session metadata
	enhanceResult.TokensUsed = totalTokens

	return enhanceResult, nil
}

// buildEnhancePrompt creates the prompt for task enhancement.
func buildEnhancePrompt(title string) string {
	return fmt.Sprintf(`Analyze this task and provide enhancement information.

Task: %s

Respond in this exact format:

<enhanced_title>A clear, actionable title for the task</enhanced_title>

<description>
A detailed description of what needs to be done, including:
- Specific goals
- Expected outcomes
- Any constraints or requirements
</description>

<weight>one of: trivial, small, medium, large, greenfield</weight>

<analysis>
<scope>Brief description of the scope</scope>
<affected_files>Comma-separated list of likely files/directories affected, or "unknown" if unclear</affected_files>
<risks>Comma-separated list of potential risks, or "none" if straightforward</risks>
<dependencies>Comma-separated list of dependencies/prerequisites, or "none"</dependencies>
<test_strategy>Brief testing approach</test_strategy>
</analysis>

Weight guidelines:
- trivial: Simple fix, < 5 lines, obvious solution (e.g., typo fix, config change)
- small: Single-file change, clear implementation (e.g., add a function, fix a bug)
- medium: Multi-file changes, some complexity (e.g., new feature, refactoring)
- large: Significant changes, multiple components (e.g., new system, major refactor)
- greenfield: New project or major new subsystem from scratch`, title)
}

// parseEnhanceResponse extracts structured data from Claude's response.
func parseEnhanceResponse(originalTitle, response string) (*Result, error) {
	result := &Result{
		OriginalTitle: originalTitle,
	}

	// Extract enhanced title
	result.EnhancedTitle = extractTag(response, "enhanced_title")
	if result.EnhancedTitle == "" {
		result.EnhancedTitle = originalTitle
	}

	// Extract description
	result.Description = extractTag(response, "description")
	if result.Description == "" {
		result.Description = originalTitle
	}

	// Extract weight
	weight := strings.ToLower(strings.TrimSpace(extractTag(response, "weight")))
	validWeights := map[string]bool{
		"trivial": true, "small": true, "medium": true,
		"large": true, "greenfield": true,
	}
	if validWeights[weight] {
		result.Weight = weight
	} else {
		result.Weight = "medium" // Default
	}

	// Extract analysis
	analysisContent := extractTag(response, "analysis")
	if analysisContent != "" {
		result.Analysis = &Analysis{
			Scope:         extractTag(analysisContent, "scope"),
			TestStrategy:  extractTag(analysisContent, "test_strategy"),
			AffectedFiles: splitList(extractTag(analysisContent, "affected_files")),
			Risks:         splitList(extractTag(analysisContent, "risks")),
			Dependencies:  splitList(extractTag(analysisContent, "dependencies")),
		}
	}

	return result, nil
}

// extractTag extracts content between XML-like tags.
func extractTag(content, tag string) string {
	pattern := fmt.Sprintf(`<%s>([^<]*)</%s>`, tag, tag)
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try multiline pattern
	pattern = fmt.Sprintf(`(?s)<%s>(.*?)</%s>`, tag, tag)
	re = regexp.MustCompile(pattern)
	matches = re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}

// splitList splits a comma-separated list, filtering empty and "none" values.
func splitList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ToLower(s) == "none" || strings.ToLower(s) == "unknown" {
		return nil
	}

	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ApplyToTask applies the enhancement result to a task.
func ApplyToTask(t *task.Task, result *Result) {
	t.Title = result.EnhancedTitle
	t.Description = result.Description
	t.Weight = task.Weight(result.Weight)
}
