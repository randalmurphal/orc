package events

import (
	"fmt"
	"io"
	"strings"
	"sync"
)

// CLIPublisher writes transcript events to an io.Writer (typically stdout).
// It wraps another publisher to also fan out events for WebSocket/API use.
type CLIPublisher struct {
	inner      Publisher
	out        io.Writer
	mu         sync.Mutex
	streamMode bool // If true, stream all transcript content
}

// CLIPublisherOption configures a CLIPublisher.
type CLIPublisherOption func(*CLIPublisher)

// WithInnerPublisher sets an inner publisher to fan out events to.
func WithInnerPublisher(p Publisher) CLIPublisherOption {
	return func(c *CLIPublisher) {
		c.inner = p
	}
}

// WithStreamMode enables full transcript streaming to output.
func WithStreamMode(enabled bool) CLIPublisherOption {
	return func(c *CLIPublisher) {
		c.streamMode = enabled
	}
}

// NewCLIPublisher creates a publisher that writes events to the given writer.
func NewCLIPublisher(out io.Writer, opts ...CLIPublisherOption) *CLIPublisher {
	p := &CLIPublisher{
		out:        out,
		streamMode: true, // Default to streaming
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// Publish writes transcript events to the output writer and fans out to inner publisher.
func (p *CLIPublisher) Publish(event Event) {
	// Fan out to inner publisher if present
	if p.inner != nil {
		p.inner.Publish(event)
	}

	// Only stream transcript events
	if event.Type != EventTranscript || !p.streamMode {
		return
	}

	line, ok := event.Data.(TranscriptLine)
	if !ok {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Format output based on type
	switch line.Type {
	case "prompt":
		fmt.Fprintf(p.out, "\n━━━ Prompt [%s iter:%d] ━━━\n", line.Phase, line.Iteration)
		fmt.Fprintln(p.out, line.Content)
	case "response":
		fmt.Fprintf(p.out, "\n━━━ Response [%s iter:%d] ━━━\n", line.Phase, line.Iteration)
		fmt.Fprintln(p.out, line.Content)
	case "chunk":
		// Streaming chunk - write directly without newline
		fmt.Fprint(p.out, line.Content)
	case "tool":
		// Tool calls - abbreviated output
		content := line.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		fmt.Fprintf(p.out, "\n🔧 Tool: %s\n", strings.TrimSpace(content))
	case "error":
		fmt.Fprintf(p.out, "\n❌ Error: %s\n", line.Content)
	}
}

// Subscribe delegates to inner publisher or returns closed channel.
func (p *CLIPublisher) Subscribe(taskID string) <-chan Event {
	if p.inner != nil {
		return p.inner.Subscribe(taskID)
	}
	ch := make(chan Event)
	close(ch)
	return ch
}

// Unsubscribe delegates to inner publisher.
func (p *CLIPublisher) Unsubscribe(taskID string, ch <-chan Event) {
	if p.inner != nil {
		p.inner.Unsubscribe(taskID, ch)
	}
}

// Close delegates to inner publisher.
func (p *CLIPublisher) Close() {
	if p.inner != nil {
		p.inner.Close()
	}
}
