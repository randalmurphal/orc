package artifact

import (
	"context"
	"fmt"
)

// IndexMetrics updates :File nodes with aggregated task metrics.
// Uses MERGE queries to create-or-update file nodes with total_tasks_touching
// and avg_retry_rate properties.
// Empty or nil changedFiles are skipped with no error.
func (idx *Indexer) IndexMetrics(ctx context.Context, taskID string, changedFiles []string, retryCount int) error {
	if len(changedFiles) == 0 {
		return nil
	}

	for _, fp := range changedFiles {
		query := `MERGE (f:File {path: $path})
ON CREATE SET f.total_tasks_touching = 1, f.avg_retry_rate = $retry_count
ON MATCH SET f.total_tasks_touching = f.total_tasks_touching + 1,
             f.avg_retry_rate = (f.avg_retry_rate * (f.total_tasks_touching - 1) + $retry_count) / f.total_tasks_touching`

		params := map[string]interface{}{
			"path":        fp,
			"task_id":     taskID,
			"retry_count": retryCount,
		}

		if _, err := idx.graph.ExecuteCypher(ctx, query, params); err != nil {
			return fmt.Errorf("update metrics for %s: %w", fp, err)
		}
	}

	return nil
}
