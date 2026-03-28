package api

import (
	"sort"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

const recentCompletionsLimit = 10

func appendRecentCompletionBounded(
	recentCompletions []*orcv1.RecentCompletion,
	completion *orcv1.RecentCompletion,
) []*orcv1.RecentCompletion {
	recentCompletions = append(recentCompletions, completion)
	sort.Slice(recentCompletions, func(i, j int) bool {
		return recentCompletions[i].CompletedAt.AsTime().After(recentCompletions[j].CompletedAt.AsTime())
	})
	if len(recentCompletions) > recentCompletionsLimit {
		return recentCompletions[:recentCompletionsLimit]
	}
	return recentCompletions
}
