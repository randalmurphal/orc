// Package llmutil provides shared LLM utilities for orc.
// This package exists to break import cycles between executor and gate.
package llmutil

import (
	"context"
	"fmt"

	llmkit "github.com/randalmurphal/llmkit/v2"
)

// SchemaResult holds the parsed JSON response with metadata.
type SchemaResult[T any] struct {
	Data     T
	Response *llmkit.Response
}

// ExecuteWithSchema is the ONLY way to make schema-constrained LLM calls.
// All callers must use this - ensures proper error handling, no fallbacks.
//
// This function:
//   - Requires a non-empty schema (errors if empty)
//   - Passes the schema to the client's Complete method
//   - Strictly parses the response - returns error on parse failure
//   - Never silently falls back or returns success on failure
//
// Returns error if:
//   - schema is empty
//   - client.Complete() fails
//   - JSON parsing fails
func ExecuteWithSchema[T any](
	ctx context.Context,
	client llmkit.Client,
	prompt string,
	schema string,
) (*SchemaResult[T], error) {
	if schema == "" {
		return nil, fmt.Errorf("schema is required for ExecuteWithSchema")
	}

	typed, err := llmkit.CompleteTyped[T](ctx, client, llmkit.Request{
		Messages:   []llmkit.Message{{Role: llmkit.RoleUser, Content: prompt}},
		JSONSchema: []byte(schema),
	})
	if err != nil {
		return nil, fmt.Errorf("schema execution failed: %w", err)
	}
	return &SchemaResult[T]{Data: typed.Value, Response: typed.Response}, nil
}
