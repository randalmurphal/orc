package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestProjectDB_TaskComments(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task first
	task := &Task{ID: "TASK-001", Title: "Test Task", Status: "running", CreatedAt: time.Now()}
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Create comment
	comment := &TaskComment{
		TaskID:     "TASK-001",
		Author:     "testuser",
		AuthorType: AuthorTypeHuman,
		Content:    "This is a test comment",
		Phase:      "implement",
	}

	if err := pdb.CreateTaskComment(comment); err != nil {
		t.Fatalf("CreateTaskComment failed: %v", err)
	}

	if comment.ID == "" {
		t.Error("comment ID not set")
	}
	if comment.CreatedAt.IsZero() {
		t.Error("comment CreatedAt not set")
	}

	// Get comment
	got, err := pdb.GetTaskComment(comment.ID)
	if err != nil {
		t.Fatalf("GetTaskComment failed: %v", err)
	}
	if got == nil {
		t.Fatal("GetTaskComment returned nil")
	}
	if got.Author != comment.Author {
		t.Errorf("Author = %q, want %q", got.Author, comment.Author)
	}
	if got.AuthorType != AuthorTypeHuman {
		t.Errorf("AuthorType = %q, want %q", got.AuthorType, AuthorTypeHuman)
	}
	if got.Content != comment.Content {
		t.Errorf("Content = %q, want %q", got.Content, comment.Content)
	}
	if got.Phase != comment.Phase {
		t.Errorf("Phase = %q, want %q", got.Phase, comment.Phase)
	}

	// List comments
	comments, err := pdb.ListTaskComments("TASK-001")
	if err != nil {
		t.Fatalf("ListTaskComments failed: %v", err)
	}
	if len(comments) != 1 {
		t.Errorf("len(comments) = %d, want 1", len(comments))
	}
}

func TestProjectDB_TaskComments_AuthorTypes(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	_ = pdb.SaveTask(task)

	// Add comments with different author types
	comments := []TaskComment{
		{TaskID: "TASK-001", Author: "user1", AuthorType: AuthorTypeHuman, Content: "Human comment"},
		{TaskID: "TASK-001", Author: "claude", AuthorType: AuthorTypeAgent, Content: "Agent note"},
		{TaskID: "TASK-001", Author: "orc", AuthorType: AuthorTypeSystem, Content: "System event"},
	}

	for i := range comments {
		if err := pdb.CreateTaskComment(&comments[i]); err != nil {
			t.Fatalf("CreateTaskComment failed: %v", err)
		}
	}

	// List by author type
	humanComments, err := pdb.ListTaskCommentsByAuthorType("TASK-001", AuthorTypeHuman)
	if err != nil {
		t.Fatalf("ListTaskCommentsByAuthorType human failed: %v", err)
	}
	if len(humanComments) != 1 {
		t.Errorf("human comments = %d, want 1", len(humanComments))
	}

	agentComments, err := pdb.ListTaskCommentsByAuthorType("TASK-001", AuthorTypeAgent)
	if err != nil {
		t.Fatalf("ListTaskCommentsByAuthorType agent failed: %v", err)
	}
	if len(agentComments) != 1 {
		t.Errorf("agent comments = %d, want 1", len(agentComments))
	}

	systemComments, err := pdb.ListTaskCommentsByAuthorType("TASK-001", AuthorTypeSystem)
	if err != nil {
		t.Fatalf("ListTaskCommentsByAuthorType system failed: %v", err)
	}
	if len(systemComments) != 1 {
		t.Errorf("system comments = %d, want 1", len(systemComments))
	}
}

func TestProjectDB_TaskComments_ByPhase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	_ = pdb.SaveTask(task)

	// Add comments for different phases
	comments := []TaskComment{
		{TaskID: "TASK-001", AuthorType: AuthorTypeHuman, Content: "Implement note 1", Phase: "implement"},
		{TaskID: "TASK-001", AuthorType: AuthorTypeHuman, Content: "Implement note 2", Phase: "implement"},
		{TaskID: "TASK-001", AuthorType: AuthorTypeHuman, Content: "Test note", Phase: "test"},
		{TaskID: "TASK-001", AuthorType: AuthorTypeHuman, Content: "General note", Phase: ""},
	}

	for i := range comments {
		_ = pdb.CreateTaskComment(&comments[i])
	}

	// List by phase
	implComments, err := pdb.ListTaskCommentsByPhase("TASK-001", "implement")
	if err != nil {
		t.Fatalf("ListTaskCommentsByPhase implement failed: %v", err)
	}
	if len(implComments) != 2 {
		t.Errorf("implement comments = %d, want 2", len(implComments))
	}

	testComments, err := pdb.ListTaskCommentsByPhase("TASK-001", "test")
	if err != nil {
		t.Fatalf("ListTaskCommentsByPhase test failed: %v", err)
	}
	if len(testComments) != 1 {
		t.Errorf("test comments = %d, want 1", len(testComments))
	}
}

