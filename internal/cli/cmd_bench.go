package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bench"
)

func newBenchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bench",
		Short: "Benchmark model configurations across workflow phases",
		Long: `Benchmark system for comparing model configurations at each workflow phase.

Uses phase-isolation testing: run an all-Opus baseline, freeze outputs, then swap
one phase's model at a time. Compare results to find the optimal model per phase.

Core concepts:
  Projects    Pinned repos with known-good tests (Go, TypeScript, Python, Rust)
  Tasks       SWE-bench style issues from real PRs with fail-to-pass tests
  Variants    Model configurations targeting specific phases
  Runs        Execution records with pass/fail, cost, and timing

Workflow:
  1. orc bench curate import suite.yaml         Import projects, tasks, variants
  2. orc bench run --baseline --trials 2         Run the all-Opus baseline
  3. orc bench run --variant codex53-high-impl   Run a variant (uses frozen outputs)
  4. orc bench report                            View results and recommendations

Data lives at ~/.orc/bench/ (bench.db, repos/, runs/).

Adding a new model = editing suite.yaml. No code changes needed.`,
	}

	cmd.AddCommand(newBenchCurateCmd())
	cmd.AddCommand(newBenchRunCmd())
	cmd.AddCommand(newBenchReportCmd())
	cmd.AddCommand(newBenchJudgeCmd())
	cmd.AddCommand(newBenchShowCmd())

	return cmd
}

// --- Curate subcommands ---

func newBenchCurateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "curate",
		Short: "Manage benchmark projects, tasks, and variants",
		Long: `Curate the benchmark test suite: add projects, tasks, and variants.

Subcommands:
  add-project   Register a pinned repo for benchmarking
  add-task      Add a SWE-bench style task (real issue + known fix)
  add-variant   Define a model configuration to test
  list          List projects, tasks, or variants
  validate      Check that all tasks are healthy (repos exist, commits valid)
  import        Bulk import from a suite.yaml file`,
	}

	cmd.AddCommand(newBenchCurateAddProjectCmd())
	cmd.AddCommand(newBenchCurateAddTaskCmd())
	cmd.AddCommand(newBenchCurateAddVariantCmd())
	cmd.AddCommand(newBenchCurateListCmd())
	cmd.AddCommand(newBenchCurateValidateCmd())
	cmd.AddCommand(newBenchCurateImportCmd())
	cmd.AddCommand(newBenchCurateExtractPatchesCmd())

	return cmd
}

