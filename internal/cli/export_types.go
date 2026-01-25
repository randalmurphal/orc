// Package cli implements the orc command-line interface.
package cli

import (
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// ExportFormatVersion is the current version of the export format.
// Version 3: state and transcripts included by default, tar.gz support
// Version 4: workflow system (workflows, phase templates, workflow runs)
const ExportFormatVersion = 4

// maxImportFileSize is the maximum size of a single file to import (100MB).
// This prevents zip/tar bomb attacks that could exhaust memory.
const maxImportFileSize = 100 * 1024 * 1024

// ExportManifest contains metadata about an export archive.
type ExportManifest struct {
	Version              int       `yaml:"version"`
	ExportedAt           time.Time `yaml:"exported_at"`
	SourceHostname       string    `yaml:"source_hostname"`
	SourceProject        string    `yaml:"source_project,omitempty"`
	OrcVersion           string    `yaml:"orc_version,omitempty"`
	TaskCount            int       `yaml:"task_count"`
	InitiativeCount      int       `yaml:"initiative_count"`
	WorkflowCount        int       `yaml:"workflow_count,omitempty"`
	PhaseTemplateCount   int       `yaml:"phase_template_count,omitempty"`
	WorkflowRunCount     int       `yaml:"workflow_run_count,omitempty"`
	ProjectCommandCount  int       `yaml:"project_command_count,omitempty"`
	IncludesState        bool      `yaml:"includes_state"`
	IncludesTranscripts  bool      `yaml:"includes_transcripts"`
	IncludesWorkflows    bool      `yaml:"includes_workflows,omitempty"`
	IncludesRuns         bool      `yaml:"includes_runs,omitempty"`
}

// ExportData contains all data for a task export.
type ExportData struct {
	// Metadata for format versioning
	Version    int       `yaml:"version"`
	ExportedAt time.Time `yaml:"exported_at"`

	// Core task data (includes execution state in Task.Execution)
	Task *task.Task `yaml:"task"`
	Spec string     `yaml:"spec,omitempty"`

	// Execution history
	Transcripts   []storage.Transcript `yaml:"transcripts,omitempty"`
	GateDecisions []db.GateDecision    `yaml:"gate_decisions,omitempty"`

	// Collaboration data
	TaskComments   []storage.TaskComment   `yaml:"task_comments,omitempty"`
	ReviewComments []storage.ReviewComment `yaml:"review_comments,omitempty"`

	// Attachments (binary data base64 encoded in YAML)
	Attachments []AttachmentExport `yaml:"attachments,omitempty"`
}

// AttachmentExport represents an attachment for export.
type AttachmentExport struct {
	Filename    string `yaml:"filename"`
	ContentType string `yaml:"content_type"`
	SizeBytes   int64  `yaml:"size_bytes"`
	IsImage     bool   `yaml:"is_image"`
	Data        []byte `yaml:"data"` // base64 encoded in YAML
}

// WorkflowExportData contains all data for a workflow export.
type WorkflowExportData struct {
	Version    int       `yaml:"version"`
	ExportedAt time.Time `yaml:"exported_at"`
	Type       string    `yaml:"type"` // "workflow"

	Workflow  *db.Workflow         `yaml:"workflow"`
	Phases    []*db.WorkflowPhase  `yaml:"phases,omitempty"`
	Variables []*db.WorkflowVariable `yaml:"variables,omitempty"`
}

// PhaseTemplateExportData contains all data for a phase template export.
type PhaseTemplateExportData struct {
	Version    int       `yaml:"version"`
	ExportedAt time.Time `yaml:"exported_at"`
	Type       string    `yaml:"type"` // "phase_template"

	PhaseTemplate *db.PhaseTemplate `yaml:"phase_template"`
}

// WorkflowRunExportData contains all data for a workflow run export.
type WorkflowRunExportData struct {
	Version    int       `yaml:"version"`
	ExportedAt time.Time `yaml:"exported_at"`
	Type       string    `yaml:"type"` // "workflow_run"

	WorkflowRun *db.WorkflowRun        `yaml:"workflow_run"`
	Phases      []*db.WorkflowRunPhase `yaml:"phases,omitempty"`
}

// ProjectCommandsExportData contains project commands for export.
type ProjectCommandsExportData struct {
	Version    int       `yaml:"version"`
	ExportedAt time.Time `yaml:"exported_at"`
	Type       string    `yaml:"type"` // "project_commands"

	Commands []*db.ProjectCommand `yaml:"commands"`
}

// InitiativeExportData contains all data for an initiative export.
type InitiativeExportData struct {
	Version    int       `yaml:"version"`
	ExportedAt time.Time `yaml:"exported_at"`
	Type       string    `yaml:"type"` // "initiative" to distinguish from task exports

	Initiative *initiative.Initiative `yaml:"initiative"`
}

// exportAllOptions contains options for the bulk export operation.
type exportAllOptions struct {
	withState       bool
	withTranscripts bool
	withInitiatives bool
	withWorkflows   bool
	withRuns        bool
}

// exportAllData contains all data to be exported.
type exportAllData struct {
	tasks           []*task.Task
	initiatives     []*initiative.Initiative
	phaseTemplates  []*db.PhaseTemplate
	workflows       []*db.Workflow
	workflowRuns    []*db.WorkflowRun
	projectCommands []*db.ProjectCommand
}
