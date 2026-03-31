package executor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/storage"
)

func (we *WorkflowExecutor) generateCompletionRecommendations(
	ctx context.Context,
	run *db.WorkflowRun,
	t *orcv1.Task,
) error {
	if t == nil || run == nil || run.TaskID == nil {
		return nil
	}

	generator := we.completionRecommendationGenerator
	if generator == nil {
		generator = we.buildAndPersistCompletionRecommendations
	}

	result, err := generator(ctx, run, t)
	if result != nil {
		we.logger.Info("completion recommendations processed",
			"task_id", t.Id,
			"run_id", run.ID,
			"generated", result.Generated,
			"filtered", result.Filtered,
			"persisted", len(result.Persisted),
			"dedupe_suppressed", result.DedupeSuppressed,
		)
	}

	return err
}

func (we *WorkflowExecutor) buildAndPersistCompletionRecommendations(
	_ context.Context,
	run *db.WorkflowRun,
	t *orcv1.Task,
) (*CompletionRecommendationResult, error) {
	outputs, err := we.backend.GetPhaseOutputsForTask(t.Id)
	if err != nil {
		return nil, fmt.Errorf("load phase outputs for completion recommendations: %w", err)
	}

	currentRunOutputs := filterPhaseOutputsForRun(outputs, run.ID)
	candidates := buildCompletionRecommendationCandidates(t, run, currentRunOutputs)
	filteredCandidates, filteredCount := filterLowSignalCandidates(candidates)
	persisted, dedupeSuppressed, persistErr := we.persistRecommendationCandidates(run, t, filteredCandidates)

	return &CompletionRecommendationResult{
		Generated:        len(candidates),
		Filtered:         filteredCount,
		DedupeSuppressed: dedupeSuppressed,
		Persisted:        persisted,
	}, persistErr
}

func filterPhaseOutputsForRun(outputs []*storage.PhaseOutputInfo, runID string) []*storage.PhaseOutputInfo {
	filtered := make([]*storage.PhaseOutputInfo, 0, len(outputs))
	for _, output := range outputs {
		if output == nil || output.WorkflowRunID != runID {
			continue
		}
		filtered = append(filtered, output)
	}
	return filtered
}

func buildCompletionRecommendationCandidates(
	t *orcv1.Task,
	run *db.WorkflowRun,
	outputs []*storage.PhaseOutputInfo,
) []controlplane.RecommendationCandidate {
	candidates := make([]controlplane.RecommendationCandidate, 0)
	changedFiles := parseChangedFilesForRecommendations(t)

	for _, output := range outputs {
		if output == nil || strings.TrimSpace(output.Content) == "" {
			continue
		}

		switch output.PhaseTemplateID {
		case "review", "review_cross":
			candidates = append(candidates, buildReviewRecommendationCandidates(t, run, output, changedFiles)...)
		case "implement":
			candidates = append(candidates, buildImplementRecommendationCandidates(t, run, output, changedFiles)...)
		}
	}

	return candidates
}

func buildReviewRecommendationCandidates(
	t *orcv1.Task,
	run *db.WorkflowRun,
	output *storage.PhaseOutputInfo,
	changedFiles []string,
) []controlplane.RecommendationCandidate {
	payload, ok := decodeRecommendationPayload(output.Content)
	if !ok {
		return nil
	}

	if _, hasNeedsChanges := payload["needs_changes"]; hasNeedsChanges {
		findings, err := ParseReviewFindings(output.Content)
		if err != nil {
			slog.Warn("skip review recommendation candidates: parse findings failed", "phase", output.PhaseTemplateID, "error", err)
			return nil
		}
		return buildCandidatesFromReviewFindings(t, run, output.PhaseTemplateID, findings, changedFiles)
	}

	hasRecommendation := false
	if _, ok := payload["recommendation"]; ok {
		hasRecommendation = true
	}
	if _, ok := payload["remaining_issues"]; ok {
		hasRecommendation = true
	}
	if _, ok := payload["user_questions"]; ok {
		hasRecommendation = true
	}
	if !hasRecommendation {
		return nil
	}

	decision, err := ParseReviewDecision(output.Content)
	if err != nil {
		slog.Warn("skip review recommendation candidates: parse decision failed", "phase", output.PhaseTemplateID, "error", err)
		return nil
	}
	return buildCandidatesFromReviewDecision(t, run, output.PhaseTemplateID, decision, changedFiles)
}

