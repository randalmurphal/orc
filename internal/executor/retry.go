// Package executor provides retry logic for cross-phase retry in orc.
package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/state"
)

// Default retry constants
const (
	// DefaultMaxRetries is the default maximum number of retries per phase
	DefaultMaxRetries = 5
)

// DefaultRetryMap returns the default mapping of failed phases to retry phases.
// When a phase fails, this map determines which earlier phase to retry from.
func DefaultRetryMap() map[string]string {
	return map[string]string{
		"test":      "implement",
		"test_unit": "implement",
		"test_e2e":  "implement",
		"validate":  "implement",
	}
}

// SaveRetryContextFile saves detailed retry context to a markdown file.
// This provides a comprehensive record of what failed and why for the
// retried phase to use in addressing the issues.
func SaveRetryContextFile(workDir, taskID, fromPhase, toPhase, reason, output string, attempt int) (string, error) {
	dir := filepath.Join(workDir, ".orc", "tasks", taskID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create task directory: %w", err)
	}

	filename := fmt.Sprintf("retry-context-%s-%d.md", fromPhase, attempt)
	path := filepath.Join(dir, filename)

	content := fmt.Sprintf(`# Retry Context

## Summary
- **From Phase**: %s
- **To Phase**: %s
- **Attempt**: %d
- **Timestamp**: %s

## Reason
%s

## Output from Failed Phase

%s
`, fromPhase, toPhase, attempt, time.Now().Format(time.RFC3339), reason, output)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("write retry context file: %w", err)
	}

	return path, nil
}

// LoadRetryContextForPhase loads retry context from state for prompt injection.
// This builds a summary suitable for inclusion in the {{RETRY_CONTEXT}} template variable.
func LoadRetryContextForPhase(s *state.State) string {
	if s == nil {
		return ""
	}

	rc := s.GetRetryContext()
	if rc == nil {
		return ""
	}

	return BuildRetryContext(rc.FromPhase, rc.Reason, rc.FailureOutput, rc.Attempt, rc.ContextFile)
}

// BuildRetryContext constructs the retry context string for prompt injection.
// This is injected into prompts via the {{RETRY_CONTEXT}} template variable.
func BuildRetryContext(fromPhase, reason, failureOutput string, attempt int, contextFile string) string {
	context := fmt.Sprintf(`## Retry Context

This phase is being re-executed due to a failure in a later phase.

**What happened:**
- Phase "%s" failed/was rejected
- Reason: %s
- This is retry attempt #%d

**What to fix:**
Please address the issues that caused the later phase to fail. The failure output is below:

---
%s
---

Focus on fixing the root cause of these issues in this phase.
`, fromPhase, reason, attempt, failureOutput)

	// If there's a context file with more details, reference it
	if contextFile != "" {
		context += fmt.Sprintf("\nDetailed context saved to: %s\n", contextFile)
	}

	return context
}

// RetryTracker tracks retry counts per phase during task execution.
type RetryTracker struct {
	counts     map[string]int
	maxRetries int
}

// NewRetryTracker creates a new retry tracker with the given max retries.
func NewRetryTracker(maxRetries int) *RetryTracker {
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}
	return &RetryTracker{
		counts:     make(map[string]int),
		maxRetries: maxRetries,
	}
}

// CanRetry returns true if the phase can be retried (hasn't exceeded max retries).
func (rt *RetryTracker) CanRetry(phase string) bool {
	return rt.counts[phase] < rt.maxRetries
}

// Increment increments the retry count for a phase and returns the new count.
func (rt *RetryTracker) Increment(phase string) int {
	rt.counts[phase]++
	return rt.counts[phase]
}

// GetCount returns the current retry count for a phase.
func (rt *RetryTracker) GetCount(phase string) int {
	return rt.counts[phase]
}

// Reset resets the retry count for a phase.
func (rt *RetryTracker) Reset(phase string) {
	delete(rt.counts, phase)
}

// ResetAll clears all retry counts.
func (rt *RetryTracker) ResetAll() {
	rt.counts = make(map[string]int)
}

// RetryOptions configures retry behavior for fresh session retries.
type RetryOptions struct {
	// What failed
	FailedPhase   string
	FailureReason string
	FailureOutput string // Last N chars of output

	// Review feedback
	ReviewComments []db.ReviewComment

	// PR feedback
	PRComments []PRCommentFeedback

	// User guidance
	Instructions string

	// Context from previous session (compressed)
	PreviousContext string

	// Attempt tracking
	AttemptNumber int
	MaxAttempts   int
}

