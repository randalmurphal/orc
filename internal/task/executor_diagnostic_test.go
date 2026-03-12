package task

import (
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

func TestExecutorDiagnosticProto_RoundTrip(t *testing.T) {
	t.Parallel()

	taskProto := &orcv1.Task{}
	SetExecutorDiagnosticProto(taskProto, ExecutorDiagnostic{
		Kind:          "panic",
		Phase:         "implement_codex",
		Reason:        "executor panic: boom",
		Detail:        "stack trace",
		ExecutorPID:   1234,
		DetectedAt:    "2026-03-12T12:00:00Z",
		LastHeartbeat: "2026-03-12T11:59:00Z",
	})

	diagnostic := GetExecutorDiagnosticProto(taskProto)
	if diagnostic == nil {
		t.Fatal("expected diagnostic")
	}
	if diagnostic.Kind != "panic" {
		t.Fatalf("kind = %q, want panic", diagnostic.Kind)
	}
	if diagnostic.Phase != "implement_codex" {
		t.Fatalf("phase = %q, want implement_codex", diagnostic.Phase)
	}
	if diagnostic.ExecutorPID != 1234 {
		t.Fatalf("executor_pid = %d, want 1234", diagnostic.ExecutorPID)
	}

	ClearExecutorDiagnosticProto(taskProto)
	if diagnostic := GetExecutorDiagnosticProto(taskProto); diagnostic != nil {
		t.Fatalf("expected cleared diagnostic, got %+v", diagnostic)
	}
}
