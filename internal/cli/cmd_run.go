// Package cli implements the orc command-line interface.
// This file contains the unified workflow-based run command.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/storage"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/internal/workflow"
)

// taskIDPattern matches task IDs like TASK-001, TASK-123, etc.
var taskIDPattern = regexp.MustCompile(`^TASK-\d+$`)

// newRunCmd creates the run command
func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <workflow> \"<prompt>\" | run <task-id>",
		Short: "Execute a workflow or resume a task",
		Long: `Execute a workflow with the given prompt, or run an existing task.

WORKFLOW EXECUTION (primary pattern):
  orc run <workflow> "<prompt>"

  Creates a new task and executes the specified workflow. The prompt describes
  what work to do.

  Built-in workflows:
    implement         Full workflow: spec, TDD, breakdown, implement, review, docs
    implement-small   Lightweight: tiny_spec, implement, review
    implement-trivial Minimal: tiny_spec, implement
    review            Code review only
    spec              Generate specification only
    docs              Documentation only
    qa                QA session

TASK EXECUTION (existing task):
  orc run <task-id>

  Runs an existing task using its assigned workflow_id.
  Task must have workflow_id set (via orc new --workflow or orc edit --workflow).

CONTEXT FLAGS:
  --task TASK-ID     Attach workflow to existing task
  --branch NAME      Run on existing branch (no task created)
  --pr NUMBER        Run on pull request branch

Examples:
  orc run implement "Add user authentication with JWT"
  orc run implement-small "Fix the login validation bug"
  orc run review --branch feature/auth
  orc run TASK-001
  orc run implement --task TASK-001 "Continue implementation"

See also:
  orc workflows    - List available workflows
  orc phases       - List phase templates
  orc runs         - View workflow run history
  orc show         - View task details`,
		Args: cobra.RangeArgs(1, 2),
		RunE: runRun,
	}

	// Context flags
	cmd.Flags().String("task", "", "Attach to existing task")
	cmd.Flags().String("branch", "", "Run on existing branch (no task)")
	cmd.Flags().Int("pr", 0, "Run on pull request branch")

	// Configuration flags
	cmd.Flags().StringP("instructions", "i", "", "Additional instructions for this run")
	cmd.Flags().StringP("category", "c", "feature", "Task category (feature, bug, refactor, chore, docs, test)")
	cmd.Flags().StringP("profile", "p", "", "Automation profile (auto, fast, safe, strict)")
	cmd.Flags().Bool("stream", false, "Stream Claude output in real-time")
	cmd.Flags().Bool("force", false, "Run despite incomplete dependencies")
	cmd.Flags().Bool("skip-gates", false, "Skip all gate evaluations during execution")

	return cmd
}

