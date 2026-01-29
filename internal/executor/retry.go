// Package executor provides retry logic for cross-phase retry in orc.
package executor

import (
	"fmt"
	"sort"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
)

// Default retry constants
const (
	// DefaultMaxRetries is the default maximum number of retries per phase
	DefaultMaxRetries = 5
)

// LoadRetryContextFromExecutionProto loads retry context from proto ExecutionState.
func LoadRetryContextFromExecutionProto(e *orcv1.ExecutionState) string {
	if e == nil || e.RetryContext == nil {
		return ""
	}

	rc := e.RetryContext
	failureOutput := ""
	if rc.FailureOutput != nil {
		failureOutput = *rc.FailureOutput
	}
	contextFile := ""
	if rc.ContextFile != nil {
		contextFile = *rc.ContextFile
	}

	return BuildRetryContext(rc.FromPhase, rc.Reason, failureOutput, int(rc.Attempt), contextFile)
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

	// Failure output (generous limit to avoid cutting off important review findings)
	// 250k chars ≈ 60k tokens - enough for comprehensive review output
	if opts.FailureOutput != "" {
		output := truncateOutput(opts.FailureOutput, 250000)
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
