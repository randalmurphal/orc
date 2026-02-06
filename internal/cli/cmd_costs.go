// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/db"
)

// newCostsCmd creates the costs command for viewing cost reports.
func newCostsCmd() *cobra.Command {
	var (
		userFilter    string
		projectFilter string
		sinceFilter   string
		byFilter      string
	)

	cmd := &cobra.Command{
		Use:   "costs",
		Short: "View cost report across projects",
		Long: `Display cost data with filtering and grouping.

By default, shows current month costs grouped by project and model.

Examples:
  orc costs                      # Current month summary
  orc costs --by user            # Group by user
  orc costs --by project         # Group by project
  orc costs --by model           # Group by model
  orc costs --user alice         # Filter to specific user
  orc costs --since 2026-01-01   # Filter by date
  orc costs --project proj-orc   # Filter to specific project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCosts(cmd, userFilter, projectFilter, sinceFilter, byFilter)
		},
	}

	cmd.Flags().StringVar(&userFilter, "user", "", "filter to specific user name")
	cmd.Flags().StringVar(&projectFilter, "project", "", "filter to specific project ID")
	cmd.Flags().StringVar(&sinceFilter, "since", "", "filter by date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&byFilter, "by", "", "group by dimension: user, project, model")

	return cmd
}

func runCosts(cmd *cobra.Command, userFilter, projectFilter, sinceFilter, groupBy string) error {
	gdb, err := openGlobalDBForCosts()
	if err != nil {
		return err
	}
	defer func() { _ = gdb.Close() }()

	var sinceTime time.Time
	if sinceFilter != "" {
		sinceTime, err = time.Parse("2006-01-02", sinceFilter)
		if err != nil {
			return fmt.Errorf("invalid date format, use YYYY-MM-DD: %s", sinceFilter)
		}
	} else {
		now := time.Now()
		sinceTime = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	var userID string
	if userFilter != "" {
		user, err := gdb.GetUserByName(userFilter)
		if err != nil {
			return fmt.Errorf("lookup user: %w", err)
		}
		if user == nil {
			return fmt.Errorf("user not found: %s", userFilter)
		}
		userID = user.ID
	}

	filter := db.CostReportFilter{
		UserID:    userID,
		ProjectID: projectFilter,
		Since:     sinceTime,
	}

	if groupBy == "" {
		return displayDefaultCostReport(cmd, gdb, filter)
	}

	filter.GroupBy = groupBy
	result, err := gdb.GetCostReport(filter)
	if err != nil {
		return fmt.Errorf("get cost report: %w", err)
	}

	displayCostReport(cmd, result, groupBy, projectFilter, gdb)
	return nil
}

// displayDefaultCostReport shows project and model breakdowns with a total.
func displayDefaultCostReport(cmd *cobra.Command, gdb *db.GlobalDB, filter db.CostReportFilter) error {
	filter.GroupBy = ""
	total, err := gdb.GetCostReport(filter)
	if err != nil {
		return fmt.Errorf("get cost report: %w", err)
	}

	filter.GroupBy = "project"
	byProject, err := gdb.GetCostReport(filter)
	if err != nil {
		return fmt.Errorf("get cost report by project: %w", err)
	}

	filter.GroupBy = "model"
	byModel, err := gdb.GetCostReport(filter)
	if err != nil {
		return fmt.Errorf("get cost report by model: %w", err)
	}

	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Cost Summary\n")
	_, _ = fmt.Fprintf(out, "Total: %s\n\n", formatCost(total.TotalCostUSD))

	if len(byProject.Breakdowns) > 0 {
		_, _ = fmt.Fprintf(out, "By Project:\n")
		for _, b := range byProject.Breakdowns {
			_, _ = fmt.Fprintf(out, "  %-30s %s\n", b.Key, formatCost(b.CostUSD))
		}
		_, _ = fmt.Fprintln(out)
	}

	if len(byModel.Breakdowns) > 0 {
		_, _ = fmt.Fprintf(out, "By Model:\n")
		for _, b := range byModel.Breakdowns {
			_, _ = fmt.Fprintf(out, "  %-30s %s\n", b.Key, formatCost(b.CostUSD))
		}
		_, _ = fmt.Fprintln(out)
	}

	if filter.ProjectID != "" {
		displayBudgetStatus(out, gdb, filter.ProjectID)
	}

	return nil
}

func displayCostReport(cmd *cobra.Command, result db.CostReportResult, groupBy, projectFilter string, gdb *db.GlobalDB) {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "Cost Summary\n")
	_, _ = fmt.Fprintf(out, "Total: %s\n\n", formatCost(result.TotalCostUSD))

	if len(result.Breakdowns) > 0 {
		// Capitalize first letter of groupBy for display label
		label := strings.ToUpper(groupBy[:1]) + groupBy[1:]
		_, _ = fmt.Fprintf(out, "By %s:\n", label)

		// Resolve user IDs to names for user grouping
		var userNames map[string]string
		if groupBy == "user" {
			userNames = resolveUserNames(gdb, result.Breakdowns)
		}

		for _, b := range result.Breakdowns {
			displayKey := b.Key
			if groupBy == "user" && userNames != nil {
				if name, ok := userNames[b.Key]; ok {
					displayKey = name
				}
			}
			_, _ = fmt.Fprintf(out, "  %-30s %s\n", displayKey, formatCost(b.CostUSD))
		}
		_, _ = fmt.Fprintln(out)
	}

	if projectFilter != "" {
		displayBudgetStatus(out, gdb, projectFilter)
	}
}

// resolveUserNames maps user IDs to display names.
func resolveUserNames(gdb *db.GlobalDB, breakdowns []db.CostBreakdownEntry) map[string]string {
	names := make(map[string]string)
	for _, b := range breakdowns {
		if b.Key == "unattributed" {
			names[b.Key] = "unattributed"
			continue
		}
		user, err := gdb.GetUser(b.Key)
		if err == nil && user != nil {
			names[b.Key] = user.Name
		}
	}
	return names
}

func displayBudgetStatus(out io.Writer, gdb *db.GlobalDB, projectID string) {
	status, err := gdb.GetBudgetStatus(projectID)
	if err != nil || status == nil {
		return
	}
	_, _ = fmt.Fprintf(out, "Budget: %s / %s (%.0f%%)\n",
		formatCost(status.CurrentMonthSpent),
		formatCost(status.MonthlyLimitUSD),
		status.PercentUsed)
}

// formatCost returns a dollar-formatted string with commas (e.g. "$12,345.67").
func formatCost(amount float64) string {
	if amount == 0 {
		return "$0.00"
	}

	s := fmt.Sprintf("%.2f", amount)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	decPart := parts[1]

	negative := false
	if strings.HasPrefix(intPart, "-") {
		negative = true
		intPart = intPart[1:]
	}

	var result []byte
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}

	formatted := "$" + string(result) + "." + decPart
	if negative {
		formatted = "-" + formatted
	}
	return formatted
}

// openGlobalDBForCosts checks cwd/.orc/orc.db first, then falls back to ~/.orc/orc.db.
func openGlobalDBForCosts() (*db.GlobalDB, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	localDB := filepath.Join(cwd, ".orc", "orc.db")
	if _, err := os.Stat(localDB); err == nil {
		return db.OpenGlobalAt(localDB)
	}

	return db.OpenGlobal()
}
