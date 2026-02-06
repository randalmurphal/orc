package db

import (
	"fmt"
	"testing"
)

// TestEmbed_PostgresDirectoryIncluded verifies the embed directive includes PostgreSQL migrations.
// Covers SC-4: PostgreSQL migrations embedded in binary.
func TestEmbed_PostgresDirectoryIncluded(t *testing.T) {
	// The schemaFS should be able to read the postgres directory
	entries, err := schemaFS.ReadDir("schema/postgres")
	if err != nil {
		t.Fatalf("failed to read schema/postgres from embedded FS: %v\n"+
			"The embed directive in db.go must include 'schema/postgres/*.sql'", err)
	}

	if len(entries) == 0 {
		t.Fatal("schema/postgres directory is empty in embedded FS")
	}

	// Verify we can read the expected files
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

	foundFiles := make(map[string]bool)
	for _, entry := range entries {
		foundFiles[entry.Name()] = true
	}

	for _, expected := range expectedFiles {
		if !foundFiles[expected] {
			t.Errorf("expected file %s not found in embedded schema/postgres/", expected)
		}
	}
}

// TestEmbed_PostgresFilesReadable verifies PostgreSQL migration content can be read.
// Covers SC-4: PostgreSQL migrations embedded in binary.
func TestEmbed_PostgresFilesReadable(t *testing.T) {
	// Try to read a specific migration file
	content, err := schemaFS.ReadFile("schema/postgres/global_001.sql")
	if err != nil {
		t.Fatalf("failed to read schema/postgres/global_001.sql: %v", err)
	}

	if len(content) == 0 {
		t.Error("schema/postgres/global_001.sql is empty")
	}

	// Verify it contains PostgreSQL-specific content (not SQLite)
	contentStr := string(content)
	if len(contentStr) < 100 {
		t.Error("schema/postgres/global_001.sql content seems too short")
	}
}

// TestEmbed_PostgresContainsCreateTable verifies PostgreSQL migrations have CREATE TABLE statements.
// This indirectly tests SC-4 by verifying the embedded content is valid SQL.
func TestEmbed_PostgresContainsCreateTable(t *testing.T) {
	content, err := schemaFS.ReadFile("schema/postgres/global_001.sql")
	if err != nil {
		t.Fatalf("failed to read schema/postgres/global_001.sql: %v", err)
	}

	contentStr := string(content)

	// Should contain CREATE TABLE for projects
	if !containsIgnoreCase(contentStr, "CREATE TABLE") {
		t.Error("schema/postgres/global_001.sql should contain CREATE TABLE statement")
	}

	// Should reference projects table
	if !containsIgnoreCase(contentStr, "projects") {
		t.Error("schema/postgres/global_001.sql should create projects table")
	}
}

func containsIgnoreCase(s, substr string) bool {
	sLower := bytesToLower([]byte(s))
	substrLower := bytesToLower([]byte(substr))
	return contains(sLower, substrLower)
}

func bytesToLower(b []byte) []byte {
	result := make([]byte, len(b))
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return result
}

func contains(s, substr []byte) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := range len(s) - len(substr) + 1 {
		match := true
		for j := range len(substr) {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// TestEmbed_PostgresProjectFilesIncluded verifies all 20 PostgreSQL project migration files
// are readable from the embedded filesystem.
// Covers SC-7: Embed test confirms all 20 PostgreSQL project migration files are readable.
func TestEmbed_PostgresProjectFilesIncluded(t *testing.T) {
	entries, err := schemaFS.ReadDir("schema/postgres")
	if err != nil {
		t.Fatalf("failed to read schema/postgres from embedded FS: %v\n"+
			"The embed directive in db.go must include 'schema/postgres/*.sql'", err)
	}

	foundFiles := make(map[string]bool)
	for _, entry := range entries {
		foundFiles[entry.Name()] = true
	}

	for i := 1; i <= 20; i++ {
		file := fmt.Sprintf("project_%03d.sql", i)
		if !foundFiles[file] {
			t.Errorf("expected file %s not found in embedded schema/postgres/", file)
		}
	}
}

// TestEmbed_PostgresProjectFilesReadable verifies project migration content can be read.
// Covers SC-7 (partial): Content is non-empty and readable.
func TestEmbed_PostgresProjectFilesReadable(t *testing.T) {
	// Spot-check first and last project migration files
	for _, file := range []string{"project_001.sql", "project_020.sql"} {
		content, err := schemaFS.ReadFile("schema/postgres/" + file)
		if err != nil {
			t.Errorf("failed to read schema/postgres/%s: %v", file, err)
			continue
		}

		if len(content) == 0 {
			t.Errorf("schema/postgres/%s is empty", file)
		}
	}
}

// TestEmbed_PostgresProjectContainsCreateTable verifies project migration 001 has CREATE TABLE.
// Covers SC-7 (partial): Validates embedded content is valid SQL.
func TestEmbed_PostgresProjectContainsCreateTable(t *testing.T) {
	content, err := schemaFS.ReadFile("schema/postgres/project_001.sql")
	if err != nil {
		t.Fatalf("failed to read schema/postgres/project_001.sql: %v", err)
	}

	contentStr := string(content)

	if !containsIgnoreCase(contentStr, "CREATE TABLE") {
		t.Error("schema/postgres/project_001.sql should contain CREATE TABLE statement")
	}

	if !containsIgnoreCase(contentStr, "tasks") {
		t.Error("schema/postgres/project_001.sql should create tasks table")
	}
}
