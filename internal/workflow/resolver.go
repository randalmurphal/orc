package workflow

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/randalmurphal/orc/templates"
	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned when a workflow or phase template is not found.
var ErrNotFound = errors.New("not found")

// ResolvedWorkflow contains the resolved workflow and its source.
type ResolvedWorkflow struct {
	Workflow *Workflow `json:"workflow"`
	Source   Source    `json:"source"`
	FilePath string    `json:"file_path,omitempty"` // For file sources
}

// ResolvedPhase contains the resolved phase template and its source.
type ResolvedPhase struct {
	Phase    *PhaseTemplate `json:"phase"`
	Source   Source         `json:"source"`
	FilePath string         `json:"file_path,omitempty"` // For file sources
}

// Resolver resolves workflows and phases from multiple sources.
// Priority order: personal > local > project > embedded
type Resolver struct {
	personalDir string // ~/.orc/
	localDir    string // .orc/local/
	projectDir  string // .orc/
	embedded    bool   // Whether to check embedded templates
}

// ResolverOption configures a Resolver.
type ResolverOption func(*Resolver)

// WithPersonalDir sets the personal directory (~/.orc/).
func WithPersonalDir(dir string) ResolverOption {
	return func(r *Resolver) {
		r.personalDir = dir
	}
}

// WithLocalDir sets the local directory (.orc/local/).
func WithLocalDir(dir string) ResolverOption {
	return func(r *Resolver) {
		r.localDir = dir
	}
}

// WithProjectDir sets the project directory (.orc/).
func WithProjectDir(dir string) ResolverOption {
	return func(r *Resolver) {
		r.projectDir = dir
	}
}

// WithEmbedded enables or disables checking embedded templates.
func WithEmbedded(enabled bool) ResolverOption {
	return func(r *Resolver) {
		r.embedded = enabled
	}
}

