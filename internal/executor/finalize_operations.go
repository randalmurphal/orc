package executor

import (
	"fmt"
	"strconv"
	"strings"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
)

// assessRisk performs risk assessment for the changes.
func (e *FinalizeExecutor) assessRisk(result *FinalizeResult, targetBranch string, cfg config.FinalizeConfig) error {
	if !cfg.RiskAssessment.Enabled {
		result.RiskLevel = "unknown"
		return nil
	}

	if e.gitSvc == nil {
		return fmt.Errorf("git service not available")
	}

	target := "origin/" + targetBranch
	diffStat, err := e.gitSvc.Context().RunGit("diff", "--stat", target+"...HEAD")
	if err != nil {
		return fmt.Errorf("get diff stat: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(diffStat), "\n")
	if len(lines) > 0 {
		result.FilesChanged = parseFileCount(lines[len(lines)-1])
	}

	numstat, err := e.gitSvc.Context().RunGit("diff", "--numstat", target+"...HEAD")
	if err == nil {
		result.LinesChanged = parseTotalLines(numstat)
	}

	result.RiskLevel = classifyRisk(result.FilesChanged, result.LinesChanged, result.ConflictsResolved)

	threshold := cfg.RiskAssessment.ReReviewThreshold
	if threshold == "" {
		threshold = "high"
	}
	result.NeedsReview = shouldTriggerReview(result.RiskLevel, threshold)

	e.logger.Info("risk assessment complete",
		"risk_level", result.RiskLevel,
		"files_changed", result.FilesChanged,
		"lines_changed", result.LinesChanged,
		"needs_review", result.NeedsReview,
	)
	return nil
}

// createFinalizeCommit creates a commit documenting the finalization.
func (e *FinalizeExecutor) createFinalizeCommit(t *orcv1.Task, result *FinalizeResult) (string, error) {
	if e.gitSvc == nil {
		return "", fmt.Errorf("git service not available")
	}

	clean, err := e.gitSvc.IsClean()
	if err != nil {
		return "", fmt.Errorf("check clean: %w", err)
	}

	if clean {
		return e.gitSvc.Context().HeadCommit()
	}

	msg := fmt.Sprintf("[orc] %s: finalize - completed\n\nPhase: finalize\nStatus: completed\nConflicts resolved: %d\nRisk level: %s\nReady for merge: YES",
		t.Id,
		result.ConflictsResolved,
		result.RiskLevel,
	)

	checkpoint, err := e.gitSvc.CreateCheckpoint(t.Id, "finalize", "completed")
	if err != nil {
		if err := e.gitSvc.Context().StageAll(); err != nil {
			return "", fmt.Errorf("stage all: %w", err)
		}
		if err := e.gitSvc.Context().Commit(msg); err != nil {
			return "", fmt.Errorf("commit: %w", err)
		}
		sha, _ := e.gitSvc.Context().HeadCommit()
		return sha, nil
	}

	return checkpoint.CommitSHA, nil
}

// shouldEscalate determines if the finalize failure should trigger escalation.
func (e *FinalizeExecutor) shouldEscalate(result *FinalizeResult, _ config.FinalizeConfig) bool {
	if result == nil {
		return false
	}
	if len(result.ConflictFiles) > 10 {
		return true
	}
	if !result.TestsPassed && len(result.TestFailures) > 5 {
		return true
	}
	return false
}

// publishProgress publishes a progress message for the finalize phase.
func (e *FinalizeExecutor) publishProgress(taskID, phaseID, message string) {
	e.publisher.Transcript(taskID, phaseID, 0, "progress", message)
}

// buildEscalationContext creates context for escalation to implement phase.
func buildEscalationContext(result *FinalizeResult) string {
	if result == nil {
		return "Finalize phase failed and requires escalation to implement phase"
	}

	var sb strings.Builder
	sb.WriteString("## Finalize Escalation Required\n\n")
	sb.WriteString("The finalize phase encountered issues that require revisiting implementation:\n\n")

	if len(result.ConflictFiles) > 0 {
		sb.WriteString("### Unresolved Conflicts\n\n")
		for _, f := range result.ConflictFiles {
			sb.WriteString("- `")
			sb.WriteString(f)
			sb.WriteString("`\n")
		}
		sb.WriteString("\n")
	}

	if !result.TestsPassed && len(result.TestFailures) > 0 {
		sb.WriteString("### Test Failures\n\n")
		for i, f := range result.TestFailures {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("... and %d more failures\n", len(result.TestFailures)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("- %s: %s\n", f.Test, f.Message))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Please review and fix these issues in the implement phase, then retry finalize.\n")
	return sb.String()
}

// buildFinalizeReport creates the finalization report output.
func buildFinalizeReport(taskID, targetBranch string, result *FinalizeResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Finalization Report: %s\n\n", taskID))
	sb.WriteString("## Sync Summary\n\n")
	sb.WriteString("| Metric | Value |\n")
	sb.WriteString("|--------|-------|\n")
	sb.WriteString(fmt.Sprintf("| Target Branch | %s |\n", targetBranch))
	sb.WriteString(fmt.Sprintf("| Conflicts Resolved | %d |\n", result.ConflictsResolved))
	sb.WriteString(fmt.Sprintf("| Files Changed (total) | %d |\n", result.FilesChanged))
	sb.WriteString(fmt.Sprintf("| Lines Changed (total) | %d |\n", result.LinesChanged))
	sb.WriteString("\n")

	if len(result.ConflictFiles) > 0 {
		sb.WriteString("## Conflict Resolution\n\n")
		sb.WriteString("| File | Status |\n")
		sb.WriteString("|------|--------|\n")
		for _, f := range result.ConflictFiles {
			sb.WriteString(fmt.Sprintf("| `%s` | ✓ Resolved |\n", f))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Test Results\n\n")
	if result.TestsPassed {
		sb.WriteString("✓ All tests passed\n\n")
	} else {
		sb.WriteString("✗ Tests failed\n\n")
	}

	sb.WriteString("## Risk Assessment\n\n")
	sb.WriteString("| Factor | Value | Risk |\n")
	sb.WriteString("|--------|-------|------|\n")
	sb.WriteString(fmt.Sprintf("| Files Changed | %d | %s |\n", result.FilesChanged, classifyFileRisk(result.FilesChanged)))
	sb.WriteString(fmt.Sprintf("| Lines Changed | %d | %s |\n", result.LinesChanged, classifyLineRisk(result.LinesChanged)))
	sb.WriteString(fmt.Sprintf("| Conflicts Resolved | %d | %s |\n", result.ConflictsResolved, classifyConflictRisk(result.ConflictsResolved)))
	sb.WriteString(fmt.Sprintf("| **Overall Risk** | | **%s** |\n", result.RiskLevel))
	sb.WriteString("\n")

	sb.WriteString("## Merge Decision\n\n")
	if result.NeedsReview {
		sb.WriteString("**Ready for Merge**: NO - Review Required\n")
		sb.WriteString("**Recommended Action**: review-then-merge\n")
	} else if result.RiskLevel == "critical" {
		sb.WriteString("**Ready for Merge**: NO - Senior Review Required\n")
		sb.WriteString("**Recommended Action**: senior-review-required\n")
	} else {
		sb.WriteString("**Ready for Merge**: YES\n")
		sb.WriteString("**Recommended Action**: auto-merge\n")
	}

	if result.CommitSHA != "" {
		sb.WriteString(fmt.Sprintf("\n**Commit**: %s\n", result.CommitSHA))
	}

	sb.WriteString("\n")
	sb.WriteString(`{"status": "complete", "summary": "Finalization complete"}`)
	sb.WriteString("\n")
	return sb.String()
}

// parseFileCount extracts file count from git diff --stat last line.
func parseFileCount(line string) int {
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		count, _ := strconv.Atoi(parts[0])
		return count
	}
	return 0
}

// parseTotalLines calculates total lines from git diff --numstat.
func parseTotalLines(numstat string) int {
	total := 0
	for _, line := range strings.Split(numstat, "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			added, _ := strconv.Atoi(parts[0])
			removed, _ := strconv.Atoi(parts[1])
			total += added + removed
		}
	}
	return total
}

// classifyRisk determines the overall risk level.
func classifyRisk(files, lines, conflicts int) string {
	if files > 30 || lines > 1000 || conflicts > 10 {
		return "critical"
	}
	if files > 15 || lines > 500 || conflicts > 3 {
		return "high"
	}
	if files > 5 || lines > 100 || conflicts > 0 {
		return "medium"
	}
	return "low"
}

// classifyFileRisk returns risk level based on file count.
func classifyFileRisk(files int) string {
	if files > 30 {
		return "Critical"
	}
	if files > 15 {
		return "High"
	}
	if files > 5 {
		return "Medium"
	}
	return "Low"
}

// classifyLineRisk returns risk level based on line count.
func classifyLineRisk(lines int) string {
	if lines > 1000 {
		return "Critical"
	}
	if lines > 500 {
		return "High"
	}
	if lines > 100 {
		return "Medium"
	}
	return "Low"
}

// classifyConflictRisk returns risk level based on conflict count.
func classifyConflictRisk(conflicts int) string {
	if conflicts > 10 {
		return "High"
	}
	if conflicts > 3 {
		return "Medium"
	}
	if conflicts > 0 {
		return "Low"
	}
	return "None"
}

// shouldTriggerReview determines if review should be triggered based on risk.
func shouldTriggerReview(riskLevel, threshold string) bool {
	riskOrder := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}
	return riskOrder[riskLevel] >= riskOrder[threshold]
}
