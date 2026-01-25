package storage

import (
	"github.com/randalmurphal/orc/internal/db"
)

// ============================================================================
// Event log and constitution - project-level config/history
// ============================================================================

func (d *DatabaseBackend) SaveEvent(e *db.EventLog) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SaveEvent(e)
}

func (d *DatabaseBackend) SaveEvents(events []*db.EventLog) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.SaveEvents(events)
}

func (d *DatabaseBackend) QueryEvents(opts db.QueryEventsOptions) ([]db.EventLog, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.QueryEvents(opts)
}

// ============================================================================
// Constitution
// ============================================================================

func (d *DatabaseBackend) SaveConstitution(content, version string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	c := &db.Constitution{
		Content: content,
		Version: version,
	}
	return d.db.SaveConstitution(c)
}

func (d *DatabaseBackend) LoadConstitution() (content string, version string, err error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	c, err := d.db.LoadConstitution()
	if err != nil {
		return "", "", err
	}
	return c.Content, c.Version, nil
}

func (d *DatabaseBackend) ConstitutionExists() (bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.db.ConstitutionExists()
}

func (d *DatabaseBackend) DeleteConstitution() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.DeleteConstitution()
}
