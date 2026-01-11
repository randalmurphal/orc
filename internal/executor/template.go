// Package executor provides task phase execution for orc.
package executor

import (
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/templates"
)

// TemplateVars holds all variables for template rendering.
type TemplateVars struct {
	TaskID          string
	TaskTitle       string
	TaskDescription string
	Phase           string
	Weight          string
	Iteration       int
	RetryContext    string
	ResearchContent string
	SpecContent     string
	DesignContent   string
}

// RenderTemplate performs variable substitution on a template string.
// Variables use the {{VAR}} format. Missing variables are replaced with
// empty strings.
func RenderTemplate(tmpl string, vars TemplateVars) string {
	replacements := map[string]string{
		"{{TASK_ID}}":          vars.TaskID,
		"{{TASK_TITLE}}":       vars.TaskTitle,
		"{{TASK_DESCRIPTION}}": vars.TaskDescription,
		"{{PHASE}}":            vars.Phase,
		"{{WEIGHT}}":           vars.Weight,
		"{{ITERATION}}":        fmt.Sprintf("%d", vars.Iteration),
		"{{RETRY_CONTEXT}}":    vars.RetryContext,
		"{{RESEARCH_CONTENT}}": vars.ResearchContent,
		"{{SPEC_CONTENT}}":     vars.SpecContent,
		"{{DESIGN_CONTENT}}":   vars.DesignContent,
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

	// Populate prior phase content from state if available
	if s != nil {
		vars.ResearchContent = loadPriorContent(s, "research")
		vars.SpecContent = loadPriorContent(s, "spec")
		vars.DesignContent = loadPriorContent(s, "design")
	}

	return vars
}

// loadPriorContent loads content from a completed prior phase.
// This reads from artifacts stored in the state.
func loadPriorContent(s *state.State, phaseID string) string {
	if s == nil || s.Phases == nil {
		return ""
	}

	ps, ok := s.Phases[phaseID]
	if !ok || ps.Status != state.StatusCompleted {
		return ""
	}

	// Prior content is stored in artifacts if available
	// For now, return empty - this can be enhanced to read from
	// artifact files when that functionality is implemented
	return ""
}
