package workflow

import (
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/db"
	"github.com/randalmurphal/orc/templates"
)

// builtinHookScripts defines the built-in hook scripts to seed from embedded templates.
var builtinHookScripts = []struct {
	ID          string
	Name        string
	Description string
	File        string // path within templates.Hooks embed FS
	EventType   string
}{
	{
		ID:          "orc-verify-completion",
		Name:        "Verify Completion",
		Description: "Validates that the phase produced proper completion JSON output",
		File:        "hooks/orc-verify-completion.sh",
		EventType:   "Stop",
	},
	{
		ID:          "orc-tdd-discipline",
		Name:        "TDD Discipline",
		Description: "Reminds Claude to write tests before implementation during TDD phases",
		File:        "hooks/orc-tdd-discipline.sh",
		EventType:   "PreToolUse",
	},
	{
		ID:          "orc-worktree-isolation",
		Name:        "Worktree Isolation",
		Description: "Enforces file operations stay within the worktree directory",
		File:        "hooks/orc-worktree-isolation.py",
		EventType:   "PreToolUse",
	},
}

// SeedHookScripts populates the database with built-in hook script definitions.
// Reads hook script files from embedded templates and creates database records.
// Returns the number of hook scripts seeded/updated. Updates existing built-in
// scripts to pick up content changes from new versions.
func SeedHookScripts(gdb *db.GlobalDB) (int, error) {
	seeded := 0

	for _, def := range builtinHookScripts {
		content, err := templates.Hooks.ReadFile(def.File)
		if err != nil {
			return seeded, fmt.Errorf("read hook script file %s: %w", def.File, err)
		}
		trimmedContent := strings.TrimSuffix(string(content), "\n")

		// Check if already exists
		existing, err := gdb.GetHookScript(def.ID)
		if err != nil {
			return seeded, fmt.Errorf("check hook script %s: %w", def.ID, err)
		}

		// Skip if exists with identical content
		if existing != nil && existing.Content == trimmedContent {
			continue
		}

		// Skip non-builtin user-modified scripts
		if existing != nil && !existing.IsBuiltin {
			continue
		}

		hs := &db.HookScript{
			ID:          def.ID,
			Name:        def.Name,
			Description: def.Description,
			Content:     trimmedContent,
			EventType:   def.EventType,
			IsBuiltin:   true,
		}

		// Preserve original creation time on update
		if existing != nil {
			hs.CreatedAt = existing.CreatedAt
		}

		if err := gdb.SaveHookScript(hs); err != nil {
			return seeded, fmt.Errorf("save hook script %s: %w", def.ID, err)
		}
		seeded++
	}

	return seeded, nil
}

// SeedSkills populates the database with built-in skill definitions.
// Currently an infrastructure stub — seeds 0 skills.
// Returns the number of skills seeded.
func SeedSkills(gdb *db.GlobalDB) (int, error) {
	// Infrastructure only — no built-in skills to seed yet.
	// This function exists so the bootstrap code can call SeedSkills
	// alongside SeedHookScripts when skills are added later.
	_ = gdb // Will be used when skills are added
	return 0, nil
}
