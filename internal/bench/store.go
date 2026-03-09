package bench

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/db/driver"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// embedFSAdapter wraps embed.FS to implement driver.SchemaFS.
type embedFSAdapter struct {
	fs embed.FS
}

func (e *embedFSAdapter) ReadDir(name string) ([]driver.DirEntry, error) {
	entries, err := e.fs.ReadDir(name)
	if err != nil {
		return nil, err
	}
	result := make([]driver.DirEntry, len(entries))
	for i, entry := range entries {
		result[i] = dirEntryAdapter{entry}
	}
	return result, nil
}

func (e *embedFSAdapter) ReadFile(name string) ([]byte, error) {
	return e.fs.ReadFile(name)
}

type dirEntryAdapter struct {
	fs.DirEntry
}

func (d dirEntryAdapter) Name() string { return d.DirEntry.Name() }
func (d dirEntryAdapter) IsDir() bool  { return d.DirEntry.IsDir() }

// Store provides CRUD operations for benchmark data.
// All data lives in a dedicated SQLite database at ~/.orc/bench/bench.db.
type Store struct {
	drv  driver.Driver
	path string
}

// DefaultDBPath returns the default bench database path.
func DefaultDBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".orc", "bench", "bench.db"), nil
}

// OpenStore opens (or creates) the bench database at the given path.
func OpenStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create bench db directory: %w", err)
	}

	drv := driver.NewSQLite()
	if err := drv.Open(path); err != nil {
		return nil, fmt.Errorf("open bench db: %w", err)
	}

	adapter := &embedFSAdapter{fs: schemaFS}
	if err := drv.Migrate(context.Background(), adapter, "bench"); err != nil {
		_ = drv.Close()
		return nil, fmt.Errorf("migrate bench db: %w", err)
	}

	return &Store{drv: drv, path: path}, nil
}

// OpenInMemory opens an in-memory bench database. Useful for testing.
func OpenInMemory() (*Store, error) {
	drv := driver.NewSQLite()
	if err := drv.Open(":memory:"); err != nil {
		return nil, fmt.Errorf("open in-memory bench db: %w", err)
	}

	adapter := &embedFSAdapter{fs: schemaFS}
	if err := drv.Migrate(context.Background(), adapter, "bench"); err != nil {
		_ = drv.Close()
		return nil, fmt.Errorf("migrate in-memory bench db: %w", err)
	}

	return &Store{drv: drv, path: ":memory:"}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.drv.Close()
}

// --- Projects ---

// SaveProject creates or updates a project.
func (s *Store) SaveProject(ctx context.Context, p *Project) error {
	_, err := s.drv.Exec(ctx, `
		INSERT INTO bench_projects (id, repo_url, commit_hash, language, test_cmd, build_cmd, lint_cmd, security_cmd)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			repo_url = excluded.repo_url,
			commit_hash = excluded.commit_hash,
			language = excluded.language,
			test_cmd = excluded.test_cmd,
			build_cmd = excluded.build_cmd,
			lint_cmd = excluded.lint_cmd,
			security_cmd = excluded.security_cmd
	`, p.ID, p.RepoURL, p.CommitHash, p.Language, p.TestCmd, p.BuildCmd, p.LintCmd, p.SecurityCmd)
	if err != nil {
		return fmt.Errorf("save project %s: %w", p.ID, err)
	}
	return nil
}

