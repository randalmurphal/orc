package events

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLIPublisher_StreamsTranscriptEvents(t *testing.T) {
	var buf bytes.Buffer
	pub := NewCLIPublisher(&buf, WithStreamMode(true))

	// Publish a prompt event
	pub.Publish(Event{
		Type:   EventTranscript,
		TaskID: "TASK-001",
		Data: TranscriptLine{
			Phase:     "implement",
			Iteration: 1,
			Type:      "prompt",
			Content:   "Test prompt content",
		},
	})

	output := buf.String()
	if !strings.Contains(output, "Prompt [implement iter:1]") {
		t.Errorf("Expected prompt header, got: %s", output)
	}
	if !strings.Contains(output, "Test prompt content") {
		t.Errorf("Expected prompt content, got: %s", output)
	}
}

func TestCLIPublisher_StreamsResponse(t *testing.T) {
	var buf bytes.Buffer
	pub := NewCLIPublisher(&buf, WithStreamMode(true))

	pub.Publish(Event{
		Type:   EventTranscript,
		TaskID: "TASK-001",
		Data: TranscriptLine{
			Phase:     "test",
			Iteration: 2,
			Type:      "response",
			Content:   "Here is my response",
		},
	})

	output := buf.String()
	if !strings.Contains(output, "Response [test iter:2]") {
		t.Errorf("Expected response header, got: %s", output)
	}
	if !strings.Contains(output, "Here is my response") {
		t.Errorf("Expected response content, got: %s", output)
	}
}

func TestCLIPublisher_StreamModeDisabled(t *testing.T) {
	var buf bytes.Buffer
	pub := NewCLIPublisher(&buf, WithStreamMode(false))

	pub.Publish(Event{
		Type:   EventTranscript,
		TaskID: "TASK-001",
		Data: TranscriptLine{
			Phase:   "implement",
			Type:    "response",
			Content: "Should not appear",
		},
	})

	if buf.Len() > 0 {
		t.Errorf("Expected no output when streaming disabled, got: %s", buf.String())
	}
}

func TestCLIPublisher_IgnoresNonTranscriptEvents(t *testing.T) {
	var buf bytes.Buffer
	pub := NewCLIPublisher(&buf, WithStreamMode(true))

	// Publish non-transcript events
	pub.Publish(Event{
		Type:   EventPhase,
		TaskID: "TASK-001",
		Data:   PhaseUpdate{Phase: "implement", Status: "running"},
	})

	pub.Publish(Event{
		Type:   EventTokens,
		TaskID: "TASK-001",
		Data:   TokenUpdate{Phase: "implement", TotalTokens: 1000},
	})

	if buf.Len() > 0 {
		t.Errorf("Expected no output for non-transcript events, got: %s", buf.String())
	}
}

func TestCLIPublisher_FansOutToInner(t *testing.T) {
	inner := NewMemoryPublisher()
	var buf bytes.Buffer
	pub := NewCLIPublisher(&buf, WithInnerPublisher(inner))

	// Subscribe to inner publisher
	ch := inner.Subscribe("TASK-001")

	// Publish event
	event := Event{
		Type:   EventTranscript,
		TaskID: "TASK-001",
		Data: TranscriptLine{
			Phase:   "implement",
			Type:    "response",
			Content: "Test",
		},
	}
	pub.Publish(event)

	// Check inner received it
	select {
	case received := <-ch:
		if received.Type != EventTranscript {
			t.Errorf("Inner publisher received wrong event type: %v", received.Type)
		}
	default:
		t.Error("Inner publisher did not receive event")
	}
}

func TestCLIPublisher_TruncatesLongToolCalls(t *testing.T) {
	var buf bytes.Buffer
	pub := NewCLIPublisher(&buf, WithStreamMode(true))

	longContent := strings.Repeat("x", 500)
	pub.Publish(Event{
		Type:   EventTranscript,
		TaskID: "TASK-001",
		Data: TranscriptLine{
			Phase:   "implement",
			Type:    "tool",
			Content: longContent,
		},
	})

	output := buf.String()
	if !strings.Contains(output, "...") {
		t.Errorf("Expected truncated tool output, got: %s", output)
	}
	if strings.Contains(output, longContent) {
		t.Error("Tool output should be truncated but got full content")
	}
}

func TestCLIPublisher_HandlesError(t *testing.T) {
	var buf bytes.Buffer
	pub := NewCLIPublisher(&buf, WithStreamMode(true))

	pub.Publish(Event{
		Type:   EventTranscript,
		TaskID: "TASK-001",
		Data: TranscriptLine{
			Phase:   "implement",
			Type:    "error",
			Content: "Something went wrong",
		},
	})

	output := buf.String()
	if !strings.Contains(output, "❌") {
		t.Errorf("Expected error emoji, got: %s", output)
	}
	if !strings.Contains(output, "Something went wrong") {
		t.Errorf("Expected error content, got: %s", output)
	}
}

func TestCLIPublisher_SpecActivityAnalyzing(t *testing.T) {
	var buf bytes.Buffer
	pub := NewCLIPublisher(&buf, WithStreamMode(true))

	pub.Publish(Event{
		Type:   EventActivity,
		TaskID: "TASK-001",
		Data: ActivityUpdate{
			Phase:    "spec",
			Activity: "spec_analyzing",
		},
	})

	output := buf.String()
	if !strings.Contains(output, "Analyzing codebase") {
		t.Errorf("Expected spec analyzing message, got: %s", output)
	}
}

func TestCLIPublisher_SpecActivityWriting(t *testing.T) {
	var buf bytes.Buffer
	pub := NewCLIPublisher(&buf, WithStreamMode(true))

	pub.Publish(Event{
		Type:   EventActivity,
		TaskID: "TASK-001",
		Data: ActivityUpdate{
			Phase:    "spec",
			Activity: "spec_writing",
		},
	})

	output := buf.String()
	if !strings.Contains(output, "Writing specification") {
		t.Errorf("Expected spec writing message, got: %s", output)
	}
}

func TestActivityUpdate_IsSpecPhaseActivity(t *testing.T) {
	tests := []struct {
		activity string
		expected bool
	}{
		{"idle", false},
		{"waiting_api", false},
		{"streaming", false},
		{"running_tool", false},
		{"processing", false},
		{"spec_analyzing", true},
		{"spec_writing", true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.activity, func(t *testing.T) {
			update := ActivityUpdate{Phase: "spec", Activity: tt.activity}
			if got := update.IsSpecPhaseActivity(); got != tt.expected {
				t.Errorf("ActivityUpdate.IsSpecPhaseActivity() = %v, want %v", got, tt.expected)
			}
		})
	}
}
