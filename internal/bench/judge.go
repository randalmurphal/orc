package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/randalmurphal/orc/internal/executor"
)

// JudgePanel evaluates the final implementation output of benchmark runs.
//
// We're testing orchestrations, not models. Judges evaluate the code a workflow
// produced — they don't know which models ran, which phases existed, or whether
// tests passed. This prevents anchoring bias and ensures independent assessment.
//
// Judges are spawned inside a workspace containing the actual code changes.
// They explore the repo naturally (git diff, file reads, etc.) and assess both
// correctness and quality. Bug context lives in .bench/context.md — not the prompt.
//
// Two frontier judges with extended reasoning (Opus thinking + GPT-5.3-Codex xhigh).
// Both judge every run — blinding mitigates self-evaluation bias:
//   - Content blinding: model names, provider refs stripped from all content
//   - Identity blinding: prompt says "a developer" not "an AI model"
//   - Mixed workflows: the judge can't tell which model did which phases
//
// Cross-referencing judge correctness scores against automated test results
// catches valid alternative solutions that the reference PR's tests can't recognize.
type JudgePanel struct {
	store           *Store
	workspace       *Workspace
	logger          *slog.Logger
	executorFactory func(cfg executor.TurnExecutorConfig) executor.TurnExecutor
	claudePath      string
	codexPath       string
}

// NewJudgePanel creates a new judge panel.
func NewJudgePanel(store *Store, opts ...JudgePanelOption) *JudgePanel {
	jp := &JudgePanel{
		store:           store,
		logger:          slog.Default(),
		executorFactory: executor.NewTurnExecutor,
		claudePath:      "claude",
		codexPath:       "codex",
	}
	for _, opt := range opts {
		opt(jp)
	}
	return jp
}

// JudgePanelOption configures a JudgePanel.
type JudgePanelOption func(*JudgePanel)

// WithJudgeExecutorFactory overrides executor creation.
func WithJudgeExecutorFactory(f func(cfg executor.TurnExecutorConfig) executor.TurnExecutor) JudgePanelOption {
	return func(jp *JudgePanel) { jp.executorFactory = f }
}

// WithJudgeWorkspace sets the workspace for creating judge review environments.
func WithJudgeWorkspace(ws *Workspace) JudgePanelOption {
	return func(jp *JudgePanel) { jp.workspace = ws }
}

// WithJudgeLogger sets the logger.
func WithJudgeLogger(l *slog.Logger) JudgePanelOption {
	return func(jp *JudgePanel) { jp.logger = l }
}

// JudgeConfig controls a single judge's identity and reasoning capabilities.
type JudgeConfig struct {
	Provider        string // "claude" or "codex"
	Model           string // "opus", "gpt-5.3-codex"
	ReasoningEffort string // Codex reasoning effort: "high", "xhigh"
	Thinking        bool   // Claude extended thinking (MAX_THINKING_TOKENS)
}

// DefaultJudgeConfigs returns the 2-judge frontier panel.
// Both judges evaluate every run — blinding mitigates self-evaluation bias.
// Only frontier models with extended reasoning produce reliable evaluations.
func DefaultJudgeConfigs() []JudgeConfig {
	return []JudgeConfig{
		{Provider: "claude", Model: "opus", Thinking: true},
		{Provider: "codex", Model: "gpt-5.3-codex", ReasoningEffort: "xhigh"},
	}
}

// ImplementRubric is the fixed rubric for evaluating implementation quality.
// This is the only rubric — we don't judge intermediate phases.
//
// Criteria are chosen to provide independent signal:
//   - functional_correctness: Did the fix work? (catches valid alternatives that tests miss)
//   - completeness: Full fix or partial/workaround?
//   - code_quality: Clean, idiomatic, follows conventions?
//   - minimal_change: Focused on the problem or scattered changes?
var ImplementRubric = JudgeRubric{
	PhaseID:  "implement",
	Criteria: []string{"functional_correctness", "completeness", "code_quality", "minimal_change"},
	MaxScore: 5,
}

// JudgeRubric defines the scoring criteria.
type JudgeRubric struct {
	PhaseID    string
	Criteria   []string
	MaxScore   int
	SystemNote string
}

// JudgeRequest is the input to a judge evaluation.
// Deliberately excludes test results — judges must form independent
// correctness assessments without anchoring on pass/fail.
type JudgeRequest struct {
	TaskTitle string
	TaskDesc  string
	Rubric    JudgeRubric
}

