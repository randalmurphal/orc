// Package progress provides progress display for orc task execution.
package progress

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

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
	default:
		return string(s)
	}
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
func (d *Display) TaskComplete(totalTokens int, totalDuration time.Duration) {
	if d.quiet {
		return
	}

	fmt.Printf("\n🎉 Task %s completed!\n", d.taskID)
	fmt.Printf("   Total tokens: %d\n", totalTokens)
	fmt.Printf("   Total time: %s\n", formatDuration(totalDuration))
}

// TaskFailed announces task failure.
// Note: Task failures are always shown, even in quiet mode, to ensure errors
// are never silently swallowed.
func (d *Display) TaskFailed(err error) {
	// Always show task failures - errors should never be silent
	fmt.Printf("\n💥 Task %s failed: %s\n", d.taskID, err)
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

	// Only show heartbeat dots when waiting for API
	if state == ActivityWaitingAPI || state == ActivityStreaming {
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
