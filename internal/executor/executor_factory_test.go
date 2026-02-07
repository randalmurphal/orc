package executor

import (
	"log/slog"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
)

func TestNewTurnExecutor_Claude(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{
		Provider: "claude",
		Logger:   slog.Default(),
	})
	if _, ok := te.(*ClaudeExecutor); !ok {
		t.Errorf("expected *ClaudeExecutor, got %T", te)
	}
}

func TestNewTurnExecutor_Codex(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{
		Provider: "codex",
		Logger:   slog.Default(),
	})
	if _, ok := te.(*CodexExecutor); !ok {
		t.Errorf("expected *CodexExecutor, got %T", te)
	}
}

func TestNewTurnExecutor_Ollama(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{
		Provider: "ollama",
		Logger:   slog.Default(),
	})
	if _, ok := te.(*CodexExecutor); !ok {
		t.Errorf("expected *CodexExecutor for ollama provider, got %T", te)
	}
}

func TestNewTurnExecutor_LMStudio(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{
		Provider: "lmstudio",
		Logger:   slog.Default(),
	})
	if _, ok := te.(*CodexExecutor); !ok {
		t.Errorf("expected *CodexExecutor for lmstudio provider, got %T", te)
	}
}

func TestNewTurnExecutor_EmptyProvider(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{
		Provider: "",
		Logger:   slog.Default(),
	})
	if _, ok := te.(*ClaudeExecutor); !ok {
		t.Errorf("expected *ClaudeExecutor for empty provider, got %T", te)
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
	if ce.claudePath != "/usr/bin/claude" {
		t.Errorf("claudePath = %q, want /usr/bin/claude", ce.claudePath)
	}
	if ce.model != "opus" {
		t.Errorf("model = %q, want opus", ce.model)
	}
	if ce.sessionID != "sess-123" {
		t.Errorf("sessionID = %q, want sess-123", ce.sessionID)
	}
	if !ce.resume {
		t.Error("resume should be true")
	}
	if ce.phaseID != "implement" {
		t.Errorf("phaseID = %q, want implement", ce.phaseID)
	}
	if ce.reviewRound != 2 {
		t.Errorf("reviewRound = %d, want 2", ce.reviewRound)
	}
	if ce.maxTurns != 100 {
		t.Errorf("maxTurns = %d, want 100", ce.maxTurns)
	}
	if !ce.producesArtifact {
		t.Error("producesArtifact should be true")
	}
}

func TestNewTurnExecutor_CodexOptions(t *testing.T) {
	te := NewTurnExecutor(TurnExecutorConfig{
		Provider:      "codex",
		CodexPath:     "/usr/bin/codex",
		Model:         "gpt-5",
		LocalProvider: "ollama",
		Resume:        true,
		ReviewRound:   1,
		Logger:        slog.Default(),
	})
	ce, ok := te.(*CodexExecutor)
	if !ok {
		t.Fatalf("expected *CodexExecutor, got %T", te)
	}
	if ce.codexPath != "/usr/bin/codex" {
		t.Errorf("codexPath = %q, want /usr/bin/codex", ce.codexPath)
	}
	if ce.model != "gpt-5" {
		t.Errorf("model = %q, want gpt-5", ce.model)
	}
	if ce.localProvider != "ollama" {
		t.Errorf("localProvider = %q, want ollama", ce.localProvider)
	}
	if !ce.resume {
		t.Error("resume should be true")
	}
	if ce.reviewRound != 1 {
		t.Errorf("reviewRound = %d, want 1", ce.reviewRound)
	}
}

func TestNormalizeCodexExecutionModel(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		model    string
		want     string
	}{
		{
			name:     "strip ollama prefix",
			provider: "ollama",
			model:    "ollama/qwen2.5",
			want:     "qwen2.5",
		},
		{
			name:     "strip lmstudio prefix",
			provider: "lmstudio",
			model:    "lmstudio/deepseek-r1",
			want:     "deepseek-r1",
		},
		{
			name:     "no prefix to strip",
			provider: "codex",
			model:    "gpt-5",
			want:     "gpt-5",
		},
		{
			name:     "mismatched prefix not stripped",
			provider: "codex",
			model:    "ollama/qwen2.5",
			want:     "ollama/qwen2.5",
		},
		{
			name:     "empty model",
			provider: "ollama",
			model:    "",
			want:     "",
		},
		{
			name:     "case insensitive provider match",
			provider: "Ollama",
			model:    "ollama/qwen2.5",
			want:     "qwen2.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCodexExecutionModel(tt.provider, tt.model)
			if got != tt.want {
				t.Errorf("normalizeCodexExecutionModel(%q, %q) = %q, want %q",
					tt.provider, tt.model, got, tt.want)
			}
		})
	}
}

func TestSetTaskSessionMetadata(t *testing.T) {
	t.Run("sets provider and model", func(t *testing.T) {
		task := &orcv1.Task{
			Id:       "TASK-001",
			Metadata: make(map[string]string),
		}
		setTaskSessionMetadata(task, "implement", "claude", "opus")

		if got := task.Metadata["phase:implement:provider"]; got != "claude" {
			t.Errorf("provider = %q, want claude", got)
		}
		if got := task.Metadata["phase:implement:model"]; got != "opus" {
			t.Errorf("model = %q, want opus", got)
		}
	})

	t.Run("creates metadata map if nil", func(t *testing.T) {
		task := &orcv1.Task{
			Id: "TASK-002",
		}
		setTaskSessionMetadata(task, "review", "codex", "gpt-5")

		if task.Metadata == nil {
			t.Fatal("metadata should have been initialized")
		}
		if got := task.Metadata["phase:review:provider"]; got != "codex" {
			t.Errorf("provider = %q, want codex", got)
		}
	})

	t.Run("nil task is safe", func(t *testing.T) {
		// Should not panic
		setTaskSessionMetadata(nil, "implement", "claude", "opus")
	})
}
