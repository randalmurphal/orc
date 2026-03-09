package executor

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// lookupJSONPathValue returns the value at a dot-separated path inside JSON text.
func lookupJSONPathValue(content string, path string) (any, bool, error) {
	var parsed any
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, false, fmt.Errorf("parse JSON: %w", err)
	}
	return lookupJSONPath(parsed, path)
}

func lookupJSONPath(value any, path string) (any, bool, error) {
	if strings.TrimSpace(path) == "" {
		return value, true, nil
	}

	current := value
	for _, part := range strings.Split(path, ".") {
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		next, ok := obj[part]
		if !ok {
			return nil, false, nil
		}
		current = next
	}

	return current, true, nil
}

func stringifyJSONValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", typed)
	}
}
