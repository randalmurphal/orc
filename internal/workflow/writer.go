package workflow

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/randalmurphal/orc/internal/util"
	"gopkg.in/yaml.v3"
)

// WriteLevel indicates where to write a workflow or phase file.
type WriteLevel string

const (
	WriteLevelPersonal WriteLevel = "personal" // ~/.orc/
	WriteLevelLocal    WriteLevel = "local"    // .orc/local/
	WriteLevelProject  WriteLevel = "project"  // .orc/
)

// ParseWriteLevel parses a string into a WriteLevel.
func ParseWriteLevel(s string) (WriteLevel, error) {
	switch s {
	case "personal", "global":
		return WriteLevelPersonal, nil
	case "local":
		return WriteLevelLocal, nil
	case "project", "":
		return WriteLevelProject, nil
	default:
		return "", fmt.Errorf("invalid write level: %s (valid: personal, local, project)", s)
	}
}

// SourceToWriteLevel converts a Source to a WriteLevel.
// Returns empty string for non-writable sources (embedded, database).
func SourceToWriteLevel(source Source) WriteLevel {
	switch source {
	case SourcePersonalGlobal:
		return WriteLevelPersonal
	case SourceProjectLocal:
		return WriteLevelLocal
	case SourceProject:
		return WriteLevelProject
	default:
		return ""
	}
}

// Writer writes workflow and phase YAML files.
type Writer struct {
	personalDir string // ~/.orc/
	localDir    string // .orc/local/
	projectDir  string // .orc/
}

// WriterOption configures a Writer.
type WriterOption func(*Writer)

// WithWriterPersonalDir sets the personal directory.
func WithWriterPersonalDir(dir string) WriterOption {
	return func(w *Writer) {
		w.personalDir = dir
	}
}

// WithWriterLocalDir sets the local directory.
func WithWriterLocalDir(dir string) WriterOption {
	return func(w *Writer) {
		w.localDir = dir
	}
}

// WithWriterProjectDir sets the project directory.
func WithWriterProjectDir(dir string) WriterOption {
	return func(w *Writer) {
		w.projectDir = dir
	}
}

// NewWriter creates a new Writer with the given options.
func NewWriter(opts ...WriterOption) *Writer {
	w := &Writer{}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// NewWriterFromOrcDir creates a Writer configured for a project.
func NewWriterFromOrcDir(orcDir string) *Writer {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	var personalDir string
	if homeDir != "" {
		personalDir = filepath.Join(homeDir, ".orc")
	}

	return NewWriter(
		WithWriterPersonalDir(personalDir),
		WithWriterLocalDir(filepath.Join(orcDir, "local")),
		WithWriterProjectDir(orcDir),
	)
}

// WriteWorkflow writes a workflow to a YAML file at the specified level.
func (w *Writer) WriteWorkflow(workflow *Workflow, level WriteLevel) (string, error) {
	dir, err := w.dirForLevel(level)
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, "workflows", workflow.ID+".yaml")
	data, err := marshalWorkflowYAML(workflow)
	if err != nil {
		return "", fmt.Errorf("marshal workflow: %w", err)
	}

	if err := util.AtomicWriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write workflow file: %w", err)
	}

	return path, nil
}

// WritePhase writes a phase template to a YAML file at the specified level.
func (w *Writer) WritePhase(phase *PhaseTemplate, level WriteLevel) (string, error) {
	dir, err := w.dirForLevel(level)
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, "phases", phase.ID+".yaml")
	data, err := marshalPhaseYAML(phase)
	if err != nil {
		return "", fmt.Errorf("marshal phase: %w", err)
	}

	if err := util.AtomicWriteFile(path, data, 0644); err != nil {
		return "", fmt.Errorf("write phase file: %w", err)
	}

	return path, nil
}

// DeleteWorkflow removes a workflow file at the specified level.
func (w *Writer) DeleteWorkflow(id string, level WriteLevel) error {
	dir, err := w.dirForLevel(level)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "workflows", id+".yaml")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete workflow file: %w", err)
	}
	return nil
}

// DeletePhase removes a phase file at the specified level.
func (w *Writer) DeletePhase(id string, level WriteLevel) error {
	dir, err := w.dirForLevel(level)
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "phases", id+".yaml")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete phase file: %w", err)
	}
	return nil
}

// WorkflowPath returns the path where a workflow would be written at the specified level.
func (w *Writer) WorkflowPath(id string, level WriteLevel) (string, error) {
	dir, err := w.dirForLevel(level)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "workflows", id+".yaml"), nil
}

// PhasePath returns the path where a phase would be written at the specified level.
func (w *Writer) PhasePath(id string, level WriteLevel) (string, error) {
	dir, err := w.dirForLevel(level)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "phases", id+".yaml"), nil
}