func TestProjectDB_TaskComments_Update(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	_ = pdb.SaveTask(task)

	// Create comment
	comment := &TaskComment{
		TaskID:     "TASK-001",
		AuthorType: AuthorTypeHuman,
		Content:    "Original content",
	}
	_ = pdb.CreateTaskComment(comment)

	// Update comment
	comment.Content = "Updated content"
	comment.Phase = "test"
	if err := pdb.UpdateTaskComment(comment); err != nil {
		t.Fatalf("UpdateTaskComment failed: %v", err)
	}

	// Verify update
	got, _ := pdb.GetTaskComment(comment.ID)
	if got.Content != "Updated content" {
		t.Errorf("Content = %q, want %q", got.Content, "Updated content")
	}
	if got.Phase != "test" {
		t.Errorf("Phase = %q, want %q", got.Phase, "test")
	}
}

func TestProjectDB_TaskComments_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	_ = pdb.SaveTask(task)

	// Create comment
	comment := &TaskComment{
		TaskID:     "TASK-001",
		AuthorType: AuthorTypeHuman,
		Content:    "To be deleted",
	}
	_ = pdb.CreateTaskComment(comment)

	// Delete comment
	if err := pdb.DeleteTaskComment(comment.ID); err != nil {
		t.Fatalf("DeleteTaskComment failed: %v", err)
	}

	// Verify deleted
	got, _ := pdb.GetTaskComment(comment.ID)
	if got != nil {
		t.Error("comment still exists after delete")
	}
}

func TestProjectDB_TaskComments_DeleteAll(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	_ = pdb.SaveTask(task)

	// Create multiple comments
	for i := 0; i < 5; i++ {
		comment := &TaskComment{
			TaskID:     "TASK-001",
			AuthorType: AuthorTypeHuman,
			Content:    "Comment",
		}
		_ = pdb.CreateTaskComment(comment)
	}

	// Delete all
	if err := pdb.DeleteAllTaskComments("TASK-001"); err != nil {
		t.Fatalf("DeleteAllTaskComments failed: %v", err)
	}

	// Verify deleted
	comments, _ := pdb.ListTaskComments("TASK-001")
	if len(comments) != 0 {
		t.Errorf("len(comments) after delete all = %d, want 0", len(comments))
	}
}

func TestProjectDB_TaskComments_Count(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	_ = pdb.SaveTask(task)

	// Initial count
	count, err := pdb.CountTaskComments("TASK-001")
	if err != nil {
		t.Fatalf("CountTaskComments failed: %v", err)
	}
	if count != 0 {
		t.Errorf("initial count = %d, want 0", count)
	}

	// Create comments
	for i := 0; i < 3; i++ {
		comment := &TaskComment{
			TaskID:     "TASK-001",
			AuthorType: AuthorTypeHuman,
			Content:    "Comment",
		}
		_ = pdb.CreateTaskComment(comment)
	}

	// Count again
	count2, _ := pdb.CountTaskComments("TASK-001")
	if count2 != 3 {
		t.Errorf("count after adding = %d, want 3", count2)
	}
}

func TestProjectDB_TaskComments_CascadeDelete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	_ = pdb.SaveTask(task)

	// Create comments
	for i := 0; i < 3; i++ {
		comment := &TaskComment{
			TaskID:     "TASK-001",
			AuthorType: AuthorTypeHuman,
			Content:    "Comment",
		}
		_ = pdb.CreateTaskComment(comment)
	}

	// Delete task - comments should cascade
	if err := pdb.DeleteTask("TASK-001"); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Verify comments deleted
	comments, _ := pdb.ListTaskComments("TASK-001")
	if len(comments) != 0 {
		t.Errorf("comments not deleted on cascade: %d remain", len(comments))
	}
}

func TestProjectDB_TaskComments_DefaultAuthor(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	_ = pdb.SaveTask(task)

	// Create comment without author - should default
	comment := &TaskComment{
		TaskID:  "TASK-001",
		Content: "Comment without author",
	}
	_ = pdb.CreateTaskComment(comment)

	// Verify defaults
	got, _ := pdb.GetTaskComment(comment.ID)
	if got.AuthorType != AuthorTypeHuman {
		t.Errorf("AuthorType = %q, want %q", got.AuthorType, AuthorTypeHuman)
	}
	if got.Author != "anonymous" {
		t.Errorf("Author = %q, want %q", got.Author, "anonymous")
	}
}