// NewResolver creates a new Resolver with the given options.
func NewResolver(opts ...ResolverOption) *Resolver {
	r := &Resolver{
		embedded: true, // Default to checking embedded
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// OrcDir returns the project directory (.orc/).
func (r *Resolver) OrcDir() string {
	return r.projectDir
}

// NewResolverFromOrcDir creates a Resolver configured for a project.
func NewResolverFromOrcDir(orcDir string) *Resolver {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("could not determine home directory", "error", err)
		homeDir = ""
	}

	var personalDir string
	if homeDir != "" {
		personalDir = filepath.Join(homeDir, ".orc")
	}

	return NewResolver(
		WithPersonalDir(personalDir),
		WithLocalDir(filepath.Join(orcDir, "local")),
		WithProjectDir(orcDir),
		WithEmbedded(true),
	)
}

// ResolveWorkflow returns the workflow for an ID, checking sources in priority order.
func (r *Resolver) ResolveWorkflow(id string) (*ResolvedWorkflow, error) {
	filename := id + ".yaml"

	sources := []struct {
		dir    string
		subdir string
		source Source
	}{
		{r.personalDir, "workflows", SourcePersonalGlobal},
		{r.localDir, "workflows", SourceProjectLocal},
		{r.projectDir, "workflows", SourceProject},
	}

	for _, s := range sources {
		if s.dir == "" {
			continue
		}
		path := filepath.Join(s.dir, s.subdir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // File doesn't exist, try next
		}

		workflow, err := parseWorkflowYAML(data)
		if err != nil {
			slog.Warn("failed to parse workflow file", "path", path, "error", err)
			continue
		}

		return &ResolvedWorkflow{
			Workflow: workflow,
			Source:   s.source,
			FilePath: path,
		}, nil
	}

	// Fall back to embedded
	if r.embedded {
		workflow, err := r.readEmbeddedWorkflow(id)
		if err != nil {
			return nil, fmt.Errorf("workflow not found: %s", id)
		}
		return &ResolvedWorkflow{
			Workflow: workflow,
			Source:   SourceEmbedded,
		}, nil
	}

	return nil, fmt.Errorf("workflow not found: %s", id)
}

// ResolvePhase returns the phase template for an ID, checking sources in priority order.
func (r *Resolver) ResolvePhase(id string) (*ResolvedPhase, error) {
	filename := id + ".yaml"

	sources := []struct {
		dir    string
		subdir string
		source Source
	}{
		{r.personalDir, "phases", SourcePersonalGlobal},
		{r.localDir, "phases", SourceProjectLocal},
		{r.projectDir, "phases", SourceProject},
	}

	for _, s := range sources {
		if s.dir == "" {
			continue
		}
		path := filepath.Join(s.dir, s.subdir, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // File doesn't exist, try next
		}

		phase, err := parsePhaseYAML(data)
		if err != nil {
			slog.Warn("failed to parse phase file", "path", path, "error", err)
			continue
		}

		return &ResolvedPhase{
			Phase:    phase,
			Source:   s.source,
			FilePath: path,
		}, nil
	}

	// Fall back to embedded
	if r.embedded {
		phase, err := r.readEmbeddedPhase(id)
		if err != nil {
			return nil, fmt.Errorf("phase not found: %s", id)
		}
		return &ResolvedPhase{
			Phase:  phase,
			Source: SourceEmbedded,
		}, nil
	}

	return nil, fmt.Errorf("phase not found: %s", id)
}

// ListWorkflows returns all available workflows from all sources.
func (r *Resolver) ListWorkflows() ([]ResolvedWorkflow, error) {
	seen := make(map[string]*ResolvedWorkflow)

	// Scan file-based sources (higher priority first)
	sources := []struct {
		dir    string
		subdir string
		source Source
	}{
		{r.personalDir, "workflows", SourcePersonalGlobal},
		{r.localDir, "workflows", SourceProjectLocal},
		{r.projectDir, "workflows", SourceProject},
	}

	for _, s := range sources {
		if s.dir == "" {
			continue
		}
		dir := filepath.Join(s.dir, s.subdir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Directory doesn't exist
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}

			id := strings.TrimSuffix(entry.Name(), ".yaml")
			if _, exists := seen[id]; exists {
				continue // Already found at higher priority
			}

			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			workflow, err := parseWorkflowYAML(data)
			if err != nil {
				slog.Warn("failed to parse workflow", "path", path, "error", err)
				continue
			}

			seen[id] = &ResolvedWorkflow{
				Workflow: workflow,
				Source:   s.source,
				FilePath: path,
			}
		}
	}

	// Add embedded workflows
	if r.embedded {
		embeddedIDs, err := r.listEmbeddedWorkflowIDs()
		if err != nil {
			slog.Warn("failed to list embedded workflows", "error", err)
		} else {
			for _, id := range embeddedIDs {
				if _, exists := seen[id]; exists {
					continue // Shadowed by file-based
				}
				workflow, err := r.readEmbeddedWorkflow(id)
				if err != nil {
					continue
				}
				seen[id] = &ResolvedWorkflow{
					Workflow: workflow,
					Source:   SourceEmbedded,
				}
			}
		}
	}

	// Convert to sorted slice
	result := make([]ResolvedWorkflow, 0, len(seen))
	for _, rw := range seen {
		result = append(result, *rw)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Workflow.ID < result[j].Workflow.ID
	})

	return result, nil
}

// ListPhases returns all available phase templates from all sources.
func (r *Resolver) ListPhases() ([]ResolvedPhase, error) {
	seen := make(map[string]*ResolvedPhase)

	// Scan file-based sources (higher priority first)
	sources := []struct {
		dir    string
		subdir string
		source Source
	}{
		{r.personalDir, "phases", SourcePersonalGlobal},
		{r.localDir, "phases", SourceProjectLocal},
		{r.projectDir, "phases", SourceProject},
	}

	for _, s := range sources {
		if s.dir == "" {
			continue
		}
		dir := filepath.Join(s.dir, s.subdir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue // Directory doesn't exist
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
				continue
			}

			id := strings.TrimSuffix(entry.Name(), ".yaml")
			if _, exists := seen[id]; exists {
				continue // Already found at higher priority
			}

			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}

			phase, err := parsePhaseYAML(data)
			if err != nil {
				slog.Warn("failed to parse phase", "path", path, "error", err)
				continue
			}

			seen[id] = &ResolvedPhase{
				Phase:    phase,
				Source:   s.source,
				FilePath: path,
			}
		}
	}

	// Add embedded phases
	if r.embedded {
		embeddedIDs, err := r.listEmbeddedPhaseIDs()
		if err != nil {
			slog.Warn("failed to list embedded phases", "error", err)
		} else {
			for _, id := range embeddedIDs {
				if _, exists := seen[id]; exists {
					continue // Shadowed by file-based
				}
				phase, err := r.readEmbeddedPhase(id)
				if err != nil {
					continue
				}
				seen[id] = &ResolvedPhase{
					Phase:  phase,
					Source: SourceEmbedded,
				}
			}
		}
	}

	// Convert to sorted slice
	result := make([]ResolvedPhase, 0, len(seen))
	for _, rp := range seen {
		result = append(result, *rp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Phase.ID < result[j].Phase.ID
	})

	return result, nil
}

