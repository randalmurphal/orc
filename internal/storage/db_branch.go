package storage

import (
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/git"
)

// ============================================================================
// Branch registry - git integration
// ============================================================================

func (d *DatabaseBackend) SaveBranch(b *Branch) error {
	if err := git.ValidateBranchName(b.Name); err != nil {
		return fmt.Errorf("save branch: %w", err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	dbBranch := &db.Branch{
		Name:         b.Name,
		Type:         db.BranchType(b.Type),
		OwnerID:      b.OwnerID,
		CreatedAt:    b.CreatedAt,
		LastActivity: b.LastActivity,
		Status:       db.BranchStatus(b.Status),
	}

	return d.db.SaveBranch(dbBranch)
}

func (d *DatabaseBackend) LoadBranch(name string) (*Branch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbBranch, err := d.db.GetBranch(name)
	if err != nil {
		return nil, err
	}
	if dbBranch == nil {
		return nil, nil
	}

	return &Branch{
		Name:         dbBranch.Name,
		Type:         BranchType(dbBranch.Type),
		OwnerID:      dbBranch.OwnerID,
		CreatedAt:    dbBranch.CreatedAt,
		LastActivity: dbBranch.LastActivity,
		Status:       BranchStatus(dbBranch.Status),
	}, nil
}

func (d *DatabaseBackend) ListBranches(opts BranchListOpts) ([]*Branch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbOpts := db.BranchListOpts{
		Type:   db.BranchType(opts.Type),
		Status: db.BranchStatus(opts.Status),
	}

	dbBranches, err := d.db.ListBranches(dbOpts)
	if err != nil {
		return nil, err
	}

	branches := make([]*Branch, len(dbBranches))
	for i, dbBranch := range dbBranches {
		branches[i] = &Branch{
			Name:         dbBranch.Name,
			Type:         BranchType(dbBranch.Type),
			OwnerID:      dbBranch.OwnerID,
			CreatedAt:    dbBranch.CreatedAt,
			LastActivity: dbBranch.LastActivity,
			Status:       BranchStatus(dbBranch.Status),
		}
	}

	return branches, nil
}

func (d *DatabaseBackend) UpdateBranchStatus(name string, status BranchStatus) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.UpdateBranchStatus(name, db.BranchStatus(status))
}

func (d *DatabaseBackend) UpdateBranchActivity(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.UpdateBranchActivity(name)
}

func (d *DatabaseBackend) DeleteBranch(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeleteBranch(name)
}

func (d *DatabaseBackend) GetStaleBranches(since time.Time) ([]*Branch, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbBranches, err := d.db.GetStaleBranches(since)
	if err != nil {
		return nil, err
	}

	branches := make([]*Branch, len(dbBranches))
	for i, dbBranch := range dbBranches {
		branches[i] = &Branch{
			Name:         dbBranch.Name,
			Type:         BranchType(dbBranch.Type),
			OwnerID:      dbBranch.OwnerID,
			CreatedAt:    dbBranch.CreatedAt,
			LastActivity: dbBranch.LastActivity,
			Status:       BranchStatus(dbBranch.Status),
		}
	}

	return branches, nil
}
