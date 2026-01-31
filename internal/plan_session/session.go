// Package plan_session provides interactive planning sessions for tasks and features.
// It spawns Claude Code to collaboratively create specifications with the user.
package plan_session

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/initiative"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
)

// Mode determines how the planning session operates.
type Mode string

const (
	// ModeTask refines an existing task.
	ModeTask Mode = "task"
	// ModeFeature creates a new feature spec (optionally generates tasks).
	ModeFeature Mode = "feature"
	// ModeInteractive prompts the user for what they want to plan.
	ModeInteractive Mode = "interactive"
)

// taskIDPattern matches task IDs like TASK-001, TASK-123, etc.
var taskIDPattern = regexp.MustCompile(`^TASK-\d+$`)

// Options configures a planning session.
type Options struct {
	// WorkDir is the working directory for the session.
	WorkDir string

	// Model is the Claude model to use.
	Model string

	// InitiativeID links the plan to an existing initiative.
	InitiativeID string

	// Weight pre-sets the task weight (skips asking).
	Weight string

	// CreateTasks determines if tasks should be created from spec output (feature mode).
	CreateTasks bool

	// DryRun shows the prompt without running Claude.
	DryRun bool

	// SkipValidation skips spec validation after session.
	SkipValidation bool

	// Shared indicates if initiative is in the shared directory.
	Shared bool

	// Backend is the storage backend for tasks and initiatives.
	Backend storage.Backend
}

// Result contains the outcome of a planning session.
type Result struct {
	// Mode is the mode that was used.
	Mode Mode

	// TaskID is the task that was planned (task mode only).
	TaskID string

	// SpecPath is the path to the generated spec file.
	SpecPath string

	// TaskIDs contains IDs of created tasks (feature mode with CreateTasks).
	TaskIDs []string

	// ValidationResult contains spec validation outcome.
	ValidationResult *task.SpecValidation
}

// DetectMode determines the planning mode from the target argument.
func DetectMode(target string, backend storage.Backend) (Mode, string, error) {
	if target == "" {
		return ModeInteractive, "", nil
	}

	// Check if it's an existing task
	if backend != nil {
		exists, _ := backend.TaskExists(target)
		if exists {
			return ModeTask, target, nil
		}
	}

	// Check if it looks like a task ID but doesn't exist
	if taskIDPattern.MatchString(target) {
		return "", "", fmt.Errorf("task %s not found", target)
	}

	// Treat as feature title
	return ModeFeature, target, nil
}

// Run executes an interactive planning session.
func Run(ctx context.Context, target string, opts Options) (*Result, error) {
	if opts.WorkDir == "" {
		var err error
		opts.WorkDir, err = config.FindProjectRoot()
		if err != nil {
			return nil, fmt.Errorf("WorkDir not specified and not in orc project: %w", err)
		}
	}

	// Detect mode
	mode, resolvedTarget, err := DetectMode(target, opts.Backend)
	if err != nil {
		return nil, err
	}

	// Handle interactive mode - prompt for target
	if mode == ModeInteractive {
		return nil, fmt.Errorf("interactive mode not yet implemented - please specify a task ID or feature title")
	}

	// Load detection if available (non-fatal - enhances prompts but not required)
	var detection *db.Detection
	pdb, err := db.OpenProject(opts.WorkDir)
	if err == nil {
		defer func() { _ = pdb.Close() }()
		var loadErr error
		detection, loadErr = pdb.LoadDetection()
		if loadErr != nil {
			slog.Debug("plan_session: could not load detection (non-fatal)", "error", loadErr)
		}
	}

	// Load initiative if specified
	var init *initiative.Initiative
	if opts.InitiativeID != "" && opts.Backend != nil {
		init, err = opts.Backend.LoadInitiative(opts.InitiativeID)
		if err != nil {
			return nil, fmt.Errorf("load initiative: %w", err)
		}
	}

	// Branch based on mode
	switch mode {
	case ModeTask:
		return runTaskMode(ctx, resolvedTarget, opts, detection, init)
	case ModeFeature:
		return runFeatureMode(ctx, resolvedTarget, opts, detection, init)
	default:
		return nil, fmt.Errorf("unknown mode: %s", mode)
	}
}

