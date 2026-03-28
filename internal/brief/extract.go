package brief

import (
	"context"
	"fmt"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
)

const maxFindingEntries = 10

// ExtractDecisions pulls decision entries from active initiatives.
func ExtractDecisions(ctx context.Context, backend *storage.DatabaseBackend) ([]Entry, error) {
	initiatives, err := backend.LoadAllInitiatives()
	if err != nil {
		return nil, fmt.Errorf("load initiatives: %w", err)
	}

	var entries []Entry
	for _, init := range initiatives {
		if init.Status == initiative.StatusArchived {
			continue
		}
		for _, dec := range init.Decisions {
			entries = append(entries, Entry{
				Content: dec.Decision,
				Source:  init.ID,
				Impact:  0.8,
			})
		}
	}
	return entries, nil
}

// ExtractFindings pulls high-severity review findings from completed tasks.
func ExtractFindings(ctx context.Context, backend *storage.DatabaseBackend) ([]Entry, error) {
	tasks, err := backend.LoadAllTasks()
	if err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}

	var entries []Entry
	for _, t := range tasks {
		if t.Status != orcv1.TaskStatus_TASK_STATUS_COMPLETED {
			continue
		}

		allFindings, err := backend.LoadAllReviewFindings(t.Id)
		if err != nil {
			return nil, fmt.Errorf("load findings for %s: %w", t.Id, err)
		}

		for _, round := range allFindings {
			for _, issue := range round.GetIssues() {
				if issue.GetSeverity() != "high" {
					continue
				}

				content := issue.GetDescription()
				if issue.GetFile() != "" {
					content = fmt.Sprintf("%s (%s)", content, issue.GetFile())
				}

				entries = append(entries, Entry{
					Content: content,
					Source:  t.Id,
					Impact:  0.8,
				})

				if len(entries) >= maxFindingEntries {
					return entries, nil
				}
			}
		}
	}
	return entries, nil
}

// ExtractIndexedArtifacts pulls recent indexed artifacts into the brief.
func ExtractIndexedArtifacts(ctx context.Context, backend *storage.DatabaseBackend) ([]Entry, error) {
	entries, err := backend.GetRecentArtifacts(db.RecentArtifactOpts{Limit: 10})
	if err != nil {
		return nil, fmt.Errorf("get recent artifacts: %w", err)
	}

	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		content := strings.TrimSpace(entry.Title)
		if entry.Content != "" {
			if content == "" {
				content = strings.TrimSpace(entry.Content)
			} else {
				content = content + ": " + strings.TrimSpace(entry.Content)
			}
		}
		if content == "" {
			continue
		}

		source := entry.Kind
		if entry.InitiativeID != "" {
			source = entry.InitiativeID
		} else if entry.SourceTaskID != "" {
			source = entry.SourceTaskID
		} else if entry.SourceThreadID != "" {
			source = entry.SourceThreadID
		}

		result = append(result, Entry{
			Content: content,
			Source:  source,
			Impact:  0.85,
		})
	}
	return result, nil
}