// readEmbeddedWorkflow reads a workflow from embedded templates.
func (r *Resolver) readEmbeddedWorkflow(id string) (*Workflow, error) {
	path := fmt.Sprintf("workflows/%s.yaml", id)
	data, err := templates.Workflows.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseWorkflowYAML(data)
}

// readEmbeddedPhase reads a phase template from embedded templates.
func (r *Resolver) readEmbeddedPhase(id string) (*PhaseTemplate, error) {
	path := fmt.Sprintf("phases/%s.yaml", id)
	data, err := templates.Phases.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parsePhaseYAML(data)
}

// listEmbeddedWorkflowIDs returns all embedded workflow IDs.
func (r *Resolver) listEmbeddedWorkflowIDs() ([]string, error) {
	entries, err := templates.Workflows.ReadDir("workflows")
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".yaml"))
	}
	return ids, nil
}

// listEmbeddedPhaseIDs returns all embedded phase template IDs.
func (r *Resolver) listEmbeddedPhaseIDs() ([]string, error) {
	entries, err := templates.Phases.ReadDir("phases")
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(entry.Name(), ".yaml"))
	}
	return ids, nil
}

// parseWorkflowYAML parses YAML data into a Workflow.
func parseWorkflowYAML(data []byte) (*Workflow, error) {
	var wf workflowYAML
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("parse workflow YAML: %w", err)
	}

	workflow := &Workflow{
		ID:              wf.ID,
		Name:            wf.Name,
		Description:     wf.Description,
		WorkflowType:    WorkflowType(wf.WorkflowType),
		DefaultModel:    wf.DefaultModel,
		DefaultThinking: wf.DefaultThinking,
		BasedOn:         wf.BasedOn,
	}

	// Convert phases
	for _, p := range wf.Phases {
		wp := WorkflowPhase{
			WorkflowID:      wf.ID,
			PhaseTemplateID: p.Template,
			Sequence:        p.Sequence,
			DependsOn:       p.DependsOn,
			ModelOverride:   p.Model,
		}
		if p.MaxIterations > 0 {
			mi := p.MaxIterations
			wp.MaxIterationsOverride = &mi
		}
		if p.Thinking != nil {
			wp.ThinkingOverride = p.Thinking
		}
		if p.GateType != "" {
			wp.GateTypeOverride = GateType(p.GateType)
		}
		if p.Condition != "" {
			wp.Condition = p.Condition
		}
		workflow.Phases = append(workflow.Phases, wp)
	}

	// Convert triggers
	for _, t := range wf.Triggers {
		wt := WorkflowTrigger{
			Event:   WorkflowTriggerEvent(t.Event),
			AgentID: t.AgentID,
			Mode:    GateMode(t.Mode),
			Enabled: t.Enabled,
		}
		workflow.Triggers = append(workflow.Triggers, wt)
	}

	// Convert variables
	for _, v := range wf.Variables {
		wv := WorkflowVariable{
			WorkflowID:   wf.ID,
			Name:         v.Name,
			Description:  v.Description,
			SourceType:   VariableSourceType(v.SourceType),
			SourceConfig: v.SourceConfig,
			Required:     v.Required,
			DefaultValue: v.DefaultValue,
		}
		workflow.Variables = append(workflow.Variables, wv)
	}

	return workflow, nil
}

