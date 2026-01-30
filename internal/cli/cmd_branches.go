// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/storage"
)

func newBranchesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branches",
		Short: "Manage orc-tracked branches",
		Long: `Manage branches tracked by orc (initiative, staging, and task branches).

Orc tracks branches it creates to help with lifecycle management:
  - Initiative branches (e.g., feature/auth) for initiative-based work
  - Staging branches (e.g., dev/randy) for personal staging areas
  - Task branches (e.g., orc/TASK-001) for individual tasks

Commands:
  list      List tracked branches
  cleanup   Delete merged or orphaned branches
  prune     Remove stale tracking entries`,
	}

	cmd.AddCommand(newBranchesListCmd())
	cmd.AddCommand(newBranchesCleanupCmd())
	cmd.AddCommand(newBranchesPruneCmd())

	return cmd
}

func newBranchesListCmd() *cobra.Command {
	var (
		branchType   string
		branchStatus string
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tracked branches",
		Long: `List all branches tracked by orc.

Use filters to narrow results:
  --type initiative|staging|task
  --status active|merged|stale|orphaned

Example:
  orc branches list
  orc branches list --type initiative
  orc branches list --status merged`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			opts := storage.BranchListOpts{
				Type:   storage.BranchType(branchType),
				Status: storage.BranchStatus(branchStatus),
			}

			branches, err := backend.ListBranches(opts)
			if err != nil {
				return fmt.Errorf("list branches: %w", err)
			}

			if len(branches) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No tracked branches found.")
				return nil
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "BRANCH\tTYPE\tOWNER\tSTATUS\tLAST ACTIVITY")
			_, _ = fmt.Fprintln(w, "------\t----\t-----\t------\t-------------")

			for _, b := range branches {
				lastActivity := formatTimeAgo(b.LastActivity)
				owner := b.OwnerID
				if owner == "" {
					owner = "-"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					b.Name, b.Type, owner, b.Status, lastActivity)
			}
			_ = w.Flush()

			return nil
		},
	}

	cmd.Flags().StringVarP(&branchType, "type", "t", "", "Filter by type (initiative|staging|task)")
	cmd.Flags().StringVarP(&branchStatus, "status", "s", "", "Filter by status (active|merged|stale|orphaned)")

	return cmd
}

func newBranchesCleanupCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Delete merged or orphaned branches",
		Long: `Delete branches that have been merged or are orphaned.

This command:
1. Lists branches with status 'merged' or 'orphaned'
2. Deletes them from git (if they exist)
3. Removes them from the tracking registry

Use --force to skip confirmation.

Example:
  orc branches cleanup
  orc branches cleanup --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			projectRoot, err := ResolveProjectPath()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			gitOps, err := git.New(projectRoot, git.DefaultConfig())
			if err != nil {
				return fmt.Errorf("init git: %w", err)
			}

			// Get merged branches
			merged, err := backend.ListBranches(storage.BranchListOpts{Status: storage.BranchStatusMerged})
			if err != nil {
				return fmt.Errorf("list merged branches: %w", err)
			}

			// Get orphaned branches
			orphaned, err := backend.ListBranches(storage.BranchListOpts{Status: storage.BranchStatusOrphaned})
			if err != nil {
				return fmt.Errorf("list orphaned branches: %w", err)
			}

			toClean := append(merged, orphaned...)
			if len(toClean) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No branches to clean up.")
				return nil
			}

			// Show what will be deleted
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Branches to clean up:")
			for _, b := range toClean {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s (%s)\n", b.Name, b.Status)
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout())

			// Confirm unless --force
			if !force && !quiet {
				_, _ = fmt.Fprint(cmd.OutOrStdout(), "Delete these branches? [y/N]: ")
				var response string
				_, _ = fmt.Scanln(&response)
				if !strings.HasPrefix(strings.ToLower(response), "y") {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Cancelled.")
					return nil
				}
			}

			// Delete branches
			deleted := 0
			for _, b := range toClean {
				// Try to delete from git
				exists, _ := gitOps.BranchExists(b.Name)
				if exists {
					if err := gitOps.DeleteBranch(b.Name, false); err != nil {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to delete git branch %s: %v\n", b.Name, err)
						// Continue to remove from registry anyway
					} else {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Deleted git branch: %s\n", b.Name)
					}
				}

				// Remove from registry
				if err := backend.DeleteBranch(b.Name); err != nil {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to remove %s from registry: %v\n", b.Name, err)
				}
				deleted++
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nCleaned up %d branch(es).\n", deleted)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation")

	return cmd
}

func newBranchesPruneCmd() *cobra.Command {
	var staleAfterDays int

	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Mark stale branches and remove invalid entries",
		Long: `Mark branches as stale if inactive, and remove invalid tracking entries.

This command:
1. Marks branches as 'stale' if inactive for more than --stale-after days
2. Removes tracking entries for branches that no longer exist in git

Use 'orc branches cleanup' to actually delete stale branches.

Example:
  orc branches prune
  orc branches prune --stale-after 30`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("get backend: %w", err)
			}
			defer func() { _ = backend.Close() }()

			projectRoot, err := ResolveProjectPath()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			gitOps, err := git.New(projectRoot, git.DefaultConfig())
			if err != nil {
				return fmt.Errorf("init git: %w", err)
			}

			// Get all active branches
			branches, err := backend.ListBranches(storage.BranchListOpts{Status: storage.BranchStatusActive})
			if err != nil {
				return fmt.Errorf("list branches: %w", err)
			}

			staleThreshold := time.Now().AddDate(0, 0, -staleAfterDays)
			staleCount := 0
			removedCount := 0

			for _, b := range branches {
				// Check if branch still exists in git
				exists, _ := gitOps.BranchExists(b.Name)
				if !exists {
					// Remove from registry
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Removing invalid entry: %s (branch no longer exists)\n", b.Name)
					if err := backend.DeleteBranch(b.Name); err != nil {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to remove %s: %v\n", b.Name, err)
					}
					removedCount++
					continue
				}

				// Check for staleness
				if b.LastActivity.Before(staleThreshold) {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Marking stale: %s (inactive since %s)\n",
						b.Name, b.LastActivity.Format("2006-01-02"))
					if err := backend.UpdateBranchStatus(b.Name, storage.BranchStatusStale); err != nil {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to mark %s as stale: %v\n", b.Name, err)
					}
					staleCount++
				}
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "\nPruned: %d stale, %d removed.\n", staleCount, removedCount)
			return nil
		},
	}

	cmd.Flags().IntVar(&staleAfterDays, "stale-after", 14, "Days of inactivity before marking stale")

	return cmd
}

// formatTimeAgo is defined in cmd_status.go
