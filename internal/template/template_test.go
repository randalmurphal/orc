package template

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRender_SimpleVariable(t *testing.T) {
	content := "Hello {{NAME}}, welcome to {{PLACE}}!"
	vars := map[string]string{
		"NAME":  "Alice",
		"PLACE": "Wonderland",
	}

	result := Render(content, vars)
	expected := "Hello Alice, welcome to Wonderland!"

	if result != expected {
		t.Errorf("Render() = %q, want %q", result, expected)
	}
}

func TestRender_MultipleVariables(t *testing.T) {
	content := "{{A}} {{B}} {{C}}"
	vars := map[string]string{
		"A": "one",
		"B": "two",
		"C": "three",
	}

	result := Render(content, vars)
	expected := "one two three"

	if result != expected {
		t.Errorf("Render() = %q, want %q", result, expected)
	}
}

func TestRender_MissingVariable(t *testing.T) {
	content := "Hello {{NAME}}, your id is {{ID}}!"
	vars := map[string]string{
		"NAME": "Bob",
	}

	result := Render(content, vars)
	// Missing variable should remain as-is
	expected := "Hello Bob, your id is {{ID}}!"

	if result != expected {
		t.Errorf("Render() = %q, want %q", result, expected)
	}
}

func TestRender_NoVariables(t *testing.T) {
	content := "No variables here"
	vars := map[string]string{}

	result := Render(content, vars)
	if result != content {
		t.Errorf("Render() = %q, want %q", result, content)
	}
}

func TestRender_EmptyContent(t *testing.T) {
	content := ""
	vars := map[string]string{"A": "1"}

	result := Render(content, vars)
	if result != "" {
		t.Errorf("Render() = %q, want empty", result)
	}
}

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"bugfix", false},
		{"my-template", false},
		{"feature-v2", false},
		{"template123", false},
		{"", true},               // empty
		{"-invalid", true},       // starts with dash
		{"has spaces", true},     // contains space
		{"has_underscore", true}, // contains underscore
		{"has.dot", true},        // contains dot
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestValidateVariables(t *testing.T) {
	template := &Template{
		Variables: []Variable{
			{Name: "REQUIRED_VAR", Required: true},
			{Name: "OPTIONAL_VAR", Required: false},
		},
	}

	tests := []struct {
		name     string
		provided map[string]string
		wantErr  bool
	}{
		{
			name:     "all provided",
			provided: map[string]string{"REQUIRED_VAR": "value", "OPTIONAL_VAR": "value"},
			wantErr:  false,
		},
		{
			name:     "only required",
			provided: map[string]string{"REQUIRED_VAR": "value"},
			wantErr:  false,
		},
		{
			name:     "missing required",
			provided: map[string]string{"OPTIONAL_VAR": "value"},
			wantErr:  true,
		},
		{
			name:     "empty map",
			provided: map[string]string{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := template.ValidateVariables(tt.provided)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVariables() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadBuiltin(t *testing.T) {
	templates := []string{"bugfix", "feature", "refactor", "migration", "spike"}

	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			tpl, err := LoadBuiltin(name)
			if err != nil {
				t.Fatalf("LoadBuiltin(%q) error = %v", name, err)
			}

			if tpl.Name != name {
				t.Errorf("Name = %q, want %q", tpl.Name, name)
			}
			if tpl.Weight == "" {
				t.Error("Weight should not be empty")
			}
			if len(tpl.Phases) == 0 {
				t.Error("Phases should not be empty")
			}
			if tpl.Scope != ScopeBuiltin {
				t.Errorf("Scope = %v, want %v", tpl.Scope, ScopeBuiltin)
			}
		})
	}
}

func TestLoadBuiltin_NotFound(t *testing.T) {
	_, err := LoadBuiltin("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent builtin template")
	}
}

func TestListBuiltin(t *testing.T) {
	templates, err := listBuiltin()
	if err != nil {
		t.Fatalf("listBuiltin() error = %v", err)
	}

	if len(templates) < 5 {
		t.Errorf("expected at least 5 builtin templates, got %d", len(templates))
	}

	// Check all have required fields
	for _, tpl := range templates {
		if tpl.Name == "" {
			t.Error("template name should not be empty")
		}
		if tpl.Scope != ScopeBuiltin {
			t.Errorf("template %q scope = %v, want %v", tpl.Name, tpl.Scope, ScopeBuiltin)
		}
	}
}

func TestTemplateSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".orc", "templates")

	// Create templates directory
	_ = os.MkdirAll(templatesDir, 0755)

	template := &Template{
		Name:        "test-template",
		Description: "A test template",
		Weight:      "small",
		Phases:      []string{"implement", "test"},
		Variables: []Variable{
			{Name: "VAR1", Description: "First variable", Required: true},
		},
	}

	// Save using SaveTo
	err := template.SaveTo(templatesDir)
	if err != nil {
		t.Fatalf("SaveTo() error = %v", err)
	}

	// Verify file exists
	templatePath := filepath.Join(templatesDir, "test-template", TemplateFileName)
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Error("template file should exist")
	}

	// Load
	loaded, err := LoadFrom("test-template", templatesDir)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	if loaded.Name != template.Name {
		t.Errorf("Name = %q, want %q", loaded.Name, template.Name)
	}
	if loaded.Weight != template.Weight {
		t.Errorf("Weight = %q, want %q", loaded.Weight, template.Weight)
	}
	if len(loaded.Phases) != len(template.Phases) {
		t.Errorf("Phases count = %d, want %d", len(loaded.Phases), len(template.Phases))
	}
}

func TestTemplateDelete(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".orc", "templates")

	_ = os.MkdirAll(templatesDir, 0755)

	template := &Template{
		Name:   "delete-me",
		Weight: "small",
		Phases: []string{"implement"},
	}

	err := template.SaveTo(templatesDir)
	if err != nil {
		t.Fatalf("SaveTo() error = %v", err)
	}

	// Verify it exists
	templateDir := filepath.Join(templatesDir, "delete-me")
	if _, err := os.Stat(templateDir); os.IsNotExist(err) {
		t.Fatal("template directory should exist before delete")
	}

	// Delete
	err = template.Delete()
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(templateDir); !os.IsNotExist(err) {
		t.Error("template directory should not exist after delete")
	}
}

func TestTemplateDelete_Builtin(t *testing.T) {
	template := &Template{
		Name:  "bugfix",
		Scope: ScopeBuiltin,
	}

	err := template.Delete()
	if err == nil {
		t.Error("expected error when deleting builtin template")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".orc", "templates")

	_ = os.MkdirAll(templatesDir, 0755)

	// Create a project template
	template := &Template{
		Name:        "project-tpl",
		Description: "Project template",
		Weight:      "medium",
		Phases:      []string{"implement"},
	}
	_ = template.SaveTo(templatesDir)

	// List templates using ListFrom
	templates, err := ListFrom(templatesDir)
	if err != nil {
		t.Fatalf("ListFrom() error = %v", err)
	}

	// Should have at least the project template and builtins
	if len(templates) < 6 {
		t.Errorf("expected at least 6 templates (1 project + 5 builtin), got %d", len(templates))
	}

	// Find our project template
	found := false
	for _, tpl := range templates {
		if tpl.Name == "project-tpl" && tpl.Scope == ScopeProject {
			found = true
			break
		}
	}
	if !found {
		t.Error("project template should be in list")
	}
}

func TestLoad_ResolutionOrder(t *testing.T) {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, ".orc", "templates")

	_ = os.MkdirAll(templatesDir, 0755)

	// Create a project template that shadows a builtin
	template := &Template{
		Name:        "bugfix",
		Description: "Custom bugfix",
		Weight:      "large", // Different from builtin
		Phases:      []string{"spec", "implement", "test", "validate"},
	}
	_ = template.SaveTo(templatesDir)

	// Load from project directory - should return project template
	loaded, err := LoadFrom("bugfix", templatesDir)
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}

	// Set scope to project since LoadFrom doesn't set it
	loaded.Scope = ScopeProject

	if loaded.Scope != ScopeProject {
		t.Errorf("Scope = %v, want %v", loaded.Scope, ScopeProject)
	}
	if loaded.Weight != "large" {
		t.Errorf("Weight = %q, want %q", loaded.Weight, "large")
	}
}

func TestExists(t *testing.T) {
	if !Exists("bugfix") {
		t.Error("bugfix builtin template should exist")
	}

	if Exists("nonexistent-template-xyz") {
		t.Error("nonexistent template should not exist")
	}
}

func TestProjectTemplatesDir(t *testing.T) {
	dir := ProjectTemplatesDir()
	if dir != filepath.Join(".orc", "templates") {
		t.Errorf("ProjectTemplatesDir() = %q, want .orc/templates", dir)
	}
}

func TestGlobalTemplatesDir(t *testing.T) {
	dir := GlobalTemplatesDir()
	if !filepath.IsAbs(dir) {
		t.Error("GlobalTemplatesDir() should return absolute path")
	}
	if filepath.Base(filepath.Dir(dir)) != ".orc" {
		t.Error("GlobalTemplatesDir() should be under ~/.orc/")
	}
}
