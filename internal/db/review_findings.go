package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ReviewFinding represents a single issue found during review.
type ReviewFinding struct {
	Severity              string `json:"severity"` // high, medium, low
	File                  string `json:"file,omitempty"`
	Line                  int    `json:"line,omitempty"`
	Description           string `json:"description"`
	Suggestion            string `json:"suggestion,omitempty"`
	Perspective           string `json:"perspective,omitempty"`
	ConstitutionViolation string `json:"constitution_violation,omitempty"` // "invariant" (blocker) or "default" (warning)
}

// ReviewFindings represents the structured output from a review round.
type ReviewFindings struct {
	TaskID      string          `json:"task_id"`
	Round       int             `json:"round"`
	Summary     string          `json:"summary"`
	Issues      []ReviewFinding `json:"issues"`
	Questions   []string        `json:"questions,omitempty"`
	Positives   []string        `json:"positives,omitempty"`
	Perspective string          `json:"perspective,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// SaveReviewFindings creates or updates review findings for a task/round.
func (p *ProjectDB) SaveReviewFindings(findings *ReviewFindings) error {
	now := time.Now().Format(time.RFC3339)

	// Marshal JSON arrays
	issuesJSON, err := json.Marshal(findings.Issues)
	if err != nil {
		return fmt.Errorf("marshal issues: %w", err)
	}
	questionsJSON, err := json.Marshal(findings.Questions)
	if err != nil {
		return fmt.Errorf("marshal questions: %w", err)
	}
	positivesJSON, err := json.Marshal(findings.Positives)
	if err != nil {
		return fmt.Errorf("marshal positives: %w", err)
	}

	_, err = p.Exec(`
		INSERT INTO review_findings (task_id, review_round, summary, issues_json, questions_json, positives_json, perspective, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_id, review_round) DO UPDATE SET
			summary = excluded.summary,
			issues_json = excluded.issues_json,
			questions_json = excluded.questions_json,
			positives_json = excluded.positives_json,
			perspective = excluded.perspective,
			created_at = excluded.created_at
	`, findings.TaskID, findings.Round, findings.Summary,
		string(issuesJSON), string(questionsJSON), string(positivesJSON),
		findings.Perspective, now)
	if err != nil {
		return fmt.Errorf("save review findings: %w", err)
	}
	return nil
}

// GetReviewFindings retrieves review findings for a task and round.
func (p *ProjectDB) GetReviewFindings(taskID string, round int) (*ReviewFindings, error) {
	row := p.QueryRow(`
		SELECT task_id, review_round, summary, issues_json, questions_json, positives_json, perspective, created_at
		FROM review_findings
		WHERE task_id = ? AND review_round = ?
	`, taskID, round)

	var f ReviewFindings
	var issuesJSON, questionsJSON, positivesJSON sql.NullString
	var perspective sql.NullString
	var createdAt string

	if err := row.Scan(&f.TaskID, &f.Round, &f.Summary, &issuesJSON, &questionsJSON, &positivesJSON, &perspective, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get review findings %s round %d: %w", taskID, round, err)
	}

	// Unmarshal JSON arrays
	if issuesJSON.Valid && issuesJSON.String != "" {
		if err := json.Unmarshal([]byte(issuesJSON.String), &f.Issues); err != nil {
			return nil, fmt.Errorf("unmarshal issues: %w", err)
		}
	}
	if f.Issues == nil {
		f.Issues = []ReviewFinding{}
	}

	if questionsJSON.Valid && questionsJSON.String != "" {
		if err := json.Unmarshal([]byte(questionsJSON.String), &f.Questions); err != nil {
			return nil, fmt.Errorf("unmarshal questions: %w", err)
		}
	}
	if f.Questions == nil {
		f.Questions = []string{}
	}

	if positivesJSON.Valid && positivesJSON.String != "" {
		if err := json.Unmarshal([]byte(positivesJSON.String), &f.Positives); err != nil {
			return nil, fmt.Errorf("unmarshal positives: %w", err)
		}
	}
	if f.Positives == nil {
		f.Positives = []string{}
	}

	if perspective.Valid {
		f.Perspective = perspective.String
	}
	// Parse timestamp - SQLite datetime() returns "2006-01-02 15:04:05" format
	// Try multiple formats for compatibility
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	} {
		if ts, err := time.Parse(layout, createdAt); err == nil {
			f.CreatedAt = ts
			break
		}
	}

	return &f, nil
}

// GetAllReviewFindings retrieves all review findings for a task (all rounds).
func (p *ProjectDB) GetAllReviewFindings(taskID string) ([]*ReviewFindings, error) {
	rows, err := p.Query(`
		SELECT task_id, review_round, summary, issues_json, questions_json, positives_json, perspective, created_at
		FROM review_findings
		WHERE task_id = ?
		ORDER BY review_round ASC
	`, taskID)
	if err != nil {
		return nil, fmt.Errorf("get all review findings %s: %w", taskID, err)
	}
	defer func() { _ = rows.Close() }()

	var results []*ReviewFindings
	for rows.Next() {
		var f ReviewFindings
		var issuesJSON, questionsJSON, positivesJSON sql.NullString
		var perspective sql.NullString
		var createdAt string

		if err := rows.Scan(&f.TaskID, &f.Round, &f.Summary, &issuesJSON, &questionsJSON, &positivesJSON, &perspective, &createdAt); err != nil {
			return nil, fmt.Errorf("scan review findings: %w", err)
		}

		// Unmarshal JSON arrays
		if issuesJSON.Valid && issuesJSON.String != "" {
			if err := json.Unmarshal([]byte(issuesJSON.String), &f.Issues); err != nil {
				return nil, fmt.Errorf("unmarshal issues: %w", err)
			}
		}
		if f.Issues == nil {
			f.Issues = []ReviewFinding{}
		}

		if questionsJSON.Valid && questionsJSON.String != "" {
			if err := json.Unmarshal([]byte(questionsJSON.String), &f.Questions); err != nil {
				return nil, fmt.Errorf("unmarshal questions: %w", err)
			}
		}
		if f.Questions == nil {
			f.Questions = []string{}
		}

		if positivesJSON.Valid && positivesJSON.String != "" {
			if err := json.Unmarshal([]byte(positivesJSON.String), &f.Positives); err != nil {
				return nil, fmt.Errorf("unmarshal positives: %w", err)
			}
		}
		if f.Positives == nil {
			f.Positives = []string{}
		}

		if perspective.Valid {
			f.Perspective = perspective.String
		}
		// Parse timestamp - SQLite datetime() returns "2006-01-02 15:04:05" format
		for _, layout := range []string{
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05",
		} {
			if ts, err := time.Parse(layout, createdAt); err == nil {
				f.CreatedAt = ts
				break
			}
		}

		results = append(results, &f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate review findings: %w", err)
	}

	return results, nil
}

// DeleteReviewFindings removes all review findings for a task.
func (p *ProjectDB) DeleteReviewFindings(taskID string) error {
	_, err := p.Exec("DELETE FROM review_findings WHERE task_id = ?", taskID)
	if err != nil {
		return fmt.Errorf("delete review findings: %w", err)
	}
	return nil
}
