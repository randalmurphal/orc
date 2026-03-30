package executor

import (
	"log/slog"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

func TestNewTurnExecutor_Claude(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{Provider: "claude", Logger: slog.Default()})
	if _, ok := te.(*ClaudeExecutor); !ok {
		t.Fatalf("expected *ClaudeExecutor, got %T", te)
	}
}

func TestNewTurnExecutor_Codex(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{Provider: "codex", Logger: slog.Default()})
	if _, ok := te.(*CodexExecutor); !ok {
		t.Fatalf("expected *CodexExecutor, got %T", te)
	}
}

func TestNewTurnExecutor_EmptyProviderDefaultsClaude(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{Provider: "", Logger: slog.Default()})
	if _, ok := te.(*ClaudeExecutor); !ok {
		t.Fatalf("expected *ClaudeExecutor, got %T", te)
	}
}

func TestNewTurnExecutor_ClaudeOptions(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{
		Provider:         "claude",
		ClaudePath:       "/usr/bin/claude",
		Model:            "opus",
		WorkingDir:       "/tmp/work",
		SessionID:        "sess-123",
		Resume:           true,
		PhaseID:          "implement",
		TaskID:           "TASK-001",
		RunID:            "run-1",
		ReviewRound:      2,
		MaxTurns:         100,
		ProducesArtifact: true,
		Logger:           slog.Default(),
	})
	ce, ok := te.(*ClaudeExecutor)
	if !ok {
		t.Fatalf("expected *ClaudeExecutor, got %T", te)
	}
	if ce.claudePath != "/usr/bin/claude" || ce.model != "opus" || ce.sessionID != "sess-123" {
		t.Fatalf("unexpected claude executor config: %+v", ce)
	}
	if !ce.resume || ce.phaseID != "implement" || ce.reviewRound != 2 || ce.maxTurns != 100 || !ce.producesArtifact {
		t.Fatalf("unexpected claude executor options: %+v", ce)
	}
}

func TestNewTurnExecutor_CodexOptions(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{
		Provider:        "codex",
		CodexPath:       "/usr/bin/codex",
		Model:           "gpt-5",
		Resume:          true,
		ReviewRound:     1,
		ReasoningEffort: "high",
		WebSearchMode:   "cached",
		Logger:          slog.Default(),
	})
	ce, ok := te.(*CodexExecutor)
	if !ok {
		t.Fatalf("expected *CodexExecutor, got %T", te)
	}
	if ce.codexPath != "/usr/bin/codex" || ce.model != "gpt-5" {
		t.Fatalf("unexpected codex executor config: %+v", ce)
	}
	if !ce.resume || ce.reviewRound != 1 || ce.reasoningEffort != "high" || ce.webSearchMode != "cached" {
		t.Fatalf("unexpected codex executor options: %+v", ce)
	}
}

func TestSetTaskSessionMetadata(t *testing.T) {
	t.Run("sets provider and model", func(t *testing.T) {
		task := &orcv1.Task{Id: "TASK-001", Metadata: make(map[string]string)}
		setTaskSessionMetadata(task, "implement", "claude", "opus")

		if got := task.Metadata["phase:implement:provider"]; got != "claude" {
			t.Fatalf("provider = %q, want claude", got)
		}
		if got := task.Metadata["phase:implement:model"]; got != "opus" {
			t.Fatalf("model = %q, want opus", got)
		}
	})

	t.Run("creates metadata map if nil", func(t *testing.T) {
		task := &orcv1.Task{Id: "TASK-002"}
		setTaskSessionMetadata(task, "review", "codex", "gpt-5")
		if task.Metadata == nil || task.Metadata["phase:review:provider"] != "codex" {
			t.Fatalf("metadata not initialized correctly: %+v", task.Metadata)
		}
	})

	t.Run("nil task is safe", func(t *testing.T) {
		setTaskSessionMetadata(nil, "implement", "claude", "opus")
	})
}
