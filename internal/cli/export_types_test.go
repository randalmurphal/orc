package cli

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/randalmurphal/orc/internal/task"
)

func TestExportDataVersion(t *testing.T) {
	t.Parallel()
	// Version 4: workflow system (workflows, phase templates, workflow runs)
	if ExportFormatVersion != 4 {
		t.Errorf("expected ExportFormatVersion 4, got %d", ExportFormatVersion)
	}
}

func TestExportManifestStruct(t *testing.T) {
	t.Parallel()
	manifest := &ExportManifest{
		Version:             4,
		ExportedAt:          time.Now(),
		SourceHostname:      "test-host",
		SourceProject:       "/path/to/project",
		OrcVersion:          "go1.21",
		TaskCount:           5,
		InitiativeCount:     1,
		WorkflowCount:       2,
		PhaseTemplateCount:  3,
		WorkflowRunCount:    4,
		IncludesState:       true,
		IncludesTranscripts: true,
		IncludesWorkflows:   true,
		IncludesRuns:        true,
	}

	// Test YAML marshaling
	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	var unmarshaled ExportManifest
	if err := yaml.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if unmarshaled.Version != 4 {
		t.Errorf("expected version 4, got %d", unmarshaled.Version)
	}
	if unmarshaled.SourceHostname != "test-host" {
		t.Errorf("expected hostname 'test-host', got %q", unmarshaled.SourceHostname)
	}
}

func TestExportDataStruct(t *testing.T) {
	t.Parallel()
	now := time.Now()
	export := &ExportData{
		Version:    4,
		ExportedAt: now,
		Task: &task.Task{
			ID:               "TASK-001",
			Title:            "Test Task",
			Status:           task.StatusRunning,
			ExecutorPID:      12345,
			ExecutorHostname: "old-host",
			CurrentPhase:     "implement",
			Execution:        task.InitExecutionState(),
		},
	}

	// Test YAML round-trip
	data, err := yaml.Marshal(export)
	if err != nil {
		t.Fatalf("marshal export: %v", err)
	}

	var unmarshaled ExportData
	if err := yaml.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal export: %v", err)
	}

	if unmarshaled.Task.ID != "TASK-001" {
		t.Errorf("expected task ID 'TASK-001', got %q", unmarshaled.Task.ID)
	}
	// Note: ExecutorPID has yaml:"-" tag, so it's NOT included in YAML export.
	// This is intentional - executor info is machine-specific and shouldn't be exported.
	// The import logic handles running tasks by transforming them to paused.
	if unmarshaled.Task.ExecutorPID != 0 {
		t.Errorf("expected PID 0 (yaml:'-' excludes it), got %d", unmarshaled.Task.ExecutorPID)
	}
}
