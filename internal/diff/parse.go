package diff

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// parseStats parses git diff --shortstat output.
// Example: " 5 files changed, 120 insertions(+), 45 deletions(-)"
func parseStats(output string) (*DiffStats, error) {
	stats := &DiffStats{}
	output = strings.TrimSpace(output)

	if output == "" {
		return stats, nil
	}

	// Pattern: N file(s) changed, N insertion(s)(+), N deletion(s)(-)
	filesRe := regexp.MustCompile(`(\d+)\s+files?\s+changed`)
	insertRe := regexp.MustCompile(`(\d+)\s+insertions?\(\+\)`)
	deleteRe := regexp.MustCompile(`(\d+)\s+deletions?\(-\)`)

	if matches := filesRe.FindStringSubmatch(output); len(matches) > 1 {
		stats.FilesChanged, _ = strconv.Atoi(matches[1])
	}
	if matches := insertRe.FindStringSubmatch(output); len(matches) > 1 {
		stats.Additions, _ = strconv.Atoi(matches[1])
	}
	if matches := deleteRe.FindStringSubmatch(output); len(matches) > 1 {
		stats.Deletions, _ = strconv.Atoi(matches[1])
	}

	return stats, nil
}

// parseNumstat parses git diff --numstat output.
// Format: additions<tab>deletions<tab>path
// Binary files show as: -<tab>-<tab>path
func parseNumstat(output string) ([]FileDiff, error) {
	var files []FileDiff
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split by tab
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		addStr := parts[0]
		delStr := parts[1]
		path := parts[2]

		// Handle renames: "old path" => "new path" format
		if strings.Contains(path, " => ") {
			// Extract the new path from rename notation
			// Could be "dir/{old => new}.ext" or "old => new"
			path = extractNewPath(path)
		}

		// Check for binary files (shown as - -)
		binary := addStr == "-" && delStr == "-"

		var additions, deletions int
		if !binary {
			additions, _ = strconv.Atoi(addStr)
			deletions, _ = strconv.Atoi(delStr)
		}

		// Infer status from additions/deletions
		status := "modified"
		if additions > 0 && deletions == 0 {
			// Could be added or just additions, will be refined by name-status
			status = "modified"
		}

		files = append(files, FileDiff{
			Path:      path,
			Status:    status,
			Additions: additions,
			Deletions: deletions,
			Binary:    binary,
			Syntax:    detectSyntax(path),
		})
	}

	return files, nil
}

// extractNewPath extracts the new path from git rename notation.
// Examples:
//   - "old.txt => new.txt" -> "new.txt"
//   - "dir/{old.txt => new.txt}" -> "dir/new.txt"
//   - "{old => new}/file.txt" -> "new/file.txt"
func extractNewPath(path string) string {
	// Handle simple case: "old => new"
	if strings.Contains(path, " => ") && !strings.Contains(path, "{") {
		parts := strings.Split(path, " => ")
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}

	// Handle brace notation: "prefix/{old => new}/suffix"
	braceRe := regexp.MustCompile(`\{([^}]*)\s+=>\s+([^}]*)\}`)
	if matches := braceRe.FindStringSubmatch(path); len(matches) == 3 {
		// Replace the brace section with the new part
		newPath := braceRe.ReplaceAllString(path, matches[2])
		return newPath
	}

	return path
}