// WorkflowExists checks if a workflow file exists at the specified level.
func (w *Writer) WorkflowExists(id string, level WriteLevel) (bool, error) {
	path, err := w.WorkflowPath(id, level)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// PhaseExists checks if a phase file exists at the specified level.
func (w *Writer) PhaseExists(id string, level WriteLevel) (bool, error) {
	path, err := w.PhasePath(id, level)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (w *Writer) dirForLevel(level WriteLevel) (string, error) {
	switch level {
	case WriteLevelPersonal:
		if w.personalDir == "" {
			return "", fmt.Errorf("personal directory not configured")
		}
		return w.personalDir, nil
	case WriteLevelLocal:
		if w.localDir == "" {
			return "", fmt.Errorf("local directory not configured")
		}
		return w.localDir, nil
	case WriteLevelProject:
		if w.projectDir == "" {
			return "", fmt.Errorf("project directory not configured")
		}
		return w.projectDir, nil
	default:
		return "", fmt.Errorf("unknown write level: %s", level)
	}
}

// marshalWorkflowYAML converts a Workflow to YAML bytes.
func marshalWorkflowYAML(workflow *Workflow) ([]byte, error) {
	wf := workflowYAML{
		ID:              workflow.ID,
		Name:            workflow.Name,
		Description:     workflow.Description,
		WorkflowType:    string(workflow.WorkflowType),
		DefaultModel:    workflow.DefaultModel,
		DefaultThinking: workflow.DefaultThinking,
		BasedOn:         workflow.BasedOn,
	}

	for _, p := range workflow.Phases {
		phase := workflowPhaseYAML{
			Template:  p.PhaseTemplateID,
			Sequence:  p.Sequence,
			DependsOn: p.DependsOn,
			Model:     p.ModelOverride,
			Condition: p.Condition,
		}
		if p.MaxIterationsOverride != nil {
			phase.MaxIterations = *p.MaxIterationsOverride
		}
		if p.ThinkingOverride != nil {
			phase.Thinking = p.ThinkingOverride
		}
		if p.GateTypeOverride != "" {
			phase.GateType = string(p.GateTypeOverride)
		}
		wf.Phases = append(wf.Phases, phase)
	}

	for _, v := range workflow.Variables {
		variable := variableYAML{
			Name:         v.Name,
			Description:  v.Description,
			SourceType:   string(v.SourceType),
			SourceConfig: v.SourceConfig,
			Required:     v.Required,
			DefaultValue: v.DefaultValue,
		}
		wf.Variables = append(wf.Variables, variable)
	}

	return yaml.Marshal(wf)
}

// marshalPhaseYAML converts a PhaseTemplate to YAML bytes.
func marshalPhaseYAML(phase *PhaseTemplate) ([]byte, error) {
	pt := phaseYAML{
		ID:               phase.ID,
		Name:             phase.Name,
		Description:      phase.Description,
		AgentID:          phase.AgentID,
		SubAgents:        phase.SubAgents,
		PromptSource:     string(phase.PromptSource),
		PromptPath:       phase.PromptPath,
		PromptContent:    phase.PromptContent,
		InputVariables:   phase.InputVariables,
		OutputSchema:     phase.OutputSchema,
		ProducesArtifact: phase.ProducesArtifact,
		ArtifactType:     phase.ArtifactType,
		MaxIterations:    phase.MaxIterations,
		Thinking:         phase.ThinkingEnabled,
		GateType:         string(phase.GateType),
		Checkpoint:       phase.Checkpoint,
		RetryFromPhase:   phase.RetryFromPhase,
		RetryPromptPath:  phase.RetryPromptPath,
	}

	return yaml.Marshal(pt)
}

// Cloner provides high-level clone operations using Resolver and Writer.
type Cloner struct {
	resolver *Resolver
	writer   *Writer
}

// NewCloner creates a new Cloner.
func NewCloner(resolver *Resolver, writer *Writer) *Cloner {
	return &Cloner{
		resolver: resolver,
		writer:   writer,
	}
}

// NewClonerFromOrcDir creates a Cloner configured for a project.
func NewClonerFromOrcDir(orcDir string) *Cloner {
	return NewCloner(
		NewResolverFromOrcDir(orcDir),
		NewWriterFromOrcDir(orcDir),
	)
}

// CloneWorkflowResult contains the result of a clone operation.
type CloneWorkflowResult struct {
	SourceID     string     `json:"source_id"`
	SourceLoc    Source     `json:"source_loc"`
	DestID       string     `json:"dest_id"`
	DestPath     string     `json:"dest_path"`
	DestLevel    WriteLevel `json:"dest_level"`
	WasOverwrite bool       `json:"was_overwrite"`
}

// CloneWorkflow clones a workflow to a new ID at the specified level.
func (c *Cloner) CloneWorkflow(sourceID, destID string, level WriteLevel, overwrite bool) (*CloneWorkflowResult, error) {
	// Resolve source workflow
	resolved, err := c.resolver.ResolveWorkflow(sourceID)
	if err != nil {
		return nil, fmt.Errorf("resolve source workflow %s: %w", sourceID, err)
	}

	// Check if destination exists
	exists, err := c.writer.WorkflowExists(destID, level)
	if err != nil {
		return nil, fmt.Errorf("check existing workflow: %w", err)
	}
	if exists && !overwrite {
		return nil, fmt.Errorf("workflow %s already exists at %s level (use --force to overwrite)", destID, level)
	}

	// Create a copy with the new ID
	cloned := *resolved.Workflow
	cloned.ID = destID
	// Update workflow ID in phases
	for i := range cloned.Phases {
		cloned.Phases[i].WorkflowID = destID
	}
	// Update workflow ID in variables
	for i := range cloned.Variables {
		cloned.Variables[i].WorkflowID = destID
	}

	// Write to destination
	destPath, err := c.writer.WriteWorkflow(&cloned, level)
	if err != nil {
		return nil, fmt.Errorf("write cloned workflow: %w", err)
	}

	return &CloneWorkflowResult{
		SourceID:     sourceID,
		SourceLoc:    resolved.Source,
		DestID:       destID,
		DestPath:     destPath,
		DestLevel:    level,
		WasOverwrite: exists,
	}, nil
}

// ClonePhaseResult contains the result of a phase clone operation.
type ClonePhaseResult struct {
	SourceID     string     `json:"source_id"`
	SourceLoc    Source     `json:"source_loc"`
	DestID       string     `json:"dest_id"`
	DestPath     string     `json:"dest_path"`
	DestLevel    WriteLevel `json:"dest_level"`
	WasOverwrite bool       `json:"was_overwrite"`
}

// ClonePhase clones a phase template to a new ID at the specified level.
func (c *Cloner) ClonePhase(sourceID, destID string, level WriteLevel, overwrite bool) (*ClonePhaseResult, error) {
	// Resolve source phase
	resolved, err := c.resolver.ResolvePhase(sourceID)
	if err != nil {
		return nil, fmt.Errorf("resolve source phase %s: %w", sourceID, err)
	}

	// Check if destination exists
	exists, err := c.writer.PhaseExists(destID, level)
	if err != nil {
		return nil, fmt.Errorf("check existing phase: %w", err)
	}
	if exists && !overwrite {
		return nil, fmt.Errorf("phase %s already exists at %s level (use --force to overwrite)", destID, level)
	}

	// Create a copy with the new ID
	cloned := *resolved.Phase
	cloned.ID = destID

	// Write to destination
	destPath, err := c.writer.WritePhase(&cloned, level)
	if err != nil {
		return nil, fmt.Errorf("write cloned phase: %w", err)
	}

	return &ClonePhaseResult{
		SourceID:     sourceID,
		SourceLoc:    resolved.Source,
		DestID:       destID,
		DestPath:     destPath,
		DestLevel:    level,
		WasOverwrite: exists,
	}, nil
}

// ReadWorkflowYAML reads and returns the raw YAML content for a workflow.
// This is useful for editing operations.
func (c *Cloner) ReadWorkflowYAML(id string) ([]byte, Source, error) {
	resolved, err := c.resolver.ResolveWorkflow(id)
	if err != nil {
		return nil, "", err
	}

	// If it's from a file, read the actual file
	if resolved.FilePath != "" {
		data, err := os.ReadFile(resolved.FilePath)
		if err != nil {
			return nil, "", fmt.Errorf("read workflow file: %w", err)
		}
		return data, resolved.Source, nil
	}

	// For embedded, marshal from the parsed object
	data, err := marshalWorkflowYAML(resolved.Workflow)
	if err != nil {
		return nil, "", fmt.Errorf("marshal embedded workflow: %w", err)
	}
	return data, resolved.Source, nil
}

// ReadPhaseYAML reads and returns the raw YAML content for a phase.
// This is useful for editing operations.
func (c *Cloner) ReadPhaseYAML(id string) ([]byte, Source, error) {
	resolved, err := c.resolver.ResolvePhase(id)
	if err != nil {
		return nil, "", err
	}

	// If it's from a file, read the actual file
	if resolved.FilePath != "" {
		data, err := os.ReadFile(resolved.FilePath)
		if err != nil {
			return nil, "", fmt.Errorf("read phase file: %w", err)
		}
		return data, resolved.Source, nil
	}

	// For embedded, marshal from the parsed object
	data, err := marshalPhaseYAML(resolved.Phase)
	if err != nil {
		return nil, "", fmt.Errorf("marshal embedded phase: %w", err)
	}
	return data, resolved.Source, nil
}
