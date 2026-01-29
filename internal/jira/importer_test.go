package jira

import (
	"context"
	"log/slog"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
)

// mockBackend implements a minimal storage.Backend for testing.
// Only the methods used by the importer are implemented.
type mockBackend struct {
	storage.Backend // Embed to satisfy interface; unused methods panic

	tasks       []*orcv1.Task
	initiatives []*initiative.Initiative
	nextTaskID  int
	nextInitID  int
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		nextTaskID: 1,
		nextInitID: 1,
	}
}

func (m *mockBackend) LoadAllTasks() ([]*orcv1.Task, error) {
	return m.tasks, nil
}

func (m *mockBackend) SaveTask(t *orcv1.Task) error {
	// Update existing or append
	for i, existing := range m.tasks {
		if existing.Id == t.Id {
			m.tasks[i] = t
			return nil
		}
	}
	m.tasks = append(m.tasks, t)
	return nil
}

func (m *mockBackend) GetNextTaskID() (string, error) {
	id := m.nextTaskID
	m.nextTaskID++
	return taskIDFromInt(id), nil
}

func (m *mockBackend) LoadAllInitiatives() ([]*initiative.Initiative, error) {
	return m.initiatives, nil
}

func (m *mockBackend) SaveInitiative(i *initiative.Initiative) error {
	for idx, existing := range m.initiatives {
		if existing.ID == i.ID {
			m.initiatives[idx] = i
			return nil
		}
	}
	m.initiatives = append(m.initiatives, i)
	return nil
}

func (m *mockBackend) GetNextInitiativeID() (string, error) {
	id := m.nextInitID
	m.nextInitID++
	return initIDFromInt(id), nil
}

func taskIDFromInt(n int) string {
	return "TASK-" + padInt(n)
}

func initIDFromInt(n int) string {
	return "INIT-" + padInt(n)
}

func padInt(n int) string {
	s := ""
	for i := 100; i >= 1; i /= 10 {
		if n < i {
			s += "0"
		}
	}
	s += intToStr(n)
	return s
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

func TestImporter_BasicImport(t *testing.T) {
	backend := newMockBackend()
	logger := slog.Default()

	issues := []Issue{
		{
			Key:       "PROJ-1",
			Summary:   "First task",
			IssueType: "Story",
			StatusKey: "new",
			Priority:  "High",
		},
		{
			Key:       "PROJ-2",
			Summary:   "Second task",
			IssueType: "Bug",
			StatusKey: "indeterminate",
			Priority:  "Medium",
		},
	}

	cfg := ImportConfig{
		EpicToInitiative: false,
		MapperCfg:        DefaultMapperConfig(),
	}

	imp := newTestImporter(backend, cfg, logger, issues)
	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.TasksCreated != 2 {
		t.Errorf("TasksCreated = %d, want 2", result.TasksCreated)
	}
	if result.TasksUpdated != 0 {
		t.Errorf("TasksUpdated = %d, want 0", result.TasksUpdated)
	}
	if len(result.Errors) != 0 {
		t.Errorf("Errors = %v, want none", result.Errors)
	}
	if len(backend.tasks) != 2 {
		t.Errorf("backend.tasks = %d, want 2", len(backend.tasks))
	}

	// Verify task fields
	task := backend.tasks[0]
	if task.Metadata["jira_key"] != "PROJ-1" {
		t.Errorf("task[0].Metadata[jira_key] = %q, want PROJ-1", task.Metadata["jira_key"])
	}
}

func TestImporter_Idempotency(t *testing.T) {
	backend := newMockBackend()
	logger := slog.Default()

	issues := []Issue{
		{
			Key:       "PROJ-1",
			Summary:   "Original title",
			IssueType: "Story",
			StatusKey: "new",
			Priority:  "High",
		},
	}

	cfg := ImportConfig{
		EpicToInitiative: false,
		MapperCfg:        DefaultMapperConfig(),
	}

	// First import
	imp := newTestImporter(backend, cfg, logger, issues)
	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("First Run() error: %v", err)
	}
	if result.TasksCreated != 1 {
		t.Errorf("First import: TasksCreated = %d, want 1", result.TasksCreated)
	}

	// Second import with updated title
	issues[0].Summary = "Updated title"
	imp = newTestImporter(backend, cfg, logger, issues)
	result, err = imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Second Run() error: %v", err)
	}
	if result.TasksUpdated != 1 {
		t.Errorf("Second import: TasksUpdated = %d, want 1", result.TasksUpdated)
	}
	if result.TasksCreated != 0 {
		t.Errorf("Second import: TasksCreated = %d, want 0", result.TasksCreated)
	}

	// Verify only one task exists (no duplicates)
	if len(backend.tasks) != 1 {
		t.Errorf("backend.tasks = %d, want 1", len(backend.tasks))
	}
	if backend.tasks[0].Title != "Updated title" {
		t.Errorf("task.Title = %q, want %q", backend.tasks[0].Title, "Updated title")
	}
}

