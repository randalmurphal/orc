package storage

import (
	"github.com/randalmurphal/orc/internal/db"
)

// --------- Phase Template Operations ---------

func (d *DatabaseBackend) SavePhaseTemplate(pt *db.PhaseTemplate) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SavePhaseTemplate(pt)
}

func (d *DatabaseBackend) GetPhaseTemplate(id string) (*db.PhaseTemplate, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetPhaseTemplate(id)
}

func (d *DatabaseBackend) ListPhaseTemplates() ([]*db.PhaseTemplate, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.ListPhaseTemplates()
}

func (d *DatabaseBackend) DeletePhaseTemplate(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeletePhaseTemplate(id)
}

// --------- Workflow Operations ---------

func (d *DatabaseBackend) SaveWorkflow(w *db.Workflow) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SaveWorkflow(w)
}

func (d *DatabaseBackend) GetWorkflow(id string) (*db.Workflow, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetWorkflow(id)
}

func (d *DatabaseBackend) ListWorkflows() ([]*db.Workflow, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.ListWorkflows()
}

func (d *DatabaseBackend) DeleteWorkflow(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeleteWorkflow(id)
}

func (d *DatabaseBackend) GetWorkflowPhases(workflowID string) ([]*db.WorkflowPhase, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetWorkflowPhases(workflowID)
}

func (d *DatabaseBackend) SaveWorkflowPhase(wp *db.WorkflowPhase) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SaveWorkflowPhase(wp)
}

func (d *DatabaseBackend) DeleteWorkflowPhase(workflowID, phaseTemplateID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeleteWorkflowPhase(workflowID, phaseTemplateID)
}

func (d *DatabaseBackend) UpdateWorkflowPhasePositions(workflowID string, positions map[string][2]float64) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.UpdateWorkflowPhasePositions(workflowID, positions)
}

func (d *DatabaseBackend) GetWorkflowVariables(workflowID string) ([]*db.WorkflowVariable, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetWorkflowVariables(workflowID)
}

func (d *DatabaseBackend) SaveWorkflowVariable(wv *db.WorkflowVariable) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SaveWorkflowVariable(wv)
}

func (d *DatabaseBackend) DeleteWorkflowVariable(workflowID, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeleteWorkflowVariable(workflowID, name)
}

// --------- Workflow Run Operations ---------

func (d *DatabaseBackend) SaveWorkflowRun(wr *db.WorkflowRun) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SaveWorkflowRun(wr)
}

func (d *DatabaseBackend) GetWorkflowRun(id string) (*db.WorkflowRun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetWorkflowRun(id)
}

func (d *DatabaseBackend) ListWorkflowRuns(opts db.WorkflowRunListOpts) ([]*db.WorkflowRun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.ListWorkflowRuns(opts)
}

func (d *DatabaseBackend) DeleteWorkflowRun(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeleteWorkflowRun(id)
}

func (d *DatabaseBackend) GetNextWorkflowRunID() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.GetNextWorkflowRunID()
}

func (d *DatabaseBackend) GetWorkflowRunPhases(runID string) ([]*db.WorkflowRunPhase, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetWorkflowRunPhases(runID)
}

func (d *DatabaseBackend) SaveWorkflowRunPhase(wrp *db.WorkflowRunPhase) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SaveWorkflowRunPhase(wrp)
}

func (d *DatabaseBackend) UpdatePhaseIterations(runID, phaseID string, iterations int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.UpdatePhaseIterations(runID, phaseID, iterations)
}

func (d *DatabaseBackend) GetRunningWorkflowsByTask() (map[string]*db.WorkflowRun, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetRunningWorkflowsByTask()
}

// --------- Project Command Operations ---------

func (d *DatabaseBackend) SaveProjectCommand(cmd *db.ProjectCommand) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SaveProjectCommand(cmd)
}

func (d *DatabaseBackend) GetProjectCommand(name string) (*db.ProjectCommand, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetProjectCommand(name)
}

func (d *DatabaseBackend) ListProjectCommands() ([]*db.ProjectCommand, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.ListProjectCommands()
}

func (d *DatabaseBackend) GetProjectCommandsMap() (map[string]*db.ProjectCommand, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetProjectCommandsMap()
}

func (d *DatabaseBackend) DeleteProjectCommand(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeleteProjectCommand(name)
}

func (d *DatabaseBackend) SetProjectCommandEnabled(name string, enabled bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SetProjectCommandEnabled(name, enabled)
}
