package bootstrap

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// orcGitignoreEntries are the entries orc adds to .gitignore.
// Runtime state (DB, worktrees, exports) lives in ~/.orc/ now.
// Only .mcp.json needs ignoring in the project directory.
var orcGitignoreEntries = []string{
	"# orc - Claude Code Task Orchestrator",
	".mcp.json",
}

// updateGitignore adds orc entries to .gitignore if not already present.
func updateGitignore(workDir string) error {
	gitignorePath := filepath.Join(workDir, ".gitignore")

	// Read existing content
	existing := make(map[string]bool)
	if file, err := os.Open(gitignorePath); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			existing[strings.TrimSpace(scanner.Text())] = true
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return fmt.Errorf("read .gitignore: %w", err)
		}
		_ = file.Close()
	}

	// Check if any orc entries are missing
	var toAdd []string
	for _, entry := range orcGitignoreEntries {
		if !existing[entry] {
			toAdd = append(toAdd, entry)
		}
	}

	// Nothing to add
	if len(toAdd) == 0 {
		return nil
	}

	// Append to .gitignore
	file, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open .gitignore: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Add blank line before our entries if file isn't empty
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat .gitignore: %w", err)
	}
	if info.Size() > 0 {
		if _, err := file.WriteString("\n"); err != nil {
			return fmt.Errorf("write to .gitignore: %w", err)
		}
	}

	for _, entry := range toAdd {
		if _, err := file.WriteString(entry + "\n"); err != nil {
			return fmt.Errorf("write to .gitignore: %w", err)
		}
	}

	return nil
}
