package bootstrap

import (
	"fmt"
	"io"

	"github.com/randalmurphal/orc/internal/hosting"
)

// HostingVerificationResult contains the results of hosting verification during init.
type HostingVerificationResult struct {
	GitRepoFound    bool
	RemoteFound     bool
	RemoteURL       string
	Provider        hosting.ProviderType
	ProviderName    string
	IsSelfHosted    bool
	BaseURL         string
	TokenEnvVar     string
	TokenExists     bool
	TokenValid      bool
	TokenUsername   string
	TokenValidationError string
	AnthropicKeySet bool
	AutoMergeEnabled   bool
	AutoMergeSupported bool
}

// PrintHostingVerification outputs the verification result to the writer.
// Uses ✓/✗ prefixes to indicate status of each check.
func PrintHostingVerification(w io.Writer, result *HostingVerificationResult) {
	// Git repository check
	if result.GitRepoFound {
		_, _ = fmt.Fprintln(w, "✓ Git repository detected")
	} else {
		_, _ = fmt.Fprintln(w, "✗ Git repository not found")
		return // No point continuing without git
	}

	// Remote check
	if result.RemoteFound {
		_, _ = fmt.Fprintf(w, "✓ Remote 'origin' found: %s\n", result.RemoteURL)
	} else {
		_, _ = fmt.Fprintln(w, "✗ No git remote configured")
		_, _ = fmt.Fprintln(w, "  Run: git remote add origin <url>")
		return
	}

	// Provider check
	if result.Provider != hosting.ProviderUnknown {
		if result.IsSelfHosted {
			_, _ = fmt.Fprintf(w, "✓ Hosting provider: %s (%s)\n", result.ProviderName, result.BaseURL)
		} else {
			_, _ = fmt.Fprintf(w, "✓ Hosting provider: %s\n", result.ProviderName)
		}
	} else {
		_, _ = fmt.Fprintln(w, "✗ Could not detect hosting provider from remote URL")
	}

	// Token env var check
	if result.TokenEnvVar != "" {
		if result.TokenExists {
			_, _ = fmt.Fprintf(w, "✓ %s found\n", result.TokenEnvVar)
		} else {
			_, _ = fmt.Fprintf(w, "✗ %s not set\n", result.TokenEnvVar)
			_, _ = fmt.Fprintf(w, "  This is required for orc run to create PRs\n")
		}
	}

	// Token validation check
	if result.TokenExists {
		if result.TokenValid {
			_, _ = fmt.Fprintf(w, "✓ Token validated (user: %s)\n", result.TokenUsername)
		} else {
			_, _ = fmt.Fprintln(w, "✗ Token validation failed")
			if result.TokenValidationError != "" {
				_, _ = fmt.Fprintf(w, "  Error: %s\n", result.TokenValidationError)
			}
		}
	}

	// Anthropic API key check
	if result.AnthropicKeySet {
		_, _ = fmt.Fprintln(w, "✓ ANTHROPIC_API_KEY set")
	} else {
		_, _ = fmt.Fprintln(w, "✗ ANTHROPIC_API_KEY not set")
		_, _ = fmt.Fprintln(w, "  This is required for orc run to execute Claude")
	}

	// Auto-merge warning for GitHub
	if result.AutoMergeEnabled && !result.AutoMergeSupported {
		_, _ = fmt.Fprintln(w, "")
		_, _ = fmt.Fprintln(w, "Warning: auto_merge is enabled but not supported on GitHub")
		_, _ = fmt.Fprintln(w, "  GitHub auto-merge requires GraphQL API (not implemented)")
		_, _ = fmt.Fprintln(w, "  Consider using completion.ci.merge_on_ci_pass instead")
	}
}
