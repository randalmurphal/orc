// Package executor provides task phase execution for orc.
package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/templates"
)

// TemplateVars holds all variables for template rendering.
type TemplateVars struct {
	TaskID           string
	TaskTitle        string
	TaskDescription  string
	Phase            string
	Weight           string
	Iteration        int
	RetryContext     string
	ResearchContent  string
	SpecContent      string
	DesignContent    string
	ImplementContent string
}

// RenderTemplate performs variable substitution on a template string.
// Variables use the {{VAR}} format. Missing variables are replaced with
// empty strings.
func RenderTemplate(tmpl string, vars TemplateVars) string {
	replacements := map[string]string{
		"{{TASK_ID}}":           vars.TaskID,
		"{{TASK_TITLE}}":        vars.TaskTitle,
		"{{TASK_DESCRIPTION}}":  vars.TaskDescription,
		"{{PHASE}}":             vars.Phase,
		"{{WEIGHT}}":            vars.Weight,
		"{{ITERATION}}":         fmt.Sprintf("%d", vars.Iteration),
		"{{RETRY_CONTEXT}}":     vars.RetryContext,
		"{{RESEARCH_CONTENT}}":  vars.ResearchContent,
		"{{SPEC_CONTENT}}":      vars.SpecContent,
		"{{DESIGN_CONTENT}}":    vars.DesignContent,
		"{{IMPLEMENT_CONTENT}}": vars.ImplementContent,
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
	vars := TemplateVars{
		TaskID:          t.ID,
		TaskTitle:       t.Title,
		TaskDescription: t.Description,
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

	return vars
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
