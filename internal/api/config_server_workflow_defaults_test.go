package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
)

// TestConfigServer_GetWorkflowDefaults tests retrieving workflow defaults via API.
func TestConfigServer_GetWorkflowDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, config.OrcDir, config.ConfigFileName)

	// Create config directory
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create config with workflow defaults
	cfg := config.Default()
	cfg.WorkflowDefaults = config.WorkflowDefaults{
		Feature:   "feature-complete",
		Bug:       "hotfix",
		Refactor:  "refactor-safe",
		Default:   "implement-standard",
	}

	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Create server
	backend := storage.NewTestBackend(t)
	server := NewConfigServer(cfg, backend, tmpDir, nil)

	// Test GetWorkflowDefaults
	req := connect.NewRequest(&orcv1.GetWorkflowDefaultsRequest{})
	resp, err := server.GetWorkflowDefaults(context.Background(), req)
	if err != nil {
		t.Fatalf("GetWorkflowDefaults failed: %v", err)
	}

	defaults := resp.Msg.WorkflowDefaults
	if defaults == nil {
		t.Fatal("WorkflowDefaults is nil")
	}

	// Verify all fields
	if defaults.Feature != "feature-complete" {
		t.Errorf("Feature = %q, want %q", defaults.Feature, "feature-complete")
	}
	if defaults.Bug != "hotfix" {
		t.Errorf("Bug = %q, want %q", defaults.Bug, "hotfix")
	}
	if defaults.Refactor != "refactor-safe" {
		t.Errorf("Refactor = %q, want %q", defaults.Refactor, "refactor-safe")
	}
	if defaults.Default != "implement-standard" {
		t.Errorf("Default = %q, want %q", defaults.Default, "implement-standard")
	}
}

// TestConfigServer_UpdateWorkflowDefaults tests updating workflow defaults via API.
func TestConfigServer_UpdateWorkflowDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, config.OrcDir, config.ConfigFileName)

	// Create config directory
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create initial config
	cfg := config.Default()
	cfg.WorkflowDefaults = config.WorkflowDefaults{
		Feature: "old-feature",
		Default: "old-default",
	}

	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatalf("Failed to save initial config: %v", err)
	}

	// Create server
	backend := storage.NewTestBackend(t)
	server := NewConfigServer(cfg, backend, tmpDir, nil)

	// Test UpdateWorkflowDefaults
	req := connect.NewRequest(&orcv1.UpdateWorkflowDefaultsRequest{
		WorkflowDefaults: &orcv1.WorkflowDefaults{
			Feature:   "new-feature",
			Bug:       "new-bug",
			Refactor:  "new-refactor",
			Default:   "new-default",
		},
	})

	resp, err := server.UpdateWorkflowDefaults(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateWorkflowDefaults failed: %v", err)
	}

	// Verify response
	defaults := resp.Msg.WorkflowDefaults
	if defaults.Feature != "new-feature" {
		t.Errorf("Feature = %q, want %q", defaults.Feature, "new-feature")
	}
	if defaults.Bug != "new-bug" {
		t.Errorf("Bug = %q, want %q", defaults.Bug, "new-bug")
	}
	if defaults.Refactor != "new-refactor" {
		t.Errorf("Refactor = %q, want %q", defaults.Refactor, "new-refactor")
	}
	if defaults.Default != "new-default" {
		t.Errorf("Default = %q, want %q", defaults.Default, "new-default")
	}

	// Verify config file was updated
	reloaded, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	if reloaded.WorkflowDefaults.Feature != "new-feature" {
		t.Errorf("Persisted Feature = %q, want %q", reloaded.WorkflowDefaults.Feature, "new-feature")
	}
	if reloaded.WorkflowDefaults.Bug != "new-bug" {
		t.Errorf("Persisted Bug = %q, want %q", reloaded.WorkflowDefaults.Bug, "new-bug")
	}
	if reloaded.WorkflowDefaults.Default != "new-default" {
		t.Errorf("Persisted Default = %q, want %q", reloaded.WorkflowDefaults.Default, "new-default")
	}
}

