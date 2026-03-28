package controlplane

import (
	"fmt"
	"sort"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	taskproto "github.com/randalmurphal/orc/internal/task"
)

type HandoffSourceKind string

const (
	HandoffSourceTask           HandoffSourceKind = "task"
	HandoffSourceThread         HandoffSourceKind = "thread"
	HandoffSourceRecommendation HandoffSourceKind = "recommendation"
	HandoffSourceAttentionItem  HandoffSourceKind = "attention_item"
)

type HandoffTargetKind string

const (
	HandoffTargetClaudeCode HandoffTargetKind = "claude_code"
	HandoffTargetCodex      HandoffTargetKind = "codex"
)

func BuildTaskHandoffPack(
	currentTask *orcv1.Task,
	phaseID string,
	recommendations []*orcv1.Recommendation,
) HandoffPack {
	if currentTask == nil {
		return HandoffPack{}
	}

	currentPhase := strings.TrimSpace(phaseID)
	if currentPhase == "" {
		currentPhase = taskproto.GetCurrentPhaseProto(currentTask)
	}

	return HandoffPack{
		TaskID:       currentTask.GetId(),
		TaskTitle:    currentTask.GetTitle(),
		CurrentPhase: currentPhase,
		Summary:      strings.TrimSpace(taskproto.GetDescriptionProto(currentTask)),
		NextSteps:    taskHandoffNextSteps(currentTask.GetId(), recommendations),
		Risks:        taskHandoffRisks(currentTask.GetId(), recommendations),
	}
}

func BuildTaskContextPack(
	currentTask *orcv1.Task,
	phaseID string,
	recommendations []*orcv1.Recommendation,
) string {
	return FormatHandoffPack(BuildTaskHandoffPack(currentTask, phaseID, recommendations))
}

func BuildRecommendationContextPack(rec *orcv1.Recommendation) string {
	if rec == nil {
		return ""
	}

	sections := []string{
		fmt.Sprintf("\nRecommendation: %s", rec.GetId()),
		fmt.Sprintf("\nKind: %s", recommendationKindName(rec.GetKind())),
		fmt.Sprintf("\nTitle: %s", compactHandoffText(rec.GetTitle(), 240)),
		fmt.Sprintf("\nSummary: %s", compactHandoffText(rec.GetSummary(), 320)),
		fmt.Sprintf("\nProposed action: %s", compactHandoffText(rec.GetProposedAction(), 320)),
		fmt.Sprintf("\nEvidence: %s", compactHandoffText(rec.GetEvidence(), 320)),
		fmt.Sprintf("\nSource task: %s", rec.GetSourceTaskId()),
		fmt.Sprintf("\nSource run: %s", rec.GetSourceRunId()),
	}
	if rec.GetSourceThreadId() != "" {
		sections = append(sections, fmt.Sprintf("\nSource thread: %s", rec.GetSourceThreadId()))
	}
	if rec.GetPromotedToType() != "" || rec.GetPromotedToId() != "" {
		sections = append(sections, fmt.Sprintf("\nPromoted to: %s %s", rec.GetPromotedToType(), rec.GetPromotedToId()))
	}

	return truncateWithOmission("## Recommendation Context\n", sections, MaxHandoffPackBytes)
}

