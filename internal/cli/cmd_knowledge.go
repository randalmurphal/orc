// Package cli implements the orc command-line interface.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/randalmurphal/orc/internal/bootstrap"
	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/db"
)

func newKnowledgeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Manage project knowledge in CLAUDE.md",
		Long: `Manage project knowledge captured in CLAUDE.md.

Knowledge includes patterns, gotchas, and decisions learned during development.
This knowledge is captured during the docs phase and stored in the CLAUDE.md
knowledge section.

Commands:
  status   Show knowledge statistics and split recommendation
  split    Move knowledge section to agent_docs/ directory
  search   Search project knowledge for keywords
  queue    List pending knowledge entries (when approval mode=queue)
  approve  Approve a pending knowledge entry
  reject   Reject a pending knowledge entry
  validate Mark an approved entry as still relevant (resets staleness)`,
	}

	cmd.AddCommand(newKnowledgeStatusCmd())
	cmd.AddCommand(newKnowledgeSplitCmd())
	cmd.AddCommand(newKnowledgeSearchCmd())
	cmd.AddCommand(newKnowledgeQueueCmd())
	cmd.AddCommand(newKnowledgeApproveCmd())
	cmd.AddCommand(newKnowledgeRejectCmd())
	cmd.AddCommand(newKnowledgeValidateCmd())

	return cmd
}

func newKnowledgeStatusCmd() *cobra.Command {
	var stalenessDays int

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show knowledge statistics and split recommendation",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			wd, err := config.FindProjectRoot()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			// Check if knowledge section exists
			if !bootstrap.HasKnowledgeSection(wd) {
				fmt.Println("No knowledge section found in CLAUDE.md")
				fmt.Println("\nRun 'orc init --force' to add the knowledge section.")
				return nil
			}

			// Get line counts
			totalLines, err := bootstrap.ClaudeMDLineCount(wd)
			if err != nil {
				return fmt.Errorf("count CLAUDE.md lines: %w", err)
			}

			knowledgeLines, err := bootstrap.KnowledgeSectionLineCount(wd)
			if err != nil {
				return fmt.Errorf("count knowledge lines: %w", err)
			}

			// Count entries
			patterns, gotchas, decisions := countKnowledgeEntries(wd)

			// Display stats
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "Knowledge Statistics")
			_, _ = fmt.Fprintln(w, "--------------------")
			_, _ = fmt.Fprintf(w, "CLAUDE.md total lines:\t%d\n", totalLines)
			_, _ = fmt.Fprintf(w, "Knowledge section lines:\t%d\n", knowledgeLines)
			_, _ = fmt.Fprintf(w, "Patterns learned:\t%d\n", patterns)
			_, _ = fmt.Fprintf(w, "Known gotchas:\t%d\n", gotchas)
			_, _ = fmt.Fprintf(w, "Decisions:\t%d\n", decisions)
			_ = w.Flush()

			// Check stale entries in queue
			pdb, err := db.OpenProject(wd)
			if err == nil {
				defer func() { _ = pdb.Close() }()
				staleCount, _ := pdb.CountStaleKnowledge(stalenessDays)
				pendingCount, _ := pdb.CountPendingKnowledge()

				if pendingCount > 0 || staleCount > 0 {
					fmt.Println()
					fmt.Println("Queue Status")
					fmt.Println("------------")
					if pendingCount > 0 {
						fmt.Printf("⏳ Pending approval: %d\n", pendingCount)
					}
					if staleCount > 0 {
						fmt.Printf("⚠️  Stale entries (>%d days): %d\n", stalenessDays, staleCount)
						fmt.Println("   Run 'orc knowledge validate <id>' to refresh validation")
					}
				}
			}

			// Check if split is recommended
			shouldSplit, _, _ := bootstrap.ShouldSuggestSplit(wd)
			fmt.Println()
			if shouldSplit {
				fmt.Println("⚠️  CLAUDE.md is getting long (>200 lines).")
				fmt.Println("   Consider running 'orc knowledge split' to move knowledge to agent_docs/")
			} else {
				fmt.Println("✓  CLAUDE.md is within recommended size")
			}

			// Check for agent_docs
			if _, err := os.Stat(filepath.Join(wd, "agent_docs")); err == nil {
				fmt.Println("\n📁 agent_docs/ directory exists")
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&stalenessDays, "staleness", 90, "Days before entry is considered stale")

	return cmd
}

func newKnowledgeSplitCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "split",
		Short: "Move knowledge section to agent_docs/ directory",
		Long: `Move the knowledge section from CLAUDE.md to separate files in agent_docs/.

This creates:
  agent_docs/
  ├── decisions.md    # Architectural decisions
  ├── patterns.md     # Code patterns
  └── gotchas.md      # Known issues and gotchas

The CLAUDE.md knowledge section is replaced with a pointer to agent_docs/.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			wd, err := config.FindProjectRoot()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			// Check if knowledge section exists
			if !bootstrap.HasKnowledgeSection(wd) {
				return fmt.Errorf("no knowledge section found in CLAUDE.md")
			}

			// Check if agent_docs already exists
			agentDocsDir := filepath.Join(wd, "agent_docs")
			if _, err := os.Stat(agentDocsDir); err == nil {
				if !force {
					return fmt.Errorf("agent_docs/ already exists (use --force to overwrite)")
				}
			}

			// Parse knowledge from CLAUDE.md
			patterns, gotchas, decisions, err := parseKnowledgeSection(wd)
			if err != nil {
				return fmt.Errorf("parse knowledge section: %w", err)
			}

			// Create agent_docs directory
			if err := os.MkdirAll(agentDocsDir, 0755); err != nil {
				return fmt.Errorf("create agent_docs: %w", err)
			}

			// Write patterns.md
			patternsPath := filepath.Join(agentDocsDir, "patterns.md")
			if err := os.WriteFile(patternsPath, []byte(formatPatternsFile(patterns)), 0644); err != nil {
				return fmt.Errorf("write patterns.md: %w", err)
			}
			fmt.Printf("Created: %s (%d entries)\n", patternsPath, len(patterns))

			// Write gotchas.md
			gotchasPath := filepath.Join(agentDocsDir, "gotchas.md")
			if err := os.WriteFile(gotchasPath, []byte(formatGotchasFile(gotchas)), 0644); err != nil {
				return fmt.Errorf("write gotchas.md: %w", err)
			}
			fmt.Printf("Created: %s (%d entries)\n", gotchasPath, len(gotchas))

			// Write decisions.md
			decisionsPath := filepath.Join(agentDocsDir, "decisions.md")
			if err := os.WriteFile(decisionsPath, []byte(formatDecisionsFile(decisions)), 0644); err != nil {
				return fmt.Errorf("write decisions.md: %w", err)
			}
			fmt.Printf("Created: %s (%d entries)\n", decisionsPath, len(decisions))

			// Update CLAUDE.md with pointer
			if err := replaceKnowledgeSectionWithPointer(wd, len(patterns), len(gotchas), len(decisions)); err != nil {
				return fmt.Errorf("update CLAUDE.md: %w", err)
			}
			fmt.Println("Updated: CLAUDE.md (replaced knowledge section with pointer)")

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing agent_docs/")

	return cmd
}

func newKnowledgeSearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <keyword>",
		Short: "Search project knowledge for keywords",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			keyword := strings.ToLower(args[0])
			wd, err := config.FindProjectRoot()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			// Search CLAUDE.md knowledge section
			patterns, gotchas, decisions, _ := parseKnowledgeSection(wd)

			// Also search agent_docs if exists
			agentDocsDir := filepath.Join(wd, "agent_docs")
			if _, err := os.Stat(agentDocsDir); err == nil {
				// Load from agent_docs files
				patternsFile := filepath.Join(agentDocsDir, "patterns.md")
				gotchasFile := filepath.Join(agentDocsDir, "gotchas.md")
				decisionsFile := filepath.Join(agentDocsDir, "decisions.md")

				patterns = append(patterns, parseTableFromFile(patternsFile)...)
				gotchas = append(gotchas, parseTableFromFile(gotchasFile)...)
				decisions = append(decisions, parseTableFromFile(decisionsFile)...)
			}

			found := false
			printedPatterns := false
			printedGotchas := false
			printedDecisions := false

			// Search patterns
			for _, p := range patterns {
				if strings.Contains(strings.ToLower(p[0]), keyword) ||
					strings.Contains(strings.ToLower(p[1]), keyword) {
					if !printedPatterns {
						fmt.Println("Matching Patterns:")
						printedPatterns = true
					}
					found = true
					fmt.Printf("  • %s: %s (from %s)\n", p[0], p[1], p[2])
				}
			}

			// Search gotchas
			for _, g := range gotchas {
				if strings.Contains(strings.ToLower(g[0]), keyword) ||
					strings.Contains(strings.ToLower(g[1]), keyword) {
					if !printedGotchas {
						if found {
							fmt.Println()
						}
						fmt.Println("Matching Gotchas:")
						printedGotchas = true
					}
					found = true
					fmt.Printf("  • %s: %s (from %s)\n", g[0], g[1], g[2])
				}
			}

			// Search decisions
			for _, d := range decisions {
				if strings.Contains(strings.ToLower(d[0]), keyword) ||
					strings.Contains(strings.ToLower(d[1]), keyword) {
					if !printedDecisions {
						if found {
							fmt.Println()
						}
						fmt.Println("Matching Decisions:")
						printedDecisions = true
					}
					found = true
					fmt.Printf("  • %s: %s (from %s)\n", d[0], d[1], d[2])
				}
			}

			if !found {
				fmt.Printf("No knowledge found matching '%s'\n", keyword)
			}

			return nil
		},
	}
}

// countKnowledgeEntries counts entries in each knowledge table.
func countKnowledgeEntries(projectDir string) (patterns, gotchas, decisions int) {
	p, g, d, _ := parseKnowledgeSection(projectDir)
	return len(p), len(g), len(d)
}

// parseKnowledgeSection extracts entries from the knowledge tables in CLAUDE.md.
// Returns patterns, gotchas, decisions as slices of [name, description, source].
func parseKnowledgeSection(projectDir string) ([][]string, [][]string, [][]string, error) {
	claudeMDPath := filepath.Join(projectDir, "CLAUDE.md")

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return nil, nil, nil, err
	}

	content := string(data)

	// Extract knowledge section
	sectionStart := "<!-- orc:knowledge:begin -->"
	sectionEnd := "<!-- orc:knowledge:end -->"

	startIdx := strings.Index(content, sectionStart)
	endIdx := strings.Index(content, sectionEnd)

	if startIdx == -1 || endIdx == -1 {
		return nil, nil, nil, nil
	}

	section := content[startIdx:endIdx]

	// Parse tables
	patterns := parseTable(section, "### Patterns Learned")
	gotchas := parseTable(section, "### Known Gotchas")
	decisions := parseTable(section, "### Decisions")

	return patterns, gotchas, decisions, nil
}

// parseTable extracts rows from a markdown table after the given header.
func parseTable(content, header string) [][]string {
	var startIdx int
	if header != "" {
		headerIdx := strings.Index(content, header)
		if headerIdx == -1 {
			return nil
		}
		startIdx = headerIdx
	}

	// Find table rows after header
	afterHeader := content[startIdx:]
	lines := strings.Split(afterHeader, "\n")

	var rows [][]string
	foundSeparator := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check if this is a table row
		if strings.HasPrefix(line, "|") && strings.HasSuffix(line, "|") {
			// Check for separator row (|---|---|---|)
			if strings.Contains(line, "---") {
				foundSeparator = true
				continue
			}

			// Skip header row (before separator)
			if !foundSeparator {
				continue
			}

			// Parse data row
			cells := strings.Split(line, "|")
			if len(cells) >= 4 { // Including empty first and last from split
				name := strings.TrimSpace(cells[1])
				desc := strings.TrimSpace(cells[2])
				source := strings.TrimSpace(cells[3])

				// Skip if looks like a header row that slipped through
				if name != "" && !isTableHeader(name) {
					rows = append(rows, []string{name, desc, source})
				}
			}
		} else if foundSeparator && (strings.HasPrefix(line, "### ") || strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "<!-- ")) {
			// Hit next section - stop parsing this table
			break
		}
	}

	return rows
}

// isTableHeader returns true if the value looks like a table header.
func isTableHeader(s string) bool {
	headers := []string{"Pattern", "Issue", "Decision", "Name", "Description", "Source", "Rationale", "Resolution"}
	for _, h := range headers {
		if s == h {
			return true
		}
	}
	return false
}

// parseTableFromFile parses knowledge entries from an agent_docs file.
func parseTableFromFile(path string) [][]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return parseTable(string(data), "")
}

// formatPatternsFile creates the patterns.md content.
func formatPatternsFile(patterns [][]string) string {
	var sb strings.Builder
	sb.WriteString("# Code Patterns\n\n")
	sb.WriteString("Reusable patterns learned during development.\n\n")
	sb.WriteString("| Pattern | Description | Source |\n")
	sb.WriteString("|---------|-------------|--------|\n")

	for _, p := range patterns {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", p[0], p[1], p[2]))
	}

	return sb.String()
}

// formatGotchasFile creates the gotchas.md content.
func formatGotchasFile(gotchas [][]string) string {
	var sb strings.Builder
	sb.WriteString("# Known Gotchas\n\n")
	sb.WriteString("Issues encountered and their resolutions.\n\n")
	sb.WriteString("| Issue | Resolution | Source |\n")
	sb.WriteString("|-------|------------|--------|\n")

	for _, g := range gotchas {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", g[0], g[1], g[2]))
	}

	return sb.String()
}

// formatDecisionsFile creates the decisions.md content.
func formatDecisionsFile(decisions [][]string) string {
	var sb strings.Builder
	sb.WriteString("# Architectural Decisions\n\n")
	sb.WriteString("Key decisions made during development.\n\n")
	sb.WriteString("| Decision | Rationale | Source |\n")
	sb.WriteString("|----------|-----------|--------|\n")

	for _, d := range decisions {
		sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", d[0], d[1], d[2]))
	}

	return sb.String()
}

// replaceKnowledgeSectionWithPointer updates CLAUDE.md to point to agent_docs/.
func replaceKnowledgeSectionWithPointer(projectDir string, patternsCount, gotchasCount, decisionsCount int) error {
	claudeMDPath := filepath.Join(projectDir, "CLAUDE.md")

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return err
	}

	content := string(data)

	// Build pointer section
	pointer := fmt.Sprintf(`## Project Knowledge

