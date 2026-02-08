package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"

	"github.com/randalmurphal/orc/internal/executor"
)

// JudgePanel manages cross-model evaluation of benchmark outputs.
// The panel uses blinding and randomized presentation order to reduce bias:
//   - Opus judges GPT outputs only
//   - GPT judges Claude outputs only
//   - Sonnet judges everything (cheap tiebreaker)
type JudgePanel struct {
	store           *Store
	executorFactory func(cfg executor.TurnExecutorConfig) executor.TurnExecutor
	claudePath      string
	codexPath       string
}

// NewJudgePanel creates a new judge panel.
func NewJudgePanel(store *Store, opts ...JudgePanelOption) *JudgePanel {
	jp := &JudgePanel{
		store:           store,
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

// JudgeConfig controls which judges evaluate which outputs.
type JudgeConfig struct {
	Provider string // "claude" or "codex"
	Model    string // "opus", "sonnet", "gpt-5.3-codex"
	// JudgesProviders lists which output providers this judge evaluates.
	// Empty means judge everything (used for Sonnet tiebreaker).
	JudgesProviders []string
}

// DefaultJudgeConfigs returns the standard 3-judge panel.
func DefaultJudgeConfigs() []JudgeConfig {
	return []JudgeConfig{
		{
			Provider:        "claude",
			Model:           "opus",
			JudgesProviders: []string{"codex"}, // Opus judges GPT outputs
		},
		{
			Provider:        "codex",
			Model:           "gpt-5.3-codex",
			JudgesProviders: []string{"claude"}, // GPT judges Claude outputs
		},
		{
			Provider:        "claude",
			Model:           "sonnet",
			JudgesProviders: nil, // Sonnet judges everything
		},
	}
}

// JudgeRubric defines the scoring criteria for a phase.
type JudgeRubric struct {
	PhaseID    string
	Criteria   []string // e.g. ["completeness", "correctness", "clarity", "efficiency"]
	MaxScore   int      // Per criterion (typically 5)
	SystemNote string   // Additional context for the judge
}

// DefaultRubric returns the default rubric for a phase.
func DefaultRubric(phaseID string) JudgeRubric {
	criteria := []string{"completeness", "correctness", "clarity", "efficiency"}

	switch phaseID {
	case "spec", "tiny_spec":
		criteria = []string{"completeness", "clarity", "testability", "scope_appropriateness"}
	case "tdd_write", "tdd_integrate":
		criteria = []string{"coverage", "correctness", "isolation", "readability"}
	case "implement":
		criteria = []string{"correctness", "completeness", "code_quality", "efficiency"}
	case "review":
		criteria = []string{"thoroughness", "actionability", "accuracy", "prioritization"}
	case "docs":
		criteria = []string{"completeness", "clarity", "accuracy", "usefulness"}
	}

	return JudgeRubric{
		PhaseID:  phaseID,
		Criteria: criteria,
		MaxScore: 5,
	}
}

// JudgeRequest is the input to a judge evaluation.
type JudgeRequest struct {
	PhaseID       string
	TaskTitle     string
	TaskDesc      string
	OutputContent string
	Rubric        JudgeRubric
	// BlindedLabel hides the variant identity from the judge.
	// e.g. "Output A" instead of "codex53-high-implement"
	BlindedLabel string
}

// JudgeResponse is the parsed output from a judge.
type JudgeResponse struct {
	Scores    map[string]int `json:"scores"`
	Reasoning string         `json:"reasoning"`
}

// EvaluateRun judges all executed (non-frozen) phases of a run.
func (jp *JudgePanel) EvaluateRun(ctx context.Context, runID string, judges []JudgeConfig) error {
	run, err := jp.store.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get run %s: %w", runID, err)
	}

	phases, err := jp.store.GetPhaseResults(ctx, runID)
	if err != nil {
		return fmt.Errorf("get phase results for %s: %w", runID, err)
	}

	// Get the variant to know the provider
	variant, err := jp.store.GetVariant(ctx, run.VariantID)
	if err != nil {
		return fmt.Errorf("get variant %s: %w", run.VariantID, err)
	}

	// Get the task for context
	task, err := jp.store.GetTask(ctx, run.TaskID)
	if err != nil {
		return fmt.Errorf("get task %s: %w", run.TaskID, err)
	}

	for _, phase := range phases {
		if phase.WasFrozen || phase.OutputContent == "" {
			continue
		}

		rubric := DefaultRubric(phase.PhaseID)

		for _, judge := range judges {
			// Check if this judge should evaluate this output
			if !jp.shouldJudge(judge, phase.Provider, phase.Model) {
				continue
			}

			// Randomize presentation order
			order := rand.Intn(100)

			req := JudgeRequest{
				PhaseID:       phase.PhaseID,
				TaskTitle:     sanitizeForBlinding(task.Title),
				TaskDesc:      sanitizeForBlinding(task.Description),
				OutputContent: sanitizeForBlinding(phase.OutputContent),
				Rubric:        rubric,
				BlindedLabel:  fmt.Sprintf("Output-%d", order%100),
			}

			resp, err := jp.executeJudge(ctx, judge, req, variant)
			if err != nil {
				// Log and continue, don't fail the whole panel
				continue
			}

			judgment := &Judgment{
				RunID:             runID,
				PhaseID:           phase.PhaseID,
				JudgeModel:        judge.Model,
				JudgeProvider:     judge.Provider,
				Scores:            resp.Scores,
				Reasoning:         resp.Reasoning,
				PresentationOrder: order,
			}

			if err := jp.store.SaveJudgment(ctx, judgment); err != nil {
				continue
			}
		}
	}

	return nil
}

// EvaluatePhase judges a specific phase across all completed runs.
func (jp *JudgePanel) EvaluatePhase(ctx context.Context, phaseID string, judges []JudgeConfig) error {
	// Get all runs
	runs, err := jp.store.ListRuns(ctx, "", "", "")
	if err != nil {
		return fmt.Errorf("list runs: %w", err)
	}

	for _, run := range runs {
		if run.Status != RunStatusPass && run.Status != RunStatusFail {
			continue
		}

		phases, err := jp.store.GetPhaseResults(ctx, run.ID)
		if err != nil {
			continue
		}

		for _, phase := range phases {
			if phase.PhaseID != phaseID || phase.WasFrozen || phase.OutputContent == "" {
				continue
			}

			variant, err := jp.store.GetVariant(ctx, run.VariantID)
			if err != nil {
				continue
			}

			task, err := jp.store.GetTask(ctx, run.TaskID)
			if err != nil {
				continue
			}

			rubric := DefaultRubric(phaseID)

			for _, judge := range judges {
				if !jp.shouldJudge(judge, phase.Provider, phase.Model) {
					continue
				}

				order := rand.Intn(100)

				req := JudgeRequest{
					PhaseID:       phaseID,
					TaskTitle:     sanitizeForBlinding(task.Title),
					TaskDesc:      sanitizeForBlinding(task.Description),
					OutputContent: sanitizeForBlinding(phase.OutputContent),
					Rubric:        rubric,
					BlindedLabel:  fmt.Sprintf("Output-%d", order%100),
				}

				resp, err := jp.executeJudge(ctx, judge, req, variant)
				if err != nil {
					continue
				}

				judgment := &Judgment{
					RunID:             run.ID,
					PhaseID:           phaseID,
					JudgeModel:        judge.Model,
					JudgeProvider:     judge.Provider,
					Scores:            resp.Scores,
					Reasoning:         resp.Reasoning,
					PresentationOrder: order,
				}

				if err := jp.store.SaveJudgment(ctx, judgment); err != nil {
					continue
				}
			}
		}
	}

	return nil
}

// shouldJudge returns true if this judge should evaluate outputs from the given provider/model.
// A judge never evaluates its own model's output (prevents self-evaluation bias).
func (jp *JudgePanel) shouldJudge(judge JudgeConfig, outputProvider, outputModel string) bool {
	// Never judge your own output
	if judge.Provider == outputProvider && judge.Model == outputModel {
		return false
	}
	if len(judge.JudgesProviders) == 0 {
		return true // Judge everything except self (e.g. Sonnet tiebreaker)
	}
	for _, p := range judge.JudgesProviders {
		if p == outputProvider {
			return true
		}
	}
	return false
}

// executeJudge runs a single judge evaluation.
func (jp *JudgePanel) executeJudge(ctx context.Context, judge JudgeConfig, req JudgeRequest, _ *Variant) (*JudgeResponse, error) {
	prompt := buildJudgePrompt(req)

	cfg := executor.TurnExecutorConfig{
		Provider:                  judge.Provider,
		Model:                    judge.Model,
		WorkingDir:                "/tmp",
		PhaseID:                  "bench-judge",
		TaskID:                   "judge",
		RunID:                    "judge",
		MaxTurns:                 1,
		ClaudePath:               jp.claudePath,
		CodexPath:                jp.codexPath,
		BypassApprovalsAndSandbox: true,
	}

	exec := jp.executorFactory(cfg)
	result, err := exec.ExecuteTurnWithoutSchema(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("judge execution failed: %w", err)
	}

	return parseJudgeResponse(result.Content, req.Rubric)
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

// buildJudgePrompt constructs the evaluation prompt for a judge.
func buildJudgePrompt(req JudgeRequest) string {
	var sb strings.Builder

	sb.WriteString("You are an expert code reviewer evaluating the quality of AI-generated output.\n\n")
	sb.WriteString("## Task Context\n")
	sb.WriteString(fmt.Sprintf("**Title:** %s\n", req.TaskTitle))
	sb.WriteString(fmt.Sprintf("**Description:** %s\n\n", req.TaskDesc))
	sb.WriteString(fmt.Sprintf("## Phase: %s\n\n", req.PhaseID))
	sb.WriteString(fmt.Sprintf("## %s\n\n", req.BlindedLabel))
	sb.WriteString("```\n")
	sb.WriteString(req.OutputContent)
	sb.WriteString("\n```\n\n")

	sb.WriteString("## Scoring Criteria\n\n")
	sb.WriteString(fmt.Sprintf("Score each criterion from 1-%d:\n", req.Rubric.MaxScore))
	for _, c := range req.Rubric.Criteria {
		sb.WriteString(fmt.Sprintf("- **%s**: 1 (poor) to %d (excellent)\n", c, req.Rubric.MaxScore))
	}

	sb.WriteString("\n## Instructions\n\n")
	sb.WriteString("1. Evaluate the output quality against each criterion\n")
	sb.WriteString("2. Provide brief reasoning for your scores\n")
	sb.WriteString("3. Respond with JSON in this exact format:\n\n")
	sb.WriteString("```json\n")
	sb.WriteString("{\n")
	sb.WriteString("  \"scores\": {\n")
	for i, c := range req.Rubric.Criteria {
		comma := ","
		if i == len(req.Rubric.Criteria)-1 {
			comma = ""
		}
		sb.WriteString(fmt.Sprintf("    \"%s\": <1-%d>%s\n", c, req.Rubric.MaxScore, comma))
	}
	sb.WriteString("  },\n")
	sb.WriteString("  \"reasoning\": \"<brief explanation>\"\n")
	sb.WriteString("}\n")
	sb.WriteString("```\n")

	if req.Rubric.SystemNote != "" {
		sb.WriteString(fmt.Sprintf("\n**Note:** %s\n", req.Rubric.SystemNote))
	}

	return sb.String()
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
			// Clamp to valid range
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

// AggregateJudgments computes average scores across multiple judgments for a phase.
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
