// Package cli implements the orc command-line interface.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

// newInitCmd creates the init command
func newInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize orc in current project",
		Long: `Initialize orc configuration in the current directory.

Creates .orc/ directory with:
  • config.yaml - project configuration
  • tasks/ - task storage directory

Example:
  orc init
  orc init --force  # overwrite existing config`,
		RunE: func(cmd *cobra.Command, args []string) error {
			force, _ := cmd.Flags().GetBool("force")

			if err := config.Init(force); err != nil {
				return err
			}

			fmt.Println("✅ orc initialized successfully")
			fmt.Println("   Config: .orc/config.yaml")
			fmt.Println("   Tasks:  .orc/tasks/")
			fmt.Println("\nNext steps:")
			fmt.Println("  orc new \"Your task title\"  - Create a new task")

			return nil
		},
	}
	cmd.Flags().Bool("force", false, "overwrite existing configuration")
	return cmd
}

// newNewCmd creates the new task command
func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <title>",
		Short: "Create a new task",
		Long: `Create a new task to be orchestrated by orc.

The task will be classified by weight (trivial, small, medium, large, greenfield)
either automatically by AI or manually via --weight flag.

Example:
  orc new "Fix authentication timeout bug"
  orc new "Implement user dashboard" --weight large
  orc new "Create new microservice" --weight greenfield`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			title := args[0]
			weight, _ := cmd.Flags().GetString("weight")
			description, _ := cmd.Flags().GetString("description")

			// Generate next task ID
			id, err := task.NextID()
			if err != nil {
				return fmt.Errorf("generate task ID: %w", err)
			}

			// Create task
			t := task.New(id, title)
			if description != "" {
				t.Description = description
			}

			// Set weight
			if weight != "" {
				t.Weight = task.Weight(weight)
			} else {
				// Default to medium if not specified
				// TODO: Add AI classification
				t.Weight = task.WeightMedium
			}

			// Save task
			if err := t.Save(); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			// Create plan from template
			p, err := plan.CreateFromTemplate(t)
			if err != nil {
				// If template not found, use default plan
				fmt.Printf("⚠️  No template for weight %s, using default plan\n", t.Weight)
				p = &plan.Plan{
					Version:     1,
					TaskID:      id,
					Weight:      t.Weight,
					Description: "Default plan",
					Phases: []plan.Phase{
						{ID: "implement", Name: "implement", Gate: plan.Gate{Type: plan.GateAuto}, Status: plan.PhasePending},
					},
				}
			}

			// Save plan
			if err := p.Save(id); err != nil {
				return fmt.Errorf("save plan: %w", err)
			}

			// Update task status
			t.Status = task.StatusPlanned
			if err := t.Save(); err != nil {
				return fmt.Errorf("update task: %w", err)
			}

			fmt.Printf("✅ Task created: %s\n", id)
			fmt.Printf("   Title:  %s\n", title)
			fmt.Printf("   Weight: %s\n", t.Weight)
			fmt.Printf("   Phases: %d\n", len(p.Phases))
			fmt.Println("\nNext steps:")
			fmt.Printf("  orc run %s    - Execute the task\n", id)
			fmt.Printf("  orc show %s   - View task details\n", id)

			return nil
		},
	}
	cmd.Flags().StringP("weight", "w", "", "task weight (trivial, small, medium, large, greenfield)")
	cmd.Flags().StringP("description", "d", "", "task description")
	return cmd
}

// newListCmd creates the list command
func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tasks",
		Long: `List all tasks in the current project.

Example:
  orc list
  orc list --status running
  orc list --weight large`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			tasks, err := task.LoadAll()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			if len(tasks) == 0 {
				fmt.Println("No tasks found. Create one with: orc new \"Your task\"")
				return nil
			}

			// Print tasks in table format
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tWEIGHT\tPHASE\tTITLE")
			fmt.Fprintln(w, "──\t──────\t──────\t─────\t─────")

			for _, t := range tasks {
				status := statusIcon(t.Status)
				phase := t.CurrentPhase
				if phase == "" {
					phase = "-"
				}
				title := truncate(t.Title, 40)
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", t.ID, status, t.Weight, phase, title)
			}

			w.Flush()
			return nil
		},
	}
}

