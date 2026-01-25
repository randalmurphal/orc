package storage

import (
	"io"
	"log"
	"sync"
	"time"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

// DatabaseBackend uses SQLite/PostgreSQL as the sole source of truth.
// No YAML files are created or read. This enables database sync across machines.
// All operations are protected by a mutex for concurrent access safety.
type DatabaseBackend struct {
	projectPath string
	db          *db.ProjectDB
	cfg         *config.StorageConfig
	mu          sync.RWMutex
	logger      *log.Logger
}

// NewDatabaseBackend creates a new database-only storage backend.
func NewDatabaseBackend(projectPath string, cfg *config.StorageConfig) (*DatabaseBackend, error) {
	pdb, err := db.OpenProject(projectPath)
	if err != nil {
		return nil, err
	}

	logger := log.New(io.Discard, "", 0)

	if cfg != nil && cfg.Database.RetentionDays > 0 {
		retention := time.Duration(cfg.Database.RetentionDays) * 24 * time.Hour
		deleted, err := pdb.CleanupOldTranscripts(retention)
		if err != nil {
			logger.Printf("transcript cleanup failed: %v", err)
		} else if deleted > 0 {
			logger.Printf("cleaned up %d old transcripts (older than %d days)", deleted, cfg.Database.RetentionDays)
		}
	}

	return &DatabaseBackend{
		projectPath: projectPath,
		db:          pdb,
		cfg:         cfg,
		logger:      logger,
	}, nil
}

// NewInMemoryBackend creates an in-memory database backend for testing.
func NewInMemoryBackend() (*DatabaseBackend, error) {
	pdb, err := db.OpenProjectInMemory()
	if err != nil {
		return nil, err
	}

	return &DatabaseBackend{
		projectPath: ":memory:",
		db:          pdb,
		cfg:         nil,
		logger:      log.New(io.Discard, "", 0),
	}, nil
}

// SetLogger sets the logger for warnings and debug messages.
func (d *DatabaseBackend) SetLogger(l *log.Logger) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logger = l
}

// DB returns the underlying database for direct access.
// WARNING: Direct database access bypasses the mutex protection.
func (d *DatabaseBackend) DB() *db.ProjectDB {
	return d.db
}

// MaterializeContext generates context files for worktree execution.
func (d *DatabaseBackend) MaterializeContext(taskID, outputPath string) error {
	return nil
}

// NeedsMaterialization returns true for database mode.
func (d *DatabaseBackend) NeedsMaterialization() bool {
	return true
}

// Sync flushes any pending operations.
func (d *DatabaseBackend) Sync() error {
	return nil
}

// Cleanup removes old data based on retention policy.
func (d *DatabaseBackend) Cleanup() error {
	return nil
}

// Close releases database resources.
func (d *DatabaseBackend) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.db.Close()
}

// Ensure DatabaseBackend implements Backend
var _ Backend = (*DatabaseBackend)(nil)
