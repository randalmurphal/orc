package storage

import (
	"fmt"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/db"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ============================================================================
// Review findings, QA results, gate decisions - quality/review outputs
// ============================================================================

func (d *DatabaseBackend) ListGateDecisions(taskID string) ([]db.GateDecision, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	decisions, err := d.db.GetGateDecisions(taskID)
	if err != nil {
		return nil, fmt.Errorf("list gate decisions: %w", err)
	}
	return decisions, nil
}

func (d *DatabaseBackend) SaveGateDecision(gd *db.GateDecision) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.db.AddGateDecision(gd); err != nil {
		return fmt.Errorf("save gate decision: %w", err)
	}
	return nil
}

// ============================================================================
// Review findings - uses proto types directly
// ============================================================================

func (d *DatabaseBackend) SaveReviewFindings(f *orcv1.ReviewRoundFindings) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbFindings := protoToDBReviewFindings(f)
	return d.db.SaveReviewFindings(dbFindings)
}

func (d *DatabaseBackend) LoadReviewFindings(taskID string, round int) (*orcv1.ReviewRoundFindings, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbFindings, err := d.db.GetReviewFindings(taskID, round)
	if err != nil {
		return nil, err
	}
	if dbFindings == nil {
		return nil, nil
	}
	return dbToProtoReviewFindings(dbFindings), nil
}

func (d *DatabaseBackend) LoadAllReviewFindings(taskID string) ([]*orcv1.ReviewRoundFindings, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbList, err := d.db.GetAllReviewFindings(taskID)
	if err != nil {
		return nil, err
	}
	result := make([]*orcv1.ReviewRoundFindings, len(dbList))
	for i, dbFindings := range dbList {
		result[i] = dbToProtoReviewFindings(dbFindings)
	}
	return result, nil
}

// protoToDBReviewFindings converts proto type to db type for persistence.
func protoToDBReviewFindings(f *orcv1.ReviewRoundFindings) *db.ReviewFindings {
	dbFindings := &db.ReviewFindings{
		TaskID:    f.TaskId,
		Round:     int(f.Round),
		Summary:   f.Summary,
		Issues:    make([]db.ReviewFinding, len(f.Issues)),
		Questions: f.Questions,
		Positives: f.Positives,
	}

	if f.AgentId != nil {
		dbFindings.AgentID = *f.AgentId
	}
	if f.CreatedAt != nil {
		dbFindings.CreatedAt = f.CreatedAt.AsTime()
	}

	for i, issue := range f.Issues {
		dbIssue := db.ReviewFinding{
			Severity:    issue.Severity,
			Description: issue.Description,
		}
		if issue.File != nil {
			dbIssue.File = *issue.File
		}
		if issue.Line != nil {
			dbIssue.Line = int(*issue.Line)
		}
		if issue.Suggestion != nil {
			dbIssue.Suggestion = *issue.Suggestion
		}
		if issue.AgentId != nil {
			dbIssue.AgentID = *issue.AgentId
		}
		if issue.ConstitutionViolation != nil {
			dbIssue.ConstitutionViolation = *issue.ConstitutionViolation
		}
		dbFindings.Issues[i] = dbIssue
	}

	if dbFindings.Issues == nil {
		dbFindings.Issues = []db.ReviewFinding{}
	}
	if dbFindings.Questions == nil {
		dbFindings.Questions = []string{}
	}
	if dbFindings.Positives == nil {
		dbFindings.Positives = []string{}
	}

	return dbFindings
}

// dbToProtoReviewFindings converts db type to proto type for API responses.
func dbToProtoReviewFindings(dbFindings *db.ReviewFindings) *orcv1.ReviewRoundFindings {
	f := &orcv1.ReviewRoundFindings{
		TaskId:    dbFindings.TaskID,
		Round:     int32(dbFindings.Round),
		Summary:   dbFindings.Summary,
		Issues:    make([]*orcv1.ReviewFinding, len(dbFindings.Issues)),
		Questions: dbFindings.Questions,
		Positives: dbFindings.Positives,
		CreatedAt: timestamppb.New(dbFindings.CreatedAt),
	}

	if dbFindings.AgentID != "" {
		f.AgentId = &dbFindings.AgentID
	}

	for i, issue := range dbFindings.Issues {
		protoIssue := &orcv1.ReviewFinding{
			Severity:    issue.Severity,
			Description: issue.Description,
		}
		if issue.File != "" {
			protoIssue.File = &issue.File
		}
		if issue.Line > 0 {
			line := int32(issue.Line)
			protoIssue.Line = &line
		}
		if issue.Suggestion != "" {
			protoIssue.Suggestion = &issue.Suggestion
		}
		if issue.AgentID != "" {
			protoIssue.AgentId = &issue.AgentID
		}
		if issue.ConstitutionViolation != "" {
			protoIssue.ConstitutionViolation = &issue.ConstitutionViolation
		}
		f.Issues[i] = protoIssue
	}

	if f.Issues == nil {
		f.Issues = []*orcv1.ReviewFinding{}
	}
	if f.Questions == nil {
		f.Questions = []string{}
	}
	if f.Positives == nil {
		f.Positives = []string{}
	}

	return f
}

