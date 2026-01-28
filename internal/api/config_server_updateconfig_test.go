// Package api provides the Connect RPC and REST API server for orc.
//
// TDD Tests for TASK-603: Build Execution Settings section with sliders and toggles
//
// Tests verify:
// - SC-1: UpdateConfig persists automation.auto_approve
// - SC-2: UpdateConfig persists execution.parallel_tasks
// - SC-3: UpdateConfig persists execution.cost_limit
// - SC-4: UpdateConfig persists claude.model
// - SC-6: GetConfig returns execution fields with defaults
// - Edge cases: boundary values, validation, partial updates, error handling
package api

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"gopkg.in/yaml.v3"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/storage"
)

// ============================================================================
// SC-1: UpdateConfig persists automation.auto_approve
// ============================================================================

func TestUpdateConfig_AutoApprove_PersistsToFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	initialCfg := config.Default()
	if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Automation: &orcv1.AutomationConfig{
			AutoApprove: true,
		},
	})

	_, err := server.UpdateConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify the file was written correctly
	data, err := os.ReadFile(filepath.Join(orcDir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var savedCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &savedCfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	automation, ok := savedCfg["automation"].(map[string]interface{})
	if !ok {
		t.Fatal("config file missing automation section")
	}

	if autoApproveVal, ok := automation["auto_approve"].(bool); !ok || !autoApproveVal {
		t.Errorf("auto_approve not persisted correctly, got %v", automation["auto_approve"])
	}
}

func TestUpdateConfig_AutoApprove_False_PersistsToFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	initialCfg := config.Default()
	if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Automation: &orcv1.AutomationConfig{
			AutoApprove: false,
		},
	})

	_, err := server.UpdateConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify persisted
	data, err := os.ReadFile(filepath.Join(orcDir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var savedCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &savedCfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	automation, ok := savedCfg["automation"].(map[string]interface{})
	if !ok {
		t.Fatal("config file missing automation section")
	}

	// auto_approve should be false (or absent, which YAML treats as false)
	if autoApproveVal, ok := automation["auto_approve"].(bool); ok && autoApproveVal {
		t.Errorf("auto_approve should be false, got %v", autoApproveVal)
	}
}

// ============================================================================
// SC-2: UpdateConfig persists execution.parallel_tasks
// ============================================================================

func TestUpdateConfig_ParallelTasks_PersistsToFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	initialCfg := config.Default()
	if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Execution: &orcv1.ExecutionConfig{
			ParallelTasks: 3,
		},
	})

	_, err := server.UpdateConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify the file was written correctly
	data, err := os.ReadFile(filepath.Join(orcDir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var savedCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &savedCfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	execution, ok := savedCfg["execution"].(map[string]interface{})
	if !ok {
		t.Fatal("config file missing execution section")
	}

	// YAML unmarshals numbers as int by default
	parallelTasks, ok := execution["parallel_tasks"].(int)
	if !ok || parallelTasks != 3 {
		t.Errorf("parallel_tasks not persisted correctly, got %v", execution["parallel_tasks"])
	}
}

// TestUpdateConfig_ParallelTasks_BoundaryValues tests edge cases for parallel_tasks
func TestUpdateConfig_ParallelTasks_BoundaryValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     int32
		wantError bool
	}{
		{"value_1_valid", 1, false},
		{"value_5_valid", 5, false},
		// AMEND-001: In proto3, parallel_tasks=0 is indistinguishable from "not set"
		// because scalar fields default to zero. We treat 0 as "not provided" rather
		// than "invalid value". This is consistent with cost_limit=0 being valid.
		{"value_0_noop", 0, false},
		{"value_6_invalid", 6, true},
		{"value_negative_invalid", -1, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			projectDir := t.TempDir()
			orcDir := filepath.Join(projectDir, ".orc")
			if err := os.MkdirAll(orcDir, 0755); err != nil {
				t.Fatalf("create .orc dir: %v", err)
			}

			initialCfg := config.Default()
			if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
				t.Fatalf("save initial config: %v", err)
			}

			backend := storage.NewTestBackend(t)
			server := NewConfigServer(initialCfg, backend, projectDir, nil)

			req := connect.NewRequest(&orcv1.UpdateConfigRequest{
				Execution: &orcv1.ExecutionConfig{
					ParallelTasks: tc.value,
				},
			})

			_, err := server.UpdateConfig(context.Background(), req)

			if tc.wantError {
				if err == nil {
					t.Errorf("expected error for parallel_tasks=%d, got nil", tc.value)
				}
				// Verify it returns INVALID_ARGUMENT code
				if connectErr, ok := err.(*connect.Error); ok {
					if connectErr.Code() != connect.CodeInvalidArgument {
						t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for parallel_tasks=%d: %v", tc.value, err)
				}
			}
		})
	}
}

// ============================================================================
// SC-3: UpdateConfig persists execution.cost_limit
// ============================================================================

