package bootstrap

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// orcGitignoreEntries are the entries orc adds to .gitignore.
var orcGitignoreEntries = []string{
	"# orc - Claude Code Task Orchestrator",
	".orc/worktrees/",
	".orc/orc.db",
	".orc/orc.db-wal",
	".orc/orc.db-shm",
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
		file.Close()
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
		return err
	}
	defer file.Close()

	// Add blank line before our entries if file isn't empty
	info, _ := file.Stat()
	if info.Size() > 0 {
		file.WriteString("\n")
	}

	for _, entry := range toAdd {
		file.WriteString(entry + "\n")
	}

	return nil
}
