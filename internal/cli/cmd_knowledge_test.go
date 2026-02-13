package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// SC-18: CLI commands registered and callable with --help.
func TestKnowledgeCommand_Registered(t *testing.T) {
	// Verify "knowledge" command exists on root
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "knowledge" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("'knowledge' command not found on root — registration is missing")
	}
}

// SC-18: knowledge start subcommand exists.
func TestKnowledgeCommand_StartSubcommand(t *testing.T) {
	cmd := findKnowledgeSubcommand("start")
	if cmd == nil {
		t.Fatal("'knowledge start' subcommand not found")
	}
}

// SC-18: knowledge stop subcommand exists.
func TestKnowledgeCommand_StopSubcommand(t *testing.T) {
	cmd := findKnowledgeSubcommand("stop")
	if cmd == nil {
		t.Fatal("'knowledge stop' subcommand not found")
	}
}

// SC-18: knowledge status subcommand exists.
func TestKnowledgeCommand_StatusSubcommand(t *testing.T) {
	cmd := findKnowledgeSubcommand("status")
	if cmd == nil {
		t.Fatal("'knowledge status' subcommand not found")
	}
}

// SC-18: --help flag succeeds for each subcommand.
func TestKnowledgeCommand_HelpFlags(t *testing.T) {
	subcmds := []string{"start", "stop", "status"}
	for _, sub := range subcmds {
		t.Run(sub, func(t *testing.T) {
			cmd := findKnowledgeSubcommand(sub)
			if cmd == nil {
				t.Fatalf("'knowledge %s' not found", sub)
			}

			// Execute with --help
			cmd.SetArgs([]string{"--help"})
			err := cmd.Execute()
			if err != nil {
				t.Errorf("'knowledge %s --help' failed: %v", sub, err)
			}
		})
	}
}

// Helper to find a subcommand of the knowledge command.
func findKnowledgeSubcommand(childName string) *cobra.Command {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "knowledge" {
			for _, sub := range cmd.Commands() {
				if sub.Name() == childName {
					return sub
				}
			}
		}
	}
	return nil
}
