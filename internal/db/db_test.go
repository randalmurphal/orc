package db

import (
	"path/filepath"
	"testing"
	"time"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if db.Path() != dbPath {
		t.Errorf("Path() = %q, want %q", db.Path(), dbPath)
	}

	// Verify pragmas are set
	var journalMode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want wal", journalMode)
	}
}

func TestOpen_CreatesParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "subdir", "nested", "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	db.Close()
}

func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Migrate global schema
	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate global failed: %v", err)
	}

	// Verify tables exist
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM projects").Scan(&count); err != nil {
		t.Errorf("projects table not created: %v", err)
	}

	// Run again - should be idempotent
	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Second Migrate failed: %v", err)
	}
}

func TestMigrate_Project(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate project failed: %v", err)
	}

	// Verify all tables exist
	tables := []string{"detection", "tasks", "phases", "transcripts", "transcripts_fts"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("table %s not created: %v", table, err)
		}
	}
}

func TestGlobalDB_Projects(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Create project
	p := Project{
		ID:        "proj-001",
		Name:      "Test Project",
		Path:      "/home/user/test",
		Language:  "go",
		CreatedAt: time.Now(),
	}

	if err := gdb.SyncProject(p); err != nil {
		t.Fatalf("SyncProject failed: %v", err)
	}

	// Get by ID
	got, err := gdb.GetProject("proj-001")
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if got.Name != p.Name {
		t.Errorf("Name = %q, want %q", got.Name, p.Name)
	}
	if got.Path != p.Path {
		t.Errorf("Path = %q, want %q", got.Path, p.Path)
	}

	// Get by path
	got2, err := gdb.GetProjectByPath("/home/user/test")
	if err != nil {
		t.Fatalf("GetProjectByPath failed: %v", err)
	}
	if got2.ID != p.ID {
		t.Errorf("ID = %q, want %q", got2.ID, p.ID)
	}

	// List
	projects, err := gdb.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects failed: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("len(projects) = %d, want 1", len(projects))
	}

	// Update
	p.Name = "Updated Name"
	if err := gdb.SyncProject(p); err != nil {
		t.Fatalf("SyncProject update failed: %v", err)
	}

	got3, _ := gdb.GetProject("proj-001")
	if got3.Name != "Updated Name" {
		t.Errorf("Name after update = %q, want %q", got3.Name, "Updated Name")
	}

	// Delete
	if err := gdb.DeleteProject("proj-001"); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}

	projects, _ = gdb.ListProjects()
	if len(projects) != 0 {
		t.Errorf("len(projects) after delete = %d, want 0", len(projects))
	}
}

func TestGlobalDB_CostTracking(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "global.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("global"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	gdb := &GlobalDB{DB: db}

	// Record some costs
	if err := gdb.RecordCost("proj-1", "TASK-001", "implement", 0.05, 1000, 500); err != nil {
		t.Fatalf("RecordCost failed: %v", err)
	}
	if err := gdb.RecordCost("proj-1", "TASK-001", "test", 0.03, 600, 300); err != nil {
		t.Fatalf("RecordCost failed: %v", err)
	}
	if err := gdb.RecordCost("proj-2", "TASK-002", "implement", 0.10, 2000, 1000); err != nil {
		t.Fatalf("RecordCost failed: %v", err)
	}

	// Get summary (all projects)
	since := time.Now().Add(-1 * time.Hour)
	summary, err := gdb.GetCostSummary("", since)
	if err != nil {
		t.Fatalf("GetCostSummary failed: %v", err)
	}

	if summary.TotalCostUSD != 0.18 {
		t.Errorf("TotalCostUSD = %f, want 0.18", summary.TotalCostUSD)
	}
	if summary.EntryCount != 3 {
		t.Errorf("EntryCount = %d, want 3", summary.EntryCount)
	}

	// Get summary (specific project)
	summary2, err := gdb.GetCostSummary("proj-1", since)
	if err != nil {
		t.Fatalf("GetCostSummary proj-1 failed: %v", err)
	}

	if summary2.TotalCostUSD != 0.08 {
		t.Errorf("TotalCostUSD for proj-1 = %f, want 0.08", summary2.TotalCostUSD)
	}
}

