package executor

import (
	"sync"

	"github.com/randalmurphal/orc/internal/hosting"
)

// autoMergeWarningState tracks whether the warning has been shown.
// This ensures the warning is only logged once per executor lifetime.
var (
	autoMergeWarningMu    sync.Mutex
	autoMergeWarningShown = make(map[*WorkflowExecutor]bool)
)

// checkAutoMergeWarning checks if auto_merge is enabled with GitHub
// and logs a warning if so, since GitHub auto-merge requires GraphQL
// which is not supported.
func (we *WorkflowExecutor) checkAutoMergeWarning(provider hosting.ProviderType) {
	// Only warn if auto_merge is enabled
	if we.orcConfig == nil || !we.orcConfig.Completion.PR.AutoMerge {
		return
	}

	// Only warn for GitHub - GitLab supports auto-merge via REST
	if provider != hosting.ProviderGitHub {
		return
	}

	// Only warn once per executor instance
	autoMergeWarningMu.Lock()
	if autoMergeWarningShown[we] {
		autoMergeWarningMu.Unlock()
		return
	}
	autoMergeWarningShown[we] = true
	autoMergeWarningMu.Unlock()

	we.logger.Warn("auto_merge is enabled but not supported on GitHub",
		"reason", "GitHub auto-merge requires GraphQL API (not REST)",
		"suggestion", "Consider using completion.ci.merge_on_ci_pass instead",
	)
}

// checkAutoMergeWarningFromConfig checks the auto-merge warning using
// the provider from the configuration.
func (we *WorkflowExecutor) checkAutoMergeWarningFromConfig() {
	if we.orcConfig == nil {
		return
	}

	// Get provider from config
	providerStr := we.orcConfig.Hosting.Provider
	if providerStr == "" || providerStr == "auto" {
		// Can't determine provider from config, skip warning
		return
	}

	provider := hosting.ProviderType(providerStr)
	we.checkAutoMergeWarning(provider)
}
