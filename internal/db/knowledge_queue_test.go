package db

import (
	"testing"
	"time"
)

func TestKnowledgeEntry_IsStale(t *testing.T) {
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)
	hundredDaysAgo := now.AddDate(0, 0, -100)

	tests := []struct {
		name          string
		entry         *KnowledgeEntry
		stalenessDays int
		want          bool
	}{
		{
			name: "pending entries are never stale",
			entry: &KnowledgeEntry{
				Status:     KnowledgePending,
				ApprovedAt: &hundredDaysAgo,
			},
			stalenessDays: 90,
			want:          false,
		},
		{
			name: "rejected entries are never stale",
			entry: &KnowledgeEntry{
				Status:     KnowledgeRejected,
				ApprovedAt: &hundredDaysAgo,
			},
			stalenessDays: 90,
			want:          false,
		},
		{
			name: "approved entry with no timestamps is stale",
			entry: &KnowledgeEntry{
				Status: KnowledgeApproved,
			},
			stalenessDays: 90,
			want:          true,
		},
		{
			name: "approved entry validated recently is not stale",
			entry: &KnowledgeEntry{
				Status:      KnowledgeApproved,
				ApprovedAt:  &hundredDaysAgo,
				ValidatedAt: &thirtyDaysAgo,
			},
			stalenessDays: 90,
			want:          false,
		},
		{
			name: "approved entry validated long ago is stale",
			entry: &KnowledgeEntry{
				Status:      KnowledgeApproved,
				ApprovedAt:  &hundredDaysAgo,
				ValidatedAt: &hundredDaysAgo,
			},
			stalenessDays: 90,
			want:          true,
		},
		{
			name: "approved entry with only approved_at within threshold",
			entry: &KnowledgeEntry{
				Status:     KnowledgeApproved,
				ApprovedAt: &thirtyDaysAgo,
			},
			stalenessDays: 90,
			want:          false,
		},
		{
			name: "approved entry with only approved_at beyond threshold",
			entry: &KnowledgeEntry{
				Status:     KnowledgeApproved,
				ApprovedAt: &hundredDaysAgo,
			},
			stalenessDays: 90,
			want:          true,
		},
		{
			name: "validated_at takes precedence over approved_at",
			entry: &KnowledgeEntry{
				Status:      KnowledgeApproved,
				ApprovedAt:  &hundredDaysAgo, // Old approval
				ValidatedAt: &thirtyDaysAgo,  // Recent validation
			},
			stalenessDays: 90,
			want:          false,
		},
		{
			name: "just within threshold boundary (89 days)",
			entry: &KnowledgeEntry{
				Status:     KnowledgeApproved,
				ApprovedAt: func() *time.Time { t := now.AddDate(0, 0, -89); return &t }(),
			},
			stalenessDays: 90,
			want:          false, // Not stale at 89 days (within 90 day threshold)
		},
		{
			name: "just beyond threshold boundary (91 days)",
			entry: &KnowledgeEntry{
				Status:     KnowledgeApproved,
				ApprovedAt: func() *time.Time { t := now.AddDate(0, 0, -91); return &t }(),
			},
			stalenessDays: 90,
			want:          true, // Stale at 91 days (beyond 90 day threshold)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.IsStale(tt.stalenessDays)
			if got != tt.want {
				t.Errorf("IsStale(%d) = %v, want %v", tt.stalenessDays, got, tt.want)
			}
		})
	}
}

