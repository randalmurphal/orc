package prompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewService(t *testing.T) {
	svc := NewService(".orc")
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.orcDir != ".orc" {
		t.Errorf("expected orcDir '.orc', got %q", svc.orcDir)
	}
}

func TestDefaultService(t *testing.T) {
	svc := DefaultService()
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.orcDir != ".orc" {
		t.Errorf("expected orcDir '.orc', got %q", svc.orcDir)
	}
}

func TestResolve_EmbeddedPrompt(t *testing.T) {
	// Use temp dir to avoid project overrides
	tmpDir := t.TempDir()
	svc := NewService(filepath.Join(tmpDir, ".orc"))

	// Resolve embedded prompt
	content, source, err := svc.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if source != SourceEmbedded {
		t.Errorf("expected source 'embedded', got %q", source)
	}

	if content == "" {
		t.Error("expected non-empty content")
	}
}

func TestResolve_ProjectOverride(t *testing.T) {
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, ".orc")
	promptsDir := filepath.Join(orcDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write override
	overrideContent := "Custom implement prompt"
	if err := os.WriteFile(filepath.Join(promptsDir, "implement.md"), []byte(overrideContent), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(orcDir)
	content, source, err := svc.Resolve("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if source != SourceProject {
		t.Errorf("expected source 'project', got %q", source)
	}

	if content != overrideContent {
		t.Errorf("expected override content, got %q", content)
	}
}

func TestResolve_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(filepath.Join(tmpDir, ".orc"))

	_, _, err := svc.Resolve("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent prompt")
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(filepath.Join(tmpDir, ".orc"))

	prompts, err := svc.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(prompts) == 0 {
		t.Error("expected at least one prompt")
	}

	// Check that implement exists
	found := false
	for _, p := range prompts {
		if p.Phase == "implement" {
			found = true
			if p.Source != SourceEmbedded {
				t.Errorf("expected implement source 'embedded', got %q", p.Source)
			}
			break
		}
	}
	if !found {
		t.Error("expected to find 'implement' prompt")
	}
}

func TestList_WithOverride(t *testing.T) {
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, ".orc")
	promptsDir := filepath.Join(orcDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write override
	if err := os.WriteFile(filepath.Join(promptsDir, "implement.md"), []byte("override"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(orcDir)
	prompts, err := svc.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find implement and verify override
	for _, p := range prompts {
		if p.Phase == "implement" {
			if !p.HasOverride {
				t.Error("expected implement to have override")
			}
			if p.Source != SourceProject {
				t.Errorf("expected source 'project', got %q", p.Source)
			}
			break
		}
	}
}

func TestGet(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(filepath.Join(tmpDir, ".orc"))

	p, err := svc.Get("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Phase != "implement" {
		t.Errorf("expected phase 'implement', got %q", p.Phase)
	}

	if p.Content == "" {
		t.Error("expected non-empty content")
	}

	if p.Source != SourceEmbedded {
		t.Errorf("expected source 'embedded', got %q", p.Source)
	}
}

func TestGet_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(filepath.Join(tmpDir, ".orc"))

	_, err := svc.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent prompt")
	}
}

func TestGetDefault(t *testing.T) {
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, ".orc")
	promptsDir := filepath.Join(orcDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write override
	if err := os.WriteFile(filepath.Join(promptsDir, "implement.md"), []byte("override"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(orcDir)

	// GetDefault should return embedded, not override
	p, err := svc.GetDefault("implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.Source != SourceEmbedded {
		t.Errorf("expected source 'embedded', got %q", p.Source)
	}

	if p.Content == "override" {
		t.Error("GetDefault should not return override content")
	}
}

func TestGetDefault_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(filepath.Join(tmpDir, ".orc"))

	_, err := svc.GetDefault("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent prompt")
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, ".orc")
	svc := NewService(orcDir)

	content := "Custom prompt content"
	if err := svc.Save("custom", content); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists
	path := filepath.Join(orcDir, "prompts", "custom.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	if string(data) != content {
		t.Errorf("expected content %q, got %q", content, string(data))
	}
}