// JudgeResponse is the parsed output from a judge.
type JudgeResponse struct {
	Scores    map[string]int `json:"scores"`
	Reasoning string         `json:"reasoning"`
}

// judgeContext carries everything executeJudge needs to set up a workspace.
type judgeContext struct {
	Run     *Run
	Task    *Task
	Project *Project
}

// EvaluateRun judges the implementation output of a run.
// Only the implement phase is evaluated — that's where the orchestration's
// quality is visible. Non-implement phases (spec, tdd, review) are intermediate
// artifacts whose value is measured by the final implementation, not in isolation.
func (jp *JudgePanel) EvaluateRun(ctx context.Context, runID string, judges []JudgeConfig) error {
	run, err := jp.store.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get run %s: %w", runID, err)
	}

	// No model diff means nothing to review
	if run.ModelDiff == "" {
		jp.logger.Debug("skipping run with no model diff", "run", runID)
		return nil
	}

	task, err := jp.store.GetTask(ctx, run.TaskID)
	if err != nil {
		return fmt.Errorf("get task %s: %w", run.TaskID, err)
	}

	project, err := jp.store.GetProject(ctx, task.ProjectID)
	if err != nil {
		return fmt.Errorf("get project %s: %w", task.ProjectID, err)
	}

	jctx := &judgeContext{Run: run, Task: task, Project: project}

	// Both frontier judges evaluate every run. Blinding mitigates self-eval bias:
	// content stripping, identity blinding, and mixed-model workflows mean the
	// judge can't reliably identify its own output.
	var succeeded int
	var lastErr error
	for _, judge := range judges {

		order := rand.Intn(100)

		req := JudgeRequest{
			TaskTitle: sanitizeForBlinding(task.Title),
			TaskDesc:  sanitizeForBlinding(task.Description),
			Rubric:    ImplementRubric,
		}

		resp, err := jp.executeJudge(ctx, judge, req, jctx)
		if err != nil {
			lastErr = err
			jp.logger.Warn("judge evaluation failed",
				"judge", judge.Model, "run", runID, "error", err)
			continue
		}

		judgment := &Judgment{
			RunID:             runID,
			PhaseID:           "implement",
			JudgeModel:        judge.Model,
			JudgeProvider:     judge.Provider,
			Scores:            resp.Scores,
			Reasoning:         resp.Reasoning,
			PresentationOrder: order,
		}

		if err := jp.store.SaveJudgment(ctx, judgment); err != nil {
			lastErr = err
			jp.logger.Warn("save judgment failed", "error", err)
			continue
		}
		succeeded++
	}

	if succeeded == 0 && len(judges) > 0 {
		return fmt.Errorf("all %d judges failed for run %s: %w", len(judges), runID, lastErr)
	}
	return nil
}

// executeJudge runs a single judge evaluation.
// The judge is always spawned inside a workspace containing the model's actual
// changes. It can run git diff, read files, and explore the codebase naturally —
// like a real code reviewer.
func (jp *JudgePanel) executeJudge(ctx context.Context, judge JudgeConfig, req JudgeRequest, jctx *judgeContext) (*JudgeResponse, error) {
	if jp.workspace == nil {
		return nil, fmt.Errorf("workspace required for judge evaluation")
	}
	if jctx.Run.ModelDiff == "" {
		return nil, fmt.Errorf("no model diff available for judge evaluation")
	}

	dir, cleanup, err := jp.setupJudgeWorkspace(jctx)
	if err != nil {
		return nil, fmt.Errorf("judge workspace setup for task %s: %w", jctx.Task.ID, err)
	}
	defer cleanup()

	// Write bug context to a file so the prompt stays fixed-size.
	// Large task descriptions won't blow up the prompt context.
	if err := writeContextFile(dir, req); err != nil {
		return nil, fmt.Errorf("write context file: %w", err)
	}

	prompt := buildJudgePrompt(req)

	cfg := executor.TurnExecutorConfig{
		Provider:                  judge.Provider,
		Model:                    judge.Model,
		WorkingDir:                dir,
		PhaseID:                  "bench-judge",
		TaskID:                   "judge",
		RunID:                    "judge",
		MaxTurns:                 50, // Generous ceiling — model stops naturally when done
		ClaudePath:               jp.claudePath,
		CodexPath:                jp.codexPath,
		BypassApprovalsAndSandbox: true,
		ReasoningEffort:          judge.ReasoningEffort,
	}

	// Enable extended thinking for Claude judges via MAX_THINKING_TOKENS env var
	if judge.Thinking {
		cfg.ClaudeConfig = &executor.PhaseClaudeConfig{
			Env: map[string]string{
				"MAX_THINKING_TOKENS": "31999",
			},
		}
	}

	turnExec := jp.executorFactory(cfg)
	result, err := turnExec.ExecuteTurnWithoutSchema(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("judge execution failed: %w", err)
	}

	return parseJudgeResponse(result.Content, req.Rubric)
}

