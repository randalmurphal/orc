package cli

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bench"
)

func newBenchCurateExtractPatchesCmd() *cobra.Command {
	var (
		force      bool
		taskIDs    []string
		dryRun     bool
		suitePath  string
		patchesDir string
	)

	cmd := &cobra.Command{
		Use:   "extract-patches",
		Short: "Extract test-only patches from GitHub PRs",
		Long: `Extract test-only patches from GitHub PRs for benchmark tasks.

For each task with a reference_pr_url, fetches the full PR diff using
'gh pr diff', splits it into test-only and source-only portions using
language-aware file patterns, and saves the test patch for evaluation.

Test patches are applied AFTER the model finishes working. They add the
PR's test changes so we can evaluate whether the model's source fix
actually passes the real tests. The model never sees the test patch.

Requires the GitHub CLI (gh) to be installed and authenticated.

Examples:
  orc bench curate extract-patches                    # All tasks
  orc bench curate extract-patches --task bbolt-002   # Single task
  orc bench curate extract-patches --dry-run          # Preview only
  orc bench curate extract-patches --force            # Re-extract all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()

			// Load all projects for language lookup
			projects, err := store.ListProjects(ctx)
			if err != nil {
				return fmt.Errorf("list projects: %w", err)
			}
			projectMap := make(map[string]*bench.Project, len(projects))
			for _, p := range projects {
				projectMap[p.ID] = p
			}

			// Resolve patches dir
			dir := patchesDir
			if dir == "" {
				dir, err = bench.DefaultPatchesDir()
				if err != nil {
					return err
				}
			}

			// Resolve suite path
			suite := suitePath
			if suite == "" {
				suite, err = bench.DefaultSuiteConfigPath()
				if err != nil {
					return err
				}
			}

			opts := bench.ExtractOptions{
				Force:      force,
				TaskIDs:    taskIDs,
				PatchesDir: dir,
				DryRun:     dryRun,
				SuitePath:  suite,
			}

			if dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), "Dry run — no files will be written.")
				fmt.Fprintln(cmd.OutOrStdout())
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Extracting test patches from GitHub PRs...")
			fmt.Fprintln(cmd.OutOrStdout())

			results, err := bench.ExtractPatches(ctx, store, projectMap, opts)
			if err != nil {
				return err
			}

			// Print results table
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, r := range results {
				status := string(r.Status)
				detail := ""

				switch r.Status {
				case bench.StatusExtracted:
					if len(r.TestFiles) == 1 {
						detail = r.TestFiles[0]
					} else {
						detail = fmt.Sprintf("%d test files", len(r.TestFiles))
					}
				case bench.StatusAlreadyExists:
					detail = "(already exists)"
				case bench.StatusNoTests:
					if len(r.SourceFiles) > 0 {
						detail = fmt.Sprintf("PR changes %d source files only", len(r.SourceFiles))
					}
				case bench.StatusNoURL:
					detail = "no reference_pr_url"
				case bench.StatusFetchFailed:
					if r.Error != nil {
						detail = r.Error.Error()
					}
				}

				fmt.Fprintf(w, "  %s\t%s\t%s\n", r.TaskID, status, detail)
			}
			w.Flush()

			// Summary counts
			counts := make(map[bench.ExtractionStatus]int)
			for _, r := range results {
				counts[r.Status]++
			}

			fmt.Fprintf(cmd.OutOrStdout(), "\nResults: %d extracted, %d exist, %d no_tests, %d failed\n",
				counts[bench.StatusExtracted],
				counts[bench.StatusAlreadyExists],
				counts[bench.StatusNoTests],
				counts[bench.StatusFetchFailed]+counts[bench.StatusNoURL],
			)

			// Update suite.yaml if not dry run and we extracted anything
			if !dryRun && counts[bench.StatusExtracted] > 0 {
				if err := bench.UpdateSuiteYAML(suite, results); err != nil {
					return fmt.Errorf("update suite.yaml: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Updated %s with test_patch_file references.\n", suite)
			}

			// Warn about failures
			var failures []string
			for _, r := range results {
				if r.Status == bench.StatusFetchFailed || r.Status == bench.StatusNoTests {
					failures = append(failures, r.TaskID)
				}
			}
			if len(failures) > 0 {
				fmt.Fprintf(cmd.OutOrStderr(), "\nWarning: %d tasks need attention: %s\n",
					len(failures), strings.Join(failures, ", "))
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Re-extract patches even if files exist")
	cmd.Flags().StringSliceVar(&taskIDs, "task", nil, "Limit to specific task IDs (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be extracted without writing")
	cmd.Flags().StringVar(&suitePath, "suite", "", "Path to suite.yaml (default: ~/.orc/bench/suite.yaml)")
	cmd.Flags().StringVar(&patchesDir, "patches-dir", "", "Override patch output directory")

	return cmd
}
