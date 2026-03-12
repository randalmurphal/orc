package executor

import (
	"fmt"
	"strings"
)

// PlanCompletionSchema is the JSON schema for the plan phase.
const PlanCompletionSchema = `{
	"type": "object",
	"properties": {
		"status": {
			"type": "string",
			"enum": ["complete", "blocked", "continue"]
		},
		"reason": {
			"type": "string"
		},
		"summary": {
			"type": "string"
		},
		"content": {
			"type": "string"
		},
		"quality_checklist": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"id": {"type": "string"},
					"check": {"type": "string"},
					"passed": {"type": "boolean"}
				},
				"required": ["id", "check", "passed"]
			}
		},
		"invariants": {
			"type": "array",
			"items": {"type": "string"}
		},
		"risk_assessment": {
			"type": "object",
			"properties": {
				"level": {
					"type": "string",
					"enum": ["low", "medium", "high", "critical"]
				},
				"tags": {
					"type": "array",
					"items": {"type": "string"}
				},
				"rationale": {"type": "string"},
				"requires_human_gate": {"type": "boolean"},
				"requires_browser_qa": {"type": "boolean"}
			}
		},
		"operational_notes": {
			"type": "object",
			"properties": {
				"rollback": {"type": "string"},
				"migration": {"type": "string"},
				"observability": {
					"type": "array",
					"items": {"type": "string"}
				},
				"external_dependencies": {
					"type": "array",
					"items": {"type": "string"}
				},
				"non_goals": {
					"type": "array",
					"items": {"type": "string"}
				}
			}
		},
		"verification_plan": {
			"type": "object",
			"properties": {
				"build": {"type": "string"},
				"lint": {"type": "string"},
				"tests": {
					"type": "array",
					"items": {"type": "string"}
				},
				"e2e": {"type": "string"}
			}
		},
		"canonical_associations": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"name": {"type": "string"},
					"source_of_truth": {"type": "string"},
					"writer_paths": {"type": "array", "items": {"type": "string"}},
					"reader_paths": {"type": "array", "items": {"type": "string"}},
					"mirrors": {"type": "array", "items": {"type": "string"}}
				},
				"required": ["name", "source_of_truth", "writer_paths", "reader_paths", "mirrors"]
			}
		},
		"provenance_variants": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"path": {"type": "string"},
					"valid_variants": {"type": "array", "items": {"type": "string"}},
					"notes": {"type": "string"}
				},
				"required": ["path", "valid_variants", "notes"]
			}
		},
		"ui_invalidation_paths": {
			"type": "array",
			"items": {
				"type": "object",
				"properties": {
					"surface": {"type": "string"},
					"update_sources": {"type": "array", "items": {"type": "string"}},
					"reset_triggers": {"type": "array", "items": {"type": "string"}},
					"stale_response_handling": {"type": "string"},
					"project_scope_key": {"type": "string"}
				},
				"required": ["surface", "update_sources", "reset_triggers", "stale_response_handling", "project_scope_key"]
			}
		}
	},
	"required": ["status"]
}`

type PlanQualityChecklistItem struct {
	ID     string `json:"id"`
	Check  string `json:"check"`
	Passed bool   `json:"passed"`
}

type PlanRiskAssessment struct {
	Level             string   `json:"level,omitempty"`
	Tags              []string `json:"tags,omitempty"`
	Rationale         string   `json:"rationale,omitempty"`
	RequiresHumanGate bool     `json:"requires_human_gate,omitempty"`
	RequiresBrowserQA bool     `json:"requires_browser_qa,omitempty"`
}

type PlanOperationalNotes struct {
	Rollback             string   `json:"rollback,omitempty"`
	Migration            string   `json:"migration,omitempty"`
	Observability        []string `json:"observability,omitempty"`
	ExternalDependencies []string `json:"external_dependencies,omitempty"`
	NonGoals             []string `json:"non_goals,omitempty"`
}

type PlanVerificationPlan struct {
	Build string   `json:"build,omitempty"`
	Lint  string   `json:"lint,omitempty"`
	Tests []string `json:"tests,omitempty"`
	E2E   string   `json:"e2e,omitempty"`
}

type PlanCanonicalAssociation struct {
	Name          string   `json:"name"`
	SourceOfTruth string   `json:"source_of_truth,omitempty"`
	WriterPaths   []string `json:"writer_paths,omitempty"`
	ReaderPaths   []string `json:"reader_paths,omitempty"`
	Mirrors       []string `json:"mirrors,omitempty"`
}

type PlanProvenanceVariant struct {
	Path          string   `json:"path"`
	ValidVariants []string `json:"valid_variants,omitempty"`
	Notes         string   `json:"notes,omitempty"`
}

type PlanUIInvalidationPath struct {
	Surface               string   `json:"surface"`
	UpdateSources         []string `json:"update_sources,omitempty"`
	ResetTriggers         []string `json:"reset_triggers,omitempty"`
	StaleResponseHandling string   `json:"stale_response_handling,omitempty"`
	ProjectScopeKey       string   `json:"project_scope_key,omitempty"`
}

