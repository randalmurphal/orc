package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAgentsMD_BasicContent(t *testing.T) {
	dir := t.TempDir()
	content := BuildAgentsMDContent(
		"Follow the rules.",
		"Task: TASK-001\nPhase: implement",
		nil,
		"",
	)

	if err := WriteAgentsMD(dir, content); err != nil {
		t.Fatalf("WriteAgentsMD failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "# Constitution") {
		t.Error("expected Constitution section")
	}
	if !strings.Contains(text, "Follow the rules.") {
		t.Error("expected constitution content")
	}
	if !strings.Contains(text, "# Phase Context") {
		t.Error("expected Phase Context section")
	}
	if !strings.Contains(text, "Task: TASK-001") {
		t.Error("expected task ID in phase context")
	}
}

func TestWriteAgentsMD_AllSections(t *testing.T) {
	dir := t.TempDir()
	content := BuildAgentsMDContent(
		"Be thorough.",
		"Phase: review",
		[]string{"Check for bugs.", "Verify tests."},
		"Run linter before approving.",
	)

	if err := WriteAgentsMD(dir, content); err != nil {
		t.Fatalf("WriteAgentsMD failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	text := string(data)
	if !strings.Contains(text, "# Constitution") {
		t.Error("expected Constitution section")
	}
	if !strings.Contains(text, "# Phase Context") {
		t.Error("expected Phase Context section")
	}
	if !strings.Contains(text, "# Agent Prompts") {
		t.Error("expected Agent Prompts section")
	}
	if !strings.Contains(text, "## Agent 1") {
		t.Error("expected Agent 1 subsection")
	}
	if !strings.Contains(text, "Check for bugs.") {
		t.Error("expected first agent prompt")
	}
	if !strings.Contains(text, "## Agent 2") {
		t.Error("expected Agent 2 subsection")
	}
	if !strings.Contains(text, "Verify tests.") {
		t.Error("expected second agent prompt")
	}
	if !strings.Contains(text, "# Additional Instructions") {
		t.Error("expected Additional Instructions section")
	}
	if !strings.Contains(text, "Run linter before approving.") {
		t.Error("expected extra instructions content")
	}
}

func TestWriteAgentsMD_EmptySectionsOmitted(t *testing.T) {
	dir := t.TempDir()
	content := BuildAgentsMDContent("", "Phase: spec", nil, "")

	if err := WriteAgentsMD(dir, content); err != nil {
		t.Fatalf("WriteAgentsMD failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	text := string(data)
	if strings.Contains(text, "# Constitution") {
		t.Error("empty constitution should be omitted")
	}
	if !strings.Contains(text, "# Phase Context") {
		t.Error("expected Phase Context section")
	}
	if strings.Contains(text, "# Agent Prompts") {
		t.Error("empty agent prompts should be omitted")
	}
	if strings.Contains(text, "# Additional Instructions") {
		t.Error("empty extra instructions should be omitted")
	}
}

func TestWriteAgentsMD_TruncationOrder(t *testing.T) {
	dir := t.TempDir()

	// Create content where extra instructions push us over 32KB.
	// Constitution + phase context are small; extra instructions are huge.
	constitution := "Do not break things."
	phaseContext := "Phase: implement"
	bigExtra := strings.Repeat("X", 40*1024) // 40KB of extra instructions

	content := BuildAgentsMDContent(constitution, phaseContext, nil, bigExtra)

	if err := WriteAgentsMD(dir, content); err != nil {
		t.Fatalf("WriteAgentsMD failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(data) > maxAgentsMDBytes {
		t.Errorf("AGENTS.md size = %d, want <= %d", len(data), maxAgentsMDBytes)
	}

	text := string(data)
	// Constitution must be preserved in full
	if !strings.Contains(text, constitution) {
		t.Error("constitution should be preserved in full")
	}
	// Phase context must be preserved in full
	if !strings.Contains(text, phaseContext) {
		t.Error("phase context should be preserved in full")
	}
}

func TestWriteAgentsMD_TruncatesAgentPromptsAfterExtra(t *testing.T) {
	dir := t.TempDir()

	constitution := "Rule 1."
	phaseContext := "Phase: review"
	// Big agent prompts + big extra instructions
	bigAgents := []string{strings.Repeat("A", 20*1024)}
	bigExtra := strings.Repeat("E", 20*1024)

	content := BuildAgentsMDContent(constitution, phaseContext, bigAgents, bigExtra)

	if err := WriteAgentsMD(dir, content); err != nil {
		t.Fatalf("WriteAgentsMD failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if len(data) > maxAgentsMDBytes {
		t.Errorf("AGENTS.md size = %d, want <= %d", len(data), maxAgentsMDBytes)
	}

	text := string(data)
	// Constitution must survive
	if !strings.Contains(text, "Rule 1.") {
		t.Error("constitution should be preserved")
	}
	// Phase context must survive
	if !strings.Contains(text, "Phase: review") {
		t.Error("phase context should be preserved")
	}
}

func TestWriteAgentsMD_EmptyDir(t *testing.T) {
	err := WriteAgentsMD("", AgentsMDContent{PhaseContext: "test"})
	if err == nil {
		t.Error("expected error for empty directory")
	}
}

func TestBuildAgentsMDContent(t *testing.T) {
	c := BuildAgentsMDContent("const", "phase", []string{"a1", "a2"}, "extra")
	if c.Constitution != "const" {
		t.Errorf("Constitution = %q", c.Constitution)
	}
	if c.PhaseContext != "phase" {
		t.Errorf("PhaseContext = %q", c.PhaseContext)
	}
	if len(c.AgentPrompts) != 2 {
		t.Errorf("AgentPrompts len = %d", len(c.AgentPrompts))
	}
	if c.ExtraInstructions != "extra" {
		t.Errorf("ExtraInstructions = %q", c.ExtraInstructions)
	}
}
