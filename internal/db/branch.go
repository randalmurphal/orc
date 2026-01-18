package db

import (
	"database/sql"
	"fmt"
	"time"
)

// BranchType represents the type of a tracked branch.
type BranchType string

const (
	BranchTypeInitiative BranchType = "initiative"
	BranchTypeStaging    BranchType = "staging"
	BranchTypeTask       BranchType = "task"
)

// BranchStatus represents the status of a tracked branch.
type BranchStatus string

const (
	BranchStatusActive   BranchStatus = "active"
	BranchStatusMerged   BranchStatus = "merged"
	BranchStatusStale    BranchStatus = "stale"
	BranchStatusOrphaned BranchStatus = "orphaned"
)

// Branch represents a tracked branch in the registry.
type Branch struct {
	Name           string
	Type           BranchType
	OwnerID        string // Initiative ID, developer name, or task ID
	BaseBranch     string // Branch this was created from (e.g., "main")
	Status         BranchStatus
	CreatedAt      time.Time
	LastActivity   time.Time
	MergedAt       *time.Time
	MergedTo       string
	MergeCommitSHA string
}

// BranchListOpts specifies options for listing branches.
type BranchListOpts struct {
	Type   BranchType
	Status BranchStatus
}

// SaveBranch creates or updates a branch in the registry.
func (p *ProjectDB) SaveBranch(b *Branch) error {
	var mergedAt *string
	if b.MergedAt != nil {
		s := b.MergedAt.Format(time.RFC3339)
		mergedAt = &s
	}

	_, err := p.Exec(`
		INSERT INTO branches (name, type, owner_id, base_branch, status, created_at, last_activity, merged_at, merged_to, merge_commit_sha)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			type = excluded.type,
			owner_id = excluded.owner_id,
			base_branch = excluded.base_branch,
			status = excluded.status,
			last_activity = excluded.last_activity,
			merged_at = excluded.merged_at,
			merged_to = excluded.merged_to,
			merge_commit_sha = excluded.merge_commit_sha
	`,
		b.Name,
		string(b.Type),
		b.OwnerID,
		b.BaseBranch,
		string(b.Status),
		b.CreatedAt.Format(time.RFC3339),
		b.LastActivity.Format(time.RFC3339),
		mergedAt,
		b.MergedTo,
		b.MergeCommitSHA,
	)
	if err != nil {
		return fmt.Errorf("save branch: %w", err)
	}
	return nil
}

// GetBranch retrieves a branch by name.
func (p *ProjectDB) GetBranch(name string) (*Branch, error) {
	row := p.QueryRow(`
		SELECT name, type, owner_id, base_branch, status, created_at, last_activity, merged_at, merged_to, merge_commit_sha
		FROM branches WHERE name = ?
	`, name)

	var b Branch
	var typeStr, statusStr, createdAtStr, lastActivityStr string
	var baseBranch, mergedAt, mergedTo, mergeCommitSHA sql.NullString

	err := row.Scan(&b.Name, &typeStr, &b.OwnerID, &baseBranch, &statusStr, &createdAtStr, &lastActivityStr, &mergedAt, &mergedTo, &mergeCommitSHA)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get branch: %w", err)
	}

	b.Type = BranchType(typeStr)
	b.Status = BranchStatus(statusStr)
	if baseBranch.Valid {
		b.BaseBranch = baseBranch.String
	}
	if mergedTo.Valid {
		b.MergedTo = mergedTo.String
	}
	if mergeCommitSHA.Valid {
		b.MergeCommitSHA = mergeCommitSHA.String
	}

	b.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	b.LastActivity, _ = time.Parse(time.RFC3339, lastActivityStr)
	if mergedAt.Valid {
		t, _ := time.Parse(time.RFC3339, mergedAt.String)
		b.MergedAt = &t
	}

	return &b, nil
}

