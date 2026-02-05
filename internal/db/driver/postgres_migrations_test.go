package driver

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestPostgresMigrations_DirectoryExists verifies that the PostgreSQL migrations directory exists.
// This test covers SC-5: Migration numbering matches SQLite.
func TestPostgresMigrations_DirectoryExists(t *testing.T) {
	schemaDir := findSchemaDir(t)
	postgresDir := filepath.Join(schemaDir, "postgres")

	if _, err := os.Stat(postgresDir); os.IsNotExist(err) {
		t.Fatalf("postgres migrations directory does not exist: %s", postgresDir)
	}
}

// TestPostgresMigrations_AllFilesExist verifies that all 10 global migration files exist.
// Covers SC-5: Migration numbering matches SQLite (both have files 001-010).
func TestPostgresMigrations_AllFilesExist(t *testing.T) {
	schemaDir := findSchemaDir(t)
	postgresDir := filepath.Join(schemaDir, "postgres")

	expectedFiles := []string{
		"global_001.sql",
		"global_002.sql",
		"global_003.sql",
		"global_004.sql",
		"global_005.sql",
		"global_006.sql",
		"global_007.sql",
		"global_008.sql",
		"global_009.sql",
		"global_010.sql",
	}

	for _, file := range expectedFiles {
		path := filepath.Join(postgresDir, file)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("missing PostgreSQL migration file: %s", file)
		}
	}
}

// TestPostgresMigrations_NoSQLiteisms verifies PostgreSQL migrations don't contain SQLite-specific syntax.
// Covers SC-3: PostgreSQL migrations use correct dialect syntax.
func TestPostgresMigrations_NoSQLiteisms(t *testing.T) {
	schemaDir := findSchemaDir(t)
	postgresDir := filepath.Join(schemaDir, "postgres")

	// Patterns that indicate SQLite-specific syntax
	sqlitePatterns := []struct {
		pattern *regexp.Regexp
		desc    string
	}{
		{regexp.MustCompile(`datetime\s*\(\s*'now'\s*\)`), "datetime('now') - use NOW() instead"},
		{regexp.MustCompile(`\bAUTOINCREMENT\b`), "AUTOINCREMENT - use SERIAL instead"},
		{regexp.MustCompile(`strftime\s*\(`), "strftime() - use PostgreSQL date functions"},
		// Look for ? placeholders but not inside quotes or comments
		{regexp.MustCompile(`(?m)^[^'-]*\?[^'-]*$`), "? placeholder - use $1, $2, etc."},
	}

	files, err := filepath.Glob(filepath.Join(postgresDir, "global_*.sql"))
	if err != nil {
		t.Fatalf("failed to glob postgres migrations: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no PostgreSQL migration files found")
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("failed to read %s: %v", file, err)
			continue
		}

		contentStr := string(content)
		for _, pat := range sqlitePatterns {
			if pat.pattern.MatchString(contentStr) {
				t.Errorf("%s contains SQLite-ism: %s", filepath.Base(file), pat.desc)
			}
		}
	}
}

// TestPostgresMigrations_UsesCorrectSyntax verifies PostgreSQL migrations use correct PostgreSQL syntax.
// Covers SC-3: PostgreSQL migrations use correct dialect syntax.
func TestPostgresMigrations_UsesCorrectSyntax(t *testing.T) {
	schemaDir := findSchemaDir(t)
	postgresDir := filepath.Join(schemaDir, "postgres")

	// Read global_001.sql which should have the basic tables
	content, err := os.ReadFile(filepath.Join(postgresDir, "global_001.sql"))
	if err != nil {
		t.Fatalf("failed to read global_001.sql: %v", err)
	}

	contentStr := string(content)

	// Should use SERIAL or BIGSERIAL for auto-incrementing PKs
	if strings.Contains(contentStr, "INTEGER PRIMARY KEY AUTOINCREMENT") {
		t.Error("global_001.sql should use SERIAL PRIMARY KEY instead of INTEGER PRIMARY KEY AUTOINCREMENT")
	}

	// Should use NOW() for default timestamps
	if strings.Contains(contentStr, "datetime('now')") {
		t.Error("global_001.sql should use NOW() or CURRENT_TIMESTAMP instead of datetime('now')")
	}

	// Should use TIMESTAMP WITH TIME ZONE for timestamp columns
	if strings.Contains(contentStr, "created_at TEXT DEFAULT") && !strings.Contains(contentStr, "TIMESTAMP") {
		t.Error("global_001.sql should use TIMESTAMP WITH TIME ZONE for timestamp columns")
	}
}

