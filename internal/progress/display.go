// Package progress provides progress display for orc task execution.
package progress

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// FileChangeStats contains summary statistics for file changes.
type FileChangeStats struct {
	FilesChanged int
	Additions    int
	Deletions    int
}

// SyncStrategy represents the sync strategy used (for contextual help).
type SyncStrategy string

const (
	// SyncStrategyRebase indicates rebase-based sync.
	SyncStrategyRebase SyncStrategy = "rebase"
	// SyncStrategyMerge indicates merge-based sync.
	SyncStrategyMerge SyncStrategy = "merge"
)

// BlockedContext provides context for blocked task display.
// This enables better guidance for manual conflict resolution.
type BlockedContext struct {
	// WorktreePath is the path to the task's worktree (relative to project root).
	WorktreePath string

	// ConflictFiles is the list of files with conflicts.
	ConflictFiles []string

	// SyncStrategy is the sync strategy being used (rebase or merge).
	SyncStrategy SyncStrategy

	// TargetBranch is the branch being synced with.
	TargetBranch string
}

// ActivityState represents what the executor is currently doing.
type ActivityState string

const (
	// ActivityIdle indicates no activity.
	ActivityIdle ActivityState = "idle"
	// ActivityWaitingAPI indicates waiting for Claude API response.
	ActivityWaitingAPI ActivityState = "waiting_api"
	// ActivityStreaming indicates actively receiving streaming response.
	ActivityStreaming ActivityState = "streaming"
	// ActivityRunningTool indicates Claude is running a tool.
	ActivityRunningTool ActivityState = "running_tool"
	// ActivityProcessing indicates processing response.
	ActivityProcessing ActivityState = "processing"
	// ActivitySpecAnalyzing indicates analyzing codebase during spec phase.
	ActivitySpecAnalyzing ActivityState = "spec_analyzing"
	// ActivitySpecWriting indicates writing specification during spec phase.
	ActivitySpecWriting ActivityState = "spec_writing"
)

// String returns a human-readable description of the activity state.
func (s ActivityState) String() string {
	switch s {
	case ActivityIdle:
		return "Idle"
	case ActivityWaitingAPI:
		return "Waiting for API"
	case ActivityStreaming:
		return "Receiving response"
	case ActivityRunningTool:
		return "Running tool"
	case ActivityProcessing:
		return "Processing"
	case ActivitySpecAnalyzing:
		return "Analyzing codebase"
	case ActivitySpecWriting:
		return "Writing specification"
	default:
		return string(s)
	}
}

// IsSpecPhaseActivity returns true if this is a spec-phase-specific activity state.
func (s ActivityState) IsSpecPhaseActivity() bool {
	return s == ActivitySpecAnalyzing || s == ActivitySpecWriting
}

// Display shows progress to user.
type Display struct {
	taskID        string
	phase         string
	iteration     int
	maxIter       int
	startTime     time.Time
	tokens        int
	quiet         bool
	activityState ActivityState
	activityStart time.Time
	mu            sync.Mutex
}

// New creates a new progress display.
func New(taskID string, quiet bool) *Display {
	return &Display{
		taskID:    taskID,
		startTime: time.Now(),
		quiet:     quiet,
	}
}

// PhaseStart announces the start of a phase.
func (d *Display) PhaseStart(phase string, maxIter int) {
	d.phase = phase
	d.maxIter = maxIter
	d.iteration = 0
	d.tokens = 0
	d.startTime = time.Now()

	if d.quiet {
		return
	}

	fmt.Printf("\n🚀 Starting phase: %s (max %d iterations)\n", phase, maxIter)
}

// Update shows progress update for current iteration.
func (d *Display) Update(iteration int, tokens int) {
	d.iteration = iteration
	d.tokens = tokens

	if d.quiet {
		return
	}

	elapsed := time.Since(d.startTime)

	// Clear line and print status
	fmt.Printf("\r⏳ %s | Phase: %s | Iteration: %d/%d | Tokens: %d | Elapsed: %s",
		d.taskID,
		d.phase,
		iteration,
		d.maxIter,
		tokens,
		formatDuration(elapsed),
	)
}

// PhaseComplete announces phase completion.
func (d *Display) PhaseComplete(phase string, commit string) {
	if d.quiet {
		return
	}

	elapsed := time.Since(d.startTime)

	// Clear the progress line first
	fmt.Print("\r" + strings.Repeat(" ", 80) + "\r")

	shortCommit := commit
	if len(shortCommit) > 7 {
		shortCommit = shortCommit[:7]
	}

	fmt.Printf("✅ Phase %s complete (commit: %s, elapsed: %s)\n",
		phase, shortCommit, formatDuration(elapsed))
}

