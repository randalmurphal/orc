package storage

import (
	"context"
	"encoding/json"
	"fmt"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
)

// SaveInitiative saves an initiative to the database.
func (d *DatabaseBackend) SaveInitiative(i *initiative.Initiative) error {
	return d.SaveInitiativeCtx(context.Background(), i)
}

// SaveInitiativeCtx saves an initiative to the database with context support.
func (d *DatabaseBackend) SaveInitiativeCtx(ctx context.Context, i *initiative.Initiative) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbInit := initiativeToDBInitiative(i)

	return d.db.RunInTx(ctx, func(tx *db.TxOps) error {
		if err := db.SaveInitiativeTx(tx, dbInit); err != nil {
			return fmt.Errorf("save initiative: %w", err)
		}

		if err := db.ClearInitiativeDecisionsTx(tx, i.ID); err != nil {
			return fmt.Errorf("clear initiative decisions: %w", err)
		}
		for _, decision := range i.Decisions {
			dbDecision := &db.InitiativeDecision{
				ID:           decision.ID,
				InitiativeID: i.ID,
				DecidedAt:    decision.Date,
				DecidedBy:    decision.By,
				Decision:     decision.Decision,
				Rationale:    decision.Rationale,
			}
			if err := db.AddInitiativeDecisionTx(tx, dbDecision); err != nil {
				return fmt.Errorf("save decision %s: %w", decision.ID, err)
			}
		}

		if err := db.ClearInitiativeTasksTx(tx, i.ID); err != nil {
			return fmt.Errorf("clear initiative tasks: %w", err)
		}
		for idx, taskRef := range i.Tasks {
			if err := db.AddTaskToInitiativeTx(tx, i.ID, taskRef.ID, idx); err != nil {
				return fmt.Errorf("add task %s to initiative: %w", taskRef.ID, err)
			}
		}

		if err := db.ClearInitiativeDependenciesTx(tx, i.ID); err != nil {
			return fmt.Errorf("clear initiative dependencies: %w", err)
		}
		for _, depID := range i.BlockedBy {
			if err := db.AddInitiativeDependencyTx(tx, i.ID, depID); err != nil {
				return fmt.Errorf("add initiative dependency %s: %w", depID, err)
			}
		}

		if err := db.ClearInitiativeCriteriaTx(tx, i.ID); err != nil {
			return fmt.Errorf("clear initiative criteria: %w", err)
		}
		for _, c := range i.Criteria {
			taskIDsJSON, err := json.Marshal(c.TaskIDs)
			if err != nil {
				return fmt.Errorf("marshal criterion task IDs: %w", err)
			}
			dbCriterion := &db.InitiativeCriterion{
				ID:           c.ID,
				InitiativeID: i.ID,
				Description:  c.Description,
				Status:       c.Status,
				TaskIDs:      string(taskIDsJSON),
				VerifiedAt:   c.VerifiedAt,
				VerifiedBy:   c.VerifiedBy,
				Evidence:     c.Evidence,
			}
			if err := db.AddInitiativeCriterionTx(tx, dbCriterion); err != nil {
				return fmt.Errorf("save criterion %s: %w", c.ID, err)
			}
		}

		return nil
	})
}

// LoadInitiative loads an initiative from the database.
func (d *DatabaseBackend) LoadInitiative(id string) (*initiative.Initiative, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbInit, err := d.db.GetInitiative(id)
	if err != nil {
		return nil, fmt.Errorf("get initiative: %w", err)
	}
	if dbInit == nil {
		return nil, fmt.Errorf("initiative %s not found", id)
	}

	i := dbInitiativeToInitiative(dbInit)

	dbDecisions, err := d.db.GetInitiativeDecisions(id)
	if err != nil {
		d.logger.Printf("warning: failed to get decisions: %v", err)
	} else {
		for _, dbDec := range dbDecisions {
			i.Decisions = append(i.Decisions, initiative.Decision{
				ID:        dbDec.ID,
				Date:      dbDec.DecidedAt,
				By:        dbDec.DecidedBy,
				Decision:  dbDec.Decision,
				Rationale: dbDec.Rationale,
			})
		}
	}

	taskIDs, err := d.db.GetInitiativeTasks(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative tasks: %v", err)
	} else {
		for _, taskID := range taskIDs {
			dbTask, err := d.db.GetTask(taskID)
			if err != nil || dbTask == nil {
				continue
			}
			i.Tasks = append(i.Tasks, initiative.TaskRef{
				ID:     taskID,
				Title:  dbTask.Title,
				Status: dbTask.Status,
			})
		}
	}

	deps, err := d.db.GetInitiativeDependencies(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative dependencies: %v", err)
	} else {
		i.BlockedBy = deps
	}

	dependents, err := d.db.GetInitiativeDependents(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative dependents: %v", err)
	} else {
		i.Blocks = dependents
	}

	dbCriteria, err := d.db.GetInitiativeCriteria(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative criteria: %v", err)
	} else {
		for _, dbc := range dbCriteria {
			c := dbCriterionToInitiativeCriterion(&dbc)
			i.Criteria = append(i.Criteria, c)
		}
		i.RecomputeCriterionSeq()
	}

	return i, nil
}

