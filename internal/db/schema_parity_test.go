package db

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/randalmurphal/orc/internal/controlplane"
)

// TestSchemaParity verifies that shared fields between
// controlplane.RecommendationCandidate and db.Recommendation keep matching JSON
// names. It lives in db/ instead of controlplane/ because db/attention_signal.go
// now imports controlplane, and moving the parity assertion here avoids a test
// import cycle.
func TestSchemaParity(t *testing.T) {
	t.Parallel()

	candidateType := reflect.TypeOf(controlplane.RecommendationCandidate{})
	dbType := reflect.TypeOf(Recommendation{})

	sharedFields := []string{"Kind", "Title", "Summary", "ProposedAction", "Evidence", "DedupeKey"}
	for _, fieldName := range sharedFields {
		candidateField, ok := candidateType.FieldByName(fieldName)
		if !ok {
			t.Fatalf("RecommendationCandidate missing field %s", fieldName)
		}
		dbField, ok := dbType.FieldByName(fieldName)
		if !ok {
			t.Fatalf("db.Recommendation missing field %s", fieldName)
		}

		candidateJSONName := jsonFieldName(candidateField)
		dbJSONName := snakeCase(dbField.Name)
		if candidateJSONName != dbJSONName {
			t.Fatalf("%s json name mismatch: candidate=%s db=%s", fieldName, candidateJSONName, dbJSONName)
		}
	}
}

func jsonFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return snakeCase(f.Name)
	}
	parts := strings.SplitN(tag, ",", 2)
	return parts[0]
}

func snakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
