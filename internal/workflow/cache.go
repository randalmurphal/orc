package workflow

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// CacheService manages the workflow/phase cache in the database.
// Files are the source of truth; DB is a fast runtime cache.
type CacheService struct {
	resolver *Resolver
	pdb      *db.ProjectDB
}

// NewCacheService creates a new cache service.
func NewCacheService(resolver *Resolver, pdb *db.ProjectDB) *CacheService {
	return &CacheService{
		resolver: resolver,
		pdb:      pdb,
	}
}

// NewCacheServiceFromOrcDir creates a cache service for a project.
func NewCacheServiceFromOrcDir(orcDir string, pdb *db.ProjectDB) *CacheService {
	return NewCacheService(
		NewResolverFromOrcDir(orcDir),
		pdb,
	)
}

// SyncResult contains the results of a sync operation.
type SyncResult struct {
	WorkflowsAdded   int      `json:"workflows_added"`
	WorkflowsUpdated int      `json:"workflows_updated"`
	PhasesAdded      int      `json:"phases_added"`
	PhasesUpdated    int      `json:"phases_updated"`
	Errors           []string `json:"errors,omitempty"`
}

// SyncAll synchronizes all workflows and phases from files to database.
func (c *CacheService) SyncAll() (*SyncResult, error) {
	result := &SyncResult{}

	// Sync phases first (workflows depend on them)
	phases, err := c.resolver.ListPhases()
	if err != nil {
		return nil, fmt.Errorf("list phases: %w", err)
	}

	for _, rp := range phases {
		dbPhase := workflowPhaseToDBPhase(rp.Phase, rp.Source)
		existing, err := c.pdb.GetPhaseTemplate(rp.Phase.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("get phase %s: %v", rp.Phase.ID, err))
			continue
		}

		if existing == nil {
			if err := c.pdb.SavePhaseTemplate(dbPhase); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("save phase %s: %v", rp.Phase.ID, err))
				continue
			}
			result.PhasesAdded++
		} else {
			// Check if file is newer (for file-based sources)
			if c.shouldUpdatePhase(existing, rp) {
				if err := c.pdb.SavePhaseTemplate(dbPhase); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("update phase %s: %v", rp.Phase.ID, err))
					continue
				}
				result.PhasesUpdated++
			}
		}
	}

	// Sync workflows
	workflows, err := c.resolver.ListWorkflows()
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	for _, rw := range workflows {
		dbWorkflow := workflowToDBWorkflow(rw.Workflow, rw.Source)
		existing, err := c.pdb.GetWorkflow(rw.Workflow.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("get workflow %s: %v", rw.Workflow.ID, err))
			continue
		}

		if existing == nil {
			if err := c.saveWorkflowWithRelations(rw.Workflow, dbWorkflow); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("save workflow %s: %v", rw.Workflow.ID, err))
				continue
			}
			result.WorkflowsAdded++
		} else {
			// Check if file is newer (for file-based sources)
			if c.shouldUpdateWorkflow(existing, rw) {
				if err := c.saveWorkflowWithRelations(rw.Workflow, dbWorkflow); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("update workflow %s: %v", rw.Workflow.ID, err))
					continue
				}
				result.WorkflowsUpdated++
			}
		}
	}

	return result, nil
}

// SyncWorkflow synchronizes a single workflow from files to database.
func (c *CacheService) SyncWorkflow(id string) error {
	rw, err := c.resolver.ResolveWorkflow(id)
	if err != nil {
		return fmt.Errorf("resolve workflow %s: %w", id, err)
	}

	dbWorkflow := workflowToDBWorkflow(rw.Workflow, rw.Source)
	return c.saveWorkflowWithRelations(rw.Workflow, dbWorkflow)
}

// SyncPhase synchronizes a single phase from files to database.
func (c *CacheService) SyncPhase(id string) error {
	rp, err := c.resolver.ResolvePhase(id)
	if err != nil {
		return fmt.Errorf("resolve phase %s: %w", id, err)
	}

	dbPhase := workflowPhaseToDBPhase(rp.Phase, rp.Source)
	return c.pdb.SavePhaseTemplate(dbPhase)
}