// ============================================================================
// QA results
// ============================================================================

func (d *DatabaseBackend) SaveQAResult(r *QAResult) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	dbResult := &db.QAResult{
		TaskID:         r.TaskID,
		Status:         r.Status,
		Summary:        r.Summary,
		Recommendation: r.Recommendation,
	}

	for _, t := range r.TestsWritten {
		dbResult.TestsWritten = append(dbResult.TestsWritten, db.QATest{
			File:        t.File,
			Description: t.Description,
			Type:        t.Type,
		})
	}

	if r.TestsRun != nil {
		dbResult.TestsRun = &db.QATestRun{
			Total:   r.TestsRun.Total,
			Passed:  r.TestsRun.Passed,
			Failed:  r.TestsRun.Failed,
			Skipped: r.TestsRun.Skipped,
		}
	}

	if r.Coverage != nil {
		dbResult.Coverage = &db.QACoverage{
			Percentage:     r.Coverage.Percentage,
			UncoveredAreas: r.Coverage.UncoveredAreas,
		}
	}

	for _, doc := range r.Documentation {
		dbResult.Documentation = append(dbResult.Documentation, db.QADoc{
			File: doc.File,
			Type: doc.Type,
		})
	}

	for _, issue := range r.Issues {
		dbResult.Issues = append(dbResult.Issues, db.QAIssue{
			Severity:     issue.Severity,
			Description:  issue.Description,
			Reproduction: issue.Reproduction,
		})
	}

	return d.db.SaveQAResult(dbResult)
}

func (d *DatabaseBackend) LoadQAResult(taskID string) (*QAResult, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	dbResult, err := d.db.GetQAResult(taskID)
	if err != nil {
		return nil, err
	}
	if dbResult == nil {
		return nil, nil
	}

	return convertDBQAResult(dbResult), nil
}

func convertDBQAResult(dbResult *db.QAResult) *QAResult {
	r := &QAResult{
		TaskID:         dbResult.TaskID,
		Status:         dbResult.Status,
		Summary:        dbResult.Summary,
		Recommendation: dbResult.Recommendation,
		CreatedAt:      dbResult.CreatedAt,
	}

	for _, t := range dbResult.TestsWritten {
		r.TestsWritten = append(r.TestsWritten, QATest{
			File:        t.File,
			Description: t.Description,
			Type:        t.Type,
		})
	}
	if r.TestsWritten == nil {
		r.TestsWritten = []QATest{}
	}

	if dbResult.TestsRun != nil {
		r.TestsRun = &QATestRun{
			Total:   dbResult.TestsRun.Total,
			Passed:  dbResult.TestsRun.Passed,
			Failed:  dbResult.TestsRun.Failed,
			Skipped: dbResult.TestsRun.Skipped,
		}
	}

	if dbResult.Coverage != nil {
		r.Coverage = &QACoverage{
			Percentage:     dbResult.Coverage.Percentage,
			UncoveredAreas: dbResult.Coverage.UncoveredAreas,
		}
	}

	for _, doc := range dbResult.Documentation {
		r.Documentation = append(r.Documentation, QADoc{
			File: doc.File,
			Type: doc.Type,
		})
	}
	if r.Documentation == nil {
		r.Documentation = []QADoc{}
	}

	for _, issue := range dbResult.Issues {
		r.Issues = append(r.Issues, QAIssue{
			Severity:     issue.Severity,
			Description:  issue.Description,
			Reproduction: issue.Reproduction,
		})
	}
	if r.Issues == nil {
		r.Issues = []QAIssue{}
	}

	return r
}