func TestImporter_SkipsStartedTasks(t *testing.T) {
	backend := newMockBackend()
	logger := slog.Default()

	// Pre-populate with a task that's already running in orc
	backend.tasks = append(backend.tasks, &orcv1.Task{
		Id:     "TASK-001",
		Title:  "Old title",
		Status: orcv1.TaskStatus_TASK_STATUS_RUNNING,
		Metadata: map[string]string{
			"jira_key": "PROJ-1",
		},
	})
	backend.nextTaskID = 2

	issues := []Issue{
		{
			Key:       "PROJ-1",
			Summary:   "New title from Jira",
			IssueType: "Story",
			StatusKey: "indeterminate",
			Priority:  "High",
		},
	}

	cfg := ImportConfig{
		EpicToInitiative: false,
		MapperCfg:        DefaultMapperConfig(),
	}

	imp := newTestImporter(backend, cfg, logger, issues)
	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.TasksSkipped != 1 {
		t.Errorf("TasksSkipped = %d, want 1", result.TasksSkipped)
	}
	// Title should NOT be updated
	if backend.tasks[0].Title != "Old title" {
		t.Errorf("task.Title = %q, want %q (should not update running task)", backend.tasks[0].Title, "Old title")
	}
}

func TestImporter_EpicToInitiative(t *testing.T) {
	backend := newMockBackend()
	logger := slog.Default()

	issues := []Issue{
		{
			Key:       "PROJ-10",
			Summary:   "Auth Epic",
			IssueType: "Epic",
			StatusKey: "indeterminate",
		},
		{
			Key:       "PROJ-11",
			Summary:   "Login page",
			IssueType: "Story",
			StatusKey: "new",
			Priority:  "High",
			ParentKey: "PROJ-10", // Child of the epic
		},
	}

	cfg := ImportConfig{
		EpicToInitiative: true,
		MapperCfg:        DefaultMapperConfig(),
	}

	imp := newTestImporter(backend, cfg, logger, issues)
	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.InitiativesCreated != 1 {
		t.Errorf("InitiativesCreated = %d, want 1", result.InitiativesCreated)
	}
	if result.TasksCreated != 1 {
		t.Errorf("TasksCreated = %d, want 1", result.TasksCreated)
	}

	// Verify initiative was created
	if len(backend.initiatives) != 1 {
		t.Fatalf("backend.initiatives = %d, want 1", len(backend.initiatives))
	}
	init := backend.initiatives[0]
	if init.Title != "Auth Epic" {
		t.Errorf("initiative.Title = %q, want %q", init.Title, "Auth Epic")
	}

	// Verify task is linked to initiative
	if len(backend.tasks) != 1 {
		t.Fatalf("backend.tasks = %d, want 1", len(backend.tasks))
	}
	task := backend.tasks[0]
	if task.GetInitiativeId() != init.ID {
		t.Errorf("task.InitiativeId = %q, want %q", task.GetInitiativeId(), init.ID)
	}
}