// setupJudgeWorkspace creates a temporary worktree with the model's changes applied.
// Returns the worktree path and a cleanup function. The judge sees the repo in the
// exact state the model left it: pre-fix commit + model's diff applied.
func (jp *JudgePanel) setupJudgeWorkspace(jctx *judgeContext) (string, func(), error) {
	// Use a unique ID so multiple judges can run concurrently on different runs
	judgeRunID := "judge-" + uuid.New().String()[:8]

	worktreePath, err := jp.workspace.SetupRun(judgeRunID, jctx.Project, jctx.Task)
	if err != nil {
		return "", nil, fmt.Errorf("setup worktree: %w", err)
	}

	repoDir := jp.workspace.ReposDir + "/" + jctx.Project.ID
	cleanup := func() {
		jp.workspace.CleanupRun(judgeRunID, repoDir)
	}

	// Apply the model's diff to recreate the post-model state
	cmd := exec.Command("git", "apply", "--allow-empty", "-")
	cmd.Dir = worktreePath
	cmd.Stdin = strings.NewReader(jctx.Run.ModelDiff)

	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("apply model diff: %s: %w", strings.TrimSpace(stderr.String()), err)
	}

	// Commit the applied changes so `git diff HEAD~1` works naturally
	addCmd := exec.Command("git", "add", "-A")
	addCmd.Dir = worktreePath
	if err := addCmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("git add: %w", err)
	}

	commitCmd := exec.Command("git", "commit", "--allow-empty", "-m", "Model changes for review")
	commitCmd.Dir = worktreePath
	commitCmd.Env = append(commitCmd.Environ(),
		"GIT_AUTHOR_NAME=bench-judge",
		"GIT_AUTHOR_EMAIL=bench@judge",
		"GIT_COMMITTER_NAME=bench-judge",
		"GIT_COMMITTER_EMAIL=bench@judge",
	)
	if err := commitCmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("git commit: %w", err)
	}

	return worktreePath, cleanup, nil
}

// buildJudgePrompt constructs the evaluation prompt for an implementation review.
// The judge is inside the repo with the model's changes committed.
//
// The prompt deliberately:
//   - Says "developer" not "AI" (identity blinding)
//   - Omits test results (prevents anchoring on pass/fail)
//   - References .bench/context.md for bug details (keeps prompt fixed-size)
//   - Provides detailed score anchors at each level (calibrated scoring)
func buildJudgePrompt(req JudgeRequest) string {
	var sb strings.Builder

	sb.WriteString(`You are a senior engineer conducting a code review.

A developer attempted to fix a bug in this repository. Their changes are in the most recent commit.

## How to Review

1. Read ` + "`.bench/context.md`" + ` for the bug description
2. Run ` + "`git diff HEAD~1`" + ` to see what changed
3. Read the changed files in full — understand the fix in context, not just the diff
4. Determine whether the changes actually fix the described bug
5. Evaluate the implementation quality

Do NOT just look at the diff in isolation. Read the surrounding code to understand whether the fix makes sense.

## Scoring Criteria

Score each criterion independently from 1 to 5:

**functional_correctness** — Does this fix the described bug?
  5: Correctly identifies and fixes the root cause
  4: Fixes the bug but approach is suboptimal
  3: Partially fixes the bug or only handles some cases
  2: Attempts a fix but misses the actual problem
  1: No meaningful fix, or introduces new bugs

**completeness** — Is the fix thorough?
  5: All cases handled, edge cases considered
  4: Main cases handled, minor gaps
  3: Core fix present but notable gaps remain
  2: Significant parts of the problem unaddressed
  1: Barely started or placeholder code

**code_quality** — Is the code well-written?
  5: Clean, idiomatic, follows project conventions
  4: Good quality with minor style issues
  3: Functional but messy or inconsistent
  2: Hard to read, poor structure
  1: Incomprehensible or clearly wrong patterns

**minimal_change** — Are the changes appropriately scoped?
  5: Precisely targeted — only what's needed to fix the bug
  4: Mostly focused with minor extras
  3: Some unnecessary changes mixed in
  2: Significant unrelated modifications
  1: Scattered across unrelated files, heavily bloated

## Response Format

After completing your review, respond with ONLY this JSON (no markdown fences, no extra text):

{
  "scores": {
`)
	for i, c := range req.Rubric.Criteria {
		comma := ","
		if i == len(req.Rubric.Criteria)-1 {
			comma = ""
		}
		fmt.Fprintf(&sb, "    \"%s\": <1-%d>%s\n", c, req.Rubric.MaxScore, comma)
	}
	sb.WriteString(`  },
  "reasoning": "<2-4 sentences explaining your assessment>"
}
`)

	return sb.String()
}

