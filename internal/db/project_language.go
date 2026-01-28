package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ProjectLanguage represents a detected language in a project.
// Supports multi-language projects (Go + TypeScript, Python + JavaScript, etc.)
type ProjectLanguage struct {
	ID           int64     `json:"id"`
	Language     string    `json:"language"`      // go, typescript, python, javascript, rust, etc.
	RootPath     string    `json:"root_path"`     // Relative path: "" = project root, "web/" = subdir
	IsPrimary    bool      `json:"is_primary"`    // User-designated primary language
	Frameworks   []string  `json:"frameworks"`    // Detected frameworks (React, Gin, FastAPI, etc.)
	BuildTool    string    `json:"build_tool"`    // npm, yarn, pnpm, bun, poetry, cargo, make
	TestCommand  string    `json:"test_command"`  // Inferred test command
	LintCommand  string    `json:"lint_command"`  // Inferred lint command
	BuildCommand string    `json:"build_command"` // Inferred build command
	DetectedAt   time.Time `json:"detected_at"`
}

// ErrProjectLanguageNotFound is returned when a language entry doesn't exist.
var ErrProjectLanguageNotFound = errors.New("project language not found")

// SaveProjectLanguage creates or updates a project language entry.
// The unique key is (language, root_path).
func (p *ProjectDB) SaveProjectLanguage(lang *ProjectLanguage) error {
	now := time.Now().UTC().Format(time.RFC3339)

	var frameworksJSON []byte
	if len(lang.Frameworks) > 0 {
		var err error
		frameworksJSON, err = json.Marshal(lang.Frameworks)
		if err != nil {
			return fmt.Errorf("marshal frameworks: %w", err)
		}
	}

	isPrimary := 0
	if lang.IsPrimary {
		isPrimary = 1
	}

	result, err := p.Exec(`
		INSERT INTO project_languages (language, root_path, is_primary, frameworks, build_tool, test_command, lint_command, build_command, detected_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(language, root_path) DO UPDATE SET
			is_primary = excluded.is_primary,
			frameworks = excluded.frameworks,
			build_tool = excluded.build_tool,
			test_command = excluded.test_command,
			lint_command = excluded.lint_command,
			build_command = excluded.build_command,
			detected_at = excluded.detected_at
	`, lang.Language, lang.RootPath, isPrimary, string(frameworksJSON), lang.BuildTool,
		lang.TestCommand, lang.LintCommand, lang.BuildCommand, now)
	if err != nil {
		return fmt.Errorf("save project language %s at %s: %w", lang.Language, lang.RootPath, err)
	}

	// Get the ID for new inserts
	if lang.ID == 0 {
		id, _ := result.LastInsertId()
		lang.ID = id
	}

	return nil
}

// GetProjectLanguage retrieves a project language by language and root path.
func (p *ProjectDB) GetProjectLanguage(language, rootPath string) (*ProjectLanguage, error) {
	row := p.QueryRow(`
		SELECT id, language, root_path, is_primary, frameworks, build_tool, test_command, lint_command, build_command, detected_at
		FROM project_languages
		WHERE language = ? AND root_path = ?
	`, language, rootPath)

	lang, err := scanProjectLanguage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectLanguageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get project language %s at %s: %w", language, rootPath, err)
	}

	return lang, nil
}

