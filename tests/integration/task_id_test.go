package integration

import (
	"path/filepath"
	"testing"

	"github.com/randalmurphal/orc/internal/task"
	"github.com/randalmurphal/orc/tests/testutil"
)

// TestTaskIDSoloMode verifies task ID generation in solo mode.
func TestTaskIDSoloMode(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")
	store := task.NewSequenceStore(seqPath)

	gen := task.NewTaskIDGenerator(task.ModeSolo, "", task.WithSequenceStore(store))

	// First task
	id1, err := gen.Next()
	if err != nil {
		t.Fatalf("generate first ID: %v", err)
	}
	if id1 != "TASK-001" {
		t.Errorf("first ID = %q, want TASK-001", id1)
	}

	// Second task
	id2, err := gen.Next()
	if err != nil {
		t.Fatalf("generate second ID: %v", err)
	}
	if id2 != "TASK-002" {
		t.Errorf("second ID = %q, want TASK-002", id2)
	}

	// Third task
	id3, err := gen.Next()
	if err != nil {
		t.Fatalf("generate third ID: %v", err)
	}
	if id3 != "TASK-003" {
		t.Errorf("third ID = %q, want TASK-003", id3)
	}
}

// TestTaskIDP2PMode verifies task ID generation in P2P mode with initials.
func TestTaskIDP2PMode(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")
	store := task.NewSequenceStore(seqPath)

	// Alice's generator
	aliceGen := task.NewTaskIDGenerator(task.ModeP2P, "AM", task.WithSequenceStore(store))

	// First task for Alice
	id1, err := aliceGen.Next()
	if err != nil {
		t.Fatalf("generate Alice's first ID: %v", err)
	}
	if id1 != "TASK-AM-001" {
		t.Errorf("Alice's first ID = %q, want TASK-AM-001", id1)
	}

	// Second task for Alice
	id2, err := aliceGen.Next()
	if err != nil {
		t.Fatalf("generate Alice's second ID: %v", err)
	}
	if id2 != "TASK-AM-002" {
		t.Errorf("Alice's second ID = %q, want TASK-AM-002", id2)
	}
}

// TestTaskIDMultiplePrefixes verifies separate sequences for different prefixes.
func TestTaskIDMultiplePrefixes(t *testing.T) {
	repo := testutil.SetupTestRepo(t)
	repo.InitSharedDir()

	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")
	store := task.NewSequenceStore(seqPath)

	// Alice's generator
	aliceGen := task.NewTaskIDGenerator(task.ModeP2P, "AM", task.WithSequenceStore(store))
	// Bob's generator
	bobGen := task.NewTaskIDGenerator(task.ModeP2P, "BJ", task.WithSequenceStore(store))

	// Alice creates first task
	aliceID1, err := aliceGen.Next()
	if err != nil {
		t.Fatalf("Alice's first: %v", err)
	}
	if aliceID1 != "TASK-AM-001" {
		t.Errorf("Alice's first = %q, want TASK-AM-001", aliceID1)
	}

	// Bob creates first task
	bobID1, err := bobGen.Next()
	if err != nil {
		t.Fatalf("Bob's first: %v", err)
	}
	if bobID1 != "TASK-BJ-001" {
		t.Errorf("Bob's first = %q, want TASK-BJ-001", bobID1)
	}

	// Alice creates second task
	aliceID2, err := aliceGen.Next()
	if err != nil {
		t.Fatalf("Alice's second: %v", err)
	}
	if aliceID2 != "TASK-AM-002" {
		t.Errorf("Alice's second = %q, want TASK-AM-002", aliceID2)
	}

	// Bob creates second task
	bobID2, err := bobGen.Next()
	if err != nil {
		t.Fatalf("Bob's second: %v", err)
	}
	if bobID2 != "TASK-BJ-002" {
		t.Errorf("Bob's second = %q, want TASK-BJ-002", bobID2)
	}
}

// TestTaskIDSequencePersistence verifies that sequence numbers persist across runs.
func TestTaskIDSequencePersistence(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")

	// First "session"
	store1 := task.NewSequenceStore(seqPath)
	gen1 := task.NewTaskIDGenerator(task.ModeSolo, "", task.WithSequenceStore(store1))

	id1, _ := gen1.Next()
	id2, _ := gen1.Next()
	id3, _ := gen1.Next()

	if id1 != "TASK-001" || id2 != "TASK-002" || id3 != "TASK-003" {
		t.Errorf("first session IDs = %s, %s, %s", id1, id2, id3)
	}

	// Second "session" (simulating new orc run)
	store2 := task.NewSequenceStore(seqPath)
	gen2 := task.NewTaskIDGenerator(task.ModeSolo, "", task.WithSequenceStore(store2))

	id4, _ := gen2.Next()
	if id4 != "TASK-004" {
		t.Errorf("after persistence, next ID = %q, want TASK-004", id4)
	}
}

// TestTaskIDParsing verifies parsing task IDs back to components.
func TestTaskIDParsing(t *testing.T) {
	tests := []struct {
		id         string
		wantPrefix string
		wantSeq    int
		wantOK     bool
	}{
		// Solo mode
		{"TASK-001", "", 1, true},
		{"TASK-999", "", 999, true},

		// P2P mode
		{"TASK-AM-001", "AM", 1, true},
		{"TASK-BJ-042", "BJ", 42, true},
		{"TASK-XYZ-123", "XYZ", 123, true},

		// Invalid
		{"", "", 0, false},
		{"TASK", "", 0, false},
		{"TASK-", "", 0, false},
		{"INVALID", "", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			prefix, seq, ok := task.ParseTaskID(tt.id)
			if ok != tt.wantOK {
				t.Errorf("ParseTaskID(%q) ok = %v, want %v", tt.id, ok, tt.wantOK)
				return
			}
			if !tt.wantOK {
				return
			}
			if prefix != tt.wantPrefix {
				t.Errorf("ParseTaskID(%q) prefix = %q, want %q", tt.id, prefix, tt.wantPrefix)
			}
			if seq != tt.wantSeq {
				t.Errorf("ParseTaskID(%q) seq = %d, want %d", tt.id, seq, tt.wantSeq)
			}
		})
	}
}

