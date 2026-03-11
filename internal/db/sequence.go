package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/randalmurphal/orc/internal/db/driver"
)

// Sequence names for atomic ID generation.
const (
	SeqWorkflowRun     = "workflow_run"
	SeqTask            = "task"
	SeqInitiative      = "initiative"
	SeqAutoTask        = "auto_task"
	SeqNote            = "note"
	SeqRecommendation  = "recommendation"
	SeqAttentionSignal = "attention_signal"
)

// NextSequence atomically increments and returns the next value for a sequence.
// This is safe for concurrent access across processes because:
// 1. UPDATE acquires an exclusive write lock in SQLite
// 2. The lock is held until the transaction commits
// 3. Other processes block until the lock is released
//
// This replaces the racy MAX()+1 pattern that caused duplicate IDs when
// parallel tasks called GetNextWorkflowRunID simultaneously.
func (p *ProjectDB) NextSequence(ctx context.Context, name string) (int, error) {
	var value int

	err := p.RunInTx(ctx, func(tx *TxOps) error {
		// Ensure the sequence exists (handles first-run case)
		var insertQuery string
		if tx.Dialect() == driver.DialectSQLite {
			insertQuery = `INSERT OR IGNORE INTO sequences (name, current_value) VALUES (?, 0)`
		} else {
			insertQuery = `INSERT INTO sequences (name, current_value) VALUES ($1, 0) ON CONFLICT DO NOTHING`
		}
		if _, err := tx.Exec(insertQuery, name); err != nil {
			return fmt.Errorf("ensure sequence exists: %w", err)
		}

		// Atomically increment and get the new value
		// UPDATE acquires exclusive lock, preventing concurrent access
		var updateQuery, selectQuery string
		if tx.Dialect() == driver.DialectSQLite {
			updateQuery = `UPDATE sequences SET current_value = current_value + 1 WHERE name = ?`
			selectQuery = `SELECT current_value FROM sequences WHERE name = ?`
		} else {
			updateQuery = `UPDATE sequences SET current_value = current_value + 1 WHERE name = $1`
			selectQuery = `SELECT current_value FROM sequences WHERE name = $1`
		}

		if _, err := tx.Exec(updateQuery, name); err != nil {
			return fmt.Errorf("increment sequence: %w", err)
		}

		if err := tx.QueryRow(selectQuery, name).Scan(&value); err != nil {
			return fmt.Errorf("read sequence value: %w", err)
		}

		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("next sequence %s: %w", name, err)
	}

	return value, nil
}

// GetSequence returns the current value of a sequence without incrementing it.
func (p *ProjectDB) GetSequence(name string) (int, error) {
	var value int
	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `SELECT COALESCE(current_value, 0) FROM sequences WHERE name = ?`
	} else {
		query = `SELECT COALESCE(current_value, 0) FROM sequences WHERE name = $1`
	}

	err := p.QueryRow(query, name).Scan(&value)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get sequence %s: %w", name, err)
	}
	return value, nil
}

// SetSequence sets a sequence to a specific value.
// Used for catch-up when existing data exceeds the stored sequence.
func (p *ProjectDB) SetSequence(name string, value int) error {
	var query string
	if p.Dialect() == driver.DialectSQLite {
		query = `INSERT INTO sequences (name, current_value) VALUES (?, ?)
			ON CONFLICT(name) DO UPDATE SET current_value = excluded.current_value`
	} else {
		query = `INSERT INTO sequences (name, current_value) VALUES ($1, $2)
			ON CONFLICT(name) DO UPDATE SET current_value = excluded.current_value`
	}

	if _, err := p.Exec(query, name, value); err != nil {
		return fmt.Errorf("set sequence %s: %w", name, err)
	}
	return nil
}
