package controlplane

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/randalmurphal/orc/internal/db"
)

func TestSchemaRoundTrip(t *testing.T) {
	t.Parallel()

	fixtures := []struct {
		name      string
		value     any
		jsonField string
	}{
		{
			name: "recommendation candidate",
			value: RecommendationCandidate{
				Kind:           "cleanup",
				Title:          "Tighten retries",
				Summary:        "The current retry path is noisy.",
				ProposedAction: "Consolidate the retry branch.",
				Evidence:       "Three separate retries diverged in review.",
				DedupeKey:      "cleanup:retry-path",
			},
			jsonField: `"proposed_action":"Consolidate the retry branch."`,
		},
		{
			name: "attention signal",
			value: AttentionSignal{
				Kind:    "blocked_task",
				TaskID:  "TASK-101",
				Title:   "Wait on schema review",
				Status:  "blocked",
				Phase:   "review",
				Summary: "Schema owner approval is still pending.",
			},
			jsonField: `"task_id":"TASK-101"`,
		},
		{
			name: "promoted draft",
			value: PromotedDraft{
				TargetType: "task",
				Title:      "Follow up on schema cleanups",
				Summary:    "Turn the cleanup notes into a queued task.",
				Content:    "Investigate the duplicate schema builder.",
			},
			jsonField: `"target_type":"task"`,
		},
		{
			name: "handoff pack",
			value: HandoffPack{
				TaskID:        "TASK-813",
				TaskTitle:     "Control-plane contracts",
				CurrentPhase:  "implement",
				Summary:       "Wire the new builtin variables before template work starts.",
				NextSteps:     []string{"Run resolver tests", "Verify executor enrichment"},
				OpenQuestions: []string{"Should handoff context include review findings by default?"},
				Risks:         []string{"Prompt budget regression if summaries grow unchecked"},
				Drafts: []PromotedDraft{
					{
						TargetType: "decision",
						Title:      "Configurable control-plane limits",
						Summary:    "Promote hard-coded limits into config later.",
						Content:    "Add config once usage stabilizes.",
					},
				},
			},
			jsonField: `"current_phase":"implement"`,
		},
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture.name, func(t *testing.T) {
			data, err := json.Marshal(fixture.value)
			if err != nil {
				t.Fatalf("marshal %s: %v", fixture.name, err)
			}
			if !strings.Contains(string(data), fixture.jsonField) {
				t.Fatalf("marshal %s missing expected field %s in %s", fixture.name, fixture.jsonField, string(data))
			}

			roundTrip := reflect.New(reflect.TypeOf(fixture.value))
			if err := json.Unmarshal(data, roundTrip.Interface()); err != nil {
				t.Fatalf("unmarshal %s: %v", fixture.name, err)
			}

			if !reflect.DeepEqual(fixture.value, roundTrip.Elem().Interface()) {
				t.Fatalf("round-trip mismatch\nwant: %#v\ngot:  %#v", fixture.value, roundTrip.Elem().Interface())
			}
		})
	}
}

func TestFormatRecommendationSummary(t *testing.T) {
	t.Parallel()

	output := FormatRecommendationSummary([]RecommendationCandidate{
		{
			Kind:           "cleanup",
			Title:          "Tighten retry contracts",
			Summary:        "The retry path needs one schema entrypoint.",
			ProposedAction: "Use the shared schema helper everywhere.",
			Evidence:       "Review found duplicate implementations.",
			DedupeKey:      "cleanup:retry-contracts",
		},
	})

	if output == "" {
		t.Fatal("FormatRecommendationSummary returned empty output")
	}
	if !strings.Contains(output, "## Pending Recommendations") {
		t.Fatalf("recommendation summary missing header: %s", output)
	}
	if !strings.Contains(output, "Tighten retry contracts") {
		t.Fatalf("recommendation summary missing title: %s", output)
	}
}

func TestFormatAttentionSummary(t *testing.T) {
	t.Parallel()

	output := FormatAttentionSummary([]AttentionSignal{
		{
			Kind:    "blocked_task",
			TaskID:  "TASK-201",
			Title:   "Schema review blocked",
			Status:  "blocked",
			Phase:   "review",
			Summary: "Waiting on schema approval.",
		},
	})

	if output == "" {
		t.Fatal("FormatAttentionSummary returned empty output")
	}
	if !strings.Contains(output, "## Attention Summary") {
		t.Fatalf("attention summary missing header: %s", output)
	}
	if !strings.Contains(output, "TASK-201") {
		t.Fatalf("attention summary missing task id: %s", output)
	}
}

