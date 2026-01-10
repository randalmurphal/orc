package executor

import (
	"github.com/randalmurphal/llmkit/parser"
)

// PhaseMarkers provides marker detection for phase completion signals.
// Uses llmkit/parser.MarkerMatcher for extraction.
var PhaseMarkers = parser.NewMarkerMatcher("phase_complete", "phase_blocked")

// IsPhaseComplete checks if the response contains a phase_complete marker
// with value "true".
func IsPhaseComplete(content string) bool {
	return parser.IsPhaseComplete(content)
}

// IsPhaseBlocked checks if the response contains a phase_blocked marker.
func IsPhaseBlocked(content string) bool {
	return parser.IsPhaseBlocked(content)
}

// GetBlockedReason returns the reason from a phase_blocked marker, if present.
func GetBlockedReason(content string) string {
	return parser.GetBlockedReason(content)
}

// PhaseCompletionStatus represents the outcome of checking phase completion.
type PhaseCompletionStatus int

const (
	// PhaseStatusContinue indicates the phase should continue iterating.
	PhaseStatusContinue PhaseCompletionStatus = iota

	// PhaseStatusComplete indicates the phase completed successfully.
	PhaseStatusComplete

	// PhaseStatusBlocked indicates the phase is blocked and needs intervention.
	PhaseStatusBlocked
)

// CheckPhaseCompletion analyzes content for completion markers.
// Returns the status and an optional reason (for blocked status).
func CheckPhaseCompletion(content string) (PhaseCompletionStatus, string) {
	if IsPhaseComplete(content) {
		return PhaseStatusComplete, ""
	}

	if IsPhaseBlocked(content) {
		return PhaseStatusBlocked, GetBlockedReason(content)
	}

	return PhaseStatusContinue, ""
}
