package driver

import "testing"

// TestSQLiteDateFormat verifies the SQLite driver returns strftime-based SQL fragments
// for various logical format identifiers.
// Covers SC-1 and SC-6.
func TestSQLiteDateFormat(t *testing.T) {
	t.Parallel()
	drv := NewSQLite()

	tests := []struct {
		name     string
		column   string
		format   string
		wantSub  string // substring that must appear in the result
		wantFull string // exact expected result (empty means use wantSub only)
	}{
		{
			name:     "day format with column",
			column:   "timestamp",
			format:   "day",
			wantFull: "strftime('%Y-%m-%d', timestamp)",
		},
		{
			name:     "week format with column",
			column:   "timestamp",
			format:   "week",
			wantFull: "strftime('%Y-W%W', timestamp)",
		},
		{
			name:     "month format with column",
			column:   "timestamp",
			format:   "month",
			wantFull: "strftime('%Y-%m', timestamp)",
		},
		{
			name:     "rfc3339 format with column",
			column:   "created_at",
			format:   "rfc3339",
			wantFull: "strftime('%Y-%m-%dT%H:%M:%SZ', created_at)",
		},
		{
			name:   "now as column (used by branch.go for formatting current time)",
			column: "'now'",
			format: "rfc3339",
			wantSub: "strftime('%Y-%m-%dT%H:%M:%SZ', 'now')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := drv.DateFormat(tt.column, tt.format)
			if tt.wantFull != "" {
				if got != tt.wantFull {
					t.Errorf("DateFormat(%q, %q) = %q, want %q", tt.column, tt.format, got, tt.wantFull)
				}
			} else if tt.wantSub != "" {
				if got != tt.wantSub {
					t.Errorf("DateFormat(%q, %q) = %q, want to contain %q", tt.column, tt.format, got, tt.wantSub)
				}
			}
		})
	}
}

// TestPostgresDateFormat verifies the PostgreSQL driver returns TO_CHAR-based SQL fragments.
// Covers SC-1 and SC-6.
func TestPostgresDateFormat(t *testing.T) {
	t.Parallel()
	drv := NewPostgres()

	tests := []struct {
		name     string
		column   string
		format   string
		wantFull string
	}{
		{
			name:     "day format with column",
			column:   "timestamp",
			format:   "day",
			wantFull: "TO_CHAR(timestamp, 'YYYY-MM-DD')",
		},
		{
			name:     "week format with column (PostgreSQL uses IW for ISO week)",
			column:   "timestamp",
			format:   "week",
			wantFull: "TO_CHAR(timestamp, 'IYYY-\"W\"IW')",
		},
		{
			name:     "month format with column",
			column:   "timestamp",
			format:   "month",
			wantFull: "TO_CHAR(timestamp, 'YYYY-MM')",
		},
		{
			name:     "rfc3339 format with column",
			column:   "created_at",
			format:   "rfc3339",
			wantFull: "TO_CHAR(created_at, 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')",
		},
		{
			name:     "now as column for branch activity",
			column:   "NOW()",
			format:   "rfc3339",
			wantFull: "TO_CHAR(NOW(), 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := drv.DateFormat(tt.column, tt.format)
			if got != tt.wantFull {
				t.Errorf("DateFormat(%q, %q) = %q, want %q", tt.column, tt.format, got, tt.wantFull)
			}
		})
	}
}

// TestSQLiteDateTrunc verifies the SQLite driver returns strftime-based truncation.
// Covers SC-1 and SC-6.
func TestSQLiteDateTrunc(t *testing.T) {
	t.Parallel()
	drv := NewSQLite()

	tests := []struct {
		name   string
		unit   string
		column string
		want   string
	}{
		{
			name:   "truncate to day",
			unit:   "day",
			column: "timestamp",
			want:   "strftime('%Y-%m-%d', timestamp)",
		},
		{
			name:   "truncate to month",
			unit:   "month",
			column: "created_at",
			want:   "strftime('%Y-%m-01', created_at)",
		},
		{
			name:   "truncate to year",
			unit:   "year",
			column: "timestamp",
			want:   "strftime('%Y-01-01', timestamp)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := drv.DateTrunc(tt.unit, tt.column)
			if got != tt.want {
				t.Errorf("DateTrunc(%q, %q) = %q, want %q", tt.unit, tt.column, got, tt.want)
			}
		})
	}
}

// TestPostgresDateTrunc verifies the PostgreSQL driver returns date_trunc() SQL.
// Covers SC-1 and SC-6.
func TestPostgresDateTrunc(t *testing.T) {
	t.Parallel()
	drv := NewPostgres()

	tests := []struct {
		name   string
		unit   string
		column string
		want   string
	}{
		{
			name:   "truncate to day",
			unit:   "day",
			column: "timestamp",
			want:   "date_trunc('day', timestamp)",
		},
		{
			name:   "truncate to month",
			unit:   "month",
			column: "created_at",
			want:   "date_trunc('month', created_at)",
		},
		{
			name:   "truncate to year",
			unit:   "year",
			column: "timestamp",
			want:   "date_trunc('year', timestamp)",
		},
		{
			name:   "truncate to week",
			unit:   "week",
			column: "timestamp",
			want:   "date_trunc('week', timestamp)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := drv.DateTrunc(tt.unit, tt.column)
			if got != tt.want {
				t.Errorf("DateTrunc(%q, %q) = %q, want %q", tt.unit, tt.column, got, tt.want)
			}
		})
	}
}

// TestDateFormatDialectDifferences confirms the two drivers produce different SQL
// for the same inputs — the whole point of the abstraction.
// Covers SC-1.
func TestDateFormatDialectDifferences(t *testing.T) {
	t.Parallel()
	sqlite := NewSQLite()
	postgres := NewPostgres()

	formats := []string{"day", "week", "month", "rfc3339"}
	for _, format := range formats {
		t.Run(format, func(t *testing.T) {
			s := sqlite.DateFormat("col", format)
			p := postgres.DateFormat("col", format)
			if s == p {
				t.Errorf("SQLite and PostgreSQL should produce different SQL for format %q, both got %q", format, s)
			}
		})
	}
}

// TestDateTruncDialectDifferences confirms the two drivers produce different SQL.
// Covers SC-1.
func TestDateTruncDialectDifferences(t *testing.T) {
	t.Parallel()
	sqlite := NewSQLite()
	postgres := NewPostgres()

	units := []string{"day", "month", "year"}
	for _, unit := range units {
		t.Run(unit, func(t *testing.T) {
			s := sqlite.DateTrunc(unit, "col")
			p := postgres.DateTrunc(unit, "col")
			if s == p {
				t.Errorf("SQLite and PostgreSQL should produce different SQL for unit %q, both got %q", unit, s)
			}
		})
	}
}
