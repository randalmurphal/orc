package llmutil

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/randalmurphal/llmkit/claude"
)

// mockSchemaClient implements claude.Client for testing.
type mockSchemaClient struct {
	response string
	err      error
}

func (m *mockSchemaClient) Complete(_ context.Context, _ claude.CompletionRequest) (*claude.CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &claude.CompletionResponse{Content: m.response}, nil
}

func (m *mockSchemaClient) StreamJSON(_ context.Context, _ claude.CompletionRequest) (<-chan claude.StreamEvent, *claude.StreamResult, error) {
	return nil, nil, nil
}

// testStruct is a simple struct for testing JSON parsing.
type testStruct struct {
	Field1 string `json:"field1"`
	Field2 int    `json:"field2"`
}

func TestExecuteWithSchema_Success(t *testing.T) {
	t.Parallel()

	data := testStruct{Field1: "value", Field2: 42}
	jsonBytes, _ := json.Marshal(data)
	client := &mockSchemaClient{response: string(jsonBytes)}

	result, err := ExecuteWithSchema[testStruct](
		context.Background(),
		client,
		"test prompt",
		`{"type": "object"}`,
	)

	if err != nil {
		t.Fatalf("ExecuteWithSchema() error = %v", err)
	}
	if result.Data.Field1 != "value" {
		t.Errorf("Field1 = %q, want %q", result.Data.Field1, "value")
	}
	if result.Data.Field2 != 42 {
		t.Errorf("Field2 = %d, want %d", result.Data.Field2, 42)
	}
}

func TestExecuteWithSchema_EmptyContent(t *testing.T) {
	t.Parallel()

	client := &mockSchemaClient{response: ""}

	result, err := ExecuteWithSchema[testStruct](
		context.Background(),
		client,
		"test prompt",
		`{"type": "object"}`,
	)

	if err == nil {
		t.Fatal("ExecuteWithSchema() expected error, got nil")
	}
	if result != nil {
		t.Errorf("ExecuteWithSchema() result = %v, want nil", result)
	}
	if !strings.Contains(err.Error(), "empty response content from API") {
		t.Errorf("error message = %q, want to contain 'empty response content from API'", err.Error())
	}
	if !strings.Contains(err.Error(), "model may have returned no output") {
		t.Errorf("error message = %q, want to contain 'model may have returned no output'", err.Error())
	}
}

func TestExecuteWithSchema_WhitespaceContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
	}{
		{"spaces only", "   "},
		{"tabs only", "\t\t"},
		{"mixed whitespace", " \t \n "},
		{"newlines only", "\n\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockSchemaClient{response: tt.response}

			result, err := ExecuteWithSchema[testStruct](
				context.Background(),
				client,
				"test prompt",
				`{"type": "object"}`,
			)

			if err == nil {
				t.Fatal("ExecuteWithSchema() expected error for whitespace, got nil")
			}
			if result != nil {
				t.Errorf("ExecuteWithSchema() result = %v, want nil", result)
			}
			// Whitespace should fail at JSON parsing stage, not empty content check
			if strings.Contains(err.Error(), "empty response content") {
				t.Errorf("whitespace should fail at JSON parse, not empty check: %v", err)
			}
		})
	}
}

func TestExecuteWithSchema_InvalidJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		response string
	}{
		{"not json", "this is not json"},
		{"partial json", `{"field1": "value"`},
		{"wrong type", `{"field1": 123, "field2": "not a number"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockSchemaClient{response: tt.response}

			result, err := ExecuteWithSchema[testStruct](
				context.Background(),
				client,
				"test prompt",
				`{"type": "object"}`,
			)

			if err == nil {
				t.Fatal("ExecuteWithSchema() expected error for invalid JSON, got nil")
			}
			if result != nil {
				t.Errorf("ExecuteWithSchema() result = %v, want nil", result)
			}
			if !strings.Contains(err.Error(), "schema response parse failed") {
				t.Errorf("error message = %q, want to contain 'schema response parse failed'", err.Error())
			}
		})
	}
}

func TestExecuteWithSchema_EmptySchema(t *testing.T) {
	t.Parallel()

	client := &mockSchemaClient{response: `{"field1": "value", "field2": 42}`}

	result, err := ExecuteWithSchema[testStruct](
		context.Background(),
		client,
		"test prompt",
		"", // empty schema
	)

	if err == nil {
		t.Fatal("ExecuteWithSchema() expected error for empty schema, got nil")
	}
	if result != nil {
		t.Errorf("ExecuteWithSchema() result = %v, want nil", result)
	}
	if !strings.Contains(err.Error(), "schema is required for ExecuteWithSchema") {
		t.Errorf("error message = %q, want to contain 'schema is required'", err.Error())
	}
}

func TestExecuteWithSchema_APIError(t *testing.T) {
	t.Parallel()

	apiErr := errors.New("rate limit exceeded")
	client := &mockSchemaClient{err: apiErr}

	result, err := ExecuteWithSchema[testStruct](
		context.Background(),
		client,
		"test prompt",
		`{"type": "object"}`,
	)

	if err == nil {
		t.Fatal("ExecuteWithSchema() expected error for API error, got nil")
	}
	if result != nil {
		t.Errorf("ExecuteWithSchema() result = %v, want nil", result)
	}
	if !strings.Contains(err.Error(), "schema execution failed") {
		t.Errorf("error message = %q, want to contain 'schema execution failed'", err.Error())
	}
	if !errors.Is(err, apiErr) {
		t.Errorf("error chain should contain original API error")
	}
}

func TestExecuteWithSchema_ValidJSON_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("null json", func(t *testing.T) {
		client := &mockSchemaClient{response: "null"}

		// For a nullable struct pointer type
		result, err := ExecuteWithSchema[*testStruct](
			context.Background(),
			client,
			"test prompt",
			`{"type": "object"}`,
		)

		if err != nil {
			t.Fatalf("ExecuteWithSchema() error = %v", err)
		}
		if result.Data != nil {
			t.Errorf("Data = %v, want nil for null JSON", result.Data)
		}
	})

	t.Run("empty object", func(t *testing.T) {
		client := &mockSchemaClient{response: "{}"}

		result, err := ExecuteWithSchema[testStruct](
			context.Background(),
			client,
			"test prompt",
			`{"type": "object"}`,
		)

		if err != nil {
			t.Fatalf("ExecuteWithSchema() error = %v", err)
		}
		// Empty object should parse successfully with zero values
		if result.Data.Field1 != "" {
			t.Errorf("Field1 = %q, want empty string", result.Data.Field1)
		}
		if result.Data.Field2 != 0 {
			t.Errorf("Field2 = %d, want 0", result.Data.Field2)
		}
	})
}

func TestTruncateForError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		maxLen  int
		want    string
	}{
		{
			name:    "short content",
			content: "short",
			maxLen:  100,
			want:    "short",
		},
		{
			name:    "exact length",
			content: "exactly20characters!",
			maxLen:  20,
			want:    "exactly20characters!",
		},
		{
			name:    "needs truncation",
			content: "this is a very long string that needs to be truncated for error messages",
			maxLen:  20,
			want:    "this is a very long ...[truncated]",
		},
		{
			name:    "empty string",
			content: "",
			maxLen:  100,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateForError(tt.content, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateForError() = %q, want %q", got, tt.want)
			}
		})
	}
}
