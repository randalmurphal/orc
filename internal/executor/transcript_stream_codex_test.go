package executor

import (
	"log/slog"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/storage"
)

// mockTranscriptBackend implements storage.Backend for transcript testing.
// Only AddTranscript is used; other methods panic via the embedded interface.
type mockTranscriptBackend struct {
	storage.Backend // embed to satisfy interface
	transcripts     []*storage.Transcript
}

func (m *mockTranscriptBackend) AddTranscript(t *storage.Transcript) error {
	m.transcripts = append(m.transcripts, t)
	return nil
}

func (m *mockTranscriptBackend) LoadTask(string) (*orcv1.Task, error) {
	return nil, nil
}

func (m *mockTranscriptBackend) SaveTask(*orcv1.Task) error {
	return nil
}

func TestStoreAssistantText_Basic(t *testing.T) {
	backend := &mockTranscriptBackend{}
	h := NewTranscriptStreamHandler(backend, slog.Default(), "TASK-001", "implement", "sess-1", "run-1", "gpt-5", nil, nil)

	h.StoreAssistantText("Hello world", "gpt-5", "msg-1", 100, 50)

	if len(backend.transcripts) != 1 {
		t.Fatalf("expected 1 transcript, got %d", len(backend.transcripts))
	}
	tr := backend.transcripts[0]
	if tr.Content != "Hello world" {
		t.Errorf("content = %q, want %q", tr.Content, "Hello world")
	}
	if tr.Role != "assistant" {
		t.Errorf("role = %q, want assistant", tr.Role)
	}
	if tr.Model != "gpt-5" {
		t.Errorf("model = %q, want gpt-5", tr.Model)
	}
	if tr.InputTokens != 100 {
		t.Errorf("input_tokens = %d, want 100", tr.InputTokens)
	}
	if tr.OutputTokens != 50 {
		t.Errorf("output_tokens = %d, want 50", tr.OutputTokens)
	}
}

func TestStoreAssistantText_Deduplication(t *testing.T) {
	backend := &mockTranscriptBackend{}
	h := NewTranscriptStreamHandler(backend, slog.Default(), "TASK-001", "implement", "sess-1", "run-1", "gpt-5", nil, nil)

	h.StoreAssistantText("msg1", "gpt-5", "same-id", 10, 5)
	h.StoreAssistantText("msg2", "gpt-5", "same-id", 10, 5) // duplicate

	if len(backend.transcripts) != 1 {
		t.Fatalf("expected 1 transcript (dedup), got %d", len(backend.transcripts))
	}
}

func TestStoreAssistantText_EmptyMessageID(t *testing.T) {
	backend := &mockTranscriptBackend{}
	h := NewTranscriptStreamHandler(backend, slog.Default(), "TASK-001", "implement", "sess-1", "run-1", "gpt-5", nil, nil)

	h.StoreAssistantText("test", "", "", 10, 5) // empty message ID -> auto-generated

	if len(backend.transcripts) != 1 {
		t.Fatalf("expected 1 transcript, got %d", len(backend.transcripts))
	}
	if backend.transcripts[0].MessageUUID == "" {
		t.Error("expected auto-generated message UUID")
	}
}

func TestStoreAssistantText_FallbackModel(t *testing.T) {
	backend := &mockTranscriptBackend{}
	h := NewTranscriptStreamHandler(backend, slog.Default(), "TASK-001", "implement", "sess-1", "run-1", "default-model", nil, nil)

	h.StoreAssistantText("test", "", "msg-1", 10, 5) // empty model -> use handler default

	if backend.transcripts[0].Model != "default-model" {
		t.Errorf("model = %q, want default-model", backend.transcripts[0].Model)
	}
}

func TestStoreAssistantText_NilBackend(t *testing.T) {
	h := NewTranscriptStreamHandler(nil, slog.Default(), "TASK-001", "implement", "sess-1", "run-1", "gpt-5", nil, nil)
	// Should not panic
	h.StoreAssistantText("test", "gpt-5", "msg-1", 10, 5)
}

func TestStoreChunkText(t *testing.T) {
	backend := &mockTranscriptBackend{}
	h := NewTranscriptStreamHandler(backend, slog.Default(), "TASK-001", "implement", "sess-1", "run-1", "gpt-5", nil, nil)

	h.StoreChunkText("partial output", "gpt-5")

	if len(backend.transcripts) != 1 {
		t.Fatalf("expected 1 transcript, got %d", len(backend.transcripts))
	}
	if backend.transcripts[0].Type != "chunk" {
		t.Fatalf("type = %q, want chunk", backend.transcripts[0].Type)
	}
	if backend.transcripts[0].Content != "partial output" {
		t.Fatalf("content = %q, want partial output", backend.transcripts[0].Content)
	}
}
