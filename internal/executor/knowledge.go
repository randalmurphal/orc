// Package executor provides the flowgraph-based execution engine for orc.
// This file contains post-phase knowledge extraction as a fallback mechanism.
package executor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	// Knowledge section markers
	knowledgeSectionStart = "<!-- orc:knowledge:begin -->"
	knowledgeSectionEnd   = "<!-- orc:knowledge:end -->"
	// Target directive for external knowledge file
	knowledgeTargetPrefix = "<!-- orc:knowledge:target:"
	knowledgeTargetSuffix = " -->"
)

// KnowledgeCapture holds extracted knowledge from transcripts.
type KnowledgeCapture struct {
	Patterns  []KnowledgeEntry
	Gotchas   []KnowledgeEntry
	Decisions []KnowledgeEntry
}

// KnowledgeEntry represents a single piece of captured knowledge.
type KnowledgeEntry struct {
	Name        string
	Description string
	Source      string // Task ID
}

// findKnowledgeTarget locates the file containing the knowledge section.
// Checks CLAUDE.md for a target directive (<!-- orc:knowledge:target:path -->).
// Returns the target path if found, otherwise returns CLAUDE.md.
func findKnowledgeTarget(projectDir string) string {
	claudeMDPath := filepath.Join(projectDir, "CLAUDE.md")

	data, err := os.ReadFile(claudeMDPath)
	if err != nil {
		return claudeMDPath
	}

	content := string(data)

	// Look for target directive
	startIdx := strings.Index(content, knowledgeTargetPrefix)
	if startIdx == -1 {
		return claudeMDPath
	}

	// Extract the target path
	afterPrefix := content[startIdx+len(knowledgeTargetPrefix):]
	endIdx := strings.Index(afterPrefix, knowledgeTargetSuffix)
	if endIdx == -1 {
		return claudeMDPath
	}

	targetPath := strings.TrimSpace(afterPrefix[:endIdx])
	if targetPath == "" {
		return claudeMDPath
	}

	return filepath.Join(projectDir, targetPath)
}

// HashKnowledgeSection computes a hash of the knowledge section.
// Checks for target directive in CLAUDE.md to find the knowledge file.
// Returns empty string if no knowledge section exists.
func HashKnowledgeSection(projectDir string) string {
	knowledgePath := findKnowledgeTarget(projectDir)

	data, err := os.ReadFile(knowledgePath)
	if err != nil {
		return ""
	}

	content := string(data)

	// Extract knowledge section
	re := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(knowledgeSectionStart) + `(.*?)` + regexp.QuoteMeta(knowledgeSectionEnd))
	matches := re.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}

	hash := sha256.Sum256([]byte(matches[1]))
	return hex.EncodeToString(hash[:])
}

// ShouldExtractKnowledge returns true if the knowledge section wasn't updated by Claude.
func ShouldExtractKnowledge(beforeHash, afterHash string) bool {
	// If hashes match, Claude didn't update the section - try fallback extraction
	return beforeHash != "" && beforeHash == afterHash
}

// ExtractKnowledgeFromTranscript looks for patterns in Claude's output that
// indicate decisions, patterns, or gotchas that should be captured.
// This is a fallback mechanism - the primary method is Claude editing CLAUDE.md directly.
func ExtractKnowledgeFromTranscript(transcript, taskID string) *KnowledgeCapture {
	capture := &KnowledgeCapture{}

	// Look for decision indicators
	decisionPatterns := []string{
		`(?i)I decided to ([^.]+)\.`,
		`(?i)I chose ([^.]+) because ([^.]+)\.`,
		`(?i)The decision was to ([^.]+)\.`,
		`(?i)We should use ([^.]+) for ([^.]+)\.`,
	}

	for _, pattern := range decisionPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(transcript, 3) // Limit to 3 matches
		for _, match := range matches {
			if len(match) >= 2 && len(match[1]) > 10 && len(match[1]) < 200 {
				entry := KnowledgeEntry{
					Name:        truncate(match[1], 50),
					Description: truncate(match[1], 100),
					Source:      taskID,
				}
				if len(match) >= 3 && match[2] != "" {
					entry.Description = truncate(match[1]+" - "+match[2], 100)
				}
				capture.Decisions = append(capture.Decisions, entry)
			}
		}
	}

	// Look for pattern indicators
	patternPatterns := []string{
		`(?i)This pattern of ([^.]+)\.`,
		`(?i)Use the pattern ([^.]+)\.`,
		`(?i)Following the ([^.]+) pattern`,
		`(?i)Implemented using ([^.]+) pattern`,
		`(?i)Using the ([^.]+) pattern`,
	}

	for _, pattern := range patternPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(transcript, 3)
		for _, match := range matches {
			if len(match) >= 2 && len(match[1]) > 5 && len(match[1]) < 100 {
				capture.Patterns = append(capture.Patterns, KnowledgeEntry{
					Name:        truncate(match[1], 50),
					Description: truncate(match[1], 100),
					Source:      taskID,
				})
			}
		}
	}

	// Look for gotcha indicators
	gotchaPatterns := []string{
		`(?i)Note: ([^.]+) doesn(?:'|')t work because ([^.]+)\.`,
		`(?i)Watch out for ([^.]+)\.`,
		`(?i)The issue was ([^.]+)\.`,
		`(?i)Had to work around ([^.]+)\.`,
		`(?i)Gotcha: ([^.]+)\.`,
		`(?i)([^.]+) doesn(?:'|')t work because ([^.]+)\.`,
	}

	for _, pattern := range gotchaPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllStringSubmatch(transcript, 3)
		for _, match := range matches {
			if len(match) >= 2 && len(match[1]) > 10 && len(match[1]) < 200 {
				entry := KnowledgeEntry{
					Name:        truncate(match[1], 50),
					Description: truncate(match[1], 100),
					Source:      taskID,
				}
				if len(match) >= 3 && match[2] != "" {
					entry.Description = truncate(match[2], 100)
				}
				capture.Gotchas = append(capture.Gotchas, entry)
			}
		}
	}

	return capture
}

