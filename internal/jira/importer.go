package jira

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
)

// ImportConfig controls the import operation.
type ImportConfig struct {
	// JQL is the JQL query to filter issues. Combined with Projects.
	JQL string
	// Projects is a list of Jira project keys to import from.
	Projects []string
	// EpicToInitiative enables epic→initiative mapping (default: true).
	EpicToInitiative bool
	// DryRun previews the import without saving anything.
	DryRun bool
	// MapperCfg controls field mapping.
	MapperCfg MapperConfig
}

// searchFunc is the function signature for fetching issues.
// Allows injection of test fakes.
type searchFunc func(ctx context.Context, jql string) ([]Issue, error)

// customFieldFunc is the function signature for fetching custom field values.
// Returns a map of issueKey → (metadataKey → stringValue).
type customFieldFunc func(ctx context.Context, jql string) (map[string]map[string]string, error)

// Importer orchestrates fetching Jira issues and saving them as orc tasks.
type Importer struct {
	client          *Client
	backend         storage.Backend
	mapper          *Mapper
	cfg             ImportConfig
	logger          *slog.Logger
	searchFunc      searchFunc
	customFieldFunc customFieldFunc
}

// NewImporter creates an Importer.
func NewImporter(client *Client, backend storage.Backend, cfg ImportConfig, logger *slog.Logger) *Importer {
	if logger == nil {
		logger = slog.Default()
	}
	imp := &Importer{
		client:  client,
		backend: backend,
		mapper:  NewMapper(cfg.MapperCfg),
		cfg:     cfg,
		logger:  logger,
	}
	imp.searchFunc = func(ctx context.Context, jql string) ([]Issue, error) {
		return client.SearchAllIssues(ctx, jql)
	}
	imp.customFieldFunc = func(ctx context.Context, jql string) (map[string]map[string]string, error) {
		return client.FetchCustomFields(ctx, jql)
	}
	return imp
}

// Run executes the import and returns the result.
func (imp *Importer) Run(ctx context.Context) (*ImportResult, error) {
	result := &ImportResult{}

	// 1. Build JQL
	jql := imp.buildJQL()
	imp.logger.Info("fetching issues from Jira", "jql", jql)

	// 2. Fetch all issues
	issues, err := imp.searchFunc(ctx, jql)
	if err != nil {
		return nil, fmt.Errorf("fetch jira issues: %w", err)
	}
	imp.logger.Info("fetched issues", "count", len(issues))

	if len(issues) == 0 {
		return result, nil
	}

	// 3. Fetch and merge custom field values (if configured)
	cfValues, err := imp.customFieldFunc(ctx, jql)
	if err != nil {
		return nil, fmt.Errorf("fetch custom fields: %w", err)
	}
	if cfValues != nil {
		for i := range issues {
			if vals, ok := cfValues[issues[i].Key]; ok {
				if issues[i].CustomFields == nil {
					issues[i].CustomFields = make(map[string]string)
				}
				for k, v := range vals {
					issues[i].CustomFields[k] = v
				}
			}
		}
		imp.logger.Info("merged custom fields", "issues_with_values", len(cfValues))
	}

	// 4. Build existing task index by jira_key for idempotency
	existingByKey, err := imp.buildExistingIndex()
	if err != nil {
		return nil, fmt.Errorf("build existing task index: %w", err)
	}

	// 5. Separate epics from regular issues
	var epics, regular []Issue
	for _, issue := range issues {
		if issue.IsEpic() {
			epics = append(epics, issue)
		} else {
			regular = append(regular, issue)
		}
	}

	// 6. Map epics to initiatives (if enabled)
	epicKeyToInitID := make(map[string]string)
	if imp.cfg.EpicToInitiative {
		err = imp.importEpics(ctx, epics, epicKeyToInitID, result)
		if err != nil {
			return nil, fmt.Errorf("import epics: %w", err)
		}
	}

	// 7. Map issues to tasks — first pass: create/update tasks
	keyToTaskID := make(map[string]string)
	var tasksToSave []*orcv1.Task
	var issuesForTasks []Issue

	for _, issue := range regular {
		task, action, err := imp.mapOrUpdateTask(issue, existingByKey)
		if err != nil {
			result.Errors = append(result.Errors, ImportError{JiraKey: issue.Key, Err: err})
			continue
		}

		// Link to initiative if epic mapping is enabled
		if imp.cfg.EpicToInitiative && issue.ParentKey != "" {
			if initID, ok := epicKeyToInitID[issue.ParentKey]; ok {
				task.InitiativeId = &initID
			}
		}

		keyToTaskID[issue.Key] = task.Id

		switch action {
		case actionCreate:
			result.TasksCreated++
		case actionUpdate:
			result.TasksUpdated++
		case actionSkip:
			result.TasksSkipped++
		}

		tasksToSave = append(tasksToSave, task)
		issuesForTasks = append(issuesForTasks, issue)
	}

	// 8. Resolve dependencies (second pass — needs full keyToTaskID)
	for i, issue := range issuesForTasks {
		blockedBy, relatedTo := imp.mapper.ResolveLinks(issue, keyToTaskID)
		if len(blockedBy) > 0 {
			tasksToSave[i].BlockedBy = blockedBy
		}
		if len(relatedTo) > 0 {
			tasksToSave[i].RelatedTo = relatedTo
		}
	}

	// 9. Save tasks
	if !imp.cfg.DryRun {
		for _, task := range tasksToSave {
			if err := imp.backend.SaveTask(task); err != nil {
				result.Errors = append(result.Errors, ImportError{
					JiraKey: task.Metadata["jira_key"],
					Err:     fmt.Errorf("save task %s: %w", task.Id, err),
				})
			}
		}
	}

	return result, nil
}

type importAction int

