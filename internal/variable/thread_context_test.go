package variable

import (
	"context"
	"testing"
)

func TestResolver_ThreadVariables(t *testing.T) {
	t.Parallel()

	resolver := NewResolver(t.TempDir())
	rctx := &ResolutionContext{
		ThreadID:                   "THR-001",
		ThreadTitle:                "Workspace thread",
		ThreadContext:              "combined context",
		ThreadHistory:              "recent history",
		ThreadLinkedContext:        "linked context",
		ThreadRecommendationDrafts: "recommendation drafts",
		ThreadDecisionDrafts:       "decision drafts",
	}

	vars, err := resolver.ResolveAll(context.Background(), nil, rctx)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	if vars["THREAD_ID"] != "THR-001" {
		t.Fatalf("THREAD_ID = %q, want THR-001", vars["THREAD_ID"])
	}
	if vars["THREAD_TITLE"] != "Workspace thread" {
		t.Fatalf("THREAD_TITLE = %q", vars["THREAD_TITLE"])
	}
	if vars["THREAD_CONTEXT"] != "combined context" {
		t.Fatalf("THREAD_CONTEXT = %q", vars["THREAD_CONTEXT"])
	}
	if vars["THREAD_HISTORY"] != "recent history" {
		t.Fatalf("THREAD_HISTORY = %q", vars["THREAD_HISTORY"])
	}
	if vars["THREAD_LINKED_CONTEXT"] != "linked context" {
		t.Fatalf("THREAD_LINKED_CONTEXT = %q", vars["THREAD_LINKED_CONTEXT"])
	}
	if vars["THREAD_RECOMMENDATION_DRAFTS"] != "recommendation drafts" {
		t.Fatalf("THREAD_RECOMMENDATION_DRAFTS = %q", vars["THREAD_RECOMMENDATION_DRAFTS"])
	}
	if vars["THREAD_DECISION_DRAFTS"] != "decision drafts" {
		t.Fatalf("THREAD_DECISION_DRAFTS = %q", vars["THREAD_DECISION_DRAFTS"])
	}
}
