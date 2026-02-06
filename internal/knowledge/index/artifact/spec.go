package artifact

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/randalmurphal/orc/internal/knowledge/store"
)

// IndexSpec indexes a task's spec content into the knowledge graph.
// Creates a :Spec node linked to the task via FROM_TASK, with TARGETS
// relationships to :File nodes extracted from the spec text.
// Empty spec is skipped with no error.
func (idx *Indexer) IndexSpec(ctx context.Context, taskID, spec string) error {
	if spec == "" {
		return nil
	}

	// Idempotent: remove old Spec nodes for this task.
	_ = idx.graph.DeleteNodesByProperty(ctx, "Spec", "task_id", taskID)

	// Create Task anchor node.
	taskNodeID, err := idx.graph.CreateNode(ctx, store.Node{
		Labels:     []string{"Task"},
		Properties: map[string]interface{}{"task_id": taskID},
	})
	if err != nil {
		return fmt.Errorf("create Task anchor: %w", err)
	}

	// Create Spec node.
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(spec)))
	specNodeID, err := idx.graph.CreateNode(ctx, store.Node{
		Labels: []string{"Spec"},
		Properties: map[string]interface{}{
			"task_id":      taskID,
			"content_hash": hash,
		},
	})
	if err != nil {
		return fmt.Errorf("create Spec node: %w", err)
	}

	// FROM_TASK relationship.
	if err := idx.graph.CreateRelationship(ctx, specNodeID, taskNodeID, "FROM_TASK", nil); err != nil {
		return fmt.Errorf("create FROM_TASK: %w", err)
	}

	// Extract file paths and create TARGETS relationships.
	filePaths := extractFilePaths(spec)
	for _, fp := range filePaths {
		fileNodeID, fileErr := idx.graph.CreateNode(ctx, store.Node{
			Labels:     []string{"File"},
			Properties: map[string]interface{}{"path": fp},
		})
		if fileErr != nil {
			return fmt.Errorf("create File node %s: %w", fp, fileErr)
		}
		if relErr := idx.graph.CreateRelationship(ctx, specNodeID, fileNodeID, "TARGETS", nil); relErr != nil {
			return fmt.Errorf("create TARGETS: %w", relErr)
		}
	}

	return nil
}