const (
	actionCreate importAction = iota
	actionUpdate
	actionSkip
)

// mapOrUpdateTask creates a new task or updates an existing one.
func (imp *Importer) mapOrUpdateTask(issue Issue, existingByKey map[string]*orcv1.Task) (*orcv1.Task, importAction, error) {
	existing, found := existingByKey[issue.Key]

	if !found {
		// New task — allocate ID
		taskID, err := imp.backend.GetNextTaskID()
		if err != nil {
			return nil, 0, fmt.Errorf("allocate task ID: %w", err)
		}
		task := imp.mapper.MapIssueToTask(issue, taskID)
		return task, actionCreate, nil
	}

	// Existing task — update fields that Jira owns, preserve orc-specific state
	task := existing

	// Only update if the task hasn't been started in orc
	if isOrcStarted(task) {
		// Task is actively being worked on in orc — don't overwrite
		return task, actionSkip, nil
	}

	// Update Jira-owned fields using the mapper's resolve methods for override support
	updated := imp.mapper.MapIssueToTask(issue, task.Id)

	// Preserve orc-specific fields that Jira doesn't own
	updated.InitiativeId = task.InitiativeId
	updated.BlockedBy = task.BlockedBy
	updated.RelatedTo = task.RelatedTo
	updated.Execution = task.Execution

	return updated, actionUpdate, nil
}

// isOrcStarted returns true if the task has been started in orc (beyond initial import state).
func isOrcStarted(t *orcv1.Task) bool {
	switch t.Status {
	case orcv1.TaskStatus_TASK_STATUS_RUNNING,
		orcv1.TaskStatus_TASK_STATUS_PAUSED,
		orcv1.TaskStatus_TASK_STATUS_BLOCKED,
		orcv1.TaskStatus_TASK_STATUS_FINALIZING,
		orcv1.TaskStatus_TASK_STATUS_COMPLETED,
		orcv1.TaskStatus_TASK_STATUS_FAILED,
		orcv1.TaskStatus_TASK_STATUS_RESOLVED:
		return true
	default:
		return false
	}
}

// importEpics processes epics and saves them as initiatives.
func (imp *Importer) importEpics(_ context.Context, epics []Issue, epicKeyToInitID map[string]string, result *ImportResult) error {
	// Build existing initiative index by jira_key metadata
	// Initiatives don't have a metadata field, so we store the mapping
	// by checking if an initiative with the same title already exists.
	// For proper idempotency, we'd need to extend the initiative model,
	// but title-matching is a reasonable v1 approach.
	existingInits, err := imp.backend.LoadAllInitiatives()
	if err != nil {
		return fmt.Errorf("load existing initiatives: %w", err)
	}

	titleToInit := make(map[string]*initiative.Initiative)
	for _, init := range existingInits {
		titleToInit[init.Title] = init
	}

	for _, epic := range epics {
		existing, found := titleToInit[epic.Summary]

		if found {
			// Update existing initiative (matched by title — see comment above)
			imp.logger.Warn("matched epic to existing initiative by title",
				"jira_key", epic.Key, "title", epic.Summary, "initiative_id", existing.ID)
			existing.Vision = epic.Description
			existing.Status = mapInitiativeStatus(epic.StatusKey)
			epicKeyToInitID[epic.Key] = existing.ID

			if !imp.cfg.DryRun {
				if err := imp.backend.SaveInitiative(existing); err != nil {
					result.Errors = append(result.Errors, ImportError{
						JiraKey: epic.Key,
						Err:     fmt.Errorf("update initiative %s: %w", existing.ID, err),
					})
					continue
				}
			}
			result.InitiativesUpdated++
		} else {
			// New initiative
			initID, err := imp.backend.GetNextInitiativeID()
			if err != nil {
				result.Errors = append(result.Errors, ImportError{
					JiraKey: epic.Key,
					Err:     fmt.Errorf("allocate initiative ID: %w", err),
				})
				continue
			}

			init := imp.mapper.MapEpicToInitiative(epic, initID)
			epicKeyToInitID[epic.Key] = initID

			if !imp.cfg.DryRun {
				if err := imp.backend.SaveInitiative(init); err != nil {
					result.Errors = append(result.Errors, ImportError{
						JiraKey: epic.Key,
						Err:     fmt.Errorf("save initiative %s: %w", initID, err),
					})
					continue
				}
			}
			result.InitiativesCreated++
		}
	}

	return nil
}

// buildExistingIndex loads all tasks and indexes them by jira_key metadata.
func (imp *Importer) buildExistingIndex() (map[string]*orcv1.Task, error) {
	tasks, err := imp.backend.LoadAllTasks()
	if err != nil {
		return nil, fmt.Errorf("load existing tasks: %w", err)
	}

	index := make(map[string]*orcv1.Task)
	for _, t := range tasks {
		if t.Metadata != nil {
			if key, ok := t.Metadata["jira_key"]; ok {
				index[key] = t
			}
		}
	}

	return index, nil
}

// buildJQL constructs the JQL query from config.
func (imp *Importer) buildJQL() string {
	parts := make([]string, 0)

	if len(imp.cfg.Projects) > 0 {
		if len(imp.cfg.Projects) == 1 {
			parts = append(parts, fmt.Sprintf("project = %q", imp.cfg.Projects[0]))
		} else {
			parts = append(parts, fmt.Sprintf("project in (%s)", joinQuoted(imp.cfg.Projects)))
		}
	}

	if imp.cfg.JQL != "" {
		parts = append(parts, imp.cfg.JQL)
	}

	if len(parts) == 0 {
		return "ORDER BY created DESC"
	}

	return strings.Join(parts, " AND ") + " ORDER BY created ASC"
}

func joinQuoted(strs []string) string {
	quoted := make([]string, len(strs))
	for i, s := range strs {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}

