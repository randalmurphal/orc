package variable

import (
	"github.com/tidwall/gjson"
)

// ExtractJSONPath extracts a value from JSON using gjson syntax.
//
// Behavior:
//   - Empty path returns the original string unchanged
//   - Path not found returns empty string (not an error)
//   - Array/object results are returned as JSON strings
//   - Scalar results are returned as strings
//   - Non-JSON input with non-empty path returns empty string
//
// gjson path syntax examples:
//   - "name" - field access
//   - "data.id" - nested field
//   - "items.0" - array index
//   - "items.#" - array length
//   - "items.#(status==\"active\")#" - filtered array
func ExtractJSONPath(jsonStr, path string) string {
	if path == "" {
		return jsonStr
	}

	// gjson.Get handles invalid JSON gracefully (returns empty result)
	result := gjson.Get(jsonStr, path)
	if !result.Exists() {
		return ""
	}

	// Preserve JSON structure for arrays/objects
	if result.IsArray() || result.IsObject() {
		return result.Raw
	}

	return result.String()
}
