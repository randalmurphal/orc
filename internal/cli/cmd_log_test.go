package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/randalmurphal/orc/internal/storage"
)

func TestDisplayFormattedContent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []string // strings that should appear in output
	}{
		{
			name:     "text block",
			content:  `[{"type": "text", "text": "Hello world"}]`,
			expected: []string{"Hello world"},
		},
		{
			name:     "tool use block",
			content:  `[{"type": "tool_use", "name": "Read", "input": {"file": "test.go"}}]`,
			expected: []string{"Tool: Read", "file"},
		},
		{
			name:    "plain text fallback",
			content: "Not JSON content",
			expected: []string{"Not JSON content"},
		},
		{
			name:    "multiple text blocks",
			content: `[{"type": "text", "text": "First"}, {"type": "text", "text": "Second"}]`,
			expected: []string{"First", "Second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			displayFormattedContent(tt.content, transcriptDisplayOptions{})

			_ = w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			_, _ = buf.ReadFrom(r)
			output := buf.String()

			for _, expected := range tt.expected {
				if !bytes.Contains([]byte(output), []byte(expected)) {
					t.Errorf("expected output to contain %q, got: %q", expected, output)
				}
			}
		})
	}
}

func TestCollectPhases(t *testing.T) {
	transcripts := []storage.Transcript{
		{Phase: "spec"},
		{Phase: "implement"},
		{Phase: "spec"},
		{Phase: "test"},
		{Phase: "implement"},
	}

	phases := collectPhases(transcripts)

	// Should have 3 unique phases in order of first appearance
	if len(phases) != 3 {
		t.Errorf("expected 3 unique phases, got %d: %v", len(phases), phases)
	}

	// Check order
	expected := []string{"spec", "implement", "test"}
	for i, p := range expected {
		if phases[i] != p {
			t.Errorf("phase[%d] = %q, want %q", i, phases[i], p)
		}
	}
}

func TestDisplaySingleTranscript(t *testing.T) {
	transcript := storage.Transcript{
		Phase:        "implement",
		Type:         "assistant",
		Model:        "claude-sonnet-4",
		Content:      `[{"type": "text", "text": "I will implement this feature."}]`,
		InputTokens:  100,
		OutputTokens: 50,
		Timestamp:    1705320000000, // 2024-01-15 10:00:00 UTC
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displaySingleTranscript(transcript, transcriptDisplayOptions{useColor: false})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Check for expected elements
	expectations := []string{
		"ASSISTANT",
		"claude-sonnet-4",
		"in:100",
		"out:50",
		"I will implement this feature",
	}

	for _, expected := range expectations {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("expected output to contain %q, got: %q", expected, output)
		}
	}
}

func TestDisplayTranscriptsPhaseHeaders(t *testing.T) {
	transcripts := []storage.Transcript{
		{Phase: "spec", Type: "user", Content: `[{"type": "text", "text": "spec prompt"}]`, Timestamp: 1},
		{Phase: "spec", Type: "assistant", Content: `[{"type": "text", "text": "spec response"}]`, Timestamp: 2},
		{Phase: "implement", Type: "user", Content: `[{"type": "text", "text": "implement prompt"}]`, Timestamp: 3},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displayTranscripts(transcripts, transcriptDisplayOptions{useColor: false})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should have phase headers
	if !bytes.Contains([]byte(output), []byte("─── spec ───")) {
		t.Error("expected spec phase header")
	}
	if !bytes.Contains([]byte(output), []byte("─── implement ───")) {
		t.Error("expected implement phase header")
	}
}

func TestTranscriptDisplayOptionsRaw(t *testing.T) {
	transcript := storage.Transcript{
		Phase:     "test",
		Type:      "assistant",
		Content:   `[{"type": "text", "text": "response text"}]`,
		Timestamp: 1705320000000,
	}

	// Capture stdout with raw option
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	displaySingleTranscript(transcript, transcriptDisplayOptions{raw: true})

	_ = w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Raw mode should show the JSON directly
	if !bytes.Contains([]byte(output), []byte(`"type": "text"`)) {
		t.Errorf("raw mode should show JSON content, got: %q", output)
	}
}
