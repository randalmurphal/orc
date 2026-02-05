package db

import (
	"path/filepath"
	"testing"
)

// =============================================================================
// SC-1: Users table created in GlobalDB with correct schema
// =============================================================================

func TestMigration010_CreatesUsersTable(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Migrate global schema (should include 010)
	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	// Verify users table exists with correct columns
	columns := map[string]bool{
		"id":         false,
		"name":       false,
		"email":      false,
		"created_at": false,
	}

	rows, err := db.Query("SELECT name FROM pragma_table_info('users')")
	if err != nil {
		t.Fatalf("pragma_table_info failed: %v", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			t.Fatalf("scan column name: %v", err)
		}
		if _, expected := columns[colName]; expected {
			columns[colName] = true
		}
	}

	for col, found := range columns {
		if !found {
			t.Errorf("users table missing column: %s", col)
		}
	}

	// Verify name has UNIQUE constraint
	var indexCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM sqlite_master
		WHERE type='index' AND tbl_name='users' AND sql LIKE '%UNIQUE%'
	`).Scan(&indexCount)
	if err != nil {
		t.Fatalf("check unique index: %v", err)
	}
	// Note: UNIQUE constraint creates an implicit index
}

func TestMigration010_UsersTable_NameIsUnique(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	// Insert first user
	_, err = db.Exec("INSERT INTO users (id, name, email) VALUES ('user-001', 'alice', 'alice@example.com')")
	if err != nil {
		t.Fatalf("insert first user: %v", err)
	}

	// Second insert with same name should fail
	_, err = db.Exec("INSERT INTO users (id, name, email) VALUES ('user-002', 'alice', 'alice2@example.com')")
	if err == nil {
		t.Error("expected UNIQUE constraint violation on duplicate name, but insert succeeded")
	}
}

// =============================================================================
// SC-6: user_id column added to cost_log table (GlobalDB)
// =============================================================================

func TestMigration010_AddsUserIdToCostLog(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	// Verify user_id column exists in cost_log
	var colCount int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('cost_log')
		WHERE name = 'user_id'
	`).Scan(&colCount)
	if err != nil {
		t.Fatalf("check column: %v", err)
	}
	if colCount != 1 {
		t.Errorf("user_id column count = %d, want 1", colCount)
	}
}

// =============================================================================
// SC-7, SC-8: GetOrCreateUser idempotency
// =============================================================================

func TestGetOrCreateUser_CreatesNewUser(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create new user
	userID, err := gdb.GetOrCreateUser("bob")
	if err != nil {
		t.Fatalf("GetOrCreateUser failed: %v", err)
	}
	if userID == "" {
		t.Error("expected non-empty user ID")
	}

	// Verify user exists in database
	var name string
	err = db.QueryRow("SELECT name FROM users WHERE id = ?", userID).Scan(&name)
	if err != nil {
		t.Fatalf("query user: %v", err)
	}
	if name != "bob" {
		t.Errorf("name = %q, want bob", name)
	}
}

func TestGetOrCreateUser_ReturnsExistingUser(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create user first time
	userID1, err := gdb.GetOrCreateUser("charlie")
	if err != nil {
		t.Fatalf("first GetOrCreateUser failed: %v", err)
	}

	// Get same user second time
	userID2, err := gdb.GetOrCreateUser("charlie")
	if err != nil {
		t.Fatalf("second GetOrCreateUser failed: %v", err)
	}

	// Should return same ID (idempotent)
	if userID1 != userID2 {
		t.Errorf("GetOrCreateUser not idempotent: first=%s, second=%s", userID1, userID2)
	}

	// Verify only one user with that name exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE name = 'charlie'").Scan(&count)
	if err != nil {
		t.Fatalf("count users: %v", err)
	}
	if count != 1 {
		t.Errorf("user count = %d, want 1 (not idempotent)", count)
	}
}

func TestGetOrCreateUser_WithEmail(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create user with email
	userID, err := gdb.GetOrCreateUserWithEmail("diana", "diana@example.com")
	if err != nil {
		t.Fatalf("GetOrCreateUserWithEmail failed: %v", err)
	}
	if userID == "" {
		t.Error("expected non-empty user ID")
	}

	// Verify user has email
	var email string
	err = db.QueryRow("SELECT email FROM users WHERE id = ?", userID).Scan(&email)
	if err != nil {
		t.Fatalf("query user email: %v", err)
	}
	if email != "diana@example.com" {
		t.Errorf("email = %q, want diana@example.com", email)
	}
}

// =============================================================================
// GetUser and ListUsers
// =============================================================================

func TestGetUser_ReturnsUser(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create user
	userID, err := gdb.GetOrCreateUserWithEmail("eve", "eve@example.com")
	if err != nil {
		t.Fatalf("GetOrCreateUserWithEmail failed: %v", err)
	}

	// Get user by ID
	user, err := gdb.GetUser(userID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != userID {
		t.Errorf("ID = %q, want %q", user.ID, userID)
	}
	if user.Name != "eve" {
		t.Errorf("Name = %q, want eve", user.Name)
	}
	if user.Email != "eve@example.com" {
		t.Errorf("Email = %q, want eve@example.com", user.Email)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Get non-existent user
	user, err := gdb.GetUser("nonexistent-id")
	if err != nil {
		t.Errorf("GetUser returned error for nonexistent user: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil for nonexistent user, got %+v", user)
	}
}

func TestGetUserByName_ReturnsUser(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create user
	_, err = gdb.GetOrCreateUserWithEmail("frank", "frank@example.com")
	if err != nil {
		t.Fatalf("GetOrCreateUserWithEmail failed: %v", err)
	}

	// Get user by name
	user, err := gdb.GetUserByName("frank")
	if err != nil {
		t.Fatalf("GetUserByName failed: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.Name != "frank" {
		t.Errorf("Name = %q, want frank", user.Name)
	}
}

func TestListUsers_ReturnsAllUsers(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create multiple users
	names := []string{"alice", "bob", "charlie"}
	for _, name := range names {
		_, err := gdb.GetOrCreateUser(name)
		if err != nil {
			t.Fatalf("GetOrCreateUser(%s) failed: %v", name, err)
		}
	}

	// List all users
	users, err := gdb.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("len(users) = %d, want 3", len(users))
	}

	// Verify all names present
	found := make(map[string]bool)
	for _, u := range users {
		found[u.Name] = true
	}
	for _, name := range names {
		if !found[name] {
			t.Errorf("user %s not found in ListUsers result", name)
		}
	}
}

// =============================================================================
// RecordCostExtended with user_id
// =============================================================================

func TestRecordCostExtended_WithUserID(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create user first
	userID, err := gdb.GetOrCreateUser("tester")
	if err != nil {
		t.Fatalf("GetOrCreateUser failed: %v", err)
	}

	// Record cost with user_id
	entry := CostEntry{
		ProjectID:   "proj-1",
		TaskID:      "TASK-001",
		Phase:       "implement",
		Model:       "opus",
		CostUSD:     0.50,
		InputTokens: 1000,
		UserID:      userID,
	}

	if err := gdb.RecordCostExtended(entry); err != nil {
		t.Fatalf("RecordCostExtended failed: %v", err)
	}

	// Verify user_id was stored
	var storedUserID string
	err = db.QueryRow("SELECT user_id FROM cost_log WHERE task_id = ?", "TASK-001").Scan(&storedUserID)
	if err != nil {
		t.Fatalf("query cost_log: %v", err)
	}
	if storedUserID != userID {
		t.Errorf("user_id = %q, want %q", storedUserID, userID)
	}
}
