package workflow

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/templates"
)

func TestParseAgentMarkdown(t *testing.T) {
	content := []byte(`---
name: test-agent
description: A test agent for unit testing
model: opus
tools: ["Read", "Grep"]
---

You are a test agent.

## Instructions

Do test things.
`)

	fm, prompt, err := ParseAgentMarkdown(content)
	if err != nil {
		t.Fatalf("ParseAgentMarkdown failed: %v", err)
	}

	if fm.Name != "test-agent" {
		t.Errorf("Name = %s, want test-agent", fm.Name)
	}
	if fm.Description != "A test agent for unit testing" {
		t.Errorf("Description = %s, want 'A test agent for unit testing'", fm.Description)
	}
	if fm.Model != "opus" {
		t.Errorf("Model = %s, want opus", fm.Model)
	}
	if len(fm.Tools) != 2 || fm.Tools[0] != "Read" || fm.Tools[1] != "Grep" {
		t.Errorf("Tools = %v, want [Read, Grep]", fm.Tools)
	}

	if prompt == "" {
		t.Error("prompt is empty")
	}
	// Prompt should contain the expected content (may have leading whitespace)
	if !strings.Contains(prompt, "You are a test agent") {
		t.Errorf("prompt does not contain 'You are a test agent': %q", prompt[:50])
	}
}

func TestParseAgentMarkdown_MissingFrontmatter(t *testing.T) {
	content := []byte(`No frontmatter here`)

	_, _, err := ParseAgentMarkdown(content)
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestParseAgentMarkdown_MissingClosingDelimiter(t *testing.T) {
	content := []byte(`---
name: test
description: test
`)

	_, _, err := ParseAgentMarkdown(content)
	if err == nil {
		t.Error("expected error for missing closing delimiter")
	}
}

func TestEmbeddedAgentsExist(t *testing.T) {
	for _, file := range builtinAgentFiles {
		content, err := templates.Agents.ReadFile(file)
		if err != nil {
			t.Errorf("Failed to read embedded agent %s: %v", file, err)
			continue
		}

		fm, prompt, err := ParseAgentMarkdown(content)
		if err != nil {
			t.Errorf("Failed to parse agent %s: %v", file, err)
			continue
		}

		if fm.Name == "" {
			t.Errorf("Agent %s has empty name", file)
		}
		if fm.Description == "" {
			t.Errorf("Agent %s has empty description", file)
		}
		if fm.Model == "" {
			t.Errorf("Agent %s has empty model", file)
		}
		if len(fm.Tools) == 0 {
			t.Errorf("Agent %s has no tools", file)
		}
		if prompt == "" {
			t.Errorf("Agent %s has empty prompt", file)
		}
	}
}

func TestSeedAgents(t *testing.T) {
	tmpDir := t.TempDir()
	gdb, err := db.OpenGlobalAt(filepath.Join(tmpDir, "orc.db"))
	if err != nil {
		t.Fatalf("OpenGlobalAt failed: %v", err)
	}
	defer func() { _ = gdb.Close() }()

	// First seed phase templates (required for foreign keys)
	_, err = SeedBuiltins(gdb)
	if err != nil {
		t.Fatalf("SeedBuiltins failed: %v", err)
	}

	// Now seed agents
	seeded, err := SeedAgents(gdb)
	if err != nil {
		t.Fatalf("SeedAgents failed: %v", err)
	}
	if seeded == 0 {
		t.Error("SeedAgents seeded 0 items")
	}

	// Verify agents were created
	agents, err := gdb.ListAgents()
	if err != nil {
		t.Fatalf("ListAgents failed: %v", err)
	}
	if len(agents) < 6 {
		t.Errorf("ListAgents returned %d agents, want at least 6", len(agents))
	}

	// Verify specific agent
	agent, err := gdb.GetAgent("code-reviewer")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if agent == nil {
		t.Fatal("code-reviewer agent not found")
	}
	if agent.Model != "opus" {
		t.Errorf("code-reviewer model = %s, want opus", agent.Model)
	}
	if len(agent.Tools) == 0 {
		t.Error("code-reviewer has no tools")
	}

	// Verify phase agents were created
	reviewAgents, err := gdb.GetPhaseAgents("review")
	if err != nil {
		t.Fatalf("GetPhaseAgents failed: %v", err)
	}
	if len(reviewAgents) < 2 {
		t.Errorf("GetPhaseAgents(review) returned %d, want at least 2", len(reviewAgents))
	}

	// Verify idempotency - seeding again should not create duplicates
	seeded2, err := SeedAgents(gdb)
	if err != nil {
		t.Fatalf("SeedAgents (2nd call) failed: %v", err)
	}
	if seeded2 != 0 {
		t.Errorf("SeedAgents (2nd call) seeded %d items, want 0", seeded2)
	}
}

func TestListBuiltinAgentIDs(t *testing.T) {
	ids := ListBuiltinAgentIDs()
	if len(ids) != 6 {
		t.Errorf("ListBuiltinAgentIDs returned %d, want 6", len(ids))
	}

	expected := map[string]bool{
		// Review agents
		"code-reviewer":         true,
		"code-simplifier":       true,
		"comment-analyzer":      true,
		"pr-test-analyzer":      true,
		"silent-failure-hunter": true,
		"type-design-analyzer":  true,
	}

	for _, id := range ids {
		if !expected[id] {
			t.Errorf("Unexpected agent ID: %s", id)
		}
	}
}