func buildCandidatesFromReviewFindings(
	t *orcv1.Task,
	run *db.WorkflowRun,
	phaseID string,
	findings *ReviewFindings,
	changedFiles []string,
) []controlplane.RecommendationCandidate {
	if findings == nil {
		return nil
	}

	candidates := make([]controlplane.RecommendationCandidate, 0, len(findings.Issues)+len(findings.Questions)+1)
	for _, issue := range findings.Issues {
		kind := recommendationKindFromSeverity(issue.Severity)
		title := buildReviewIssueTitle(issue, kind)
		summary := strings.TrimSpace(issue.Description)
		action := firstNonEmpty(strings.TrimSpace(issue.Suggestion), "Address the review finding in the touched code path.")
		evidence := buildRecommendationEvidence(
			run,
			phaseID,
			fmt.Sprintf("Review round %d reported a %s-severity issue. %s", findings.Round, issue.Severity, firstNonEmpty(issueLocation(issue), findings.Summary)),
			changedFiles,
		)
		candidates = append(candidates, finalizeRecommendationCandidate(t.GetId(), controlplane.RecommendationCandidate{
			Kind:           kind,
			Title:          title,
			Summary:        summary,
			ProposedAction: action,
			Evidence:       evidence,
			Confidence:     confidenceFromSeverity(issue.Severity),
		}))
	}

	for _, question := range findings.Questions {
		candidates = append(candidates, finalizeRecommendationCandidate(t.GetId(), controlplane.RecommendationCandidate{
			Kind:           db.RecommendationKindDecisionRequest,
			Title:          fmt.Sprintf("Resolve review question for %s", t.GetId()),
			Summary:        strings.TrimSpace(question),
			ProposedAction: "Provide the missing decision or clarification before follow-on work continues.",
			Evidence: buildRecommendationEvidence(
				run,
				phaseID,
				fmt.Sprintf("Review raised a clarification request. %s", findings.Summary),
				changedFiles,
			),
			Confidence: "medium",
		}))
	}

	if len(candidates) == 0 && findings.NeedsChanges && strings.TrimSpace(findings.Summary) != "" {
		candidates = append(candidates, finalizeRecommendationCandidate(t.GetId(), controlplane.RecommendationCandidate{
			Kind:           db.RecommendationKindFollowUp,
			Title:          fmt.Sprintf("Follow up on review findings for %s", t.GetId()),
			Summary:        strings.TrimSpace(findings.Summary),
			ProposedAction: "Review the findings summary and close the remaining implementation gap.",
			Evidence:       buildRecommendationEvidence(run, phaseID, findings.Summary, changedFiles),
			Confidence:     "medium",
		}))
	}

	return candidates
}

func buildCandidatesFromReviewDecision(
	t *orcv1.Task,
	run *db.WorkflowRun,
	phaseID string,
	decision *ReviewDecision,
	changedFiles []string,
) []controlplane.RecommendationCandidate {
	if decision == nil {
		return nil
	}

	candidates := make([]controlplane.RecommendationCandidate, 0, len(decision.RemainingIssues)+len(decision.UserQuestions)+1)
	for _, issue := range decision.RemainingIssues {
		kind := recommendationKindFromSeverity(issue.Severity)
		candidates = append(candidates, finalizeRecommendationCandidate(t.GetId(), controlplane.RecommendationCandidate{
			Kind:           kind,
			Title:          buildReviewIssueTitle(issue, kind),
			Summary:        strings.TrimSpace(issue.Description),
			ProposedAction: firstNonEmpty(strings.TrimSpace(issue.Suggestion), decision.Recommendation, "Address the remaining review issue."),
			Evidence: buildRecommendationEvidence(
				run,
				phaseID,
				fmt.Sprintf("Review decision status=%s left a %s-severity issue unresolved. %s", decision.Status, issue.Severity, decision.Summary),
				changedFiles,
			),
			Confidence: confidenceFromSeverity(issue.Severity),
		}))
	}

	for _, question := range decision.UserQuestions {
		candidates = append(candidates, finalizeRecommendationCandidate(t.GetId(), controlplane.RecommendationCandidate{
			Kind:           db.RecommendationKindDecisionRequest,
			Title:          fmt.Sprintf("Operator decision needed for %s", t.GetId()),
			Summary:        strings.TrimSpace(question),
			ProposedAction: firstNonEmpty(strings.TrimSpace(decision.Recommendation), "Answer the review question and decide the next step."),
			Evidence:       buildRecommendationEvidence(run, phaseID, decision.Summary, changedFiles),
			Confidence:     "medium",
		}))
	}

	if len(candidates) == 0 && decision.Status == ReviewStatusNeedsUserInput && strings.TrimSpace(decision.Recommendation) != "" {
		candidates = append(candidates, finalizeRecommendationCandidate(t.GetId(), controlplane.RecommendationCandidate{
			Kind:           db.RecommendationKindDecisionRequest,
			Title:          fmt.Sprintf("Operator decision needed for %s", t.GetId()),
			Summary:        strings.TrimSpace(decision.Recommendation),
			ProposedAction: strings.TrimSpace(decision.Recommendation),
			Evidence:       buildRecommendationEvidence(run, phaseID, decision.Summary, changedFiles),
			Confidence:     "medium",
		}))
	}

	return candidates
}

