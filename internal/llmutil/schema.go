// Package llmutil provides shared LLM utilities for orc.
// This package exists to break import cycles between executor and gate.
package llmutil

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/randalmurphal/llmkit/claude"
)

// SchemaResult holds the parsed JSON response with metadata.
type SchemaResult[T any] struct {
	Data     T
	Response *claude.CompletionResponse
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
	client claude.Client,
	prompt string,
	schema string,
) (*SchemaResult[T], error) {
	if schema == "" {
		return nil, fmt.Errorf("schema is required for ExecuteWithSchema")
	}

	resp, err := client.Complete(ctx, claude.CompletionRequest{
		Messages:   []claude.Message{{Role: claude.RoleUser, Content: prompt}},
		JSONSchema: schema,
	})
	if err != nil {
		return nil, fmt.Errorf("schema execution failed: %w", err)
	}

	// Check for empty content before JSON parsing
	if resp.Content == "" {
		return nil, fmt.Errorf("empty response content from API (model may have returned no output)")
	}

	// Strict parsing - no fallbacks
	var result T
	if err := json.Unmarshal([]byte(resp.Content), &result); err != nil {
		return nil, fmt.Errorf("schema response parse failed (content=%q): %w",
			truncateForError(resp.Content, 200), err)
	}

	return &SchemaResult[T]{Data: result, Response: resp}, nil
}

// truncateForError truncates content for error messages.
func truncateForError(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "...[truncated]"
}
