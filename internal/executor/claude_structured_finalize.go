package executor

import (
	"context"
	"fmt"
	"strings"
)

const claudeStructuredFinalizeAttempts = 3

func shouldUseClaudeStructuredFinalize(cfg PhaseExecutionConfig, adapter ProviderAdapter) bool {
	if adapter == nil || adapter.Name() != "claude" {
		return false
	}

	switch cfg.PhaseID {
	case "review", "review_cross":
		return true
	default:
		return false
	}
}

func executeClaudeStructuredFinalize(
	ctx context.Context,
	turnExec TurnExecutor,
	cfg PhaseExecutionConfig,
	initialPrompt string,
) (*TurnResult, []*TurnResult, error) {
	analysisTurn, err := turnExec.ExecuteTurnWithoutSchema(ctx, initialPrompt)
	if err != nil {
		if analysisTurn != nil {
			return nil, []*TurnResult{analysisTurn}, err
		}
		return nil, nil, err
	}

	turns := []*TurnResult{analysisTurn}
	if analysisTurn != nil && analysisTurn.SessionID != "" {
		turnExec.UpdateSessionID(analysisTurn.SessionID)
	}
	finalizePrompt := buildClaudeStructuredFinalizePrompt(cfg.PhaseID, cfg.ReviewRound, "")

	for attempt := 1; attempt <= claudeStructuredFinalizeAttempts; attempt++ {
		finalTurn, finalizeErr := turnExec.ExecuteTurn(ctx, finalizePrompt)
		if finalTurn != nil {
			turns = append(turns, finalTurn)
		}
		if finalizeErr == nil {
			return finalTurn, turns, nil
		}
		if !isClaudeStructuredFinalizeRetryableError(finalizeErr) || attempt == claudeStructuredFinalizeAttempts {
			return finalTurn, turns, finalizeErr
		}

		finalizePrompt = buildClaudeStructuredFinalizePrompt(cfg.PhaseID, cfg.ReviewRound, finalizeErr.Error())
	}

	return nil, turns, fmt.Errorf("claude structured finalize exhausted retries")
}

func buildClaudeStructuredFinalizePrompt(phaseID string, reviewRound int, priorErr string) string {
	var b strings.Builder

	b.WriteString("The review work in this session is already complete.\n")
	b.WriteString("Return the final structured review result now using the active JSON schema.\n")
	b.WriteString("Do not continue reviewing. Do not spawn subagents. Do not write prose outside the structured response.\n")

	if priorErr != "" {
		b.WriteString("\nThe previous finalize attempt did not produce a valid structured response.\n")
		b.WriteString("Fix that now. Return only the structured result.\n")
		b.WriteString("Previous finalize error:\n")
		b.WriteString(priorErr)
		b.WriteString("\n")
	}

	if phaseID == "review" && reviewRound == 2 {
		b.WriteString("\nRequired fields for this final response:\n")
		b.WriteString("- status: pass | fail | needs_user_input\n")
		b.WriteString("- gaps_addressed: boolean\n")
		b.WriteString("- summary: short decision summary\n")
		b.WriteString("- recommendation: what should happen next\n")
		b.WriteString("- issues_resolved, remaining_issues, user_questions when applicable\n")
		return b.String()
	}

	b.WriteString("\nRequired fields for this final response:\n")
	b.WriteString("- needs_changes: boolean\n")
	b.WriteString("- round: 1\n")
	b.WriteString("- summary: short findings summary\n")
	b.WriteString("- issues: [] when nothing blocks, otherwise concrete findings\n")
	b.WriteString("- include file and line on blocking issues when known\n")
	b.WriteString("- questions and positives only when useful\n")

	return b.String()
}

func isClaudeStructuredFinalizeRetryableError(err error) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	return strings.Contains(msg, "no structured output received") ||
		strings.Contains(msg, "structured_output is empty") ||
		strings.Contains(msg, "phase completion JSON parse failed")
}