// parsePhaseYAML parses YAML data into a PhaseTemplate.
func parsePhaseYAML(data []byte) (*PhaseTemplate, error) {
	var pt phaseYAML
	if err := yaml.Unmarshal(data, &pt); err != nil {
		return nil, fmt.Errorf("parse phase YAML: %w", err)
	}

	phase := &PhaseTemplate{
		ID:               pt.ID,
		Name:             pt.Name,
		Description:      pt.Description,
		AgentID:          pt.AgentID,   // Agent reference (executor)
		SubAgents:        pt.SubAgents, // Sub-agents (JSON array)
		PromptSource:     PromptSource(pt.PromptSource),
		PromptPath:       pt.PromptPath,
		PromptContent:    pt.PromptContent,
		InputVariables:   pt.InputVariables,
		OutputSchema:     pt.OutputSchema,
		ProducesArtifact: pt.ProducesArtifact,
		ArtifactType:     pt.ArtifactType,
		MaxIterations:    pt.MaxIterations,
		GateType:         GateType(pt.GateType),
		Checkpoint:       pt.Checkpoint,
		RetryFromPhase:   pt.RetryFromPhase,
		RetryPromptPath:  pt.RetryPromptPath,
		ClaudeConfig:     pt.ClaudeConfig,
	}

	if pt.Thinking != nil {
		phase.ThinkingEnabled = pt.Thinking
	}

	// Set defaults
	if phase.PromptSource == "" {
		phase.PromptSource = PromptSourceEmbedded
	}
	if phase.MaxIterations == 0 {
		phase.MaxIterations = 20
	}
	if phase.GateType == "" {
		phase.GateType = GateAuto
	}

	return phase, nil
}

// workflowYAML is the YAML structure for workflow files.
type workflowYAML struct {
	ID              string              `yaml:"id"`
	Name            string              `yaml:"name"`
	Description     string              `yaml:"description,omitempty"`
	WorkflowType    string              `yaml:"workflow_type,omitempty"`
	DefaultModel    string              `yaml:"default_model,omitempty"`
	DefaultThinking bool                `yaml:"default_thinking,omitempty"`
	BasedOn         string              `yaml:"based_on,omitempty"`
	Phases          []workflowPhaseYAML `yaml:"phases,omitempty"`
	Variables       []variableYAML      `yaml:"variables,omitempty"`
	Triggers        []workflowTriggerYAML `yaml:"triggers,omitempty"`
}

type workflowTriggerYAML struct {
	Event   string `yaml:"event"`
	AgentID string `yaml:"agent_id"`
	Mode    string `yaml:"mode,omitempty"`
	Enabled bool   `yaml:"enabled,omitempty"`
}

type workflowPhaseYAML struct {
	Template      string   `yaml:"template"`
	Sequence      int      `yaml:"sequence"`
	DependsOn     []string `yaml:"depends_on,omitempty"`
	MaxIterations int      `yaml:"max_iterations,omitempty"`
	Model         string   `yaml:"model,omitempty"`
	Thinking      *bool    `yaml:"thinking,omitempty"`
	GateType      string   `yaml:"gate_type,omitempty"`
	Condition     string   `yaml:"condition,omitempty"`
}

type variableYAML struct {
	Name         string `yaml:"name"`
	Description  string `yaml:"description,omitempty"`
	SourceType   string `yaml:"source_type"`
	SourceConfig string `yaml:"source_config,omitempty"`
	Required     bool   `yaml:"required,omitempty"`
	DefaultValue string `yaml:"default_value,omitempty"`
}

// phaseYAML is the YAML structure for phase template files.
type phaseYAML struct {
	ID          string `yaml:"id"`
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`

	// Agent configuration (WHO runs this phase)
	AgentID   string `yaml:"agent_id,omitempty"`   // References agents.id
	SubAgents string `yaml:"sub_agents,omitempty"` // JSON array of agent IDs

	// Prompt configuration
	PromptSource  string   `yaml:"prompt_source,omitempty"`
	PromptPath    string   `yaml:"prompt_path,omitempty"`
	PromptContent string   `yaml:"prompt_content,omitempty"`

	// Contract
	InputVariables   []string `yaml:"input_variables,omitempty"`
	OutputSchema     string   `yaml:"output_schema,omitempty"`
	OutputVarName    string   `yaml:"output_var_name,omitempty"`
	OutputType       string   `yaml:"output_type,omitempty"`
	ProducesArtifact bool     `yaml:"produces_artifact,omitempty"`
	ArtifactType     string   `yaml:"artifact_type,omitempty"`

	// Execution config
	MaxIterations  int    `yaml:"max_iterations,omitempty"`
	Thinking       *bool  `yaml:"thinking,omitempty"` // Phase-level thinking
	GateType       string `yaml:"gate_type,omitempty"`
	Checkpoint     bool   `yaml:"checkpoint,omitempty"`
	RetryFromPhase string `yaml:"retry_from_phase,omitempty"`
	RetryPromptPath string `yaml:"retry_prompt_path,omitempty"`
	QualityChecks  string `yaml:"quality_checks,omitempty"`
	ClaudeConfig   string `yaml:"claude_config,omitempty"`
}