func BuildThreadContextPack(thread *orcv1.Thread) string {
	if thread == nil {
		return ""
	}

	sections := []string{
		fmt.Sprintf("\nThread: %s %s", thread.GetId(), compactHandoffText(thread.GetTitle(), 200)),
		fmt.Sprintf("\nStatus: %s", thread.GetStatus()),
	}
	if thread.GetTaskId() != "" {
		sections = append(sections, fmt.Sprintf("\nTask: %s", thread.GetTaskId()))
	}
	if thread.GetInitiativeId() != "" {
		sections = append(sections, fmt.Sprintf("\nInitiative: %s", thread.GetInitiativeId()))
	}
	if thread.GetFileContext() != "" {
		sections = append(sections, fmt.Sprintf("\nFile context: %s", compactHandoffText(thread.GetFileContext(), 240)))
	}

	for _, link := range thread.GetLinks() {
		var section strings.Builder
		fmt.Fprintf(&section, "\nLink [%s]: %s", link.GetLinkType(), compactHandoffText(link.GetTitle(), 200))
		if link.GetTargetId() != "" {
			fmt.Fprintf(&section, "\n  Target: %s", compactHandoffText(link.GetTargetId(), 240))
		}
		sections = append(sections, section.String())
	}

	for _, message := range recentThreadMessages(thread.GetMessages(), 6) {
		sections = append(sections, fmt.Sprintf("\nMessage [%s]: %s", message.GetRole(), compactHandoffText(message.GetContent(), 320)))
	}

	for _, draft := range thread.GetRecommendationDrafts() {
		var section strings.Builder
		fmt.Fprintf(&section, "\nDraft [recommendation]: %s", compactHandoffText(draft.GetTitle(), 200))
		if draft.GetSummary() != "" {
			fmt.Fprintf(&section, "\n  Summary: %s", compactHandoffText(draft.GetSummary(), 240))
		}
		if draft.GetProposedAction() != "" {
			fmt.Fprintf(&section, "\n  Proposed action: %s", compactHandoffText(draft.GetProposedAction(), 240))
		}
		sections = append(sections, section.String())
	}

	for _, draft := range thread.GetDecisionDrafts() {
		var section strings.Builder
		fmt.Fprintf(&section, "\nDraft [decision]: %s", compactHandoffText(draft.GetDecision(), 200))
		if draft.GetRationale() != "" {
			fmt.Fprintf(&section, "\n  Rationale: %s", compactHandoffText(draft.GetRationale(), 240))
		}
		sections = append(sections, section.String())
	}

	return truncateWithOmission("## Thread Context\n", sections, MaxHandoffPackBytes)
}

func BuildAttentionItemContextPack(item *orcv1.AttentionItem) string {
	if item == nil {
		return ""
	}

	sections := []string{
		fmt.Sprintf("\nAttention item: %s", compactHandoffText(item.GetId(), 200)),
		fmt.Sprintf("\nType: %s", attentionItemTypeName(item.GetType())),
		fmt.Sprintf("\nTitle: %s", compactHandoffText(item.GetTitle(), 240)),
		fmt.Sprintf("\nSummary: %s", compactHandoffText(attentionItemSummary(item), 320)),
	}
	if item.GetTaskId() != "" {
		sections = append(sections, fmt.Sprintf("\nTask: %s", item.GetTaskId()))
	}
	if item.GetSignalKind() != "" {
		sections = append(sections, fmt.Sprintf("\nSignal kind: %s", item.GetSignalKind()))
	}
	if item.GetReferenceType() != "" || item.GetReferenceId() != "" {
		sections = append(sections, fmt.Sprintf("\nReference: %s %s", item.GetReferenceType(), item.GetReferenceId()))
	}
	for _, action := range item.GetAvailableActions() {
		if action == orcv1.AttentionAction_ATTENTION_ACTION_UNSPECIFIED {
			continue
		}
		sections = append(sections, fmt.Sprintf("\nAvailable action: %s", attentionActionName(action)))
	}

	return truncateWithOmission("## Attention Item Context\n", sections, MaxHandoffPackBytes)
}

func BuildBootstrapPrompt(sourceType HandoffSourceKind, contextPack string) (string, error) {
	trimmedPack := strings.TrimSpace(contextPack)
	if trimmedPack == "" {
		return "", fmt.Errorf("context pack is required")
	}

	var instruction string
	switch sourceType {
	case HandoffSourceTask:
		instruction = "Continue the task from this handoff. Start with the current phase, next steps, and risks, then proceed with the work."
	case HandoffSourceThread:
		instruction = "Continue the thread from this handoff. Pick up the latest discussion points and respond or act on them."
	case HandoffSourceRecommendation:
		instruction = "Continue the recommendation follow-up from this handoff. Assess it, decide on the next action, and carry that forward."
	case HandoffSourceAttentionItem:
		instruction = "Continue the attention item follow-up from this handoff. Resolve the issue or make the next required operator move."
	default:
		return "", fmt.Errorf("unsupported handoff source type %q", sourceType)
	}

	return "<context>\n" + trimmedPack + "\n</context>\n\n" + instruction, nil
}