// TestTaskIDResolvePrefix verifies prefix resolution from identity config.
func TestTaskIDResolvePrefix(t *testing.T) {
	tests := []struct {
		name     string
		source   task.PrefixSource
		identity *task.IdentityConfig
		wantErr  bool
	}{
		{
			name:     "none returns empty",
			source:   task.PrefixNone,
			identity: nil,
			wantErr:  false,
		},
		{
			name:   "initials with identity",
			source: task.PrefixInitials,
			identity: &task.IdentityConfig{
				Initials: "AM",
			},
			wantErr: false,
		},
		{
			name:     "initials without identity",
			source:   task.PrefixInitials,
			identity: nil,
			wantErr:  true,
		},
		{
			name:   "initials with empty initials",
			source: task.PrefixInitials,
			identity: &task.IdentityConfig{
				Initials: "",
			},
			wantErr: true,
		},
		{
			name:     "username always works",
			source:   task.PrefixUsername,
			identity: nil,
			wantErr:  false,
		},
		{
			name:     "machine always works",
			source:   task.PrefixMachine,
			identity: nil,
			wantErr:  false,
		},
		{
			name:   "email_hash with email",
			source: task.PrefixEmailHash,
			identity: &task.IdentityConfig{
				Email: "test@example.com",
			},
			wantErr: false,
		},
		{
			name:     "email_hash without email",
			source:   task.PrefixEmailHash,
			identity: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix, err := task.ResolvePrefix(tt.source, tt.identity)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolvePrefix() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			t.Logf("ResolvePrefix(%s) = %q", tt.source, prefix)
		})
	}
}

// TestTaskIDRequiresStore verifies that a sequence store is required.
func TestTaskIDRequiresStore(t *testing.T) {
	// Generator without sequence store should error
	gen := task.NewTaskIDGenerator(task.ModeSolo, "")

	_, err := gen.Next()
	if err == nil {
		t.Error("expected error when no sequence store is configured")
	}
}

// TestTaskIDSequenceStoreWithCatchup verifies catch-up when sequence
// is set ahead (e.g., from external sync).
func TestTaskIDSequenceStoreWithCatchup(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")

	// Create store and set sequence to simulate existing tasks up to 5
	store := task.NewSequenceStore(seqPath)
	if err := store.SetSequence("", 5); err != nil {
		t.Fatalf("set sequence: %v", err)
	}

	// Generator should continue from stored sequence
	gen := task.NewTaskIDGenerator(task.ModeSolo, "",
		task.WithSequenceStore(store),
	)

	// Next ID should be TASK-006
	id, err := gen.Next()
	if err != nil {
		t.Fatalf("generate next ID: %v", err)
	}
	if id != "TASK-006" {
		t.Errorf("next ID = %q, want TASK-006 (catch-up from stored sequence)", id)
	}
}

// TestTaskIDPrefixNormalization verifies that prefixes are normalized to uppercase.
func TestTaskIDPrefixNormalization(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")
	store := task.NewSequenceStore(seqPath)

	// Create generator with lowercase prefix
	gen := task.NewTaskIDGenerator(task.ModeP2P, "am", task.WithSequenceStore(store))

	id, err := gen.Next()
	if err != nil {
		t.Fatalf("generate ID: %v", err)
	}

	// ID should have uppercase prefix
	if id != "TASK-AM-001" {
		t.Errorf("ID = %q, want TASK-AM-001 (prefix normalized to uppercase)", id)
	}

	// Generator should report uppercase prefix
	if gen.Prefix() != "AM" {
		t.Errorf("Prefix() = %q, want AM", gen.Prefix())
	}
}

// TestTaskIDMode verifies that Mode() returns correct mode.
func TestTaskIDMode(t *testing.T) {
	tests := []struct {
		mode task.Mode
		want task.Mode
	}{
		{task.ModeSolo, task.ModeSolo},
		{task.ModeP2P, task.ModeP2P},
		{task.ModeTeam, task.ModeTeam},
	}

	for _, tt := range tests {
		t.Run(string(tt.mode), func(t *testing.T) {
			gen := task.NewTaskIDGenerator(tt.mode, "")
			if gen.Mode() != tt.want {
				t.Errorf("Mode() = %v, want %v", gen.Mode(), tt.want)
			}
		})
	}
}

// TestTaskIDSoloIgnoresPrefix verifies that solo mode ignores prefix parameter.
func TestTaskIDSoloIgnoresPrefix(t *testing.T) {
	repo := testutil.SetupTestRepo(t)

	seqPath := filepath.Join(repo.OrcDir, "local", "sequences.yaml")
	store := task.NewSequenceStore(seqPath)

	// Create solo generator with prefix (should be ignored)
	gen := task.NewTaskIDGenerator(task.ModeSolo, "AM", task.WithSequenceStore(store))

	id, err := gen.Next()
	if err != nil {
		t.Fatalf("generate ID: %v", err)
	}

	// ID should NOT have prefix (solo mode)
	if id != "TASK-001" {
		t.Errorf("ID = %q, want TASK-001 (solo mode ignores prefix)", id)
	}
}