// PRCommentFeedback represents a PR comment to address.
type PRCommentFeedback struct {
	Author   string
	Body     string
	FilePath string
	Line     int
}

// RetryState represents persisted retry state for tracking.
type RetryState struct {
	TaskID        string    `json:"task_id"`
	Phase         string    `json:"phase"`
	AttemptNumber int       `json:"attempt_number"`
	StartedAt     time.Time `json:"started_at"`
	Context       string    `json:"context"` // The injected context
}

// BuildRetryContextForFreshSession builds comprehensive context for a fresh retry session.
// This creates a complete context package for injecting into a new Claude session.
func BuildRetryContextForFreshSession(opts RetryOptions) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# Retry Context\n\n")
	sb.WriteString(fmt.Sprintf("This is attempt %d of %d.\n\n", opts.AttemptNumber, opts.MaxAttempts))

	// Previous failure summary
	sb.WriteString("## Previous Attempt Summary\n\n")
	if opts.FailedPhase != "" {
		sb.WriteString(fmt.Sprintf("Phase `%s` failed on the previous attempt.\n\n", opts.FailedPhase))
	}
	if opts.FailureReason != "" {
		sb.WriteString(fmt.Sprintf("**Reason:** %s\n\n", opts.FailureReason))
	}

	// Failure output (truncated)
	if opts.FailureOutput != "" {
		output := truncateOutput(opts.FailureOutput, 1500)
		sb.WriteString("### Failure Output\n\n")
		sb.WriteString("```\n")
		sb.WriteString(output)
		sb.WriteString("\n```\n\n")
	}

	// Review comments
	if len(opts.ReviewComments) > 0 {
		sb.WriteString("## Review Comments to Address\n\n")
		sb.WriteString(formatReviewCommentsForContext(opts.ReviewComments))
		sb.WriteString("\n")
	}

	// PR comments
	if len(opts.PRComments) > 0 {
		sb.WriteString("## PR Feedback to Address\n\n")
		sb.WriteString(formatPRCommentsForContext(opts.PRComments))
		sb.WriteString("\n")
	}

	// User instructions
	if opts.Instructions != "" {
		sb.WriteString("## Additional Instructions\n\n")
		sb.WriteString(opts.Instructions)
		sb.WriteString("\n\n")
	}

	// Previous context summary (if provided)
	if opts.PreviousContext != "" {
		sb.WriteString("## Context from Previous Session\n\n")
		sb.WriteString(opts.PreviousContext)
		sb.WriteString("\n\n")
	}

	// Call to action
	sb.WriteString("---\n\n")
	sb.WriteString("Please address all issues above and complete the task. ")
	sb.WriteString("Make sure to:\n")
	sb.WriteString("1. Fix all identified issues\n")
	sb.WriteString("2. Run tests to verify fixes\n")
	sb.WriteString("3. Ensure no regressions were introduced\n")

	return sb.String()
}