func BuildCLICommand(target HandoffTargetKind, prompt string) (string, error) {
	trimmedPrompt := strings.TrimSpace(prompt)
	if trimmedPrompt == "" {
		return "", fmt.Errorf("bootstrap prompt is required")
	}

	switch target {
	case HandoffTargetClaudeCode:
		return "claude -p " + shellQuote(trimmedPrompt), nil
	case HandoffTargetCodex:
		return "codex " + shellQuote(trimmedPrompt), nil
	default:
		return "", fmt.Errorf("unsupported handoff target %q", target)
	}
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func taskHandoffNextSteps(taskID string, recommendations []*orcv1.Recommendation) []string {
	steps := make([]string, 0)
	seen := make(map[string]struct{})

	for _, recommendation := range recommendations {
		if recommendation.GetSourceTaskId() != taskID {
			continue
		}
		if recommendation.GetStatus() != orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING {
			continue
		}
		if recommendation.GetKind() == orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK {
			continue
		}

		step := strings.TrimSpace(recommendation.GetProposedAction())
		if step == "" {
			step = strings.TrimSpace(recommendation.GetSummary())
		}
		if step == "" {
			step = strings.TrimSpace(recommendation.GetTitle())
		}
		if step == "" {
			continue
		}
		if _, exists := seen[step]; exists {
			continue
		}

		seen[step] = struct{}{}
		steps = append(steps, step)
	}

	sort.Strings(steps)
	return steps
}

func taskHandoffRisks(taskID string, recommendations []*orcv1.Recommendation) []string {
	risks := make([]string, 0)
	seen := make(map[string]struct{})

	for _, recommendation := range recommendations {
		if recommendation.GetSourceTaskId() != taskID {
			continue
		}
		if recommendation.GetStatus() != orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING {
			continue
		}
		if recommendation.GetKind() != orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK {
			continue
		}

		risk := strings.TrimSpace(recommendation.GetTitle())
		summary := strings.TrimSpace(recommendation.GetSummary())
		if risk == "" {
			risk = summary
		} else if summary != "" {
			risk = risk + ": " + summary
		}
		if risk == "" {
			continue
		}
		if _, exists := seen[risk]; exists {
			continue
		}

		seen[risk] = struct{}{}
		risks = append(risks, risk)
	}

	sort.Strings(risks)
	return risks
}

func compactHandoffText(value string, maxRunes int) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if trimmed == "" {
		return ""
	}
	if maxRunes <= 0 {
		return trimmed
	}

	runes := []rune(trimmed)
	if len(runes) <= maxRunes {
		return trimmed
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func recentThreadMessages(messages []*orcv1.ThreadMessage, limit int) []*orcv1.ThreadMessage {
	if len(messages) <= limit {
		return messages
	}
	return messages[len(messages)-limit:]
}

func recommendationKindName(kind orcv1.RecommendationKind) string {
	return strings.TrimPrefix(strings.ToLower(kind.String()), "recommendation_kind_")
}

func attentionItemTypeName(itemType orcv1.AttentionItemType) string {
	return strings.TrimPrefix(strings.ToLower(itemType.String()), "attention_item_type_")
}

func attentionActionName(action orcv1.AttentionAction) string {
	return strings.TrimPrefix(strings.ToLower(action.String()), "attention_action_")
}

func attentionItemSummary(item *orcv1.AttentionItem) string {
	if item == nil {
		return ""
	}
	if item.GetDescription() != "" {
		return item.GetDescription()
	}
	if item.GetBlockedReason() != "" {
		return item.GetBlockedReason()
	}
	if item.GetGateQuestion() != "" {
		return item.GetGateQuestion()
	}
	if item.GetErrorMessage() != "" {
		return item.GetErrorMessage()
	}
	return "Needs operator attention."
}
