package executor

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/randalmurphal/orc/internal/config"
)

// QAStatus represents the status from a QA session.
type QAStatus string

const (
	QAStatusPass           QAStatus = "pass"
	QAStatusFail           QAStatus = "fail"
	QAStatusNeedsAttention QAStatus = "needs_attention"
)

// QATest represents a test written during QA.
type QATest struct {
	File        string `json:"file"`
	Description string `json:"description"`
	Type        string `json:"type"` // e2e, integration, unit
}

// QATestRun represents test execution results.
type QATestRun struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Skipped int `json:"skipped"`
}

// QACoverage represents code coverage information.
type QACoverage struct {
	Percentage     float64 `json:"percentage"`
	UncoveredAreas string  `json:"uncovered_areas,omitempty"`
}

// QADoc represents documentation created during QA.
type QADoc struct {
	File string `json:"file"`
	Type string `json:"type"` // feature, api, testing
}

// QAIssue represents an issue found during QA.
type QAIssue struct {
	Severity     string `json:"severity"` // high, medium, low
	Description  string `json:"description"`
	Reproduction string `json:"reproduction,omitempty"`
}

// QAResult represents the complete result of a QA session.
type QAResult struct {
	Status         QAStatus   `json:"status"`
	Summary        string     `json:"summary"`
	TestsWritten   []QATest   `json:"tests_written,omitempty"`
	TestsRun       *QATestRun `json:"tests_run,omitempty"`
	Coverage       *QACoverage `json:"coverage,omitempty"`
	Documentation  []QADoc    `json:"documentation,omitempty"`
	Issues         []QAIssue  `json:"issues,omitempty"`
	Recommendation string     `json:"recommendation"`
}

// ShouldRunQA checks if QA should run based on config and task weight.
func ShouldRunQA(cfg *config.Config, weight string) bool {
	if cfg == nil {
		// Default: run QA except for trivial tasks
		return weight != "trivial"
	}
	if !cfg.QA.Enabled {
		return false
	}
	// Check if weight is in skip list
	for _, skip := range cfg.QA.SkipForWeights {
		if skip == weight {
			return false
		}
	}
	return true
}

// ParseQAResult extracts QA result from Claude's response.
func ParseQAResult(response string) (*QAResult, error) {
	result := &QAResult{
		TestsWritten:  []QATest{},
		Documentation: []QADoc{},
		Issues:        []QAIssue{},
	}

	// Extract <qa_result> block
	resultRe := regexp.MustCompile(`(?s)<qa_result>(.*?)</qa_result>`)
	resultMatch := resultRe.FindStringSubmatch(response)
	if resultMatch == nil {
		return nil, fmt.Errorf("no <qa_result> block found in response")
	}
	content := resultMatch[1]

	// Parse status
	statusRe := regexp.MustCompile(`<status>(pass|fail|needs_attention)</status>`)
	if m := statusRe.FindStringSubmatch(content); m != nil {
		result.Status = QAStatus(m[1])
	}

	// Parse summary
	summaryRe := regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)
	if m := summaryRe.FindStringSubmatch(content); m != nil {
		result.Summary = strings.TrimSpace(m[1])
	}

	// Parse tests_written
	testsWrittenRe := regexp.MustCompile(`(?s)<tests_written>(.*?)</tests_written>`)
	if m := testsWrittenRe.FindStringSubmatch(content); m != nil {
		testRe := regexp.MustCompile(`(?s)<test>(.*?)</test>`)
		tests := testRe.FindAllStringSubmatch(m[1], -1)
		for _, tm := range tests {
			test := QATest{}
			if fm := regexp.MustCompile(`<file>(.*?)</file>`).FindStringSubmatch(tm[1]); fm != nil {
				test.File = strings.TrimSpace(fm[1])
			}
			if dm := regexp.MustCompile(`<description>(.*?)</description>`).FindStringSubmatch(tm[1]); dm != nil {
				test.Description = strings.TrimSpace(dm[1])
			}
			if tym := regexp.MustCompile(`<type>(.*?)</type>`).FindStringSubmatch(tm[1]); tym != nil {
				test.Type = strings.TrimSpace(tym[1])
			}
			result.TestsWritten = append(result.TestsWritten, test)
		}
	}

	// Parse tests_run
	testsRunRe := regexp.MustCompile(`(?s)<tests_run>(.*?)</tests_run>`)
	if m := testsRunRe.FindStringSubmatch(content); m != nil {
		result.TestsRun = &QATestRun{}
		runContent := m[1]
		if tm := regexp.MustCompile(`<total>(\d+)</total>`).FindStringSubmatch(runContent); tm != nil {
			fmt.Sscanf(tm[1], "%d", &result.TestsRun.Total)
		}
		if pm := regexp.MustCompile(`<passed>(\d+)</passed>`).FindStringSubmatch(runContent); pm != nil {
			fmt.Sscanf(pm[1], "%d", &result.TestsRun.Passed)
		}
		if fm := regexp.MustCompile(`<failed>(\d+)</failed>`).FindStringSubmatch(runContent); fm != nil {
			fmt.Sscanf(fm[1], "%d", &result.TestsRun.Failed)
		}
		if sm := regexp.MustCompile(`<skipped>(\d+)</skipped>`).FindStringSubmatch(runContent); sm != nil {
			fmt.Sscanf(sm[1], "%d", &result.TestsRun.Skipped)
		}
	}

	// Parse coverage
	coverageRe := regexp.MustCompile(`(?s)<coverage>(.*?)</coverage>`)
	if m := coverageRe.FindStringSubmatch(content); m != nil {
		result.Coverage = &QACoverage{}
		covContent := m[1]
		if pm := regexp.MustCompile(`<percentage>([0-9.]+)%?</percentage>`).FindStringSubmatch(covContent); pm != nil {
			fmt.Sscanf(pm[1], "%f", &result.Coverage.Percentage)
		}
		if um := regexp.MustCompile(`<uncovered_areas>(.*?)</uncovered_areas>`).FindStringSubmatch(covContent); um != nil {
			result.Coverage.UncoveredAreas = strings.TrimSpace(um[1])
		}
	}

	// Parse documentation
	docRe := regexp.MustCompile(`(?s)<documentation>(.*?)</documentation>`)
	if m := docRe.FindStringSubmatch(content); m != nil {
		fileRe := regexp.MustCompile(`(?s)<file>(.*?)</file>`)
		typeRe := regexp.MustCompile(`<type>(.*?)</type>`)

		// Handle multiple doc entries or single entry
		// Try to find structured entries first
		docContent := m[1]
		files := fileRe.FindAllStringSubmatch(docContent, -1)
		types := typeRe.FindAllStringSubmatch(docContent, -1)

		for i := 0; i < len(files); i++ {
			doc := QADoc{
				File: strings.TrimSpace(files[i][1]),
			}
			if i < len(types) {
				doc.Type = strings.TrimSpace(types[i][1])
			}
			result.Documentation = append(result.Documentation, doc)
		}
	}

	// Parse issues
	issuesRe := regexp.MustCompile(`(?s)<issues>(.*?)</issues>`)
	if m := issuesRe.FindStringSubmatch(content); m != nil {
		issueRe := regexp.MustCompile(`(?s)<issue\s+severity="(high|medium|low)">(.*?)</issue>`)
		issues := issueRe.FindAllStringSubmatch(m[1], -1)
		for _, im := range issues {
			issue := QAIssue{
				Severity: im[1],
			}
			issueContent := im[2]
			if dm := regexp.MustCompile(`(?s)<description>(.*?)</description>`).FindStringSubmatch(issueContent); dm != nil {
				issue.Description = strings.TrimSpace(dm[1])
			}
			if rm := regexp.MustCompile(`(?s)<reproduction>(.*?)</reproduction>`).FindStringSubmatch(issueContent); rm != nil {
				issue.Reproduction = strings.TrimSpace(rm[1])
			}
			result.Issues = append(result.Issues, issue)
		}
	}

	// Parse recommendation
	recRe := regexp.MustCompile(`(?s)<recommendation>(.*?)</recommendation>`)
	if m := recRe.FindStringSubmatch(content); m != nil {
		result.Recommendation = strings.TrimSpace(m[1])
	}

	return result, nil
}