// PlanResponse extends the standard content-producing response with policy
// signals consumed by downstream phases and gates.
type PlanResponse struct {
	Status                string                     `json:"status"`
	Reason                string                     `json:"reason,omitempty"`
	Summary               string                     `json:"summary,omitempty"`
	Content               string                     `json:"content,omitempty"`
	QualityChecklist      []PlanQualityChecklistItem `json:"quality_checklist,omitempty"`
	Invariants            []string                   `json:"invariants,omitempty"`
	RiskAssessment        *PlanRiskAssessment        `json:"risk_assessment,omitempty"`
	OperationalNotes      *PlanOperationalNotes      `json:"operational_notes,omitempty"`
	VerificationPlan      *PlanVerificationPlan      `json:"verification_plan,omitempty"`
	CanonicalAssociations []PlanCanonicalAssociation `json:"canonical_associations,omitempty"`
	ProvenanceVariants    []PlanProvenanceVariant    `json:"provenance_variants,omitempty"`
	UIInvalidationPaths   []PlanUIInvalidationPath   `json:"ui_invalidation_paths,omitempty"`
}

func ParsePlanResponse(content string) (*PlanResponse, error) {
	var resp PlanResponse
	if err := unmarshalWithFallback(strings.TrimSpace(content), &resp); err != nil {
		return nil, fmt.Errorf("invalid plan response JSON: %w", err)
	}

	switch resp.Status {
	case "complete", "blocked", "continue":
	default:
		return nil, fmt.Errorf("invalid plan status: %q (expected complete, blocked, or continue)", resp.Status)
	}

	if resp.QualityChecklist == nil {
		resp.QualityChecklist = []PlanQualityChecklistItem{}
	}
	if resp.Invariants == nil {
		resp.Invariants = []string{}
	}
	if resp.RiskAssessment != nil && resp.RiskAssessment.Tags == nil {
		resp.RiskAssessment.Tags = []string{}
	}
	if resp.OperationalNotes != nil {
		if resp.OperationalNotes.Observability == nil {
			resp.OperationalNotes.Observability = []string{}
		}
		if resp.OperationalNotes.ExternalDependencies == nil {
			resp.OperationalNotes.ExternalDependencies = []string{}
		}
		if resp.OperationalNotes.NonGoals == nil {
			resp.OperationalNotes.NonGoals = []string{}
		}
	}
	if resp.VerificationPlan != nil && resp.VerificationPlan.Tests == nil {
		resp.VerificationPlan.Tests = []string{}
	}
	if resp.CanonicalAssociations == nil {
		resp.CanonicalAssociations = []PlanCanonicalAssociation{}
	}
	for i := range resp.CanonicalAssociations {
		if resp.CanonicalAssociations[i].WriterPaths == nil {
			resp.CanonicalAssociations[i].WriterPaths = []string{}
		}
		if resp.CanonicalAssociations[i].ReaderPaths == nil {
			resp.CanonicalAssociations[i].ReaderPaths = []string{}
		}
		if resp.CanonicalAssociations[i].Mirrors == nil {
			resp.CanonicalAssociations[i].Mirrors = []string{}
		}
	}
	if resp.ProvenanceVariants == nil {
		resp.ProvenanceVariants = []PlanProvenanceVariant{}
	}
	for i := range resp.ProvenanceVariants {
		if resp.ProvenanceVariants[i].ValidVariants == nil {
			resp.ProvenanceVariants[i].ValidVariants = []string{}
		}
	}
	if resp.UIInvalidationPaths == nil {
		resp.UIInvalidationPaths = []PlanUIInvalidationPath{}
	}
	for i := range resp.UIInvalidationPaths {
		if resp.UIInvalidationPaths[i].UpdateSources == nil {
			resp.UIInvalidationPaths[i].UpdateSources = []string{}
		}
		if resp.UIInvalidationPaths[i].ResetTriggers == nil {
			resp.UIInvalidationPaths[i].ResetTriggers = []string{}
		}
	}

	return &resp, nil
}

func PlanRequiresBrowserQA(content string) bool {
	resp, err := ParsePlanResponse(content)
	if err != nil || resp.RiskAssessment == nil {
		return false
	}
	return resp.RiskAssessment.RequiresBrowserQA
}

func PlanRequiresHumanGate(content string, threshold string) bool {
	resp, err := ParsePlanResponse(content)
	if err != nil || resp.RiskAssessment == nil {
		return false
	}
	if resp.RiskAssessment.RequiresHumanGate {
		return true
	}
	return riskLevelMeetsOrExceeds(resp.RiskAssessment.Level, threshold)
}

func riskLevelMeetsOrExceeds(level string, threshold string) bool {
	order := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}

	levelRank, levelOK := order[strings.ToLower(strings.TrimSpace(level))]
	thresholdRank, thresholdOK := order[strings.ToLower(strings.TrimSpace(threshold))]
	if !levelOK || !thresholdOK {
		return false
	}

	return levelRank >= thresholdRank
}
