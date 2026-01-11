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

	// Add subcommands
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newNewCmd())
	rootCmd.AddCommand(newListCmd())
	rootCmd.AddCommand(newShowCmd())
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(newPauseCmd())
	rootCmd.AddCommand(newStopCmd())
	rootCmd.AddCommand(newResumeCmd())
	rootCmd.AddCommand(newRewindCmd())
	rootCmd.AddCommand(newStatusCmd())
	rootCmd.AddCommand(newLogCmd())
	rootCmd.AddCommand(newDiffCmd())
	rootCmd.AddCommand(newApproveCmd())
	rootCmd.AddCommand(newRejectCmd())
	rootCmd.AddCommand(newExportCmd())
	rootCmd.AddCommand(newImportCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newServeCmd())
	rootCmd.AddCommand(newSkipCmd())
	rootCmd.AddCommand(newCleanupCmd())
	rootCmd.AddCommand(newProjectsCmd())
	rootCmd.AddCommand(newSessionCmd())
	rootCmd.AddCommand(newCostCmd())
	rootCmd.AddCommand(newTemplateCmd())
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
