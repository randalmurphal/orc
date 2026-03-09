package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWorkflowDefaults_GetDefaultWorkflow tests retrieval of default workflows.
func TestWorkflowDefaults_GetDefaultWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		config   WorkflowDefaults
		category string
		want     string
	}{
		{
			name: "feature has custom default",
			config: WorkflowDefaults{
				Feature: "feature-complete",
				Bug:     "hotfix",
				Default: "crossmodel-standard",
			},
			category: "feature",
			want:     "feature-complete",
		},
		{
			name: "bug has custom default",
			config: WorkflowDefaults{
				Feature: "feature-complete",
				Bug:     "hotfix",
				Default: "crossmodel-standard",
			},
			category: "bug",
			want:     "hotfix",
		},
		{
			name: "unknown category uses default",
			config: WorkflowDefaults{
				Feature: "feature-complete",
				Bug:     "hotfix",
				Default: "crossmodel-standard",
			},
			category: "docs",
			want:     "crossmodel-standard",
		},
		{
			name: "empty category uses default",
			config: WorkflowDefaults{
				Default: "crossmodel-standard",
			},
			category: "",
			want:     "crossmodel-standard",
		},
		{
			name: "missing default and category returns empty",
			config: WorkflowDefaults{
				Feature: "feature-complete",
			},
			category: "bug",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetDefaultWorkflow(tt.category)
			if got != tt.want {
				t.Errorf("GetDefaultWorkflow(%q) = %q, want %q", tt.category, got, tt.want)
			}
		})
	}
}

// TestWorkflowDefaults_SetDefaultWorkflow tests setting default workflows.
func TestWorkflowDefaults_SetDefaultWorkflow(t *testing.T) {
	config := WorkflowDefaults{
		Default: "crossmodel-standard",
	}

	// Set feature default
	config.SetDefaultWorkflow("feature", "feature-workflow")
	if config.Feature != "feature-workflow" {
		t.Errorf("Feature = %q, want %q", config.Feature, "feature-workflow")
	}

	// Set bug default
	config.SetDefaultWorkflow("bug", "bug-workflow")
	if config.Bug != "bug-workflow" {
		t.Errorf("Bug = %q, want %q", config.Bug, "bug-workflow")
	}

	// Set default
	config.SetDefaultWorkflow("", "new-default")
	if config.Default != "new-default" {
		t.Errorf("Default = %q, want %q", config.Default, "new-default")
	}
}

// TestWorkflowDefaults_ListCategories tests listing available categories.
func TestWorkflowDefaults_ListCategories(t *testing.T) {
	config := WorkflowDefaults{
		Feature:  "feature-workflow",
		Bug:      "bug-workflow",
		Refactor: "refactor-workflow",
		Default:  "standard-workflow",
	}

	categories := config.ListCategories()

	expected := []string{"feature", "bug", "refactor"}
	if len(categories) != len(expected) {
		t.Errorf("ListCategories() returned %d categories, want %d", len(categories), len(expected))
	}

	for _, exp := range expected {
		found := false
		for _, cat := range categories {
			if cat == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ListCategories() missing category %q", exp)
		}
	}
}

