package planner

import (
	"bytes"
	"fmt"
	"path/filepath"
	"text/template"

	"github.com/randalmurphal/orc/templates"
)

// PromptData contains the data for prompt template rendering.
type PromptData struct {
	// SpecFiles is a formatted list of spec files
	SpecFiles string

	// SpecContent is the aggregated content of all spec files
	SpecContent string

	// ProjectName is the name of the project
	ProjectName string

	// ProjectPath is the path to the project
	ProjectPath string

	// Language is the detected project language
	Language string

	// Frameworks is a comma-separated list of detected frameworks
	Frameworks string

	// Initiative context (optional)
	InitiativeID        string
	InitiativeTitle     string
	InitiativeVision    string
	InitiativeDecisions string
}

// GeneratePrompt generates the planning prompt from spec files.
func GeneratePrompt(files []*SpecFile, data *PromptData) (string, error) {
	// Read template from centralized templates
	tmplContent, err := templates.Prompts.ReadFile("prompts/plan_from_spec.md")
	if err != nil {
		return "", fmt.Errorf("read prompt template: %w", err)
	}

	// Parse template
	tmpl, err := template.New("plan_prompt").Parse(string(tmplContent))
	if err != nil {
		return "", fmt.Errorf("parse prompt template: %w", err)
	}

	// Populate data
	if data == nil {
		data = &PromptData{}
	}
	data.SpecFiles = DescribeFiles(files)
	data.SpecContent = AggregateContent(files)

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute prompt template: %w", err)
	}

	return buf.String(), nil
}

// ProjectNameFromPath extracts the project name from a path.
func ProjectNameFromPath(path string) string {
	return filepath.Base(path)
}