// newShowCmd creates the show command
func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <task-id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			p, _ := plan.Load(id)
			s, _ := state.Load(id)

			// Print task details
			fmt.Printf("\n%s - %s\n", t.ID, t.Title)
			fmt.Printf("────────────────────────────────────────────\n")
			fmt.Printf("Status:    %s\n", t.Status)
			fmt.Printf("Weight:    %s\n", t.Weight)
			fmt.Printf("Branch:    %s\n", t.Branch)
			fmt.Printf("Created:   %s\n", t.CreatedAt.Format(time.RFC3339))

			if t.StartedAt != nil {
				fmt.Printf("Started:   %s\n", t.StartedAt.Format(time.RFC3339))
			}
			if t.CompletedAt != nil {
				fmt.Printf("Completed: %s\n", t.CompletedAt.Format(time.RFC3339))
			}

			if t.Description != "" {
				fmt.Printf("\nDescription:\n%s\n", t.Description)
			}

			// Print phases
			if p != nil && len(p.Phases) > 0 {
				fmt.Printf("\nPhases:\n")
				for _, phase := range p.Phases {
					status := phaseStatusIcon(phase.Status)
					fmt.Printf("  %s %s", status, phase.ID)
					if phase.CommitSHA != "" {
						fmt.Printf(" (commit: %s)", phase.CommitSHA[:7])
					}
					fmt.Println()
				}
			}

			// Print execution state
			if s != nil && s.Tokens.TotalTokens > 0 {
				fmt.Printf("\nTokens Used: %d\n", s.Tokens.TotalTokens)
			}

			return nil
		},
	}
}

// newRunCmd creates the run command
func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <task-id>",
		Short: "Execute a task",
		Long: `Execute a task through its phases.

The task will be executed according to its plan (based on weight).
Each phase creates a git checkpoint for rewindability.

Automation profiles control gate behavior:
  auto   - Fully automated, no human intervention (default)
  fast   - Minimal gates, speed over safety
  safe   - AI reviews, human approval only for merge
  strict - Human gates on spec/review/merge

Example:
  orc run TASK-001
  orc run TASK-001 --profile safe
  orc run TASK-001 --phase implement  # run specific phase`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]
			profile, _ := cmd.Flags().GetString("profile")

			// Load task
			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			// Check if task can run
			if !t.CanRun() && t.Status != task.StatusRunning {
				return fmt.Errorf("task cannot be run (status: %s)", t.Status)
			}

			// Load plan
			p, err := plan.Load(id)
			if err != nil {
				return fmt.Errorf("load plan: %w", err)
			}

			// Load or create state
			s, err := state.Load(id)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}

			// Load config
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Apply profile if specified
			if profile != "" {
				cfg.ApplyProfile(config.AutomationProfile(profile))
			}

			// Set up signal handling for graceful shutdown
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\n⚠️  Interrupt received, saving state...")
				cancel()
			}()

			// Create progress display
			disp := progress.New(id, quiet)
			disp.Info(fmt.Sprintf("Starting task %s (%s) [profile: %s]", id, t.Weight, cfg.Profile))

			// Create executor with config
			exec := executor.NewWithConfig(executor.ConfigFromOrc(cfg), cfg)

			// Execute task
			err = exec.ExecuteTask(ctx, t, p, s)
			if err != nil {
				if ctx.Err() != nil {
					disp.TaskInterrupted()
					return nil // Clean interrupt
				}
				disp.TaskFailed(err)
				return err
			}

			disp.TaskComplete(s.Tokens.TotalTokens, time.Since(s.StartedAt))
			return nil
		},
	}
	cmd.Flags().String("phase", "", "run specific phase only")
	cmd.Flags().StringP("profile", "p", "", "automation profile (auto, fast, safe, strict)")
	cmd.Flags().Bool("continue", false, "continue from last checkpoint")
	return cmd
}

