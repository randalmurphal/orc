package controlplane

import (
	"strconv"
	"strings"
)

const (
	// MaxRecommendationSummaryBytes limits recommendation prompt context.
	MaxRecommendationSummaryBytes = 8 << 10

	// MaxAttentionSummaryBytes limits attention prompt context.
	MaxAttentionSummaryBytes = 4 << 10

	// MaxHandoffPackBytes limits handoff prompt context.
	MaxHandoffPackBytes = 16 << 10
)

func truncateWithOmission(header string, items []string, maxBytes int) string {
	if len(items) == 0 || maxBytes <= 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString(header)

	included := 0
	for index, item := range items {
		omitted := len(items) - index - 1
		candidate := builder.String() + item
		if omitted > 0 {
			candidate += omissionLine(omitted)
		}
		if len([]byte(candidate)) > maxBytes {
			break
		}

		builder.WriteString(item)
		included++
	}

	omitted := len(items) - included
	if omitted > 0 {
		notice := omissionLine(omitted)
		if len([]byte(builder.String()+notice)) <= maxBytes {
			builder.WriteString(notice)
		}
	}

	result := strings.TrimSpace(builder.String())
	if result == strings.TrimSpace(header) {
		return ""
	}
	return result
}

func omissionLine(omitted int) string {
	return "\n... and " + strconv.Itoa(omitted) + " more items"
}