func buildImplementRecommendationCandidates(
	t *orcv1.Task,
	run *db.WorkflowRun,
	output *storage.PhaseOutputInfo,
	changedFiles []string,
) []controlplane.RecommendationCandidate {
	response, err := ParseImplementResponse(output.Content)
	if err != nil {
		slog.Warn("skip implement recommendation candidates: parse response failed", "phase", output.PhaseTemplateID, "error", err)
		return nil
	}
	if response == nil {
		return nil
	}

	candidates := make([]controlplane.RecommendationCandidate, 0)
	if response.Verification != nil {
		candidates = append(candidates, buildVerificationStatusCandidates(t, run, output.PhaseTemplateID, response.Verification, changedFiles)...)
	}

	for _, issue := range response.PreExistingIssues {
		candidates = append(candidates, finalizeRecommendationCandidate(t.GetId(), controlplane.RecommendationCandidate{
			Kind:           db.RecommendationKindFollowUp,
			Title:          fmt.Sprintf("Track pre-existing issue after %s", t.GetId()),
			Summary:        strings.TrimSpace(issue),
			ProposedAction: "Keep this issue outside the completed task and address it in explicit follow-up work.",
			Evidence:       buildRecommendationEvidence(run, output.PhaseTemplateID, response.Summary, changedFiles),
			Confidence:     "medium",
		}))
	}

	return candidates
}

func buildVerificationStatusCandidates(
	t *orcv1.Task,
	run *db.WorkflowRun,
	phaseID string,
	verification *ImplementVerification,
	changedFiles []string,
) []controlplane.RecommendationCandidate {
	candidates := make([]controlplane.RecommendationCandidate, 0)
	appendStatusCandidate := func(label string, status *VerificationStatus) {
		if status == nil {
			return
		}
		normalized := strings.ToUpper(strings.TrimSpace(status.Status))
		if normalized != "FAIL" && normalized != "SKIPPED" {
			return
		}

		kind := db.RecommendationKindFollowUp
		confidence := "medium"
		if normalized == "FAIL" {
			kind = db.RecommendationKindRisk
			confidence = "high"
		}

		summary := fmt.Sprintf("%s verification finished with status %s.", label, normalized)
		if strings.TrimSpace(status.Evidence) != "" {
			summary = fmt.Sprintf("%s %s", summary, strings.TrimSpace(status.Evidence))
		}
		action := "Re-run the verification and close the gap before relying on this completed task."
		if normalized == "FAIL" {
			action = "Investigate the failing verification result and decide whether follow-up work is required."
		}

		candidates = append(candidates, finalizeRecommendationCandidate(t.GetId(), controlplane.RecommendationCandidate{
			Kind:           kind,
			Title:          fmt.Sprintf("%s verification needs follow-up", label),
			Summary:        summary,
			ProposedAction: action,
			Evidence:       buildRecommendationEvidence(run, phaseID, status.Command, changedFiles),
			Confidence:     confidence,
		}))
	}

	appendStatusCandidate("Tests", verification.Tests)
	appendStatusCandidate("Build", verification.Build)
	appendStatusCandidate("Linting", verification.Linting)

	if verification.Wiring != nil {
		appendStatusCandidate("Wiring", &VerificationStatus{
			Status:   verification.Wiring.Status,
			Evidence: verification.Wiring.Evidence,
		})
	}

	for _, criterion := range verification.SuccessCriteria {
		if strings.ToUpper(strings.TrimSpace(criterion.Status)) != "FAIL" {
			continue
		}
		candidates = append(candidates, finalizeRecommendationCandidate(t.GetId(), controlplane.RecommendationCandidate{
			Kind:           db.RecommendationKindFollowUp,
			Title:          fmt.Sprintf("Finish success criterion %s", criterion.ID),
			Summary:        firstNonEmpty(strings.TrimSpace(criterion.Evidence), fmt.Sprintf("Success criterion %s did not pass during implementation verification.", criterion.ID)),
			ProposedAction: "Create explicit follow-up work for the failed success criterion before treating the task as fully closed.",
			Evidence:       buildRecommendationEvidence(run, phaseID, criterion.ID, changedFiles),
			Confidence:     "medium",
		}))
	}

	return candidates
}

