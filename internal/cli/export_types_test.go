package cli

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
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
	hostname := "old-host"
	currentPhase := "implement"
	export := &ExportData{
		Version:    4,
		ExportedAt: now,
		Task: &orcv1.Task{
			Id:               "TASK-001",
			Title:            "Test Task",
			Status:           orcv1.TaskStatus_TASK_STATUS_RUNNING,
			ExecutorPid:      12345,
			ExecutorHostname: &hostname,
			CurrentPhase:     &currentPhase,
			Execution:        task.InitProtoExecutionState(),
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

	if unmarshaled.Task.Id != "TASK-001" {
		t.Errorf("expected task ID 'TASK-001', got %q", unmarshaled.Task.Id)
	}
	// Note: ExecutorPid is included in YAML for proto types, but import logic
	// still handles running tasks by transforming them to paused.
	// The proto type uses yaml marshaling from the generated code.
}
