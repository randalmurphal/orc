package config

import (
	"strings"
	"testing"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *Config
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "default config is valid",
			cfg:     Default(),
			wantErr: false,
		},
		// Visibility validation
		{
			name: "valid visibility: all",
			cfg: &Config{
				Team: TeamConfig{Visibility: "all"},
			},
			wantErr: false,
		},
		{
			name: "valid visibility: assigned",
			cfg: &Config{
				Team: TeamConfig{Visibility: "assigned"},
			},
			wantErr: false,
		},
		{
			name: "valid visibility: owned",
			cfg: &Config{
				Team: TeamConfig{Visibility: "owned"},
			},
			wantErr: false,
		},
		{
			name: "empty visibility is allowed",
			cfg: &Config{
				Team: TeamConfig{Visibility: ""},
			},
			wantErr: false,
		},
		{
			name: "invalid visibility returns error",
			cfg: &Config{
				Team: TeamConfig{Visibility: "invalid"},
			},
			wantErr:   true,
			errSubstr: "invalid team.visibility",
		},
		{
			name: "invalid visibility: typo",
			cfg: &Config{
				Team: TeamConfig{Visibility: "alll"},
			},
			wantErr:   true,
			errSubstr: "team.visibility",
		},
		// Mode validation
		{
			name: "valid mode: local",
			cfg: &Config{
				Team: TeamConfig{Mode: "local"},
			},
			wantErr: false,
		},
		{
			name: "valid mode: shared_db",
			cfg: &Config{
				Team: TeamConfig{Mode: "shared_db"},
			},
			wantErr: false,
		},
		{
			name: "valid mode: sync_server",
			cfg: &Config{
				Team: TeamConfig{Mode: "sync_server"},
			},
			wantErr: false,
		},
		{
			name: "empty mode is allowed",
			cfg: &Config{
				Team: TeamConfig{Mode: ""},
			},
			wantErr: false,
		},
		{
			name: "invalid mode returns error",
			cfg: &Config{
				Team: TeamConfig{Mode: "invalid"},
			},
			wantErr:   true,
			errSubstr: "invalid team.mode",
		},
		{
			name: "invalid mode: typo",
			cfg: &Config{
				Team: TeamConfig{Mode: "loca"},
			},
			wantErr:   true,
			errSubstr: "team.mode",
		},
		// Combined validation
		{
			name: "valid visibility and mode together",
			cfg: &Config{
				Team: TeamConfig{
					Visibility: "assigned",
					Mode:       "shared_db",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid visibility with valid mode fails on visibility",
			cfg: &Config{
				Team: TeamConfig{
					Visibility: "bad",
					Mode:       "local",
				},
			},
			wantErr:   true,
			errSubstr: "visibility",
		},
		{
			name: "valid visibility with invalid mode fails on mode",
			cfg: &Config{
				Team: TeamConfig{
					Visibility: "all",
					Mode:       "bad",
				},
			},
			wantErr:   true,
			errSubstr: "mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errSubstr != "" {
				if err == nil {
					t.Errorf("Validate() expected error containing %q, got nil", tt.errSubstr)
				} else if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("Validate() error = %q, want error containing %q", err.Error(), tt.errSubstr)
				}
			}
		})
	}
}

func TestValidVisibilities(t *testing.T) {
	// Ensure ValidVisibilities contains expected values
	expected := []string{"all", "assigned", "owned"}
	if len(ValidVisibilities) != len(expected) {
		t.Errorf("ValidVisibilities length = %d, want %d", len(ValidVisibilities), len(expected))
	}

	for _, v := range expected {
		found := false
		for _, valid := range ValidVisibilities {
			if valid == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidVisibilities missing %q", v)
		}
	}
}

func TestValidModes(t *testing.T) {
	// Ensure ValidModes contains expected values
	expected := []string{"local", "shared_db", "sync_server"}
	if len(ValidModes) != len(expected) {
		t.Errorf("ValidModes length = %d, want %d", len(ValidModes), len(expected))
	}

	for _, v := range expected {
		found := false
		for _, valid := range ValidModes {
			if valid == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ValidModes missing %q", v)
		}
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		search string
		want   bool
	}{
		{
			name:   "found in slice",
			slice:  []string{"a", "b", "c"},
			search: "b",
			want:   true,
		},
		{
			name:   "not found in slice",
			slice:  []string{"a", "b", "c"},
			search: "d",
			want:   false,
		},
		{
			name:   "empty slice",
			slice:  []string{},
			search: "a",
			want:   false,
		},
		{
			name:   "first element",
			slice:  []string{"a", "b", "c"},
			search: "a",
			want:   true,
		},
		{
			name:   "last element",
			slice:  []string{"a", "b", "c"},
			search: "c",
			want:   true,
		},
		{
			name:   "case sensitive",
			slice:  []string{"All", "Assigned"},
			search: "all",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := contains(tt.slice, tt.search)
			if got != tt.want {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.slice, tt.search, got, tt.want)
			}
		})
	}
}

func TestConfig_Validate_ErrorMessages(t *testing.T) {
	// Test that error messages contain the allowed values
	cfg := &Config{
		Team: TeamConfig{Visibility: "bad"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid visibility")
	}

	// Error should mention allowed values
	errStr := err.Error()
	for _, v := range ValidVisibilities {
		if !strings.Contains(errStr, v) {
			t.Errorf("error message should contain allowed value %q, got: %s", v, errStr)
		}
	}

	// Test mode error message
	cfg = &Config{
		Team: TeamConfig{Mode: "bad"},
	}
	err = cfg.Validate()
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}

	errStr = err.Error()
	for _, v := range ValidModes {
		if !strings.Contains(errStr, v) {
			t.Errorf("error message should contain allowed value %q, got: %s", v, errStr)
		}
	}
}