// IsStale checks if any workflow or phase files are newer than their DB entries.
func (c *CacheService) IsStale() (bool, error) {
	workflows, err := c.resolver.ListWorkflows()
	if err != nil {
		return false, fmt.Errorf("list workflows: %w", err)
	}

	for _, rw := range workflows {
		existing, err := c.pdb.GetWorkflow(rw.Workflow.ID)
		if err != nil {
			return true, nil // Error = stale
		}
		if existing == nil {
			return true, nil // Missing = stale
		}
		if c.shouldUpdateWorkflow(existing, rw) {
			return true, nil
		}
	}

	phases, err := c.resolver.ListPhases()
	if err != nil {
		return false, fmt.Errorf("list phases: %w", err)
	}

	for _, rp := range phases {
		existing, err := c.pdb.GetPhaseTemplate(rp.Phase.ID)
		if err != nil {
			return true, nil
		}
		if existing == nil {
			return true, nil
		}
		if c.shouldUpdatePhase(existing, rp) {
			return true, nil
		}
	}

	return false, nil
}

// ForceSync forces a sync of all workflows and phases, including embedded.
// This is used when the binary is updated and we want to refresh DB content.
func (c *CacheService) ForceSync() (*SyncResult, error) {
	result := &SyncResult{}

	// Sync phases first (workflows depend on them)
	phases, err := c.resolver.ListPhases()
	if err != nil {
		return nil, fmt.Errorf("list phases: %w", err)
	}

	for _, rp := range phases {
		dbPhase := workflowPhaseToDBPhase(rp.Phase, rp.Source)
		existing, err := c.pdb.GetPhaseTemplate(rp.Phase.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("get phase %s: %v", rp.Phase.ID, err))
			continue
		}

		if existing == nil {
			if err := c.pdb.SavePhaseTemplate(dbPhase); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("save phase %s: %v", rp.Phase.ID, err))
				continue
			}
			result.PhasesAdded++
		} else {
			// Force update regardless of source
			if err := c.pdb.SavePhaseTemplate(dbPhase); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("update phase %s: %v", rp.Phase.ID, err))
				continue
			}
			result.PhasesUpdated++
		}
	}

	// Sync workflows
	workflows, err := c.resolver.ListWorkflows()
	if err != nil {
		return nil, fmt.Errorf("list workflows: %w", err)
	}

	for _, rw := range workflows {
		dbWorkflow := workflowToDBWorkflow(rw.Workflow, rw.Source)
		existing, err := c.pdb.GetWorkflow(rw.Workflow.ID)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("get workflow %s: %v", rw.Workflow.ID, err))
			continue
		}

		if existing == nil {
			if err := c.saveWorkflowWithRelations(rw.Workflow, dbWorkflow); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("save workflow %s: %v", rw.Workflow.ID, err))
				continue
			}
			result.WorkflowsAdded++
		} else {
			// Force update regardless of source
			if err := c.saveWorkflowWithRelations(rw.Workflow, dbWorkflow); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("update workflow %s: %v", rw.Workflow.ID, err))
				continue
			}
			result.WorkflowsUpdated++
		}
	}

	return result, nil
}

// EnsureSynced checks staleness and syncs if needed.
// Returns true if sync was performed.
func (c *CacheService) EnsureSynced() (bool, error) {
	stale, err := c.IsStale()
	if err != nil {
		return false, err
	}

	if !stale {
		return false, nil
	}

	result, err := c.SyncAll()
	if err != nil {
		return false, err
	}

	if len(result.Errors) > 0 {
		slog.Warn("sync completed with errors",
			"workflows_added", result.WorkflowsAdded,
			"workflows_updated", result.WorkflowsUpdated,
			"phases_added", result.PhasesAdded,
			"phases_updated", result.PhasesUpdated,
			"errors", result.Errors)
	} else {
		slog.Debug("sync completed",
			"workflows_added", result.WorkflowsAdded,
			"workflows_updated", result.WorkflowsUpdated,
			"phases_added", result.PhasesAdded,
			"phases_updated", result.PhasesUpdated)
	}

	return true, nil
}