// runTaskMode handles planning for an existing task.
func runTaskMode(ctx context.Context, taskID string, opts Options, detection *db.Detection, init *initiative.Initiative) (*Result, error) {
	if opts.Backend == nil {
		return nil, fmt.Errorf("backend is required for task mode")
	}

	// Load the task
	t, err := opts.Backend.LoadTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("load task: %w", err)
	}

	// Override weight if specified
	if opts.Weight != "" {
		t.Weight = task.WeightToProto(opts.Weight)
	}

	// Get description as string (proto uses *string)
	description := ""
	if t.Description != nil {
		description = *t.Description
	}

	// Generate prompt
	prompt, err := GeneratePrompt(PromptData{
		Mode:        ModeTask,
		Title:       t.Title,
		TaskID:      t.Id,
		TaskWeight:  task.WeightFromProto(t.Weight),
		Description: description,
		WorkDir:     opts.WorkDir,
		Detection:   detection,
		Initiative:  init,
	})
	if err != nil {
		return nil, fmt.Errorf("generate prompt: %w", err)
	}

	// Dry run - just show the prompt
	if opts.DryRun {
		fmt.Printf("=== Plan Session Prompt ===\n\n%s\n", prompt)
		return &Result{Mode: ModeTask, TaskID: taskID}, nil
	}

	// Spawn Claude
	spawner := NewSpawner(SpawnerOptions{
		WorkDir:                    opts.WorkDir,
		Model:                      opts.Model,
		DangerouslySkipPermissions: true,
	})

	if err := spawner.RunInteractive(ctx, prompt); err != nil {
		return nil, fmt.Errorf("run claude: %w", err)
	}

	result := &Result{
		Mode:   ModeTask,
		TaskID: taskID,
	}

	// Check if spec exists in database
	specExists, _ := opts.Backend.SpecExistsForTask(taskID)
	if specExists {
		result.SpecPath = "(stored in database)"

		// Validate spec if not skipped
		if !opts.SkipValidation {
			specContent, err := opts.Backend.GetSpecForTask(taskID)
			if err == nil && specContent != "" {
				result.ValidationResult = ValidateSpec(specContent, t.Weight)
			}
		}

		// Update task status to planned (spec created)
		t.Status = orcv1.TaskStatus_TASK_STATUS_PLANNED
		if saveErr := opts.Backend.SaveTask(t); saveErr != nil {
			// Log but don't fail - spec was created successfully
			fmt.Fprintf(os.Stderr, "Warning: could not update task status: %v\n", saveErr)
		}
	}

	return result, nil
}

// runFeatureMode handles planning for a new feature.
func runFeatureMode(ctx context.Context, title string, opts Options, detection *db.Detection, init *initiative.Initiative) (*Result, error) {
	// Generate prompt
	prompt, err := GeneratePrompt(PromptData{
		Mode:        ModeFeature,
		Title:       title,
		WorkDir:     opts.WorkDir,
		Detection:   detection,
		Initiative:  init,
		CreateTasks: opts.CreateTasks,
	})
	if err != nil {
		return nil, fmt.Errorf("generate prompt: %w", err)
	}

	// Dry run - just show the prompt
	if opts.DryRun {
		fmt.Printf("=== Plan Session Prompt ===\n\n%s\n", prompt)
		return &Result{Mode: ModeFeature}, nil
	}

	// Spawn Claude
	spawner := NewSpawner(SpawnerOptions{
		WorkDir:                    opts.WorkDir,
		Model:                      opts.Model,
		DangerouslySkipPermissions: true,
	})

	if err := spawner.RunInteractive(ctx, prompt); err != nil {
		return nil, fmt.Errorf("run claude: %w", err)
	}

	result := &Result{
		Mode: ModeFeature,
	}

	// Find the spec file
	specPath := findFeatureSpecFile(opts.WorkDir, opts.InitiativeID)
	if specPath != "" {
		result.SpecPath = specPath
	}

	return result, nil
}

// findFeatureSpecFile looks for a recently created feature spec file.
func findFeatureSpecFile(workDir, initiativeID string) string {
	// Check initiative-specific location first
	if initiativeID != "" {
		initPath := filepath.Join(workDir, ".orc", "initiatives", initiativeID, "spec.md")
		if _, err := os.Stat(initPath); err == nil {
			return initPath
		}
		// Also check shared location
		sharedPath := filepath.Join(workDir, ".orc", "shared", "initiatives", initiativeID, "spec.md")
		if _, err := os.Stat(sharedPath); err == nil {
			return sharedPath
		}
	}

	// Check default spec location
	defaultPath := filepath.Join(workDir, ".orc", "specs")
	entries, err := os.ReadDir(defaultPath)
	if err != nil {
		return ""
	}

	// Find the most recent .md file
	var latestPath string
	var latestTime int64
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filepath.Ext(e.Name()) != ".md" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Unix() > latestTime {
			latestTime = info.ModTime().Unix()
			latestPath = filepath.Join(defaultPath, e.Name())
		}
	}

	return latestPath
}