// TestConfig_LoadWorkflowDefaults tests loading workflow defaults from config file.
func TestConfig_LoadWorkflowDefaults(t *testing.T) {
	// Create temporary config file with workflow defaults
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `version: 1
workflow_defaults:
  feature: "feature-complete"
  bug: "hotfix"
  refactor: "refactor-safe"
  default: "crossmodel-standard"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Load config
	config, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify workflow defaults
	if config.WorkflowDefaults.Feature != "feature-complete" {
		t.Errorf("Feature = %q, want %q", config.WorkflowDefaults.Feature, "feature-complete")
	}
	if config.WorkflowDefaults.Bug != "hotfix" {
		t.Errorf("Bug = %q, want %q", config.WorkflowDefaults.Bug, "hotfix")
	}
	if config.WorkflowDefaults.Refactor != "refactor-safe" {
		t.Errorf("Refactor = %q, want %q", config.WorkflowDefaults.Refactor, "refactor-safe")
	}
	if config.WorkflowDefaults.Default != "crossmodel-standard" {
		t.Errorf("Default = %q, want %q", config.WorkflowDefaults.Default, "crossmodel-standard")
	}
}

// TestConfig_SaveWorkflowDefaults tests saving workflow defaults to config file.
func TestConfig_SaveWorkflowDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Create config with workflow defaults
	config := Default()
	config.WorkflowDefaults = WorkflowDefaults{
		Feature: "feature-advanced",
		Bug:     "bug-urgent",
		Default: "standard-impl",
	}

	// Save to file
	if err := config.SaveTo(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load back and verify
	loaded, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if loaded.WorkflowDefaults.Feature != "feature-advanced" {
		t.Errorf("Feature = %q, want %q", loaded.WorkflowDefaults.Feature, "feature-advanced")
	}
	if loaded.WorkflowDefaults.Bug != "bug-urgent" {
		t.Errorf("Bug = %q, want %q", loaded.WorkflowDefaults.Bug, "bug-urgent")
	}
	if loaded.WorkflowDefaults.Default != "standard-impl" {
		t.Errorf("Default = %q, want %q", loaded.WorkflowDefaults.Default, "standard-impl")
	}
}

// TestWorkflowDefaults_ValidateWorkflowIDs tests workflow ID validation.
func TestWorkflowDefaults_ValidateWorkflowIDs(t *testing.T) {
	validWorkflows := []string{"implement-small", "implement-medium", "feature-complete", "hotfix"}

	tests := []struct {
		name     string
		config   WorkflowDefaults
		wantErr  bool
		errorMsg string
	}{
		{
			name: "all valid workflows",
			config: WorkflowDefaults{
				Feature: "feature-complete",
				Bug:     "hotfix",
				Default: "implement-small",
			},
			wantErr: false,
		},
		{
			name: "invalid feature workflow",
			config: WorkflowDefaults{
				Feature: "nonexistent-workflow",
				Default: "implement-small",
			},
			wantErr:  true,
			errorMsg: "invalid workflow ID for feature: nonexistent-workflow",
		},
		{
			name: "invalid default workflow",
			config: WorkflowDefaults{
				Feature: "feature-complete",
				Default: "invalid-workflow",
			},
			wantErr:  true,
			errorMsg: "invalid default workflow ID: invalid-workflow",
		},
		{
			name:    "empty config is valid",
			config:  WorkflowDefaults{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateWorkflowIDs(validWorkflows)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkflowIDs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errorMsg != "" && err.Error() != tt.errorMsg {
				t.Errorf("ValidateWorkflowIDs() error = %q, want %q", err.Error(), tt.errorMsg)
			}
		})
	}
}

// TestWorkflowDefaults_BackwardCompatibility tests backward compatibility with single workflow field.
func TestWorkflowDefaults_BackwardCompatibility(t *testing.T) {
	tmpDir := t.TempDir()

	// Test loading old config format with single workflow field
	oldConfigPath := filepath.Join(tmpDir, "old-config.yaml")
	oldConfigContent := `version: 1
workflow: "legacy-workflow"
`

	if err := os.WriteFile(oldConfigPath, []byte(oldConfigContent), 0644); err != nil {
		t.Fatalf("Failed to write old config: %v", err)
	}

	config, err := LoadFile(oldConfigPath)
	if err != nil {
		t.Fatalf("Failed to load old config: %v", err)
	}

	// Should still have the workflow field
	if config.Workflow != "legacy-workflow" {
		t.Errorf("Workflow = %q, want %q", config.Workflow, "legacy-workflow")
	}

	// Built-in defaults still apply even when loading the old single-workflow format.
	if config.WorkflowDefaults.Default != "crossmodel-standard" {
		t.Errorf("WorkflowDefaults.Default = %q, want %q", config.WorkflowDefaults.Default, "crossmodel-standard")
	}
}

// TestWorkflowDefaults_ResolutionPriority tests workflow resolution priority.
func TestWorkflowDefaults_ResolutionPriority(t *testing.T) {
	config := Config{
		Workflow: "legacy-single-workflow",
		WorkflowDefaults: WorkflowDefaults{
			Feature: "feature-workflow",
			Default: "default-workflow",
		},
	}

	// Test resolution priority
	tests := []struct {
		name           string
		explicitWF     string
		category       string
		expectedWF     string
		expectedSource string
	}{
		{
			name:           "explicit workflow wins",
			explicitWF:     "explicit-workflow",
			category:       "feature",
			expectedWF:     "explicit-workflow",
			expectedSource: "explicit",
		},
		{
			name:           "category default used when no explicit",
			explicitWF:     "",
			category:       "feature",
			expectedWF:     "feature-workflow",
			expectedSource: "category_default",
		},
		{
			name:           "general default for unknown category",
			explicitWF:     "",
			category:       "docs",
			expectedWF:     "default-workflow",
			expectedSource: "general_default",
		},
		{
			name:           "legacy workflow when no defaults",
			explicitWF:     "",
			category:       "chore",
			expectedWF:     "legacy-single-workflow",
			expectedSource: "legacy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate clearing defaults for legacy test
			if tt.expectedSource == "legacy" {
				config.WorkflowDefaults.Default = ""
			} else if tt.name == "general default for unknown category" {
				config.WorkflowDefaults.Default = "default-workflow"
			}

			workflow, source := config.ResolveWorkflow(tt.explicitWF, tt.category)
			if workflow != tt.expectedWF {
				t.Errorf("ResolveWorkflow() workflow = %q, want %q", workflow, tt.expectedWF)
			}
			if source != tt.expectedSource {
				t.Errorf("ResolveWorkflow() source = %q, want %q", source, tt.expectedSource)
			}
		})
	}
}