func TestKnowledgeQueueCRUD(t *testing.T) {
	// Create temp directory for test db
	tmpDir := t.TempDir()

	// Open project DB
	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Test QueueKnowledge
	entry, err := pdb.QueueKnowledge(KnowledgePattern, "Test Pattern", "A reusable pattern", "TASK-001", "test-user")
	if err != nil {
		t.Fatalf("QueueKnowledge() error = %v", err)
	}
	if entry == nil {
		t.Fatal("QueueKnowledge() returned nil entry")
	}
	if entry.Type != KnowledgePattern {
		t.Errorf("entry.Type = %v, want %v", entry.Type, KnowledgePattern)
	}
	if entry.Status != KnowledgePending {
		t.Errorf("entry.Status = %v, want %v", entry.Status, KnowledgePending)
	}

	// Test GetKnowledgeEntry
	retrieved, err := pdb.GetKnowledgeEntry(entry.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeEntry() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("GetKnowledgeEntry() returned nil")
	}
	if retrieved.Name != "Test Pattern" {
		t.Errorf("retrieved.Name = %v, want %v", retrieved.Name, "Test Pattern")
	}

	// Test ListPendingKnowledge
	pending, err := pdb.ListPendingKnowledge()
	if err != nil {
		t.Fatalf("ListPendingKnowledge() error = %v", err)
	}
	if len(pending) != 1 {
		t.Errorf("ListPendingKnowledge() returned %d entries, want 1", len(pending))
	}

	// Test CountPendingKnowledge
	count, err := pdb.CountPendingKnowledge()
	if err != nil {
		t.Fatalf("CountPendingKnowledge() error = %v", err)
	}
	if count != 1 {
		t.Errorf("CountPendingKnowledge() = %d, want 1", count)
	}

	// Test ApproveKnowledge
	approved, err := pdb.ApproveKnowledge(entry.ID, "approver")
	if err != nil {
		t.Fatalf("ApproveKnowledge() error = %v", err)
	}
	if approved.Status != KnowledgeApproved {
		t.Errorf("approved.Status = %v, want %v", approved.Status, KnowledgeApproved)
	}
	if approved.ApprovedBy != "approver" {
		t.Errorf("approved.ApprovedBy = %v, want approver", approved.ApprovedBy)
	}
	if approved.ApprovedAt == nil {
		t.Error("approved.ApprovedAt is nil")
	}

	// Test ValidateKnowledge
	validated, err := pdb.ValidateKnowledge(entry.ID, "validator")
	if err != nil {
		t.Fatalf("ValidateKnowledge() error = %v", err)
	}
	if validated.ValidatedBy != "validator" {
		t.Errorf("validated.ValidatedBy = %v, want validator", validated.ValidatedBy)
	}
	if validated.ValidatedAt == nil {
		t.Error("validated.ValidatedAt is nil")
	}

	// Test DeleteKnowledge
	err = pdb.DeleteKnowledge(entry.ID)
	if err != nil {
		t.Fatalf("DeleteKnowledge() error = %v", err)
	}

	deleted, err := pdb.GetKnowledgeEntry(entry.ID)
	if err != nil {
		t.Fatalf("GetKnowledgeEntry() after delete error = %v", err)
	}
	if deleted != nil {
		t.Error("Entry still exists after delete")
	}
}

func TestKnowledgeQueueReject(t *testing.T) {
	tmpDir := t.TempDir()

	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	defer func() { _ = pdb.Close() }()

	entry, _ := pdb.QueueKnowledge(KnowledgeGotcha, "Test Gotcha", "A gotcha", "TASK-002", "test-user")

	err = pdb.RejectKnowledge(entry.ID, "Not useful")
	if err != nil {
		t.Fatalf("RejectKnowledge() error = %v", err)
	}

	rejected, _ := pdb.GetKnowledgeEntry(entry.ID)
	if rejected.Status != KnowledgeRejected {
		t.Errorf("rejected.Status = %v, want %v", rejected.Status, KnowledgeRejected)
	}
	if rejected.RejectedReason != "Not useful" {
		t.Errorf("rejected.RejectedReason = %v, want 'Not useful'", rejected.RejectedReason)
	}
}

func TestListStaleKnowledge(t *testing.T) {
	tmpDir := t.TempDir()

	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Create and approve an entry
	entry, _ := pdb.QueueKnowledge(KnowledgeDecision, "Test Decision", "A decision", "TASK-003", "test-user")
	_, _ = pdb.ApproveKnowledge(entry.ID, "approver")

	// With a small staleness threshold (0 days), it should be stale immediately
	stale, err := pdb.ListStaleKnowledge(0)
	if err != nil {
		t.Fatalf("ListStaleKnowledge() error = %v", err)
	}
	// The entry was just approved, so with 0 days threshold it should be stale
	// (anything not validated today would be stale with 0 days)
	if len(stale) != 1 {
		t.Errorf("ListStaleKnowledge(0) returned %d entries, want 1", len(stale))
	}

	// Validate the entry
	_, _ = pdb.ValidateKnowledge(entry.ID, "validator")

	// With a large threshold (365 days), should not be stale
	stale, err = pdb.ListStaleKnowledge(365)
	if err != nil {
		t.Fatalf("ListStaleKnowledge() error = %v", err)
	}
	if len(stale) != 0 {
		t.Errorf("ListStaleKnowledge(365) returned %d entries, want 0", len(stale))
	}
}

func TestApproveAllPending(t *testing.T) {
	tmpDir := t.TempDir()

	pdb, err := OpenProject(tmpDir)
	if err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	defer func() { _ = pdb.Close() }()

	// Create multiple pending entries
	_, _ = pdb.QueueKnowledge(KnowledgePattern, "Pattern 1", "Desc 1", "TASK-001", "user")
	_, _ = pdb.QueueKnowledge(KnowledgeGotcha, "Gotcha 1", "Desc 2", "TASK-001", "user")
	_, _ = pdb.QueueKnowledge(KnowledgeDecision, "Decision 1", "Desc 3", "TASK-001", "user")

	count, err := pdb.ApproveAllPending("bulk-approver")
	if err != nil {
		t.Fatalf("ApproveAllPending() error = %v", err)
	}
	if count != 3 {
		t.Errorf("ApproveAllPending() = %d, want 3", count)
	}

	pendingCount, _ := pdb.CountPendingKnowledge()
	if pendingCount != 0 {
		t.Errorf("After ApproveAllPending, pending count = %d, want 0", pendingCount)
	}
}