// LoadAllInitiatives loads all initiatives from the database.
func (d *DatabaseBackend) LoadAllInitiatives() ([]*initiative.Initiative, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbInits, err := d.db.ListInitiatives(db.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("list initiatives: %w", err)
	}

	allDecisions, err := d.db.GetAllInitiativeDecisions()
	if err != nil {
		d.logger.Printf("warning: failed to batch load decisions: %v", err)
		allDecisions = make(map[string][]db.InitiativeDecision)
	}

	allTaskRefs, err := d.db.GetAllInitiativeTaskRefs()
	if err != nil {
		d.logger.Printf("warning: failed to batch load task refs: %v", err)
		allTaskRefs = make(map[string][]db.InitiativeTaskRef)
	}

	allDeps, err := d.db.GetAllInitiativeDependencies()
	if err != nil {
		d.logger.Printf("warning: failed to batch load dependencies: %v", err)
		allDeps = make(map[string][]string)
	}

	allDependents, err := d.db.GetAllInitiativeDependents()
	if err != nil {
		d.logger.Printf("warning: failed to batch load dependents: %v", err)
		allDependents = make(map[string][]string)
	}

	allCriteria, err := d.db.GetAllInitiativeCriteria()
	if err != nil {
		d.logger.Printf("warning: failed to batch load criteria: %v", err)
		allCriteria = make(map[string][]db.InitiativeCriterion)
	}

	initiatives := make([]*initiative.Initiative, 0, len(dbInits))
	for _, dbInit := range dbInits {
		i := dbInitiativeToInitiative(&dbInit)

		if dbDecisions, ok := allDecisions[i.ID]; ok {
			for _, dbDec := range dbDecisions {
				i.Decisions = append(i.Decisions, initiative.Decision{
					ID:        dbDec.ID,
					Date:      dbDec.DecidedAt,
					By:        dbDec.DecidedBy,
					Decision:  dbDec.Decision,
					Rationale: dbDec.Rationale,
				})
			}
		}

		if taskRefs, ok := allTaskRefs[i.ID]; ok {
			for _, ref := range taskRefs {
				i.Tasks = append(i.Tasks, initiative.TaskRef{
					ID:     ref.TaskID,
					Title:  ref.Title,
					Status: ref.Status,
				})
			}
		}

		if deps, ok := allDeps[i.ID]; ok {
			i.BlockedBy = deps
		}

		if dependents, ok := allDependents[i.ID]; ok {
			i.Blocks = dependents
		}

		if dbCriteria, ok := allCriteria[i.ID]; ok {
			for _, dbc := range dbCriteria {
				c := dbCriterionToInitiativeCriterion(&dbc)
				i.Criteria = append(i.Criteria, c)
			}
			i.RecomputeCriterionSeq()
		}

		initiatives = append(initiatives, i)
	}

	return initiatives, nil
}

// DeleteInitiative removes an initiative from the database.
func (d *DatabaseBackend) DeleteInitiative(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.db.DeleteInitiative(id); err != nil {
		return fmt.Errorf("delete initiative: %w", err)
	}
	return nil
}

// InitiativeExists checks if an initiative exists in the database.
func (d *DatabaseBackend) InitiativeExists(id string) (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	init, err := d.db.GetInitiative(id)
	if err != nil {
		return false, fmt.Errorf("check initiative: %w", err)
	}
	return init != nil, nil
}

