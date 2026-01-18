package db

import (
	"database/sql"
	"fmt"
	"time"
)

// SyncState represents the sync state for P2P replication.
type SyncState struct {
	SiteID          string
	LastSyncVersion int64
	LastSyncAt      *time.Time
	SyncEnabled     bool
	SyncMode        string // 'none', 'folder', 'http'
	SyncEndpoint    string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// GetSyncState retrieves the sync state.
func (p *ProjectDB) GetSyncState() (*SyncState, error) {
	row := p.QueryRow(`
		SELECT site_id, last_sync_version, last_sync_at, sync_enabled, sync_mode, sync_endpoint, created_at, updated_at
		FROM sync_state WHERE id = 1
	`)

	var s SyncState
	var lastSyncAt, syncEndpoint sql.NullString
	var syncEnabled int
	var createdAt, updatedAt string

	if err := row.Scan(&s.SiteID, &s.LastSyncVersion, &lastSyncAt, &syncEnabled, &s.SyncMode, &syncEndpoint, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get sync state: %w", err)
	}

	s.SyncEnabled = syncEnabled == 1
	if lastSyncAt.Valid {
		if ts, err := time.Parse(time.RFC3339, lastSyncAt.String); err == nil {
			s.LastSyncAt = &ts
		}
	}
	if syncEndpoint.Valid {
		s.SyncEndpoint = syncEndpoint.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		s.CreatedAt = ts
	}
	if ts, err := time.Parse(time.RFC3339, updatedAt); err == nil {
		s.UpdatedAt = ts
	}

	return &s, nil
}

// UpdateSyncState updates the sync state.
func (p *ProjectDB) UpdateSyncState(s *SyncState) error {
	now := time.Now().Format(time.RFC3339)
	syncEnabled := 0
	if s.SyncEnabled {
		syncEnabled = 1
	}

	var lastSyncAt *string
	if s.LastSyncAt != nil {
		ts := s.LastSyncAt.Format(time.RFC3339)
		lastSyncAt = &ts
	}

	_, err := p.Exec(`
		UPDATE sync_state SET
			last_sync_version = ?,
			last_sync_at = ?,
			sync_enabled = ?,
			sync_mode = ?,
			sync_endpoint = ?,
			updated_at = ?
		WHERE id = 1
	`, s.LastSyncVersion, lastSyncAt, syncEnabled, s.SyncMode, s.SyncEndpoint, now)
	if err != nil {
		return fmt.Errorf("update sync state: %w", err)
	}
	return nil
}
