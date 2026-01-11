package e2e

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/config"
	"github.com/randalmurphal/orc/internal/git"
	"github.com/randalmurphal/orc/internal/lock"
	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/tests/testutil"
)

// TestP2PWorkflowTwoUsers simulates two users working on separate tasks
// in a P2P workflow.
func TestP2PWorkflowTwoUsers(t *testing.T) {
	// Setup shared repository
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	// Create user configs for Alice and Bob
	aliceHome := testutil.MockUserConfig(t, "AM")
	bobHome := testutil.MockUserConfig(t, "BJ")

	// Sequence store location
	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")

	// === Alice creates a task ===
	t.Run("Alice creates task", func(t *testing.T) {
		store := task.NewSequenceStore(seqPath)
		gen := task.NewTaskIDGenerator(task.ModeP2P, "AM",
			task.WithSequenceStore(store),
			task.WithTasksDir(filepath.Join(repo.OrcDir, "tasks")),
		)

		taskID, err := gen.Next()
		if err != nil {
			t.Fatalf("Alice generate task ID: %v", err)
		}

		if taskID != "TASK-AM-001" {
			t.Errorf("Alice task ID = %q, want TASK-AM-001", taskID)
		}

		// Create task directory
		taskDir := filepath.Join(repo.OrcDir, "tasks", taskID)
		if err := os.MkdirAll(taskDir, 0755); err != nil {
			t.Fatalf("create Alice task dir: %v", err)
		}

		// Create task.yaml
		taskData := map[string]any{
			"id":       taskID,
			"title":    "Alice's feature",
			"weight":   "small",
			"status":   "pending",
			"executor": "AM",
		}
		testutil.WriteYAML(t, filepath.Join(taskDir, "task.yaml"), taskData)

		// Verify task exists
		testutil.AssertFileExists(t, filepath.Join(taskDir, "task.yaml"))
	})

	// === Bob creates a task ===
	t.Run("Bob creates task", func(t *testing.T) {
		store := task.NewSequenceStore(seqPath)
		gen := task.NewTaskIDGenerator(task.ModeP2P, "BJ",
			task.WithSequenceStore(store),
			task.WithTasksDir(filepath.Join(repo.OrcDir, "tasks")),
		)

		taskID, err := gen.Next()
		if err != nil {
			t.Fatalf("Bob generate task ID: %v", err)
		}

		if taskID != "TASK-BJ-001" {
			t.Errorf("Bob task ID = %q, want TASK-BJ-001", taskID)
		}

		// Create task directory
		taskDir := filepath.Join(repo.OrcDir, "tasks", taskID)
		if err := os.MkdirAll(taskDir, 0755); err != nil {
			t.Fatalf("create Bob task dir: %v", err)
		}

		// Create task.yaml
		taskData := map[string]any{
			"id":       taskID,
			"title":    "Bob's bugfix",
			"weight":   "trivial",
			"status":   "pending",
			"executor": "BJ",
		}
		testutil.WriteYAML(t, filepath.Join(taskDir, "task.yaml"), taskData)

		// Verify task exists
		testutil.AssertFileExists(t, filepath.Join(taskDir, "task.yaml"))
	})

	// === Both users work on the same task ID ===
	t.Run("Both users work on same task", func(t *testing.T) {
		taskID := "TASK-001"

		// Alice's branch and worktree
		aliceBranch := git.BranchName(taskID, "am")
		aliceWorktree := git.WorktreePath(
			filepath.Join(repo.OrcDir, "worktrees"),
			taskID,
			"am",
		)

		// Bob's branch and worktree
		bobBranch := git.BranchName(taskID, "bj")
		bobWorktree := git.WorktreePath(
			filepath.Join(repo.OrcDir, "worktrees"),
			taskID,
			"bj",
		)

		// Verify branches are different
		if aliceBranch == bobBranch {
			t.Errorf("Alice and Bob should have different branches: %s vs %s", aliceBranch, bobBranch)
		}
		if aliceBranch != "orc/TASK-001-am" {
			t.Errorf("Alice branch = %q, want orc/TASK-001-am", aliceBranch)
		}
		if bobBranch != "orc/TASK-001-bj" {
			t.Errorf("Bob branch = %q, want orc/TASK-001-bj", bobBranch)
		}

		// Verify worktrees are different
		if aliceWorktree == bobWorktree {
			t.Errorf("Alice and Bob should have different worktrees: %s vs %s", aliceWorktree, bobWorktree)
		}

		// Create worktrees
		if err := os.MkdirAll(aliceWorktree, 0755); err != nil {
			t.Fatalf("create Alice worktree: %v", err)
		}
		if err := os.MkdirAll(bobWorktree, 0755); err != nil {
			t.Fatalf("create Bob worktree: %v", err)
		}

		// Both can acquire PID guards (no conflict)
		aliceGuard := lock.NewPIDGuard(aliceWorktree)
		bobGuard := lock.NewPIDGuard(bobWorktree)

		if err := aliceGuard.Acquire(); err != nil {
			t.Errorf("Alice acquire failed: %v", err)
		}
		if err := bobGuard.Acquire(); err != nil {
			t.Errorf("Bob acquire failed: %v", err)
		}

		// Verify both PID files exist
		testutil.AssertFileExists(t, filepath.Join(aliceWorktree, lock.PIDFileName))
		testutil.AssertFileExists(t, filepath.Join(bobWorktree, lock.PIDFileName))

		// Cleanup
		aliceGuard.Release()
		bobGuard.Release()
	})

	// Use configs to verify
	_ = aliceHome
	_ = bobHome
}

