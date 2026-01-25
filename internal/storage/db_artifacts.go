package storage

import (
	"fmt"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/task"
)

// ============================================================================
// Phase outputs, specs, attachments - things phases produce
// ============================================================================

func (d *DatabaseBackend) SavePhaseOutput(output *PhaseOutputInfo) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbOutput := &db.PhaseOutput{
		ID:              output.ID,
		WorkflowRunID:   output.WorkflowRunID,
		PhaseTemplateID: output.PhaseTemplateID,
		TaskID:          output.TaskID,
		Content:         output.Content,
		ContentHash:     output.ContentHash,
		OutputVarName:   output.OutputVarName,
		ArtifactType:    output.ArtifactType,
		Source:          output.Source,
		Iteration:       output.Iteration,
		CreatedAt:       output.CreatedAt,
		UpdatedAt:       output.UpdatedAt,
	}
	if err := d.db.SavePhaseOutput(dbOutput); err != nil {
		return fmt.Errorf("save phase output: %w", err)
	}
	output.ID = dbOutput.ID
	return nil
}

func (d *DatabaseBackend) GetPhaseOutput(runID, phaseTemplateID string) (*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutput, err := d.db.GetPhaseOutput(runID, phaseTemplateID)
	if err != nil {
		return nil, fmt.Errorf("get phase output: %w", err)
	}
	if dbOutput == nil {
		return nil, nil
	}
	return dbPhaseOutputToInfo(dbOutput), nil
}

func (d *DatabaseBackend) GetPhaseOutputByVarName(runID, varName string) (*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutput, err := d.db.GetPhaseOutputByVarName(runID, varName)
	if err != nil {
		return nil, fmt.Errorf("get phase output by var name: %w", err)
	}
	if dbOutput == nil {
		return nil, nil
	}
	return dbPhaseOutputToInfo(dbOutput), nil
}

func (d *DatabaseBackend) GetAllPhaseOutputs(runID string) ([]*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutputs, err := d.db.GetAllPhaseOutputs(runID)
	if err != nil {
		return nil, fmt.Errorf("get all phase outputs: %w", err)
	}

	outputs := make([]*PhaseOutputInfo, len(dbOutputs))
	for i, dbOutput := range dbOutputs {
		outputs[i] = dbPhaseOutputToInfo(dbOutput)
	}
	return outputs, nil
}

func (d *DatabaseBackend) LoadPhaseOutputsAsMap(runID string) (map[string]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.LoadPhaseOutputsAsMap(runID)
}

func (d *DatabaseBackend) GetSpecForTask(taskID string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.GetSpecForTask(taskID)
}

func (d *DatabaseBackend) GetFullSpecForTask(taskID string) (*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutput, err := d.db.GetFullSpecForTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("get full spec for task: %w", err)
	}
	if dbOutput == nil {
		return nil, nil
	}
	return dbPhaseOutputToInfo(dbOutput), nil
}

func (d *DatabaseBackend) SpecExistsForTask(taskID string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.SpecExistsForTask(taskID)
}

func (d *DatabaseBackend) SaveSpecForTask(taskID, content, source string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SaveSpecForTask(taskID, content, source)
}

func (d *DatabaseBackend) GetPhaseOutputsForTask(taskID string) ([]*PhaseOutputInfo, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOutputs, err := d.db.GetPhaseOutputsForTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("get phase outputs for task: %w", err)
	}

	outputs := make([]*PhaseOutputInfo, len(dbOutputs))
	for i, dbOutput := range dbOutputs {
		outputs[i] = dbPhaseOutputToInfo(dbOutput)
	}
	return outputs, nil
}

func (d *DatabaseBackend) DeletePhaseOutput(runID, phaseTemplateID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeletePhaseOutput(runID, phaseTemplateID)
}

func (d *DatabaseBackend) PhaseOutputExists(runID, phaseTemplateID string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.PhaseOutputExists(runID, phaseTemplateID)
}

func dbPhaseOutputToInfo(dbOutput *db.PhaseOutput) *PhaseOutputInfo {
	return &PhaseOutputInfo{
		ID:              dbOutput.ID,
		WorkflowRunID:   dbOutput.WorkflowRunID,
		PhaseTemplateID: dbOutput.PhaseTemplateID,
		TaskID:          dbOutput.TaskID,
		Content:         dbOutput.Content,
		ContentHash:     dbOutput.ContentHash,
		OutputVarName:   dbOutput.OutputVarName,
		ArtifactType:    dbOutput.ArtifactType,
		Source:          dbOutput.Source,
		Iteration:       dbOutput.Iteration,
		CreatedAt:       dbOutput.CreatedAt,
		UpdatedAt:       dbOutput.UpdatedAt,
	}
}

// ============================================================================
// Attachments
// ============================================================================

func (d *DatabaseBackend) SaveAttachment(taskID, filename, contentType string, data []byte) (*task.Attachment, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	isImage := isImageContentType(contentType)
	dbAttachment := &db.Attachment{
		TaskID:      taskID,
		Filename:    filename,
		ContentType: contentType,
		SizeBytes:   int64(len(data)),
		Data:        data,
		IsImage:     isImage,
	}
	if err := d.db.SaveAttachment(dbAttachment); err != nil {
		return nil, err
	}

	return &task.Attachment{
		Filename:    filename,
		Size:        int64(len(data)),
		ContentType: contentType,
		CreatedAt:   dbAttachment.CreatedAt,
		IsImage:     isImage,
	}, nil
}

func (d *DatabaseBackend) GetAttachment(taskID, filename string) (*task.Attachment, []byte, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbAttachment, err := d.db.GetAttachment(taskID, filename)
	if err != nil {
		return nil, nil, err
	}
	if dbAttachment == nil {
		return nil, nil, fmt.Errorf("attachment %s not found", filename)
	}

	attachment := &task.Attachment{
		Filename:    dbAttachment.Filename,
		Size:        dbAttachment.SizeBytes,
		ContentType: dbAttachment.ContentType,
		CreatedAt:   dbAttachment.CreatedAt,
		IsImage:     dbAttachment.IsImage,
	}
	return attachment, dbAttachment.Data, nil
}

func (d *DatabaseBackend) ListAttachments(taskID string) ([]*task.Attachment, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbAttachments, err := d.db.ListAttachments(taskID)
	if err != nil {
		return nil, err
	}

	attachments := make([]*task.Attachment, len(dbAttachments))
	for i, a := range dbAttachments {
		attachments[i] = &task.Attachment{
			Filename:    a.Filename,
			Size:        a.SizeBytes,
			ContentType: a.ContentType,
			CreatedAt:   a.CreatedAt,
			IsImage:     a.IsImage,
		}
	}
	return attachments, nil
}

func (d *DatabaseBackend) DeleteAttachment(taskID, filename string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeleteAttachment(taskID, filename)
}

func isImageContentType(contentType string) bool {
	switch contentType {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml":
		return true
	default:
		return false
	}
}
