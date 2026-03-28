package controlplane

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestHandoffContextPackBuildersReturnContent(t *testing.T) {
	t.Parallel()

	taskOutput := BuildTaskContextPack(
		&orcv1.Task{
			Id:           "TASK-001",
			Title:        "Ship handoff actions",
			Description:  stringPtr("Add CLI handoff actions to the control plane."),
			CurrentPhase: stringPtr("implement"),
		},
		"implement",
		[]*orcv1.Recommendation{
			{
				SourceTaskId:   "TASK-001",
				Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
				Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP,
				ProposedAction: "Wire the API into the task detail page.",
			},
		},
	)
	if taskOutput == "" {
		t.Fatal("task handoff pack should not be empty")
	}

	threadOutput := BuildThreadContextPack(&orcv1.Thread{
		Id:       "THR-001",
		Title:    "Operator follow-up",
		Status:   "active",
		TaskId:   "TASK-001",
		Messages: []*orcv1.ThreadMessage{{Role: "user", Content: "Please continue the rollout plan."}},
	})
	if threadOutput == "" {
		t.Fatal("thread handoff pack should not be empty")
	}

	recommendationOutput := BuildRecommendationContextPack(&orcv1.Recommendation{
		Id:             "REC-001",
		Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_CLEANUP,
		Title:          "Remove duplicate polling",
		Summary:        "Two reload loops race each other.",
		ProposedAction: "Collapse onto one signal subscription.",
		Evidence:       "Dashboard emits duplicate fetches after every event.",
		SourceTaskId:   "TASK-001",
		SourceRunId:    "RUN-001",
	})
	if recommendationOutput == "" {
		t.Fatal("recommendation handoff pack should not be empty")
	}

	attentionOutput := BuildAttentionItemContextPack(&orcv1.AttentionItem{
		Id:               "proj-001:failed-TASK-001",
		Type:             orcv1.AttentionItemType_ATTENTION_ITEM_TYPE_FAILED_TASK,
		Title:            "TASK-001 failed",
		Description:      "Verification blew up in review.",
		TaskId:           "TASK-001",
		AvailableActions: []orcv1.AttentionAction{orcv1.AttentionAction_ATTENTION_ACTION_RETRY},
	})
	if attentionOutput == "" {
		t.Fatal("attention item handoff pack should not be empty")
	}
}

func TestHandoffContextPackLimit(t *testing.T) {
	t.Parallel()

	recommendations := make([]*orcv1.Recommendation, 0, 200)
	for i := 0; i < 200; i++ {
		recommendations = append(recommendations, &orcv1.Recommendation{
			Id:             "REC-" + strings.Repeat("1", i%9+1),
			SourceTaskId:   "TASK-001",
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP,
			ProposedAction: strings.Repeat("follow-up step ", 16) + string(rune('A'+(i%26))) + strings.Repeat("x", i),
		})
	}

	output := BuildTaskContextPack(
		&orcv1.Task{
			Id:           "TASK-001",
			Title:        "Large handoff pack",
			Description:  stringPtr(strings.Repeat("summary ", 32)),
			CurrentPhase: stringPtr("review"),
		},
		"review",
		recommendations,
	)

	if len([]byte(output)) > MaxHandoffPackBytes {
		t.Fatalf("output length = %d, want <= %d", len([]byte(output)), MaxHandoffPackBytes)
	}
	if !strings.Contains(output, "... and ") {
		t.Fatalf("expected omission notice in output: %s", output)
	}
}

func TestCLICommand(t *testing.T) {
	t.Parallel()

	prompt := "Need \"quotes\"\n$HOME `rm -rf /` and 'single quotes'"

	claudeCommand, err := BuildCLICommand(HandoffTargetClaudeCode, prompt)
	if err != nil {
		t.Fatalf("BuildCLICommand(claude): %v", err)
	}
	if !strings.HasPrefix(claudeCommand, "claude -p ") {
		t.Fatalf("claude command = %q, want prefix %q", claudeCommand, "claude -p ")
	}
	assertShellArgumentRoundTrip(t, strings.TrimPrefix(claudeCommand, "claude -p "), prompt)

	codexCommand, err := BuildCLICommand(HandoffTargetCodex, prompt)
	if err != nil {
		t.Fatalf("BuildCLICommand(codex): %v", err)
	}
	if !strings.HasPrefix(codexCommand, "codex ") {
		t.Fatalf("codex command = %q, want prefix %q", codexCommand, "codex ")
	}
	assertShellArgumentRoundTrip(t, strings.TrimPrefix(codexCommand, "codex "), prompt)
}