// newPauseCmd creates the pause command
func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <task-id>",
		Short: "Pause task execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status != task.StatusRunning {
				return fmt.Errorf("task is not running (status: %s)", t.Status)
			}

			t.Status = task.StatusPaused
			if err := t.Save(); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("⏸️  Task %s paused\n", id)
			fmt.Printf("   Resume with: orc resume %s\n", id)
			return nil
		},
	}
}

// newStopCmd creates the stop command
func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <task-id>",
		Short: "Stop task execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status == task.StatusCompleted {
				return fmt.Errorf("task is already completed")
			}

			t.Status = task.StatusFailed
			if err := t.Save(); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("🛑 Task %s stopped\n", id)
			return nil
		},
	}
}

// newResumeCmd creates the resume command
func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <task-id>",
		Short: "Resume a paused or interrupted task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status != task.StatusPaused && t.Status != task.StatusBlocked {
				return fmt.Errorf("task cannot be resumed (status: %s)", t.Status)
			}

			p, err := plan.Load(id)
			if err != nil {
				return fmt.Errorf("load plan: %w", err)
			}

			s, err := state.Load(id)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			// Set up signal handling
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				fmt.Println("\n⚠️  Interrupt received, saving state...")
				cancel()
			}()

			disp := progress.New(id, quiet)
			disp.Info(fmt.Sprintf("Resuming task %s", id))

			exec := executor.New(executor.ConfigFromOrc(cfg))

			// Find resume phase
			resumePhase := s.GetResumePhase()
			if resumePhase == "" {
				resumePhase = s.CurrentPhase
			}
			if resumePhase == "" {
				return fmt.Errorf("no phase to resume from")
			}

			err = exec.ResumeFromPhase(ctx, t, p, s, resumePhase)
			if err != nil {
				if ctx.Err() != nil {
					disp.TaskInterrupted()
					return nil
				}
				disp.TaskFailed(err)
				return err
			}

			disp.TaskComplete(s.Tokens.TotalTokens, time.Since(s.StartedAt))
			return nil
		},
	}
}

// newRewindCmd creates the rewind command
func newRewindCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rewind <task-id> --to <phase>",
		Short: "Rewind task to a checkpoint",
		Long: `Rewind a task to a previous checkpoint.

This uses git reset to restore the codebase state at that checkpoint.
All changes after that checkpoint will be lost.

Example:
  orc rewind TASK-001 --to spec
  orc rewind TASK-001 --to implement`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]
			toPhase, _ := cmd.Flags().GetString("to")

			if toPhase == "" {
				return fmt.Errorf("--to flag is required")
			}

			// Load plan to get commit SHA for the phase
			p, err := plan.Load(id)
			if err != nil {
				return fmt.Errorf("load plan: %w", err)
			}

			phase := p.GetPhase(toPhase)
			if phase == nil {
				return fmt.Errorf("phase %s not found", toPhase)
			}

			if phase.CommitSHA == "" {
				return fmt.Errorf("phase %s has no checkpoint", toPhase)
			}

			fmt.Printf("⚠️  This will reset to commit %s\n", phase.CommitSHA[:7])
			fmt.Println("   All changes after this point will be lost!")
			fmt.Print("   Continue? [y/N]: ")

			var input string
			fmt.Scanln(&input)
			if input != "y" && input != "Y" {
				fmt.Println("Aborted")
				return nil
			}

			// Load state and reset phases after this one
			s, err := state.Load(id)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}

			// Mark later phases as pending
			foundTarget := false
			for i := range p.Phases {
				if p.Phases[i].ID == toPhase {
					foundTarget = true
					p.Phases[i].Status = plan.PhasePending
					p.Phases[i].CommitSHA = ""
					continue
				}
				if foundTarget {
					p.Phases[i].Status = plan.PhasePending
					p.Phases[i].CommitSHA = ""
					if s.Phases[p.Phases[i].ID] != nil {
						s.Phases[p.Phases[i].ID].Status = state.StatusPending
					}
				}
			}

			// Save updated state
			if err := p.Save(id); err != nil {
				return fmt.Errorf("save plan: %w", err)
			}
			if err := s.Save(); err != nil {
				return fmt.Errorf("save state: %w", err)
			}

			fmt.Printf("✅ Rewound to phase: %s\n", toPhase)
			fmt.Printf("   Run: orc run %s to continue\n", id)
			return nil
		},
	}
	cmd.Flags().String("to", "", "phase to rewind to (required)")
	cmd.MarkFlagRequired("to")
	return cmd
}

