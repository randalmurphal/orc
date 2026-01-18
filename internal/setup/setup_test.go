package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
)

func TestGeneratePrompt(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some source files to affect size estimation
	_ = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)

	detection := &db.Detection{
		Language:    "go",
		Frameworks:  []string{"cobra", "viper"},
		BuildTools:  []string{"make"},
		HasTests:    true,
		TestCommand: "go test ./...",
		LintCommand: "golangci-lint run",
	}

	prompt, err := GeneratePrompt(tmpDir, detection)
	if err != nil {
		t.Fatalf("GeneratePrompt failed: %v", err)
	}

	// Check key sections are present (matching actual template content)
	checks := []string{
		"## Project Detection",
		"| **Language** | go |",
		"cobra",
		"go test ./...",
		"## Your Instructions",
	}

	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing %q", check)
		}
	}
}

func TestGeneratePrompt_NilDetection(t *testing.T) {
	tmpDir := t.TempDir()

	prompt, err := GeneratePrompt(tmpDir, nil)
	if err != nil {
		t.Fatalf("GeneratePrompt with nil detection failed: %v", err)
	}

	if !strings.Contains(prompt, "unknown") {
		t.Error("expected 'unknown' for nil detection")
	}
}

func TestGeneratePrompt_WithExistingClaudeMD(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .claude/CLAUDE.md
	claudeDir := filepath.Join(tmpDir, ".claude")
	_ = os.MkdirAll(claudeDir, 0755)
	_ = os.WriteFile(filepath.Join(claudeDir, "CLAUDE.md"), []byte("# Existing Content\n\nSome instructions."), 0644)

	prompt, err := GeneratePrompt(tmpDir, nil)
	if err != nil {
		t.Fatalf("GeneratePrompt failed: %v", err)
	}

	if !strings.Contains(prompt, "Existing CLAUDE.md") {
		t.Error("prompt should mention existing CLAUDE.md")
	}
	if !strings.Contains(prompt, "Some instructions") {
		t.Error("prompt should include existing content")
	}
}

func TestEstimateProjectSize(t *testing.T) {
	tests := []struct {
		name      string
		fileCount int
		want      string
	}{
		{"empty", 0, "small"},
		{"small", 20, "small"},
		{"medium", 100, "medium"},
		{"large", 1000, "large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			// Create source files with unique names
			for i := 0; i < tt.fileCount; i++ {
				fname := filepath.Join(tmpDir, "file"+fmt.Sprintf("%04d", i)+".go")
				_ = os.WriteFile(fname, []byte("package x"), 0644)
			}

			got := estimateProjectSize(tmpDir)
			if got != tt.want {
				t.Errorf("estimateProjectSize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewSpawner(t *testing.T) {
	spawner := NewSpawner(SpawnerOptions{
		WorkDir: "/tmp/test",
		Model:   "opus",
	})

	if spawner.opts.ClaudePath != "claude" {
		t.Errorf("ClaudePath = %q, want 'claude'", spawner.opts.ClaudePath)
	}
}

func TestValidator_Validate(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .orc/config.yaml
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte("version: 1\nprofile: auto"), 0644)

	validator := NewValidator(tmpDir)
	errors := validator.Validate()

	if len(errors) > 0 {
		t.Errorf("unexpected validation errors: %v", errors)
	}
}

func TestValidator_Validate_MissingConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty .orc directory (no config)
	_ = os.MkdirAll(filepath.Join(tmpDir, ".orc"), 0755)

	validator := NewValidator(tmpDir)
	errors := validator.Validate()

	if len(errors) == 0 {
		t.Error("expected validation error for missing config")
	}
}

func TestValidator_Validate_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty config.yaml
	orcDir := filepath.Join(tmpDir, ".orc")
	_ = os.MkdirAll(orcDir, 0755)
	_ = os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(""), 0644)

	validator := NewValidator(tmpDir)
	errors := validator.Validate()

	found := false
	for _, e := range errors {
		if strings.Contains(e, "empty") {
			found = true
			break
		}
	}

	if !found {
		t.Error("expected validation error about empty config")
	}
}

func TestValidator_ValidateSkillFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Valid skill file
	validSkill := `---
name: test-skill
description: A test skill
---

# Test Skill

Instructions here.
`
	skillPath := filepath.Join(tmpDir, "SKILL.md")
	_ = os.WriteFile(skillPath, []byte(validSkill), 0644)

	validator := NewValidator(tmpDir)
	errors := validator.ValidateSkillFile(skillPath)

	if len(errors) > 0 {
		t.Errorf("unexpected skill validation errors: %v", errors)
	}
}

func TestValidator_ValidateSkillFile_Invalid(t *testing.T) {
	tmpDir := t.TempDir()

	// Invalid skill file (no frontmatter)
	invalidSkill := `# Test Skill

No frontmatter here.
`
	skillPath := filepath.Join(tmpDir, "SKILL.md")
	_ = os.WriteFile(skillPath, []byte(invalidSkill), 0644)

	validator := NewValidator(tmpDir)
	errors := validator.ValidateSkillFile(skillPath)

	if len(errors) == 0 {
		t.Error("expected validation error for missing frontmatter")
	}
}