func newBenchCurateAddProjectCmd() *cobra.Command {
	var (
		repoURL     string
		commitHash  string
		language    string
		testCmd     string
		buildCmd    string
		lintCmd     string
		securityCmd string
	)

	cmd := &cobra.Command{
		Use:   "add-project <id>",
		Short: "Register a project for benchmarking",
		Long: `Register a pinned repository for benchmark testing.

The repo will be cloned to ~/.orc/bench/repos/<id>/ on first run.
The commit hash pins the exact version used for reproducible benchmarks.

Example:
  orc bench curate add-project bbolt \
    --repo https://github.com/etcd-io/bbolt \
    --commit abc123def \
    --language go \
    --test-cmd "go test ./..." \
    --build-cmd "go build ./..."`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			p := &bench.Project{
				ID:          args[0],
				RepoURL:     repoURL,
				CommitHash:  commitHash,
				Language:    language,
				TestCmd:     testCmd,
				BuildCmd:    buildCmd,
				LintCmd:     lintCmd,
				SecurityCmd: securityCmd,
			}
			if err := p.Validate(); err != nil {
				return err
			}

			if err := store.SaveProject(context.Background(), p); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added project %s (%s)\n", p.ID, p.Language)
			return nil
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo", "", "Repository URL (required)")
	cmd.Flags().StringVar(&commitHash, "commit", "", "Pinned commit hash (required)")
	cmd.Flags().StringVar(&language, "language", "", "Programming language (required)")
	cmd.Flags().StringVar(&testCmd, "test-cmd", "", "Test command (required)")
	cmd.Flags().StringVar(&buildCmd, "build-cmd", "", "Build command")
	cmd.Flags().StringVar(&lintCmd, "lint-cmd", "", "Lint command")
	cmd.Flags().StringVar(&securityCmd, "security-cmd", "", "Security scan command")
	_ = cmd.MarkFlagRequired("repo")
	_ = cmd.MarkFlagRequired("commit")
	_ = cmd.MarkFlagRequired("language")
	_ = cmd.MarkFlagRequired("test-cmd")

	return cmd
}

func newBenchCurateAddTaskCmd() *cobra.Command {
	var (
		projectID      string
		tier           string
		category       string
		description    string
		preFixCommit   string
		referencePRURL string
	)

	cmd := &cobra.Command{
		Use:   "add-task <id> <title>",
		Short: "Add a benchmark task",
		Long: `Add a benchmark task from a real PR.

Tasks represent real issues with known fixes. The model is given the issue
description, checked out at the pre-fix commit. After the model finishes,
the test patch from the reference PR is applied and tests are run.

Example:
  orc bench curate add-task bbolt-001 "Fix read-only file creation" \
    --project bbolt \
    --tier trivial \
    --category bug \
    --pre-fix-commit abc123 \
    --description "bolt.Open in read-only mode creates the file..."`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			t := &bench.Task{
				ID:             args[0],
				ProjectID:      projectID,
				Tier:           bench.Tier(tier),
				Category:       category,
				Title:          args[1],
				Description:    description,
				PreFixCommit:   preFixCommit,
				ReferencePRURL: referencePRURL,
			}
			if err := t.Validate(); err != nil {
				return err
			}

			if err := store.SaveTask(context.Background(), t); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Added task %s (%s/%s)\n", t.ID, t.ProjectID, t.Tier)
			return nil
		},
	}

	cmd.Flags().StringVar(&projectID, "project", "", "Project ID (required)")
	cmd.Flags().StringVar(&tier, "tier", "", "Complexity tier: trivial, small, medium, large (required)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Issue description (required)")
	cmd.Flags().StringVar(&preFixCommit, "pre-fix-commit", "", "Commit before the fix (required)")
	cmd.Flags().StringVar(&referencePRURL, "reference-pr", "", "Reference PR URL")
	cmd.Flags().StringVar(&category, "category", "", "Task category: bug, feature, refactor, etc.")
	_ = cmd.MarkFlagRequired("project")
	_ = cmd.MarkFlagRequired("tier")
	_ = cmd.MarkFlagRequired("description")
	_ = cmd.MarkFlagRequired("pre-fix-commit")

	return cmd
}

func newBenchCurateAddVariantCmd() *cobra.Command {
	var (
		name         string
		baseWorkflow string
		isBaseline   bool
		overridesRaw string
	)

	cmd := &cobra.Command{
		Use:   "add-variant <id>",
		Short: "Define a model configuration variant",
		Long: `Define a variant for benchmark testing.

Variants specify which model to use for specific workflow phases. Phases without
overrides use frozen outputs from the baseline run.

The --overrides flag takes JSON mapping phase IDs to model configs:
  {"implement": {"provider":"codex","model":"gpt-5.3-codex","reasoning_effort":"high"}}

Example:
  orc bench curate add-variant codex53-high-impl \
    --name "Codex 5.3 High Implement" \
    --workflow medium \
    --overrides '{"implement":{"provider":"codex","model":"gpt-5.3-codex","reasoning_effort":"high"}}'

  orc bench curate add-variant baseline-opus \
    --name "All Opus Baseline" \
    --workflow medium \
    --baseline`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			var overrides map[string]bench.PhaseOverride
			if overridesRaw != "" {
				overrides, err = bench.ParseOverrides(overridesRaw)
				if err != nil {
					return fmt.Errorf("parse overrides: %w", err)
				}
			}

			v := &bench.Variant{
				ID:             args[0],
				Name:           name,
				BaseWorkflow:   baseWorkflow,
				PhaseOverrides: overrides,
				IsBaseline:     isBaseline,
			}
			if err := v.Validate(); err != nil {
				return err
			}

			if err := store.SaveVariant(context.Background(), v); err != nil {
				return err
			}

			label := v.ID
			if v.IsBaseline {
				label += " (baseline)"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added variant %s\n", label)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Display name (required)")
	cmd.Flags().StringVar(&baseWorkflow, "workflow", "", "Base workflow: trivial, small, medium, large (required)")
	cmd.Flags().BoolVar(&isBaseline, "baseline", false, "Mark as the baseline variant")
	cmd.Flags().StringVar(&overridesRaw, "overrides", "", "Phase overrides as JSON")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("workflow")

	return cmd
}

func newBenchCurateListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [projects|tasks|variants]",
		Short: "List benchmark entities",
		Long: `List projects, tasks, or variants in the benchmark suite.

Examples:
  orc bench curate list projects
  orc bench curate list tasks
  orc bench curate list variants`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"projects", "tasks", "variants", "runs"},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()

			switch args[0] {
			case "projects":
				return listBenchProjects(cmd, store, ctx)
			case "tasks":
				return listBenchTasks(cmd, store, ctx)
			case "variants":
				return listBenchVariants(cmd, store, ctx)
			case "runs":
				return listBenchRuns(cmd, store, ctx)
			default:
				return fmt.Errorf("unknown entity type: %s (use projects, tasks, variants, or runs)", args[0])
			}
		},
	}
	return cmd
}