// PhaseFailed announces phase failure.
func (d *Display) PhaseFailed(phase string, err error) {
	if d.quiet {
		return
	}

	// Clear the progress line first
	fmt.Print("\r" + strings.Repeat(" ", 80) + "\r")

	fmt.Printf("❌ Phase %s failed: %s\n", phase, err)
}

// GatePending announces waiting for a gate.
func (d *Display) GatePending(gate string, gateType string) {
	if d.quiet {
		return
	}

	icon := "⏸️"
	action := "Waiting for gate"

	switch gateType {
	case "human":
		icon = "👤"
		action = "Waiting for human approval"
	case "ai":
		icon = "🤖"
		action = "AI review in progress"
	case "auto":
		icon = "⚡"
		action = "Auto-validating"
	}

	fmt.Printf("\n%s  %s: %s\n", icon, action, gate)
}

// GateApproved announces gate approval.
func (d *Display) GateApproved(gate string) {
	if d.quiet {
		return
	}

	fmt.Printf("✅ Gate approved: %s\n", gate)
}

// GateRejected announces gate rejection.
func (d *Display) GateRejected(gate string, reason string) {
	if d.quiet {
		return
	}

	fmt.Printf("❌ Gate rejected: %s - %s\n", gate, reason)
}

// TaskComplete announces task completion.
func (d *Display) TaskComplete(totalTokens int, totalDuration time.Duration, fileStats *FileChangeStats) {
	if d.quiet {
		return
	}

	fmt.Printf("\n🎉 Task %s completed!\n", d.taskID)
	fmt.Printf("   Total tokens: %d\n", totalTokens)
	fmt.Printf("   Total time: %s\n", formatDuration(totalDuration))

	// Show file change summary if available
	if fileStats != nil && fileStats.FilesChanged > 0 {
		fmt.Printf("   Modified: %d %s (+%d/-%d)\n",
			fileStats.FilesChanged,
			pluralize(fileStats.FilesChanged, "file", "files"),
			fileStats.Additions,
			fileStats.Deletions,
		)
	}
}

// pluralize returns singular or plural form based on count.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// TaskFailed announces task failure.
// Note: Task failures are always shown, even in quiet mode, to ensure errors
// are never silently swallowed.
func (d *Display) TaskFailed(err error) {
	// Always show task failures - errors should never be silent
	fmt.Printf("\n💥 Task %s failed: %s\n", d.taskID, err)
}

// TaskBlocked announces that task phases completed but the task is blocked.
// This is shown when all phases succeed but completion actions fail (e.g., sync conflicts).
// Always shown even in quiet mode since it requires user action.
func (d *Display) TaskBlocked(totalTokens int, totalDuration time.Duration, reason string) {
	d.TaskBlockedWithContext(totalTokens, totalDuration, reason, nil)
}

// TaskBlockedWithContext announces that task phases completed but the task is blocked,
// with additional context for better guidance on manual conflict resolution.
func (d *Display) TaskBlockedWithContext(totalTokens int, totalDuration time.Duration, reason string, ctx *BlockedContext) {
	// Always show blocked status - requires user action
	fmt.Printf("\n⚠️  Task %s blocked: %s\n", d.taskID, reason)
	fmt.Printf("   All phases completed, but sync with target branch failed.\n")

	// Show enhanced guidance if context is available
	if ctx != nil && ctx.WorktreePath != "" {
		fmt.Println()
		fmt.Printf("   Worktree: %s\n", ctx.WorktreePath)

		// Show conflicted files if available
		if len(ctx.ConflictFiles) > 0 {
			fmt.Println("   Conflicted files:")
			for _, f := range ctx.ConflictFiles {
				fmt.Printf("     - %s\n", f)
			}
		}

		// Provide step-by-step resolution commands
		fmt.Println()
		fmt.Println("   To resolve manually:")
		fmt.Println("   " + strings.Repeat("─", 44))
		fmt.Printf("   cd %s\n", ctx.WorktreePath)
		fmt.Println("   git fetch origin")

		// Contextual commands based on sync strategy
		targetBranch := ctx.TargetBranch
		if targetBranch == "" {
			targetBranch = "main"
		}

		if ctx.SyncStrategy == SyncStrategyMerge {
			fmt.Printf("   git merge origin/%s\n", targetBranch)
			fmt.Println()
			fmt.Println("   # For each conflicted file:")
			fmt.Println("   #   1. Edit the file to resolve conflict markers")
			fmt.Println("   #   2. git add <file>")
			fmt.Println()
			fmt.Println("   git commit -m \"Resolve merge conflicts\"")
		} else {
			// Default to rebase instructions (most common)
			fmt.Printf("   git rebase origin/%s\n", targetBranch)
			fmt.Println()
			fmt.Println("   # For each conflicted file:")
			fmt.Println("   #   1. Edit the file to resolve conflict markers")
			fmt.Println("   #   2. git add <file>")
			fmt.Println()
			fmt.Println("   git rebase --continue")
		}
		fmt.Println("   " + strings.Repeat("─", 44))

		// Show verification command
		fmt.Println()
		fmt.Println("   Verify resolution:")
		fmt.Println("     git diff --name-only --diff-filter=U  # Should show no files")
		fmt.Println()
	}

	fmt.Printf("   Then resume:\n")
	fmt.Printf("     orc resume %s\n", d.taskID)
	fmt.Println()
	fmt.Printf("   Total tokens: %d\n", totalTokens)
	fmt.Printf("   Total time: %s\n", formatDuration(totalDuration))
}

