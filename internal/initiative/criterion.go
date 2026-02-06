package initiative

import (
	"fmt"
	"time"
)

// CriterionStatus represents the verification status of an acceptance criterion.
type CriterionStatus = string

const (
	CriterionStatusUncovered CriterionStatus = "uncovered"
	CriterionStatusCovered   CriterionStatus = "covered"
	CriterionStatusSatisfied CriterionStatus = "satisfied"
	CriterionStatusRegressed CriterionStatus = "regressed"
)

// Criterion represents an acceptance criterion for an initiative.
type Criterion struct {
	ID          string          `json:"id" yaml:"id"`
	Description string          `json:"description" yaml:"description"`
	TaskIDs     []string        `json:"task_ids" yaml:"task_ids"`
	Status      CriterionStatus `json:"status" yaml:"status"`
	VerifiedAt  string          `json:"verified_at,omitempty" yaml:"verified_at,omitempty"`
	VerifiedBy  string          `json:"verified_by,omitempty" yaml:"verified_by,omitempty"`
	Evidence    string          `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}

// CoverageReport contains aggregated coverage statistics for initiative criteria.
type CoverageReport struct {
	Total     int          `json:"total"`
	Uncovered int          `json:"uncovered"`
	Covered   int          `json:"covered"`
	Satisfied int          `json:"satisfied"`
	Regressed int          `json:"regressed"`
	Criteria  []*Criterion `json:"criteria"`
}

// ValidateCriterionStatus checks if a status string is valid.
func ValidateCriterionStatus(status string) error {
	switch status {
	case CriterionStatusUncovered, CriterionStatusCovered,
		CriterionStatusSatisfied, CriterionStatusRegressed:
		return nil
	default:
		return fmt.Errorf("invalid criterion status %q", status)
	}
}

// RecomputeCriterionSeq recalculates the criterion sequence number from existing criteria.
// Call this after loading criteria from the database to ensure AddCriterion generates
// correct IDs.
func (i *Initiative) RecomputeCriterionSeq() {
	maxSeq := 0
	for _, c := range i.Criteria {
		var seq int
		if _, err := fmt.Sscanf(c.ID, "AC-%d", &seq); err == nil && seq > maxSeq {
			maxSeq = seq
		}
	}
	i.criterionSeq = maxSeq
}

// AddCriterion adds a new acceptance criterion with an auto-generated ID.
func (i *Initiative) AddCriterion(description string) {
	i.criterionSeq++
	id := fmt.Sprintf("AC-%03d", i.criterionSeq)

	i.Criteria = append(i.Criteria, &Criterion{
		ID:          id,
		Description: description,
		TaskIDs:     []string{},
		Status:      CriterionStatusUncovered,
	})
	i.UpdatedAt = time.Now()
}

// GetCriterion returns a criterion by ID, or nil if not found.
func (i *Initiative) GetCriterion(id string) *Criterion {
	for _, c := range i.Criteria {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// RemoveCriterion removes a criterion by ID. Returns true if found and removed.
func (i *Initiative) RemoveCriterion(id string) bool {
	for idx, c := range i.Criteria {
		if c.ID == id {
			i.Criteria = append(i.Criteria[:idx], i.Criteria[idx+1:]...)
			i.UpdatedAt = time.Now()
			return true
		}
	}
	return false
}

// MapCriterionToTask maps a task ID to a criterion.
// Transitions status from uncovered to covered. Duplicate mappings are idempotent.
func (i *Initiative) MapCriterionToTask(criterionID, taskID string) error {
	c := i.GetCriterion(criterionID)
	if c == nil {
		return fmt.Errorf("criterion %s not found", criterionID)
	}

	// Check for duplicate
	for _, existing := range c.TaskIDs {
		if existing == taskID {
			return nil
		}
	}

	c.TaskIDs = append(c.TaskIDs, taskID)

	// Transition from uncovered to covered
	if c.Status == CriterionStatusUncovered {
		c.Status = CriterionStatusCovered
	}

	i.UpdatedAt = time.Now()
	return nil
}

// VerifyCriterion sets the verification status and evidence for a criterion.
func (i *Initiative) VerifyCriterion(id string, status CriterionStatus, evidence string) error {
	if err := ValidateCriterionStatus(status); err != nil {
		return err
	}

	c := i.GetCriterion(id)
	if c == nil {
		return fmt.Errorf("criterion %s not found", id)
	}

	c.Status = status
	c.Evidence = evidence
	c.VerifiedAt = time.Now().Format(time.RFC3339)
	if c.VerifiedBy == "" {
		c.VerifiedBy = "orc"
	}

	i.UpdatedAt = time.Now()
	return nil
}

// VerifyAllCriteria marks all criteria as verified by the given author.
// Returns all criteria after verification.
func (i *Initiative) VerifyAllCriteria(author string) []*Criterion {
	now := time.Now().Format(time.RFC3339)
	for _, c := range i.Criteria {
		c.VerifiedBy = author
		c.VerifiedAt = now
	}
	i.UpdatedAt = time.Now()
	return i.Criteria
}

// GetUncoveredCriteria returns all criteria with uncovered status.
func (i *Initiative) GetUncoveredCriteria() []*Criterion {
	var result []*Criterion
	for _, c := range i.Criteria {
		if c.Status == CriterionStatusUncovered {
			result = append(result, c)
		}
	}
	return result
}

// GetCoverageReport returns aggregated coverage statistics.
func (i *Initiative) GetCoverageReport() CoverageReport {
	report := CoverageReport{
		Criteria: make([]*Criterion, 0, len(i.Criteria)),
	}

	for _, c := range i.Criteria {
		report.Total++
		switch c.Status {
		case CriterionStatusUncovered:
			report.Uncovered++
		case CriterionStatusCovered:
			report.Covered++
		case CriterionStatusSatisfied:
			report.Satisfied++
		case CriterionStatusRegressed:
			report.Regressed++
		}
		report.Criteria = append(report.Criteria, c)
	}

	return report
}
