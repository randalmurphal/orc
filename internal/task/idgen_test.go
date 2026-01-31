package task

import (

	"path/filepath"
	"testing"
)

func TestSequenceStore_NextSequence(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")
	store := NewSequenceStore(seqPath)

	// First sequence should be 1
	seq, err := store.NextSequence("AM")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if seq != 1 {
		t.Errorf("first sequence = %d, want 1", seq)
	}

	// Second sequence should be 2
	seq, err = store.NextSequence("AM")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if seq != 2 {
		t.Errorf("second sequence = %d, want 2", seq)
	}

	// Different prefix starts at 1
	seq, err = store.NextSequence("BJ")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if seq != 1 {
		t.Errorf("BJ first sequence = %d, want 1", seq)
	}

	// AM should continue at 3
	seq, err = store.NextSequence("AM")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if seq != 3 {
		t.Errorf("AM third sequence = %d, want 3", seq)
	}
}

func TestSequenceStore_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "local", "sequences.yaml")

	// Create store and generate some sequences
	store1 := NewSequenceStore(seqPath)
	_, _ = store1.NextSequence("AM")
	_, _ = store1.NextSequence("AM")
	_, _ = store1.NextSequence("AM")

	// Create new store instance - should continue from persisted state
	store2 := NewSequenceStore(seqPath)
	seq, err := store2.NextSequence("AM")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if seq != 4 {
		t.Errorf("persisted sequence = %d, want 4", seq)
	}
}

func TestSequenceStore_SoloMode(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")
	store := NewSequenceStore(seqPath)

	// Empty prefix uses "_solo" internally
	seq1, _ := store.NextSequence("")
	seq2, _ := store.NextSequence("")

	if seq1 != 1 || seq2 != 2 {
		t.Errorf("solo sequences = %d, %d, want 1, 2", seq1, seq2)
	}

	// Verify it's stored under _solo key
	current, _ := store.GetSequence("")
	if current != 2 {
		t.Errorf("GetSequence('') = %d, want 2", current)
	}
}

func TestSequenceStore_CaseNormalization(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")
	store := NewSequenceStore(seqPath)

	// Different cases should be normalized
	_, _ = store.NextSequence("am")
	_, _ = store.NextSequence("AM")
	seq, _ := store.NextSequence("Am")

	if seq != 3 {
		t.Errorf("case-normalized sequence = %d, want 3", seq)
	}
}

func TestSequenceStore_SetSequence(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")
	store := NewSequenceStore(seqPath)

	// Set sequence directly
	err := store.SetSequence("AM", 100)
	if err != nil {
		t.Fatalf("SetSequence failed: %v", err)
	}

	// Verify it was set
	current, err := store.GetSequence("AM")
	if err != nil {
		t.Fatalf("GetSequence failed: %v", err)
	}
	if current != 100 {
		t.Errorf("GetSequence = %d, want 100", current)
	}

	// Next should return 101
	next, err := store.NextSequence("AM")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if next != 101 {
		t.Errorf("NextSequence = %d, want 101", next)
	}
}

func TestSequenceStore_SetSequence_SoloMode(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")
	store := NewSequenceStore(seqPath)

	// Set sequence for solo mode (empty prefix)
	err := store.SetSequence("", 50)
	if err != nil {
		t.Fatalf("SetSequence failed: %v", err)
	}

	// Verify it was set
	current, err := store.GetSequence("")
	if err != nil {
		t.Fatalf("GetSequence failed: %v", err)
	}
	if current != 50 {
		t.Errorf("GetSequence = %d, want 50", current)
	}
}

func TestTaskIDGenerator_SoloMode(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")
	store := NewSequenceStore(seqPath)

	gen := NewTaskIDGenerator(ModeSolo, "", WithSequenceStore(store))

	id1, err := gen.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if id1 != "TASK-001" {
		t.Errorf("first ID = %s, want TASK-001", id1)
	}

	id2, err := gen.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if id2 != "TASK-002" {
		t.Errorf("second ID = %s, want TASK-002", id2)
	}
}

func TestTaskIDGenerator_P2PMode(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, ".orc", "local", "sequences.yaml")

	store := NewSequenceStore(seqPath)
	gen := NewTaskIDGenerator(ModeP2P, "AM", WithSequenceStore(store))

	id1, err := gen.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if id1 != "TASK-AM-001" {
		t.Errorf("first ID = %s, want TASK-AM-001", id1)
	}

	id2, err := gen.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if id2 != "TASK-AM-002" {
		t.Errorf("second ID = %s, want TASK-AM-002", id2)
	}
}