// newStatusCmd creates the status command
func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show orc status",
		Long: `Show current orc status including:
  • Active tasks and their phases
  • Pending approvals
  • Recent completions`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			tasks, err := task.LoadAll()
			if err != nil {
				return fmt.Errorf("load tasks: %w", err)
			}

			// Count by status
			var running, paused, blocked, completed int
			for _, t := range tasks {
				switch t.Status {
				case task.StatusRunning:
					running++
				case task.StatusPaused:
					paused++
				case task.StatusBlocked:
					blocked++
				case task.StatusCompleted:
					completed++
				}
			}

			fmt.Println("orc status")
			fmt.Println("──────────")
			fmt.Printf("Running:   %d\n", running)
			fmt.Printf("Paused:    %d\n", paused)
			fmt.Printf("Blocked:   %d\n", blocked)
			fmt.Printf("Completed: %d\n", completed)
			fmt.Printf("Total:     %d\n", len(tasks))

			// Show running/blocked tasks
			if running > 0 || blocked > 0 {
				fmt.Println("\nActive tasks:")
				for _, t := range tasks {
					if t.Status == task.StatusRunning || t.Status == task.StatusBlocked {
						fmt.Printf("  %s - %s [%s] %s\n", statusIcon(t.Status), t.ID, t.CurrentPhase, t.Title)
					}
				}
			}

			return nil
		},
	}
}

// newLogCmd creates the log command
func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log <task-id>",
		Short: "Show task transcripts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			transcriptsDir := fmt.Sprintf(".orc/tasks/%s/transcripts", id)
			entries, err := os.ReadDir(transcriptsDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Println("No transcripts found for this task")
					return nil
				}
				return fmt.Errorf("read transcripts: %w", err)
			}

			if len(entries) == 0 {
				fmt.Println("No transcripts found for this task")
				return nil
			}

			fmt.Printf("Transcripts for %s:\n", id)
			for _, entry := range entries {
				fmt.Printf("  %s/%s\n", transcriptsDir, entry.Name())
			}

			return nil
		},
	}
}

// newDiffCmd creates the diff command
func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <task-id>",
		Short: "Show task changes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Showing diff for task: %s\n", args[0])
			// TODO: Implement git diff
			return nil
		},
	}
}

// newApproveCmd creates the approve command
func newApproveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "approve <task-id>",
		Short: "Approve a gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			if t.Status != task.StatusBlocked {
				return fmt.Errorf("task is not blocked (status: %s)", t.Status)
			}

			t.Status = task.StatusPlanned
			if err := t.Save(); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("✅ Task %s approved\n", id)
			fmt.Printf("   Run: orc run %s to continue\n", id)
			return nil
		},
	}
}

// newRejectCmd creates the reject command
func newRejectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject <task-id>",
		Short: "Reject a gate",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]
			reason, _ := cmd.Flags().GetString("reason")

			t, err := task.Load(id)
			if err != nil {
				return fmt.Errorf("load task: %w", err)
			}

			s, err := state.Load(id)
			if err != nil {
				return fmt.Errorf("load state: %w", err)
			}

			if reason == "" {
				reason = "rejected by user"
			}

			s.RecordGateDecision(s.CurrentPhase, "human", false, reason)
			if err := s.Save(); err != nil {
				return fmt.Errorf("save state: %w", err)
			}

			t.Status = task.StatusFailed
			if err := t.Save(); err != nil {
				return fmt.Errorf("save task: %w", err)
			}

			fmt.Printf("❌ Task %s rejected: %s\n", id, reason)
			return nil
		},
	}
	cmd.Flags().String("reason", "", "rejection reason")
	return cmd
}

// newExportCmd creates the export command
func newExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export <task-id>",
		Short: "Export task context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Exporting task: %s\n", args[0])
			// TODO: Implement export
			return nil
		},
	}
}

