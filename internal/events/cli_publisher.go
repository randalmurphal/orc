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

	// Handle different event types
	switch event.Type {
	case EventTranscript:
		if !p.streamMode {
			return
		}
		p.handleTranscript(event)
	case EventActivity:
		p.handleActivity(event)
	case EventHeartbeat:
		p.handleHeartbeat(event)
	case EventWarning:
		p.handleWarning(event)
	}
}

// handleTranscript processes transcript events.
func (p *CLIPublisher) handleTranscript(event Event) {
	line, ok := event.Data.(TranscriptLine)
	if !ok {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Format output based on type
	switch line.Type {
	case "prompt":
		_, _ = fmt.Fprintf(p.out, "\n━━━ Prompt [%s iter:%d] ━━━\n", line.Phase, line.Iteration)
		_, _ = fmt.Fprintln(p.out, line.Content)
	case "response":
		_, _ = fmt.Fprintf(p.out, "\n━━━ Response [%s iter:%d] ━━━\n", line.Phase, line.Iteration)
		_, _ = fmt.Fprintln(p.out, line.Content)
	case "chunk":
		// Streaming chunk - write directly without newline
		_, _ = fmt.Fprint(p.out, line.Content)
	case "tool":
		// Tool calls - abbreviated output
		content := line.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		_, _ = fmt.Fprintf(p.out, "\n🔧 Tool: %s\n", strings.TrimSpace(content))
	case "error":
		_, _ = fmt.Fprintf(p.out, "\n❌ Error: %s\n", line.Content)
	}
}

// handleActivity processes activity state change events.
func (p *CLIPublisher) handleActivity(event Event) {
	activity, ok := event.Data.(ActivityUpdate)
	if !ok {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Handle activity state changes with appropriate messages
	switch activity.Activity {
	case "waiting_api":
		_, _ = fmt.Fprintf(p.out, "\n⏳ Waiting for Claude API...")
	case "running_tool":
		_, _ = fmt.Fprintf(p.out, "\n🔧 Running tool...")
	case "spec_analyzing":
		_, _ = fmt.Fprintf(p.out, "\n🔍 Analyzing codebase...")
	case "spec_writing":
		_, _ = fmt.Fprintf(p.out, "\n📝 Writing specification...")
	}
}

// handleHeartbeat processes heartbeat events.
func (p *CLIPublisher) handleHeartbeat(_ Event) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Print a dot to show progress
	_, _ = fmt.Fprint(p.out, ".")
}

// handleWarning processes warning events.
func (p *CLIPublisher) handleWarning(event Event) {
	warning, ok := event.Data.(WarningData)
	if !ok {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	_, _ = fmt.Fprintf(p.out, "\n⚠️  %s\n", warning.Message)
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
