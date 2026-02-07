package executor

import "testing"

func TestDefaultMaxRetries_HasValue3(t *testing.T) {
	t.Parallel()

	if DefaultMaxRetries != 3 {
		t.Errorf("DefaultMaxRetries = %d, want 3", DefaultMaxRetries)
	}
}