// GetNextInitiativeID generates the next initiative ID from the database.
func (d *DatabaseBackend) GetNextInitiativeID() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.GetNextInitiativeID()
}

// ============================================================================
// Initiative conversion helpers
// ============================================================================

func initiativeToDBInitiative(i *initiative.Initiative) *db.Initiative {
	return &db.Initiative{
		ID:               i.ID,
		Title:            i.Title,
		Status:           string(i.Status),
		OwnerInitials:    i.Owner.Initials,
		OwnerDisplayName: i.Owner.DisplayName,
		OwnerEmail:       i.Owner.Email,
		Vision:           i.Vision,
		BranchBase:       i.BranchBase,
		BranchPrefix:     i.BranchPrefix,
		MergeStatus:      i.MergeStatus,
		MergeCommit:      i.MergeCommit,
		CreatedAt:        i.CreatedAt,
		UpdatedAt:        i.UpdatedAt,
	}
}

func dbCriterionToProtoCriterion(dbc *db.InitiativeCriterion) *orcv1.Criterion {
	var taskIDs []string
	if dbc.TaskIDs != "" {
		_ = json.Unmarshal([]byte(dbc.TaskIDs), &taskIDs)
	}
	if taskIDs == nil {
		taskIDs = []string{}
	}
	return &orcv1.Criterion{
		Id:          dbc.ID,
		Description: dbc.Description,
		Status:      dbc.Status,
		TaskIds:     taskIDs,
		VerifiedAt:  dbc.VerifiedAt,
		VerifiedBy:  dbc.VerifiedBy,
		Evidence:    dbc.Evidence,
	}
}

func dbCriterionToInitiativeCriterion(dbc *db.InitiativeCriterion) *initiative.Criterion {
	var taskIDs []string
	if dbc.TaskIDs != "" {
		_ = json.Unmarshal([]byte(dbc.TaskIDs), &taskIDs)
	}
	if taskIDs == nil {
		taskIDs = []string{}
	}
	return &initiative.Criterion{
		ID:          dbc.ID,
		Description: dbc.Description,
		Status:      dbc.Status,
		TaskIDs:     taskIDs,
		VerifiedAt:  dbc.VerifiedAt,
		VerifiedBy:  dbc.VerifiedBy,
		Evidence:    dbc.Evidence,
	}
}

func dbInitiativeToInitiative(dbInit *db.Initiative) *initiative.Initiative {
	return &initiative.Initiative{
		ID:     dbInit.ID,
		Title:  dbInit.Title,
		Status: initiative.Status(dbInit.Status),
		Owner: initiative.Identity{
			Initials:    dbInit.OwnerInitials,
			DisplayName: dbInit.OwnerDisplayName,
			Email:       dbInit.OwnerEmail,
		},
		Vision:       dbInit.Vision,
		BranchBase:   dbInit.BranchBase,
		BranchPrefix: dbInit.BranchPrefix,
		MergeStatus:  dbInit.MergeStatus,
		MergeCommit:  dbInit.MergeCommit,
		CreatedAt:    dbInit.CreatedAt,
		UpdatedAt:    dbInit.UpdatedAt,
	}
}

// ============================================================================
// Proto Initiative Operations (orcv1.Initiative)
// ============================================================================

// SaveInitiativeProto saves a proto initiative to the database.
func (d *DatabaseBackend) SaveInitiativeProto(i *orcv1.Initiative) error {
	return d.SaveInitiativeProtoCtx(context.Background(), i)
}

