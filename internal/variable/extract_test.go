package variable

import "testing"

func TestExtractJSONPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		jsonStr  string
		path     string
		expected string
	}{
		{
			name:     "simple field",
			jsonStr:  `{"name": "test"}`,
			path:     "name",
			expected: "test",
		},
		{
			name:     "nested field",
			jsonStr:  `{"data": {"id": 123}}`,
			path:     "data.id",
			expected: "123",
		},
		{
			name:     "array index",
			jsonStr:  `{"items": [{"x": 1}, {"x": 2}]}`,
			path:     "items.0.x",
			expected: "1",
		},
		{
			name:     "second array element",
			jsonStr:  `{"items": [{"x": 1}, {"x": 2}]}`,
			path:     "items.1.x",
			expected: "2",
		},
		{
			name:     "missing path returns empty",
			jsonStr:  `{"a": "b"}`,
			path:     "missing",
			expected: "",
		},
		{
			name:     "array returns JSON",
			jsonStr:  `{"arr": [1, 2, 3]}`,
			path:     "arr",
			expected: "[1, 2, 3]",
		},
		{
			name:     "object returns JSON",
			jsonStr:  `{"obj": {"a": 1, "b": 2}}`,
			path:     "obj",
			expected: `{"a": 1, "b": 2}`,
		},
		{
			name:     "empty path returns original",
			jsonStr:  `{"foo": "bar"}`,
			path:     "",
			expected: `{"foo": "bar"}`,
		},
		{
			name:     "deeply nested",
			jsonStr:  `{"a": {"b": {"c": {"d": "deep"}}}}`,
			path:     "a.b.c.d",
			expected: "deep",
		},
		{
			name:     "boolean value",
			jsonStr:  `{"active": true}`,
			path:     "active",
			expected: "true",
		},
		{
			name:     "null value",
			jsonStr:  `{"value": null}`,
			path:     "value",
			expected: "",
		},
		{
			name:     "status field common pattern",
			jsonStr:  `{"status": "complete", "data": {"score": 95}}`,
			path:     "data.score",
			expected: "95",
		},
		{
			name:     "non-JSON input returns original",
			jsonStr:  "plain text content",
			path:     "field",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ExtractJSONPath(tt.jsonStr, tt.path)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
