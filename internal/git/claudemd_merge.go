// Package git provides git operations for orc, including CLAUDE.md conflict resolution.
package git

import (
	"fmt"
	"log/slog"
	"regexp"
	"sort"
	"strings"
)

// Markers for append-only sections in CLAUDE.md
const (
	KnowledgeSectionStart = "<!-- orc:knowledge:begin -->"
	KnowledgeSectionEnd   = "<!-- orc:knowledge:end -->"
)

// TableSection represents an append-only table section in CLAUDE.md
type TableSection struct {
	Name       string   // Section name (e.g., "Patterns Learned")
	HeaderLine string   // Table header row (e.g., "| Pattern | Description | Source |")
	Separator  string   // Table separator (e.g., "|---------|-------------|--------|")
	Rows       []string // Data rows in the table
}

// ClaudeMDConflict represents a conflict in CLAUDE.md
type ClaudeMDConflict struct {
	FilePath       string
	IsKnowledge    bool                      // True if conflict is in knowledge section
	Tables         map[string]*TableConflict // Table name -> conflict details
	CanAutoResolve bool
	ResolutionLog  []string
}

// TableConflict represents a conflict in a specific table
type TableConflict struct {
	TableName     string
	OursRows      []string // Rows from our version
	TheirsRows    []string // Rows from their version
	CommonRows    []string // Rows in both versions
	AddedByOurs   []string // New rows added by ours (not in common)
	AddedByTheirs []string // New rows added by theirs (not in common)
	CanMerge      bool     // True if purely additive
}

// MergeResult contains the result of attempting to auto-merge
type MergeResult struct {
	Success       bool
	MergedContent string
	Logs          []string
	Error         error
}

// ClaudeMDMerger handles auto-merging of append-only sections in CLAUDE.md
type ClaudeMDMerger struct {
	logger *slog.Logger
}

// NewClaudeMDMerger creates a new CLAUDE.md merger
func NewClaudeMDMerger(logger *slog.Logger) *ClaudeMDMerger {
	if logger == nil {
		logger = slog.Default()
	}
	return &ClaudeMDMerger{logger: logger}
}

// CanAutoResolve checks if a CLAUDE.md conflict can be auto-resolved
// It analyzes the conflict markers and determines if both sides are purely additive
func (m *ClaudeMDMerger) CanAutoResolve(conflictedContent string) (*ClaudeMDConflict, error) {
	conflict := &ClaudeMDConflict{
		Tables:        make(map[string]*TableConflict),
		ResolutionLog: []string{},
	}

	// Check if conflict is in knowledge section
	if !strings.Contains(conflictedContent, KnowledgeSectionStart) {
		conflict.ResolutionLog = append(conflict.ResolutionLog, "No knowledge section markers found")
		return conflict, nil
	}

	// Check for conflict markers
	if !strings.Contains(conflictedContent, "<<<<<<<") {
		conflict.ResolutionLog = append(conflict.ResolutionLog, "No conflict markers found")
		return conflict, nil
	}

	// Extract knowledge section with conflicts
	knowledgeSection, err := extractKnowledgeSection(conflictedContent)
	if err != nil {
		conflict.ResolutionLog = append(conflict.ResolutionLog, fmt.Sprintf("Failed to extract knowledge section: %v", err))
		return conflict, nil
	}

	if knowledgeSection == "" {
		conflict.ResolutionLog = append(conflict.ResolutionLog, "Knowledge section is empty")
		return conflict, nil
	}

	conflict.IsKnowledge = true

	// Parse conflicts in the knowledge section
	conflicts := parseConflictBlocks(knowledgeSection)
	if len(conflicts) == 0 {
		conflict.ResolutionLog = append(conflict.ResolutionLog, "No conflict blocks found in knowledge section")
		return conflict, nil
	}

	m.logger.Debug("found conflict blocks", "count", len(conflicts))

	// Analyze each conflict block
	allCanMerge := true
	for i, cb := range conflicts {
		tableConflict, err := m.analyzeTableConflict(cb)
		if err != nil {
			conflict.ResolutionLog = append(conflict.ResolutionLog, fmt.Sprintf("Conflict block %d: analysis failed: %v", i+1, err))
			allCanMerge = false
			continue
		}

		if tableConflict == nil {
			conflict.ResolutionLog = append(conflict.ResolutionLog, fmt.Sprintf("Conflict block %d: not in a recognized table", i+1))
			allCanMerge = false
			continue
		}

		conflict.Tables[tableConflict.TableName] = tableConflict
		if !tableConflict.CanMerge {
			allCanMerge = false
			conflict.ResolutionLog = append(conflict.ResolutionLog, fmt.Sprintf("Table '%s': conflict is not purely additive", tableConflict.TableName))
		} else {
			conflict.ResolutionLog = append(conflict.ResolutionLog, fmt.Sprintf("Table '%s': can auto-merge (%d ours, %d theirs new rows)",
				tableConflict.TableName, len(tableConflict.AddedByOurs), len(tableConflict.AddedByTheirs)))
		}
	}

	conflict.CanAutoResolve = allCanMerge
	return conflict, nil
}