// TestP2PWorkflowConfigResolution verifies that P2P mode properly resolves
// config from shared and personal sources.
func TestP2PWorkflowConfigResolution(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	// Create user config with identity
	userHome := testutil.MockUserConfig(t, "AM")

	// Load config with user dir override
	loader := config.NewLoader(repo.RootDir)
	loader.SetUserDir(filepath.Join(userHome, ".orc"))

	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	// Verify P2P mode from shared config
	if tc.Config.TaskID.Mode != "p2p" {
		t.Errorf("TaskID.Mode = %q, want p2p", tc.Config.TaskID.Mode)
	}

	// Verify identity from personal config
	if tc.Config.Identity.Initials != "AM" {
		t.Errorf("Identity.Initials = %q, want AM", tc.Config.Identity.Initials)
	}

	// Verify executor prefix is set
	if tc.Config.ExecutorPrefix() != "AM" {
		t.Errorf("ExecutorPrefix() = %q, want AM", tc.Config.ExecutorPrefix())
	}
}

// TestP2PWorkflowTeamYamlUpdate verifies that team.yaml is updated correctly
// when members join.
func TestP2PWorkflowTeamYamlUpdate(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	teamPath := filepath.Join(repo.OrcDir, "shared", "team.yaml")

	// Initial state
	team := testutil.ReadYAML(t, teamPath)
	members := team["members"].([]interface{})
	if len(members) != 0 {
		t.Errorf("expected 0 initial members, got %d", len(members))
	}

	// Alice joins
	team["members"] = []interface{}{
		map[string]interface{}{
			"initials": "AM",
			"name":     "Alice Martinez",
			"email":    "alice@example.com",
		},
	}
	team["reserved_prefixes"] = []interface{}{"AM"}
	testutil.WriteYAML(t, teamPath, team)

	// Bob joins
	team = testutil.ReadYAML(t, teamPath)
	members = team["members"].([]interface{})
	members = append(members, map[string]interface{}{
		"initials": "BJ",
		"name":     "Bob Johnson",
	})
	team["members"] = members

	reserved := team["reserved_prefixes"].([]interface{})
	reserved = append(reserved, "BJ")
	team["reserved_prefixes"] = reserved
	testutil.WriteYAML(t, teamPath, team)

	// Verify final state
	finalTeam := testutil.ReadYAML(t, teamPath)
	finalMembers := finalTeam["members"].([]interface{})
	finalReserved := finalTeam["reserved_prefixes"].([]interface{})

	if len(finalMembers) != 2 {
		t.Errorf("expected 2 members, got %d", len(finalMembers))
	}
	if len(finalReserved) != 2 {
		t.Errorf("expected 2 reserved prefixes, got %d", len(finalReserved))
	}
}

// TestP2PWorkflowSequenceIsolation verifies that task sequences are isolated
// per prefix.
func TestP2PWorkflowSequenceIsolation(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")
	store := task.NewSequenceStore(seqPath)

	// Alice's sequence
	aliceGen := task.NewTaskIDGenerator(task.ModeP2P, "AM", task.WithSequenceStore(store))

	aliceID1, _ := aliceGen.Next()
	aliceID2, _ := aliceGen.Next()
	aliceID3, _ := aliceGen.Next()

	if aliceID1 != "TASK-AM-001" {
		t.Errorf("Alice ID 1 = %q, want TASK-AM-001", aliceID1)
	}
	if aliceID2 != "TASK-AM-002" {
		t.Errorf("Alice ID 2 = %q, want TASK-AM-002", aliceID2)
	}
	if aliceID3 != "TASK-AM-003" {
		t.Errorf("Alice ID 3 = %q, want TASK-AM-003", aliceID3)
	}

	// Bob's sequence (starts from 001, independent of Alice)
	bobGen := task.NewTaskIDGenerator(task.ModeP2P, "BJ", task.WithSequenceStore(store))

	bobID1, _ := bobGen.Next()
	bobID2, _ := bobGen.Next()

	if bobID1 != "TASK-BJ-001" {
		t.Errorf("Bob ID 1 = %q, want TASK-BJ-001", bobID1)
	}
	if bobID2 != "TASK-BJ-002" {
		t.Errorf("Bob ID 2 = %q, want TASK-BJ-002", bobID2)
	}

	// Verify sequences are stored separately
	seqData := testutil.ReadYAML(t, seqPath)
	prefixes, ok := seqData["prefixes"].(map[string]interface{})
	if !ok {
		t.Fatal("prefixes should be a map")
	}

	if prefixes["AM"] != 3 {
		t.Errorf("AM sequence = %v, want 3", prefixes["AM"])
	}
	if prefixes["BJ"] != 2 {
		t.Errorf("BJ sequence = %v, want 2", prefixes["BJ"])
	}
}

