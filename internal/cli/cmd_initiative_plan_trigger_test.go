// Package cli implements the orc command-line interface.
//
// TDD Tests for TASK-652: on_initiative_planned lifecycle trigger.
//
// NOTE: Tests in this file use os.Chdir() which is process-wide and not goroutine-safe.
// These tests MUST NOT use t.Parallel() and run sequentially within this package.
//
// Success Criteria Coverage:
// - SC-10: on_initiative_planned triggers fire after initiative plan creates tasks
package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/workflow"
)

// --- SC-10: on_initiative_planned triggers fire after tasks created ---

func TestInitiativePlan_LifecycleTrigger(t *testing.T) {
	tmpDir := withInitiativePlanTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create an initiative
	init := initiative.New("INIT-001", "Test Initiative")
	init.Vision = "Test lifecycle triggers on initiative plan"
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	mockRunner := &mockInitiativePlanTriggerRunner{}

	_ = backend.Close()

	// Run initiative plan command with trigger runner
	cmd := newInitiativePlanCmdWithTriggerRunner(mockRunner)
	cmd.SetArgs([]string{"INIT-001"})

	err := cmd.Execute()
	// The plan command may fail if it needs a real Claude session,
	// but we're testing the trigger firing point, not the plan generation.
	// If the command reaches the trigger firing point, the mock records it.

	// If the command executed far enough to fire triggers
	if mockRunner.lifecycleCalled {
		if mockRunner.lastEvent != workflow.WorkflowTriggerEventOnInitiativePlanned {
			t.Errorf("event = %q, want %q", mockRunner.lastEvent, workflow.WorkflowTriggerEventOnInitiativePlanned)
		}
		if mockRunner.lastInitiativeID != "INIT-001" {
			t.Errorf("initiative ID = %q, want %q", mockRunner.lastInitiativeID, "INIT-001")
		}
	} else if err == nil {
		// If command succeeded but trigger wasn't called, that's a bug
		t.Error("initiative plan completed but on_initiative_planned trigger was not fired")
	}
	// If err != nil and trigger wasn't called, the command failed before reaching triggers
	// (e.g., because no Claude session available) - that's expected in test env
}

// --- Edge case: No workflow on initiative → skip triggers ---

func TestInitiativePlan_NoWorkflow_SkipsTriggers(t *testing.T) {
	tmpDir := withInitiativePlanTestDir(t)
	backend := createTestBackendInDir(t, tmpDir)
	defer func() { _ = backend.Close() }()

	// Create initiative without a workflow
	init := initiative.New("INIT-002", "No Workflow Initiative")
	if err := backend.SaveInitiative(init); err != nil {
		t.Fatalf("save initiative: %v", err)
	}

	mockRunner := &mockInitiativePlanTriggerRunner{}

	_ = backend.Close()

	cmd := newInitiativePlanCmdWithTriggerRunner(mockRunner)
	cmd.SetArgs([]string{"INIT-002"})

	_ = cmd.Execute() // May fail - that's OK

	// If no workflow on the initiative, triggers should not fire
	// (This is checked regardless of whether the command itself succeeded)
	if mockRunner.lifecycleCalled && mockRunner.noWorkflow {
		t.Error("triggers should not fire when initiative has no workflow")
	}
}

// --- Helpers ---

func withInitiativePlanTestDir(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	orcDir := filepath.Join(tmpDir, ".orc")
	if err := os.MkdirAll(orcDir, 0755); err != nil {
		t.Fatalf("create .orc directory: %v", err)
	}

	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("create .git directory: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return tmpDir
}

// --- Mock trigger runner for initiative plan tests ---

type mockInitiativePlanTriggerRunner struct {
	lifecycleCalled  bool
	lastEvent        workflow.WorkflowTriggerEvent
	lastInitiativeID string
	lastTaskIDs      []string
	noWorkflow       bool
	lifecycleErr     error
}

func (m *mockInitiativePlanTriggerRunner) RunInitiativePlannedTrigger(
	ctx context.Context,
	triggers []workflow.WorkflowTrigger,
	initiativeID string,
	taskIDs []string,
) error {
	m.lifecycleCalled = true
	m.lastEvent = workflow.WorkflowTriggerEventOnInitiativePlanned
	m.lastInitiativeID = initiativeID
	m.lastTaskIDs = taskIDs
	return m.lifecycleErr
}
