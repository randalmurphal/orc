package brief

import "sort"

// EstimateTokens approximates the token count for a string.
// Uses ~4 characters per token as a rough estimate.
func EstimateTokens(s string) int {
	if len(s) == 0 {
		return 0
	}
	return len(s) / 4
}

// ApplyTokenBudget truncates a section to fit within a token budget.
// Removes lowest-impact entries first to stay within budget.
func ApplyTokenBudget(section Section, budget int) Section {
	if budget <= 0 {
		return Section{Category: section.Category}
	}

	// Sort entries by impact descending (high impact first)
	sorted := make([]Entry, len(section.Entries))
	copy(sorted, section.Entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Impact > sorted[j].Impact
	})

	var kept []Entry
	totalTokens := 0
	for _, e := range sorted {
		tokens := EstimateTokens(e.Content)
		if totalTokens+tokens > budget {
			break
		}
		kept = append(kept, e)
		totalTokens += tokens
	}

	return Section{
		Category: section.Category,
		Entries:  kept,
	}
}

// ApplyTotalBudget enforces a global token budget across all sections.
// Removes lowest-impact entries from the largest sections first.
func ApplyTotalBudget(sections []Section, maxTokens int) []Section {
	// Calculate current total
	total := 0
	for _, s := range sections {
		for _, e := range s.Entries {
			total += EstimateTokens(e.Content)
		}
	}

	if total <= maxTokens {
		return sections
	}

	// Collect all entries with section index for tracking
	type indexedEntry struct {
		sectionIdx int
		entryIdx   int
		entry      Entry
		tokens     int
	}

	var all []indexedEntry
	for si, s := range sections {
		for ei, e := range s.Entries {
			all = append(all, indexedEntry{
				sectionIdx: si,
				entryIdx:   ei,
				entry:      e,
				tokens:     EstimateTokens(e.Content),
			})
		}
	}

	// Sort by impact ascending (lowest impact first = first to remove)
	sort.Slice(all, func(i, j int) bool {
		return all[i].entry.Impact < all[j].entry.Impact
	})

	// Remove lowest-impact entries until within budget
	removed := make(map[[2]int]bool)
	for _, ie := range all {
		if total <= maxTokens {
			break
		}
		removed[[2]int{ie.sectionIdx, ie.entryIdx}] = true
		total -= ie.tokens
	}

	// Rebuild sections without removed entries
	result := make([]Section, len(sections))
	for si, s := range sections {
		result[si] = Section{Category: s.Category}
		for ei, e := range s.Entries {
			if !removed[[2]int{si, ei}] {
				result[si].Entries = append(result[si].Entries, e)
			}
		}
	}
	return result
}