func runRun(cmd *cobra.Command, args []string) error {
	// Determine execution mode based on arguments
	var workflowID, prompt, existingTaskID string

	if len(args) == 1 {
		arg := args[0]
		if taskIDPattern.MatchString(arg) {
			// Legacy pattern: orc run TASK-001
			existingTaskID = arg
		} else {
			// Workflow without prompt - error
			return fmt.Errorf("missing prompt: orc run %s \"<prompt>\"", arg)
		}
	} else {
		// orc run <workflow> "prompt"
		workflowID = args[0]
		prompt = args[1]
	}

	// Get flags
	taskFlag, _ := cmd.Flags().GetString("task")
	branch, _ := cmd.Flags().GetString("branch")
	prNum, _ := cmd.Flags().GetInt("pr")
	instructions, _ := cmd.Flags().GetString("instructions")
	categoryStr, _ := cmd.Flags().GetString("category")
	profile, _ := cmd.Flags().GetString("profile")
	stream, _ := cmd.Flags().GetBool("stream")
	force, _ := cmd.Flags().GetBool("force")
	skipGates, _ := cmd.Flags().GetBool("skip-gates")

	// Handle --task flag
	if taskFlag != "" {
		existingTaskID = taskFlag
	}

	// Find project root (supports multi-project via --project flag)
	projectRoot, err := ResolveProjectPath()
	if err != nil {
		return err
	}

	// Load config
	orcConfig, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Apply profile if specified
	if profile != "" {
		orcConfig.ApplyProfile(config.AutomationProfile(profile))
	}

	// Open databases
	pdb, err := db.OpenProject(projectRoot)
	if err != nil {
		return fmt.Errorf("open project database: %w", err)
	}
	defer func() { _ = pdb.Close() }()

	// Open global DB for workflows and agents
	gdb, err := db.OpenGlobal()
	if err != nil {
		return fmt.Errorf("open global database: %w", err)
	}
	defer func() { _ = gdb.Close() }()

	// Seed built-in workflows if not already seeded (into global DB)
	if _, err := workflow.SeedBuiltins(gdb); err != nil {
		return fmt.Errorf("seed workflows: %w", err)
	}

	// Seed built-in agents and phase-agent associations
	if _, err := workflow.SeedAgents(gdb); err != nil {
		return fmt.Errorf("seed agents: %w", err)
	}

	// Get backend
	backend, err := getBackend()
	if err != nil {
		return fmt.Errorf("get backend: %w", err)
	}
	defer func() { _ = backend.Close() }()

	// If we have an existing task, load it and use its workflow_id
	var existingTask *orcv1.Task
	if existingTaskID != "" {
		existingTask, err = backend.LoadTask(existingTaskID)
		if err != nil {
			return fmt.Errorf("load task: %w", err)
		}

		// Check task status
		if err := checkTaskCanRunProto(existingTask, force); err != nil {
			return err
		}

		// Check dependencies
		if err := checkTaskDependenciesProto(backend, existingTask, force); err != nil {
			return err
		}

		// If no workflow specified, use task's workflow - MUST be set
		if workflowID == "" {
			workflowID = task.GetWorkflowIDProto(existingTask)
			if workflowID == "" {
				return fmt.Errorf("task %s has no workflow_id set - cannot run\n\nSet workflow with: orc edit %s --workflow <workflow-id>\nSee available workflows: orc workflows", existingTaskID, existingTaskID)
			}
		}

		// Use task description as prompt if not provided
		if prompt == "" {
			prompt = task.GetDescriptionProto(existingTask)
		}
	}

	// Verify workflow exists (in global DB)
	wf, err := gdb.GetWorkflow(workflowID)
	if err != nil {
		return fmt.Errorf("get workflow: %w", err)
	}
	if wf == nil {
		return fmt.Errorf("workflow not found: %s\n\nRun 'orc workflows' to see available workflows", workflowID)
	}

	// Determine context type
	contextType := executor.ContextDefault
	if existingTaskID != "" {
		contextType = executor.ContextTask
	} else if branch != "" {
		contextType = executor.ContextBranch
	} else if prNum > 0 {
		contextType = executor.ContextPR
	}

	// Parse category
	category := task.CategoryToProto(categoryStr)
	if !task.IsValidCategoryProto(category) {
		return fmt.Errorf("invalid category: %s (valid: feature, bug, refactor, chore, docs, test)", categoryStr)
	}

	// Create workflow executor
	gitOps, err := NewGitOpsFromConfig(projectRoot, orcConfig)
	if err != nil {
		return fmt.Errorf("init git: %w", err)
	}

	claudePath := orcConfig.ClaudePath
	if claudePath == "" {
		claudePath = "claude"
	}

	// Build executor options
	execOpts := []executor.WorkflowExecutorOption{
		executor.WithWorkflowGitOps(gitOps),
		executor.WithWorkflowClaudePath(claudePath),
	}

	if skipGates {
		execOpts = append(execOpts, executor.WithSkipGates(true))
	}

	// Create persistent publisher for database event logging
	// CLI always persists events to enable `orc log` and event history
	persistentPub := events.NewPersistentPublisher(backend, "cli", nil)
	defer persistentPub.Close()

	// Add streaming CLI output if requested, wrapping persistent publisher
	if verbose || stream {
		cliPub := events.NewCLIPublisher(os.Stdout,
			events.WithStreamMode(true),
			events.WithInnerPublisher(persistentPub),
		)
		execOpts = append(execOpts, executor.WithWorkflowPublisher(cliPub))
		defer cliPub.Close()
	} else {
		// No streaming output, but still persist events
		execOpts = append(execOpts, executor.WithWorkflowPublisher(persistentPub))
	}

	we := executor.NewWorkflowExecutor(
		backend,
		pdb,
		orcConfig,
		projectRoot,
		execOpts...,
	)

	// Build run options
	opts := executor.WorkflowRunOptions{
		ContextType:  contextType,
		Prompt:       prompt,
		Instructions: instructions,
		TaskID:       existingTaskID,
		Branch:       branch,
		PRID:         prNum,
		Category:     category,
		Stream:       stream,
	}

	// Setup signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		if !quiet {
			fmt.Println("\n⚠️  Interrupt received, saving state...")
		}
		cancel()
	}()

	// Create progress display
	taskID := existingTaskID
	if taskID == "" {
		taskID = "NEW"
	}
	disp := progress.New(taskID, quiet)
	disp.Info(fmt.Sprintf("Running workflow: %s [profile: %s]", workflowID, orcConfig.Profile))
	if prompt != "" && len(prompt) <= 60 {
		disp.Info(fmt.Sprintf("Prompt: %s", prompt))
	}

	// Log worktree path
	if orcConfig.Worktree.Enabled {
		resolvedDir := config.ResolveWorktreeDir(orcConfig.Worktree.Dir, projectRoot)
		worktreePath := filepath.Join(resolvedDir, "orc-"+taskID)
		fmt.Fprintf(os.Stderr, "Worktree: %s\n", worktreePath)
	}

	// Execute workflow
	result, err := we.Run(ctx, workflowID, opts)
	if err != nil {
		if ctx.Err() != nil {
			disp.TaskInterrupted()
			fmt.Println("\nUse 'orc runs' to see status, 'orc resume' to continue.")
			return nil
		}
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	// Display results
	if !quiet {
		fmt.Println()
		fmt.Printf("✅ Workflow completed: %s\n", result.RunID)
		fmt.Printf("  Phases completed: %d\n", len(result.PhaseResults))
		fmt.Printf("  Total cost: $%.4f\n", result.TotalCostUSD)
		fmt.Printf("  Total tokens: %d\n", result.TotalTokens)

		if result.TaskID != "" {
			fmt.Printf("\n  Task: %s\n", result.TaskID)
			fmt.Println("  Use 'orc show " + result.TaskID + "' to see task details")
		}
	}

	return nil
}