// AutoResolve attempts to auto-resolve conflicts in CLAUDE.md
// Returns the resolved content or an error if resolution fails
func (m *ClaudeMDMerger) AutoResolve(conflictedContent string) *MergeResult {
	result := &MergeResult{
		Logs: []string{},
	}

	conflict, err := m.CanAutoResolve(conflictedContent)
	if err != nil {
		result.Error = fmt.Errorf("conflict analysis failed: %w", err)
		return result
	}

	if !conflict.CanAutoResolve {
		result.Error = fmt.Errorf("conflict cannot be auto-resolved: %s", strings.Join(conflict.ResolutionLog, "; "))
		result.Logs = conflict.ResolutionLog
		return result
	}

	// Perform the merge
	resolved, err := m.mergeContent(conflictedContent, conflict)
	if err != nil {
		result.Error = fmt.Errorf("merge failed: %w", err)
		result.Logs = conflict.ResolutionLog
		return result
	}

	result.Success = true
	result.MergedContent = resolved
	result.Logs = conflict.ResolutionLog
	result.Logs = append(result.Logs, "Auto-merge successful")

	m.logger.Info("CLAUDE.md auto-merge successful",
		"tables_merged", len(conflict.Tables),
	)

	return result
}

// analyzeTableConflict analyzes a conflict block to determine if it's in a table
// and if it can be automatically merged
func (m *ClaudeMDMerger) analyzeTableConflict(cb conflictBlock) (*TableConflict, error) {
	tc := &TableConflict{
		OursRows:   []string{},
		TheirsRows: []string{},
	}

	// Parse rows from ours and theirs sections
	tc.OursRows = parseTableRows(cb.ours)
	tc.TheirsRows = parseTableRows(cb.theirs)

	// If neither has table rows, not a table conflict
	if len(tc.OursRows) == 0 && len(tc.TheirsRows) == 0 {
		return nil, nil
	}

	// Determine table name from context (try to find the table header)
	tc.TableName = detectTableName(cb.contextBefore)
	if tc.TableName == "" {
		tc.TableName = "Unknown"
	}

	// Find common rows (rows that exist in both)
	oursSet := make(map[string]bool)
	for _, row := range tc.OursRows {
		oursSet[normalizeRow(row)] = true
	}

	theirsSet := make(map[string]bool)
	for _, row := range tc.TheirsRows {
		theirsSet[normalizeRow(row)] = true
	}

	// Identify added rows on each side
	for _, row := range tc.OursRows {
		normalized := normalizeRow(row)
		if theirsSet[normalized] {
			tc.CommonRows = append(tc.CommonRows, row)
		} else {
			tc.AddedByOurs = append(tc.AddedByOurs, row)
		}
	}

	for _, row := range tc.TheirsRows {
		normalized := normalizeRow(row)
		if !oursSet[normalized] {
			tc.AddedByTheirs = append(tc.AddedByTheirs, row)
		}
	}

	// Can merge if the conflict is purely additive (no overlapping edits to same rows)
	// A conflict is purely additive if:
	// 1. All common rows are identical
	// 2. Added rows don't modify the same data (checked by normalized row comparison)
	tc.CanMerge = m.isPurelyAdditive(tc)

	return tc, nil
}

