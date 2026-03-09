// Package bench provides a benchmarking system for comparing model configurations
// across workflow phases. It measures which models perform best at which phases
// using phase-isolation testing with frozen outputs from a baseline run.
package bench

import (
	"encoding/json"
	"fmt"
	"time"
)

// Project is a test project: a pinned repository used for benchmarking.
type Project struct {
	ID          string `yaml:"id" json:"id"`
	RepoURL     string `yaml:"repo_url" json:"repo_url"`
	CommitHash  string `yaml:"commit_hash" json:"commit_hash"`
	Language    string `yaml:"language" json:"language"`
	TestCmd     string `yaml:"test_cmd" json:"test_cmd"`
	BuildCmd    string `yaml:"build_cmd,omitempty" json:"build_cmd,omitempty"`
	LintCmd     string `yaml:"lint_cmd,omitempty" json:"lint_cmd,omitempty"`
	SecurityCmd string `yaml:"security_cmd,omitempty" json:"security_cmd,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Validate checks that required fields are set.
func (p *Project) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("project id is required")
	}
	if p.RepoURL == "" {
		return fmt.Errorf("project %s: repo_url is required", p.ID)
	}
	if p.CommitHash == "" {
		return fmt.Errorf("project %s: commit_hash is required", p.ID)
	}
	if p.Language == "" {
		return fmt.Errorf("project %s: language is required", p.ID)
	}
	if p.TestCmd == "" {
		return fmt.Errorf("project %s: test_cmd is required", p.ID)
	}
	return nil
}

// Tier classifies task complexity.
type Tier string

const (
	TierTrivial Tier = "trivial"
	TierSmall   Tier = "small"
	TierMedium  Tier = "medium"
	TierLarge   Tier = "large"
)

// ValidTiers is the set of valid tier values.
var ValidTiers = map[Tier]bool{
	TierTrivial: true,
	TierSmall:   true,
	TierMedium:  true,
	TierLarge:   true,
}

// Task is a curated benchmark task: a real issue from a real repo with a known solution.
// The model gets the issue description and works on the pre-fix commit.
// After the model finishes, TestPatch is applied and the test suite runs.
// If tests pass, the model solved the problem.
type Task struct {
	ID             string `yaml:"id" json:"id"`
	ProjectID      string `yaml:"project_id" json:"project_id"`
	Tier           Tier   `yaml:"tier" json:"tier"`
	Category       string `yaml:"category,omitempty" json:"category,omitempty"` // bug, feature, refactor, etc.
	Title          string `yaml:"title" json:"title"`
	Description    string `yaml:"description" json:"description"`
	PreFixCommit   string `yaml:"pre_fix_commit" json:"pre_fix_commit"`
	ReferencePRURL string `yaml:"reference_pr_url,omitempty" json:"reference_pr_url,omitempty"`
	ReferenceDiff  string `yaml:"reference_diff,omitempty" json:"reference_diff,omitempty"`
	TestPatch      string `yaml:"test_patch,omitempty" json:"test_patch,omitempty"`      // Test-only diff from reference PR — applied AFTER model finishes for evaluation
	TestPatchFile  string `yaml:"test_patch_file,omitempty" json:"-"`                   // Path to .patch file (resolved during import, content goes into TestPatch)
	Excluded       bool   `yaml:"-" json:"excluded,omitempty"`                          // Exclude from comparative analysis (name mismatch, broken test patch)
	ExcludeReason  string `yaml:"-" json:"exclude_reason,omitempty"`                    // Why this task is excluded
	CreatedAt      time.Time `json:"created_at"`
}

// Validate checks that required fields are set.
func (t *Task) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("task id is required")
	}
	if t.ProjectID == "" {
		return fmt.Errorf("task %s: project_id is required", t.ID)
	}
	if !ValidTiers[t.Tier] {
		return fmt.Errorf("task %s: invalid tier %q", t.ID, t.Tier)
	}
	if t.Title == "" {
		return fmt.Errorf("task %s: title is required", t.ID)
	}
	if t.Description == "" {
		return fmt.Errorf("task %s: description is required", t.ID)
	}
	if t.PreFixCommit == "" {
		return fmt.Errorf("task %s: pre_fix_commit is required", t.ID)
	}
	return nil
}

// PhaseOverride specifies the model configuration for a specific phase in a variant.
// Adding a new model = adding a new PhaseOverride. No code changes needed.
type PhaseOverride struct {
	Provider        string `yaml:"provider" json:"provider"`
	Model           string `yaml:"model" json:"model"`
	ReasoningEffort string `yaml:"reasoning_effort,omitempty" json:"reasoning_effort,omitempty"`
	Thinking        *bool  `yaml:"thinking,omitempty" json:"thinking,omitempty"`
}

// Variant defines a model configuration for benchmarking.
// Each variant targets specific phases with specific model overrides.
// Phases without overrides use frozen outputs from the baseline.
type Variant struct {
	ID             string                   `yaml:"id" json:"id"`
	Name           string                   `yaml:"name" json:"name"`
	Description    string                   `yaml:"description,omitempty" json:"description,omitempty"`
	BaseWorkflow   string                   `yaml:"base_workflow" json:"base_workflow"`
	PhaseOverrides map[string]PhaseOverride `yaml:"phase_overrides" json:"phase_overrides"`
	IsBaseline     bool                     `yaml:"is_baseline,omitempty" json:"is_baseline,omitempty"`
	// ApplicableTiers explicitly restricts which task tiers this variant runs against.
	// When empty, tiers are inferred from PhaseOverrides via PhaseApplicableTiers.
	// When set, only tasks matching these tiers are included — prevents running
	// combo variants against tiers where the combo adds no signal (e.g., running
	// an implement+review combo on trivial tasks that only have implement).
	ApplicableTiers []Tier    `yaml:"applicable_tiers,omitempty" json:"applicable_tiers,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// Validate checks that required fields are set.
func (v *Variant) Validate() error {
	if v.ID == "" {
		return fmt.Errorf("variant id is required")
	}
	if v.Name == "" {
		return fmt.Errorf("variant %s: name is required", v.ID)
	}
	if v.BaseWorkflow == "" {
		return fmt.Errorf("variant %s: base_workflow is required", v.ID)
	}
	return nil
}

// OverridesJSON returns the phase overrides as a JSON string for storage.
func (v *Variant) OverridesJSON() string {
	if v.PhaseOverrides == nil {
		return "{}"
	}
	b, err := json.Marshal(v.PhaseOverrides)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ParseOverrides parses a JSON string into PhaseOverrides.
func ParseOverrides(jsonStr string) (map[string]PhaseOverride, error) {
	if jsonStr == "" || jsonStr == "{}" {
		return nil, nil
	}
	var overrides map[string]PhaseOverride
	if err := json.Unmarshal([]byte(jsonStr), &overrides); err != nil {
		return nil, fmt.Errorf("parse phase overrides: %w", err)
	}
	return overrides, nil
}

// ApplicableTiersJSON returns the applicable tiers as a JSON array string.
func (v *Variant) ApplicableTiersJSON() string {
	if len(v.ApplicableTiers) == 0 {
		return "[]"
	}
	b, err := json.Marshal(v.ApplicableTiers)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// ParseApplicableTiers parses a JSON array of tiers.
func ParseApplicableTiers(jsonStr string) []Tier {
	if jsonStr == "" || jsonStr == "[]" {
		return nil
	}
	var tiers []Tier
	if err := json.Unmarshal([]byte(jsonStr), &tiers); err != nil {
		return nil
	}
	return tiers
}

// RunStatus is the status of a benchmark run.
type RunStatus string

const (
	RunStatusPending RunStatus = "pending"
	RunStatusRunning RunStatus = "running"
	RunStatusPass    RunStatus = "pass"
	RunStatusFail    RunStatus = "fail"
	RunStatusError   RunStatus = "error"
)

// Run is a single execution of a variant against a task.
type Run struct {
	ID           string    `json:"id"`
	VariantID    string    `json:"variant_id"`
	TaskID       string    `json:"task_id"`
	TrialNumber  int       `json:"trial_number"`
	Status       RunStatus `json:"status"`
	StartedAt    time.Time `json:"started_at,omitzero"`
	CompletedAt  time.Time `json:"completed_at,omitzero"`
	ErrorMessage string    `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`

	// Evaluation metrics (populated after evaluator.RunAll completes)
	TestPass         bool `json:"test_pass"`
	TestCount        int  `json:"test_count"`
	RegressionCount  int  `json:"regression_count"`
	LintWarnings     int  `json:"lint_warnings"`
	BuildSuccess     bool `json:"build_success"`
	SecurityFindings int  `json:"security_findings"`

	// ModelDiff is the git diff of the model's changes against the pre-fix commit.
	// Captured before worktree cleanup so we can inspect what the model actually did.
	ModelDiff      string `json:"model_diff,omitempty"`
	TestOutput     string `json:"test_output,omitempty"`
	BuildOutput    string `json:"build_output,omitempty"`
	LintOutput     string `json:"lint_output,omitempty"`
	SecurityOutput string `json:"security_output,omitempty"`
}

// PhaseResult holds metrics for a single phase execution within a run.
type PhaseResult struct {
	ID                   int     `json:"id"`
	RunID                string  `json:"run_id"`
	PhaseID              string  `json:"phase_id"`
	WasFrozen            bool    `json:"was_frozen"`
	Provider             string  `json:"provider"`
	Model                string  `json:"model"`
	ReasoningEffort      string  `json:"reasoning_effort,omitempty"`
	ThinkingEnabled      bool    `json:"thinking_enabled"`
	InputTokens          int     `json:"input_tokens"`
	OutputTokens         int     `json:"output_tokens"`
	ReasoningTokens      int     `json:"reasoning_tokens"`
	CacheReadTokens      int     `json:"cache_read_tokens"`
	CacheCreationTokens  int     `json:"cache_creation_tokens"`
	CostUSD              float64 `json:"cost_usd"`
	DurationMs           int     `json:"duration_ms"`
	TestPass             bool    `json:"test_pass"`
	TestCount            int     `json:"test_count"`
	RegressionCount      int     `json:"regression_count"`
	LintWarnings         int     `json:"lint_warnings"`
	CoverageDelta        float64 `json:"coverage_delta"`
	SecurityFindings     int     `json:"security_findings"`
	FrozenOutputID       string  `json:"frozen_output_id,omitempty"`
	OutputContent        string  `json:"output_content,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
}

// FrozenOutput is a cached phase output for controlled replay.
// When running a variant, all phases except the target use frozen outputs
// from the baseline, ensuring we isolate the effect of changing one phase's model.
type FrozenOutput struct {
	ID            string `json:"id"`
	TaskID        string `json:"task_id"`
	PhaseID       string `json:"phase_id"`
	VariantID     string `json:"variant_id"`
	TrialNumber   int    `json:"trial_number"`
	OutputContent string `json:"output_content"`
	OutputVarName string `json:"output_var_name"`
	CreatedAt     time.Time `json:"created_at"`
}

// Judgment is a cross-model qualitative evaluation.
// Opus judges GPT outputs, GPT judges Claude outputs, Sonnet judges everything.
type Judgment struct {
	ID                int               `json:"id"`
	RunID             string            `json:"run_id"`
	PhaseID           string            `json:"phase_id"`
	JudgeModel        string            `json:"judge_model"`
	JudgeProvider     string            `json:"judge_provider"`
	Scores            map[string]int    `json:"scores"`
	Reasoning         string            `json:"reasoning,omitempty"`
	PresentationOrder int               `json:"presentation_order"`
	CreatedAt         time.Time         `json:"created_at"`
}

// ScoresJSON returns scores as a JSON string for storage.
func (j *Judgment) ScoresJSON() string {
	if j.Scores == nil {
		return "{}"
	}
	b, err := json.Marshal(j.Scores)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ParseScores parses a JSON string into a scores map.
func ParseScores(jsonStr string) (map[string]int, error) {
	if jsonStr == "" || jsonStr == "{}" {
		return nil, nil
	}
	var scores map[string]int
	if err := json.Unmarshal([]byte(jsonStr), &scores); err != nil {
		return nil, fmt.Errorf("parse scores: %w", err)
	}
	return scores, nil
}
