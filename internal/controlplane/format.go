package controlplane

import (
	"fmt"
	"strings"
)

// FormatRecommendationSummary renders recommendation candidates as markdown for
// prompt injection.
func FormatRecommendationSummary(items []RecommendationCandidate) string {
	if len(items) == 0 {
		return ""
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		var section strings.Builder
		fmt.Fprintf(&section, "\n- [%s] %s", item.Kind, item.Title)
		if item.Summary != "" {
			fmt.Fprintf(&section, "\n  Summary: %s", item.Summary)
		}
		if item.ProposedAction != "" {
			fmt.Fprintf(&section, "\n  Proposed action: %s", item.ProposedAction)
		}
		if item.Evidence != "" {
			fmt.Fprintf(&section, "\n  Evidence: %s", item.Evidence)
		}
		if item.DedupeKey != "" {
			fmt.Fprintf(&section, "\n  Dedupe key: %s", item.DedupeKey)
		}
		lines = append(lines, section.String())
	}

	return truncateWithOmission("## Pending Recommendations\n", lines, MaxRecommendationSummaryBytes)
}

// FormatAttentionSummary renders attention signals as markdown for prompt
// injection.
func FormatAttentionSummary(items []AttentionSignal) string {
	if len(items) == 0 {
		return ""
	}

	lines := make([]string, 0, len(items))
	for _, item := range items {
		var section strings.Builder
		fmt.Fprintf(&section, "\n- %s: %s [%s]", item.TaskID, item.Title, item.Status)
		if item.Phase != "" {
			fmt.Fprintf(&section, "\n  Phase: %s", item.Phase)
		}
		if item.Summary != "" {
			fmt.Fprintf(&section, "\n  Summary: %s", item.Summary)
		}
		if item.Kind != "" {
			fmt.Fprintf(&section, "\n  Kind: %s", item.Kind)
		}
		lines = append(lines, section.String())
	}

	return truncateWithOmission("## Attention Summary\n", lines, MaxAttentionSummaryBytes)
}

// FormatHandoffPack renders a handoff pack as markdown for prompt injection.
func FormatHandoffPack(pack HandoffPack) string {
	if pack.TaskID == "" &&
		pack.TaskTitle == "" &&
		pack.CurrentPhase == "" &&
		pack.Summary == "" &&
		len(pack.NextSteps) == 0 &&
		len(pack.OpenQuestions) == 0 &&
		len(pack.Risks) == 0 &&
		len(pack.Drafts) == 0 {
		return ""
	}

	sections := make([]string, 0, 4+len(pack.NextSteps)+len(pack.OpenQuestions)+len(pack.Risks)+len(pack.Drafts))
	if pack.TaskID != "" || pack.TaskTitle != "" {
		sections = append(sections, fmt.Sprintf("\nTask: %s %s", pack.TaskID, pack.TaskTitle))
	}
	if pack.CurrentPhase != "" {
		sections = append(sections, fmt.Sprintf("\nCurrent phase: %s", pack.CurrentPhase))
	}
	if pack.Summary != "" {
		sections = append(sections, fmt.Sprintf("\nSummary: %s", pack.Summary))
	}
	for _, step := range pack.NextSteps {
		sections = append(sections, "\nNext step: "+step)
	}
	for _, question := range pack.OpenQuestions {
		sections = append(sections, "\nOpen question: "+question)
	}
	for _, risk := range pack.Risks {
		sections = append(sections, "\nRisk: "+risk)
	}
	for _, draft := range pack.Drafts {
		var section strings.Builder
		fmt.Fprintf(&section, "\nDraft [%s]: %s", draft.TargetType, draft.Title)
		if draft.Summary != "" {
			fmt.Fprintf(&section, "\n  Summary: %s", draft.Summary)
		}
		if draft.Content != "" {
			fmt.Fprintf(&section, "\n  Content: %s", draft.Content)
		}
		sections = append(sections, section.String())
	}

	return truncateWithOmission("## Handoff Pack\n", sections, MaxHandoffPackBytes)
}
