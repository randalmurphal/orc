// Package template provides task template management for orc.
package template

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

const (
	// TemplatesDir is the templates directory name
	TemplatesDir = "templates"
	// TemplateFileName is the template definition file name
	TemplateFileName = "template.yaml"
	// GlobalOrcDir is the global orc directory
	GlobalOrcDir = ".orc"
)

//go:embed builtin/*.yaml
var builtinFS embed.FS

// Scope defines where a template is stored.
type Scope string

const (
	// ScopeProject indicates a project-local template
	ScopeProject Scope = "project"
	// ScopeGlobal indicates a global (user-level) template
	ScopeGlobal Scope = "global"
	// ScopeBuiltin indicates a built-in template
	ScopeBuiltin Scope = "builtin"
)

// Variable represents a template variable.
type Variable struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required" json:"required"`
	Default     string `yaml:"default,omitempty" json:"default,omitempty"`
}

// Defaults contains default values for task creation.
type Defaults struct {
	BranchPrefix string `yaml:"branch_prefix,omitempty" json:"branch_prefix,omitempty"`
}

// Template represents a task template.
type Template struct {
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Version     int               `yaml:"version" json:"version"`
	Weight      string            `yaml:"weight" json:"weight"`
	Phases      []string          `yaml:"phases" json:"phases"`
	Variables   []Variable        `yaml:"variables,omitempty" json:"variables,omitempty"`
	Prompts     map[string]string `yaml:"prompts,omitempty" json:"prompts,omitempty"`
	Defaults    *Defaults         `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	CreatedFrom string            `yaml:"created_from,omitempty" json:"created_from,omitempty"`
	CreatedAt   time.Time         `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	Author      string            `yaml:"author,omitempty" json:"author,omitempty"`
	Scope       Scope             `yaml:"-" json:"scope"`
	Path        string            `yaml:"-" json:"-"`
}

// TemplateInfo contains summary information about a template.
type TemplateInfo struct {
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Weight      string     `json:"weight"`
	Phases      []string   `json:"phases"`
	Scope       Scope      `json:"scope"`
	Variables   []Variable `json:"variables,omitempty"`
}

// Load loads a template by name, searching in order:
// 1. Project templates (.orc/templates/)
// 2. Global templates (~/.orc/templates/)
// 3. Built-in templates
func Load(name string) (*Template, error) {
	// Try project templates first
	t, err := LoadFrom(name, ProjectTemplatesDir())
	if err == nil {
		t.Scope = ScopeProject
		return t, nil
	}

	// Try global templates
	t, err = LoadFrom(name, GlobalTemplatesDir())
	if err == nil {
		t.Scope = ScopeGlobal
		return t, nil
	}

	// Try built-in templates
	t, err = LoadBuiltin(name)
	if err == nil {
		t.Scope = ScopeBuiltin
		return t, nil
	}

	return nil, fmt.Errorf("template %q not found", name)
}

// LoadFrom loads a template from a specific base directory.
func LoadFrom(name, baseDir string) (*Template, error) {
	templateDir := filepath.Join(baseDir, name)
	templatePath := filepath.Join(templateDir, TemplateFileName)

	data, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("read template %s: %w", name, err)
	}

	var t Template
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}

	t.Name = name
	t.Path = templateDir
	return &t, nil
}

// LoadBuiltin loads a built-in template.
func LoadBuiltin(name string) (*Template, error) {
	path := fmt.Sprintf("builtin/%s.yaml", name)
	data, err := builtinFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("builtin template %s not found", name)
	}

	var t Template
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse builtin template %s: %w", name, err)
	}

	t.Name = name
	t.Scope = ScopeBuiltin
	return &t, nil
}

// Save saves the template to disk.
func (t *Template) Save(global bool) error {
	var baseDir string
	if global {
		baseDir = GlobalTemplatesDir()
		t.Scope = ScopeGlobal
	} else {
		baseDir = ProjectTemplatesDir()
		t.Scope = ScopeProject
	}

	templateDir := filepath.Join(baseDir, t.Name)
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("create template directory: %w", err)
	}

	t.Path = templateDir
	t.Version = 1
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}

	data, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal template: %w", err)
	}

	templatePath := filepath.Join(templateDir, TemplateFileName)
	if err := os.WriteFile(templatePath, data, 0644); err != nil {
		return fmt.Errorf("write template: %w", err)
	}

	// Save custom prompts if any
	for phase, promptFile := range t.Prompts {
		// If it's just a filename, we need the content from somewhere
		// This is typically set when creating from a task
		if promptFile != "" && !strings.HasPrefix(promptFile, "/") {
			// The prompt content should be stored separately
			// For now, just ensure the file reference is correct
			t.Prompts[phase] = promptFile
		}
	}

	return nil
}

// SaveTo saves the template to a specific base directory.
func (t *Template) SaveTo(baseDir string) error {
	templateDir := filepath.Join(baseDir, t.Name)
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("create template directory: %w", err)
	}

	t.Path = templateDir
	t.Version = 1
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}

	data, err := yaml.Marshal(t)
	if err != nil {
		return fmt.Errorf("marshal template: %w", err)
	}

	templatePath := filepath.Join(templateDir, TemplateFileName)
	if err := os.WriteFile(templatePath, data, 0644); err != nil {
		return fmt.Errorf("write template: %w", err)
	}

	return nil
}

// Delete removes the template from disk.
func (t *Template) Delete() error {
	if t.Scope == ScopeBuiltin {
		return fmt.Errorf("cannot delete built-in template %q", t.Name)
	}

	if t.Path == "" {
		return fmt.Errorf("template path not set")
	}

	return os.RemoveAll(t.Path)
}

