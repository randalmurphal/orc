// Package cli implements the orc command-line interface.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/events"
	"github.com/randalmurphal/orc/internal/executor"
	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/progress"
	"github.com/randalmurphal/orc/internal/state"
	"github.com/randalmurphal/orc/internal/task"
)

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

Artifact detection:
  When artifacts from previous runs exist (e.g., spec.md), orc will prompt
  whether to skip that phase. Use --auto-skip to skip automatically.

Example:
  orc run TASK-001
  orc run TASK-001 --profile safe
  orc run TASK-001 --auto-skip         # skip phases with existing artifacts
  orc run TASK-001 --phase implement   # run specific phase`,
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
				// Provide helpful error message based on status
				switch t.Status {
				case task.StatusPaused:
					fmt.Printf("Task %s is paused.\n\n", id)
					fmt.Printf("To resume:  orc resume %s\n", id)
					fmt.Printf("To restart: orc rewind %s --to <phase>\n", id)
					return nil
				case task.StatusBlocked:
					fmt.Printf("Task %s is blocked and needs user input.\n\n", id)
					fmt.Println("Check the task for pending questions or approvals.")
					fmt.Printf("To view:    orc show %s\n", id)
					return nil
				case task.StatusCompleted:
					fmt.Printf("Task %s is already completed.\n\n", id)
					fmt.Printf("To rerun:   orc rewind %s --to <phase>\n", id)
					fmt.Printf("To view:    orc show %s\n", id)
					return nil
				case task.StatusFailed:
					fmt.Printf("Task %s has failed.\n\n", id)
					fmt.Printf("To retry:   orc rewind %s --to <phase>\n", id)
					fmt.Printf("To view:    orc log %s\n", id)
					return nil
				default:
					return fmt.Errorf("task cannot be run (status: %s)", t.Status)
				}
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

			// Handle artifact detection and skip prompting
			autoSkip, _ := cmd.Flags().GetBool("auto-skip")
			if cfg.ArtifactSkip.Enabled {
				// Override config with flag if specified
				if autoSkip {
					cfg.ArtifactSkip.AutoSkip = true
				}

				// Detect artifacts and prompt/skip as appropriate
				taskDir := task.TaskDir(id)
				detector := executor.NewArtifactDetectorWithDir(taskDir, id, t.Weight)

				// Get phase IDs from plan
				phaseIDs := make([]string, len(p.Phases))
				for i, phase := range p.Phases {
					phaseIDs[i] = phase.ID
				}

				// Check each configured phase for artifacts
				for _, phaseID := range cfg.ArtifactSkip.Phases {
					// Skip if phase not in plan or already completed
					if !containsPhase(phaseIDs, phaseID) || s.IsPhaseCompleted(phaseID) {
						continue
					}

					status := detector.DetectPhaseArtifacts(phaseID)
					if status.HasArtifacts && status.CanAutoSkip {
						shouldSkip := cfg.ArtifactSkip.AutoSkip

						if !shouldSkip && !quiet {
							// Prompt user
							fmt.Printf("\n📄 %s already exists. Skip %s phase? [Y/n]: ",
								strings.Join(status.Artifacts, ", "), phaseID)
							reader := bufio.NewReader(os.Stdin)
							input, err := reader.ReadString('\n')
							if err != nil {
								// On EOF or error, don't skip - safer to run the phase
								shouldSkip = false
							} else {
								input = strings.TrimSpace(strings.ToLower(input))
								shouldSkip = input == "" || input == "y" || input == "yes"
							}
						}

						if shouldSkip {
							reason := fmt.Sprintf("artifact exists: %s", status.Description)
							s.SkipPhase(phaseID, reason)

							// Also update plan status
							if phase := p.GetPhase(phaseID); phase != nil {
								phase.Status = plan.PhaseSkipped
							}

							if !quiet {
								fmt.Printf("⊘ Skipping %s phase: %s\n", phaseID, reason)
							}
						}
					}
				}

				// Save state and plan if any phases were skipped
				if err := s.Save(); err != nil {
					return fmt.Errorf("save state after artifact skip: %w", err)
				}
				if err := p.Save(id); err != nil {
					return fmt.Errorf("save plan after artifact skip: %w", err)
				}
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

			// Set up streaming publisher if verbose or --stream flag is set
			stream, _ := cmd.Flags().GetBool("stream")
			if verbose || stream {
				publisher := events.NewCLIPublisher(os.Stdout, events.WithStreamMode(true))
				exec.SetPublisher(publisher)
				defer publisher.Close()
			}

			// Execute task
			err = exec.ExecuteTask(ctx, t, p, s)
			if err != nil {
				if ctx.Err() != nil {
					// Update task and state status for clean interrupt
					s.InterruptPhase(s.CurrentPhase)
					s.Save()
					t.Status = task.StatusBlocked
					t.Save()
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
	cmd.Flags().Bool("stream", false, "stream Claude transcript to stdout")
	cmd.Flags().Bool("auto-skip", false, "automatically skip phases with existing artifacts")
	return cmd
}

// containsPhase checks if a phase ID is in the list.
func containsPhase(phases []string, phaseID string) bool {
	for _, p := range phases {
		if p == phaseID {
			return true
		}
	}
	return false
}
