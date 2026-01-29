package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"

	orcv1 "github.com/randalmurphal/orc/gen/proto/orc/v1"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/jira"
)

func newImportJiraCmd() *cobra.Command {
	var (
		url      string
		email    string
		token    string
		projects []string
		jql      string
		noEpics  bool
		dryRun   bool
		weight   string
		queue    string
	)

	cmd := &cobra.Command{
		Use:   "jira",
		Short: "Import Jira Cloud issues as orc tasks",
		Long: `Import issues from Jira Cloud into orc as tasks.

Epics are mapped to orc initiatives by default (disable with --no-epics).
Re-importing is idempotent — existing tasks are updated, not duplicated.
Tasks that are already running in orc are skipped.

Authentication requires a Jira Cloud API token:
  1. Generate at https://id.atlassian.com/manage-profile/security/api-tokens
  2. Set ORC_JIRA_TOKEN environment variable (recommended)
  3. Or pass --token flag

Configuration can be set in .orc/config.yaml under the 'jira' key:
  jira:
    url: "https://acme.atlassian.net"
    email: "user@acme.com"

Examples:
  orc import jira --url https://acme.atlassian.net --project PROJ
  orc import jira --jql "sprint in openSprints()" --dry-run
  orc import jira --project PROJ --no-epics
  orc import jira --project PROJ --weight small --queue active`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfg, err := config.Load()
			if err != nil {
				// Non-fatal — config may not exist
				cfg = &config.Config{}
			}

			// Resolve auth: flags > env > config
			jiraURL := resolveString(url, "", cfg.Jira.URL)
			jiraEmail := resolveString(email, "ORC_JIRA_EMAIL", cfg.Jira.Email)
			jiraToken := resolveString(token, cfg.Jira.GetTokenEnvVar(), "")

			if jiraURL == "" {
				return fmt.Errorf("jira URL is required: use --url flag or set jira.url in config")
			}
			if jiraEmail == "" {
				return fmt.Errorf("jira email is required: use --email flag or set jira.email in config")
			}
			if jiraToken == "" {
				return fmt.Errorf("jira API token is required: set %s env var or use --token flag", cfg.Jira.GetTokenEnvVar())
			}

			// Create Jira client
			client, err := jira.NewClient(jira.ClientConfig{
				BaseURL:  jiraURL,
				Email:    jiraEmail,
				APIToken: jiraToken,
			})
			if err != nil {
				return fmt.Errorf("create jira client: %w", err)
			}

			// Verify auth
			ctx := context.Background()
			if err := client.CheckAuth(ctx); err != nil {
				return fmt.Errorf("jira authentication failed: %w", err)
			}

			// Get storage backend
			backend, err := getBackend()
			if err != nil {
				return fmt.Errorf("open storage: %w", err)
			}
			defer func() { _ = backend.Close() }()

			// Build import config
			epicToInit := cfg.Jira.GetEpicToInitiative()
			if noEpics {
				epicToInit = false
			}

			mapperCfg := jira.DefaultMapperConfig()
			if w := resolveWeight(weight, cfg.Jira.DefaultWeight); w != orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED {
				mapperCfg.DefaultWeight = w
			}
			if q := resolveQueue(queue, cfg.Jira.DefaultQueue); q != orcv1.TaskQueue_TASK_QUEUE_UNSPECIFIED {
				mapperCfg.DefaultQueue = q
			}

			importCfg := jira.ImportConfig{
				JQL:              jql,
				Projects:         projects,
				EpicToInitiative: epicToInit,
				DryRun:           dryRun,
				MapperCfg:        mapperCfg,
			}

			// Run import
			logger := slog.Default()
			importer := jira.NewImporter(client, backend, importCfg, logger)
			result, err := importer.Run(ctx)
			if err != nil {
				return fmt.Errorf("jira import failed: %w", err)
			}

			// Print results
			printImportResult(result, dryRun)

			if len(result.Errors) > 0 {
				return fmt.Errorf("%d issue(s) failed to import", len(result.Errors))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&url, "url", "", "Jira Cloud URL (e.g., https://acme.atlassian.net)")
	cmd.Flags().StringVar(&email, "email", "", "Email for authentication (or ORC_JIRA_EMAIL)")
	cmd.Flags().StringVar(&token, "token", "", "API token (or ORC_JIRA_TOKEN, recommended)")
	cmd.Flags().StringSliceVar(&projects, "project", nil, "Project key(s) to import from")
	cmd.Flags().StringVar(&jql, "jql", "", "JQL query to filter issues")
	cmd.Flags().BoolVar(&noEpics, "no-epics", false, "Disable epic-to-initiative mapping")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be imported")
	cmd.Flags().StringVar(&weight, "weight", "", "Default weight for imported tasks (trivial|small|medium|large)")
	cmd.Flags().StringVar(&queue, "queue", "", "Default queue for imported tasks (active|backlog)")

	return cmd
}

// resolveString resolves a value from flag, env var, or config (in priority order).
func resolveString(flag, envVar, configVal string) string {
	if flag != "" {
		return flag
	}
	if envVar != "" {
		if v := os.Getenv(envVar); v != "" {
			return v
		}
	}
	return configVal
}

func resolveWeight(flag, configVal string) orcv1.TaskWeight {
	val := flag
	if val == "" {
		val = configVal
	}
	if val == "" {
		return orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED
	}
	switch strings.ToLower(val) {
	case "trivial":
		return orcv1.TaskWeight_TASK_WEIGHT_TRIVIAL
	case "small":
		return orcv1.TaskWeight_TASK_WEIGHT_SMALL
	case "medium":
		return orcv1.TaskWeight_TASK_WEIGHT_MEDIUM
	case "large":
		return orcv1.TaskWeight_TASK_WEIGHT_LARGE
	default:
		return orcv1.TaskWeight_TASK_WEIGHT_UNSPECIFIED
	}
}

func resolveQueue(flag, configVal string) orcv1.TaskQueue {
	val := flag
	if val == "" {
		val = configVal
	}
	if val == "" {
		return orcv1.TaskQueue_TASK_QUEUE_UNSPECIFIED
	}
	switch strings.ToLower(val) {
	case "active":
		return orcv1.TaskQueue_TASK_QUEUE_ACTIVE
	case "backlog":
		return orcv1.TaskQueue_TASK_QUEUE_BACKLOG
	default:
		return orcv1.TaskQueue_TASK_QUEUE_UNSPECIFIED
	}
}

func printImportResult(result *jira.ImportResult, dryRun bool) {
	prefix := ""
	if dryRun {
		prefix = "[dry-run] "
	}

	fmt.Printf("%sJira import complete:\n", prefix)

	if result.InitiativesCreated > 0 || result.InitiativesUpdated > 0 {
		fmt.Printf("  Initiatives: %d created, %d updated\n",
			result.InitiativesCreated, result.InitiativesUpdated)
	}

	fmt.Printf("  Tasks: %d created, %d updated, %d skipped\n",
		result.TasksCreated, result.TasksUpdated, result.TasksSkipped)

	if len(result.Errors) > 0 {
		fmt.Printf("  Errors: %d\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("    %s: %v\n", e.JiraKey, e.Err)
		}
	}
}