func TestProjectDB_Detection(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Store detection
	d := &Detection{
		Language:    "go",
		Frameworks:  []string{"cobra", "viper"},
		BuildTools:  []string{"go"},
		HasTests:    true,
		TestCommand: "go test ./...",
		LintCommand: "golangci-lint run",
	}

	if err := pdb.StoreDetection(d); err != nil {
		t.Fatalf("StoreDetection failed: %v", err)
	}

	// Load detection
	got, err := pdb.LoadDetection()
	if err != nil {
		t.Fatalf("LoadDetection failed: %v", err)
	}

	if got.Language != d.Language {
		t.Errorf("Language = %q, want %q", got.Language, d.Language)
	}
	if len(got.Frameworks) != 2 {
		t.Errorf("len(Frameworks) = %d, want 2", len(got.Frameworks))
	}
	if !got.HasTests {
		t.Error("HasTests = false, want true")
	}
}

func TestProjectDB_Tasks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	now := time.Now()
	task := &Task{
		ID:          "TASK-001",
		Title:       "Fix login bug",
		Description: "Users can't login with special characters",
		Weight:      "small",
		Status:      "created",
		CreatedAt:   now,
	}

	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}

	// Get task
	got, err := pdb.GetTask("TASK-001")
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}
	if got.Title != task.Title {
		t.Errorf("Title = %q, want %q", got.Title, task.Title)
	}

	// List tasks
	tasks, total, err := pdb.ListTasks(ListOpts{})
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(tasks) != 1 {
		t.Errorf("len(tasks) = %d, want 1", len(tasks))
	}

	// Update task
	startedAt := time.Now()
	task.Status = "running"
	task.StartedAt = &startedAt
	if err := pdb.SaveTask(task); err != nil {
		t.Fatalf("SaveTask update failed: %v", err)
	}

	got2, _ := pdb.GetTask("TASK-001")
	if got2.Status != "running" {
		t.Errorf("Status = %q, want running", got2.Status)
	}
	if got2.StartedAt == nil {
		t.Error("StartedAt is nil, want non-nil")
	}

	// Add more tasks and test filtering
	task2 := &Task{ID: "TASK-002", Title: "Task 2", Status: "completed", CreatedAt: now}
	task3 := &Task{ID: "TASK-003", Title: "Task 3", Status: "running", CreatedAt: now}
	pdb.SaveTask(task2)
	pdb.SaveTask(task3)

	// Filter by status
	running, _, _ := pdb.ListTasks(ListOpts{Status: "running"})
	if len(running) != 2 {
		t.Errorf("running tasks = %d, want 2", len(running))
	}

	// Pagination
	page, _, _ := pdb.ListTasks(ListOpts{Limit: 2})
	if len(page) != 2 {
		t.Errorf("paginated tasks = %d, want 2", len(page))
	}

	// Delete
	if err := pdb.DeleteTask("TASK-001"); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	deleted, _ := pdb.GetTask("TASK-001")
	if deleted != nil {
		t.Error("task still exists after delete")
	}
}

func TestProjectDB_Phases(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task first
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	pdb.SaveTask(task)

	// Save phases
	now := time.Now()
	phases := []Phase{
		{TaskID: "TASK-001", PhaseID: "implement", Status: "completed", Iterations: 1, StartedAt: &now, CompletedAt: &now},
		{TaskID: "TASK-001", PhaseID: "test", Status: "running", Iterations: 2, StartedAt: &now},
	}

	for _, ph := range phases {
		if err := pdb.SavePhase(&ph); err != nil {
			t.Fatalf("SavePhase failed: %v", err)
		}
	}

	// Get phases
	got, err := pdb.GetPhases("TASK-001")
	if err != nil {
		t.Fatalf("GetPhases failed: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(phases) = %d, want 2", len(got))
	}

	// Update phase
	phases[1].Status = "completed"
	phases[1].CompletedAt = &now
	if err := pdb.SavePhase(&phases[1]); err != nil {
		t.Fatalf("SavePhase update failed: %v", err)
	}

	got2, _ := pdb.GetPhases("TASK-001")
	for _, ph := range got2 {
		if ph.PhaseID == "test" && ph.Status != "completed" {
			t.Error("test phase not updated")
		}
	}
}

func TestProjectDB_Transcripts(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	pdb.SaveTask(task)

	// Add transcripts
	transcripts := []Transcript{
		{TaskID: "TASK-001", Phase: "implement", Iteration: 1, Role: "user", Content: "Fix the authentication bug"},
		{TaskID: "TASK-001", Phase: "implement", Iteration: 1, Role: "assistant", Content: "I'll fix the authentication module"},
		{TaskID: "TASK-001", Phase: "test", Iteration: 1, Role: "user", Content: "Run the test suite"},
	}

	for i := range transcripts {
		if err := pdb.AddTranscript(&transcripts[i]); err != nil {
			t.Fatalf("AddTranscript failed: %v", err)
		}
		if transcripts[i].ID == 0 {
			t.Error("transcript ID not set")
		}
	}

	// Get transcripts
	got, err := pdb.GetTranscripts("TASK-001")
	if err != nil {
		t.Fatalf("GetTranscripts failed: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len(transcripts) = %d, want 3", len(got))
	}
}

