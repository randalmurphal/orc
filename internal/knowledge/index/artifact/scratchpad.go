package artifact

import (
	"context"
	"fmt"

	"github.com/randalmurphal/orc/internal/knowledge/store"
	"github.com/randalmurphal/orc/internal/storage"
)

// IndexScratchpad indexes scratchpad entries by category into the knowledge graph.
//   - "decision" → :Decision node with source="scratchpad", linked via FROM_TASK
//   - "observation" → :Observation node, linked via ABOUT to mentioned files
//   - "warning" → :Warning node, linked via ABOUT to mentioned files
//   - "blocker" → updates :File difficulty_score via Cypher MERGE
//   - "todo" → silently skipped (no graph nodes)
//
// Entries with empty or invalid UTF-8 content are skipped.
// Nil or empty entries are skipped with no error.
func (idx *Indexer) IndexScratchpad(ctx context.Context, taskID string, entries []storage.ScratchpadEntry) error {
	if len(entries) == 0 {
		return nil
	}

	for _, entry := range entries {
		if !isValidContent(entry.Content) {
			continue
		}

		switch entry.Category {
		case "decision":
			if err := idx.indexScratchpadDecision(ctx, taskID, entry); err != nil {
				return err
			}
		case "observation":
			if err := idx.indexScratchpadObservation(ctx, taskID, entry); err != nil {
				return err
			}
		case "warning":
			if err := idx.indexScratchpadWarning(ctx, taskID, entry); err != nil {
				return err
			}
		case "blocker":
			if err := idx.indexScratchpadBlocker(ctx, entry); err != nil {
				return err
			}
		case "todo":
			// Silently skip todo entries.
		}
	}

	return nil
}

func (idx *Indexer) indexScratchpadDecision(ctx context.Context, taskID string, entry storage.ScratchpadEntry) error {
	// Create Task anchor node.
	taskNodeID, err := idx.graph.CreateNode(ctx, store.Node{
		Labels:     []string{"Task"},
		Properties: map[string]interface{}{"task_id": taskID},
	})
	if err != nil {
		return fmt.Errorf("create Task anchor: %w", err)
	}

	// Create Decision node with source="scratchpad".
	decNodeID, err := idx.graph.CreateNode(ctx, store.Node{
		Labels: []string{"Decision"},
		Properties: map[string]interface{}{
			"task_id":  taskID,
			"content":  entry.Content,
			"phase_id": entry.PhaseID,
			"source":   "scratchpad",
		},
	})
	if err != nil {
		return fmt.Errorf("create scratchpad Decision node: %w", err)
	}

	if relErr := idx.graph.CreateRelationship(ctx, decNodeID, taskNodeID, "FROM_TASK", nil); relErr != nil {
		return fmt.Errorf("create FROM_TASK: %w", relErr)
	}

	return nil
}

func (idx *Indexer) indexScratchpadObservation(ctx context.Context, taskID string, entry storage.ScratchpadEntry) error {
	obsNodeID, err := idx.graph.CreateNode(ctx, store.Node{
		Labels: []string{"Observation"},
		Properties: map[string]interface{}{
			"task_id":  taskID,
			"content":  entry.Content,
			"phase_id": entry.PhaseID,
		},
	})
	if err != nil {
		return fmt.Errorf("create Observation node: %w", err)
	}

	// ABOUT relationships to mentioned files.
	filePaths := extractFilePaths(entry.Content)
	for _, fp := range filePaths {
		fileNodeID, fileErr := idx.graph.CreateNode(ctx, store.Node{
			Labels:     []string{"File"},
			Properties: map[string]interface{}{"path": fp},
		})
		if fileErr != nil {
			return fmt.Errorf("create File node %s: %w", fp, fileErr)
		}
		if relErr := idx.graph.CreateRelationship(ctx, obsNodeID, fileNodeID, "ABOUT", nil); relErr != nil {
			return fmt.Errorf("create ABOUT: %w", relErr)
		}
	}

	return nil
}

func (idx *Indexer) indexScratchpadWarning(ctx context.Context, taskID string, entry storage.ScratchpadEntry) error {
	warnNodeID, err := idx.graph.CreateNode(ctx, store.Node{
		Labels: []string{"Warning"},
		Properties: map[string]interface{}{
			"task_id":  taskID,
			"content":  entry.Content,
			"phase_id": entry.PhaseID,
		},
	})
	if err != nil {
		return fmt.Errorf("create Warning node: %w", err)
	}

	// ABOUT relationships to mentioned files.
	filePaths := extractFilePaths(entry.Content)
	for _, fp := range filePaths {
		fileNodeID, fileErr := idx.graph.CreateNode(ctx, store.Node{
			Labels:     []string{"File"},
			Properties: map[string]interface{}{"path": fp},
		})
		if fileErr != nil {
			return fmt.Errorf("create File node %s: %w", fp, fileErr)
		}
		if relErr := idx.graph.CreateRelationship(ctx, warnNodeID, fileNodeID, "ABOUT", nil); relErr != nil {
			return fmt.Errorf("create ABOUT: %w", relErr)
		}
	}

	return nil
}

func (idx *Indexer) indexScratchpadBlocker(ctx context.Context, entry storage.ScratchpadEntry) error {
	// Extract file paths and update their difficulty_score.
	filePaths := extractFilePaths(entry.Content)
	for _, fp := range filePaths {
		query := `MERGE (f:File {path: $path})
ON CREATE SET f.difficulty_score = 1
ON MATCH SET f.difficulty_score = f.difficulty_score + 1`

		params := map[string]interface{}{"path": fp}
		if _, err := idx.graph.ExecuteCypher(ctx, query, params); err != nil {
			return fmt.Errorf("update difficulty_score for %s: %w", fp, err)
		}
	}

	return nil
}
