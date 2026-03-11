package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/randalmurphal/orc/internal/controlplane"
	"github.com/randalmurphal/orc/internal/db/driver"
)

type attentionSignalScanner interface {
	Scan(dest ...any) error
}

func (p *ProjectDB) GetNextAttentionSignalID(ctx context.Context) (string, error) {
	num, err := p.NextSequence(ctx, SeqAttentionSignal)
	if err != nil {
		return "", fmt.Errorf("get next attention signal sequence: %w", err)
	}
	return fmt.Sprintf("ATT-%03d", num), nil
}

func (p *ProjectDB) SaveAttentionSignal(signal *controlplane.PersistedAttentionSignal) error {
	if signal == nil {
		return fmt.Errorf("attention signal is required")
	}
	if err := validateAttentionSignal(signal); err != nil {
		return err
	}
	if signal.ID == "" {
		id, err := p.GetNextAttentionSignalID(context.Background())
		if err != nil {
			return err
		}
		signal.ID = id
	}

	return p.RunInTx(context.Background(), func(tx *TxOps) error {
		existing, err := getActiveAttentionSignalByReferenceTx(tx, signal.Kind, signal.ReferenceType, signal.ReferenceID)
		if err != nil {
			return err
		}
		if existing != nil {
			return updateAttentionSignalTx(tx, p.Driver(), existing.ID, signal)
		}
		return createAttentionSignalTx(tx, p.Driver(), signal)
	})
}

func (p *ProjectDB) CreateAttentionSignal(signal *controlplane.PersistedAttentionSignal) error {
	if signal == nil {
		return fmt.Errorf("attention signal is required")
	}
	if err := validateAttentionSignal(signal); err != nil {
		return err
	}
	if signal.ID == "" {
		id, err := p.GetNextAttentionSignalID(context.Background())
		if err != nil {
			return err
		}
		signal.ID = id
	}

	return p.RunInTx(context.Background(), func(tx *TxOps) error {
		return createAttentionSignalTx(tx, p.Driver(), signal)
	})
}

func (p *ProjectDB) GetAttentionSignal(id string) (*controlplane.PersistedAttentionSignal, error) {
	placeholder := p.Placeholder(1)
	signal, err := getAttentionSignalByRow(p.QueryRow(fmt.Sprintf(`
		SELECT id, kind, status, reference_type, reference_id, title, summary,
		       created_at, updated_at, resolved_at, resolved_by
		FROM attention_signals
		WHERE id = %s
	`, placeholder), id))
	if err != nil {
		return nil, fmt.Errorf("get attention signal %s: %w", id, err)
	}
	return signal, nil
}