func TestProjectDB_TranscriptSearch(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task and transcripts
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	pdb.SaveTask(task)

	transcripts := []Transcript{
		{TaskID: "TASK-001", Phase: "implement", Iteration: 1, Role: "assistant", Content: "Fixed the authentication bug in login handler"},
		{TaskID: "TASK-001", Phase: "test", Iteration: 1, Role: "assistant", Content: "All unit tests are passing now"},
		{TaskID: "TASK-001", Phase: "implement", Iteration: 1, Role: "assistant", Content: "Updated the database schema"},
	}

	for i := range transcripts {
		pdb.AddTranscript(&transcripts[i])
	}

	// Search for "authentication"
	matches, err := pdb.SearchTranscripts("authentication")
	if err != nil {
		t.Fatalf("SearchTranscripts failed: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("len(matches) for 'authentication' = %d, want 1", len(matches))
	}

	// Search for "test"
	matches2, err := pdb.SearchTranscripts("tests")
	if err != nil {
		t.Fatalf("SearchTranscripts failed: %v", err)
	}
	if len(matches2) != 1 {
		t.Errorf("len(matches) for 'tests' = %d, want 1", len(matches2))
	}
}

func TestProjectDB_CascadeDelete(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create task with phases and transcripts
	task := &Task{ID: "TASK-001", Title: "Test", Status: "running", CreatedAt: time.Now()}
	pdb.SaveTask(task)

	now := time.Now()
	pdb.SavePhase(&Phase{TaskID: "TASK-001", PhaseID: "implement", Status: "completed", StartedAt: &now})
	pdb.AddTranscript(&Transcript{TaskID: "TASK-001", Phase: "implement", Content: "Test content"})

	// Delete task - should cascade
	if err := pdb.DeleteTask("TASK-001"); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	// Verify phases deleted
	phases, _ := pdb.GetPhases("TASK-001")
	if len(phases) != 0 {
		t.Error("phases not deleted on cascade")
	}

	// Verify transcripts deleted
	transcripts, _ := pdb.GetTranscripts("TASK-001")
	if len(transcripts) != 0 {
		t.Error("transcripts not deleted on cascade")
	}
}

func TestProjectDB_Initiatives(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create initiative
	init := &Initiative{
		ID:               "INIT-001",
		Title:            "User Authentication",
		Status:           "draft",
		OwnerInitials:    "RM",
		OwnerDisplayName: "Randy",
		Vision:           "Secure authentication using JWT tokens",
	}

	if err := pdb.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative failed: %v", err)
	}

	// Get initiative
	got, err := pdb.GetInitiative("INIT-001")
	if err != nil {
		t.Fatalf("GetInitiative failed: %v", err)
	}
	if got.Title != init.Title {
		t.Errorf("Title = %q, want %q", got.Title, init.Title)
	}
	if got.OwnerInitials != init.OwnerInitials {
		t.Errorf("OwnerInitials = %q, want %q", got.OwnerInitials, init.OwnerInitials)
	}
	if got.Vision != init.Vision {
		t.Errorf("Vision = %q, want %q", got.Vision, init.Vision)
	}

	// Update initiative
	init.Status = "active"
	if err := pdb.SaveInitiative(init); err != nil {
		t.Fatalf("SaveInitiative update failed: %v", err)
	}

	got2, _ := pdb.GetInitiative("INIT-001")
	if got2.Status != "active" {
		t.Errorf("Status = %q, want active", got2.Status)
	}

	// List initiatives
	init2 := &Initiative{ID: "INIT-002", Title: "API Refactor", Status: "draft"}
	pdb.SaveInitiative(init2)

	initiatives, err := pdb.ListInitiatives(ListOpts{})
	if err != nil {
		t.Fatalf("ListInitiatives failed: %v", err)
	}
	if len(initiatives) != 2 {
		t.Errorf("len(initiatives) = %d, want 2", len(initiatives))
	}

	// Filter by status
	activeInits, _ := pdb.ListInitiatives(ListOpts{Status: "active"})
	if len(activeInits) != 1 {
		t.Errorf("active initiatives = %d, want 1", len(activeInits))
	}

	// Delete initiative
	if err := pdb.DeleteInitiative("INIT-002"); err != nil {
		t.Fatalf("DeleteInitiative failed: %v", err)
	}

	deleted, _ := pdb.GetInitiative("INIT-002")
	if deleted != nil {
		t.Error("initiative still exists after delete")
	}
}

