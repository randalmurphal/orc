// Package lock provides same-user execution protection via PID guard.
//
// This package implements a simple protection mechanism to prevent the same
// user from accidentally running the same task twice. It does NOT provide
// cross-user locking - multiple users CAN run the same task simultaneously,
// each with their own worktree and branch.
//
// Design Philosophy:
// - No cross-user locking (worktree isolation handles conflicts)
// - PID guard prevents accidental double-runs by same user
// - Simple, lightweight, no heartbeats or TTL required
package lock

// Mode represents the coordination mode.
type Mode string

const (
	// ModeSolo is the default mode with no prefix (single user).
	ModeSolo Mode = "solo"
	// ModeP2P uses prefixed IDs for multi-user coordination.
	ModeP2P Mode = "p2p"
	// ModeTeam uses server-based coordination with prefixed IDs.
	ModeTeam Mode = "team"
)