func TestFormatHandoffPack(t *testing.T) {
	t.Parallel()

	output := FormatHandoffPack(HandoffPack{
		TaskID:        "TASK-813",
		TaskTitle:     "Control-plane contracts",
		CurrentPhase:  "implement",
		Summary:       "Context is ready for the next actor.",
		NextSteps:     []string{"Run targeted tests"},
		OpenQuestions: []string{"Should templates opt in per phase?"},
		Risks:         []string{"Prompt bloat if this grows without limits"},
	})

	if output == "" {
		t.Fatal("FormatHandoffPack returned empty output")
	}
	if !strings.Contains(output, "## Handoff Pack") {
		t.Fatalf("handoff pack missing header: %s", output)
	}
	if !strings.Contains(output, "Context is ready for the next actor.") {
		t.Fatalf("handoff pack missing summary: %s", output)
	}
}

func TestLimitsRecommendationSummary(t *testing.T) {
	t.Parallel()

	if got := FormatRecommendationSummary(nil); got != "" {
		t.Fatalf("FormatRecommendationSummary(nil) = %q, want empty string", got)
	}

	items := make([]RecommendationCandidate, 0, 50)
	for i := 0; i < 50; i++ {
		items = append(items, RecommendationCandidate{
			Kind:           "cleanup",
			Title:          strings.Repeat("Title ", 20),
			Summary:        strings.Repeat("Summary text ", 40),
			ProposedAction: strings.Repeat("Proposed action ", 20),
			Evidence:       strings.Repeat("Evidence ", 20),
			DedupeKey:      "cleanup:item",
		})
	}

	output := FormatRecommendationSummary(items)
	if output == "" {
		t.Fatal("expected truncated recommendation summary, got empty string")
	}
	if len([]byte(output)) > MaxRecommendationSummaryBytes {
		t.Fatalf("recommendation summary length = %d, want <= %d", len([]byte(output)), MaxRecommendationSummaryBytes)
	}
	if !strings.HasSuffix(output, "more items") {
		t.Fatalf("recommendation summary missing omission notice: %s", output)
	}

	attentionOutput := FormatAttentionSummary(buildAttentionSignals(50))
	if len([]byte(attentionOutput)) > MaxAttentionSummaryBytes {
		t.Fatalf("attention summary length = %d, want <= %d", len([]byte(attentionOutput)), MaxAttentionSummaryBytes)
	}
	if !strings.HasSuffix(attentionOutput, "more items") {
		t.Fatalf("attention summary missing omission notice: %s", attentionOutput)
	}

	handoffOutput := FormatHandoffPack(HandoffPack{
		Summary:       "handoff",
		NextSteps:     repeatedList(200, strings.Repeat("next step ", 20)),
		OpenQuestions: repeatedList(200, strings.Repeat("question ", 20)),
		Risks:         repeatedList(200, strings.Repeat("risk ", 20)),
	})
	if len([]byte(handoffOutput)) > MaxHandoffPackBytes {
		t.Fatalf("handoff pack length = %d, want <= %d", len([]byte(handoffOutput)), MaxHandoffPackBytes)
	}
	if !strings.HasSuffix(handoffOutput, "more items") {
		t.Fatalf("handoff pack missing omission notice: %s", handoffOutput)
	}
}

func TestSchemaParity(t *testing.T) {
	t.Parallel()

	candidateType := reflect.TypeOf(RecommendationCandidate{})
	dbType := reflect.TypeOf(db.Recommendation{})

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

func buildAttentionSignals(count int) []AttentionSignal {
	signals := make([]AttentionSignal, 0, count)
	for i := 0; i < count; i++ {
		signals = append(signals, AttentionSignal{
			Kind:    "blocked_task",
			TaskID:  "TASK-LIMIT",
			Title:   strings.Repeat("Blocked task ", 10),
			Status:  "blocked",
			Phase:   "review",
			Summary: strings.Repeat("Needs attention ", 20),
		})
	}
	return signals
}

func repeatedList(count int, value string) []string {
	items := make([]string, 0, count)
	for i := 0; i < count; i++ {
		items = append(items, value)
	}
	return items
}

func jsonFieldName(field reflect.StructField) string {
	return strings.Split(field.Tag.Get("json"), ",")[0]
}

func snakeCase(value string) string {
	var builder strings.Builder
	for index, r := range value {
		if unicode.IsUpper(r) {
			if index > 0 {
				builder.WriteByte('_')
			}
			builder.WriteRune(unicode.ToLower(r))
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}