func (p *ProjectDB) ListActiveAttentionSignals() ([]*controlplane.PersistedAttentionSignal, error) {
	rows, err := p.Query(`
		SELECT id, kind, status, reference_type, reference_id, title, summary,
		       created_at, updated_at, resolved_at, resolved_by
		FROM attention_signals
		WHERE resolved_at IS NULL
		ORDER BY updated_at DESC, created_at DESC, id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list active attention signals: %w", err)
	}
	defer func() { _ = rows.Close() }()

	signals := make([]*controlplane.PersistedAttentionSignal, 0)
	for rows.Next() {
		signal, scanErr := scanAttentionSignal(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		signals = append(signals, signal)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attention signals: %w", err)
	}
	return signals, nil
}

func (p *ProjectDB) ResolveAttentionSignal(id, resolvedBy string) (*controlplane.PersistedAttentionSignal, error) {
	return p.resolveAttentionSignal(context.Background(), id, resolvedBy)
}

func (p *ProjectDB) CountActiveAttentionSignals() (int, error) {
	var count int
	if err := p.QueryRow(`
		SELECT COUNT(*)
		FROM attention_signals
		WHERE resolved_at IS NULL
	`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active attention signals: %w", err)
	}
	return count, nil
}

func (p *ProjectDB) resolveAttentionSignal(ctx context.Context, id, resolvedBy string) (*controlplane.PersistedAttentionSignal, error) {
	returnValue := &controlplane.PersistedAttentionSignal{}
	err := p.RunInTx(ctx, func(tx *TxOps) error {
		existing, err := getAttentionSignalTx(tx, id)
		if err != nil {
			return err
		}
		if existing == nil {
			return fmt.Errorf("attention signal %s not found", id)
		}
		if existing.ResolvedAt != nil {
			*returnValue = *existing
			return nil
		}

		now := p.Driver().Now()
		args := []any{
			controlplane.AttentionSignalStatusResolved,
			resolvedBy,
			id,
		}
		query := fmt.Sprintf(`
			UPDATE attention_signals
			SET status = %s,
			    updated_at = %s,
			    resolved_at = %s,
			    resolved_by = %s
			WHERE id = %s
		`,
			txPlaceholder(tx, 1),
			now,
			now,
			txPlaceholder(tx, 2),
			txPlaceholder(tx, 3),
		)
		if tx.Dialect() == driver.DialectSQLite {
			query = fmt.Sprintf(`
				UPDATE attention_signals
				SET status = ?,
				    updated_at = %s,
				    resolved_at = %s,
				    resolved_by = ?
				WHERE id = ?
			`, now, now)
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return fmt.Errorf("resolve attention signal %s: %w", id, err)
		}

		updated, err := getAttentionSignalTx(tx, id)
		if err != nil {
			return err
		}
		if updated == nil {
			return fmt.Errorf("attention signal %s not found after resolve", id)
		}
		*returnValue = *updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return returnValue, nil
}

func createAttentionSignalTx(
	tx *TxOps,
	drv driver.Driver,
	signal *controlplane.PersistedAttentionSignal,
) error {
	now := drv.Now()
	query := fmt.Sprintf(`
		INSERT INTO attention_signals (
			id, kind, status, reference_type, reference_id, title, summary, created_at, updated_at
		) VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s)
	`,
		txPlaceholder(tx, 1),
		txPlaceholder(tx, 2),
		txPlaceholder(tx, 3),
		txPlaceholder(tx, 4),
		txPlaceholder(tx, 5),
		txPlaceholder(tx, 6),
		txPlaceholder(tx, 7),
		now,
		now,
	)
	if tx.Dialect() == driver.DialectSQLite {
		query = fmt.Sprintf(`
			INSERT INTO attention_signals (
				id, kind, status, reference_type, reference_id, title, summary, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, %s, %s)
		`, now, now)
	}

	if _, err := tx.Exec(query,
		signal.ID,
		string(signal.Kind),
		signal.Status,
		signal.ReferenceType,
		signal.ReferenceID,
		signal.Title,
		signal.Summary,
	); err != nil {
		return fmt.Errorf("insert attention signal %s: %w", signal.ID, err)
	}

	created, err := getAttentionSignalTx(tx, signal.ID)
	if err != nil {
		return err
	}
	if created == nil {
		return fmt.Errorf("attention signal %s not found after insert", signal.ID)
	}
	*signal = *created
	return nil
}

func updateAttentionSignalTx(
	tx *TxOps,
	drv driver.Driver,
	id string,
	signal *controlplane.PersistedAttentionSignal,
) error {
	now := drv.Now()
	query := fmt.Sprintf(`
		UPDATE attention_signals
		SET status = %s,
		    title = %s,
		    summary = %s,
		    updated_at = %s,
		    resolved_at = NULL,
		    resolved_by = ''
		WHERE id = %s
	`,
		txPlaceholder(tx, 1),
		txPlaceholder(tx, 2),
		txPlaceholder(tx, 3),
		now,
		txPlaceholder(tx, 4),
	)
	if tx.Dialect() == driver.DialectSQLite {
		query = fmt.Sprintf(`
			UPDATE attention_signals
			SET status = ?,
			    title = ?,
			    summary = ?,
			    updated_at = %s,
			    resolved_at = NULL,
			    resolved_by = ''
			WHERE id = ?
		`, now)
	}

	if _, err := tx.Exec(query, signal.Status, signal.Title, signal.Summary, id); err != nil {
		return fmt.Errorf("update attention signal %s: %w", id, err)
	}

	updated, err := getAttentionSignalTx(tx, id)
	if err != nil {
		return err
	}
	if updated == nil {
		return fmt.Errorf("attention signal %s not found after update", id)
	}
	*signal = *updated
	return nil
}

func getActiveAttentionSignalByReferenceTx(
	tx *TxOps,
	kind controlplane.AttentionSignalKind,
	referenceType string,
	referenceID string,
) (*controlplane.PersistedAttentionSignal, error) {
	query := fmt.Sprintf(`
		SELECT id, kind, status, reference_type, reference_id, title, summary,
		       created_at, updated_at, resolved_at, resolved_by
		FROM attention_signals
		WHERE kind = %s
		  AND reference_type = %s
		  AND reference_id = %s
		  AND resolved_at IS NULL
		ORDER BY updated_at DESC, created_at DESC, id DESC
		LIMIT 1
	`,
		txPlaceholder(tx, 1),
		txPlaceholder(tx, 2),
		txPlaceholder(tx, 3),
	)
	return getAttentionSignalByRow(tx.QueryRow(query, string(kind), referenceType, referenceID))
}

func getAttentionSignalTx(tx *TxOps, id string) (*controlplane.PersistedAttentionSignal, error) {
	query := fmt.Sprintf(`
		SELECT id, kind, status, reference_type, reference_id, title, summary,
		       created_at, updated_at, resolved_at, resolved_by
		FROM attention_signals
		WHERE id = %s
	`, txPlaceholder(tx, 1))
	return getAttentionSignalByRow(tx.QueryRow(query, id))
}

func getAttentionSignalByRow(row attentionSignalScanner) (*controlplane.PersistedAttentionSignal, error) {
	signal, err := scanAttentionSignal(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return signal, nil
}

func scanAttentionSignal(row attentionSignalScanner) (*controlplane.PersistedAttentionSignal, error) {
	var signal controlplane.PersistedAttentionSignal
	var kind string
	var summary sql.NullString
	var resolvedBy sql.NullString
	var createdAt any
	var updatedAt any
	var resolvedAt any

	if err := row.Scan(
		&signal.ID,
		&kind,
		&signal.Status,
		&signal.ReferenceType,
		&signal.ReferenceID,
		&signal.Title,
		&summary,
		&createdAt,
		&updatedAt,
		&resolvedAt,
		&resolvedBy,
	); err != nil {
		return nil, err
	}

	signal.Kind = controlplane.AttentionSignalKind(kind)
	if summary.Valid {
		signal.Summary = summary.String
	}
	signal.CreatedAt = timestampOrZero(createdAt)
	signal.UpdatedAt = timestampOrZero(updatedAt)
	if resolvedTime, ok := scannedTimestamp(resolvedAt); ok {
		signal.ResolvedAt = &resolvedTime
	}
	if resolvedBy.Valid {
		signal.ResolvedBy = resolvedBy.String
	}

	return &signal, nil
}

func validateAttentionSignal(signal *controlplane.PersistedAttentionSignal) error {
	if signal == nil {
		return fmt.Errorf("attention signal is required")
	}
	if signal.Kind == "" {
		return fmt.Errorf("attention signal kind is required")
	}
	if !isValidAttentionSignalKind(signal.Kind) {
		return fmt.Errorf("invalid attention signal kind %q", signal.Kind)
	}
	if strings.TrimSpace(signal.Status) == "" {
		return fmt.Errorf("attention signal status is required")
	}
	if strings.TrimSpace(signal.ReferenceType) == "" {
		return fmt.Errorf("attention signal reference type is required")
	}
	if !isValidAttentionSignalReferenceType(signal.ReferenceType) {
		return fmt.Errorf("invalid attention signal reference type %q", signal.ReferenceType)
	}
	if strings.TrimSpace(signal.ReferenceID) == "" {
		return fmt.Errorf("attention signal reference id is required")
	}
	if strings.TrimSpace(signal.Title) == "" {
		return fmt.Errorf("attention signal title is required")
	}
	return nil
}

func isValidAttentionSignalKind(kind controlplane.AttentionSignalKind) bool {
	switch kind {
	case controlplane.AttentionSignalKindBlocker,
		controlplane.AttentionSignalKindDecisionRequest,
		controlplane.AttentionSignalKindDiscussionNeeded,
		controlplane.AttentionSignalKindVerificationSummary:
		return true
	default:
		return false
	}
}

func isValidAttentionSignalReferenceType(referenceType string) bool {
	switch referenceType {
	case controlplane.AttentionSignalReferenceTypeTask,
		controlplane.AttentionSignalReferenceTypeRecommendation,
		controlplane.AttentionSignalReferenceTypeRun,
		controlplane.AttentionSignalReferenceTypeInitiative:
		return true
	default:
		return false
	}
}

func txPlaceholder(tx *TxOps, index int) string {
	if tx.Dialect() == driver.DialectSQLite {
		return "?"
	}
	return fmt.Sprintf("$%d", index)
}
