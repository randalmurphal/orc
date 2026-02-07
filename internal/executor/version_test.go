package executor

import "testing"

func TestGetVersionInfo(t *testing.T) {
	vi := GetVersionInfo()

	// In a test binary, ReadBuildInfo succeeds but Main.Version is empty.
	// Just verify the function runs without panic and returns something sane.
	if vi.GoVersion == "" {
		t.Error("expected non-empty GoVersion from build info")
	}
}
