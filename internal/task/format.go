package task

import (
	"fmt"
	"strings"
	"time"
)

// FormatDuration converts a time.Duration to a human-readable string.
// Examples: "2h 15m", "45s", "0s" for zero, "1d 3h" for long durations.
// Negative durations are formatted with a leading "-".
func FormatDuration(d time.Duration) string {
	// Handle negative durations
	if d < 0 {
		return "-" + FormatDuration(-d)
	}

	// Round down to seconds (sub-second durations become 0s)
	totalSeconds := int64(d.Seconds())

	// Zero or sub-second duration
	if totalSeconds == 0 {
		return "0s"
	}

	// Extract components
	days := totalSeconds / 86400
	totalSeconds -= days * 86400

	hours := totalSeconds / 3600
	totalSeconds -= hours * 3600

	minutes := totalSeconds / 60
	seconds := totalSeconds - minutes*60

	// Build output string
	var parts []string

	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}

	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}

	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}

	// Only include seconds if no hours component
	if seconds > 0 && hours == 0 && days == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}

	// Join with spaces
	if len(parts) == 0 {
		return "0s"
	}

	return strings.Join(parts, " ")
}