// saveWorkflowWithRelations saves a workflow and its phases/variables to the DB.
func (c *CacheService) saveWorkflowWithRelations(wf *Workflow, dbWorkflow *db.Workflow) error {
	if err := c.pdb.SaveWorkflow(dbWorkflow); err != nil {
		return fmt.Errorf("save workflow: %w", err)
	}

	// Save phases
	for _, phase := range wf.Phases {
		dbPhase := workflowPhaseToDBWorkflowPhase(&phase)
		if err := c.pdb.SaveWorkflowPhase(dbPhase); err != nil {
			return fmt.Errorf("save workflow phase %s: %w", phase.PhaseTemplateID, err)
		}
	}

	// Save variables
	for _, variable := range wf.Variables {
		dbVar := workflowVariableToDBVariable(&variable)
		if err := c.pdb.SaveWorkflowVariable(dbVar); err != nil {
			return fmt.Errorf("save workflow variable %s: %w", variable.Name, err)
		}
	}

	return nil
}

// shouldUpdateWorkflow checks if a workflow should be updated based on file modification time.
func (c *CacheService) shouldUpdateWorkflow(existing *db.Workflow, rw ResolvedWorkflow) bool {
	// For embedded sources, don't update if already exists (idempotent seeding).
	// Embedded templates are versioned with the binary - explicit sync or version
	// bump handles updates to embedded content.
	if rw.Source == SourceEmbedded {
		return false // Already exists, don't update
	}

	// For file-based, check modification time
	if rw.FilePath != "" {
		info, err := os.Stat(rw.FilePath)
		if err != nil {
			return true // Can't stat = update
		}
		return info.ModTime().After(existing.UpdatedAt)
	}

	return false
}

// shouldUpdatePhase checks if a phase should be updated based on file modification time.
func (c *CacheService) shouldUpdatePhase(existing *db.PhaseTemplate, rp ResolvedPhase) bool {
	// For embedded sources, don't update if already exists (idempotent seeding).
	// Embedded templates are versioned with the binary - explicit sync or version
	// bump handles updates to embedded content.
	if rp.Source == SourceEmbedded {
		return false // Already exists, don't update
	}

	// For file-based, check modification time
	if rp.FilePath != "" {
		info, err := os.Stat(rp.FilePath)
		if err != nil {
			return true
		}
		return info.ModTime().After(existing.UpdatedAt)
	}

	return false
}

// workflowToDBWorkflow converts a workflow.Workflow to db.Workflow.
func workflowToDBWorkflow(wf *Workflow, source Source) *db.Workflow {
	return &db.Workflow{
		ID:              wf.ID,
		Name:            wf.Name,
		Description:     wf.Description,
		WorkflowType:    string(wf.WorkflowType),
		DefaultModel:    wf.DefaultModel,
		DefaultThinking: wf.DefaultThinking,
		IsBuiltin:       source == SourceEmbedded,
		BasedOn:         wf.BasedOn,
		CreatedAt:       wf.CreatedAt,
		UpdatedAt:       time.Now(),
	}
}

// workflowPhaseToDBPhase converts a workflow.PhaseTemplate to db.PhaseTemplate.
func workflowPhaseToDBPhase(pt *PhaseTemplate, source Source) *db.PhaseTemplate {
	var inputVarsJSON string
	if len(pt.InputVariables) > 0 {
		data, _ := json.Marshal(pt.InputVariables)
		inputVarsJSON = string(data)
	}

	return &db.PhaseTemplate{
		ID:               pt.ID,
		Name:             pt.Name,
		Description:      pt.Description,
		PromptSource:     string(pt.PromptSource),
		PromptContent:    pt.PromptContent,
		PromptPath:       pt.PromptPath,
		InputVariables:   inputVarsJSON,
		OutputSchema:     pt.OutputSchema,
		ProducesArtifact: pt.ProducesArtifact,
		ArtifactType:     pt.ArtifactType,
		MaxIterations:    pt.MaxIterations,
		ModelOverride:    pt.ModelOverride,
		ThinkingEnabled:  pt.ThinkingEnabled,
		GateType:         string(pt.GateType),
		Checkpoint:       pt.Checkpoint,
		RetryFromPhase:   pt.RetryFromPhase,
		RetryPromptPath:  pt.RetryPromptPath,
		ClaudeConfig:     pt.ClaudeConfig,
		SystemPrompt:     pt.SystemPrompt,
		IsBuiltin:        source == SourceEmbedded,
		CreatedAt:        pt.CreatedAt,
		UpdatedAt:        time.Now(),
	}
}

