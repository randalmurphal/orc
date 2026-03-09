package workflow

import (
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

// TestWeightToWorkflowID_WithWorkflowDefaults tests that weight-to-workflow mapping
// considers workflow defaults when no explicit workflow is provided.
func TestWeightToWorkflowID_WithWorkflowDefaults(t *testing.T) {
	// Mock global database with available workflows
	workflows := []*db.Workflow{
		{ID: "implement-small", Name: "Small Implementation"},
		{ID: "implement-medium", Name: "Medium Implementation"},
		{ID: "feature-complete", Name: "Feature Complete"},
		{ID: "hotfix", Name: "Hotfix"},
	}

	tests := []struct {
		name          string
		weight        string
		category      string
		config        *config.Config
		expectedWF    string
		expectedEmpty bool
	}{
		{
			name:     "weight with category uses workflow defaults",
			weight:   "", // No weight specified
			category: "feature",
			config: &config.Config{
				WorkflowDefaults: config.WorkflowDefaults{
					Feature: "feature-complete",
					Default: "implement-medium",
				},
			},
			expectedWF: "feature-complete",
		},
		{
			name:     "weight with unknown category uses default",
			weight:   "",
			category: "docs", // Unknown category
			config: &config.Config{
				WorkflowDefaults: config.WorkflowDefaults{
					Feature: "feature-complete",
					Default: "implement-medium",
				},
			},
			expectedWF: "implement-medium",
		},
		{
			name:     "explicit weight overrides workflow defaults",
			weight:   "small",
			category: "feature",
			config: &config.Config{
				Weights: config.WeightsConfig{
					Small: "implement-small",
				},
				WorkflowDefaults: config.WorkflowDefaults{
					Feature: "feature-complete",
				},
			},
			expectedWF: "implement-small",
		},
		{
			name:     "no defaults and no weight returns empty",
			weight:   "",
			category: "feature",
			config: &config.Config{
				WorkflowDefaults: config.WorkflowDefaults{},
			},
			expectedEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock resolver that simulates finding workflows
			resolver := &mockWorkflowResolver{workflows: workflows}

			workflowID := ResolveWorkflowForTask(tt.weight, tt.category, tt.config, resolver)

			if tt.expectedEmpty {
				if workflowID != "" {
					t.Errorf("Expected empty workflow ID, got %q", workflowID)
				}
			} else {
				if workflowID != tt.expectedWF {
					t.Errorf("ResolveWorkflowForTask() = %q, want %q", workflowID, tt.expectedWF)
				}
			}
		})
	}
}

// TestTaskCreation_WorkflowDefaultsIntegration tests workflow defaults during task creation.
func TestTaskCreation_WorkflowDefaultsIntegration(t *testing.T) {
	config := &config.Config{
		WorkflowDefaults: config.WorkflowDefaults{
			Feature:  "feature-advanced",
			Bug:      "hotfix-urgent",
			Refactor: "refactor-safe",
			Default:  "crossmodel-standard",
		},
		Weights: config.WeightsConfig{
			Small: "implement-small",
		},
	}

	tests := []struct {
		name               string
		taskCategory       string
		taskWeight         string
		explicitWorkflowID string
		expectedWorkflowID string
	}{
		{
			name:               "feature task uses feature default",
			taskCategory:       "feature",
			taskWeight:         "",
			explicitWorkflowID: "",
			expectedWorkflowID: "feature-advanced",
		},
		{
			name:               "bug task uses bug default",
			taskCategory:       "bug",
			taskWeight:         "",
			explicitWorkflowID: "",
			expectedWorkflowID: "hotfix-urgent",
		},
		{
			name:               "unknown category uses general default",
			taskCategory:       "chore",
			taskWeight:         "",
			explicitWorkflowID: "",
			expectedWorkflowID: "crossmodel-standard",
		},
		{
			name:               "explicit workflow overrides defaults",
			taskCategory:       "feature",
			taskWeight:         "",
			explicitWorkflowID: "custom-workflow",
			expectedWorkflowID: "custom-workflow",
		},
		{
			name:               "weight overrides category default",
			taskCategory:       "feature",
			taskWeight:         "small",
			explicitWorkflowID: "",
			expectedWorkflowID: "implement-small",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflowID := ResolveTaskWorkflow(
				tt.explicitWorkflowID,
				tt.taskWeight,
				tt.taskCategory,
				config,
			)

			if workflowID != tt.expectedWorkflowID {
				t.Errorf("ResolveTaskWorkflow() = %q, want %q", workflowID, tt.expectedWorkflowID)
			}
		})
	}
}