// TestPostgresMigrations_SameTableCount verifies PostgreSQL creates same tables as SQLite.
// Covers SC-1: PostgreSQL migrations create identical table structures to SQLite.
func TestPostgresMigrations_SameTableCount(t *testing.T) {
	schemaDir := findSchemaDir(t)

	// Parse table names from SQLite migrations
	sqliteTables := extractTableNames(t, filepath.Join(schemaDir, "global_*.sql"))

	// Parse table names from PostgreSQL migrations
	postgresTables := extractTableNames(t, filepath.Join(schemaDir, "postgres", "global_*.sql"))

	// Compare
	if len(sqliteTables) == 0 {
		t.Fatal("no SQLite tables found")
	}

	if len(postgresTables) == 0 {
		t.Fatal("no PostgreSQL tables found - migrations likely don't exist yet")
	}

	// Check that all SQLite tables exist in PostgreSQL
	for table := range sqliteTables {
		if !postgresTables[table] {
			t.Errorf("table %s exists in SQLite but not in PostgreSQL migrations", table)
		}
	}

	// Check for extra tables in PostgreSQL (shouldn't happen)
	for table := range postgresTables {
		if !sqliteTables[table] {
			t.Errorf("table %s exists in PostgreSQL but not in SQLite migrations", table)
		}
	}
}

// TestPostgresMigrations_SameIndexes verifies PostgreSQL creates same indexes as SQLite.
// Covers SC-6: All indexes from SQLite exist in PostgreSQL.
func TestPostgresMigrations_SameIndexes(t *testing.T) {
	schemaDir := findSchemaDir(t)

	// Parse index names from SQLite migrations
	sqliteIndexes := extractIndexNames(t, filepath.Join(schemaDir, "global_*.sql"))

	// Parse index names from PostgreSQL migrations
	postgresIndexes := extractIndexNames(t, filepath.Join(schemaDir, "postgres", "global_*.sql"))

	if len(sqliteIndexes) == 0 {
		t.Fatal("no SQLite indexes found")
	}

	if len(postgresIndexes) == 0 {
		t.Fatal("no PostgreSQL indexes found - migrations likely don't exist yet")
	}

	// Check that all SQLite indexes exist in PostgreSQL
	for idx := range sqliteIndexes {
		if !postgresIndexes[idx] {
			t.Errorf("index %s exists in SQLite but not in PostgreSQL migrations", idx)
		}
	}
}

// TestPostgresMigrations_ColumnsMatch verifies key tables have matching columns.
// Covers SC-1: PostgreSQL migrations create identical table structures to SQLite.
func TestPostgresMigrations_ColumnsMatch(t *testing.T) {
	schemaDir := findSchemaDir(t)

	// Key tables to verify column structure
	tablesToCheck := []string{"projects", "cost_log", "users", "workflows", "phase_templates", "agents"}

	for _, table := range tablesToCheck {
		t.Run(table, func(t *testing.T) {
			sqliteCols := extractTableColumns(t, filepath.Join(schemaDir, "global_*.sql"), table)
			postgresCols := extractTableColumns(t, filepath.Join(schemaDir, "postgres", "global_*.sql"), table)

			if len(sqliteCols) == 0 {
				t.Fatalf("no columns found for table %s in SQLite", table)
			}

			if len(postgresCols) == 0 {
				t.Fatalf("no columns found for table %s in PostgreSQL - migrations likely don't exist yet", table)
			}

			// Check column names match
			for col := range sqliteCols {
				if !postgresCols[col] {
					t.Errorf("column %s.%s exists in SQLite but not in PostgreSQL", table, col)
				}
			}
		})
	}
}

// TestPostgresMigrations_NumberingMatchesSQLite verifies migration file numbering is identical.
// Covers SC-5: Migration numbering matches SQLite (both have files 001-010).
func TestPostgresMigrations_NumberingMatchesSQLite(t *testing.T) {
	schemaDir := findSchemaDir(t)

	// Get SQLite migration numbers
	sqliteFiles, err := filepath.Glob(filepath.Join(schemaDir, "global_*.sql"))
	if err != nil {
		t.Fatalf("failed to glob SQLite migrations: %v", err)
	}

	// Get PostgreSQL migration numbers
	postgresFiles, err := filepath.Glob(filepath.Join(schemaDir, "postgres", "global_*.sql"))
	if err != nil {
		t.Fatalf("failed to glob PostgreSQL migrations: %v", err)
	}

	if len(postgresFiles) == 0 {
		t.Fatal("no PostgreSQL migration files found")
	}

	// Extract just the numbers and compare
	sqliteNums := make(map[string]bool)
	for _, f := range sqliteFiles {
		base := filepath.Base(f)
		if num := extractMigrationNumber(base); num != "" {
			sqliteNums[num] = true
		}
	}

	postgresNums := make(map[string]bool)
	for _, f := range postgresFiles {
		base := filepath.Base(f)
		if num := extractMigrationNumber(base); num != "" {
			postgresNums[num] = true
		}
	}

	// Verify all SQLite numbers exist in PostgreSQL
	var missingNums []string
	for num := range sqliteNums {
		if !postgresNums[num] {
			missingNums = append(missingNums, num)
		}
	}

	if len(missingNums) > 0 {
		sort.Strings(missingNums)
		t.Errorf("PostgreSQL migrations missing numbers: %v", missingNums)
	}
}

