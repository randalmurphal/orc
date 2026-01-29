package jira

import (
	"strings"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
)

// MapperConfig controls how Jira fields are mapped to orc types.
type MapperConfig struct {
	// DefaultWeight is the orc weight for imported tasks (default: medium).
	DefaultWeight orcv1.TaskWeight
	// DefaultQueue is the orc queue for imported tasks (default: backlog).
	DefaultQueue orcv1.TaskQueue
}

// DefaultMapperConfig returns the default mapper configuration.
func DefaultMapperConfig() MapperConfig {
	return MapperConfig{
		DefaultWeight: orcv1.TaskWeight_TASK_WEIGHT_MEDIUM,
		DefaultQueue:  orcv1.TaskQueue_TASK_QUEUE_BACKLOG,
	}
}

// Mapper converts Jira issues to orc tasks and initiatives.
type Mapper struct {
	cfg MapperConfig
}

// NewMapper creates a Mapper with the given configuration.
func NewMapper(cfg MapperConfig) *Mapper {
	return &Mapper{cfg: cfg}
}

// MapIssueToTask converts a Jira issue to an orc task.
// taskID should be pre-allocated via backend.GetNextTaskID().
func (m *Mapper) MapIssueToTask(issue Issue, taskID string) *orcv1.Task {
	desc := issue.Description
	t := &orcv1.Task{
		Id:          taskID,
		Title:       issue.Summary,
		Description: &desc,
		Weight:      m.cfg.DefaultWeight,
		Status:      mapStatus(issue.StatusKey),
		Queue:       mapQueue(issue.StatusKey, m.cfg.DefaultQueue),
		Priority:    mapPriority(issue.Priority),
		Category:    mapCategory(issue.IssueType),
		Metadata:    make(map[string]string),
		CreatedAt:   timestamppb.New(issue.Created),
		UpdatedAt:   timestamppb.New(time.Now()),
	}

	// Jira key is the idempotency anchor
	t.Metadata["jira_key"] = issue.Key

	// Store Jira-specific data that doesn't map directly
	if len(issue.Labels) > 0 {
		t.Metadata["jira_labels"] = strings.Join(issue.Labels, ",")
	}
	if len(issue.Components) > 0 {
		t.Metadata["jira_components"] = strings.Join(issue.Components, ",")
	}
	if issue.Status != "" {
		t.Metadata["jira_status"] = issue.Status
	}

	return t
}

// MapEpicToInitiative converts a Jira epic to an orc initiative.
// initiativeID should be pre-allocated via backend.GetNextInitiativeID().
func (m *Mapper) MapEpicToInitiative(epic Issue, initiativeID string) *initiative.Initiative {
	init := &initiative.Initiative{
		Version: 1,
		ID:      initiativeID,
		Title:   epic.Summary,
		Status:  mapInitiativeStatus(epic.StatusKey),
		Vision:  epic.Description,
	}
	return init
}

// ResolveLinks maps Jira issue links to orc blocked_by and related_to arrays.
// keyToTaskID maps Jira issue keys to orc task IDs for the current import set.
// Links to issues outside the import set are ignored.
func (m *Mapper) ResolveLinks(issue Issue, keyToTaskID map[string]string) (blockedBy, relatedTo []string) {
	for _, link := range issue.IssueLinks {
		targetID, ok := keyToTaskID[link.LinkedKey]
		if !ok {
			// Linked issue not in import set — skip
			continue
		}

		linkName := strings.ToLower(link.Type)

		// "Blocks" link type: outward = "blocks", inward = "is blocked by"
		if linkName == "blocks" {
			if link.Direction == LinkInward {
				// This issue "is blocked by" the linked issue
				blockedBy = append(blockedBy, targetID)
			}
			// Outward "blocks" is the reverse — the linked issue is blocked by us,
			// which we don't record on this task.
			continue
		}

		// Everything else (Relates, Cloners, Duplicate, etc.) is informational
		relatedTo = append(relatedTo, targetID)
	}
	return blockedBy, relatedTo
}

// mapPriority converts Jira's 5-level priority to orc's 4-level priority.
func mapPriority(jiraPriority string) orcv1.TaskPriority {
	switch strings.ToLower(jiraPriority) {
	case "highest":
		return orcv1.TaskPriority_TASK_PRIORITY_CRITICAL
	case "high":
		return orcv1.TaskPriority_TASK_PRIORITY_HIGH
	case "medium":
		return orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	case "low", "lowest":
		return orcv1.TaskPriority_TASK_PRIORITY_LOW
	default:
		return orcv1.TaskPriority_TASK_PRIORITY_NORMAL
	}
}

// mapCategory converts Jira issue type to orc task category.
func mapCategory(issueType string) orcv1.TaskCategory {
	switch strings.ToLower(issueType) {
	case "bug":
		return orcv1.TaskCategory_TASK_CATEGORY_BUG
	case "story", "task", "epic":
		return orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	case "sub-task", "subtask":
		return orcv1.TaskCategory_TASK_CATEGORY_CHORE
	case "improvement":
		return orcv1.TaskCategory_TASK_CATEGORY_REFACTOR
	default:
		return orcv1.TaskCategory_TASK_CATEGORY_FEATURE
	}
}

// mapStatus converts Jira's status category key to orc task status.
// Jira status category keys: "new", "indeterminate", "done", "undefined".
func mapStatus(statusCategoryKey string) orcv1.TaskStatus {
	switch statusCategoryKey {
	case "done":
		return orcv1.TaskStatus_TASK_STATUS_COMPLETED
	default:
		// Both "new" and "indeterminate" map to created — orc doesn't start tasks on import.
		return orcv1.TaskStatus_TASK_STATUS_CREATED
	}
}

// mapQueue determines the orc queue based on Jira status.
// "In progress" items go to active, everything else goes to the default queue.
func mapQueue(statusCategoryKey string, defaultQueue orcv1.TaskQueue) orcv1.TaskQueue {
	switch statusCategoryKey {
	case "indeterminate": // In Progress
		return orcv1.TaskQueue_TASK_QUEUE_ACTIVE
	case "done":
		return orcv1.TaskQueue_TASK_QUEUE_ACTIVE
	default:
		return defaultQueue
	}
}

// mapInitiativeStatus converts Jira status category to initiative status.
func mapInitiativeStatus(statusCategoryKey string) initiative.Status {
	switch statusCategoryKey {
	case "done":
		return initiative.StatusCompleted
	case "indeterminate":
		return initiative.StatusActive
	default:
		return initiative.StatusDraft
	}
}
