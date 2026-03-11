package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
)

func TestEnsureWorkflowCachesSynced_LoadsProjectWorkflowIntoBothCaches(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	orcDir := filepath.Join(projectRoot, ".orc")
	if err := os.MkdirAll(filepath.Join(orcDir, "workflows"), 0755); err != nil {
		t.Fatalf("mkdir workflows: %v", err)
	}

	workflowYAML := []byte(`id: project-workflow
name: Project Workflow
completion_action: none
phases:
  - template: implement
    sequence: 0
`)
	if err := os.WriteFile(filepath.Join(orcDir, "workflows", "project-workflow.yaml"), workflowYAML, 0644); err != nil {
		t.Fatalf("write workflow: %v", err)
	}

	gdb, err := db.OpenGlobalAt(filepath.Join(t.TempDir(), "global.db"))
	if err != nil {
		t.Fatalf("OpenGlobalAt: %v", err)
	}
	defer func() { _ = gdb.Close() }()

	pdb, err := db.OpenProjectAtPath(filepath.Join(t.TempDir(), "project.db"))
	if err != nil {
		t.Fatalf("OpenProjectAtPath: %v", err)
	}
	defer func() { _ = pdb.Close() }()

	if err := ensureWorkflowCachesSynced(projectRoot, gdb, pdb); err != nil {
		t.Fatalf("ensureWorkflowCachesSynced: %v", err)
	}

	globalWorkflow, err := gdb.GetWorkflow("project-workflow")
	if err != nil {
		t.Fatalf("global GetWorkflow: %v", err)
	}
	if globalWorkflow == nil {
		t.Fatal("global workflow cache missing project workflow")
	}

	projectWorkflow, err := pdb.GetWorkflow("project-workflow")
	if err != nil {
		t.Fatalf("project GetWorkflow: %v", err)
	}
	if projectWorkflow == nil {
		t.Fatal("project workflow cache missing project workflow")
	}

	projectPhases, err := pdb.GetWorkflowPhases("project-workflow")
	if err != nil {
		t.Fatalf("project GetWorkflowPhases: %v", err)
	}
	if len(projectPhases) != 1 {
		t.Fatalf("project workflow phases = %d, want 1", len(projectPhases))
	}
}
