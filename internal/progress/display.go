// Package progress provides progress display for orc task execution.
package progress

import (
	"fmt"
	"strings"
	"time"
)

// Display shows progress to user.
type Display struct {
	taskID    string
	phase     string
	iteration int
	maxIter   int
	startTime time.Time
	tokens    int
	quiet     bool
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
func (d *Display) TaskFailed(err error) {
	if d.quiet {
		return
	}

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