func TestTaskIDGenerator_WithSequenceStore(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")
	store := NewSequenceStore(seqPath)

	gen := NewTaskIDGenerator(ModeP2P, "BJ", WithSequenceStore(store))

	id1, _ := gen.Next()
	id2, _ := gen.Next()
	id3, _ := gen.Next()

	if id1 != "TASK-BJ-001" || id2 != "TASK-BJ-002" || id3 != "TASK-BJ-003" {
		t.Errorf("IDs = %s, %s, %s, want TASK-BJ-001, TASK-BJ-002, TASK-BJ-003", id1, id2, id3)
	}
}

func TestTaskIDGenerator_MultiplePrefixes(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")
	store := NewSequenceStore(seqPath)

	genAlice := NewTaskIDGenerator(ModeP2P, "AM", WithSequenceStore(store))
	genBob := NewTaskIDGenerator(ModeP2P, "BJ", WithSequenceStore(store))

	id1, _ := genAlice.Next() // AM-001
	id2, _ := genBob.Next()   // BJ-001
	id3, _ := genAlice.Next() // AM-002
	id4, _ := genBob.Next()   // BJ-002

	expected := []string{"TASK-AM-001", "TASK-BJ-001", "TASK-AM-002", "TASK-BJ-002"}
	actual := []string{id1, id2, id3, id4}

	for i, want := range expected {
		if actual[i] != want {
			t.Errorf("ID[%d] = %s, want %s", i, actual[i], want)
		}
	}
}

func TestTaskIDGenerator_ScanExisting(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")

	store := NewSequenceStore(seqPath)
	// Set sequence to 10 to simulate existing tasks ahead
	_ = store.SetSequence("AM", 10)

	gen := NewTaskIDGenerator(ModeP2P, "AM",
		WithSequenceStore(store),
	)

	id, err := gen.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if id != "TASK-AM-011" {
		t.Errorf("ID = %s, want TASK-AM-011", id)
	}
}

func TestTaskIDGenerator_SequenceStoreCatchUp(t *testing.T) {
	tmpDir := t.TempDir()
	seqPath := filepath.Join(tmpDir, "sequences.yaml")

	store := NewSequenceStore(seqPath)
	// Set sequence far ahead to simulate catch-up scenario
	_ = store.SetSequence("AM", 100)

	gen := NewTaskIDGenerator(ModeP2P, "AM",
		WithSequenceStore(store),
	)

	// First call should return 101
	id1, err := gen.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if id1 != "TASK-AM-101" {
		t.Errorf("ID = %s, want TASK-AM-101", id1)
	}

	// Verify store was incremented
	current, _ := store.GetSequence("AM")
	if current != 101 {
		t.Errorf("stored sequence after first call = %d, want 101", current)
	}

	// Next call should return 102
	id2, err := gen.Next()
	if err != nil {
		t.Fatalf("Next() failed: %v", err)
	}
	if id2 != "TASK-AM-102" {
		t.Errorf("ID = %s, want TASK-AM-102", id2)
	}

	// Verify store was incremented again
	current, _ = store.GetSequence("AM")
	if current != 102 {
		t.Errorf("stored sequence = %d, want 102", current)
	}
}

func TestResolvePrefix_None(t *testing.T) {
	prefix, err := ResolvePrefix(PrefixNone, nil)
	if err != nil {
		t.Fatalf("ResolvePrefix failed: %v", err)
	}
	if prefix != "" {
		t.Errorf("prefix = %q, want empty", prefix)
	}
}

func TestResolvePrefix_Initials(t *testing.T) {
	identity := &IdentityConfig{Initials: "am"}

	prefix, err := ResolvePrefix(PrefixInitials, identity)
	if err != nil {
		t.Fatalf("ResolvePrefix failed: %v", err)
	}
	if prefix != "AM" {
		t.Errorf("prefix = %q, want AM", prefix)
	}
}

func TestResolvePrefix_Initials_Missing(t *testing.T) {
	_, err := ResolvePrefix(PrefixInitials, nil)
	if err == nil {
		t.Error("expected error for missing initials")
	}

	_, err = ResolvePrefix(PrefixInitials, &IdentityConfig{})
	if err == nil {
		t.Error("expected error for empty initials")
	}
}

func TestResolvePrefix_Username(t *testing.T) {
	prefix, err := ResolvePrefix(PrefixUsername, nil)
	if err != nil {
		t.Fatalf("ResolvePrefix failed: %v", err)
	}
	// Just verify it returns something non-empty
	if prefix == "" {
		t.Error("expected non-empty username prefix")
	}
}