// GetProject returns a project by ID.
func (s *Store) GetProject(ctx context.Context, id string) (*Project, error) {
	row := s.drv.QueryRow(ctx, `
		SELECT id, repo_url, commit_hash, language, test_cmd, build_cmd, lint_cmd, security_cmd, created_at
		FROM bench_projects WHERE id = ?
	`, id)

	p := &Project{}
	var createdAt string
	if err := row.Scan(&p.ID, &p.RepoURL, &p.CommitHash, &p.Language, &p.TestCmd, &p.BuildCmd, &p.LintCmd, &p.SecurityCmd, &createdAt); err != nil {
		return nil, fmt.Errorf("get project %s: %w", id, err)
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return p, nil
}

// ListProjects returns all projects.
func (s *Store) ListProjects(ctx context.Context) ([]*Project, error) {
	rows, err := s.drv.Query(ctx, `
		SELECT id, repo_url, commit_hash, language, test_cmd, build_cmd, lint_cmd, security_cmd, created_at
		FROM bench_projects ORDER BY id
	`)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []*Project
	for rows.Next() {
		p := &Project{}
		var createdAt string
		if err := rows.Scan(&p.ID, &p.RepoURL, &p.CommitHash, &p.Language, &p.TestCmd, &p.BuildCmd, &p.LintCmd, &p.SecurityCmd, &createdAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// DeleteProject deletes a project and all related data (tasks, runs, phase results, judgments, frozen outputs).
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	// Get tasks for this project to cascade through their children
	tasks, err := s.ListTasks(ctx, id, "")
	if err != nil {
		return fmt.Errorf("delete project %s: list tasks: %w", id, err)
	}
	for _, t := range tasks {
		if err := s.DeleteTask(ctx, t.ID); err != nil {
			return fmt.Errorf("delete project %s: %w", id, err)
		}
	}
	if _, err := s.drv.Exec(ctx, `DELETE FROM bench_projects WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete project %s: %w", id, err)
	}
	return nil
}

// --- Tasks ---

// SaveTask creates or updates a task.
func (s *Store) SaveTask(ctx context.Context, t *Task) error {
	_, err := s.drv.Exec(ctx, `
		INSERT INTO bench_tasks (id, project_id, tier, category, title, description, pre_fix_commit, reference_pr_url, reference_diff, test_patch)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id = excluded.project_id,
			tier = excluded.tier,
			category = excluded.category,
			title = excluded.title,
			description = excluded.description,
			pre_fix_commit = excluded.pre_fix_commit,
			reference_pr_url = excluded.reference_pr_url,
			reference_diff = excluded.reference_diff,
			test_patch = excluded.test_patch
	`, t.ID, t.ProjectID, string(t.Tier), t.Category, t.Title, t.Description, t.PreFixCommit, t.ReferencePRURL, t.ReferenceDiff, t.TestPatch)
	if err != nil {
		return fmt.Errorf("save task %s: %w", t.ID, err)
	}
	return nil
}

// GetTask returns a task by ID.
func (s *Store) GetTask(ctx context.Context, id string) (*Task, error) {
	row := s.drv.QueryRow(ctx, `
		SELECT id, project_id, tier, category, title, description, pre_fix_commit, reference_pr_url, reference_diff, test_patch, excluded, exclude_reason, created_at
		FROM bench_tasks WHERE id = ?
	`, id)

	t := &Task{}
	var createdAt string
	if err := row.Scan(&t.ID, &t.ProjectID, &t.Tier, &t.Category, &t.Title, &t.Description, &t.PreFixCommit, &t.ReferencePRURL, &t.ReferenceDiff, &t.TestPatch, &t.Excluded, &t.ExcludeReason, &createdAt); err != nil {
		return nil, fmt.Errorf("get task %s: %w", id, err)
	}
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return t, nil
}

// ListTasks returns all tasks, optionally filtered by project and/or tier.
func (s *Store) ListTasks(ctx context.Context, projectID string, tier Tier) ([]*Task, error) {
	query := `SELECT id, project_id, tier, category, title, description, pre_fix_commit, reference_pr_url, reference_diff, test_patch, excluded, exclude_reason, created_at FROM bench_tasks WHERE 1=1`
	var args []any

	if projectID != "" {
		query += ` AND project_id = ?`
		args = append(args, projectID)
	}
	if tier != "" {
		query += ` AND tier = ?`
		args = append(args, string(tier))
	}
	query += ` ORDER BY project_id, tier, id`

	rows, err := s.drv.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tasks []*Task
	for rows.Next() {
		t := &Task{}
		var createdAt string
		if err := rows.Scan(&t.ID, &t.ProjectID, &t.Tier, &t.Category, &t.Title, &t.Description, &t.PreFixCommit, &t.ReferencePRURL, &t.ReferenceDiff, &t.TestPatch, &t.Excluded, &t.ExcludeReason, &createdAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// ExcludeTask marks a task as excluded from comparative analysis.
func (s *Store) ExcludeTask(ctx context.Context, id string, reason string) error {
	_, err := s.drv.Exec(ctx, `UPDATE bench_tasks SET excluded = TRUE, exclude_reason = ? WHERE id = ?`, reason, id)
	if err != nil {
		return fmt.Errorf("exclude task %s: %w", id, err)
	}
	return nil
}

// IncludeTask removes exclusion from a task.
func (s *Store) IncludeTask(ctx context.Context, id string) error {
	_, err := s.drv.Exec(ctx, `UPDATE bench_tasks SET excluded = FALSE, exclude_reason = '' WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("include task %s: %w", id, err)
	}
	return nil
}

// DeleteTask deletes a task and all related data (runs, phase results, judgments, frozen outputs).
func (s *Store) DeleteTask(ctx context.Context, id string) error {
	// Delete judgments and phase results for all runs of this task
	runs, err := s.ListRuns(ctx, "", id, "")
	if err != nil {
		return fmt.Errorf("delete task %s: list runs: %w", id, err)
	}
	for _, r := range runs {
		if _, err := s.drv.Exec(ctx, `DELETE FROM bench_judgments WHERE run_id = ?`, r.ID); err != nil {
			return fmt.Errorf("delete task %s: delete judgments for run %s: %w", id, r.ID, err)
		}
		if _, err := s.drv.Exec(ctx, `DELETE FROM bench_phase_results WHERE run_id = ?`, r.ID); err != nil {
			return fmt.Errorf("delete task %s: delete phase results for run %s: %w", id, r.ID, err)
		}
	}
	// Delete runs
	if _, err := s.drv.Exec(ctx, `DELETE FROM bench_runs WHERE task_id = ?`, id); err != nil {
		return fmt.Errorf("delete task %s runs: %w", id, err)
	}
	// Delete frozen outputs
	if _, err := s.drv.Exec(ctx, `DELETE FROM bench_frozen_outputs WHERE task_id = ?`, id); err != nil {
		return fmt.Errorf("delete task %s frozen outputs: %w", id, err)
	}
	// Delete the task itself
	if _, err := s.drv.Exec(ctx, `DELETE FROM bench_tasks WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete task %s: %w", id, err)
	}
	return nil
}

// TasksForVariant returns tasks applicable to a variant based on its phase overrides.
// Baseline (no overrides) runs all tasks. Phase-override variants only run against
// tasks whose tier's workflow actually contains the overridden phase. For example,
// a spec-override variant only runs against medium+large tasks (trivial/small
// workflows have no spec phase), avoiding wasteful runs identical to baseline.
func (s *Store) TasksForVariant(ctx context.Context, v *Variant) ([]*Task, error) {
	// Baseline runs all tasks across all tiers
	if v.IsBaseline || len(v.PhaseOverrides) == 0 {
		return s.ListTasks(ctx, "", "")
	}

	// Determine applicable tiers: explicit restriction > inferred from phase overrides
	tierSet := make(map[Tier]bool)
	if len(v.ApplicableTiers) > 0 {
		// Variant explicitly declares which tiers it cares about
		for _, t := range v.ApplicableTiers {
			tierSet[t] = true
		}
	} else {
		// Infer from overridden phases (original behavior)
		for phaseID := range v.PhaseOverrides {
			if tiers, ok := PhaseApplicableTiers[phaseID]; ok {
				for _, t := range tiers {
					tierSet[t] = true
				}
			}
		}
	}

	// Collect tasks for applicable tiers (in order)
	var allTasks []*Task
	for _, tier := range []Tier{TierTrivial, TierSmall, TierMedium, TierLarge} {
		if tierSet[tier] {
			tasks, err := s.ListTasks(ctx, "", tier)
			if err != nil {
				return nil, err
			}
			allTasks = append(allTasks, tasks...)
		}
	}
	return allTasks, nil
}

// --- Variants ---

// SaveVariant creates or updates a variant.
func (s *Store) SaveVariant(ctx context.Context, v *Variant) error {
	_, err := s.drv.Exec(ctx, `
		INSERT INTO bench_variants (id, name, description, base_workflow, phase_overrides, is_baseline, applicable_tiers)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description,
			base_workflow = excluded.base_workflow,
			phase_overrides = excluded.phase_overrides,
			is_baseline = excluded.is_baseline,
			applicable_tiers = excluded.applicable_tiers
	`, v.ID, v.Name, v.Description, v.BaseWorkflow, v.OverridesJSON(), v.IsBaseline, v.ApplicableTiersJSON())
	if err != nil {
		return fmt.Errorf("save variant %s: %w", v.ID, err)
	}
	return nil
}

// GetVariant returns a variant by ID.
func (s *Store) GetVariant(ctx context.Context, id string) (*Variant, error) {
	row := s.drv.QueryRow(ctx, `
		SELECT id, name, description, base_workflow, phase_overrides, is_baseline, created_at, applicable_tiers
		FROM bench_variants WHERE id = ?
	`, id)

	v := &Variant{}
	var overridesJSON, createdAt, tiersJSON string
	if err := row.Scan(&v.ID, &v.Name, &v.Description, &v.BaseWorkflow, &overridesJSON, &v.IsBaseline, &createdAt, &tiersJSON); err != nil {
		return nil, fmt.Errorf("get variant %s: %w", id, err)
	}
	v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	v.ApplicableTiers = ParseApplicableTiers(tiersJSON)
	var err error
	v.PhaseOverrides, err = ParseOverrides(overridesJSON)
	if err != nil {
		return nil, fmt.Errorf("get variant %s: %w", id, err)
	}
	return v, nil
}

// ListVariants returns all variants.
func (s *Store) ListVariants(ctx context.Context) ([]*Variant, error) {
	rows, err := s.drv.Query(ctx, `
		SELECT id, name, description, base_workflow, phase_overrides, is_baseline, created_at, applicable_tiers
		FROM bench_variants ORDER BY is_baseline DESC, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var variants []*Variant
	for rows.Next() {
		v := &Variant{}
		var overridesJSON, createdAt, tiersJSON string
		if err := rows.Scan(&v.ID, &v.Name, &v.Description, &v.BaseWorkflow, &overridesJSON, &v.IsBaseline, &createdAt, &tiersJSON); err != nil {
			return nil, fmt.Errorf("scan variant: %w", err)
		}
		v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		v.ApplicableTiers = ParseApplicableTiers(tiersJSON)
		v.PhaseOverrides, err = ParseOverrides(overridesJSON)
		if err != nil {
			return nil, fmt.Errorf("parse overrides for variant %s: %w", v.ID, err)
		}
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

// GetBaselineVariant returns the baseline variant (is_baseline = true).
func (s *Store) GetBaselineVariant(ctx context.Context) (*Variant, error) {
	row := s.drv.QueryRow(ctx, `
		SELECT id, name, description, base_workflow, phase_overrides, is_baseline, created_at, applicable_tiers
		FROM bench_variants WHERE is_baseline = TRUE LIMIT 1
	`)

	v := &Variant{}
	var overridesJSON, createdAt, tiersJSON string
	if err := row.Scan(&v.ID, &v.Name, &v.Description, &v.BaseWorkflow, &overridesJSON, &v.IsBaseline, &createdAt, &tiersJSON); err != nil {
		return nil, fmt.Errorf("get baseline variant: %w", err)
	}
	v.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	v.ApplicableTiers = ParseApplicableTiers(tiersJSON)
	var parseErr error
	v.PhaseOverrides, parseErr = ParseOverrides(overridesJSON)
	if parseErr != nil {
		return nil, fmt.Errorf("get baseline variant: parse overrides: %w", parseErr)
	}
	return v, nil
}

// DeleteVariant deletes a variant and all related data (runs, phase results, judgments, frozen outputs).
func (s *Store) DeleteVariant(ctx context.Context, id string) error {
	// Delete judgments and phase results for all runs of this variant
	runs, err := s.ListRuns(ctx, id, "", "")
	if err != nil {
		return fmt.Errorf("delete variant %s: list runs: %w", id, err)
	}
	for _, r := range runs {
		if _, err := s.drv.Exec(ctx, `DELETE FROM bench_judgments WHERE run_id = ?`, r.ID); err != nil {
			return fmt.Errorf("delete variant %s: delete judgments for run %s: %w", id, r.ID, err)
		}
		if _, err := s.drv.Exec(ctx, `DELETE FROM bench_phase_results WHERE run_id = ?`, r.ID); err != nil {
			return fmt.Errorf("delete variant %s: delete phase results for run %s: %w", id, r.ID, err)
		}
	}
	// Delete runs
	if _, err := s.drv.Exec(ctx, `DELETE FROM bench_runs WHERE variant_id = ?`, id); err != nil {
		return fmt.Errorf("delete variant %s runs: %w", id, err)
	}
	// Delete frozen outputs
	if _, err := s.drv.Exec(ctx, `DELETE FROM bench_frozen_outputs WHERE variant_id = ?`, id); err != nil {
		return fmt.Errorf("delete variant %s frozen outputs: %w", id, err)
	}
	// Delete the variant itself
	if _, err := s.drv.Exec(ctx, `DELETE FROM bench_variants WHERE id = ?`, id); err != nil {
		return fmt.Errorf("delete variant %s: %w", id, err)
	}
	return nil
}

// --- Runs ---

// SaveRun creates or updates a run.
func (s *Store) SaveRun(ctx context.Context, r *Run) error {
	var startedAt, completedAt *string
	if !r.StartedAt.IsZero() {
		s := r.StartedAt.Format(time.RFC3339)
		startedAt = &s
	}
	if !r.CompletedAt.IsZero() {
		s := r.CompletedAt.Format(time.RFC3339)
		completedAt = &s
	}

	_, err := s.drv.Exec(ctx, `
		INSERT INTO bench_runs (id, variant_id, task_id, trial_number, status, started_at, completed_at, error_message,
			test_pass, test_count, regression_count, lint_warnings, build_success, security_findings,
			model_diff, test_output, build_output, lint_output, security_output)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at,
			error_message = excluded.error_message,
			test_pass = excluded.test_pass,
			test_count = excluded.test_count,
			regression_count = excluded.regression_count,
			lint_warnings = excluded.lint_warnings,
			build_success = excluded.build_success,
			security_findings = excluded.security_findings,
			model_diff = excluded.model_diff,
			test_output = excluded.test_output,
			build_output = excluded.build_output,
			lint_output = excluded.lint_output,
			security_output = excluded.security_output
	`, r.ID, r.VariantID, r.TaskID, r.TrialNumber, string(r.Status), startedAt, completedAt, r.ErrorMessage,
		r.TestPass, r.TestCount, r.RegressionCount, r.LintWarnings, r.BuildSuccess, r.SecurityFindings,
		r.ModelDiff, r.TestOutput, r.BuildOutput, r.LintOutput, r.SecurityOutput)
	if err != nil {
		return fmt.Errorf("save run %s: %w", r.ID, err)
	}
	return nil
}

// DeleteRunByCombo removes any existing run for the same (variant, task, trial).
// Used to allow clean retries — stale error/fail runs from previous attempts
// would otherwise block the unique constraint.
func (s *Store) DeleteRunByCombo(ctx context.Context, variantID, taskID string, trial int) error {
	// Also clean up associated phase results and judgments
	_, err := s.drv.Exec(ctx, `
		DELETE FROM bench_phase_results WHERE run_id IN (
			SELECT id FROM bench_runs WHERE variant_id = ? AND task_id = ? AND trial_number = ?
		)`, variantID, taskID, trial)
	if err != nil {
		return fmt.Errorf("delete phase results for combo: %w", err)
	}

	_, err = s.drv.Exec(ctx, `
		DELETE FROM bench_judgments WHERE run_id IN (
			SELECT id FROM bench_runs WHERE variant_id = ? AND task_id = ? AND trial_number = ?
		)`, variantID, taskID, trial)
	if err != nil {
		return fmt.Errorf("delete judgments for combo: %w", err)
	}

	_, err = s.drv.Exec(ctx, `
		DELETE FROM bench_runs WHERE variant_id = ? AND task_id = ? AND trial_number = ?
	`, variantID, taskID, trial)
	if err != nil {
		return fmt.Errorf("delete run for combo: %w", err)
	}
	return nil
}

// GetRun returns a run by ID.
func (s *Store) GetRun(ctx context.Context, id string) (*Run, error) {
	row := s.drv.QueryRow(ctx, `
		SELECT id, variant_id, task_id, trial_number, status, started_at, completed_at, error_message, created_at,
			test_pass, test_count, regression_count, lint_warnings, build_success, security_findings,
			model_diff, test_output, build_output, lint_output, security_output
		FROM bench_runs WHERE id = ?
	`, id)

	r := &Run{}
	var startedAt, completedAt, createdAt *string
	if err := row.Scan(&r.ID, &r.VariantID, &r.TaskID, &r.TrialNumber, &r.Status, &startedAt, &completedAt, &r.ErrorMessage, &createdAt,
		&r.TestPass, &r.TestCount, &r.RegressionCount, &r.LintWarnings, &r.BuildSuccess, &r.SecurityFindings,
		&r.ModelDiff, &r.TestOutput, &r.BuildOutput, &r.LintOutput, &r.SecurityOutput); err != nil {
		return nil, fmt.Errorf("get run %s: %w", id, err)
	}
	if startedAt != nil {
		r.StartedAt, _ = time.Parse(time.RFC3339, *startedAt)
	}
	if completedAt != nil {
		r.CompletedAt, _ = time.Parse(time.RFC3339, *completedAt)
	}
	if createdAt != nil {
		r.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
	}
	return r, nil
}

// ListRuns returns runs filtered by variant and/or task and/or status.
// Returns summary data: metrics, status, and model_diff — but omits large output
// fields (test_output, build_output, lint_output, security_output) for performance.
// Use GetRun for full output data.
func (s *Store) ListRuns(ctx context.Context, variantID, taskID string, status RunStatus) ([]*Run, error) {
	query := `SELECT id, variant_id, task_id, trial_number, status, started_at, completed_at, error_message, created_at,
		test_pass, test_count, regression_count, lint_warnings, build_success, security_findings, model_diff
		FROM bench_runs WHERE 1=1`
	var args []any

	if variantID != "" {
		query += ` AND variant_id = ?`
		args = append(args, variantID)
	}
	if taskID != "" {
		query += ` AND task_id = ?`
		args = append(args, taskID)
	}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, string(status))
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.drv.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var runs []*Run
	for rows.Next() {
		r := &Run{}
		var startedAt, completedAt, createdAt *string
		if err := rows.Scan(&r.ID, &r.VariantID, &r.TaskID, &r.TrialNumber, &r.Status, &startedAt, &completedAt, &r.ErrorMessage, &createdAt,
			&r.TestPass, &r.TestCount, &r.RegressionCount, &r.LintWarnings, &r.BuildSuccess, &r.SecurityFindings, &r.ModelDiff); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		if startedAt != nil {
			r.StartedAt, _ = time.Parse(time.RFC3339, *startedAt)
		}
		if completedAt != nil {
			r.CompletedAt, _ = time.Parse(time.RFC3339, *completedAt)
		}
		if createdAt != nil {
			r.CreatedAt, _ = time.Parse(time.RFC3339, *createdAt)
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// CountRunsByStatus returns pass/fail/error counts for a variant.
func (s *Store) CountRunsByStatus(ctx context.Context, variantID string) (pass, fail, errCount int, err error) {
	rows, qErr := s.drv.Query(ctx, `
		SELECT status, COUNT(*) FROM bench_runs WHERE variant_id = ? GROUP BY status
	`, variantID)
	if qErr != nil {
		return 0, 0, 0, fmt.Errorf("count runs: %w", qErr)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var status string
		var count int
		if scanErr := rows.Scan(&status, &count); scanErr != nil {
			return 0, 0, 0, fmt.Errorf("scan run count: %w", scanErr)
		}
		switch RunStatus(status) {
		case RunStatusPass:
			pass = count
		case RunStatusFail:
			fail = count
		case RunStatusError:
			errCount = count
		}
	}
	return pass, fail, errCount, rows.Err()
}

// --- Phase Results ---

// SavePhaseResult saves a phase result.
func (s *Store) SavePhaseResult(ctx context.Context, pr *PhaseResult) error {
	result, err := s.drv.Exec(ctx, `
		INSERT INTO bench_phase_results (
			run_id, phase_id, was_frozen, provider, model, reasoning_effort, thinking_enabled,
			input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_creation_tokens,
			cost_usd, duration_ms, test_pass, test_count, regression_count,
			lint_warnings, coverage_delta, security_findings, frozen_output_id, output_content
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, pr.RunID, pr.PhaseID, pr.WasFrozen, pr.Provider, pr.Model, pr.ReasoningEffort, pr.ThinkingEnabled,
		pr.InputTokens, pr.OutputTokens, pr.ReasoningTokens, pr.CacheReadTokens, pr.CacheCreationTokens,
		pr.CostUSD, pr.DurationMs, pr.TestPass, pr.TestCount, pr.RegressionCount,
		pr.LintWarnings, pr.CoverageDelta, pr.SecurityFindings, pr.FrozenOutputID, pr.OutputContent)
	if err != nil {
		return fmt.Errorf("save phase result: %w", err)
	}
	id, _ := result.LastInsertId()
	pr.ID = int(id)
	return nil
}

// GetPhaseResults returns all phase results for a run.
func (s *Store) GetPhaseResults(ctx context.Context, runID string) ([]*PhaseResult, error) {
	rows, err := s.drv.Query(ctx, `
		SELECT id, run_id, phase_id, was_frozen, provider, model, reasoning_effort, thinking_enabled,
			input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_creation_tokens,
			cost_usd, duration_ms, test_pass, test_count, regression_count,
			lint_warnings, coverage_delta, security_findings, frozen_output_id, output_content, created_at
		FROM bench_phase_results WHERE run_id = ? ORDER BY id
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("get phase results for run %s: %w", runID, err)
	}
	defer func() { _ = rows.Close() }()

	var results []*PhaseResult
	for rows.Next() {
		pr := &PhaseResult{}
		var createdAt string
		if err := rows.Scan(
			&pr.ID, &pr.RunID, &pr.PhaseID, &pr.WasFrozen, &pr.Provider, &pr.Model, &pr.ReasoningEffort, &pr.ThinkingEnabled,
			&pr.InputTokens, &pr.OutputTokens, &pr.ReasoningTokens, &pr.CacheReadTokens, &pr.CacheCreationTokens,
			&pr.CostUSD, &pr.DurationMs, &pr.TestPass, &pr.TestCount, &pr.RegressionCount,
			&pr.LintWarnings, &pr.CoverageDelta, &pr.SecurityFindings, &pr.FrozenOutputID, &pr.OutputContent, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan phase result: %w", err)
		}
		pr.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		results = append(results, pr)
	}
	return results, rows.Err()
}

// --- Frozen Outputs ---

// SaveFrozenOutput saves a frozen output.
func (s *Store) SaveFrozenOutput(ctx context.Context, fo *FrozenOutput) error {
	_, err := s.drv.Exec(ctx, `
		INSERT INTO bench_frozen_outputs (id, task_id, phase_id, variant_id, trial_number, output_content, output_var_name)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, phase_id, variant_id, trial_number) DO UPDATE SET
			id = excluded.id,
			output_content = excluded.output_content,
			output_var_name = excluded.output_var_name
	`, fo.ID, fo.TaskID, fo.PhaseID, fo.VariantID, fo.TrialNumber, fo.OutputContent, fo.OutputVarName)
	if err != nil {
		return fmt.Errorf("save frozen output: %w", err)
	}
	return nil
}

// GetFrozenOutput returns a specific frozen output.
func (s *Store) GetFrozenOutput(ctx context.Context, taskID, phaseID, variantID string, trial int) (*FrozenOutput, error) {
	row := s.drv.QueryRow(ctx, `
		SELECT id, task_id, phase_id, variant_id, trial_number, output_content, output_var_name, created_at
		FROM bench_frozen_outputs WHERE task_id = ? AND phase_id = ? AND variant_id = ? AND trial_number = ?
	`, taskID, phaseID, variantID, trial)

	fo := &FrozenOutput{}
	var createdAt string
	if err := row.Scan(&fo.ID, &fo.TaskID, &fo.PhaseID, &fo.VariantID, &fo.TrialNumber, &fo.OutputContent, &fo.OutputVarName, &createdAt); err != nil {
		return nil, fmt.Errorf("get frozen output: %w", err)
	}
	fo.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return fo, nil
}

// GetFrozenOutputsForTask returns all frozen outputs for a task from a specific variant.
func (s *Store) GetFrozenOutputsForTask(ctx context.Context, taskID, variantID string, trial int) ([]*FrozenOutput, error) {
	rows, err := s.drv.Query(ctx, `
		SELECT id, task_id, phase_id, variant_id, trial_number, output_content, output_var_name, created_at
		FROM bench_frozen_outputs WHERE task_id = ? AND variant_id = ? AND trial_number = ?
		ORDER BY phase_id
	`, taskID, variantID, trial)
	if err != nil {
		return nil, fmt.Errorf("get frozen outputs for task %s: %w", taskID, err)
	}
	defer func() { _ = rows.Close() }()

	var outputs []*FrozenOutput
	for rows.Next() {
		fo := &FrozenOutput{}
		var createdAt string
		if err := rows.Scan(&fo.ID, &fo.TaskID, &fo.PhaseID, &fo.VariantID, &fo.TrialNumber, &fo.OutputContent, &fo.OutputVarName, &createdAt); err != nil {
			return nil, fmt.Errorf("scan frozen output: %w", err)
		}
		fo.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		outputs = append(outputs, fo)
	}
	return outputs, rows.Err()
}

// --- Judgments ---

// SaveJudgment saves a judgment.
func (s *Store) SaveJudgment(ctx context.Context, j *Judgment) error {
	result, err := s.drv.Exec(ctx, `
		INSERT INTO bench_judgments (run_id, phase_id, judge_model, judge_provider, scores, reasoning, presentation_order)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, j.RunID, j.PhaseID, j.JudgeModel, j.JudgeProvider, j.ScoresJSON(), j.Reasoning, j.PresentationOrder)
	if err != nil {
		return fmt.Errorf("save judgment: %w", err)
	}
	id, _ := result.LastInsertId()
	j.ID = int(id)
	return nil
}

// GetJudgments returns all judgments for a run.
func (s *Store) GetJudgments(ctx context.Context, runID string) ([]*Judgment, error) {
	rows, err := s.drv.Query(ctx, `
		SELECT id, run_id, phase_id, judge_model, judge_provider, scores, reasoning, presentation_order, created_at
		FROM bench_judgments WHERE run_id = ? ORDER BY id
	`, runID)
	if err != nil {
		return nil, fmt.Errorf("get judgments for run %s: %w", runID, err)
	}
	defer func() { _ = rows.Close() }()

	var judgments []*Judgment
	for rows.Next() {
		j := &Judgment{}
		var scoresJSON, createdAt string
		if err := rows.Scan(&j.ID, &j.RunID, &j.PhaseID, &j.JudgeModel, &j.JudgeProvider, &scoresJSON, &j.Reasoning, &j.PresentationOrder, &createdAt); err != nil {
			return nil, fmt.Errorf("scan judgment: %w", err)
		}
		j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		var parseErr error
		j.Scores, parseErr = ParseScores(scoresJSON)
		if parseErr != nil {
			return nil, fmt.Errorf("parse judgment scores: %w", parseErr)
		}
		judgments = append(judgments, j)
	}
	return judgments, rows.Err()
}

// GetJudgmentsForPhase returns all judgments for a specific phase across all runs.
func (s *Store) GetJudgmentsForPhase(ctx context.Context, phaseID string) ([]*Judgment, error) {
	rows, err := s.drv.Query(ctx, `
		SELECT id, run_id, phase_id, judge_model, judge_provider, scores, reasoning, presentation_order, created_at
		FROM bench_judgments WHERE phase_id = ? ORDER BY run_id, id
	`, phaseID)
	if err != nil {
		return nil, fmt.Errorf("get judgments for phase %s: %w", phaseID, err)
	}
	defer func() { _ = rows.Close() }()

	var judgments []*Judgment
	for rows.Next() {
		j := &Judgment{}
		var scoresJSON, createdAt string
		if err := rows.Scan(&j.ID, &j.RunID, &j.PhaseID, &j.JudgeModel, &j.JudgeProvider, &scoresJSON, &j.Reasoning, &j.PresentationOrder, &createdAt); err != nil {
			return nil, fmt.Errorf("scan judgment: %w", err)
		}
		j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		var parseErr error
		j.Scores, parseErr = ParseScores(scoresJSON)
		if parseErr != nil {
			return nil, fmt.Errorf("parse judgment scores: %w", parseErr)
		}
		judgments = append(judgments, j)
	}
	return judgments, rows.Err()
}

// --- Cross-Phase Queries ---

// ListPhaseResultsByPhase returns all non-frozen phase results for a given phase across all runs.
// Efficient alternative to iterating all variants -> runs -> phases in Go.
func (s *Store) ListPhaseResultsByPhase(ctx context.Context, phaseID string) ([]*PhaseResult, error) {
	rows, err := s.drv.Query(ctx, `
		SELECT pr.id, pr.run_id, pr.phase_id, pr.was_frozen, pr.provider, pr.model, pr.reasoning_effort, pr.thinking_enabled,
			pr.input_tokens, pr.output_tokens, pr.reasoning_tokens, pr.cache_read_tokens, pr.cache_creation_tokens,
			pr.cost_usd, pr.duration_ms, pr.test_pass, pr.test_count, pr.regression_count,
			pr.lint_warnings, pr.coverage_delta, pr.security_findings, pr.frozen_output_id, pr.output_content, pr.created_at
		FROM bench_phase_results pr
		WHERE pr.phase_id = ? AND pr.was_frozen = FALSE
		ORDER BY pr.run_id, pr.id
	`, phaseID)
	if err != nil {
		return nil, fmt.Errorf("list phase results for phase %s: %w", phaseID, err)
	}
	defer func() { _ = rows.Close() }()

	var results []*PhaseResult
	for rows.Next() {
		pr := &PhaseResult{}
		var createdAt string
		if err := rows.Scan(
			&pr.ID, &pr.RunID, &pr.PhaseID, &pr.WasFrozen, &pr.Provider, &pr.Model, &pr.ReasoningEffort, &pr.ThinkingEnabled,
			&pr.InputTokens, &pr.OutputTokens, &pr.ReasoningTokens, &pr.CacheReadTokens, &pr.CacheCreationTokens,
			&pr.CostUSD, &pr.DurationMs, &pr.TestPass, &pr.TestCount, &pr.RegressionCount,
			&pr.LintWarnings, &pr.CoverageDelta, &pr.SecurityFindings, &pr.FrozenOutputID, &pr.OutputContent, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan phase result: %w", err)
		}
		pr.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		results = append(results, pr)
	}
	return results, rows.Err()
}
