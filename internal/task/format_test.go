package task

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		// Zero duration
		{"zero", 0, "0s"},

		// Seconds only
		{"1 second", time.Second, "1s"},
		{"45 seconds", 45 * time.Second, "45s"},
		{"59 seconds", 59 * time.Second, "59s"},

		// Minutes only
		{"1 minute", time.Minute, "1m"},
		{"30 minutes", 30 * time.Minute, "30m"},
		{"59 minutes", 59 * time.Minute, "59m"},

		// Minutes and seconds
		{"1 minute 30 seconds", time.Minute + 30*time.Second, "1m 30s"},
		{"5 minutes 1 second", 5*time.Minute + time.Second, "5m 1s"},

		// Hours only
		{"1 hour", time.Hour, "1h"},
		{"5 hours", 5 * time.Hour, "5h"},
		{"23 hours", 23 * time.Hour, "23h"},

		// Hours and minutes
		{"1 hour 15 minutes", time.Hour + 15*time.Minute, "1h 15m"},
		{"2 hours 30 minutes", 2*time.Hour + 30*time.Minute, "2h 30m"},

		// Hours, minutes, and seconds (shows only hours and minutes)
		{"1h 15m 30s - omits seconds", time.Hour + 15*time.Minute + 30*time.Second, "1h 15m"},

		// Days
		{"1 day", 24 * time.Hour, "1d"},
		{"2 days", 48 * time.Hour, "2d"},
		{"1 day 3 hours", 24*time.Hour + 3*time.Hour, "1d 3h"},
		{"3 days 12 hours", 3*24*time.Hour + 12*time.Hour, "3d 12h"},

		// Negative durations
		{"negative 5 seconds", -5 * time.Second, "-5s"},
		{"negative 1 minute", -time.Minute, "-1m"},
		{"negative 2h 15m", -2*time.Hour - 15*time.Minute, "-2h 15m"},
		{"negative 1 day", -24 * time.Hour, "-1d"},

		// Sub-second durations (rounded down to 0s)
		{"500 milliseconds", 500 * time.Millisecond, "0s"},
		{"999 milliseconds", 999 * time.Millisecond, "0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}
