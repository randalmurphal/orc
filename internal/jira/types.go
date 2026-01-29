// Package jira provides Jira Cloud import functionality for orc.
// It fetches issues via the Jira REST API v3 and maps them to orc tasks and initiatives.
package jira

import "time"

// Issue represents a simplified Jira issue with the fields orc cares about.
// Mapped from the go-atlassian IssueScheme during fetch.
type Issue struct {
	Key         string
	Summary     string
	Description string // Already converted from ADF to Markdown
	IssueType   string // e.g., "Epic", "Story", "Task", "Bug", "Sub-task"
	IsSubtask   bool
	Status      string // Jira status name (e.g., "To Do", "In Progress", "Done")
	StatusKey   string // Status category key: "new", "indeterminate", "done"
	Priority    string // Jira priority name: "Highest", "High", "Medium", "Low", "Lowest"
	Labels      []string
	Components  []string
	ParentKey   string // Parent issue key (for subtasks and epic children)
	IssueLinks  []IssueLink
	Created     time.Time
	Updated     time.Time

	// Additional fields for richer metadata
	Assignee     string            // Display name of assignee
	Reporter     string            // Display name of reporter
	Resolution   string            // Resolution name (e.g., "Done", "Won't Do")
	FixVersions  []string          // Version names
	DueDate      string            // ISO date string (e.g., "2025-03-15") or empty
	Project      string            // Project key (e.g., "PROJ")
	CustomFields map[string]string // Extracted custom fields (metadata key → string value)
}

// IsEpic returns true if this issue is an Epic.
func (i Issue) IsEpic() bool {
	return i.IssueType == "Epic"
}

// IssueLink represents a directional link between two Jira issues.
type IssueLink struct {
	// Type is the link type name (e.g., "Blocks", "Relates")
	Type string
	// Direction indicates whether this issue is the inward or outward side.
	Direction LinkDirection
	// LinkedKey is the key of the other issue in the link.
	LinkedKey string
}

// LinkDirection indicates the direction of an issue link.
type LinkDirection int

const (
	// LinkInward means this issue is the inward side (e.g., "is blocked by").
	LinkInward LinkDirection = iota
	// LinkOutward means this issue is the outward side (e.g., "blocks").
	LinkOutward
)

// ImportResult summarizes the outcome of a Jira import operation.
type ImportResult struct {
	TasksCreated       int
	TasksUpdated       int
	TasksSkipped       int
	InitiativesCreated int
	InitiativesUpdated int
	Errors             []ImportError
}

// ImportError records a failure to import a specific Jira issue.
type ImportError struct {
	JiraKey string
	Err     error
}

func (e ImportError) Error() string {
	return e.JiraKey + ": " + e.Err.Error()
}