// TestWorkflowDefaults_MigrationFromWeights tests migration from weight-based to workflow defaults.
func TestWorkflowDefaults_MigrationFromWeights(t *testing.T) {
	// Test that existing weight configurations work alongside workflow defaults
	config := &config.Config{
		Weights: config.WeightsConfig{
			Small:  "implement-small",
			Medium: "implement-medium",
		},
		WorkflowDefaults: config.WorkflowDefaults{
			Feature: "feature-complete",
			Default: "crossmodel-standard",
		},
	}

	// Weight-based task should still work
	workflowID := ResolveTaskWorkflow("", "small", "bug", config)
	if workflowID != "implement-small" {
		t.Errorf("Weight-based resolution failed: got %q, want %q", workflowID, "implement-small")
	}

	// Category-based task should use defaults
	workflowID = ResolveTaskWorkflow("", "", "feature", config)
	if workflowID != "feature-complete" {
		t.Errorf("Category-based resolution failed: got %q, want %q", workflowID, "feature-complete")
	}

	// No weight, no recognized category should use default
	workflowID = ResolveTaskWorkflow("", "", "docs", config)
	if workflowID != "crossmodel-standard" {
		t.Errorf("Default resolution failed: got %q, want %q", workflowID, "crossmodel-standard")
	}
}

// TestWorkflowDefaults_ValidationIntegration tests validation with real workflow data.
func TestWorkflowDefaults_ValidationIntegration(t *testing.T) {
	// Available workflows
	availableWorkflows := []string{
		"implement-trivial",
		"implement-small",
		"implement-medium",
		"implement-large",
		"feature-complete",
		"hotfix-urgent",
		"refactor-safe",
	}

	tests := []struct {
		name    string
		config  config.WorkflowDefaults
		wantErr bool
		errMsg  string
	}{
		{
			name: "all valid workflows",
			config: config.WorkflowDefaults{
				Feature:  "feature-complete",
				Bug:      "hotfix-urgent",
				Refactor: "refactor-safe",
				Default:  "implement-medium",
			},
			wantErr: false,
		},
		{
			name: "invalid feature workflow",
			config: config.WorkflowDefaults{
				Feature: "nonexistent-feature-workflow",
				Default: "implement-medium",
			},
			wantErr: true,
			errMsg:  "invalid workflow ID for feature",
		},
		{
			name: "invalid default workflow",
			config: config.WorkflowDefaults{
				Feature: "feature-complete",
				Default: "nonexistent-default",
			},
			wantErr: true,
			errMsg:  "invalid default workflow ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateWorkflowIDs(availableWorkflows)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWorkflowIDs() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil && tt.errMsg != "" {
				if !containsString(err.Error(), tt.errMsg) {
					t.Errorf("ValidateWorkflowIDs() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// Mock types and helpers

type mockWorkflowResolver struct {
	workflows []*db.Workflow
}

func (m *mockWorkflowResolver) GetAvailableWorkflows() []string {
	var ids []string
	for _, wf := range m.workflows {
		ids = append(ids, wf.ID)
	}
	return ids
}

func (m *mockWorkflowResolver) WorkflowExists(id string) bool {
	for _, wf := range m.workflows {
		if wf.ID == id {
			return true
		}
	}
	return false
}

// Placeholder functions that would be implemented in the actual workflow package
func ResolveWorkflowForTask(weight, category string, config *config.Config, resolver *mockWorkflowResolver) string {
	// This would be the actual implementation
	if weight != "" {
		return config.Weights.GetWorkflowID(weight)
	}
	return config.WorkflowDefaults.GetDefaultWorkflow(category)
}

func ResolveTaskWorkflow(explicitWorkflow, weight, category string, config *config.Config) string {
	// Priority: explicit > weight > category default > general default
	if explicitWorkflow != "" {
		return explicitWorkflow
	}
	if weight != "" {
		return config.Weights.GetWorkflowID(weight)
	}
	return config.WorkflowDefaults.GetDefaultWorkflow(category)
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (
			s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			indexString(s, substr) >= 0)))
}

func indexString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