func TestUpdateConfig_CostLimit_PersistsToFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	initialCfg := config.Default()
	if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Execution: &orcv1.ExecutionConfig{
			CostLimit: 50,
		},
	})

	_, err := server.UpdateConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify the file was written correctly
	data, err := os.ReadFile(filepath.Join(orcDir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var savedCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &savedCfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	execution, ok := savedCfg["execution"].(map[string]interface{})
	if !ok {
		t.Fatal("config file missing execution section")
	}

	costLimit, ok := execution["cost_limit"].(int)
	if !ok || costLimit != 50 {
		t.Errorf("cost_limit not persisted correctly, got %v", execution["cost_limit"])
	}
}

func TestUpdateConfig_CostLimit_BoundaryValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		value     int32
		wantError bool
	}{
		{"value_0_valid", 0, false},
		{"value_100_valid", 100, false},
		{"value_50_valid", 50, false},
		{"value_negative_invalid", -1, true},
		{"value_101_invalid", 101, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			projectDir := t.TempDir()
			orcDir := filepath.Join(projectDir, ".orc")
			if err := os.MkdirAll(orcDir, 0755); err != nil {
				t.Fatalf("create .orc dir: %v", err)
			}

			initialCfg := config.Default()
			if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
				t.Fatalf("save initial config: %v", err)
			}

			backend := storage.NewTestBackend(t)
			server := NewConfigServer(initialCfg, backend, projectDir, nil)

			req := connect.NewRequest(&orcv1.UpdateConfigRequest{
				Execution: &orcv1.ExecutionConfig{
					CostLimit: tc.value,
				},
			})

			_, err := server.UpdateConfig(context.Background(), req)

			if tc.wantError {
				if err == nil {
					t.Errorf("expected error for cost_limit=%d, got nil", tc.value)
				}
				if connectErr, ok := err.(*connect.Error); ok {
					if connectErr.Code() != connect.CodeInvalidArgument {
						t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for cost_limit=%d: %v", tc.value, err)
				}
			}
		})
	}
}

// ============================================================================
// SC-4: UpdateConfig persists claude.model
// ============================================================================

func TestUpdateConfig_Model_PersistsToFile(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	initialCfg := config.Default()
	if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Claude: &orcv1.ClaudeConfig{
			Model: "claude-opus-4-20250514",
		},
	})

	_, err := server.UpdateConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify the file was written correctly
	data, err := os.ReadFile(filepath.Join(orcDir, "config.yaml"))
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var savedCfg map[string]interface{}
	if err := yaml.Unmarshal(data, &savedCfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	model, ok := savedCfg["model"].(string)
	if !ok || model != "claude-opus-4-20250514" {
		t.Errorf("model not persisted correctly, got %v", savedCfg["model"])
	}
}

func TestUpdateConfig_Model_InvalidModel(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	initialCfg := config.Default()
	if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Claude: &orcv1.ClaudeConfig{
			Model: "gpt-4o-invalid",
		},
	})

	_, err := server.UpdateConfig(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for invalid model, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInvalidArgument {
		t.Errorf("expected CodeInvalidArgument, got %v", connectErr.Code())
	}
}

// ============================================================================
// SC-6: GetConfig returns execution fields with defaults
// ============================================================================

func TestGetConfig_ReturnsExecutionFields(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	// Set specific execution values
	initialCfg := config.Default()
	initialCfg.Execution.ParallelTasks = 4
	initialCfg.Execution.CostLimit = 75
	if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
		t.Fatalf("save config: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.GetConfigRequest{})
	resp, err := server.GetConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	execution := resp.Msg.Config.Execution
	if execution == nil {
		t.Fatal("GetConfig response missing execution section")
	}

	t.Logf("GetConfig.Execution: parallel_tasks=%d, cost_limit=%d",
		execution.ParallelTasks, execution.CostLimit)

	if execution.ParallelTasks != 4 {
		t.Errorf("GetConfig.Execution.ParallelTasks = %d, want 4", execution.ParallelTasks)
	}
	if execution.CostLimit != 75 {
		t.Errorf("GetConfig.Execution.CostLimit = %d, want 75", execution.CostLimit)
	}
}

func TestGetConfig_ReturnsDefaultsWhenNotConfigured(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	// Write a minimal config without execution section
	minimalCfg := "version: 1\n"
	if err := os.WriteFile(filepath.Join(orcDir, "config.yaml"), []byte(minimalCfg), 0644); err != nil {
		t.Fatalf("write minimal config: %v", err)
	}

	// Use Default() for the server config - this has the built-in defaults
	initialCfg := config.Default()
	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.GetConfigRequest{})
	resp, err := server.GetConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	execution := resp.Msg.Config.Execution
	if execution == nil {
		t.Fatal("GetConfig response missing execution section")
	}

	if execution.ParallelTasks != 2 {
		t.Errorf("GetConfig.Execution.ParallelTasks = %d, want 2 (default)", execution.ParallelTasks)
	}
	if execution.CostLimit != 25 {
		t.Errorf("GetConfig.Execution.CostLimit = %d, want 25 (default)", execution.CostLimit)
	}
}

