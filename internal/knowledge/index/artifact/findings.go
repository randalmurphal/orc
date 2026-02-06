package artifact

import (
	"context"
	"fmt"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/knowledge/store"
)

// IndexFindings indexes review findings into the knowledge graph.
// Creates :Finding nodes with ABOUT relationships to :File nodes
// and FOUND_BY relationships to reviewer agent nodes.
// Empty or nil findings are skipped with no error.
func (idx *Indexer) IndexFindings(ctx context.Context, taskID string, findings []*orcv1.ReviewRoundFindings) error {
	if len(findings) == 0 {
		return nil
	}

	for _, round := range findings {
		for _, issue := range round.GetIssues() {
			props := map[string]interface{}{
				"task_id":     taskID,
				"severity":    issue.GetSeverity(),
				"description": issue.GetDescription(),
			}
			if issue.File != nil {
				props["file_path"] = *issue.File
			}
			if issue.Line != nil {
				props["line"] = *issue.Line
			}

			findingNodeID, err := idx.graph.CreateNode(ctx, store.Node{
				Labels:     []string{"Finding"},
				Properties: props,
			})
			if err != nil {
				return fmt.Errorf("create Finding node: %w", err)
			}

			// ABOUT relationship to File node.
			if issue.File != nil {
				fileNodeID, fileErr := idx.graph.CreateNode(ctx, store.Node{
					Labels:     []string{"File"},
					Properties: map[string]interface{}{"path": *issue.File},
				})
				if fileErr != nil {
					return fmt.Errorf("create File node: %w", fileErr)
				}
				if relErr := idx.graph.CreateRelationship(ctx, findingNodeID, fileNodeID, "ABOUT", nil); relErr != nil {
					return fmt.Errorf("create ABOUT: %w", relErr)
				}
			}

			// FOUND_BY relationship to reviewer agent.
			if issue.AgentId != nil && *issue.AgentId != "" {
				agentNodeID, agentErr := idx.graph.CreateNode(ctx, store.Node{
					Labels:     []string{"Agent"},
					Properties: map[string]interface{}{"agent_id": *issue.AgentId},
				})
				if agentErr != nil {
					return fmt.Errorf("create Agent node: %w", agentErr)
				}
				if relErr := idx.graph.CreateRelationship(ctx, findingNodeID, agentNodeID, "FOUND_BY", nil); relErr != nil {
					return fmt.Errorf("create FOUND_BY: %w", relErr)
				}
			}
		}
	}

	return nil
}