// SaveInitiativeProtoCtx saves a proto initiative with context support.
func (d *DatabaseBackend) SaveInitiativeProtoCtx(ctx context.Context, i *orcv1.Initiative) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbInit := protoInitiativeToDBInitiative(i)

	return d.db.RunInTx(ctx, func(tx *db.TxOps) error {
		if err := db.SaveInitiativeTx(tx, dbInit); err != nil {
			return fmt.Errorf("save initiative: %w", err)
		}

		// Save decisions
		if err := db.ClearInitiativeDecisionsTx(tx, i.Id); err != nil {
			return fmt.Errorf("clear initiative decisions: %w", err)
		}
		for _, decision := range i.Decisions {
			dbDecision := protoDecisionToDBDecision(i.Id, decision)
			if err := db.AddInitiativeDecisionTx(tx, dbDecision); err != nil {
				return fmt.Errorf("save decision %s: %w", decision.Id, err)
			}
		}

		// Save task refs
		if err := db.ClearInitiativeTasksTx(tx, i.Id); err != nil {
			return fmt.Errorf("clear initiative tasks: %w", err)
		}
		for idx, taskRef := range i.Tasks {
			if err := db.AddTaskToInitiativeTx(tx, i.Id, taskRef.Id, idx); err != nil {
				return fmt.Errorf("add task %s to initiative: %w", taskRef.Id, err)
			}
		}

		// Save dependencies
		if err := db.ClearInitiativeDependenciesTx(tx, i.Id); err != nil {
			return fmt.Errorf("clear initiative dependencies: %w", err)
		}
		for _, depID := range i.BlockedBy {
			if err := db.AddInitiativeDependencyTx(tx, i.Id, depID); err != nil {
				return fmt.Errorf("add initiative dependency %s: %w", depID, err)
			}
		}

		// Save criteria
		if err := db.ClearInitiativeCriteriaTx(tx, i.Id); err != nil {
			return fmt.Errorf("clear initiative criteria: %w", err)
		}
		for _, c := range i.Criteria {
			taskIDs := c.TaskIds
			if taskIDs == nil {
				taskIDs = []string{}
			}
			taskIDsJSON, err := json.Marshal(taskIDs)
			if err != nil {
				return fmt.Errorf("marshal criterion task IDs: %w", err)
			}
			dbCriterion := &db.InitiativeCriterion{
				ID:           c.Id,
				InitiativeID: i.Id,
				Description:  c.Description,
				Status:       c.Status,
				TaskIDs:      string(taskIDsJSON),
				VerifiedAt:   c.VerifiedAt,
				VerifiedBy:   c.VerifiedBy,
				Evidence:     c.Evidence,
			}
			if err := db.AddInitiativeCriterionTx(tx, dbCriterion); err != nil {
				return fmt.Errorf("save criterion %s: %w", c.Id, err)
			}
		}

		return nil
	})
}

// LoadInitiativeProto loads an initiative as proto type from the database.
func (d *DatabaseBackend) LoadInitiativeProto(id string) (*orcv1.Initiative, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbInit, err := d.db.GetInitiative(id)
	if err != nil {
		return nil, fmt.Errorf("get initiative: %w", err)
	}
	if dbInit == nil {
		return nil, fmt.Errorf("initiative %s not found", id)
	}

	i := dbInitiativeToProtoInitiative(dbInit)

	// Load decisions
	dbDecisions, err := d.db.GetInitiativeDecisions(id)
	if err != nil {
		d.logger.Printf("warning: failed to get decisions: %v", err)
	} else {
		for _, dbDec := range dbDecisions {
			i.Decisions = append(i.Decisions, dbDecisionToProtoDecision(&dbDec))
		}
	}

	// Load task refs
	taskIDs, err := d.db.GetInitiativeTasks(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative tasks: %v", err)
	} else {
		for _, taskID := range taskIDs {
			dbTask, err := d.db.GetTask(taskID)
			if err != nil || dbTask == nil {
				continue
			}
			i.Tasks = append(i.Tasks, dbTaskRefToProtoTaskRef(taskID, dbTask.Title, dbTask.Status))
		}
	}

	// Load dependencies
	deps, err := d.db.GetInitiativeDependencies(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative dependencies: %v", err)
	} else {
		i.BlockedBy = deps
	}

	// Load dependents
	dependents, err := d.db.GetInitiativeDependents(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative dependents: %v", err)
	} else {
		i.Blocks = dependents
	}

	// Load criteria
	dbCriteria, err := d.db.GetInitiativeCriteria(id)
	if err != nil {
		d.logger.Printf("warning: failed to get initiative criteria: %v", err)
	} else {
		for _, dbc := range dbCriteria {
			i.Criteria = append(i.Criteria, dbCriterionToProtoCriterion(&dbc))
		}
	}

	return i, nil
}

