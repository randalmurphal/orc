package bench

import (
	"testing"
)

func TestExtractTestCount(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   int
	}{
		{
			name: "go test verbose output",
			output: `=== RUN   TestOpen
--- PASS: TestOpen (0.01s)
=== RUN   TestBasicInsert
--- PASS: TestBasicInsert (0.02s)
=== RUN   TestPageSplit
--- FAIL: TestPageSplit (0.01s)
FAIL
FAIL	github.com/etcd-io/bbolt	0.5s`,
			want: 3,
		},
		{
			name:   "pytest output",
			output: "===== 12 passed, 3 failed in 1.5s =====",
			want:   15,
		},
		{
			name:   "rust cargo test",
			output: "test result: ok. 8 passed; 2 failed; 0 ignored",
			want:   10,
		},
		{
			name:   "empty output",
			output: "",
			want:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTestCount(tt.output)
			if got != tt.want {
				t.Errorf("extractTestCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountRegressions(t *testing.T) {
	output := `
--- FAIL: TestBasicInsert (0.01s)
--- FAIL: TestPageSplit (0.02s)
--- PASS: TestOpen (0.01s)
`
	passToPass := []string{"TestBasicInsert", "TestOpen", "TestCreate"}

	got := countRegressions(output, passToPass)
	// TestBasicInsert appears on a FAIL line → regression
	// TestOpen appears on a PASS line → NOT a regression (per-line check)
	// TestCreate doesn't appear → not counted
	if got != 1 {
		t.Errorf("expected exactly 1 regression (TestBasicInsert), got %d", got)
	}
}

func TestCountTestsRun(t *testing.T) {
	output := `
=== RUN   TestPageSplit
--- PASS: TestPageSplit (0.01s)
=== RUN   TestBasicInsert
--- PASS: TestBasicInsert (0.02s)
`
	tests := []string{"TestPageSplit", "TestBasicInsert", "TestMissing"}

	got := countTestsRun(output, tests)
	if got != 2 {
		t.Errorf("expected 2 tests found, got %d", got)
	}
}

func TestExtractNumberBefore(t *testing.T) {
	tests := []struct {
		line    string
		keyword string
		want    int
	}{
		{"12 passed", "passed", 12},
		{"3 failed", "failed", 3},
		{"  42 passed, 3 failed", "passed", 42},
		{"no numbers here", "passed", 0},
		{"", "passed", 0},
	}

	for _, tt := range tests {
		got := extractNumberBefore(tt.line, tt.keyword)
		if got != tt.want {
			t.Errorf("extractNumberBefore(%q, %q) = %d, want %d", tt.line, tt.keyword, got, tt.want)
		}
	}
}
