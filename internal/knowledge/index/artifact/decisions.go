package artifact

import (
	"context"
	"fmt"

	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/knowledge/store"
)

// IndexDecisions indexes initiative decisions into the knowledge graph.
// Creates :Decision nodes linked via FROM_INITIATIVE to the initiative
// and AFFECTS to changed files.
// Skips if initiativeID is empty or decisions is nil/empty.
func (idx *Indexer) IndexDecisions(ctx context.Context, taskID, initiativeID string, decisions []initiative.Decision, changedFiles []string) error {
	if initiativeID == "" || len(decisions) == 0 {
		return nil
	}

	// Create Initiative anchor node.
	initNodeID, err := idx.graph.CreateNode(ctx, store.Node{
		Labels:     []string{"Initiative"},
		Properties: map[string]interface{}{"initiative_id": initiativeID},
	})
	if err != nil {
		return fmt.Errorf("create Initiative anchor: %w", err)
	}

	// Create File nodes for changed files (shared across all decisions).
	fileNodeIDs := make(map[string]string)
	for _, fp := range changedFiles {
		fileNodeID, fileErr := idx.graph.CreateNode(ctx, store.Node{
			Labels:     []string{"File"},
			Properties: map[string]interface{}{"path": fp},
		})
		if fileErr != nil {
			return fmt.Errorf("create File node %s: %w", fp, fileErr)
		}
		fileNodeIDs[fp] = fileNodeID
	}

	for _, dec := range decisions {
		decNodeID, decErr := idx.graph.CreateNode(ctx, store.Node{
			Labels: []string{"Decision"},
			Properties: map[string]interface{}{
				"decision_id": dec.ID,
				"content":     dec.Decision,
				"rationale":   dec.Rationale,
				"task_id":     taskID,
			},
		})
		if decErr != nil {
			return fmt.Errorf("create Decision node: %w", decErr)
		}

		// FROM_INITIATIVE relationship.
		if relErr := idx.graph.CreateRelationship(ctx, decNodeID, initNodeID, "FROM_INITIATIVE", nil); relErr != nil {
			return fmt.Errorf("create FROM_INITIATIVE: %w", relErr)
		}

		// AFFECTS relationships to changed files.
		for _, fileNodeID := range fileNodeIDs {
			if relErr := idx.graph.CreateRelationship(ctx, decNodeID, fileNodeID, "AFFECTS", nil); relErr != nil {
				return fmt.Errorf("create AFFECTS: %w", relErr)
			}
		}
	}

	return nil
}