// LoadAllInitiativesProto loads all initiatives as proto types from the database.
func (d *DatabaseBackend) LoadAllInitiativesProto() ([]*orcv1.Initiative, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbInits, err := d.db.ListInitiatives(db.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("list initiatives: %w", err)
	}

	// Batch load all related data
	allDecisions, err := d.db.GetAllInitiativeDecisions()
	if err != nil {
		d.logger.Printf("warning: failed to batch load decisions: %v", err)
		allDecisions = make(map[string][]db.InitiativeDecision)
	}

	allTaskRefs, err := d.db.GetAllInitiativeTaskRefs()
	if err != nil {
		d.logger.Printf("warning: failed to batch load task refs: %v", err)
		allTaskRefs = make(map[string][]db.InitiativeTaskRef)
	}

	allDeps, err := d.db.GetAllInitiativeDependencies()
	if err != nil {
		d.logger.Printf("warning: failed to batch load dependencies: %v", err)
		allDeps = make(map[string][]string)
	}

	allDependents, err := d.db.GetAllInitiativeDependents()
	if err != nil {
		d.logger.Printf("warning: failed to batch load dependents: %v", err)
		allDependents = make(map[string][]string)
	}

	allCriteria, err := d.db.GetAllInitiativeCriteria()
	if err != nil {
		d.logger.Printf("warning: failed to batch load criteria: %v", err)
		allCriteria = make(map[string][]db.InitiativeCriterion)
	}

	initiatives := make([]*orcv1.Initiative, 0, len(dbInits))
	for _, dbInit := range dbInits {
		i := dbInitiativeToProtoInitiative(&dbInit)

		// Apply pre-fetched decisions
		if dbDecisions, ok := allDecisions[i.Id]; ok {
			for _, dbDec := range dbDecisions {
				i.Decisions = append(i.Decisions, dbDecisionToProtoDecision(&dbDec))
			}
		}

		// Apply pre-fetched task refs
		if taskRefs, ok := allTaskRefs[i.Id]; ok {
			for _, ref := range taskRefs {
				i.Tasks = append(i.Tasks, dbTaskRefToProtoTaskRef(ref.TaskID, ref.Title, ref.Status))
			}
		}

		// Apply pre-fetched dependencies
		if deps, ok := allDeps[i.Id]; ok {
			i.BlockedBy = deps
		}

		// Apply pre-fetched dependents
		if dependents, ok := allDependents[i.Id]; ok {
			i.Blocks = dependents
		}

		// Apply pre-fetched criteria
		if dbCriteriaList, ok := allCriteria[i.Id]; ok {
			for _, dbc := range dbCriteriaList {
				i.Criteria = append(i.Criteria, dbCriterionToProtoCriterion(&dbc))
			}
		}

		initiatives = append(initiatives, i)
	}

	return initiatives, nil
}

// ============================================================================
// Initiative Note Operations
// ============================================================================

// SaveInitiativeNote saves an initiative note to the database.
func (d *DatabaseBackend) SaveInitiativeNote(n *db.InitiativeNote) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveInitiativeNote(n)
}

// GetInitiativeNote retrieves an initiative note by ID.
func (d *DatabaseBackend) GetInitiativeNote(id string) (*db.InitiativeNote, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetInitiativeNote(id)
}

// GetInitiativeNotes retrieves all notes for an initiative.
func (d *DatabaseBackend) GetInitiativeNotes(initiativeID string) ([]db.InitiativeNote, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetInitiativeNotes(initiativeID)
}

// GetInitiativeNotesByType retrieves notes for an initiative filtered by type.
func (d *DatabaseBackend) GetInitiativeNotesByType(initiativeID, noteType string) ([]db.InitiativeNote, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetInitiativeNotesByType(initiativeID, noteType)
}

// GetInitiativeNotesBySourceTask retrieves notes created by a specific task.
func (d *DatabaseBackend) GetInitiativeNotesBySourceTask(taskID string) ([]db.InitiativeNote, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetInitiativeNotesBySourceTask(taskID)
}

// DeleteInitiativeNote removes an initiative note by ID.
func (d *DatabaseBackend) DeleteInitiativeNote(noteID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.DeleteInitiativeNote(noteID)
}

// GetNextNoteID generates the next note ID.
func (d *DatabaseBackend) GetNextNoteID() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.GetNextNoteID()
}
