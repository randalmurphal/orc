package brief

import (
	"context"
	"fmt"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
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
