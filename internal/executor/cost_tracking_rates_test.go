package executor

import (
	"math"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
)

func TestEstimateTokenCostUSD_KnownProviders(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
		input    int64
		output   int64
		want     float64
	}{
		{"claude opus 1M each", "claude", "opus", 1_000_000, 1_000_000, 30.0},
		{"claude sonnet 1M each", "claude", "sonnet", 1_000_000, 1_000_000, 18.0},
		{"claude haiku 1M each", "claude", "haiku", 1_000_000, 1_000_000, 6.0},
		{"codex gpt-5 1M each", "codex", "gpt-5", 1_000_000, 1_000_000, 15.75},
		{"codex gpt-4.1 1M each", "codex", "gpt-4.1", 1_000_000, 1_000_000, 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EstimateTokenCostUSD(tt.provider, tt.model, tt.input, tt.output, 0, 0)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("EstimateTokenCostUSD(%q, %q, %d, %d) = %f, want %f",
					tt.provider, tt.model, tt.input, tt.output, got, tt.want)
			}
		})
	}
}

func TestEstimateTokenCostUSD_UnknownProviderReturnsZero(t *testing.T) {
	got := EstimateTokenCostUSD("unknown_provider", "any-model", 1_000_000, 1_000_000, 0, 0)
	if got != 0 {
		t.Fatalf("EstimateTokenCostUSD(unknown) = %f, want 0", got)
	}
}

func TestEstimateTokenCostUSD_UnknownModelReturnsZero(t *testing.T) {
	got := EstimateTokenCostUSD("codex", "unknown-model", 1_000_000, 1_000_000, 0, 0)
	if got != 0 {
		t.Fatalf("EstimateTokenCostUSD(codex, unknown-model) = %f, want 0", got)
	}
}

func TestEstimateTokenCostUSD_PrefixModelMatch(t *testing.T) {
	// "gpt-5.3-codex" should match the "gpt-5" rate entry via prefix matching
	got := EstimateTokenCostUSD("codex", "gpt-5.3-codex", 1_000_000, 1_000_000, 0, 0)
	want := 15.75 // gpt-5 rates: 1.75 input + 14.0 output
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("prefix match gpt-5.3-codex = %f, want %f", got, want)
	}
}

func TestEstimateTokenCostUSD_PrefixMatchLongestWins(t *testing.T) {
	// When multiple prefixes match, longest wins
	rates := map[string]map[string]TokenRate{
		"test": {
			"gpt-4":   {Input: 1.0, Output: 1.0},
			"gpt-4.1": {Input: 5.0, Output: 5.0},
		},
	}
	// "gpt-4.1-mini" should match "gpt-4.1" (longer), not "gpt-4"
	got := EstimateTokenCostUSDWithRates(rates, "test", "gpt-4.1-mini", 1_000_000, 1_000_000, 0, 0)
	want := 10.0 // 5.0 + 5.0 from gpt-4.1
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("longest prefix match = %f, want %f", got, want)
	}
}

func TestEstimateTokenCostUSD_CacheTokens(t *testing.T) {
	// Claude opus: cache_read=0.5, cache_write=6.25 per 1M
	got := EstimateTokenCostUSD("claude", "opus", 0, 0, 1_000_000, 1_000_000)
	want := 0.5 + 6.25
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("cache token cost = %f, want %f", got, want)
	}
}

func TestEstimateTokenCostUSDWithRates_CustomRates(t *testing.T) {
	custom := map[string]map[string]TokenRate{
		"codex": {
			"gpt-5": {Input: 1.0, Output: 2.0},
		},
	}

	got := EstimateTokenCostUSDWithRates(custom, "codex", "gpt-5", 1_000_000, 1_000_000, 0, 0)
	want := 3.0
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("EstimateTokenCostUSDWithRates() = %f, want %f", got, want)
	}
}

func TestEstimateTokenCostUSD_LargeTokenCounts(t *testing.T) {
	// Verify int64 doesn't overflow with large token counts
	got := EstimateTokenCostUSD("claude", "sonnet", 10_000_000_000, 5_000_000_000, 0, 0)
	// 10B tokens * $3/1M = $30,000 input + 5B * $15/1M = $75,000 output = $105,000
	want := 105_000.0
	if math.Abs(got-want) > 1e-3 {
		t.Fatalf("large token cost = %f, want %f", got, want)
	}
}

func TestProviderRatesForConfig_NilConfig(t *testing.T) {
	merged := ProviderRatesForConfig(nil)
	got := EstimateTokenCostUSDWithRates(merged, "codex", "gpt-5", 1_000_000, 1_000_000, 0, 0)
	want := 15.75
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("nil config cost = %f, want %f", got, want)
	}
}

func TestProviderRatesForConfig_MergesOverrides(t *testing.T) {
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			Rates: map[string]map[string]config.ProviderRateConfig{
				"codex": {
					"gpt-5": {Input: 1.25, Output: 2.5},
				},
				"custom_provider": {
					"*": {Input: 0.5, Output: 0.5},
				},
			},
		},
	}

	merged := ProviderRatesForConfig(cfg)

	// Overridden codex gpt-5
	got := EstimateTokenCostUSDWithRates(merged, "codex", "gpt-5", 1_000_000, 1_000_000, 0, 0)
	if math.Abs(got-3.75) > 1e-9 {
		t.Fatalf("codex override cost = %f, want 3.75", got)
	}

	// New custom provider with wildcard
	customGot := EstimateTokenCostUSDWithRates(merged, "custom_provider", "any-model", 1_000_000, 1_000_000, 0, 0)
	if math.Abs(customGot-1.0) > 1e-9 {
		t.Fatalf("custom provider wildcard cost = %f, want 1.0", customGot)
	}

	// Default table must remain unchanged after merge
	defaultCost := EstimateTokenCostUSD("codex", "gpt-5", 1_000_000, 1_000_000, 0, 0)
	if math.Abs(defaultCost-15.75) > 1e-9 {
		t.Fatalf("default cost table mutated: got %f, want 15.75", defaultCost)
	}
}

func TestProviderRatesForConfig_PreservesNonOverriddenDefaults(t *testing.T) {
	cfg := &config.Config{
		Providers: config.ProvidersConfig{
			Rates: map[string]map[string]config.ProviderRateConfig{
				"codex": {
					"gpt-5": {Input: 99.0, Output: 99.0},
				},
			},
		},
	}

	merged := ProviderRatesForConfig(cfg)

	// Claude rates should still be present and unmodified
	claudeCost := EstimateTokenCostUSDWithRates(merged, "claude", "opus", 1_000_000, 1_000_000, 0, 0)
	want := 30.0 // 5 + 25
	if math.Abs(claudeCost-want) > 1e-9 {
		t.Fatalf("claude rates changed after codex override: got %f, want %f", claudeCost, want)
	}

	// Non-overridden codex model (gpt-4.1) should remain
	gpt41Cost := EstimateTokenCostUSDWithRates(merged, "codex", "gpt-4.1", 1_000_000, 1_000_000, 0, 0)
	wantGPT41 := 10.0 // 2 + 8
	if math.Abs(gpt41Cost-wantGPT41) > 1e-9 {
		t.Fatalf("gpt-4.1 rates changed after gpt-5 override: got %f, want %f", gpt41Cost, wantGPT41)
	}
}