// TestConfigServer_UpdateWorkflowDefaults_PartialUpdate tests partial updates.
func TestConfigServer_UpdateWorkflowDefaults_PartialUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, config.OrcDir, config.ConfigFileName)

	// Create config directory
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create initial config
	cfg := config.Default()
	cfg.WorkflowDefaults = config.WorkflowDefaults{
		Feature:  "existing-feature",
		Bug:      "existing-bug",
		Default:  "existing-default",
	}

	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatalf("Failed to save initial config: %v", err)
	}

	// Create server
	backend := storage.NewTestBackend(t)
	server := NewConfigServer(cfg, backend, tmpDir, nil)

	// Test partial update (only feature and default)
	req := connect.NewRequest(&orcv1.UpdateWorkflowDefaultsRequest{
		WorkflowDefaults: &orcv1.WorkflowDefaults{
			Feature: "updated-feature",
			Default: "updated-default",
			// Bug and others not specified
		},
	})

	resp, err := server.UpdateWorkflowDefaults(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateWorkflowDefaults failed: %v", err)
	}

	// Verify updated fields
	defaults := resp.Msg.WorkflowDefaults
	if defaults.Feature != "updated-feature" {
		t.Errorf("Feature = %q, want %q", defaults.Feature, "updated-feature")
	}
	if defaults.Default != "updated-default" {
		t.Errorf("Default = %q, want %q", defaults.Default, "updated-default")
	}

	// Verify unchanged field
	if defaults.Bug != "existing-bug" {
		t.Errorf("Bug = %q, want %q (should be unchanged)", defaults.Bug, "existing-bug")
	}
}

// TestConfigServer_UpdateWorkflowDefaults_Validation tests validation during updates.
func TestConfigServer_UpdateWorkflowDefaults_Validation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, config.OrcDir, config.ConfigFileName)

	// Create config directory
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	// Create initial config
	cfg := config.Default()
	if err := cfg.SaveTo(configPath); err != nil {
		t.Fatalf("Failed to save initial config: %v", err)
	}

	// Create server
	backend := storage.NewTestBackend(t)
	server := NewConfigServer(cfg, backend, tmpDir, nil)

	tests := []struct {
		name    string
		request *orcv1.UpdateWorkflowDefaultsRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "empty workflow IDs are allowed",
			request: &orcv1.UpdateWorkflowDefaultsRequest{
				WorkflowDefaults: &orcv1.WorkflowDefaults{
					Feature: "",
					Default: "valid-workflow",
				},
			},
			wantErr: false,
		},
		{
			name: "nil workflow defaults",
			request: &orcv1.UpdateWorkflowDefaultsRequest{
				WorkflowDefaults: nil,
			},
			wantErr: true,
			errMsg:  "workflow_defaults is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := server.UpdateWorkflowDefaults(context.Background(), connect.NewRequest(tt.request))

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateWorkflowDefaults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("UpdateWorkflowDefaults() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

// TestConfigServer_GetWorkflowDefaults_ProjectScope tests project-scoped workflow defaults.
func TestConfigServer_GetWorkflowDefaults_ProjectScope(t *testing.T) {
	tmpDir := t.TempDir()

	// Create project-specific config
	projectConfigPath := filepath.Join(tmpDir, config.OrcDir, config.ConfigFileName)
	if err := os.MkdirAll(filepath.Dir(projectConfigPath), 0755); err != nil {
		t.Fatalf("Failed to create project config dir: %v", err)
	}

	projectCfg := config.Default()
	projectCfg.WorkflowDefaults = config.WorkflowDefaults{
		Feature: "project-feature",
		Default: "project-default",
	}

	if err := projectCfg.SaveTo(projectConfigPath); err != nil {
		t.Fatalf("Failed to save project config: %v", err)
	}

	// Create server with project cache for multi-project support
	backend := storage.NewTestBackend(t)
	server := NewConfigServer(projectCfg, backend, tmpDir, nil)

	// Test with project ID
	req := connect.NewRequest(&orcv1.GetWorkflowDefaultsRequest{
		ProjectId: "test-project",
	})

	resp, err := server.GetWorkflowDefaults(context.Background(), req)
	if err != nil {
		t.Fatalf("GetWorkflowDefaults with project failed: %v", err)
	}

	defaults := resp.Msg.WorkflowDefaults
	if defaults.Feature != "project-feature" {
		t.Errorf("Project Feature = %q, want %q", defaults.Feature, "project-feature")
	}
	if defaults.Default != "project-default" {
		t.Errorf("Project Default = %q, want %q", defaults.Default, "project-default")
	}
}

// contains is a helper to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) &&
		   (s == substr || (len(s) > len(substr) &&
		    (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		     containsHelper(s, substr))))
}

func containsHelper(s, substr string) bool {
	for i := 1; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}