func TestDelete(t *testing.T) {
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, ".orc")
	promptsDir := filepath.Join(orcDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write override
	path := filepath.Join(promptsDir, "custom.md")
	if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewService(orcDir)
	if err := svc.Delete("custom"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file deleted
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted")
	}
}

func TestDelete_NonexistentOK(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(filepath.Join(tmpDir, ".orc"))

	// Delete nonexistent should not error
	if err := svc.Delete("nonexistent"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHasOverride(t *testing.T) {
	tmpDir := t.TempDir()
	orcDir := filepath.Join(tmpDir, ".orc")
	promptsDir := filepath.Join(orcDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatal(err)
	}

	svc := NewService(orcDir)

	// No override initially
	if svc.HasOverride("test") {
		t.Error("expected no override initially")
	}

	// Create override
	if err := os.WriteFile(filepath.Join(promptsDir, "test.md"), []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Now should have override
	if !svc.HasOverride("test") {
		t.Error("expected override to exist")
	}
}

func TestGetVariableReference(t *testing.T) {
	vars := GetVariableReference()
	if len(vars) == 0 {
		t.Error("expected at least one variable")
	}

	// Check required variables
	required := []string{"{{TASK_ID}}", "{{TASK_TITLE}}", "{{PHASE}}"}
	for _, v := range required {
		if _, ok := vars[v]; !ok {
			t.Errorf("expected variable %s to exist", v)
		}
	}
}

func TestFinalizePromptExists(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewService(filepath.Join(tmpDir, ".orc"))

	// Verify finalize prompt exists
	p, err := svc.Get("finalize")
	if err != nil {
		t.Fatalf("finalize prompt should exist: %v", err)
	}

	if p.Source != SourceEmbedded {
		t.Errorf("expected source 'embedded', got %q", p.Source)
	}

	// Verify required sections exist in the prompt
	requiredSections := []string{
		"Finalize Phase",        // Title
		"Sync",                  // Sync instructions
		"Conflict Resolution",   // Conflict resolution rules
		"NEVER remove features", // Key rule
		"intentions",            // Merge intentions, not text
		"Run Full Test Suite",   // Run tests after resolution
		"Risk Assessment",       // Risk assessment section
		"Merge Decision",        // Merge decision output format
	}

	for _, section := range requiredSections {
		if !strings.Contains(p.Content, section) {
			t.Errorf("finalize prompt missing required section: %q", section)
		}
	}

	// Verify required variables are present
	requiredVars := []string{
		"{{TASK_ID}}",
		"{{TASK_TITLE}}",
		"{{TASK_BRANCH}}",
		"{{TARGET_BRANCH}}",
	}

	for _, v := range requiredVars {
		if !strings.Contains(p.Content, v) {
			t.Errorf("finalize prompt missing required variable: %s", v)
		}
	}
}

func TestExtractVariables(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name:     "no variables",
			content:  "Just plain text",
			expected: []string{},
		},
		{
			name:     "single variable",
			content:  "Task: {{TASK_ID}}",
			expected: []string{"{{TASK_ID}}"},
		},
		{
			name:     "multiple variables",
			content:  "{{TASK_ID}} - {{TASK_TITLE}}",
			expected: []string{"{{TASK_ID}}", "{{TASK_TITLE}}"},
		},
		{
			name:     "duplicate variables",
			content:  "{{TASK_ID}} and {{TASK_ID}} again",
			expected: []string{"{{TASK_ID}}"},
		},
		{
			name:     "sorted output",
			content:  "{{PHASE}} then {{ITERATION}} then {{TASK_ID}}",
			expected: []string{"{{ITERATION}}", "{{PHASE}}", "{{TASK_ID}}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVariables(tt.content)

			// Check length
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d variables, got %d: %v", len(tt.expected), len(result), result)
				return
			}

			// Check contents
			for i, v := range tt.expected {
				if result[i] != v {
					t.Errorf("expected variable[%d] = %q, got %q", i, v, result[i])
				}
			}
		})
	}
}