// TaskInterrupted announces task interruption.
func (d *Display) TaskInterrupted() {
	if d.quiet {
		return
	}

	fmt.Printf("\n⚠️  Task %s interrupted. Resume with: orc resume %s\n", d.taskID, d.taskID)
}

// Info prints an informational message.
func (d *Display) Info(msg string) {
	if d.quiet {
		return
	}

	fmt.Printf("ℹ️  %s\n", msg)
}

// Warning prints a warning message.
func (d *Display) Warning(msg string) {
	if d.quiet {
		return
	}

	fmt.Printf("⚠️  %s\n", msg)
}

// Error prints an error message.
func (d *Display) Error(msg string) {
	// Errors are always shown even in quiet mode
	fmt.Printf("❌ %s\n", msg)
}

// SetActivity updates the current activity state.
func (d *Display) SetActivity(state ActivityState) {
	d.mu.Lock()
	d.activityState = state
	d.activityStart = time.Now()
	d.mu.Unlock()

	if d.quiet {
		return
	}

	// Show activity change on a new line for important state changes
	switch state {
	case ActivityWaitingAPI:
		fmt.Printf("\n⏳ Waiting for Claude API...")
	case ActivityRunningTool:
		fmt.Printf("\n🔧 Running tool...")
	case ActivitySpecAnalyzing:
		fmt.Printf("\n🔍 Analyzing codebase...")
	case ActivitySpecWriting:
		fmt.Printf("\n📝 Writing specification...")
	}
}

// Heartbeat prints a progress dot to indicate activity.
func (d *Display) Heartbeat() {
	if d.quiet {
		return
	}

	d.mu.Lock()
	state := d.activityState
	elapsed := time.Since(d.activityStart)
	d.mu.Unlock()

	// Show heartbeat dots for API wait states and spec phase activities
	showHeartbeat := state == ActivityWaitingAPI ||
		state == ActivityStreaming ||
		state == ActivitySpecAnalyzing ||
		state == ActivitySpecWriting

	if showHeartbeat {
		fmt.Printf(".")
		// After 5 dots (2.5 min with 30s interval), add elapsed time
		if elapsed > 2*time.Minute {
			fmt.Printf(" (%s)", formatDuration(elapsed))
		}
	}
}

// IdleWarning prints a warning about idle state.
func (d *Display) IdleWarning(idleDuration time.Duration) {
	// Idle warnings are always shown - they indicate potential issues
	fmt.Printf("\n⚠️  No activity for %s - API may be slow or stuck\n", formatDuration(idleDuration))
}

// TurnTimeout prints a timeout warning.
func (d *Display) TurnTimeout(turnDuration time.Duration) {
	// Timeouts are always shown
	fmt.Printf("\n⏰ Turn timeout after %s - cancelling request\n", formatDuration(turnDuration))
}

// ActivityUpdate shows current activity with elapsed time.
func (d *Display) ActivityUpdate() {
	if d.quiet {
		return
	}

	d.mu.Lock()
	state := d.activityState
	elapsed := time.Since(d.activityStart)
	iteration := d.iteration
	maxIter := d.maxIter
	phase := d.phase
	d.mu.Unlock()

	// Clear line and print status with activity
	fmt.Printf("\r⏳ %s | Phase: %s | Iteration: %d/%d | %s (%s)   ",
		d.taskID,
		phase,
		iteration,
		maxIter,
		state.String(),
		formatDuration(elapsed),
	)
}

// Cancelled announces that a request was cancelled.
func (d *Display) Cancelled() {
	// Cancellations are always shown
	fmt.Printf("\n🛑 Request cancelled\n")
}

// formatDuration formats a duration for display.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}
	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}