See [agent_docs/](agent_docs/) for patterns, gotchas, and decisions:
- [Patterns](agent_docs/patterns.md) (%d items)
- [Gotchas](agent_docs/gotchas.md) (%d items)
- [Decisions](agent_docs/decisions.md) (%d items)

Check decisions before making architectural choices.
`, patternsCount, gotchasCount, decisionsCount)

	// Replace knowledge section
	sectionStart := "<!-- orc:knowledge:begin -->"
	sectionEnd := "<!-- orc:knowledge:end -->"

	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(sectionStart) + `.*?` + regexp.QuoteMeta(sectionEnd))
	newContent := re.ReplaceAllString(content, sectionStart+"\n"+pointer+sectionEnd)

	return os.WriteFile(claudeMDPath, []byte(newContent), 0644)
}

func newKnowledgeQueueCmd() *cobra.Command {
	var showAll bool

	cmd := &cobra.Command{
		Use:   "queue",
		Short: "List pending knowledge entries",
		Long: `List knowledge entries in the approval queue.

By default, shows only pending entries. Use --all to see all entries.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			wd, err := config.FindProjectRoot()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			pdb, err := db.OpenProject(wd)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = pdb.Close() }()

			var entries []*db.KnowledgeEntry
			if showAll {
				// Get all entries by querying each type and status
				pending, _ := pdb.ListPendingKnowledge()
				entries = append(entries, pending...)

				for _, ktype := range []db.KnowledgeType{db.KnowledgePattern, db.KnowledgeGotcha, db.KnowledgeDecision} {
					approved, _ := pdb.ListKnowledgeByType(ktype, db.KnowledgeApproved)
					rejected, _ := pdb.ListKnowledgeByType(ktype, db.KnowledgeRejected)
					entries = append(entries, approved...)
					entries = append(entries, rejected...)
				}
			} else {
				entries, err = pdb.ListPendingKnowledge()
				if err != nil {
					return fmt.Errorf("list pending: %w", err)
				}
			}

			if len(entries) == 0 {
				if showAll {
					fmt.Println("No knowledge entries found.")
				} else {
					fmt.Println("No pending knowledge entries in queue.")
				}
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tTYPE\tNAME\tSOURCE\tSTATUS")
			_, _ = fmt.Fprintln(w, "--\t----\t----\t------\t------")

			for _, e := range entries {
				name := e.Name
				if len(name) > 30 {
					name = name[:27] + "..."
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					e.ID, e.Type, name, e.SourceTask, e.Status)
			}
			_ = w.Flush()

			return nil
		},
	}

	cmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all entries, not just pending")

	return cmd
}

