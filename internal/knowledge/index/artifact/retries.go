package artifact

import (
	"context"
	"fmt"

	"github.com/randalmurphal/orc/internal/knowledge/store"
)

// IndexRetries indexes retry attempts into the knowledge graph.
// Creates :Retry nodes linked via FROM_TASK to the task.
// Empty or nil retries are skipped with no error.
func (idx *Indexer) IndexRetries(ctx context.Context, taskID string, retries []RetryInfo) error {
	if len(retries) == 0 {
		return nil
	}

	// Idempotent: remove old Retry nodes for this task.
	_ = idx.graph.DeleteNodesByProperty(ctx, "Retry", "task_id", taskID)

	// Create Task anchor node.
	taskNodeID, err := idx.graph.CreateNode(ctx, store.Node{
		Labels:     []string{"Task"},
		Properties: map[string]interface{}{"task_id": taskID},
	})
	if err != nil {
		return fmt.Errorf("create Task anchor: %w", err)
	}

	for _, r := range retries {
		retryNodeID, retryErr := idx.graph.CreateNode(ctx, store.Node{
			Labels: []string{"Retry"},
			Properties: map[string]interface{}{
				"task_id":    taskID,
				"attempt":    r.Attempt,
				"reason":     r.Reason,
				"from_phase": r.FromPhase,
			},
		})
		if retryErr != nil {
			return fmt.Errorf("create Retry node: %w", retryErr)
		}

		if relErr := idx.graph.CreateRelationship(ctx, retryNodeID, taskNodeID, "FROM_TASK", nil); relErr != nil {
			return fmt.Errorf("create FROM_TASK: %w", relErr)
		}
	}

	return nil
}