// checkTaskCanRunProto verifies that a task is in a runnable state.
func checkTaskCanRunProto(t *orcv1.Task, force bool) error {
	if task.CanRunProto(t) || t.Status == orcv1.TaskStatus_TASK_STATUS_RUNNING {
		return nil
	}

	switch t.Status {
	case orcv1.TaskStatus_TASK_STATUS_PAUSED:
		return fmt.Errorf("task %s is paused\n\nTo resume: orc resume %s", t.Id, t.Id)
	case orcv1.TaskStatus_TASK_STATUS_BLOCKED:
		return fmt.Errorf("task %s is blocked and needs user input\n\nTo view: orc show %s", t.Id, t.Id)
	case orcv1.TaskStatus_TASK_STATUS_COMPLETED:
		if force {
			return nil
		}
		return fmt.Errorf("task %s is already completed\n\nTo rerun: use --force flag", t.Id)
	case orcv1.TaskStatus_TASK_STATUS_FAILED:
		return fmt.Errorf("task %s has failed\n\nTo resume: orc resume %s\nTo view log: orc log %s", t.Id, t.Id, t.Id)
	default:
		return fmt.Errorf("task cannot be run (status: %s)", task.StatusFromProto(t.Status))
	}
}

// checkTaskDependenciesProto verifies that task dependencies are satisfied.
func checkTaskDependenciesProto(backend storage.Backend, t *orcv1.Task, force bool) error {
	if len(t.BlockedBy) == 0 {
		return nil
	}

	// Load all tasks to check blocker status
	allTasks, err := backend.LoadAllTasks()
	if err != nil {
		return fmt.Errorf("load tasks for dependency check: %w", err)
	}

	// Build task map
	taskMap := make(map[string]*orcv1.Task)
	for _, tsk := range allTasks {
		taskMap[tsk.Id] = tsk
	}

	// Get incomplete blockers
	blockers := task.GetIncompleteBlockersProto(t, taskMap)
	if len(blockers) == 0 {
		return nil
	}

	if force {
		if !quiet {
			fmt.Printf("\n⚠️  Running despite incomplete dependencies:\n")
			for _, b := range blockers {
				fmt.Printf("    - %s: %s (%s)\n", b.ID, b.Title, b.Status)
			}
			fmt.Println()
		}
		return nil
	}

	fmt.Printf("\n⚠️  This task is blocked by incomplete tasks:\n")
	for _, b := range blockers {
		fmt.Printf("    - %s: %s (%s)\n", b.ID, b.Title, b.Status)
	}
	fmt.Println("\nUse --force to run anyway")
	return fmt.Errorf("task is blocked by incomplete dependencies")
}
