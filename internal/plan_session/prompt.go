package plan_session

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
)

//go:embed builtin/plan_session.md
var builtinPromptTemplate string

// PromptOverridePath is the path to the user-overridable prompt template.
const PromptOverridePath = ".orc/prompts/plan.md"

// PromptData contains the data used to generate the planning prompt.
type PromptData struct {
	// Mode is the planning mode (task or feature).
	Mode Mode

	// Title is the task/feature title.
	Title string

	// TaskID is the task ID (task mode only).
	TaskID string

	// TaskWeight is the task weight (task mode only).
	TaskWeight string

	// Description is the existing task description (task mode only).
	Description string

	// WorkDir is the working directory.
	WorkDir string

	// Detection is the project detection info.
	Detection *db.Detection

	// Initiative is the linked initiative (optional).
	Initiative *initiative.Initiative

	// CreateTasks determines if task creation instructions are included (feature mode).
	CreateTasks bool
}

// GeneratePrompt creates the planning session prompt.
func GeneratePrompt(data PromptData) (string, error) {
	// Load template (check for override first)
	templateContent := builtinPromptTemplate
	if data.WorkDir != "" {
		overridePath := filepath.Join(data.WorkDir, PromptOverridePath)
		if content, err := os.ReadFile(overridePath); err == nil {
			templateContent = string(content)
		}
	}

	// Build template data
	tmplData := map[string]any{
		"Mode":        string(data.Mode),
		"Title":       data.Title,
		"ProjectName": filepath.Base(data.WorkDir),
		"ProjectPath": data.WorkDir,
		"CreateTasks": data.CreateTasks,
	}

	// Add task-specific info
	if data.Mode == ModeTask {
		tmplData["TaskID"] = data.TaskID
		tmplData["Weight"] = data.TaskWeight
		tmplData["TaskDescription"] = data.Description
	}

	// Add detection info
	if data.Detection != nil {
		tmplData["Language"] = data.Detection.Language
		tmplData["Frameworks"] = strings.Join(data.Detection.Frameworks, ", ")
		tmplData["BuildTools"] = strings.Join(data.Detection.BuildTools, ", ")
		tmplData["HasTests"] = data.Detection.HasTests
		tmplData["TestCommand"] = data.Detection.TestCommand
	}

	// Add initiative info
	if data.Initiative != nil {
		tmplData["HasInitiative"] = true
		tmplData["InitiativeID"] = data.Initiative.ID
		tmplData["InitiativeTitle"] = data.Initiative.Title
		tmplData["InitiativeVision"] = data.Initiative.Vision
		tmplData["InitiativeDecisions"] = formatDecisions(data.Initiative.Decisions)
	}

	// Parse and execute template
	tmpl, err := template.New("plan").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tmplData); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// formatDecisions formats initiative decisions for the prompt.
func formatDecisions(decisions []initiative.Decision) string {
	if len(decisions) == 0 {
		return ""
	}

	var b strings.Builder
	for _, d := range decisions {
		b.WriteString(fmt.Sprintf("- %s", d.Decision))
		if d.Rationale != "" {
			b.WriteString(fmt.Sprintf(" (%s)", d.Rationale))
		}
		b.WriteString("\n")
	}
	return b.String()
}
