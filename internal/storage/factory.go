package storage

import (
	"github.com/randalmurphal/orc/internal/config"
)

// NewBackend creates a storage backend based on the configuration.
// All storage modes now use the DatabaseBackend - SQLite is the source of truth.
func NewBackend(projectPath string, cfg *config.StorageConfig) (Backend, error) {
	// All modes now use database backend - SQLite is the source of truth
	return NewDatabaseBackend(projectPath, cfg)
}
