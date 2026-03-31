package executor

import "github.com/randalmurphal/orc/internal/config"

// getFinalizeConfig returns the finalize configuration with defaults.
func (e *FinalizeExecutor) getFinalizeConfig() config.FinalizeConfig {
	if e.orcConfig == nil {
		return config.FinalizeConfig{
			Enabled:     true,
			AutoTrigger: true,
			Sync: config.FinalizeSyncConfig{
				Strategy: config.FinalizeSyncMerge,
			},
			ConflictResolution: config.ConflictResolutionConfig{
				Enabled: true,
			},
			RiskAssessment: config.RiskAssessmentConfig{
				Enabled:           true,
				ReReviewThreshold: "high",
			},
			Gates: config.FinalizeGatesConfig{
				PreMerge: "auto",
			},
		}
	}
	return e.orcConfig.Completion.Finalize
}
