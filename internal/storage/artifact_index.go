package storage

import (
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
)

func (d *DatabaseBackend) SaveArtifactIndexEntry(entry *db.ArtifactIndexEntry) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.db.SaveArtifactIndexEntry(entry); err != nil {
		return fmt.Errorf("save artifact index entry: %w", err)
	}
	return nil
}

func (d *DatabaseBackend) QueryArtifactIndex(opts db.ArtifactIndexQueryOpts) ([]db.ArtifactIndexEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	entries, err := d.db.QueryArtifactIndex(opts)
	if err != nil {
		return nil, fmt.Errorf("query artifact index: %w", err)
	}
	return entries, nil
}

func (d *DatabaseBackend) QueryArtifactIndexByDedupeKey(dedupeKey string) ([]db.ArtifactIndexEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	entries, err := d.db.QueryArtifactIndexByDedupeKey(dedupeKey)
	if err != nil {
		return nil, fmt.Errorf("query artifact index by dedupe key: %w", err)
	}
	return entries, nil
}

func (d *DatabaseBackend) GetRecentArtifacts(opts db.RecentArtifactOpts) ([]db.ArtifactIndexEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	entries, err := d.db.GetRecentArtifacts(opts)
	if err != nil {
		return nil, fmt.Errorf("get recent artifacts: %w", err)
	}
	return entries, nil
}

func SaveArtifactIndexEntryIfAbsent(backend Backend, entry *db.ArtifactIndexEntry) error {
	if backend == nil {
		return fmt.Errorf("backend is required")
	}
	if entry == nil {
		return fmt.Errorf("artifact index entry is required")
	}

	if strings.TrimSpace(entry.DedupeKey) != "" {
		matches, err := backend.QueryArtifactIndexByDedupeKey(entry.DedupeKey)
		if err != nil {
			return err
		}
		for _, match := range matches {
			if match.Kind == entry.Kind {
				return nil
			}
		}
	}

	return backend.SaveArtifactIndexEntry(entry)
}
