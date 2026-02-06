package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// --- SC-11: CLI `orc index` commands ---

// SC-11: `orc index` command is registered on rootCmd.
func TestIndexCmd_Registered(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "index" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("'index' command not found on root — registration is missing")
	}
}

// SC-11: `orc index` has correct usage and short description.
func TestIndexCmd_Usage(t *testing.T) {
	cmd := findCommand(rootCmd, "index")
	if cmd == nil {
		t.Fatal("'index' command not found")
	}

	if cmd.Use == "" {
		t.Error("index command should have Use string")
	}
	if cmd.Short == "" {
		t.Error("index command should have Short description")
	}
}

// SC-11: `orc index --incremental` flag exists.
func TestIndexCmd_IncrementalFlag(t *testing.T) {
	cmd := findCommand(rootCmd, "index")
	if cmd == nil {
		t.Fatal("'index' command not found")
	}

	flag := cmd.Flags().Lookup("incremental")
	if flag == nil {
		t.Fatal("'--incremental' flag not found on index command")
	}
}

// SC-11: `orc index --status` flag exists.
func TestIndexCmd_StatusFlag(t *testing.T) {
	cmd := findCommand(rootCmd, "index")
	if cmd == nil {
		t.Fatal("'index' command not found")
	}

	flag := cmd.Flags().Lookup("status")
	if flag == nil {
		t.Fatal("'--status' flag not found on index command")
	}
}

// SC-11: `orc index --help` succeeds.
func TestIndexCmd_Help(t *testing.T) {
	cmd := findCommand(rootCmd, "index")
	if cmd == nil {
		t.Fatal("'index' command not found")
	}

	cmd.SetArgs([]string{"--help"})
	err := cmd.Execute()
	if err != nil {
		t.Errorf("'index --help' failed: %v", err)
	}
}

// SC-11: Existing `orc knowledge` subcommands are preserved.
func TestIndexCmd_KnowledgeSubcommandsPreserved(t *testing.T) {
	// Ensure adding `orc index` didn't break existing knowledge commands
	expectedSubcmds := []string{"start", "stop", "status", "query"}
	for _, sub := range expectedSubcmds {
		cmd := findKnowledgeSubcommand(sub)
		if cmd == nil {
			t.Errorf("'knowledge %s' subcommand missing — adding index broke knowledge", sub)
		}
	}
}

// --- Test helpers ---

func findCommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}