// parseFileDiff parses unified diff output for a single file.
func parseFileDiff(output, filePath string) *FileDiff {
	diff := &FileDiff{
		Path:   filePath,
		Status: "modified",
		Hunks:  []Hunk{},
	}

	lines := strings.Split(output, "\n")
	var currentHunk *Hunk
	var oldLine, newLine int

	// @@ -start,count +start,count @@ optional header
	hunkRe := regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

	for _, line := range lines {
		// Check for binary file marker
		if strings.HasPrefix(line, "Binary files") {
			diff.Binary = true
			continue
		}

		// Check for new hunk
		if matches := hunkRe.FindStringSubmatch(line); matches != nil {
			// Save previous hunk
			if currentHunk != nil {
				diff.Hunks = append(diff.Hunks, *currentHunk)
			}

			oldStart, _ := strconv.Atoi(matches[1])
			oldLines := 1
			if matches[2] != "" {
				oldLines, _ = strconv.Atoi(matches[2])
			}
			newStart, _ := strconv.Atoi(matches[3])
			newLines := 1
			if matches[4] != "" {
				newLines, _ = strconv.Atoi(matches[4])
			}

			currentHunk = &Hunk{
				OldStart: oldStart,
				OldLines: oldLines,
				NewStart: newStart,
				NewLines: newLines,
				Lines:    []Line{},
			}
			oldLine = oldStart
			newLine = newStart
			continue
		}

		if currentHunk == nil {
			continue
		}

		// Parse diff lines
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			currentHunk.Lines = append(currentHunk.Lines, Line{
				Type:    "addition",
				Content: strings.TrimPrefix(line, "+"),
				NewLine: newLine,
			})
			diff.Additions++
			newLine++
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			currentHunk.Lines = append(currentHunk.Lines, Line{
				Type:    "deletion",
				Content: strings.TrimPrefix(line, "-"),
				OldLine: oldLine,
			})
			diff.Deletions++
			oldLine++
		} else if strings.HasPrefix(line, " ") {
			currentHunk.Lines = append(currentHunk.Lines, Line{
				Type:    "context",
				Content: strings.TrimPrefix(line, " "),
				OldLine: oldLine,
				NewLine: newLine,
			})
			oldLine++
			newLine++
		} else if line == "" && len(currentHunk.Lines) > 0 {
			// Empty line in diff - treat as context if we have previous lines
			lastLine := currentHunk.Lines[len(currentHunk.Lines)-1]
			if lastLine.Type == "context" {
				currentHunk.Lines = append(currentHunk.Lines, Line{
					Type:    "context",
					Content: "",
					OldLine: oldLine,
					NewLine: newLine,
				})
				oldLine++
				newLine++
			}
		}
	}

	// Don't forget the last hunk
	if currentHunk != nil {
		diff.Hunks = append(diff.Hunks, *currentHunk)
	}

	return diff
}

// detectSyntax determines the syntax highlighting language from file extension.
func detectSyntax(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))

	// Check for special filenames first
	switch base {
	case "dockerfile", "containerfile":
		return "dockerfile"
	case "makefile", "gnumakefile":
		return "makefile"
	case ".gitignore", ".dockerignore":
		return "gitignore"
	case "cmakelists.txt":
		return "cmake"
	}

	// Check by extension
	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".jsx":
		return "jsx"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "tsx"
	case ".py", ".pyi":
		return "python"
	case ".rs":
		return "rust"
	case ".rb":
		return "ruby"
	case ".java":
		return "java"
	case ".kt", ".kts":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".c", ".h":
		return "c"
	case ".cpp", ".hpp", ".cc", ".cxx", ".hxx":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".fs", ".fsx":
		return "fsharp"
	case ".swift":
		return "swift"
	case ".m", ".mm":
		return "objective-c"
	case ".php":
		return "php"
	case ".pl", ".pm":
		return "perl"
	case ".lua":
		return "lua"
	case ".r":
		return "r"
	case ".jl":
		return "julia"
	case ".ex", ".exs":
		return "elixir"
	case ".erl", ".hrl":
		return "erlang"
	case ".hs":
		return "haskell"
	case ".ml", ".mli":
		return "ocaml"
	case ".clj", ".cljs", ".cljc":
		return "clojure"
	case ".lisp", ".cl":
		return "lisp"
	case ".scm", ".ss":
		return "scheme"
	case ".nim":
		return "nim"
	case ".zig":
		return "zig"
	case ".v":
		return "v"
	case ".d":
		return "d"
	case ".css":
		return "css"
	case ".scss":
		return "scss"
	case ".sass":
		return "sass"
	case ".less":
		return "less"
	case ".html", ".htm":
		return "html"
	case ".vue":
		return "vue"
	case ".svelte":
		return "svelte"
	case ".xml", ".xsl", ".xslt":
		return "xml"
	case ".svg":
		return "svg"
	case ".json":
		return "json"
	case ".json5":
		return "json5"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".ini", ".cfg":
		return "ini"
	case ".md", ".markdown":
		return "markdown"
	case ".rst":
		return "rst"
	case ".tex":
		return "latex"
	case ".sql":
		return "sql"
	case ".graphql", ".gql":
		return "graphql"
	case ".proto":
		return "protobuf"
	case ".sh", ".bash", ".zsh":
		return "bash"
	case ".ps1", ".psm1":
		return "powershell"
	case ".bat", ".cmd":
		return "batch"
	case ".fish":
		return "fish"
	case ".diff", ".patch":
		return "diff"
	case ".tf", ".tfvars":
		return "hcl"
	case ".nix":
		return "nix"
	case ".dhall":
		return "dhall"
	case ".sol":
		return "solidity"
	case ".vy":
		return "vyper"
	case ".wasm":
		return "wasm"
	case ".wat":
		return "wat"
	default:
		return "text"
	}
}
