// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
	quiet   bool
	jsonOut bool
	plain   bool // Disable emoji/unicode for terminal compatibility
)

// Command group IDs
const (
	groupCore         = "core"
	groupTaskMgmt     = "task"
	groupInspection   = "inspect"
	groupPhaseControl = "phase"
	groupPlanning     = "planning"
	groupConfig       = "config"
	groupGit          = "git"
	groupImportExport = "io"
	groupAdvanced     = "advanced"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "orc",
	Short: "Intelligent Claude Code orchestrator",
	Long: `orc orchestrates Claude Code tasks with appropriate rigor based on task weight.

Features:
  • Task classification (trivial, small, medium, large, greenfield)
  • Phase-based execution with git checkpoints
  • Quality gates that scale with task complexity
  • Full visibility via transcripts and UI
  • Rewindable to any checkpoint

Quick start:
  orc init                    Initialize orc in current project
  orc new "Fix login bug"     Create a new task
  orc run TASK-001            Execute the task
  orc status                  Show current state`,
	SilenceUsage: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is .orc/config.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "output as JSON")
	rootCmd.PersistentFlags().BoolVar(&plain, "plain", false, "plain output without emoji (for terminal compatibility)")

	// Add command groups
	rootCmd.AddGroup(
		&cobra.Group{ID: groupCore, Title: "Core Commands:"},
		&cobra.Group{ID: groupTaskMgmt, Title: "Task Management:"},
		&cobra.Group{ID: groupInspection, Title: "Inspection:"},
		&cobra.Group{ID: groupPhaseControl, Title: "Phase Control:"},
		&cobra.Group{ID: groupPlanning, Title: "Planning & Orchestration:"},
		&cobra.Group{ID: groupConfig, Title: "Configuration:"},
		&cobra.Group{ID: groupGit, Title: "Git & Branches:"},
		&cobra.Group{ID: groupImportExport, Title: "Import/Export:"},
		&cobra.Group{ID: groupAdvanced, Title: "Team & Advanced:"},
	)

	// Core Commands
	addCmd(newInitCmd(), groupCore)
	addCmd(newNewCmd(), groupCore)
	addCmd(newRunCmd(), groupCore)
	addCmd(newStatusCmd(), groupCore)
	addCmd(newListCmd(), groupCore)

	// Task Management
	addCmd(newShowCmd(), groupTaskMgmt)
	addCmd(newEditCmd(), groupTaskMgmt)
	addCmd(newDeleteCmd(), groupTaskMgmt)
	addCmd(newPauseCmd(), groupTaskMgmt)
	addCmd(newResumeCmd(), groupTaskMgmt)
	addCmd(newStopCmd(), groupTaskMgmt)
	addCmd(newCleanupCmd(), groupTaskMgmt)

	// Inspection
	addCmd(newLogCmd(), groupInspection)
	addCmd(newDiffCmd(), groupInspection)
	addCmd(newDepsCmd(), groupInspection)
	addCmd(newSearchCmd(), groupInspection)

	// Phase Control
	addCmd(newRewindCmd(), groupPhaseControl)
	addCmd(newResetCmd(), groupPhaseControl)
	addCmd(newResolveCmd(), groupPhaseControl)
	addCmd(newFinalizeCmd(), groupPhaseControl)
	addCmd(newSkipCmd(), groupPhaseControl)
	addCmd(newApproveCmd(), groupPhaseControl)
	addCmd(newRejectCmd(), groupPhaseControl)

	// Planning & Orchestration
	addCmd(newSetupCmd(), groupPlanning)
	addCmd(newInitiativeCmd(), groupPlanning)

	// Configuration
	addCmd(newConfigCmd(), groupConfig)
	addCmd(newConstitutionCmd(), groupConfig)
	addCmd(newDocsCmd(), groupConfig)
	addCmd(newTemplateCmd(), groupConfig)
	addCmd(newServeCmd(), groupConfig)

	// Git & Branches
	addCmd(newStagingCmd(), groupGit)
	addCmd(newBranchesCmd(), groupGit)

	// Import/Export
	addCmd(newExportCmd(), groupImportExport)
	addCmd(newImportCmd(), groupImportExport)

	// Team & Advanced
	addCmd(newTeamCmd(), groupAdvanced)
	addCmd(newPoolCmd(), groupAdvanced)
	addCmd(newAutomationCmd(), groupAdvanced)
	addCmd(newKnowledgeCmd(), groupAdvanced)
	addCmd(newCommentCmd(), groupAdvanced)
	addCmd(newProjectsCmd(), groupAdvanced)
	addCmd(newVersionCmd(), groupAdvanced)
	addCmd(newGoodbyeCmd(), groupAdvanced)
}

// addCmd adds a command to root with the specified group
func addCmd(cmd *cobra.Command, groupID string) {
	cmd.GroupID = groupID
	rootCmd.AddCommand(cmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		// Search for config in .orc directory
		viper.AddConfigPath(".orc")
		viper.AddConfigPath("$HOME/.orc")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("ORC")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}