func TestBootstrapPrompt(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		sourceType   HandoffSourceKind
		wantContains string
	}{
		{name: "task", sourceType: HandoffSourceTask, wantContains: "Continue the task"},
		{name: "thread", sourceType: HandoffSourceThread, wantContains: "Continue the thread"},
		{name: "recommendation", sourceType: HandoffSourceRecommendation, wantContains: "Continue the recommendation"},
		{name: "attention_item", sourceType: HandoffSourceAttentionItem, wantContains: "Continue the attention item"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			prompt, err := BuildBootstrapPrompt(tc.sourceType, "## Context\nSummary: keep going")
			if err != nil {
				t.Fatalf("BuildBootstrapPrompt(%s): %v", tc.sourceType, err)
			}
			if !strings.Contains(prompt, "<context>\n## Context\nSummary: keep going\n</context>") {
				t.Fatalf("prompt missing context wrapper: %s", prompt)
			}
			if !strings.Contains(prompt, tc.wantContains) {
				t.Fatalf("prompt missing source instruction %q: %s", tc.wantContains, prompt)
			}
		})
	}
}

func TestTaskPackParity(t *testing.T) {
	t.Parallel()

	taskItem := &orcv1.Task{
		Id:           "TASK-001",
		Title:        "Parity check",
		Description:  stringPtr("Make sure the task handoff pack stays aligned."),
		CurrentPhase: stringPtr("implement"),
	}
	recommendations := []*orcv1.Recommendation{
		{
			SourceTaskId:   "TASK-001",
			Status:         orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP,
			ProposedAction: "Ship the UI action.",
		},
		{
			SourceTaskId: "TASK-001",
			Status:       orcv1.RecommendationStatus_RECOMMENDATION_STATUS_PENDING,
			Kind:         orcv1.RecommendationKind_RECOMMENDATION_KIND_RISK,
			Title:        "Clipboard errors",
			Summary:      "Clipboard writes fail outside secure contexts.",
		},
	}

	pack := BuildTaskHandoffPack(taskItem, "implement", recommendations)
	formatted := FormatHandoffPack(pack)
	built := BuildTaskContextPack(taskItem, "implement", recommendations)

	if built != formatted {
		t.Fatalf("task context pack parity mismatch\nformatted:\n%s\n\nbuilt:\n%s", formatted, built)
	}

	for _, section := range []string{"Task:", "Current phase:", "Summary:", "Next step:", "Risk:"} {
		if !strings.Contains(built, section) {
			t.Fatalf("task handoff pack missing section %q: %s", section, built)
		}
	}
}

func assertShellArgumentRoundTrip(t *testing.T, quotedArg string, want string) {
	t.Helper()

	scriptFile, err := os.CreateTemp(t.TempDir(), "handoff-shell-*.sh")
	if err != nil {
		t.Fatalf("CreateTemp() failed: %v", err)
	}
	script := "set -- " + quotedArg + "\nprintf '%s' \"$1\""
	if err := os.WriteFile(scriptFile.Name(), []byte(script), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) failed: %v", scriptFile.Name(), err)
	}

	cmd := exec.Command("sh", scriptFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shell round-trip failed: %v\noutput: %s", err, output)
	}
	if string(output) != want {
		t.Fatalf("shell round-trip output = %q, want %q", string(output), want)
	}
}

func stringPtr(value string) *string {
	return &value
}

func TestThreadHandoffTruncationUsesOmissionNotice(t *testing.T) {
	t.Parallel()

	messages := make([]*orcv1.ThreadMessage, 0, 40)
	for i := 0; i < 40; i++ {
		messages = append(messages, &orcv1.ThreadMessage{
			Role:    "assistant",
			Content: strings.Repeat("message body ", 30),
		})
	}

	output := BuildThreadContextPack(&orcv1.Thread{
		Id:       "THR-001",
		Title:    "Large thread",
		Status:   "active",
		TaskId:   "TASK-001",
		Messages: messages,
	})
	if len([]byte(output)) > MaxHandoffPackBytes {
		t.Fatalf("thread output length = %d, want <= %d", len([]byte(output)), MaxHandoffPackBytes)
	}
}

func TestRecommendationContextIncludesPromotion(t *testing.T) {
	t.Parallel()

	output := BuildRecommendationContextPack(&orcv1.Recommendation{
		Id:             "REC-001",
		Kind:           orcv1.RecommendationKind_RECOMMENDATION_KIND_FOLLOW_UP,
		Title:          "Promote a task",
		Summary:        "The work should become a tracked task.",
		ProposedAction: "Create TASK-099.",
		Evidence:       "Operator asked for a concrete backlog item.",
		SourceTaskId:   "TASK-001",
		SourceRunId:    "RUN-001",
		SourceThreadId: "THR-001",
		PromotedToType: "task",
		PromotedToId:   "TASK-099",
		UpdatedAt:      timestamppb.Now(),
	})

	for _, fragment := range []string{"Recommendation:", "Source thread: THR-001", "Promoted to: task TASK-099"} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("recommendation output missing %q: %s", fragment, output)
		}
	}
}
