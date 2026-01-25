package storage

import (
	"context"
	"fmt"

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