// List returns all available templates.
func List() ([]TemplateInfo, error) {
	var templates []TemplateInfo

	// Project templates
	projectTemplates, err := listFromDir(ProjectTemplatesDir(), ScopeProject)
	if err == nil {
		templates = append(templates, projectTemplates...)
	}

	// Global templates
	globalTemplates, err := listFromDir(GlobalTemplatesDir(), ScopeGlobal)
	if err == nil {
		templates = append(templates, globalTemplates...)
	}

	// Built-in templates
	builtinTemplates, err := listBuiltin()
	if err == nil {
		templates = append(templates, builtinTemplates...)
	}

	return templates, nil
}

// ListFrom returns templates from a specific base directory plus built-ins.
func ListFrom(projectTemplatesDir string) ([]TemplateInfo, error) {
	var templates []TemplateInfo

	// Project templates from specified directory
	projectTemplates, err := listFromDir(projectTemplatesDir, ScopeProject)
	if err == nil {
		templates = append(templates, projectTemplates...)
	}

	// Built-in templates
	builtinTemplates, err := listBuiltin()
	if err == nil {
		templates = append(templates, builtinTemplates...)
	}

	return templates, nil
}

// listFromDir lists templates in a directory.
func listFromDir(baseDir string, scope Scope) ([]TemplateInfo, error) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var templates []TemplateInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		t, err := LoadFrom(entry.Name(), baseDir)
		if err != nil {
			continue
		}

		templates = append(templates, TemplateInfo{
			Name:        t.Name,
			Description: t.Description,
			Weight:      t.Weight,
			Phases:      t.Phases,
			Scope:       scope,
			Variables:   t.Variables,
		})
	}

	return templates, nil
}

// listBuiltin lists built-in templates.
func listBuiltin() ([]TemplateInfo, error) {
	entries, err := builtinFS.ReadDir("builtin")
	if err != nil {
		return nil, err
	}

	var templates []TemplateInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml")
		t, err := LoadBuiltin(name)
		if err != nil {
			continue
		}

		templates = append(templates, TemplateInfo{
			Name:        t.Name,
			Description: t.Description,
			Weight:      t.Weight,
			Phases:      t.Phases,
			Scope:       ScopeBuiltin,
			Variables:   t.Variables,
		})
	}

	return templates, nil
}

// SaveFromTask creates a template from a completed task.
func SaveFromTask(taskID, name, description string, global bool, backend storage.Backend) (*Template, error) {
	if backend == nil {
		return nil, fmt.Errorf("backend is required")
	}

	// Load the task
	t, err := backend.LoadTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}

	// Derive phases from task weight
	phases := phasesForWeight(t.Weight)

	template := &Template{
		Name:        name,
		Description: description,
		Weight:      task.WeightFromProto(t.Weight),
		Phases:      phases,
		CreatedFrom: taskID,
		CreatedAt:   time.Now(),
	}

	if err := template.Save(global); err != nil {
		return nil, err
	}

	return template, nil
}

// Render substitutes variables in content.
func Render(content string, vars map[string]string) string {
	for key, value := range vars {
		placeholder := fmt.Sprintf("{{%s}}", key)
		content = strings.ReplaceAll(content, placeholder, value)
	}
	return content
}

// ValidateName checks if a template name is valid.
// Names must be alphanumeric with dashes only.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("template name cannot be empty")
	}

	validName := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)
	if !validName.MatchString(name) {
		return fmt.Errorf("template name must be alphanumeric with dashes (got %q)", name)
	}

	return nil
}

// ValidateVariables checks that all required variables are provided.
func (t *Template) ValidateVariables(provided map[string]string) error {
	for _, v := range t.Variables {
		if v.Required {
			if _, ok := provided[v.Name]; !ok {
				return fmt.Errorf("required variable %q not provided", v.Name)
			}
		}
	}
	return nil
}

// GetPromptContent returns the content of a custom prompt for a phase.
func (t *Template) GetPromptContent(phase string) (string, error) {
	promptFile, ok := t.Prompts[phase]
	if !ok || promptFile == "" {
		return "", nil // No custom prompt
	}

	if t.Scope == ScopeBuiltin {
		// Built-in prompts would be embedded
		return "", nil
	}

	promptPath := filepath.Join(t.Path, promptFile)
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("read prompt %s: %w", phase, err)
	}

	return string(data), nil
}

// ProjectTemplatesDir returns the project templates directory.
func ProjectTemplatesDir() string {
	return filepath.Join(".orc", TemplatesDir)
}

// GlobalTemplatesDir returns the global templates directory.
func GlobalTemplatesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, GlobalOrcDir, TemplatesDir)
}

// Exists checks if a template exists by name.
func Exists(name string) bool {
	_, err := Load(name)
	return err == nil
}

// phasesForWeight returns the phase IDs for a given task weight.
func phasesForWeight(weight orcv1.TaskWeight) []string {
	switch weight {
	case orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL:
		return []string{"tiny_spec", "implement"}
	case orcv1.TaskWeight_TASK_WEIGHT_SMALL:
		return []string{"tiny_spec", "implement", "review"}
	case orcv1.TaskWeight_TASK_WEIGHT_MEDIUM:
		return []string{"spec", "tdd_write", "implement", "review", "docs"}
	case orcv1.TaskWeight_TASK_WEIGHT_LARGE:
		return []string{"spec", "tdd_write", "breakdown", "implement", "review", "docs"}
	default:
		return []string{"spec", "implement", "review"}
	}
}