// workflowPhaseToDBWorkflowPhase converts a workflow.WorkflowPhase to db.WorkflowPhase.
func workflowPhaseToDBWorkflowPhase(wp *WorkflowPhase) *db.WorkflowPhase {
	var dependsOnJSON string
	if len(wp.DependsOn) > 0 {
		data, _ := json.Marshal(wp.DependsOn)
		dependsOnJSON = string(data)
	}

	dbPhase := &db.WorkflowPhase{
		ID:              wp.ID,
		WorkflowID:      wp.WorkflowID,
		PhaseTemplateID: wp.PhaseTemplateID,
		Sequence:        wp.Sequence,
		DependsOn:       dependsOnJSON,
		ModelOverride:   wp.ModelOverride,
		Condition:       wp.Condition,
	}

	if wp.MaxIterationsOverride != nil {
		dbPhase.MaxIterationsOverride = wp.MaxIterationsOverride
	}
	if wp.ThinkingOverride != nil {
		dbPhase.ThinkingOverride = wp.ThinkingOverride
	}
	if wp.GateTypeOverride != "" {
		dbPhase.GateTypeOverride = string(wp.GateTypeOverride)
	}
	if wp.ClaudeConfigOverride != "" {
		dbPhase.ClaudeConfigOverride = wp.ClaudeConfigOverride
	}

	return dbPhase
}

// workflowVariableToDBVariable converts a workflow.WorkflowVariable to db.WorkflowVariable.
func workflowVariableToDBVariable(wv *WorkflowVariable) *db.WorkflowVariable {
	return &db.WorkflowVariable{
		ID:           wv.ID,
		WorkflowID:   wv.WorkflowID,
		Name:         wv.Name,
		Description:  wv.Description,
		SourceType:   string(wv.SourceType),
		SourceConfig: wv.SourceConfig,
		Required:     wv.Required,
		DefaultValue: wv.DefaultValue,
	}
}

// DBPhaseToWorkflowPhase converts a db.PhaseTemplate to workflow.PhaseTemplate.
func DBPhaseToWorkflowPhase(dbPt *db.PhaseTemplate) *PhaseTemplate {
	pt := &PhaseTemplate{
		ID:               dbPt.ID,
		Name:             dbPt.Name,
		Description:      dbPt.Description,
		PromptSource:     PromptSource(dbPt.PromptSource),
		PromptContent:    dbPt.PromptContent,
		PromptPath:       dbPt.PromptPath,
		OutputSchema:     dbPt.OutputSchema,
		ProducesArtifact: dbPt.ProducesArtifact,
		ArtifactType:     dbPt.ArtifactType,
		MaxIterations:    dbPt.MaxIterations,
		ModelOverride:    dbPt.ModelOverride,
		ThinkingEnabled:  dbPt.ThinkingEnabled,
		GateType:         GateType(dbPt.GateType),
		Checkpoint:       dbPt.Checkpoint,
		RetryFromPhase:   dbPt.RetryFromPhase,
		RetryPromptPath:  dbPt.RetryPromptPath,
		ClaudeConfig:     dbPt.ClaudeConfig,
		SystemPrompt:     dbPt.SystemPrompt,
		IsBuiltin:        dbPt.IsBuiltin,
		CreatedAt:        dbPt.CreatedAt,
		UpdatedAt:        dbPt.UpdatedAt,
	}

	// Parse input variables
	if dbPt.InputVariables != "" {
		_ = json.Unmarshal([]byte(dbPt.InputVariables), &pt.InputVariables)
	}

	return pt
}

// DBWorkflowToWorkflow converts a db.Workflow to workflow.Workflow.
func DBWorkflowToWorkflow(dbWf *db.Workflow) *Workflow {
	return &Workflow{
		ID:              dbWf.ID,
		Name:            dbWf.Name,
		Description:     dbWf.Description,
		WorkflowType:    WorkflowType(dbWf.WorkflowType),
		DefaultModel:    dbWf.DefaultModel,
		DefaultThinking: dbWf.DefaultThinking,
		IsBuiltin:       dbWf.IsBuiltin,
		BasedOn:         dbWf.BasedOn,
		CreatedAt:       dbWf.CreatedAt,
		UpdatedAt:       dbWf.UpdatedAt,
	}
}