// ============================================================================
// Edge cases and error handling
// ============================================================================

func TestUpdateConfig_CreateConfigFileIfNotExists(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	// Don't create .orc dir or config file

	initialCfg := config.Default()
	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Execution: &orcv1.ExecutionConfig{
			ParallelTasks: 3,
		},
	})

	_, err := server.UpdateConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Verify the file was created
	configPath := filepath.Join(projectDir, ".orc", "config.yaml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}
}

func TestUpdateConfig_WriteError_ReturnsInternalError(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	// Make config file read-only directory to cause write error
	configPath := filepath.Join(orcDir, "config.yaml")
	initialCfg := config.Default()
	if err := initialCfg.SaveTo(configPath); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	// Make the file and directory read-only to prevent writes
	if err := os.Chmod(configPath, 0444); err != nil {
		t.Fatalf("chmod file: %v", err)
	}
	if err := os.Chmod(orcDir, 0555); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(orcDir, 0755)
		_ = os.Chmod(configPath, 0644)
	})

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	req := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Execution: &orcv1.ExecutionConfig{
			ParallelTasks: 3,
		},
	})

	_, err := server.UpdateConfig(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for read-only directory, got nil")
	}

	connectErr, ok := err.(*connect.Error)
	if !ok {
		t.Fatalf("expected connect.Error, got %T", err)
	}
	if connectErr.Code() != connect.CodeInternal {
		t.Errorf("expected CodeInternal, got %v", connectErr.Code())
	}
}

// ============================================================================
// Integration: Round-trip test
// ============================================================================

func TestConfigRoundTrip_UpdateThenGet(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	initialCfg := config.Default()
	if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	// Update config with specific values
	updateReq := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Automation: &orcv1.AutomationConfig{
			AutoApprove: false,
		},
		Execution: &orcv1.ExecutionConfig{
			ParallelTasks: 4,
			CostLimit:     80,
		},
		Claude: &orcv1.ClaudeConfig{
			Model: "claude-opus-4-20250514",
		},
	})

	_, err := server.UpdateConfig(context.Background(), updateReq)
	if err != nil {
		t.Fatalf("UpdateConfig failed: %v", err)
	}

	// Get config back
	getReq := connect.NewRequest(&orcv1.GetConfigRequest{})
	getResp, err := server.GetConfig(context.Background(), getReq)
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	cfg := getResp.Msg.Config

	// Verify all values were preserved
	if cfg.Automation.AutoApprove {
		t.Error("AutoApprove should be false after update")
	}
	if cfg.Execution.ParallelTasks != 4 {
		t.Errorf("ParallelTasks = %d, want 4", cfg.Execution.ParallelTasks)
	}
	if cfg.Execution.CostLimit != 80 {
		t.Errorf("CostLimit = %d, want 80", cfg.Execution.CostLimit)
	}
	if cfg.Claude.Model != "claude-opus-4-20250514" {
		t.Errorf("Model = %q, want claude-opus-4-20250514", cfg.Claude.Model)
	}
}

// ============================================================================
// Partial updates preserve other fields
// ============================================================================

func TestUpdateConfig_PartialUpdate_PreservesOtherFields(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	orcDir := filepath.Join(projectDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc dir: %v", err)
	}

	initialCfg := config.Default()
	if err := initialCfg.SaveTo(filepath.Join(orcDir, "config.yaml")); err != nil {
		t.Fatalf("save initial config: %v", err)
	}

	backend := storage.NewTestBackend(t)
	server := NewConfigServer(initialCfg, backend, projectDir, nil)

	// First update: set parallel_tasks
	req1 := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Execution: &orcv1.ExecutionConfig{
			ParallelTasks: 3,
		},
	})
	_, err := server.UpdateConfig(context.Background(), req1)
	if err != nil {
		t.Fatalf("first UpdateConfig failed: %v", err)
	}

	// Second update: set cost_limit only
	req2 := connect.NewRequest(&orcv1.UpdateConfigRequest{
		Execution: &orcv1.ExecutionConfig{
			CostLimit: 50,
		},
	})
	_, err = server.UpdateConfig(context.Background(), req2)
	if err != nil {
		t.Fatalf("second UpdateConfig failed: %v", err)
	}

	// Get config and verify both values
	getReq := connect.NewRequest(&orcv1.GetConfigRequest{})
	getResp, err := server.GetConfig(context.Background(), getReq)
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	execution := getResp.Msg.Config.Execution
	if execution.ParallelTasks != 3 {
		t.Errorf("ParallelTasks = %d after partial update, want 3 (preserved)", execution.ParallelTasks)
	}
	if execution.CostLimit != 50 {
		t.Errorf("CostLimit = %d after partial update, want 50", execution.CostLimit)
	}
}