// ListBranches returns all tracked branches matching the filter options.
func (p *ProjectDB) ListBranches(opts BranchListOpts) ([]*Branch, error) {
	query := `SELECT name, type, owner_id, base_branch, status, created_at, last_activity, merged_at, merged_to, merge_commit_sha FROM branches WHERE 1=1`
	var args []any

	if opts.Type != "" {
		query += " AND type = ?"
		args = append(args, string(opts.Type))
	}
	if opts.Status != "" {
		query += " AND status = ?"
		args = append(args, string(opts.Status))
	}
	query += " ORDER BY last_activity DESC"

	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var branches []*Branch
	for rows.Next() {
		var b Branch
		var typeStr, statusStr, createdAtStr, lastActivityStr string
		var baseBranch, mergedAt, mergedTo, mergeCommitSHA sql.NullString

		if err := rows.Scan(&b.Name, &typeStr, &b.OwnerID, &baseBranch, &statusStr, &createdAtStr, &lastActivityStr, &mergedAt, &mergedTo, &mergeCommitSHA); err != nil {
			return nil, fmt.Errorf("scan branch: %w", err)
		}

		b.Type = BranchType(typeStr)
		b.Status = BranchStatus(statusStr)
		if baseBranch.Valid {
			b.BaseBranch = baseBranch.String
		}
		if mergedTo.Valid {
			b.MergedTo = mergedTo.String
		}
		if mergeCommitSHA.Valid {
			b.MergeCommitSHA = mergeCommitSHA.String
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		b.LastActivity, _ = time.Parse(time.RFC3339, lastActivityStr)
		if mergedAt.Valid {
			t, _ := time.Parse(time.RFC3339, mergedAt.String)
			b.MergedAt = &t
		}

		branches = append(branches, &b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate branches: %w", err)
	}

	return branches, nil
}

// UpdateBranchStatus updates a branch's status.
func (p *ProjectDB) UpdateBranchStatus(name string, status BranchStatus) error {
	_, err := p.Exec(`UPDATE branches SET status = ?, last_activity = datetime('now') WHERE name = ?`, string(status), name)
	if err != nil {
		return fmt.Errorf("update branch status: %w", err)
	}
	return nil
}

// UpdateBranchActivity updates a branch's last_activity timestamp.
func (p *ProjectDB) UpdateBranchActivity(name string) error {
	// Use RFC3339 format to match time.Parse in GetBranch
	_, err := p.Exec(`UPDATE branches SET last_activity = strftime('%Y-%m-%dT%H:%M:%SZ', 'now') WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("update branch activity: %w", err)
	}
	return nil
}

// DeleteBranch removes a branch from the registry.
func (p *ProjectDB) DeleteBranch(name string) error {
	_, err := p.Exec(`DELETE FROM branches WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}
	return nil
}

// GetStaleBranches returns branches that haven't had activity since the given time.
func (p *ProjectDB) GetStaleBranches(since time.Time) ([]*Branch, error) {
	rows, err := p.Query(`
		SELECT name, type, owner_id, base_branch, status, created_at, last_activity, merged_at, merged_to, merge_commit_sha
		FROM branches
		WHERE status = 'active' AND last_activity < ?
		ORDER BY last_activity ASC
	`, since.Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("get stale branches: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var branches []*Branch
	for rows.Next() {
		var b Branch
		var typeStr, statusStr, createdAtStr, lastActivityStr string
		var baseBranch, mergedAt, mergedTo, mergeCommitSHA sql.NullString

		if err := rows.Scan(&b.Name, &typeStr, &b.OwnerID, &baseBranch, &statusStr, &createdAtStr, &lastActivityStr, &mergedAt, &mergedTo, &mergeCommitSHA); err != nil {
			return nil, fmt.Errorf("scan branch: %w", err)
		}

		b.Type = BranchType(typeStr)
		b.Status = BranchStatus(statusStr)
		if baseBranch.Valid {
			b.BaseBranch = baseBranch.String
		}
		if mergedTo.Valid {
			b.MergedTo = mergedTo.String
		}
		if mergeCommitSHA.Valid {
			b.MergeCommitSHA = mergeCommitSHA.String
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
		b.LastActivity, _ = time.Parse(time.RFC3339, lastActivityStr)
		if mergedAt.Valid {
			t, _ := time.Parse(time.RFC3339, mergedAt.String)
			b.MergedAt = &t
		}

		branches = append(branches, &b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate stale branches: %w", err)
	}

	return branches, nil
}