func TestImporter_DependencyResolution(t *testing.T) {
	backend := newMockBackend()
	logger := slog.Default()

	issues := []Issue{
		{
			Key:       "PROJ-1",
			Summary:   "First task",
			IssueType: "Story",
			StatusKey: "new",
			Priority:  "Medium",
		},
		{
			Key:       "PROJ-2",
			Summary:   "Depends on first",
			IssueType: "Story",
			StatusKey: "new",
			Priority:  "Medium",
			IssueLinks: []IssueLink{
				{Type: "Blocks", Direction: LinkInward, LinkedKey: "PROJ-1"},
			},
		},
		{
			Key:       "PROJ-3",
			Summary:   "Related to first",
			IssueType: "Story",
			StatusKey: "new",
			Priority:  "Medium",
			IssueLinks: []IssueLink{
				{Type: "Relates", Direction: LinkOutward, LinkedKey: "PROJ-1"},
			},
		},
	}

	cfg := ImportConfig{
		EpicToInitiative: false,
		MapperCfg:        DefaultMapperConfig(),
	}

	imp := newTestImporter(backend, cfg, logger, issues)
	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.TasksCreated != 3 {
		t.Errorf("TasksCreated = %d, want 3", result.TasksCreated)
	}

	// Find PROJ-2 task
	var proj2Task *orcv1.Task
	for _, task := range backend.tasks {
		if task.Metadata["jira_key"] == "PROJ-2" {
			proj2Task = task
			break
		}
	}
	if proj2Task == nil {
		t.Fatal("PROJ-2 task not found")
	}

	// Find PROJ-1 task ID
	var proj1ID string
	for _, task := range backend.tasks {
		if task.Metadata["jira_key"] == "PROJ-1" {
			proj1ID = task.Id
			break
		}
	}

	if len(proj2Task.BlockedBy) != 1 || proj2Task.BlockedBy[0] != proj1ID {
		t.Errorf("PROJ-2 BlockedBy = %v, want [%s]", proj2Task.BlockedBy, proj1ID)
	}

	// Find PROJ-3 task
	var proj3Task *orcv1.Task
	for _, task := range backend.tasks {
		if task.Metadata["jira_key"] == "PROJ-3" {
			proj3Task = task
			break
		}
	}
	if proj3Task == nil {
		t.Fatal("PROJ-3 task not found")
	}
	if len(proj3Task.RelatedTo) != 1 || proj3Task.RelatedTo[0] != proj1ID {
		t.Errorf("PROJ-3 RelatedTo = %v, want [%s]", proj3Task.RelatedTo, proj1ID)
	}
}

func TestImporter_DryRun(t *testing.T) {
	backend := newMockBackend()
	logger := slog.Default()

	issues := []Issue{
		{
			Key:       "PROJ-1",
			Summary:   "Should not save",
			IssueType: "Story",
			StatusKey: "new",
			Priority:  "Medium",
		},
	}

	cfg := ImportConfig{
		EpicToInitiative: false,
		DryRun:           true,
		MapperCfg:        DefaultMapperConfig(),
	}

	imp := newTestImporter(backend, cfg, logger, issues)
	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.TasksCreated != 1 {
		t.Errorf("TasksCreated = %d, want 1 (counted even in dry-run)", result.TasksCreated)
	}

	// Nothing should be saved
	if len(backend.tasks) != 0 {
		t.Errorf("backend.tasks = %d, want 0 (dry-run should not save)", len(backend.tasks))
	}
}

func TestImporter_EmptyResult(t *testing.T) {
	backend := newMockBackend()
	logger := slog.Default()

	cfg := ImportConfig{
		EpicToInitiative: false,
		MapperCfg:        DefaultMapperConfig(),
	}

	imp := newTestImporter(backend, cfg, logger, nil)
	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result.TasksCreated != 0 || result.TasksUpdated != 0 {
		t.Errorf("Expected zero counts for empty import")
	}
}

