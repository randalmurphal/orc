package jira

import (
	"testing"
	"time"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
)

func TestMapPriority(t *testing.T) {
	tests := []struct {
		input    string
		expected orcv1.TaskPriority
	}{
		{"Highest", orcv1.TaskPriority_TASK_PRIORITY_CRITICAL},
		{"highest", orcv1.TaskPriority_TASK_PRIORITY_CRITICAL},
		{"High", orcv1.TaskPriority_TASK_PRIORITY_HIGH},
		{"Medium", orcv1.TaskPriority_TASK_PRIORITY_NORMAL},
		{"Low", orcv1.TaskPriority_TASK_PRIORITY_LOW},
		{"Lowest", orcv1.TaskPriority_TASK_PRIORITY_LOW},
		{"", orcv1.TaskPriority_TASK_PRIORITY_NORMAL},
		{"Unknown", orcv1.TaskPriority_TASK_PRIORITY_NORMAL},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapPriority(tt.input)
			if got != tt.expected {
				t.Errorf("mapPriority(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapCategory(t *testing.T) {
	tests := []struct {
		input    string
		expected orcv1.TaskCategory
	}{
		{"Bug", orcv1.TaskCategory_TASK_CATEGORY_BUG},
		{"bug", orcv1.TaskCategory_TASK_CATEGORY_BUG},
		{"Story", orcv1.TaskCategory_TASK_CATEGORY_FEATURE},
		{"Task", orcv1.TaskCategory_TASK_CATEGORY_FEATURE},
		{"Epic", orcv1.TaskCategory_TASK_CATEGORY_FEATURE},
		{"Sub-task", orcv1.TaskCategory_TASK_CATEGORY_CHORE},
		{"subtask", orcv1.TaskCategory_TASK_CATEGORY_CHORE},
		{"Improvement", orcv1.TaskCategory_TASK_CATEGORY_REFACTOR},
		{"", orcv1.TaskCategory_TASK_CATEGORY_FEATURE},
		{"CustomType", orcv1.TaskCategory_TASK_CATEGORY_FEATURE},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapCategory(tt.input)
			if got != tt.expected {
				t.Errorf("mapCategory(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected orcv1.TaskStatus
	}{
		{"new", orcv1.TaskStatus_TASK_STATUS_CREATED},
		{"indeterminate", orcv1.TaskStatus_TASK_STATUS_CREATED},
		{"done", orcv1.TaskStatus_TASK_STATUS_COMPLETED},
		{"undefined", orcv1.TaskStatus_TASK_STATUS_CREATED},
		{"", orcv1.TaskStatus_TASK_STATUS_CREATED},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapStatus(tt.input)
			if got != tt.expected {
				t.Errorf("mapStatus(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapQueue(t *testing.T) {
	tests := []struct {
		input        string
		defaultQueue orcv1.TaskQueue
		expected     orcv1.TaskQueue
	}{
		{"new", orcv1.TaskQueue_TASK_QUEUE_BACKLOG, orcv1.TaskQueue_TASK_QUEUE_BACKLOG},
		{"indeterminate", orcv1.TaskQueue_TASK_QUEUE_BACKLOG, orcv1.TaskQueue_TASK_QUEUE_ACTIVE},
		{"done", orcv1.TaskQueue_TASK_QUEUE_BACKLOG, orcv1.TaskQueue_TASK_QUEUE_ACTIVE},
		{"new", orcv1.TaskQueue_TASK_QUEUE_ACTIVE, orcv1.TaskQueue_TASK_QUEUE_ACTIVE},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapQueue(tt.input, tt.defaultQueue)
			if got != tt.expected {
				t.Errorf("mapQueue(%q, %v) = %v, want %v", tt.input, tt.defaultQueue, got, tt.expected)
			}
		})
	}
}

func TestMapInitiativeStatus(t *testing.T) {
	tests := []struct {
		input    string
		expected initiative.Status
	}{
		{"done", initiative.StatusCompleted},
		{"indeterminate", initiative.StatusActive},
		{"new", initiative.StatusDraft},
		{"", initiative.StatusDraft},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapInitiativeStatus(tt.input)
			if got != tt.expected {
				t.Errorf("mapInitiativeStatus(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestMapIssueToTask(t *testing.T) {
	mapper := NewMapper(DefaultMapperConfig())
	now := time.Now()

	issue := Issue{
		Key:         "PROJ-123",
		Summary:     "Fix login bug",
		Description: "The login page crashes on submit",
		IssueType:   "Bug",
		Status:      "To Do",
		StatusKey:   "new",
		Priority:    "High",
		Labels:      []string{"frontend", "urgent"},
		Components:  []string{"auth", "web"},
		Created:     now,
		Updated:     now,
	}

	task := mapper.MapIssueToTask(issue, "TASK-001")

	if task.Id != "TASK-001" {
		t.Errorf("ID = %q, want TASK-001", task.Id)
	}
	if task.Title != "Fix login bug" {
		t.Errorf("Title = %q, want %q", task.Title, "Fix login bug")
	}
	if task.GetDescription() != "The login page crashes on submit" {
		t.Errorf("Description = %q, want %q", task.GetDescription(), "The login page crashes on submit")
	}
	if task.Weight != orcv1.TaskWeight_TASK_WEIGHT_MEDIUM {
		t.Errorf("Weight = %v, want MEDIUM", task.Weight)
	}
	if task.Status != orcv1.TaskStatus_TASK_STATUS_CREATED {
		t.Errorf("Status = %v, want CREATED", task.Status)
	}
	if task.Queue != orcv1.TaskQueue_TASK_QUEUE_BACKLOG {
		t.Errorf("Queue = %v, want BACKLOG", task.Queue)
	}
	if task.Priority != orcv1.TaskPriority_TASK_PRIORITY_HIGH {
		t.Errorf("Priority = %v, want HIGH", task.Priority)
	}
	if task.Category != orcv1.TaskCategory_TASK_CATEGORY_BUG {
		t.Errorf("Category = %v, want BUG", task.Category)
	}
	if task.Metadata["jira_key"] != "PROJ-123" {
		t.Errorf("Metadata[jira_key] = %q, want PROJ-123", task.Metadata["jira_key"])
	}
	if task.Metadata["jira_labels"] != "frontend,urgent" {
		t.Errorf("Metadata[jira_labels] = %q, want %q", task.Metadata["jira_labels"], "frontend,urgent")
	}
	if task.Metadata["jira_components"] != "auth,web" {
		t.Errorf("Metadata[jira_components] = %q, want %q", task.Metadata["jira_components"], "auth,web")
	}
	if task.Metadata["jira_status"] != "To Do" {
		t.Errorf("Metadata[jira_status] = %q, want %q", task.Metadata["jira_status"], "To Do")
	}
}

func TestMapIssueToTask_NoLabelsOrComponents(t *testing.T) {
	mapper := NewMapper(DefaultMapperConfig())

	issue := Issue{
		Key:       "PROJ-1",
		Summary:   "Minimal issue",
		StatusKey: "new",
	}

	task := mapper.MapIssueToTask(issue, "TASK-001")

	if _, ok := task.Metadata["jira_labels"]; ok {
		t.Error("Expected no jira_labels metadata for empty labels")
	}
	if _, ok := task.Metadata["jira_components"]; ok {
		t.Error("Expected no jira_components metadata for empty components")
	}
}

func TestMapEpicToInitiative(t *testing.T) {
	mapper := NewMapper(DefaultMapperConfig())

	epic := Issue{
		Key:         "PROJ-10",
		Summary:     "User Authentication",
		Description: "Implement JWT-based auth",
		IssueType:   "Epic",
		StatusKey:   "indeterminate",
	}

	init := mapper.MapEpicToInitiative(epic, "INIT-001")

	if init.ID != "INIT-001" {
		t.Errorf("ID = %q, want INIT-001", init.ID)
	}
	if init.Title != "User Authentication" {
		t.Errorf("Title = %q, want %q", init.Title, "User Authentication")
	}
	if init.Vision != "Implement JWT-based auth" {
		t.Errorf("Vision = %q, want %q", init.Vision, "Implement JWT-based auth")
	}
	if init.Status != initiative.StatusActive {
		t.Errorf("Status = %v, want Active", init.Status)
	}
}

func TestResolveLinks(t *testing.T) {
	mapper := NewMapper(DefaultMapperConfig())

	keyToTaskID := map[string]string{
		"PROJ-1": "TASK-001",
		"PROJ-2": "TASK-002",
		"PROJ-3": "TASK-003",
	}

	issue := Issue{
		Key: "PROJ-4",
		IssueLinks: []IssueLink{
			{Type: "Blocks", Direction: LinkInward, LinkedKey: "PROJ-1"},  // PROJ-4 is blocked by PROJ-1
			{Type: "Blocks", Direction: LinkOutward, LinkedKey: "PROJ-2"}, // PROJ-4 blocks PROJ-2 (not recorded on this task)
			{Type: "Relates", Direction: LinkOutward, LinkedKey: "PROJ-3"},
			{Type: "Blocks", Direction: LinkInward, LinkedKey: "PROJ-99"}, // Not in import set
		},
	}

	blockedBy, relatedTo := mapper.ResolveLinks(issue, keyToTaskID)

	if len(blockedBy) != 1 || blockedBy[0] != "TASK-001" {
		t.Errorf("blockedBy = %v, want [TASK-001]", blockedBy)
	}
	if len(relatedTo) != 1 || relatedTo[0] != "TASK-003" {
		t.Errorf("relatedTo = %v, want [TASK-003]", relatedTo)
	}
}

func TestResolveLinks_NoLinks(t *testing.T) {
	mapper := NewMapper(DefaultMapperConfig())

	issue := Issue{Key: "PROJ-1"}
	blockedBy, relatedTo := mapper.ResolveLinks(issue, map[string]string{})

	if blockedBy != nil {
		t.Errorf("blockedBy = %v, want nil", blockedBy)
	}
	if relatedTo != nil {
		t.Errorf("relatedTo = %v, want nil", relatedTo)
	}
}
