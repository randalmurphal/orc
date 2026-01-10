// Package executor provides task phase execution with LLM integration.
package executor

import (
	"context"

	"github.com/randalmurphal/llmkit/claude"
)

// ctxKey is a private type for context keys to avoid collisions.
type ctxKey string

// llmKey is the context key for the LLM client.
const llmKey ctxKey = "orc.llm"

// WithLLM adds an LLM client to the context.
// This allows nodes to access the LLM client via LLM(ctx).
//
// Example:
//
//	client := claude.NewClaudeCLI(...)
//	ctx := executor.WithLLM(context.Background(), client)
//	// Now flowgraph nodes can use executor.LLM(ctx)
func WithLLM(ctx context.Context, client claude.Client) context.Context {
	return context.WithValue(ctx, llmKey, client)
}

// LLM retrieves the LLM client from context.
// Returns nil if no client is configured.
//
// Example:
//
//	func myNode(ctx flowgraph.Context, s State) (State, error) {
//	    client := executor.LLM(ctx)
//	    if client == nil {
//	        return s, fmt.Errorf("no LLM client available")
//	    }
//	    // Use client...
//	}
func LLM(ctx context.Context) claude.Client {
	if c, ok := ctx.Value(llmKey).(claude.Client); ok {
		return c
	}
	return nil
}