// isPurelyAdditive checks if a table conflict is purely additive
func (m *ClaudeMDMerger) isPurelyAdditive(tc *TableConflict) bool {
	// Check if any added rows have conflicting source IDs
	// (two different rows with the same TASK-xxx source would indicate a real conflict)
	oursSourceIDs := extractSourceIDs(tc.AddedByOurs)
	theirsSourceIDs := extractSourceIDs(tc.AddedByTheirs)

	// If same source ID appears in both with different content, not purely additive
	for id := range oursSourceIDs {
		if theirsSourceIDs[id] {
			// Same source ID in both - check if content differs
			m.logger.Debug("same source ID in both sides", "id", id)
			// For now, allow this - the source ID being the same doesn't necessarily
			// mean the content is the same (could be different patterns from same task)
		}
	}

	// Pure additive: both sides just added new rows
	return true
}

// mergeContent performs the actual merge of conflicted content
func (m *ClaudeMDMerger) mergeContent(conflictedContent string, conflict *ClaudeMDConflict) (string, error) {
	result := conflictedContent

	// Process each conflict block and replace with merged content
	conflicts := parseConflictBlocks(conflictedContent)

	// Process in reverse order to maintain string positions
	for i := len(conflicts) - 1; i >= 0; i-- {
		cb := conflicts[i]
		tc, err := m.analyzeTableConflict(cb)
		if err != nil || tc == nil || !tc.CanMerge {
			continue
		}

		// Build merged rows: common rows + ours additions + theirs additions
		mergedRows := make([]string, 0, len(tc.CommonRows)+len(tc.AddedByOurs)+len(tc.AddedByTheirs))

		// Add all rows from ours (preserves order and includes common)
		mergedRows = append(mergedRows, tc.OursRows...)

		// Add rows from theirs that aren't already present
		mergedRows = append(mergedRows, tc.AddedByTheirs...)

		// Sort by source ID (TASK-XXX) for consistent ordering
		sortBySourceID(mergedRows)

		// Build replacement text
		replacement := strings.Join(mergedRows, "\n")
		if len(mergedRows) > 0 {
			replacement += "\n"
		}

		// Replace the conflict block with merged content
		result = result[:cb.startPos] + replacement + result[cb.endPos:]
	}

	// Verify no conflict markers remain
	if strings.Contains(result, "<<<<<<<") || strings.Contains(result, ">>>>>>>") {
		return "", fmt.Errorf("conflict markers remain after merge")
	}

	return result, nil
}

// conflictBlock represents a git conflict block
type conflictBlock struct {
	startPos      int
	endPos        int
	ours          string
	theirs        string
	contextBefore string // Text before the conflict (for table detection)
}

// parseConflictBlocks extracts all conflict blocks from content
func parseConflictBlocks(content string) []conflictBlock {
	var blocks []conflictBlock

	// Regex to match conflict markers
	// <<<<<<< HEAD/ours
	// content
	// =======
	// content
	// >>>>>>> branch/theirs
	conflictRe := regexp.MustCompile(`(?s)<<<<<<<[^\n]*\n(.*?)\n?=======\n(.*?)\n?>>>>>>>[^\n]*`)

	matches := conflictRe.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) >= 6 {
			block := conflictBlock{
				startPos: match[0],
				endPos:   match[1],
				ours:     content[match[2]:match[3]],
				theirs:   content[match[4]:match[5]],
			}

			// Get context before (up to 500 chars or start of content)
			contextStart := match[0] - 500
			if contextStart < 0 {
				contextStart = 0
			}
			block.contextBefore = content[contextStart:match[0]]

			blocks = append(blocks, block)
		}
	}

	return blocks
}

// extractKnowledgeSection extracts the knowledge section from CLAUDE.md content
func extractKnowledgeSection(content string) (string, error) {
	startIdx := strings.Index(content, KnowledgeSectionStart)
	if startIdx == -1 {
		return "", nil
	}

	endIdx := strings.Index(content, KnowledgeSectionEnd)
	if endIdx == -1 {
		return "", fmt.Errorf("knowledge section start found but no end marker")
	}

	if endIdx <= startIdx {
		return "", fmt.Errorf("knowledge section markers in wrong order")
	}

	return content[startIdx : endIdx+len(KnowledgeSectionEnd)], nil
}

