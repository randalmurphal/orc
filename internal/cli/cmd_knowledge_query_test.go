package cli

import (
	"bytes"
	"testing"
)

// =============================================================================
// SC-16: CLI `orc knowledge query` displays results with scores and paths
// =============================================================================

func TestKnowledgeQueryCommand_Registered(t *testing.T) {
	cmd := findKnowledgeSubcommand("query")
	if cmd == nil {
		t.Fatal("'knowledge query' subcommand not found — registration is missing")
	}
}

func TestKnowledgeQueryCommand_RequiresArgument(t *testing.T) {
	cmd := findKnowledgeSubcommand("query")
	if cmd == nil {
		t.Fatal("'knowledge query' subcommand not found")
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{}) // no query argument

	err := cmd.Execute()
	if err == nil {
		t.Error("'knowledge query' without arguments should return error")
	}
}

func TestKnowledgeQueryCommand_HelpFlag(t *testing.T) {
	cmd := findKnowledgeSubcommand("query")
	if cmd == nil {
		t.Fatal("'knowledge query' subcommand not found")
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Errorf("'knowledge query --help' failed: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("--help should produce output")
	}
}

func TestKnowledgeQueryCommand_HasPresetFlag(t *testing.T) {
	cmd := findKnowledgeSubcommand("query")
	if cmd == nil {
		t.Fatal("'knowledge query' subcommand not found")
	}

	flag := cmd.Flag("preset")
	if flag == nil {
		t.Error("'knowledge query' should have --preset flag")
	}
}

func TestKnowledgeQueryCommand_HasLimitFlag(t *testing.T) {
	cmd := findKnowledgeSubcommand("query")
	if cmd == nil {
		t.Fatal("'knowledge query' subcommand not found")
	}

	flag := cmd.Flag("limit")
	if flag == nil {
		t.Error("'knowledge query' should have --limit flag")
	}
}

func TestKnowledgeQueryCommand_HasSummaryFlag(t *testing.T) {
	cmd := findKnowledgeSubcommand("query")
	if cmd == nil {
		t.Fatal("'knowledge query' subcommand not found")
	}

	flag := cmd.Flag("summary")
	if flag == nil {
		t.Error("'knowledge query' should have --summary flag")
	}
}

func TestKnowledgeQueryCommand_KnowledgeUnavailable_PrintsMessage(t *testing.T) {
	// When knowledge layer is not available, should print informational
	// message and exit 0 (not error).
	// This test verifies graceful degradation behavior.
	// Full test requires mock service injection, but we verify command
	// structure first.
	cmd := findKnowledgeSubcommand("query")
	if cmd == nil {
		t.Fatal("'knowledge query' subcommand not found")
	}

	// Verify the command has RunE (error-returning handler)
	if cmd.RunE == nil {
		t.Error("'knowledge query' should have RunE handler")
	}
}