// HasHighSeverityIssues checks if there are any high-severity issues.
func (r *QAResult) HasHighSeverityIssues() bool {
	for _, issue := range r.Issues {
		if issue.Severity == "high" {
			return true
		}
	}
	return false
}

// AllTestsPassed checks if all tests passed.
func (r *QAResult) AllTestsPassed() bool {
	if r.TestsRun == nil {
		return true // No tests run, assume pass
	}
	return r.TestsRun.Failed == 0
}

// FormatQAResultSummary formats QA result for display.
func FormatQAResultSummary(result *QAResult) string {
	if result == nil {
		return "No QA result available."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("QA Status: %s\n\n", strings.ToUpper(string(result.Status))))
	sb.WriteString(fmt.Sprintf("Summary: %s\n\n", result.Summary))

	if result.TestsRun != nil {
		sb.WriteString(fmt.Sprintf("Tests: %d total, %d passed, %d failed, %d skipped\n",
			result.TestsRun.Total, result.TestsRun.Passed, result.TestsRun.Failed, result.TestsRun.Skipped))
	}

	if result.Coverage != nil {
		sb.WriteString(fmt.Sprintf("Coverage: %.1f%%\n", result.Coverage.Percentage))
	}

	if len(result.TestsWritten) > 0 {
		sb.WriteString(fmt.Sprintf("\nTests Written: %d\n", len(result.TestsWritten)))
		for _, t := range result.TestsWritten {
			sb.WriteString(fmt.Sprintf("  - %s (%s): %s\n", t.File, t.Type, t.Description))
		}
	}

	if len(result.Documentation) > 0 {
		sb.WriteString(fmt.Sprintf("\nDocumentation: %d files\n", len(result.Documentation)))
		for _, d := range result.Documentation {
			sb.WriteString(fmt.Sprintf("  - %s (%s)\n", d.File, d.Type))
		}
	}

	if len(result.Issues) > 0 {
		sb.WriteString(fmt.Sprintf("\nIssues: %d\n", len(result.Issues)))
		for _, i := range result.Issues {
			sb.WriteString(fmt.Sprintf("  - [%s] %s\n", strings.ToUpper(i.Severity), i.Description))
		}
	}

	if result.Recommendation != "" {
		sb.WriteString(fmt.Sprintf("\nRecommendation: %s\n", result.Recommendation))
	}

	return sb.String()
}