// Helper functions

// findSchemaDir locates the schema directory relative to the test file.
func findSchemaDir(t *testing.T) string {
	t.Helper()

	// Try relative path from driver package
	schemaDir := filepath.Join("..", "schema")
	if _, err := os.Stat(schemaDir); err == nil {
		return schemaDir
	}

	// Try from project root
	schemaDir = filepath.Join("internal", "db", "schema")
	if _, err := os.Stat(schemaDir); err == nil {
		return schemaDir
	}

	t.Fatal("could not find schema directory")
	return ""
}

// extractTableNames parses CREATE TABLE statements and returns table names.
func extractTableNames(t *testing.T, pattern string) map[string]bool {
	t.Helper()

	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("failed to glob %s: %v", pattern, err)
	}

	tables := make(map[string]bool)
	tableRe := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("failed to read %s: %v", file, err)
			continue
		}

		matches := tableRe.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			if len(match) > 1 {
				tables[match[1]] = true
			}
		}
	}

	return tables
}

// extractIndexNames parses CREATE INDEX statements and returns index names.
func extractIndexNames(t *testing.T, pattern string) map[string]bool {
	t.Helper()

	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("failed to glob %s: %v", pattern, err)
	}

	indexes := make(map[string]bool)
	indexRe := regexp.MustCompile(`(?i)CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("failed to read %s: %v", file, err)
			continue
		}

		matches := indexRe.FindAllStringSubmatch(string(content), -1)
		for _, match := range matches {
			if len(match) > 1 {
				indexes[match[1]] = true
			}
		}
	}

	return indexes
}

// extractTableColumns parses column names for a specific table.
func extractTableColumns(t *testing.T, pattern string, tableName string) map[string]bool {
	t.Helper()

	files, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("failed to glob %s: %v", pattern, err)
	}

	columns := make(map[string]bool)

	// Match CREATE TABLE and capture the column definitions block
	tableRe := regexp.MustCompile(`(?is)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?` + tableName + `\s*\(([^;]+)\)`)
	// Match ALTER TABLE ADD COLUMN
	alterRe := regexp.MustCompile(`(?i)ALTER\s+TABLE\s+` + tableName + `\s+ADD\s+(?:COLUMN\s+)?(\w+)`)

	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Extract columns from CREATE TABLE
		if matches := tableRe.FindStringSubmatch(string(content)); len(matches) > 1 {
			// Parse column definitions
			colDefs := matches[1]
			// Split by comma but be careful of nested parens
			cols := splitColumns(colDefs)
			for _, col := range cols {
				col = strings.TrimSpace(col)
				if col == "" {
					continue
				}
				// Skip constraints (PRIMARY KEY, FOREIGN KEY, UNIQUE, etc.)
				upperCol := strings.ToUpper(col)
				if strings.HasPrefix(upperCol, "PRIMARY") ||
					strings.HasPrefix(upperCol, "FOREIGN") ||
					strings.HasPrefix(upperCol, "UNIQUE(") ||
					strings.HasPrefix(upperCol, "CHECK") {
					continue
				}
				// First word is the column name
				parts := strings.Fields(col)
				if len(parts) > 0 {
					columns[parts[0]] = true
				}
			}
		}

		// Extract columns from ALTER TABLE ADD COLUMN
		for _, match := range alterRe.FindAllStringSubmatch(string(content), -1) {
			if len(match) > 1 {
				columns[match[1]] = true
			}
		}
	}

	return columns
}

// splitColumns splits a column definition block by commas, respecting parentheses.
func splitColumns(s string) []string {
	var result []string
	var current strings.Builder
	depth := 0

	for _, r := range s {
		switch r {
		case '(':
			depth++
			current.WriteRune(r)
		case ')':
			depth--
			current.WriteRune(r)
		case ',':
			if depth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(r)
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// extractMigrationNumber extracts the number from a migration filename.
func extractMigrationNumber(filename string) string {
	re := regexp.MustCompile(`global_(\d+)\.sql`)
	if matches := re.FindStringSubmatch(filename); len(matches) > 1 {
		return matches[1]
	}
	return ""
}