func listBenchProjects(cmd *cobra.Command, store *bench.Store, ctx context.Context) error {
	projects, err := store.ListProjects(ctx)
	if err != nil {
		return err
	}

	if jsonOut {
		return outputJSON(cmd, projects)
	}

	if len(projects) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No projects registered. Use 'orc bench curate add-project' or 'import'.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tLANGUAGE\tTEST CMD\tREPO")
	for _, p := range projects {
		repo := p.RepoURL
		if len(repo) > 50 {
			repo = repo[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.ID, p.Language, p.TestCmd, repo)
	}
	return w.Flush()
}

func listBenchTasks(cmd *cobra.Command, store *bench.Store, ctx context.Context) error {
	tasks, err := store.ListTasks(ctx, "", "")
	if err != nil {
		return err
	}

	if jsonOut {
		return outputJSON(cmd, tasks)
	}

	if len(tasks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No tasks registered. Use 'orc bench curate add-task' or 'import'.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPROJECT\tTIER\tCATEGORY\tTITLE\tTEST_PATCH")
	for _, t := range tasks {
		title := t.Title
		if len(title) > 40 {
			title = title[:37] + "..."
		}
		hasPatch := "no"
		if t.TestPatch != "" {
			hasPatch = "yes"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", t.ID, t.ProjectID, t.Tier, t.Category, title, hasPatch)
	}
	return w.Flush()
}

func listBenchVariants(cmd *cobra.Command, store *bench.Store, ctx context.Context) error {
	variants, err := store.ListVariants(ctx)
	if err != nil {
		return err
	}

	if jsonOut {
		return outputJSON(cmd, variants)
	}

	if len(variants) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No variants defined. Use 'orc bench curate add-variant' or 'import'.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tWORKFLOW\tBASELINE\tOVERRIDES")
	for _, v := range variants {
		baseline := ""
		if v.IsBaseline {
			baseline = "*"
		}
		overrideCount := len(v.PhaseOverrides)
		var overrideSummary string
		if overrideCount == 0 {
			overrideSummary = "(none)"
		} else {
			overrideSummary = fmt.Sprintf("%d phases", overrideCount)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", v.ID, v.Name, v.BaseWorkflow, baseline, overrideSummary)
	}
	return w.Flush()
}

func listBenchRuns(cmd *cobra.Command, store *bench.Store, ctx context.Context) error {
	runs, err := store.ListRuns(ctx, "", "", "")
	if err != nil {
		return err
	}

	if jsonOut {
		return outputJSON(cmd, runs)
	}

	if len(runs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No runs yet. Use 'orc bench run --baseline' to start.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tVARIANT\tTASK\tTRIAL\tSTATUS\tTEST\tBUILD\tDURATION\tDIFF")
	for _, r := range runs {
		shortID := r.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		test := "fail"
		if r.TestPass {
			test = "pass"
		}
		build := "fail"
		if r.BuildSuccess {
			build = "pass"
		}
		hasDiff := "no"
		if r.ModelDiff != "" {
			hasDiff = "yes"
		}
		dur := ""
		if !r.StartedAt.IsZero() && !r.CompletedAt.IsZero() {
			dur = r.CompletedAt.Sub(r.StartedAt).Round(time.Second).String()
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			shortID, r.VariantID, r.TaskID, r.TrialNumber, r.Status, test, build, dur, hasDiff)
	}
	return w.Flush()
}

func newBenchShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <run-id>",
		Short: "Show run details including model diff",
		Long: `Display details of a benchmark run including what the model changed.

The run ID can be a prefix (first 8+ characters).

Examples:
  orc bench show 06888da7
  orc bench show 06888da7 --diff`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()

			// Support prefix matching
			runID := args[0]
			run, err := findRunByPrefix(ctx, store, runID)
			if err != nil {
				return err
			}

			showDiff, _ := cmd.Flags().GetBool("diff")

			if jsonOut {
				return outputJSON(cmd, run)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Run:      %s\n", run.ID)
			fmt.Fprintf(out, "Variant:  %s\n", run.VariantID)
			fmt.Fprintf(out, "Task:     %s\n", run.TaskID)
			fmt.Fprintf(out, "Trial:    %d\n", run.TrialNumber)
			fmt.Fprintf(out, "Status:   %s\n", run.Status)
			fmt.Fprintf(out, "Test:     %v\n", run.TestPass)
			fmt.Fprintf(out, "Build:    %v\n", run.BuildSuccess)
			if !run.StartedAt.IsZero() && !run.CompletedAt.IsZero() {
				fmt.Fprintf(out, "Duration: %s\n", run.CompletedAt.Sub(run.StartedAt).Round(time.Second))
			}
			if run.ErrorMessage != "" {
				fmt.Fprintf(out, "Error:    %s\n", run.ErrorMessage)
			}

			// Show phase results (cost, tokens, duration)
			phases, phaseErr := store.GetPhaseResults(ctx, run.ID)
			if phaseErr == nil && len(phases) > 0 {
				fmt.Fprintf(out, "\n--- Phase Results ---\n")
				pw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
				fmt.Fprintln(pw, "PHASE\tMODEL\tCOST\tIN_TOK\tOUT_TOK\tDURATION\tFROZEN")
				var totalCost float64
				var totalIn, totalOut int
				for _, p := range phases {
					frozen := ""
					if p.WasFrozen {
						frozen = "yes"
					}
					dur := time.Duration(p.DurationMs) * time.Millisecond
					fmt.Fprintf(pw, "%s\t%s\t$%.4f\t%d\t%d\t%s\t%s\n",
						p.PhaseID, p.Model, p.CostUSD, p.InputTokens, p.OutputTokens, dur.Round(time.Second), frozen)
					totalCost += p.CostUSD
					totalIn += p.InputTokens
					totalOut += p.OutputTokens
				}
				fmt.Fprintf(pw, "TOTAL\t\t$%.4f\t%d\t%d\t\t\n", totalCost, totalIn, totalOut)
				_ = pw.Flush()
			}

			if showDiff {
				fmt.Fprintf(out, "\n--- Model Diff ---\n")
				if run.ModelDiff == "" {
					fmt.Fprintln(out, "(no diff captured)")
				} else {
					fmt.Fprintln(out, run.ModelDiff)
				}
			}

			showTests, _ := cmd.Flags().GetBool("test-output")
			if showTests && run.TestOutput != "" {
				fmt.Fprintf(out, "\n--- Test Output ---\n")
				fmt.Fprintln(out, run.TestOutput)
			}
			if showTests && run.BuildOutput != "" {
				fmt.Fprintf(out, "\n--- Build Output ---\n")
				fmt.Fprintln(out, run.BuildOutput)
			}

			return nil
		},
	}
	cmd.Flags().Bool("diff", true, "Show the model's code diff")
	cmd.Flags().Bool("test-output", false, "Show test and build output")
	return cmd
}

func findRunByPrefix(ctx context.Context, store *bench.Store, prefix string) (*bench.Run, error) {
	// Try exact match first
	run, err := store.GetRun(ctx, prefix)
	if err == nil {
		return run, nil
	}

	// Try prefix match
	runs, err := store.ListRuns(ctx, "", "", "")
	if err != nil {
		return nil, err
	}
	var matchIDs []string
	for _, r := range runs {
		if len(r.ID) >= len(prefix) && r.ID[:len(prefix)] == prefix {
			matchIDs = append(matchIDs, r.ID)
		}
	}
	if len(matchIDs) == 0 {
		return nil, fmt.Errorf("no run found matching %q", prefix)
	}
	if len(matchIDs) > 1 {
		return nil, fmt.Errorf("ambiguous prefix %q matches %d runs", prefix, len(matchIDs))
	}
	// Full fetch to get all fields (ListRuns omits large text fields)
	return store.GetRun(ctx, matchIDs[0])
}

func newBenchCurateValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate benchmark suite health",
		Long: `Check that all projects, tasks, and variants are properly configured.

Validates:
  - All tasks reference existing projects
  - Exactly one baseline variant exists
  - No orphaned references`,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			ctx := context.Background()

			projects, err := store.ListProjects(ctx)
			if err != nil {
				return err
			}
			tasks, err := store.ListTasks(ctx, "", "")
			if err != nil {
				return err
			}
			variants, err := store.ListVariants(ctx)
			if err != nil {
				return err
			}

			projectIDs := make(map[string]bool)
			for _, p := range projects {
				projectIDs[p.ID] = true
			}

			var issues []string

			// Check task→project references
			for _, t := range tasks {
				if !projectIDs[t.ProjectID] {
					issues = append(issues, fmt.Sprintf("task %s references unknown project %s", t.ID, t.ProjectID))
				}
			}

			// Check baseline
			baselineCount := 0
			for _, v := range variants {
				if v.IsBaseline {
					baselineCount++
				}
			}
			if baselineCount == 0 {
				issues = append(issues, "no baseline variant defined")
			} else if baselineCount > 1 {
				issues = append(issues, fmt.Sprintf("multiple baseline variants (%d)", baselineCount))
			}

			if len(issues) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Validation issues:")
				for _, issue := range issues {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", issue)
				}
				return fmt.Errorf("found %d validation issue(s)", len(issues))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Suite is valid: %d projects, %d tasks, %d variants\n",
				len(projects), len(tasks), len(variants))
			return nil
		},
	}
}

func newBenchCurateImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import <suite.yaml>",
		Short: "Import benchmark suite from YAML",
		Long: `Bulk import projects, tasks, and variants from a suite.yaml file.

Existing entries are updated (upsert). This is the recommended way to manage
the benchmark suite configuration.

Example:
  orc bench curate import ~/.orc/bench/suite.yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := bench.LoadSuiteConfig(args[0])
			if err != nil {
				return err
			}

			store, err := openBenchStore()
			if err != nil {
				return err
			}
			defer store.Close()

			suiteDir := filepath.Dir(args[0])
			if err := cfg.ImportToStore(context.Background(), store, suiteDir); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Imported %d projects, %d tasks, %d variants\n",
				len(cfg.Projects), len(cfg.Tasks), len(cfg.Variants))
			return nil
		},
	}
	return cmd
}

// --- Helpers ---

// openBenchStore opens the bench database at the default path.
func openBenchStore() (*bench.Store, error) {
	dbPath, err := bench.DefaultDBPath()
	if err != nil {
		return nil, fmt.Errorf("resolve bench db path: %w", err)
	}
	store, err := bench.OpenStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open bench db: %w", err)
	}
	return store, nil
}

// outputJSON encodes v as indented JSON to the command's output.
func outputJSON(cmd *cobra.Command, v any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