// HasEntries returns true if any knowledge was captured.
func (c *KnowledgeCapture) HasEntries() bool {
	return len(c.Patterns) > 0 || len(c.Gotchas) > 0 || len(c.Decisions) > 0
}

// AppendKnowledgeToClaudeMD adds extracted knowledge entries to the knowledge section.
// Checks for target directive in CLAUDE.md to find the knowledge file.
// Returns nil if no changes were made.
func AppendKnowledgeToClaudeMD(projectDir string, capture *KnowledgeCapture) error {
	if capture == nil || !capture.HasEntries() {
		return nil
	}

	knowledgePath := findKnowledgeTarget(projectDir)

	data, err := os.ReadFile(knowledgePath)
	if err != nil {
		return fmt.Errorf("read knowledge file %s: %w", knowledgePath, err)
	}

	content := string(data)

	// Find knowledge section
	re := regexp.MustCompile(`(?s)(` + regexp.QuoteMeta(knowledgeSectionStart) + `)(.*?)(` + regexp.QuoteMeta(knowledgeSectionEnd) + `)`)
	matches := re.FindStringSubmatchIndex(content)
	if matches == nil {
		return fmt.Errorf("no knowledge section found in %s", knowledgePath)
	}

	// Extract the section content
	sectionContent := content[matches[4]:matches[5]]

	// Append patterns
	for _, p := range capture.Patterns {
		row := fmt.Sprintf("| %s | %s | %s |\n", p.Name, p.Description, p.Source)
		sectionContent = insertTableRow(sectionContent, "### Patterns Learned", row)
	}

	// Append gotchas
	for _, g := range capture.Gotchas {
		row := fmt.Sprintf("| %s | %s | %s |\n", g.Name, g.Description, g.Source)
		sectionContent = insertTableRow(sectionContent, "### Known Gotchas", row)
	}

	// Append decisions
	for _, d := range capture.Decisions {
		row := fmt.Sprintf("| %s | %s | %s |\n", d.Name, d.Description, d.Source)
		sectionContent = insertTableRow(sectionContent, "### Decisions", row)
	}

	// Rebuild content
	newContent := content[:matches[4]] + sectionContent + content[matches[5]:]

	if err := os.WriteFile(knowledgePath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("write knowledge file %s: %w", knowledgePath, err)
	}

	return nil
}

// insertTableRow inserts a row after the table header row for the given section.
func insertTableRow(content, sectionHeader, row string) string {
	// Find the section header
	headerIdx := strings.Index(content, sectionHeader)
	if headerIdx == -1 {
		return content
	}

	// Find the table separator row (|---|...|)
	afterHeader := content[headerIdx:]
	separatorRe := regexp.MustCompile(`\|[-|]+\|\n`)
	sepMatch := separatorRe.FindStringIndex(afterHeader)
	if sepMatch == nil {
		return content
	}

	// Insert after the separator
	insertPoint := headerIdx + sepMatch[1]
	return content[:insertPoint] + row + content[insertPoint:]
}

// truncate limits a string to maxLen characters.
func truncate(s string, maxLen int) string {
	// Clean up whitespace and newlines
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "  ", " ")

	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// tryKnowledgeExtraction attempts to extract knowledge from the docs phase transcript
// if Claude didn't update the CLAUDE.md knowledge section directly.
// This is a fallback mechanism - the primary method is Claude editing CLAUDE.md.
func (e *Executor) tryKnowledgeExtraction(taskID string) {
	projectDir := e.config.WorkDir

	// Load the docs phase transcript
	transcript, err := LoadDocsPhaseTranscript(projectDir, taskID)
	if err != nil {
		e.logger.Debug("failed to load docs transcript", "error", err)
		return
	}

	if transcript == "" {
		e.logger.Debug("no docs transcript found, skipping extraction")
		return
	}

	// Extract knowledge from transcript
	capture := ExtractKnowledgeFromTranscript(transcript, taskID)
	if !capture.HasEntries() {
		e.logger.Debug("no knowledge extracted from transcript")
		return
	}

	// Append to CLAUDE.md
	if err := AppendKnowledgeToClaudeMD(projectDir, capture); err != nil {
		e.logger.Warn("failed to append knowledge to CLAUDE.md", "error", err)
		return
	}

	e.logger.Info("extracted knowledge from docs transcript",
		"patterns", len(capture.Patterns),
		"gotchas", len(capture.Gotchas),
		"decisions", len(capture.Decisions),
	)
}

// LoadDocsPhaseTranscript loads the transcript content from the docs phase.
func LoadDocsPhaseTranscript(projectDir, taskID string) (string, error) {
	transcriptDir := filepath.Join(projectDir, ".orc", "tasks", taskID, "transcripts")

	// Find docs phase transcripts
	pattern := filepath.Join(transcriptDir, "docs-*.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("glob transcripts: %w", err)
	}

	if len(matches) == 0 {
		return "", nil
	}

	// Concatenate all docs phase transcripts
	var content strings.Builder
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		content.Write(data)
		content.WriteString("\n")
	}

	return content.String(), nil
}