// TestP2PWorkflowBranchNaming verifies correct branch naming in P2P mode.
func TestP2PWorkflowBranchNaming(t *testing.T) {
	tests := []struct {
		taskID   string
		executor string
		want     string
	}{
		// Task created by Alice, no executor suffix needed
		{"TASK-AM-001", "", "orc/TASK-AM-001"},
		{"TASK-BJ-001", "", "orc/TASK-BJ-001"},

		// Shared task with executor suffix
		{"TASK-001", "am", "orc/TASK-001-am"},
		{"TASK-001", "bj", "orc/TASK-001-bj"},

		// Alice's task being executed by Alice
		{"TASK-AM-001", "am", "orc/TASK-AM-001-am"},

		// Alice's task being executed by Bob (review scenario)
		{"TASK-AM-001", "bj", "orc/TASK-AM-001-bj"},
	}

	for _, tt := range tests {
		t.Run(tt.taskID+"_"+tt.executor, func(t *testing.T) {
			got := git.BranchName(tt.taskID, tt.executor)
			if got != tt.want {
				t.Errorf("BranchName(%q, %q) = %q, want %q", tt.taskID, tt.executor, got, tt.want)
			}
		})
	}
}

// TestP2PWorkflowParallelExecution verifies that multiple users can execute
// tasks in parallel without conflict.
func TestP2PWorkflowParallelExecution(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	// Create multiple tasks
	taskIDs := []string{"TASK-001", "TASK-002", "TASK-003"}
	users := []string{"am", "bj", "cd"}

	// Create worktrees for all combinations
	for _, taskID := range taskIDs {
		for _, user := range users {
			worktreePath := git.WorktreePath(
				filepath.Join(repo.OrcDir, "worktrees"),
				taskID,
				user,
			)

			if err := os.MkdirAll(worktreePath, 0755); err != nil {
				t.Fatalf("create worktree for %s/%s: %v", taskID, user, err)
			}

			// Acquire PID guard
			guard := lock.NewPIDGuard(worktreePath)
			if err := guard.Acquire(); err != nil {
				t.Errorf("acquire for %s/%s failed: %v", taskID, user, err)
			}

			// Create state file
			state := map[string]any{
				"task_id":  taskID,
				"executor": user,
				"phase":    "implement",
			}
			testutil.WriteYAML(t, filepath.Join(worktreePath, "state.yaml"), state)
		}
	}

	// Verify all worktrees exist and are independent
	for _, taskID := range taskIDs {
		for _, user := range users {
			worktreePath := git.WorktreePath(
				filepath.Join(repo.OrcDir, "worktrees"),
				taskID,
				user,
			)

			testutil.AssertWorktreeExists(t, worktreePath)
			testutil.AssertFileExists(t, filepath.Join(worktreePath, lock.PIDFileName))
			testutil.AssertFileExists(t, filepath.Join(worktreePath, "state.yaml"))

			// Verify state content
			state := testutil.ReadYAML(t, filepath.Join(worktreePath, "state.yaml"))
			if state["task_id"] != taskID {
				t.Errorf("state task_id = %v, want %s", state["task_id"], taskID)
			}
			if state["executor"] != user {
				t.Errorf("state executor = %v, want %s", state["executor"], user)
			}
		}
	}
}

// TestP2PWorkflowSoloToP2PTransition verifies transitioning from solo to P2P mode.
func TestP2PWorkflowSoloToP2PTransition(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	// Initially in solo mode
	loader := config.NewLoader(repo.RootDir)
	tc, err := loader.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if tc.Config.TaskID.Mode != "solo" {
		t.Errorf("initial mode = %q, want solo", tc.Config.TaskID.Mode)
	}

	// Create solo task
	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")
	store := task.NewSequenceStore(seqPath)
	soloGen := task.NewTaskIDGenerator(task.ModeSolo, "", task.WithSequenceStore(store))

	soloID, _ := soloGen.Next()
	if soloID != "TASK-001" {
		t.Errorf("solo task ID = %q, want TASK-001", soloID)
	}

	// Initialize shared directory (transition to P2P)
	repo.InitSharedDir()

	// Reload config
	tc, err = loader.Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if tc.Config.TaskID.Mode != "p2p" {
		t.Errorf("after transition mode = %q, want p2p", tc.Config.TaskID.Mode)
	}

	// Create P2P task (with user identity)
	userHome := testutil.MockUserConfig(t, "AM")
	loader.SetUserDir(filepath.Join(userHome, ".orc"))

	tc, _ = loader.Load()

	p2pGen := task.NewTaskIDGenerator(task.ModeP2P, tc.Config.Identity.Initials,
		task.WithSequenceStore(store))

	p2pID, _ := p2pGen.Next()
	if p2pID != "TASK-AM-001" {
		t.Errorf("P2P task ID = %q, want TASK-AM-001", p2pID)
	}

	// Old solo task still exists
	testutil.AssertFileExists(t, seqPath)
}
