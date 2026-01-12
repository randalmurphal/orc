package storage

import (
	"fmt"

	"github.com/randalmurphal/orc/internal/config"
)

// NewBackend creates a storage backend based on the configuration.
// For hybrid mode (default), it creates a HybridBackend that uses
// files as source of truth with SQLite cache for FTS.
func NewBackend(projectPath string, cfg *config.StorageConfig) (Backend, error) {
	switch cfg.Mode {
	case config.StorageModeHybrid, "":
		// Default to hybrid mode
		return NewHybridBackend(projectPath, cfg)
	case config.StorageModeFiles:
		// Files-only mode (future implementation)
		// For now, use hybrid with cache disabled
		noCacheCfg := *cfg
		noCacheCfg.Database.CacheTranscripts = false
		return NewHybridBackend(projectPath, &noCacheCfg)
	case config.StorageModeDatabase:
		// Database-primary mode (future implementation)
		return nil, fmt.Errorf("database-primary storage mode not yet implemented")
	default:
		return nil, fmt.Errorf("unknown storage mode: %s", cfg.Mode)
	}
}