func newKnowledgeApproveCmd() *cobra.Command {
	var approveAll bool

	cmd := &cobra.Command{
		Use:   "approve [id]",
		Short: "Approve a pending knowledge entry",
		Long: `Approve a knowledge entry, moving it from the queue to CLAUDE.md.

Use --all to approve all pending entries at once.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			if !approveAll && len(args) == 0 {
				return fmt.Errorf("specify an entry ID or use --all")
			}

			wd, err := config.FindProjectRoot()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			pdb, err := db.OpenProject(wd)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = pdb.Close() }()

			if approveAll {
				count, err := pdb.ApproveAllPending("cli")
				if err != nil {
					return fmt.Errorf("approve all: %w", err)
				}
				if count == 0 {
					fmt.Println("No pending entries to approve.")
				} else {
					fmt.Printf("Approved %d entries.\n", count)
					// TODO: Write approved entries to CLAUDE.md
				}
				return nil
			}

			id := args[0]
			entry, err := pdb.ApproveKnowledge(id, "cli")
			if err != nil {
				return fmt.Errorf("approve %s: %w", id, err)
			}

			fmt.Printf("Approved: %s (%s: %s)\n", entry.ID, entry.Type, entry.Name)

			// TODO: Write approved entry to CLAUDE.md

			return nil
		},
	}

	cmd.Flags().BoolVar(&approveAll, "all", false, "Approve all pending entries")

	return cmd
}

func newKnowledgeRejectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reject <id> [reason]",
		Short: "Reject a pending knowledge entry",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			id := args[0]
			reason := "rejected via CLI"
			if len(args) > 1 {
				reason = strings.Join(args[1:], " ")
			}

			wd, err := config.FindProjectRoot()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			pdb, err := db.OpenProject(wd)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = pdb.Close() }()

			if err := pdb.RejectKnowledge(id, reason); err != nil {
				return fmt.Errorf("reject %s: %w", id, err)
			}

			fmt.Printf("Rejected: %s (reason: %s)\n", id, reason)

			return nil
		},
	}
}

func newKnowledgeValidateCmd() *cobra.Command {
	var listStale bool
	var stalenessDays int

	cmd := &cobra.Command{
		Use:   "validate [id]",
		Short: "Mark an approved entry as still relevant (resets staleness)",
		Long: `Validate that a knowledge entry is still relevant.

This updates the validated_at timestamp, resetting the staleness timer.
Use this to confirm that older decisions/patterns are still valid.

Use --list to see all stale entries that need validation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := config.RequireInit(); err != nil {
				return err
			}

			wd, err := config.FindProjectRoot()
			if err != nil {
				return fmt.Errorf("find project root: %w", err)
			}

			pdb, err := db.OpenProject(wd)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = pdb.Close() }()

			// List stale entries mode
			if listStale {
				entries, err := pdb.ListStaleKnowledge(stalenessDays)
				if err != nil {
					return fmt.Errorf("list stale entries: %w", err)
				}

				if len(entries) == 0 {
					fmt.Println("No stale entries. All knowledge is up to date!")
					return nil
				}

				fmt.Printf("Stale entries (>%d days since validation):\n\n", stalenessDays)
				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintln(w, "ID\tTYPE\tNAME\tLAST VALIDATED")
				_, _ = fmt.Fprintln(w, "--\t----\t----\t--------------")

				for _, e := range entries {
					lastValidated := "never"
					if e.ValidatedAt != nil {
						lastValidated = e.ValidatedAt.Format("2006-01-02")
					} else if e.ApprovedAt != nil {
						lastValidated = e.ApprovedAt.Format("2006-01-02") + " (approved)"
					}

					name := e.Name
					if len(name) > 30 {
						name = name[:27] + "..."
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.ID, e.Type, name, lastValidated)
				}
				_ = w.Flush()

				fmt.Printf("\nRun 'orc knowledge validate <id>' to mark as still relevant.\n")
				return nil
			}

			// Validate specific entry
			if len(args) == 0 {
				return fmt.Errorf("specify an entry ID or use --list to see stale entries")
			}

			id := args[0]
			entry, err := pdb.ValidateKnowledge(id, "cli")
			if err != nil {
				return fmt.Errorf("validate %s: %w", id, err)
			}

			fmt.Printf("Validated: %s (%s: %s)\n", entry.ID, entry.Type, entry.Name)
			fmt.Printf("Entry confirmed as still relevant at %s\n", entry.ValidatedAt.Format("2006-01-02 15:04:05"))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&listStale, "list", "l", false, "List all stale entries")
	cmd.Flags().IntVar(&stalenessDays, "staleness", 90, "Days before entry is considered stale")

	return cmd
}