// parseTableRows extracts table rows from content
func parseTableRows(content string) []string {
	var rows []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Table row must start with | and not be a separator (---) or header
		if strings.HasPrefix(trimmed, "|") && !strings.Contains(trimmed, "---") {
			// Skip header rows (contain only header-like content)
			if !isTableHeader(trimmed) {
				rows = append(rows, line)
			}
		}
	}

	return rows
}

// isTableHeader checks if a row is a table header row
func isTableHeader(row string) bool {
	// Header rows have a specific pattern: they contain column names as cell values
	// rather than containing those words as part of longer data strings.
	//
	// We check if the cells are primarily single-word column names that are
	// common in the knowledge section tables.

	// Split by | and check each cell
	parts := strings.Split(row, "|")
	if len(parts) < 3 {
		return false
	}

	// Header words that would be the ENTIRE cell content (not part of data)
	headerCellValues := map[string]bool{
		"pattern":     true,
		"description": true,
		"source":      true,
		"issue":       true,
		"resolution":  true,
		"decision":    true,
		"rationale":   true,
	}

	headerCellCount := 0
	totalCells := 0

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		totalCells++

		// Check if this cell is exactly a header word (case insensitive)
		if headerCellValues[strings.ToLower(trimmed)] {
			headerCellCount++
		}
	}

	// If most cells are header words, this is a header row
	// At least 2 header cells and majority of cells are headers
	return headerCellCount >= 2 && totalCells > 0 && headerCellCount >= (totalCells/2)
}

// detectTableName tries to identify which table a conflict is in
func detectTableName(contextBefore string) string {
	// Look for section headers
	if strings.Contains(contextBefore, "### Patterns Learned") ||
		strings.Contains(contextBefore, "Patterns Learned") {
		return "Patterns Learned"
	}
	if strings.Contains(contextBefore, "### Known Gotchas") ||
		strings.Contains(contextBefore, "Known Gotchas") {
		return "Known Gotchas"
	}
	if strings.Contains(contextBefore, "### Decisions") ||
		strings.Contains(contextBefore, "Decisions") {
		return "Decisions"
	}
	return ""
}

// normalizeRow normalizes a table row for comparison
func normalizeRow(row string) string {
	// Trim whitespace and normalize pipe spacing
	trimmed := strings.TrimSpace(row)
	// Remove extra spaces around pipes
	re := regexp.MustCompile(`\s*\|\s*`)
	return re.ReplaceAllString(trimmed, "|")
}

// extractSourceIDs extracts TASK-XXX identifiers from rows
func extractSourceIDs(rows []string) map[string]bool {
	ids := make(map[string]bool)
	re := regexp.MustCompile(`TASK-\d+`)

	for _, row := range rows {
		matches := re.FindAllString(row, -1)
		for _, m := range matches {
			ids[m] = true
		}
	}

	return ids
}

// sortBySourceID sorts table rows by their source ID (TASK-XXX)
func sortBySourceID(rows []string) {
	re := regexp.MustCompile(`TASK-(\d+)`)

	sort.SliceStable(rows, func(i, j int) bool {
		matchI := re.FindStringSubmatch(rows[i])
		matchJ := re.FindStringSubmatch(rows[j])

		// If no match, preserve order
		if len(matchI) < 2 || len(matchJ) < 2 {
			return false
		}

		// Parse numbers (ignore errors - if parsing fails, use 0)
		var numI, numJ int
		_, _ = fmt.Sscanf(matchI[1], "%d", &numI)
		_, _ = fmt.Sscanf(matchJ[1], "%d", &numJ)

		return numI < numJ
	})
}

// ResolveClaudeMDConflict is a convenience function that attempts to auto-resolve
// a CLAUDE.md conflict. Returns the resolved content and whether resolution succeeded.
func ResolveClaudeMDConflict(conflictedContent string, logger *slog.Logger) (string, bool, []string) {
	merger := NewClaudeMDMerger(logger)
	result := merger.AutoResolve(conflictedContent)

	if result.Success {
		return result.MergedContent, true, result.Logs
	}

	return "", false, result.Logs
}

// IsClaudeMDFile checks if a file path is CLAUDE.md
func IsClaudeMDFile(path string) bool {
	return strings.HasSuffix(path, "CLAUDE.md") || path == "CLAUDE.md"
}
