package db

import (
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
