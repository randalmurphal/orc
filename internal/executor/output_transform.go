package executor

import (
	"fmt"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/internal/variable"
)

// ApplyOutputTransform applies an output transformation to the given input.
// It supports three transform types:
//   - "format_findings": Parses JSON review findings and formats them using FormatFindingsForRound2
//   - "json_extract": Extracts a JSON field using ExtractPath (gjson syntax)
//   - "passthrough": Returns input unchanged
//
// If cfg is nil, it behaves as passthrough (returns input unchanged).
func ApplyOutputTransform(cfg *db.OutputTransformConfig, input string) (string, error) {
	// Nil config = passthrough
	if cfg == nil {
		return input, nil
	}

	switch cfg.Type {
	case "passthrough":
		return input, nil

	case "format_findings":
		// Parse the JSON input as review findings
		findings, err := ParseReviewFindings(input)
		if err != nil {
			return "", fmt.Errorf("format_findings transform: %w", err)
		}
		return FormatFindingsForRound2(findings), nil

	case "json_extract":
		// Extract a field from JSON using gjson path
		return variable.ExtractJSONPath(input, cfg.ExtractPath), nil

	default:
		return "", fmt.Errorf("unknown output transform type: %q", cfg.Type)
	}
}

// applyOutputTransform applies an output transformation using the SourceVar to get input
// from the variable set. Used internally by the loop execution.
//
// If cfg is nil (no transform configured), returns the raw REVIEW_OUTPUT variable value
// as a fallback for review loop compatibility.
func applyOutputTransform(cfg *db.OutputTransformConfig, vars variable.VariableSet, rctx *variable.ResolutionContext) (string, error) {
	if cfg == nil {
		// No transform configured - return REVIEW_OUTPUT raw if available (fallback)
		if sourceVal, ok := vars["REVIEW_OUTPUT"]; ok {
			return sourceVal, nil
		}
		return "", nil
	}

	// Get input from SourceVar
	input, ok := vars[cfg.SourceVar]
	if !ok {
		// Try to get from prior outputs in rctx (for phase_output sources)
		if rctx != nil && rctx.PriorOutputs != nil {
			// SourceVar might reference a phase name
			if output, found := rctx.PriorOutputs[cfg.SourceVar]; found {
				input = output
			}
		}
	}

	return ApplyOutputTransform(cfg, input)
}
