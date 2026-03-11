package storage

import (
	"github.com/randalmurphal/orc/internal/controlplane"
)

func (d *DatabaseBackend) SaveAttentionSignal(signal *controlplane.PersistedAttentionSignal) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.SaveAttentionSignal(signal)
}

func (d *DatabaseBackend) LoadAttentionSignal(id string) (*controlplane.PersistedAttentionSignal, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.GetAttentionSignal(id)
}

func (d *DatabaseBackend) LoadActiveAttentionSignals() ([]*controlplane.PersistedAttentionSignal, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.ListActiveAttentionSignals()
}

func (d *DatabaseBackend) ResolveAttentionSignal(id string, resolvedBy string) (*controlplane.PersistedAttentionSignal, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.db.ResolveAttentionSignal(id, resolvedBy)
}

func (d *DatabaseBackend) CountActiveAttentionSignals() (int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return d.db.CountActiveAttentionSignals()
}