func TestResolvePrefix_EmailHash(t *testing.T) {
	identity := &IdentityConfig{Email: "alice@example.com"}

	prefix, err := ResolvePrefix(PrefixEmailHash, identity)
	if err != nil {
		t.Fatalf("ResolvePrefix failed: %v", err)
	}
	if len(prefix) != 4 {
		t.Errorf("prefix length = %d, want 4", len(prefix))
	}

	// Same email should produce same hash
	prefix2, _ := ResolvePrefix(PrefixEmailHash, identity)
	if prefix != prefix2 {
		t.Errorf("hash not deterministic: %s != %s", prefix, prefix2)
	}

	// Different email should produce different hash
	identity2 := &IdentityConfig{Email: "bob@example.com"}
	prefix3, _ := ResolvePrefix(PrefixEmailHash, identity2)
	if prefix == prefix3 {
		t.Errorf("different emails produced same hash")
	}
}

func TestResolvePrefix_EmailHash_Missing(t *testing.T) {
	_, err := ResolvePrefix(PrefixEmailHash, nil)
	if err == nil {
		t.Error("expected error for missing email")
	}

	_, err = ResolvePrefix(PrefixEmailHash, &IdentityConfig{})
	if err == nil {
		t.Error("expected error for empty email")
	}
}

func TestResolvePrefix_Machine(t *testing.T) {
	prefix, err := ResolvePrefix(PrefixMachine, nil)
	if err != nil {
		t.Fatalf("ResolvePrefix failed: %v", err)
	}
	// Just verify it returns something non-empty
	if prefix == "" {
		t.Error("expected non-empty machine prefix")
	}
	// And it's reasonable length
	if len(prefix) > 12 {
		t.Errorf("machine prefix too long: %d chars", len(prefix))
	}
}

func TestResolvePrefix_Unknown(t *testing.T) {
	_, err := ResolvePrefix("unknown", nil)
	if err == nil {
		t.Error("expected error for unknown prefix source")
	}
}

func TestParseTaskID_Solo(t *testing.T) {
	tests := []struct {
		id     string
		prefix string
		seq    int
		ok     bool
	}{
		{"TASK-001", "", 1, true},
		{"TASK-123", "", 123, true},
		{"TASK-999", "", 999, true},
		{"INVALID", "", 0, false},
		{"TASK-", "", 0, false},
		{"TASK-abc", "", 0, false},
	}

	for _, tt := range tests {
		prefix, seq, ok := ParseTaskID(tt.id)
		if ok != tt.ok || prefix != tt.prefix || seq != tt.seq {
			t.Errorf("ParseTaskID(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tt.id, prefix, seq, ok, tt.prefix, tt.seq, tt.ok)
		}
	}
}

func TestParseTaskID_Prefixed(t *testing.T) {
	tests := []struct {
		id     string
		prefix string
		seq    int
		ok     bool
	}{
		{"TASK-AM-001", "AM", 1, true},
		{"TASK-BJ-123", "BJ", 123, true},
		{"TASK-alice-001", "ALICE", 1, true},
		{"TASK-a1b2-001", "A1B2", 1, true},
		{"TASK-laptop-999", "LAPTOP", 999, true},
		{"TASK--001", "", 0, false}, // Empty prefix
		{"TASK-AM-", "", 0, false},  // Missing sequence
	}

	for _, tt := range tests {
		prefix, seq, ok := ParseTaskID(tt.id)
		if ok != tt.ok || prefix != tt.prefix || seq != tt.seq {
			t.Errorf("ParseTaskID(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tt.id, prefix, seq, ok, tt.prefix, tt.seq, tt.ok)
		}
	}
}

func TestDefaultSequencePath(t *testing.T) {
	path := DefaultSequencePath("")
	expected := ".orc/local/sequences.yaml"
	if path != expected {
		t.Errorf("DefaultSequencePath(\"\") = %q, want %q", path, expected)
	}
}

func TestTaskIDGenerator_Prefix(t *testing.T) {
	gen := NewTaskIDGenerator(ModeP2P, "am")

	if gen.Prefix() != "AM" {
		t.Errorf("Prefix() = %q, want AM", gen.Prefix())
	}
	if gen.Mode() != ModeP2P {
		t.Errorf("Mode() = %v, want %v", gen.Mode(), ModeP2P)
	}
}

func TestTaskIDGenerator_NoStore(t *testing.T) {
	// Without a sequence store, generation should return an error
	gen := NewTaskIDGenerator(ModeSolo, "")

	_, err := gen.Next()
	if err == nil {
		t.Fatal("Next() should fail without a sequence store")
	}
}
