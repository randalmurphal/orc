package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// QATest represents a test written during QA.
type QATest struct {
	File        string `json:"file"`
	Description string `json:"description"`
	Type        string `json:"type"`
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
	Type string `json:"type"`
}

// QAIssue represents an issue found during QA.
type QAIssue struct {
	Severity     string `json:"severity"`
	Description  string `json:"description"`
	Reproduction string `json:"reproduction,omitempty"`
}

// QAResult represents the complete result of a QA session.
type QAResult struct {
	TaskID         string      `json:"task_id"`
	Status         string      `json:"status"`
	Summary        string      `json:"summary"`
	TestsWritten   []QATest    `json:"tests_written,omitempty"`
	TestsRun       *QATestRun  `json:"tests_run,omitempty"`
	Coverage       *QACoverage `json:"coverage,omitempty"`
	Documentation  []QADoc     `json:"documentation,omitempty"`
	Issues         []QAIssue   `json:"issues,omitempty"`
	Recommendation string      `json:"recommendation"`
	CreatedAt      time.Time   `json:"created_at"`
}

// SaveQAResult saves or updates QA results for a task.
func (p *ProjectDB) SaveQAResult(r *QAResult) error {
	testsWrittenJSON, err := json.Marshal(r.TestsWritten)
	if err != nil {
		return fmt.Errorf("marshal tests_written: %w", err)
	}

	var testsRunJSON []byte
	if r.TestsRun != nil {
		testsRunJSON, err = json.Marshal(r.TestsRun)
		if err != nil {
			return fmt.Errorf("marshal tests_run: %w", err)
		}
	}

	var coverageJSON []byte
	if r.Coverage != nil {
		coverageJSON, err = json.Marshal(r.Coverage)
		if err != nil {
			return fmt.Errorf("marshal coverage: %w", err)
		}
	}

	documentationJSON, err := json.Marshal(r.Documentation)
	if err != nil {
		return fmt.Errorf("marshal documentation: %w", err)
	}

	issuesJSON, err := json.Marshal(r.Issues)
	if err != nil {
		return fmt.Errorf("marshal issues: %w", err)
	}

	_, err = p.Exec(`
		INSERT INTO qa_results (task_id, status, summary, tests_written_json, tests_run_json, coverage_json, documentation_json, issues_json, recommendation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id) DO UPDATE SET
			status = excluded.status,
			summary = excluded.summary,
			tests_written_json = excluded.tests_written_json,
			tests_run_json = excluded.tests_run_json,
			coverage_json = excluded.coverage_json,
			documentation_json = excluded.documentation_json,
			issues_json = excluded.issues_json,
			recommendation = excluded.recommendation
	`, r.TaskID, r.Status, r.Summary,
		bytesToNullString(testsWrittenJSON),
		bytesToNullString(testsRunJSON),
		bytesToNullString(coverageJSON),
		bytesToNullString(documentationJSON),
		bytesToNullString(issuesJSON),
		r.Recommendation)
	if err != nil {
		return fmt.Errorf("save qa result %s: %w", r.TaskID, err)
	}
	return nil
}

// bytesToNullString converts []byte to sql.NullString for QA result storage.
func bytesToNullString(b []byte) sql.NullString {
	if len(b) == 0 {
		return sql.NullString{}
	}
	return sql.NullString{String: string(b), Valid: true}
}

// GetQAResult retrieves QA results for a task.
func (p *ProjectDB) GetQAResult(taskID string) (*QAResult, error) {
	var r QAResult
	var testsWrittenJSON, testsRunJSON, coverageJSON, documentationJSON, issuesJSON sql.NullString
	var createdAt string

	err := p.QueryRow(`
		SELECT task_id, status, summary, tests_written_json, tests_run_json, coverage_json, documentation_json, issues_json, recommendation, created_at
		FROM qa_results
		WHERE task_id = ?
	`, taskID).Scan(&r.TaskID, &r.Status, &r.Summary, &testsWrittenJSON, &testsRunJSON, &coverageJSON, &documentationJSON, &issuesJSON, &r.Recommendation, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get qa result %s: %w", taskID, err)
	}

	// Parse JSON fields
	if testsWrittenJSON.Valid && testsWrittenJSON.String != "" {
		if err := json.Unmarshal([]byte(testsWrittenJSON.String), &r.TestsWritten); err != nil {
			return nil, fmt.Errorf("unmarshal tests_written: %w", err)
		}
	}
	if r.TestsWritten == nil {
		r.TestsWritten = []QATest{}
	}

	if testsRunJSON.Valid && testsRunJSON.String != "" {
		r.TestsRun = &QATestRun{}
		if err := json.Unmarshal([]byte(testsRunJSON.String), r.TestsRun); err != nil {
			return nil, fmt.Errorf("unmarshal tests_run: %w", err)
		}
	}

	if coverageJSON.Valid && coverageJSON.String != "" {
		r.Coverage = &QACoverage{}
		if err := json.Unmarshal([]byte(coverageJSON.String), r.Coverage); err != nil {
			return nil, fmt.Errorf("unmarshal coverage: %w", err)
		}
	}

	if documentationJSON.Valid && documentationJSON.String != "" {
		if err := json.Unmarshal([]byte(documentationJSON.String), &r.Documentation); err != nil {
			return nil, fmt.Errorf("unmarshal documentation: %w", err)
		}
	}
	if r.Documentation == nil {
		r.Documentation = []QADoc{}
	}

	if issuesJSON.Valid && issuesJSON.String != "" {
		if err := json.Unmarshal([]byte(issuesJSON.String), &r.Issues); err != nil {
			return nil, fmt.Errorf("unmarshal issues: %w", err)
		}
	}
	if r.Issues == nil {
		r.Issues = []QAIssue{}
	}

	// Parse timestamp - SQLite datetime() returns "2006-01-02 15:04:05" format
	// Try multiple formats for compatibility
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	} {
		if ts, err := time.Parse(layout, createdAt); err == nil {
			r.CreatedAt = ts
			break
		}
	}

	return &r, nil
}

// DeleteQAResult deletes QA results for a task.
func (p *ProjectDB) DeleteQAResult(taskID string) error {
	_, err := p.Exec(`DELETE FROM qa_results WHERE task_id = ?`, taskID)
	if err != nil {
		return fmt.Errorf("delete qa result %s: %w", taskID, err)
	}
	return nil
}