func TestProjectDB_InitiativeDecisions(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create initiative first
	init := &Initiative{ID: "INIT-001", Title: "Test", Status: "draft"}
	pdb.SaveInitiative(init)

	// Add decisions
	dec1 := &InitiativeDecision{
		ID:           "DEC-001",
		InitiativeID: "INIT-001",
		Decision:     "Use JWT tokens for authentication",
		Rationale:    "Industry standard, stateless",
		DecidedBy:    "RM",
		DecidedAt:    time.Now(),
	}
	if err := pdb.AddInitiativeDecision(dec1); err != nil {
		t.Fatalf("AddInitiativeDecision failed: %v", err)
	}

	dec2 := &InitiativeDecision{
		ID:           "DEC-002",
		InitiativeID: "INIT-001",
		Decision:     "7-day token expiry",
		Rationale:    "Security best practice",
		DecidedBy:    "RM",
		DecidedAt:    time.Now(),
	}
	pdb.AddInitiativeDecision(dec2)

	// Get decisions
	decisions, err := pdb.GetInitiativeDecisions("INIT-001")
	if err != nil {
		t.Fatalf("GetInitiativeDecisions failed: %v", err)
	}
	if len(decisions) != 2 {
		t.Errorf("len(decisions) = %d, want 2", len(decisions))
	}
	if decisions[0].Decision != dec1.Decision {
		t.Errorf("Decision = %q, want %q", decisions[0].Decision, dec1.Decision)
	}
}

func TestProjectDB_InitiativeTasks(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create initiative
	init := &Initiative{ID: "INIT-001", Title: "Test", Status: "draft"}
	pdb.SaveInitiative(init)

	// Create tasks
	pdb.SaveTask(&Task{ID: "TASK-001", Title: "Task 1", Status: "pending", CreatedAt: time.Now()})
	pdb.SaveTask(&Task{ID: "TASK-002", Title: "Task 2", Status: "pending", CreatedAt: time.Now()})
	pdb.SaveTask(&Task{ID: "TASK-003", Title: "Task 3", Status: "pending", CreatedAt: time.Now()})

	// Link tasks to initiative
	if err := pdb.AddTaskToInitiative("INIT-001", "TASK-001", 1); err != nil {
		t.Fatalf("AddTaskToInitiative failed: %v", err)
	}
	pdb.AddTaskToInitiative("INIT-001", "TASK-003", 2)
	pdb.AddTaskToInitiative("INIT-001", "TASK-002", 3)

	// Get tasks in order
	taskIDs, err := pdb.GetInitiativeTasks("INIT-001")
	if err != nil {
		t.Fatalf("GetInitiativeTasks failed: %v", err)
	}
	if len(taskIDs) != 3 {
		t.Errorf("len(taskIDs) = %d, want 3", len(taskIDs))
	}
	if taskIDs[0] != "TASK-001" || taskIDs[1] != "TASK-003" || taskIDs[2] != "TASK-002" {
		t.Errorf("taskIDs = %v, want [TASK-001, TASK-003, TASK-002]", taskIDs)
	}

	// Update sequence
	pdb.AddTaskToInitiative("INIT-001", "TASK-002", 0) // Move to first
	taskIDs2, _ := pdb.GetInitiativeTasks("INIT-001")
	if taskIDs2[0] != "TASK-002" {
		t.Errorf("first task after reorder = %s, want TASK-002", taskIDs2[0])
	}

	// Remove task from initiative
	if err := pdb.RemoveTaskFromInitiative("INIT-001", "TASK-003"); err != nil {
		t.Fatalf("RemoveTaskFromInitiative failed: %v", err)
	}
	taskIDs3, _ := pdb.GetInitiativeTasks("INIT-001")
	if len(taskIDs3) != 2 {
		t.Errorf("len(taskIDs) after remove = %d, want 2", len(taskIDs3))
	}
}

