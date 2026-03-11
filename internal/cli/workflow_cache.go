package cli

import (
	"fmt"
	"path/filepath"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/workflow"
)

func ensureWorkflowCachesSynced(projectRoot string, gdb *db.GlobalDB, pdb *db.ProjectDB) error {
	orcDir := filepath.Join(projectRoot, ".orc")

	globalCache := workflow.NewCacheServiceFromOrcDir(orcDir, gdb)
	if _, err := globalCache.EnsureSynced(); err != nil {
		return fmt.Errorf("sync global workflow cache: %w", err)
	}

	projectCache := workflow.NewCacheServiceFromOrcDir(orcDir, pdb)
	if _, err := projectCache.EnsureSynced(); err != nil {
		return fmt.Errorf("sync project workflow cache: %w", err)
	}

	return nil
}
