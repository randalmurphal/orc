package variable

import "testing"

func TestIndexedArtifactsBuiltin(t *testing.T) {
	t.Parallel()

	resolver := NewResolver("")
	vars, err := resolver.ResolveAll(t.Context(), nil, &ResolutionContext{
		IndexedArtifacts: "## Indexed Artifacts\n- [task_outcome] Missing nil guard",
	})
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	if got := vars["INDEXED_ARTIFACTS"]; got == "" {
		t.Fatal("INDEXED_ARTIFACTS variable missing")
	}
}
