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
	SourcePersonalGlobal Source = "personal_global" // ~/.orc/prompts/
	SourceProjectLocal   Source = "project_local"   // ~/.orc/projects/<id>/prompts/
	SourceProject        Source = "project"         // .orc/prompts/
	SourceEmbedded       Source = "embedded"        // Embedded in binary
	SourceInline         Source = "inline"          // Inline in plan YAML (handled by executor)
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
	orcDir   string
	resolver *Resolver
}

// NewService creates a new prompt service.
func NewService(orcDir string) *Service {
	return &Service{
		orcDir:   orcDir,
		resolver: NewResolverFromOrcDir(orcDir),
	}
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

// Resolve returns the prompt content for a phase, using the full resolution chain:
// 1. Personal (~/.orc/prompts/)
// 2. Local (~/.orc/projects/<id>/prompts/)
// 3. Project (.orc/prompts/)
// 4. Embedded (built-in)
//
// Supports prompt inheritance via frontmatter (extends, prepend, append).
// Returns content, source, and any error.
func (s *Service) Resolve(phase string) (string, Source, error) {
	resolved, err := s.resolver.Resolve(phase)
	if err != nil {
		return "", "", err
	}
	return resolved.Content, resolved.Source, nil
}

// List returns information about all available prompts.
func (s *Service) List() ([]PromptInfo, error) {
	// Get embedded prompts
	entries, err := templates.Prompts.ReadDir("prompts")
	if err != nil {
		return nil, fmt.Errorf("read embedded prompts: %w", err)
	}

	// Build map of prompts from embedded
	prompts := make(map[string]*PromptInfo)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		phase := strings.TrimSuffix(entry.Name(), ".md")

		// Use resolver to get the actual content and source
		resolved, err := s.resolver.Resolve(phase)
		if err != nil {
			slog.Debug("failed to resolve prompt", "phase", phase, "error", err)
			continue
		}

		prompts[phase] = &PromptInfo{
			Phase:       phase,
			Source:      resolved.Source,
			HasOverride: resolved.Source != SourceEmbedded,
			Variables:   extractVariables(resolved.Content),
		}
	}

	// Also scan all override directories for custom prompts not in embedded
	overrideDirs := []struct {
		dir    string
		source Source
	}{
		{s.resolver.personalDir, SourcePersonalGlobal},
		{s.resolver.localDir, SourceProjectLocal},
		{s.resolver.projectDir, SourceProject},
	}

	for _, od := range overrideDirs {
		if od.dir == "" {
			continue
		}
		dirEntries, err := os.ReadDir(od.dir)
		if err != nil {
			continue // Directory doesn't exist, skip
		}
		for _, entry := range dirEntries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			phase := strings.TrimSuffix(entry.Name(), ".md")
			if _, exists := prompts[phase]; exists {
				continue // Already have this prompt from higher priority source
			}

			// Resolve to get actual source (may be higher priority than this dir)
			resolved, err := s.resolver.Resolve(phase)
			if err != nil {
				slog.Debug("failed to resolve custom prompt", "phase", phase, "error", err)
				continue
			}

			prompts[phase] = &PromptInfo{
				Phase:       phase,
				Source:      resolved.Source,
				HasOverride: true, // Custom prompt = always an override
				Variables:   extractVariables(resolved.Content),
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

// HasOverride checks if an override exists for a phase in any source.
// Returns true if the prompt is overridden in personal, local, or project directories.
func (s *Service) HasOverride(phase string) bool {
	resolved, err := s.resolver.Resolve(phase)
	if err != nil {
		return false
	}
	return resolved.Source != SourceEmbedded
}

// GetVariableReference returns documentation for available template variables.
// NOTE: Phase output variables (e.g., SPEC_CONTENT, TDD_TESTS_CONTENT) are dynamic
// and derived from each phase template's output_var_name field. The OUTPUT_<PHASE_ID>
// pattern is always available as a generic accessor for any phase's output.
func GetVariableReference() map[string]string {
	return map[string]string{
		// Task context
		"{{TASK_ID}}":          "The task identifier (e.g., TASK-001)",
		"{{TASK_TITLE}}":       "The task title from user input",
		"{{TASK_DESCRIPTION}}": "The task description (if provided)",
		"{{TASK_CATEGORY}}":    "Task category (feature, bug, refactor, chore, docs, test)",
		"{{WEIGHT}}":           "Task weight classification (trivial/small/medium/large)",

		// Execution context
		"{{PHASE}}":     "Current phase ID",
		"{{ITERATION}}": "Current iteration number within the phase",

		// Retry context (only populated when retrying a failed phase)
		"{{RETRY_ATTEMPT}}":    "Retry attempt number (e.g., 2, 3)",
		"{{RETRY_FROM_PHASE}}": "Phase that triggered the retry (e.g., review)",
		"{{RETRY_REASON}}":     "Reason the retry was triggered",

		// Git context
		"{{WORKTREE_PATH}}": "Absolute path to the isolated worktree directory",
		"{{PROJECT_ROOT}}":  "Project root directory",
		"{{TASK_BRANCH}}":   "The git branch for this task (e.g., orc/TASK-001)",
		"{{TARGET_BRANCH}}": "The target branch for merging (e.g., main)",

		// Project detection
		"{{LANGUAGE}}":     "Primary programming language",
		"{{HAS_FRONTEND}}": "Whether project has a frontend",
		"{{HAS_TESTS}}":    "Whether project has existing tests",
		"{{FRAMEWORKS}}":   "Detected frameworks",

		// Commands
		"{{TEST_COMMAND}}":  "Project test command",
		"{{LINT_COMMAND}}":  "Project lint command",
		"{{BUILD_COMMAND}}": "Project build command",

		// Phase outputs (dynamic — derived from phase template output_var_name)
		"{{OUTPUT_<PHASE_ID>}}": "Generic phase output (e.g., {{OUTPUT_SPEC}}, {{OUTPUT_IMPLEMENT}})",
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
