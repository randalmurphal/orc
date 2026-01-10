// Package executor provides the Ralph-style execution engine for orc.
package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/plan"
	"github.com/randalmurphal/orc/internal/task"
)

// Config holds executor configuration.
type Config struct {
	// ClaudePath is the path to the claude CLI
	ClaudePath string

	// MaxIterations is the maximum iterations per phase (Ralph-style)
	MaxIterations int

	// Timeout is the maximum time per phase
	Timeout time.Duration

	// WorkDir is the working directory for execution
	WorkDir string
}

// DefaultConfig returns the default executor configuration.
func DefaultConfig() *Config {
	return &Config{
		ClaudePath:    "claude",
		MaxIterations: 30,
		Timeout:       10 * time.Minute,
		WorkDir:       ".",
	}
}

// Result represents the result of a phase execution.
type Result struct {
	Phase      string
	Status     plan.PhaseStatus
	Iterations int
	Duration   time.Duration
	Output     string
	Error      error
	Artifacts  []string
	CommitSHA  string
}

// Executor runs task phases using Claude Code.
type Executor struct {
	config *Config
}

// New creates a new executor with the given configuration.
func New(cfg *Config) *Executor {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	return &Executor{config: cfg}
}

// ExecutePhase runs a single phase with Ralph-style looping.
func (e *Executor) ExecutePhase(ctx context.Context, t *task.Task, p *plan.Phase, prompt string) (*Result, error) {
	start := time.Now()
	result := &Result{
		Phase:  p.ID,
		Status: plan.PhaseRunning,
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, e.config.Timeout)
	defer cancel()

	// Ralph-style loop: keep iterating until completion or max iterations
	for iteration := 1; iteration <= e.config.MaxIterations; iteration++ {
		result.Iterations = iteration

		// Check context cancellation
		select {
		case <-ctx.Done():
			result.Status = plan.PhaseFailed
			result.Error = ctx.Err()
			result.Duration = time.Since(start)
			return result, ctx.Err()
		default:
		}

		// Execute Claude with the phase prompt
		output, err := e.runClaude(ctx, prompt)
		if err != nil {
			// Check if it's a retriable error
			if isRetriable(err) && iteration < e.config.MaxIterations {
				continue
			}
			result.Status = plan.PhaseFailed
			result.Error = err
			result.Duration = time.Since(start)
			return result, err
		}

		result.Output = output

		// Check for completion signal
		if e.isPhaseComplete(output, p) {
			result.Status = plan.PhaseCompleted
			result.Duration = time.Since(start)
			return result, nil
		}

		// Check for blocked signal
		if blocked, reason := e.isPhaseBlocked(output); blocked {
			result.Status = plan.PhaseFailed
			result.Error = fmt.Errorf("phase blocked: %s", reason)
			result.Duration = time.Since(start)
			return result, result.Error
		}

		// Continue to next iteration (Ralph-style persistence)
	}

	// Max iterations reached
	result.Status = plan.PhaseFailed
	result.Error = fmt.Errorf("max iterations (%d) reached without completion", e.config.MaxIterations)
	result.Duration = time.Since(start)
	return result, result.Error
}

// runClaude executes Claude Code with the given prompt.
func (e *Executor) runClaude(ctx context.Context, prompt string) (string, error) {
	// Build command
	cmd := exec.CommandContext(ctx, e.config.ClaudePath,
		"--print", // Non-interactive mode
		"--output-format", "text",
		"-p", prompt, // Prompt
	)
	cmd.Dir = e.config.WorkDir

	// Execute
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("claude execution failed: %w", err)
	}

	return string(output), nil
}

// isPhaseComplete checks if the output indicates phase completion.
func (e *Executor) isPhaseComplete(output string, p *plan.Phase) bool {
	// Check for completion tags based on phase
	completionTags := []string{
		fmt.Sprintf("<%s_complete>true</%s_complete>", p.ID, p.ID),
		"<phase_complete>true</phase_complete>",
	}

	for _, tag := range completionTags {
		if strings.Contains(output, tag) {
			return true
		}
	}

	return false
}

// isPhaseBlocked checks if the output indicates the phase is blocked.
func (e *Executor) isPhaseBlocked(output string) (bool, string) {
	if strings.Contains(output, "<phase_blocked>") {
		// Extract reason if present
		start := strings.Index(output, "<phase_blocked>")
		end := strings.Index(output, "</phase_blocked>")
		if start != -1 && end != -1 {
			return true, output[start+15 : end]
		}
		return true, "unknown reason"
	}
	return false, ""
}

// isRetriable determines if an error is retriable.
func isRetriable(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Add retriable error patterns here
	retriablePatterns := []string{
		"timeout",
		"connection refused",
		"rate limit",
	}
	for _, pattern := range retriablePatterns {
		if strings.Contains(strings.ToLower(errStr), pattern) {
			return true
		}
	}
	return false
}
