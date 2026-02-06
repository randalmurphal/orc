// Tests for TASK-021: BriefConfig in the config system.
//
// SC-5: Config supports brief.max_tokens and brief.stale_threshold fields
// via the standard config hierarchy.
package config

import (
	"testing"
)

// =============================================================================
// SC-5: Config supports brief.max_tokens and brief.stale_threshold
//
// BriefConfig should have sensible defaults (max_tokens=3000, stale_threshold=3)
// and be accessible via the standard GetValue/SetValue reflection-based routing.
// =============================================================================

func TestConfig_BriefSettings_Defaults(t *testing.T) {
	cfg := Default()

	// Verify BriefConfig has expected defaults
	if cfg.Brief.MaxTokens != 3000 {
		t.Errorf("Brief.MaxTokens default = %d, want 3000", cfg.Brief.MaxTokens)
	}

	if cfg.Brief.StaleThreshold != 3 {
		t.Errorf("Brief.StaleThreshold default = %d, want 3", cfg.Brief.StaleThreshold)
	}
}

func TestConfig_BriefSettings_GetValue(t *testing.T) {
	cfg := Default()

	// GetValue should traverse into Brief struct
	maxTokens, err := cfg.GetValue("brief.max_tokens")
	if err != nil {
		t.Fatalf("GetValue(brief.max_tokens) error: %v", err)
	}
	if maxTokens != "3000" {
		t.Errorf("GetValue(brief.max_tokens) = %q, want %q", maxTokens, "3000")
	}

	staleThreshold, err := cfg.GetValue("brief.stale_threshold")
	if err != nil {
		t.Fatalf("GetValue(brief.stale_threshold) error: %v", err)
	}
	if staleThreshold != "3" {
		t.Errorf("GetValue(brief.stale_threshold) = %q, want %q", staleThreshold, "3")
	}
}

func TestConfig_BriefSettings_SetValue(t *testing.T) {
	cfg := Default()

	// SetValue should update Brief.MaxTokens
	if err := cfg.SetValue("brief.max_tokens", "5000"); err != nil {
		t.Fatalf("SetValue(brief.max_tokens, 5000) error: %v", err)
	}
	if cfg.Brief.MaxTokens != 5000 {
		t.Errorf("Brief.MaxTokens after SetValue = %d, want 5000", cfg.Brief.MaxTokens)
	}

	// SetValue should update Brief.StaleThreshold
	if err := cfg.SetValue("brief.stale_threshold", "10"); err != nil {
		t.Fatalf("SetValue(brief.stale_threshold, 10) error: %v", err)
	}
	if cfg.Brief.StaleThreshold != 10 {
		t.Errorf("Brief.StaleThreshold after SetValue = %d, want 10", cfg.Brief.StaleThreshold)
	}
}

func TestConfig_BriefSettings_SetValue_InvalidValues(t *testing.T) {
	cfg := Default()

	// Non-numeric value should fail
	if err := cfg.SetValue("brief.max_tokens", "not-a-number"); err == nil {
		t.Error("SetValue(brief.max_tokens, not-a-number) should return error")
	}

	// Verify original value unchanged after failed set
	if cfg.Brief.MaxTokens != 3000 {
		t.Errorf("Brief.MaxTokens should be unchanged after failed SetValue, got %d", cfg.Brief.MaxTokens)
	}
}

func TestConfig_BriefSettings_UnknownKey(t *testing.T) {
	cfg := Default()

	// Unknown sub-key should fail
	_, err := cfg.GetValue("brief.nonexistent")
	if err == nil {
		t.Error("GetValue(brief.nonexistent) should return error for unknown key")
	}
}