func TestBuildJQL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ImportConfig
		expected string
	}{
		{
			name:     "no filters",
			cfg:      ImportConfig{},
			expected: "ORDER BY created DESC",
		},
		{
			name: "single project",
			cfg: ImportConfig{
				Projects: []string{"PROJ"},
			},
			expected: `project = "PROJ" ORDER BY created ASC`,
		},
		{
			name: "multiple projects",
			cfg: ImportConfig{
				Projects: []string{"PROJ", "OTHER"},
			},
			expected: `project in ("PROJ", "OTHER") ORDER BY created ASC`,
		},
		{
			name: "jql only",
			cfg: ImportConfig{
				JQL: "sprint in openSprints()",
			},
			expected: "sprint in openSprints() ORDER BY created ASC",
		},
		{
			name: "project and jql combined",
			cfg: ImportConfig{
				Projects: []string{"PROJ"},
				JQL:      "status = Open",
			},
			expected: `project = "PROJ" AND status = Open ORDER BY created ASC`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imp := &Importer{cfg: tt.cfg}
			got := imp.buildJQL()
			if got != tt.expected {
				t.Errorf("buildJQL() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// newTestImporter creates an Importer with a fake client for testing.
// We bypass the real Client by injecting a custom search function.
func newTestImporter(backend *mockBackend, cfg ImportConfig, logger *slog.Logger, issues []Issue) *Importer {
	imp := &Importer{
		backend: backend,
		mapper:  NewMapper(cfg.MapperCfg),
		cfg:     cfg,
		logger:  logger,
	}
	// Override the client's search — we inject issues directly
	imp.searchFunc = func(_ context.Context, _ string) ([]Issue, error) {
		return issues, nil
	}
	// Default: no custom fields
	imp.customFieldFunc = func(_ context.Context, _ string) (map[string]map[string]string, error) {
		return nil, nil
	}
	return imp
}

func TestImporter_CustomFields(t *testing.T) {
	backend := newMockBackend()
	logger := slog.Default()

	issues := []Issue{
		{
			Key:       "PROJ-1",
			Summary:   "Task with custom fields",
			IssueType: "Story",
			StatusKey: "new",
			Priority:  "High",
		},
		{
			Key:       "PROJ-2",
			Summary:   "Task without custom fields",
			IssueType: "Bug",
			StatusKey: "new",
			Priority:  "Medium",
		},
	}

	cfg := ImportConfig{
		EpicToInitiative: false,
		MapperCfg:        DefaultMapperConfig(),
	}

	imp := newTestImporter(backend, cfg, logger, issues)
	// Override customFieldFunc to return values for PROJ-1
	imp.customFieldFunc = func(_ context.Context, _ string) (map[string]map[string]string, error) {
		return map[string]map[string]string{
			"PROJ-1": {
				"jira_sprint":       "Sprint 5",
				"jira_story_points": "8",
			},
		}, nil
	}

	result, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if result.TasksCreated != 2 {
		t.Errorf("TasksCreated = %d, want 2", result.TasksCreated)
	}

	// Find PROJ-1 task and verify custom fields in metadata
	var proj1Task *orcv1.Task
	for _, task := range backend.tasks {
		if task.Metadata["jira_key"] == "PROJ-1" {
			proj1Task = task
			break
		}
	}
	if proj1Task == nil {
		t.Fatal("PROJ-1 task not found")
	}
	if proj1Task.Metadata["jira_sprint"] != "Sprint 5" {
		t.Errorf("Metadata[jira_sprint] = %q, want %q", proj1Task.Metadata["jira_sprint"], "Sprint 5")
	}
	if proj1Task.Metadata["jira_story_points"] != "8" {
		t.Errorf("Metadata[jira_story_points] = %q, want %q", proj1Task.Metadata["jira_story_points"], "8")
	}

	// Find PROJ-2 task and verify no custom fields
	var proj2Task *orcv1.Task
	for _, task := range backend.tasks {
		if task.Metadata["jira_key"] == "PROJ-2" {
			proj2Task = task
			break
		}
	}
	if proj2Task == nil {
		t.Fatal("PROJ-2 task not found")
	}
	if _, ok := proj2Task.Metadata["jira_sprint"]; ok {
		t.Error("PROJ-2 should not have jira_sprint metadata")
	}
}
