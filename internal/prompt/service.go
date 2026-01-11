// Package prompt provides prompt template management.
package prompt

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/templates"
)

// Source indicates where a prompt came from.
type Source string

const (
	SourceProject  Source = "project"  // .orc/prompts/
	SourceEmbedded Source = "embedded" // Embedded in binary
	SourceInline   Source = "inline"   // Inline in plan YAML
)

// PromptInfo contains metadata about a prompt.
type PromptInfo struct {
	Phase       string   `json:"phase"`
	Source      Source   `json:"source"`
	HasOverride bool     `json:"has_override"`
	Variables   []string `json:"variables"`
}

// Prompt contains full prompt data.
type Prompt struct {
	Phase     string   `json:"phase"`
	Content   string   `json:"content"`
	Source    Source   `json:"source"`
	Variables []string `json:"variables"`
}

// Service manages prompt templates.
type Service struct {
	orcDir string
}

// NewService creates a new prompt service.
func NewService(orcDir string) *Service {
	return &Service{orcDir: orcDir}
}

// DefaultService creates a service using the default .orc directory.
func DefaultService() *Service {
	return NewService(".orc")
}

// projectPromptsDir returns the path to the project prompts directory.
func (s *Service) projectPromptsDir() string {
	return filepath.Join(s.orcDir, "prompts")
}

// projectPromptPath returns the path to a project prompt file.
func (s *Service) projectPromptPath(phase string) string {
	return filepath.Join(s.projectPromptsDir(), phase+".md")
}

// Resolve returns the prompt content for a phase, resolving from project override first,
// then falling back to embedded templates.
// Returns content, source, and any error.
func (s *Service) Resolve(phase string) (string, Source, error) {
	// Try project override first
	projectPath := s.projectPromptPath(phase)
	if content, err := os.ReadFile(projectPath); err == nil {
		return string(content), SourceProject, nil
	}

	// Fall back to embedded
	embeddedPath := fmt.Sprintf("prompts/%s.md", phase)
	content, err := templates.Prompts.ReadFile(embeddedPath)
	if err != nil {
		return "", "", fmt.Errorf("prompt not found: %s", phase)
	}

	return string(content), SourceEmbedded, nil
}

// List returns information about all available prompts.
func (s *Service) List() ([]PromptInfo, error) {
	// Get embedded prompts
	entries, err := templates.Prompts.ReadDir("prompts")
	if err != nil {
		return nil, fmt.Errorf("read embedded prompts: %w", err)
	}

	// Build map of prompts
	prompts := make(map[string]*PromptInfo)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		phase := strings.TrimSuffix(entry.Name(), ".md")

		// Read content to extract variables
		content, _, err := s.Resolve(phase)
		if err != nil {
			slog.Debug("failed to resolve prompt for variable extraction", "phase", phase, "error", err)
		}
		vars := extractVariables(content)

		prompts[phase] = &PromptInfo{
			Phase:       phase,
			Source:      SourceEmbedded,
			HasOverride: false,
			Variables:   vars,
		}
	}

	// Check for project overrides
	projectDir := s.projectPromptsDir()
	if entries, err := os.ReadDir(projectDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			phase := strings.TrimSuffix(entry.Name(), ".md")

			// Read content to extract variables
			content, err := os.ReadFile(filepath.Join(projectDir, entry.Name()))
			vars := []string{}
			if err == nil {
				vars = extractVariables(string(content))
			}

			if info, exists := prompts[phase]; exists {
				info.HasOverride = true
				info.Source = SourceProject
				info.Variables = vars
			} else {
				prompts[phase] = &PromptInfo{
					Phase:       phase,
					Source:      SourceProject,
					HasOverride: true,
					Variables:   vars,
				}
			}
		}
	}

	// Convert to sorted slice
	result := make([]PromptInfo, 0, len(prompts))
	for _, info := range prompts {
		result = append(result, *info)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Phase < result[j].Phase
	})

	return result, nil
}

// Get returns the full prompt data for a phase.
func (s *Service) Get(phase string) (*Prompt, error) {
	content, source, err := s.Resolve(phase)
	if err != nil {
		return nil, err
	}

	return &Prompt{
		Phase:     phase,
		Content:   content,
		Source:    source,
		Variables: extractVariables(content),
	}, nil
}

// GetDefault returns the embedded default prompt for a phase.
func (s *Service) GetDefault(phase string) (*Prompt, error) {
	embeddedPath := fmt.Sprintf("prompts/%s.md", phase)
	content, err := templates.Prompts.ReadFile(embeddedPath)
	if err != nil {
		return nil, fmt.Errorf("embedded prompt not found: %s", phase)
	}

	return &Prompt{
		Phase:     phase,
		Content:   string(content),
		Source:    SourceEmbedded,
		Variables: extractVariables(string(content)),
	}, nil
}

// Save saves a project override for a prompt.
func (s *Service) Save(phase, content string) error {
	projectDir := s.projectPromptsDir()
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return fmt.Errorf("create prompts directory: %w", err)
	}

	path := s.projectPromptPath(phase)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("save prompt: %w", err)
	}

	return nil
}

// Delete removes a project override, falling back to embedded default.
func (s *Service) Delete(phase string) error {
	path := s.projectPromptPath(phase)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete prompt: %w", err)
	}
	return nil
}

// HasOverride checks if a project override exists for a phase.
func (s *Service) HasOverride(phase string) bool {
	path := s.projectPromptPath(phase)
	_, err := os.Stat(path)
	return err == nil
}

// GetVariableReference returns documentation for available template variables.
func GetVariableReference() map[string]string {
	return map[string]string{
		"{{TASK_ID}}":          "The task identifier (e.g., TASK-001)",
		"{{TASK_TITLE}}":       "The task title from user input",
		"{{TASK_DESCRIPTION}}": "The task description (if provided)",
		"{{WEIGHT}}":           "Task weight classification (trivial/small/medium/large/greenfield)",
		"{{PHASE}}":            "Current phase ID",
		"{{ITERATION}}":        "Current iteration number within the phase",
		"{{RESEARCH_CONTENT}}": "Output from the research phase (if applicable)",
		"{{SPEC_CONTENT}}":     "Output from the spec phase (if applicable)",
		"{{DESIGN_CONTENT}}":   "Output from the design phase (if applicable)",
		"{{RETRY_CONTEXT}}":    "Context from failed phase when retrying",
	}
}

// extractVariables finds all template variables in content.
var variableRegex = regexp.MustCompile(`\{\{[A-Z_]+\}\}`)

func extractVariables(content string) []string {
	matches := variableRegex.FindAllString(content, -1)

	// Deduplicate
	seen := make(map[string]bool)
	result := make([]string, 0, len(matches))
	for _, m := range matches {
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}

	sort.Strings(result)
	return result
}