// newImportCmd creates the import command
func newImportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import <file>",
		Short: "Import context into task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("Importing from: %s\n", args[0])
			// TODO: Implement import
			return nil
		},
	}
}

// newConfigCmd creates the config command
func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config [key] [value]",
		Short: "Get or set configuration",
		Long: `Get or set orc configuration.

Automation profiles:
  auto   - Fully automated, no human intervention (default)
  fast   - Minimal gates, speed over safety
  safe   - AI reviews, human approval only for merge
  strict - Human gates on spec/review/merge

Example:
  orc config                    # show all
  orc config profile            # show profile
  orc config profile safe       # set profile`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			if len(args) == 0 {
				// Show all config
				fmt.Println("Current configuration:")
				fmt.Println()
				fmt.Println("Automation:")
				fmt.Printf("  profile:         %s\n", cfg.Profile)
				fmt.Printf("  gates.default:   %s\n", cfg.Gates.DefaultType)
				fmt.Printf("  retry.enabled:   %v\n", cfg.Retry.Enabled)
				fmt.Printf("  retry.max:       %d\n", cfg.Retry.MaxRetries)
				fmt.Println()
				fmt.Println("Execution:")
				fmt.Printf("  model:           %s\n", cfg.Model)
				fmt.Printf("  max_iterations:  %d\n", cfg.MaxIterations)
				fmt.Printf("  timeout:         %s\n", cfg.Timeout)
				fmt.Println()
				fmt.Println("Git:")
				fmt.Printf("  branch_prefix:   %s\n", cfg.BranchPrefix)
				fmt.Printf("  commit_prefix:   %s\n", cfg.CommitPrefix)
				return nil
			}

			// Set config value
			if len(args) == 2 {
				key, value := args[0], args[1]
				switch key {
				case "profile":
					cfg.ApplyProfile(config.AutomationProfile(value))
					if err := cfg.Save(); err != nil {
						return fmt.Errorf("save config: %w", err)
					}
					fmt.Printf("Set profile to: %s\n", value)
				default:
					return fmt.Errorf("unknown config key: %s", key)
				}
				return nil
			}

			// Show specific key
			key := args[0]
			switch key {
			case "profile":
				fmt.Println(cfg.Profile)
			case "gates":
				fmt.Printf("default: %s\n", cfg.Gates.DefaultType)
				fmt.Printf("auto_approve: %v\n", cfg.Gates.AutoApproveOnSuccess)
				if len(cfg.Gates.PhaseOverrides) > 0 {
					fmt.Println("phase_overrides:")
					for k, v := range cfg.Gates.PhaseOverrides {
						fmt.Printf("  %s: %s\n", k, v)
					}
				}
			case "retry":
				fmt.Printf("enabled: %v\n", cfg.Retry.Enabled)
				fmt.Printf("max_retries: %d\n", cfg.Retry.MaxRetries)
				if len(cfg.Retry.RetryMap) > 0 {
					fmt.Println("retry_map:")
					for k, v := range cfg.Retry.RetryMap {
						fmt.Printf("  %s -> %s\n", k, v)
					}
				}
			default:
				return fmt.Errorf("unknown config key: %s", key)
			}
			return nil
		},
	}
}

// newVersionCmd creates the version command
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show orc version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("orc version 0.1.0-dev")
		},
	}
}

// Helper functions

func statusIcon(status task.Status) string {
	switch status {
	case task.StatusCreated:
		return "📝"
	case task.StatusClassifying:
		return "🔍"
	case task.StatusPlanned:
		return "📋"
	case task.StatusRunning:
		return "⏳"
	case task.StatusPaused:
		return "⏸️"
	case task.StatusBlocked:
		return "🚫"
	case task.StatusCompleted:
		return "✅"
	case task.StatusFailed:
		return "❌"
	default:
		return "❓"
	}
}

func phaseStatusIcon(status plan.PhaseStatus) string {
	switch status {
	case plan.PhasePending:
		return "○"
	case plan.PhaseRunning:
		return "◐"
	case plan.PhaseCompleted:
		return "●"
	case plan.PhaseFailed:
		return "✗"
	case plan.PhaseSkipped:
		return "⊘"
	default:
		return "?"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