// formatReviewCommentsForContext formats review comments grouped by file.
func formatReviewCommentsForContext(comments []db.ReviewComment) string {
	// Group by file
	byFile := make(map[string][]db.ReviewComment)
	for _, c := range comments {
		key := c.FilePath
		if key == "" {
			key = "_general"
		}
		byFile[key] = append(byFile[key], c)
	}

	var sb strings.Builder

	// General comments first
	if general, ok := byFile["_general"]; ok {
		sb.WriteString("### General Comments\n\n")
		for _, c := range general {
			severity := normalizeSeverity(string(c.Severity))
			sb.WriteString(fmt.Sprintf("- **[%s]** %s\n", severity, c.Content))
		}
		sb.WriteString("\n")
		delete(byFile, "_general")
	}

	// Build sorted list of files for deterministic output
	files := make([]string, 0, len(byFile))
	for file := range byFile {
		files = append(files, file)
	}
	sort.Strings(files)

	// File-specific comments in sorted order
	for _, file := range files {
		fileComments := byFile[file]
		sb.WriteString(fmt.Sprintf("### `%s`\n\n", file))
		for _, c := range fileComments {
			severity := normalizeSeverity(string(c.Severity))
			if c.LineNumber > 0 {
				sb.WriteString(fmt.Sprintf("- **Line %d** [%s]: %s\n",
					c.LineNumber, severity, c.Content))
			} else {
				sb.WriteString(fmt.Sprintf("- [%s]: %s\n",
					severity, c.Content))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// normalizeSeverity returns an uppercase severity string, defaulting to INFO if empty.
func normalizeSeverity(severity string) string {
	if severity == "" {
		return "INFO"
	}
	return strings.ToUpper(severity)
}

// formatPRCommentsForContext formats PR comments for context.
func formatPRCommentsForContext(comments []PRCommentFeedback) string {
	var sb strings.Builder

	for _, c := range comments {
		if c.FilePath != "" {
			sb.WriteString(fmt.Sprintf("**%s:%d** (@%s)\n", c.FilePath, c.Line, c.Author))
		} else {
			sb.WriteString(fmt.Sprintf("**@%s**:\n", c.Author))
		}
		sb.WriteString(fmt.Sprintf("> %s\n\n", strings.ReplaceAll(c.Body, "\n", "\n> ")))
	}

	return sb.String()
}

// truncateOutput truncates output to maxLen, keeping the end (most relevant).
func truncateOutput(output string, maxLen int) string {
	if len(output) <= maxLen {
		return output
	}
	return "...(truncated)...\n" + output[len(output)-maxLen:]
}

// CompressPreviousContext creates a compressed summary of a previous session.
// This extracts key information from transcripts for injection into retry context.
func CompressPreviousContext(transcripts []db.Transcript) string {
	if len(transcripts) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("Previous session summary:\n")

	// Extract key information from transcripts
	var lastPhase string
	var keyPoints []string

	for _, t := range transcripts {
		if t.Phase != lastPhase {
			lastPhase = t.Phase
			sb.WriteString(fmt.Sprintf("- Phase `%s` was executed\n", t.Phase))
		}

		// Look for key patterns in content
		if strings.Contains(t.Content, "error") || strings.Contains(t.Content, "Error") {
			// Extract error context
			lines := strings.Split(t.Content, "\n")
			for _, line := range lines {
				if isErrorLine(line) {
					keyPoints = append(keyPoints, strings.TrimSpace(line))
					if len(keyPoints) > 5 {
						break
					}
				}
			}
		}
	}

	if len(keyPoints) > 0 {
		sb.WriteString("\nKey issues encountered:\n")
		for _, point := range keyPoints {
			sb.WriteString(fmt.Sprintf("- %s\n", truncateString(point, 200)))
		}
	}

	return sb.String()
}

// isErrorLine checks if a line contains an actual error indicator, avoiding false positives
// like "No errors" or "0 errors".
func isErrorLine(line string) bool {
	lower := strings.ToLower(line)

	// Skip lines that explicitly state no errors
	if strings.Contains(lower, "no error") ||
		strings.Contains(lower, "0 error") ||
		strings.Contains(lower, "zero error") ||
		strings.Contains(lower, "without error") {
		return false
	}

	// Look for actual error patterns
	// Error at start of line or after punctuation/whitespace
	if strings.HasPrefix(lower, "error:") ||
		strings.HasPrefix(lower, "error ") ||
		strings.Contains(lower, ": error:") ||
		strings.Contains(lower, ": error ") ||
		strings.Contains(lower, " error:") ||
		strings.Contains(lower, "\terror:") ||
		strings.Contains(lower, "error[") || // Rust-style errors
		strings.Contains(lower, "failed:") ||
		strings.Contains(lower, "failure:") ||
		strings.Contains(lower, "panic:") ||
		strings.Contains(lower, "fatal:") {
		return true
	}

	// Check for ERROR in uppercase (common log format)
	if strings.Contains(line, "ERROR") ||
		strings.Contains(line, "FAILED") ||
		strings.Contains(line, "FATAL") {
		return true
	}

	return false
}

// truncateString truncates a string to maxLen with ellipsis.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ShouldContinueRetrying checks if we should continue retrying.
func ShouldContinueRetrying(current, max int) bool {
	return current < max
}

// IncrementRetryAttempt returns the next attempt number.
func IncrementRetryAttempt(current int) int {
	return current + 1
}

// BuildRetryPreview builds a preview of the retry context without triggering retry.
// This is useful for showing users what context will be injected.
func BuildRetryPreview(opts RetryOptions) RetryPreview {
	context := BuildRetryContextForFreshSession(opts)
	return RetryPreview{
		TaskID:          "",
		CurrentPhase:    opts.FailedPhase,
		OpenComments:    len(opts.ReviewComments),
		PRComments:      len(opts.PRComments),
		ContextPreview:  context,
		EstimatedTokens: len(context) / 4, // Rough estimate: 4 chars per token
	}
}

// RetryPreview represents a preview of retry context.
type RetryPreview struct {
	TaskID          string `json:"task_id"`
	CurrentPhase    string `json:"current_phase"`
	OpenComments    int    `json:"open_comments"`
	PRComments      int    `json:"pr_comments"`
	ContextPreview  string `json:"context_preview"`
	EstimatedTokens int    `json:"estimated_tokens"`
}
