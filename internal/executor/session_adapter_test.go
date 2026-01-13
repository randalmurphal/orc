package executor

import (
	"testing"
)

func TestTokenUsageEffectiveInputTokens(t *testing.T) {
	tests := []struct {
		name     string
		usage    TokenUsage
		expected int
	}{
		{
			name: "all sources",
			usage: TokenUsage{
				InputTokens:              100,
				CacheCreationInputTokens: 500,
				CacheReadInputTokens:     200,
			},
			expected: 800, // 100 + 500 + 200
		},
		{
			name: "no cache tokens",
			usage: TokenUsage{
				InputTokens:              100,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     0,
			},
			expected: 100,
		},
		{
			name: "only cache read (bug scenario)",
			usage: TokenUsage{
				InputTokens:              5,
				CacheCreationInputTokens: 0,
				CacheReadInputTokens:     10000,
			},
			expected: 10005, // Raw InputTokens=5 misleadingly low; effective shows true size
		},
		{
			name: "typical cached session",
			usage: TokenUsage{
				InputTokens:              56,    // Raw appears tiny
				CacheCreationInputTokens: 8000,  // Initial cache creation
				CacheReadInputTokens:     20000, // System prompt etc from cache
			},
			expected: 28056, // Actual context size
		},
		{
			name: "empty",
			usage: TokenUsage{},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.usage.EffectiveInputTokens()
			if result != tt.expected {
				t.Errorf("EffectiveInputTokens() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestTokenUsageEffectiveTotalTokens(t *testing.T) {
	tests := []struct {
		name     string
		usage    TokenUsage
		expected int
	}{
		{
			name: "full context with output",
			usage: TokenUsage{
				InputTokens:              100,
				OutputTokens:             500,
				CacheCreationInputTokens: 1000,
				CacheReadInputTokens:     2000,
			},
			expected: 3600, // (100+1000+2000) + 500
		},
		{
			name: "no cache",
			usage: TokenUsage{
				InputTokens:  100,
				OutputTokens: 50,
			},
			expected: 150,
		},
		{
			name: "realistic phase execution",
			usage: TokenUsage{
				InputTokens:              56,
				OutputTokens:             13259,
				CacheCreationInputTokens: 8161,
				CacheReadInputTokens:     19191,
			},
			// effective input = 56 + 8161 + 19191 = 27408
			// effective total = 27408 + 13259 = 40667
			expected: 40667,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.usage.EffectiveTotalTokens()
			if result != tt.expected {
				t.Errorf("EffectiveTotalTokens() = %d, want %d", result, tt.expected)
			}
		})
	}
}