func TestProjectDB_TaskDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create tasks
	pdb.SaveTask(&Task{ID: "TASK-001", Title: "Task 1", Status: "pending", CreatedAt: time.Now()})
	pdb.SaveTask(&Task{ID: "TASK-002", Title: "Task 2", Status: "pending", CreatedAt: time.Now()})
	pdb.SaveTask(&Task{ID: "TASK-003", Title: "Task 3", Status: "pending", CreatedAt: time.Now()})

	// Add dependencies: TASK-002 depends on TASK-001, TASK-003 depends on TASK-001 and TASK-002
	if err := pdb.AddTaskDependency("TASK-002", "TASK-001"); err != nil {
		t.Fatalf("AddTaskDependency failed: %v", err)
	}
	pdb.AddTaskDependency("TASK-003", "TASK-001")
	pdb.AddTaskDependency("TASK-003", "TASK-002")

	// Get dependencies for TASK-003
	deps, err := pdb.GetTaskDependencies("TASK-003")
	if err != nil {
		t.Fatalf("GetTaskDependencies failed: %v", err)
	}
	if len(deps) != 2 {
		t.Errorf("len(deps) for TASK-003 = %d, want 2", len(deps))
	}

	// Get dependents for TASK-001
	dependents, err := pdb.GetTaskDependents("TASK-001")
	if err != nil {
		t.Fatalf("GetTaskDependents failed: %v", err)
	}
	if len(dependents) != 2 {
		t.Errorf("len(dependents) for TASK-001 = %d, want 2", len(dependents))
	}

	// Remove a dependency
	if err := pdb.RemoveTaskDependency("TASK-003", "TASK-002"); err != nil {
		t.Fatalf("RemoveTaskDependency failed: %v", err)
	}
	deps2, _ := pdb.GetTaskDependencies("TASK-003")
	if len(deps2) != 1 {
		t.Errorf("len(deps) after remove = %d, want 1", len(deps2))
	}

	// Clear all dependencies
	if err := pdb.ClearTaskDependencies("TASK-003"); err != nil {
		t.Fatalf("ClearTaskDependencies failed: %v", err)
	}
	deps3, _ := pdb.GetTaskDependencies("TASK-003")
	if len(deps3) != 0 {
		t.Errorf("len(deps) after clear = %d, want 0", len(deps3))
	}

	// Test duplicate dependency is ignored
	pdb.AddTaskDependency("TASK-002", "TASK-001") // Already exists
	deps4, _ := pdb.GetTaskDependencies("TASK-002")
	if len(deps4) != 1 {
		t.Errorf("len(deps) after duplicate = %d, want 1", len(deps4))
	}
}

func TestProjectDB_InitiativeDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, ".orc", "orc.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	if err := db.Migrate("project"); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	pdb := &ProjectDB{DB: db}

	// Create initiatives
	pdb.SaveInitiative(&Initiative{ID: "INIT-001", Title: "Build System", Status: "draft"})
	pdb.SaveInitiative(&Initiative{ID: "INIT-002", Title: "React Migration", Status: "draft"})
	pdb.SaveInitiative(&Initiative{ID: "INIT-003", Title: "Component Library", Status: "draft"})

	// Add dependencies: INIT-002 depends on INIT-001, INIT-003 depends on INIT-001 and INIT-002
	if err := pdb.AddInitiativeDependency("INIT-002", "INIT-001"); err != nil {
		t.Fatalf("AddInitiativeDependency failed: %v", err)
	}
	pdb.AddInitiativeDependency("INIT-003", "INIT-001")
	pdb.AddInitiativeDependency("INIT-003", "INIT-002")

	// Get dependencies for INIT-003
	deps, err := pdb.GetInitiativeDependencies("INIT-003")
	if err != nil {
		t.Fatalf("GetInitiativeDependencies failed: %v", err)
	}
	if len(deps) != 2 {
		t.Errorf("len(deps) for INIT-003 = %d, want 2", len(deps))
	}

	// Get dependents for INIT-001
	dependents, err := pdb.GetInitiativeDependents("INIT-001")
	if err != nil {
		t.Fatalf("GetInitiativeDependents failed: %v", err)
	}
	if len(dependents) != 2 {
		t.Errorf("len(dependents) for INIT-001 = %d, want 2", len(dependents))
	}

	// Remove a dependency
	if err := pdb.RemoveInitiativeDependency("INIT-003", "INIT-002"); err != nil {
		t.Fatalf("RemoveInitiativeDependency failed: %v", err)
	}
	deps2, _ := pdb.GetInitiativeDependencies("INIT-003")
	if len(deps2) != 1 {
		t.Errorf("len(deps) after remove = %d, want 1", len(deps2))
	}

	// Clear all dependencies
	if err := pdb.ClearInitiativeDependencies("INIT-003"); err != nil {
		t.Fatalf("ClearInitiativeDependencies failed: %v", err)
	}
	deps3, _ := pdb.GetInitiativeDependencies("INIT-003")
	if len(deps3) != 0 {
		t.Errorf("len(deps) after clear = %d, want 0", len(deps3))
	}

	// Test duplicate dependency is ignored
	pdb.AddInitiativeDependency("INIT-002", "INIT-001") // Already exists
	deps4, _ := pdb.GetInitiativeDependencies("INIT-002")
	if len(deps4) != 1 {
		t.Errorf("len(deps) after duplicate = %d, want 1", len(deps4))
	}
}
