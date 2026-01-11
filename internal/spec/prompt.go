package spec

import (
	"bytes"
	_ "embed"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
)

//go:embed builtin/spec_session.md
var builtinPromptTemplate string

// PromptData contains the data used to generate the spec prompt.
type PromptData struct {
	// Title is the feature/spec title
	Title string

	// WorkDir is the working directory
	WorkDir string

	// Detection is the project detection info
	Detection *db.Detection

	// Initiative is the linked initiative (optional)
	Initiative *initiative.Initiative

	// CreateTasks determines if task creation instructions are included
	CreateTasks bool
}

// GeneratePrompt creates the spec session prompt.
func GeneratePrompt(data PromptData) (string, error) {
	// Build template data
	tmplData := map[string]any{
		"Title":       data.Title,
		"ProjectName": filepath.Base(data.WorkDir),
		"ProjectPath": data.WorkDir,
		"CreateTasks": data.CreateTasks,
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
	tmpl, err := template.New("spec").Parse(builtinPromptTemplate)
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
