package db

import (
	"context"
	"sync"
	"testing"
)

func TestNextSequence(t *testing.T) {
	pdb := openTestProjectDB(t)

	ctx := context.Background()

	// First call should return 1
	val1, err := pdb.NextSequence(ctx, "test_seq")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if val1 != 1 {
		t.Errorf("first call: got %d, want 1", val1)
	}

	// Second call should return 2
	val2, err := pdb.NextSequence(ctx, "test_seq")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if val2 != 2 {
		t.Errorf("second call: got %d, want 2", val2)
	}

	// Different sequence should start at 1
	val3, err := pdb.NextSequence(ctx, "other_seq")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if val3 != 1 {
		t.Errorf("different sequence: got %d, want 1", val3)
	}
}

func TestNextSequenceConcurrent(t *testing.T) {
	pdb := openTestProjectDB(t)

	ctx := context.Background()
	const numGoroutines = 50 // Simulate concurrent access

	// Use a WaitGroup to synchronize goroutines
	var wg sync.WaitGroup
	results := make(chan int, numGoroutines)
	errors := make(chan error, numGoroutines)

	// Start all goroutines at roughly the same time
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startBarrier.Wait() // Wait for all goroutines to be ready

			val, err := pdb.NextSequence(ctx, "concurrent_test")
			if err != nil {
				errors <- err
				return
			}
			results <- val
		}()
	}

	// Release all goroutines simultaneously
	startBarrier.Done()

	// Wait for all to complete
	wg.Wait()
	close(results)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Fatalf("NextSequence error: %v", err)
	}

	// Collect all results
	seen := make(map[int]bool)
	for val := range results {
		if seen[val] {
			t.Errorf("duplicate sequence value: %d", val)
		}
		seen[val] = true
	}

	// Verify we got all expected values (1 through numGoroutines)
	if len(seen) != numGoroutines {
		t.Errorf("expected %d unique values, got %d", numGoroutines, len(seen))
	}

	for i := 1; i <= numGoroutines; i++ {
		if !seen[i] {
			t.Errorf("missing expected value: %d", i)
		}
	}
}

func TestGetNextWorkflowRunIDConcurrent(t *testing.T) {
	pdb := openTestProjectDB(t)

	const numGoroutines = 20

	var wg sync.WaitGroup
	results := make(chan string, numGoroutines)
	errors := make(chan error, numGoroutines)

	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startBarrier.Wait()

			id, err := pdb.GetNextWorkflowRunID()
			if err != nil {
				errors <- err
				return
			}
			results <- id
		}()
	}

	startBarrier.Done()
	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Fatalf("GetNextWorkflowRunID error: %v", err)
	}

	// Verify no duplicates
	seen := make(map[string]bool)
	for id := range results {
		if seen[id] {
			t.Errorf("duplicate workflow run ID: %s", id)
		}
		seen[id] = true
	}

	if len(seen) != numGoroutines {
		t.Errorf("expected %d unique IDs, got %d", numGoroutines, len(seen))
	}
}

func TestGetNextTaskIDConcurrent(t *testing.T) {
	pdb := openTestProjectDB(t)

	const numGoroutines = 20

	var wg sync.WaitGroup
	results := make(chan string, numGoroutines)
	errors := make(chan error, numGoroutines)

	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startBarrier.Wait()

			id, err := pdb.GetNextTaskID()
			if err != nil {
				errors <- err
				return
			}
			results <- id
		}()
	}

	startBarrier.Done()
	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Fatalf("GetNextTaskID error: %v", err)
	}

	seen := make(map[string]bool)
	for id := range results {
		if seen[id] {
			t.Errorf("duplicate task ID: %s", id)
		}
		seen[id] = true
	}

	if len(seen) != numGoroutines {
		t.Errorf("expected %d unique IDs, got %d", numGoroutines, len(seen))
	}
}

func TestGetNextInitiativeIDConcurrent(t *testing.T) {
	pdb := openTestProjectDB(t)

	const numGoroutines = 20

	var wg sync.WaitGroup
	results := make(chan string, numGoroutines)
	errors := make(chan error, numGoroutines)

	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startBarrier.Wait()

			id, err := pdb.GetNextInitiativeID()
			if err != nil {
				errors <- err
				return
			}
			results <- id
		}()
	}

	startBarrier.Done()
	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		t.Fatalf("GetNextInitiativeID error: %v", err)
	}

	seen := make(map[string]bool)
	for id := range results {
		if seen[id] {
			t.Errorf("duplicate initiative ID: %s", id)
		}
		seen[id] = true
	}

	if len(seen) != numGoroutines {
		t.Errorf("expected %d unique IDs, got %d", numGoroutines, len(seen))
	}
}

func TestSetSequence(t *testing.T) {
	pdb := openTestProjectDB(t)

	// Set a sequence to a specific value
	if err := pdb.SetSequence("test", 100); err != nil {
		t.Fatalf("SetSequence failed: %v", err)
	}

	// Get should return that value
	val, err := pdb.GetSequence("test")
	if err != nil {
		t.Fatalf("GetSequence failed: %v", err)
	}
	if val != 100 {
		t.Errorf("GetSequence: got %d, want 100", val)
	}

	// Next should return 101
	next, err := pdb.NextSequence(context.Background(), "test")
	if err != nil {
		t.Fatalf("NextSequence failed: %v", err)
	}
	if next != 101 {
		t.Errorf("NextSequence after SetSequence: got %d, want 101", next)
	}
}