func filterLowSignalCandidates(candidates []controlplane.RecommendationCandidate) ([]controlplane.RecommendationCandidate, int) {
	filtered := make([]controlplane.RecommendationCandidate, 0, len(candidates))
	filteredCount := 0

	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Title) == "" || strings.TrimSpace(candidate.Summary) == "" {
			filteredCount++
			continue
		}

		switch strings.ToLower(strings.TrimSpace(candidate.Confidence)) {
		case "none", "low":
			filteredCount++
			continue
		}

		filtered = append(filtered, candidate)
	}

	return filtered, filteredCount
}

func (we *WorkflowExecutor) persistRecommendationCandidates(
	run *db.WorkflowRun,
	t *orcv1.Task,
	candidates []controlplane.RecommendationCandidate,
) ([]*orcv1.Recommendation, int, error) {
	persisted := make([]*orcv1.Recommendation, 0, len(candidates))
	dedupeSuppressed := 0
	var persistErr error

	for _, candidate := range candidates {
		if err := validateRecommendationCandidate(candidate); err != nil {
			persistErr = errors.Join(persistErr, fmt.Errorf("candidate %q invalid: %w", candidate.Title, err))
			continue
		}

		kind, err := recommendationKindProtoForCandidate(candidate.Kind)
		if err != nil {
			persistErr = errors.Join(persistErr, err)
			continue
		}

		rec := &orcv1.Recommendation{
			Kind:           kind,
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Title:          candidate.Title,
			Summary:        candidate.Summary,
			ProposedAction: candidate.ProposedAction,
			Evidence:       candidate.Evidence,
			SourceTaskId:   t.GetId(),
			SourceRunId:    run.ID,
			DedupeKey:      candidate.DedupeKey,
		}

		if err := we.backend.SaveRecommendation(rec); err != nil {
			if isRecommendationDedupeError(err) {
				dedupeSuppressed++
				continue
			}
			persistErr = errors.Join(persistErr, fmt.Errorf("save recommendation %q: %w", candidate.Title, err))
			continue
		}

		persisted = append(persisted, rec)
		we.publishRecommendationCreated(rec)
	}

	return persisted, dedupeSuppressed, persistErr
}

func validateRecommendationCandidate(candidate controlplane.RecommendationCandidate) error {
	switch candidate.Kind {
	case db.RecommendationKindCleanup, db.RecommendationKindRisk, db.RecommendationKindFollowUp, db.RecommendationKindDecisionRequest:
	default:
		return fmt.Errorf("invalid recommendation kind %q", candidate.Kind)
	}

	if strings.TrimSpace(candidate.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(candidate.Summary) == "" {
		return fmt.Errorf("summary is required")
	}
	if strings.TrimSpace(candidate.ProposedAction) == "" {
		return fmt.Errorf("proposed_action is required")
	}
	if strings.TrimSpace(candidate.Evidence) == "" {
		return fmt.Errorf("evidence is required")
	}
	if strings.TrimSpace(candidate.DedupeKey) == "" {
		return fmt.Errorf("dedupe_key is required")
	}

	return nil
}

func recommendationKindProtoForCandidate(kind string) (orcv1.RecommendationKind, error) {
	switch kind {
	case db.RecommendationKindCleanup:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP, nil
	case db.RecommendationKindRisk:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK, nil
	case db.RecommendationKindFollowUp:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP, nil
	case db.RecommendationKindDecisionRequest:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_DECISION_REQUEST, nil
	default:
		return orcv1.RecommendationKind_RECOMMENDATION_KIND_UNSPECIFIED, fmt.Errorf("invalid recommendation kind %q", kind)
	}
}

func finalizeRecommendationCandidate(taskID string, candidate controlplane.RecommendationCandidate) controlplane.RecommendationCandidate {
	candidate.Kind = strings.TrimSpace(candidate.Kind)
	candidate.Title = strings.TrimSpace(candidate.Title)
	candidate.Summary = strings.TrimSpace(candidate.Summary)
	candidate.ProposedAction = strings.TrimSpace(candidate.ProposedAction)
	candidate.Evidence = strings.TrimSpace(candidate.Evidence)
	candidate.Confidence = strings.ToLower(strings.TrimSpace(candidate.Confidence))
	if candidate.DedupeKey == "" {
		candidate.DedupeKey = completionRecommendationDedupeKey(taskID, candidate)
	}
	return candidate
}

func completionRecommendationDedupeKey(taskID string, candidate controlplane.RecommendationCandidate) string {
	payload := strings.Join([]string{
		candidate.Kind,
		normalizeRecommendationText(candidate.Title),
		normalizeRecommendationText(candidate.Summary),
		normalizeRecommendationText(candidate.ProposedAction),
	}, "\n")
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("task:%s:%s:%s", taskID, candidate.Kind, hex.EncodeToString(sum[:])[:16])
}