// ListProjectLanguages returns all project languages ordered by primary status and language.
func (p *ProjectDB) ListProjectLanguages() ([]*ProjectLanguage, error) {
	rows, err := p.Query(`
		SELECT id, language, root_path, is_primary, frameworks, build_tool, test_command, lint_command, build_command, detected_at
		FROM project_languages
		ORDER BY is_primary DESC, language ASC, root_path ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list project languages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var langs []*ProjectLanguage
	for rows.Next() {
		lang, err := scanProjectLanguageRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan project language: %w", err)
		}
		langs = append(langs, lang)
	}

	return langs, rows.Err()
}

// GetPrimaryLanguage returns the primary language, or the first detected language if none is marked primary.
func (p *ProjectDB) GetPrimaryLanguage() (*ProjectLanguage, error) {
	row := p.QueryRow(`
		SELECT id, language, root_path, is_primary, frameworks, build_tool, test_command, lint_command, build_command, detected_at
		FROM project_languages
		ORDER BY is_primary DESC, detected_at ASC
		LIMIT 1
	`)

	lang, err := scanProjectLanguage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectLanguageNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get primary project language: %w", err)
	}

	return lang, nil
}

// SetPrimaryLanguage sets a language as the primary, clearing the primary flag from others.
func (p *ProjectDB) SetPrimaryLanguage(language, rootPath string) error {
	// Clear all primary flags
	_, err := p.Exec("UPDATE project_languages SET is_primary = 0")
	if err != nil {
		return fmt.Errorf("clear primary flags: %w", err)
	}

	// Set the new primary
	result, err := p.Exec(`
		UPDATE project_languages
		SET is_primary = 1
		WHERE language = ? AND root_path = ?
	`, language, rootPath)
	if err != nil {
		return fmt.Errorf("set primary language %s at %s: %w", language, rootPath, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProjectLanguageNotFound
	}

	return nil
}

// DeleteProjectLanguage removes a project language entry.
func (p *ProjectDB) DeleteProjectLanguage(language, rootPath string) error {
	result, err := p.Exec("DELETE FROM project_languages WHERE language = ? AND root_path = ?", language, rootPath)
	if err != nil {
		return fmt.Errorf("delete project language %s at %s: %w", language, rootPath, err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrProjectLanguageNotFound
	}

	return nil
}

// DeleteAllProjectLanguages removes all project language entries.
// Used during re-detection.
func (p *ProjectDB) DeleteAllProjectLanguages() error {
	_, err := p.Exec("DELETE FROM project_languages")
	if err != nil {
		return fmt.Errorf("delete all project languages: %w", err)
	}
	return nil
}

// HasFrontend returns true if any detected language indicates a frontend stack.
func (p *ProjectDB) HasFrontend() (bool, error) {
	var count int
	err := p.QueryRow(`
		SELECT COUNT(*) FROM project_languages
		WHERE language IN ('typescript', 'javascript')
		   OR root_path IN ('web', 'frontend', 'client', 'ui')
		   OR frameworks LIKE '%React%'
		   OR frameworks LIKE '%Vue%'
		   OR frameworks LIKE '%Svelte%'
		   OR frameworks LIKE '%Angular%'
	`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check has frontend: %w", err)
	}
	return count > 0, nil
}

// --------- Scanners ---------

func scanProjectLanguage(row rowScanner) (*ProjectLanguage, error) {
	lang := &ProjectLanguage{}
	var frameworksJSON, buildTool, testCmd, lintCmd, buildCmd sql.NullString
	var isPrimary int
	var detectedAt string

	err := row.Scan(
		&lang.ID,
		&lang.Language,
		&lang.RootPath,
		&isPrimary,
		&frameworksJSON,
		&buildTool,
		&testCmd,
		&lintCmd,
		&buildCmd,
		&detectedAt,
	)
	if err != nil {
		return nil, err
	}

	lang.IsPrimary = isPrimary == 1
	lang.BuildTool = buildTool.String
	lang.TestCommand = testCmd.String
	lang.LintCommand = lintCmd.String
	lang.BuildCommand = buildCmd.String
	lang.DetectedAt, _ = time.Parse(time.RFC3339, detectedAt)

	if frameworksJSON.Valid && frameworksJSON.String != "" {
		if err := json.Unmarshal([]byte(frameworksJSON.String), &lang.Frameworks); err != nil {
			// Ignore JSON parse errors, just leave empty
			lang.Frameworks = nil
		}
	}

	return lang, nil
}

func scanProjectLanguageRow(rows *sql.Rows) (*ProjectLanguage, error) {
	return scanProjectLanguage(rows)
}
