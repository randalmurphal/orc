// Package executor provides retry logic for cross-phase retry in orc.
package executor

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/randalmurphal/orc/internal/state"
)

// Default retry constants
const (
	// DefaultMaxRetries is the default maximum number of retries per phase
	DefaultMaxRetries = 2
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