// blindingPatterns is a compiled regex that matches model-identifying content.
// Case-insensitive to catch all variations (Claude, CLAUDE, claude, etc.)
var blindingPatterns = regexp.MustCompile(
	`(?im)` +
		// Co-Authored-By lines (entire line, any model/email)
		`(^[Cc]o-[Aa]uthored-[Bb]y:\s*.*$)` +
		// Model names with optional version suffixes
		`|(claude[\s-]*(opus|sonnet|haiku)[\s-]*[\d.]*)` +
		`|(claude[\s-]+[\d.]+)` +
		`|(gpt[\s-]*[\d]+[\w.\-]*)` +
		`|(codex[\s-]*[\d]*)` +
		`|(o[134][\s-]*(mini|preview)?)` +
		`|(gemini[\s-]*[\d.]*[\w\-]*)` +
		`|(deepseek[\s-]*\w*)` +
		`|(mistral[\s-]*\w*)` +
		`|(llama[\s-]*[\d.]*)` +
		// Standalone model family names (word boundaries)
		`|(\bclaude\b)` +
		// Provider names
		`|(\banthrop\w+\b)` +
		`|(\bopenai\b)` +
		`|(\bdeepseek\b)` +
		`|(\bdeep\s*mind\b)` +
		`|(\bmeta\s*ai\b)` +
		// Provider email addresses
		`|(noreply@[\w.]+\.com)` +
		// Orc commit prefix
		`|(\[orc\])`,
)

// sanitizeForBlinding strips model-identifying content from output before judging.
// Uses case-insensitive regex to catch all variations of model names, provider
// references, co-author attribution lines, and framework markers.
func sanitizeForBlinding(content string) string {
	return blindingPatterns.ReplaceAllString(content, "[REDACTED]")
}

// writeContextFile writes the bug description to .bench/context.md in the workspace.
// This keeps the prompt fixed-size (just instructions + rubric) while the variable-
// length bug context lives in a file the judge reads during review.
func writeContextFile(dir string, req JudgeRequest) error {
	var sb strings.Builder
	sb.WriteString("# Bug Report\n\n")
	fmt.Fprintf(&sb, "**Title:** %s\n\n", req.TaskTitle)
	sb.WriteString("## Description\n\n")
	sb.WriteString(req.TaskDesc)
	sb.WriteString("\n")

	benchDir := filepath.Join(dir, ".bench")
	if err := os.MkdirAll(benchDir, 0755); err != nil {
		return fmt.Errorf("create .bench dir: %w", err)
	}
	return os.WriteFile(filepath.Join(benchDir, "context.md"), []byte(sb.String()), 0644)
}

// parseJudgeResponse extracts scores from the judge's output.
func parseJudgeResponse(content string, rubric JudgeRubric) (*JudgeResponse, error) {
	// Try to find JSON in the response
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("no JSON found in judge response")
	}

	jsonStr := content[start : end+1]

	var resp JudgeResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("parse judge response: %w", err)
	}

	// Validate scores are within range
	var missing []string
	for _, criterion := range rubric.Criteria {
		score, ok := resp.Scores[criterion]
		if !ok {
			missing = append(missing, criterion)
			continue
		}
		if score < 1 || score > rubric.MaxScore {
			if score < 1 {
				resp.Scores[criterion] = 1
			} else {
				resp.Scores[criterion] = rubric.MaxScore
			}
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("judge response missing criteria: %v", missing)
	}

	return &resp, nil
}

// AggregateJudgments computes average scores across multiple judgments.
func AggregateJudgments(judgments []*Judgment) map[string]float64 {
	if len(judgments) == 0 {
		return nil
	}

	sums := make(map[string]float64)
	counts := make(map[string]int)

	for _, j := range judgments {
		for criterion, score := range j.Scores {
			sums[criterion] += float64(score)
			counts[criterion]++
		}
	}

	result := make(map[string]float64)
	for criterion, sum := range sums {
		if counts[criterion] > 0 {
			result[criterion] = sum / float64(counts[criterion])
		}
	}
	return result
}
