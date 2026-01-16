// Package automation provides trigger-based automation for orc tasks.
package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/randalmurphal/orc/internal/db"
)

// ProjectDBAdapter adapts ProjectDB to the automation Database interface.
type ProjectDBAdapter struct {
	pdb *db.ProjectDB
}

// NewProjectDBAdapter creates a new adapter for the ProjectDB.
func NewProjectDBAdapter(pdb *db.ProjectDB) *ProjectDBAdapter {
	return &ProjectDBAdapter{pdb: pdb}
}

// SaveTrigger saves or updates a trigger in the database.
func (a *ProjectDBAdapter) SaveTrigger(ctx context.Context, trigger *Trigger) error {
	// Serialize config as JSON
	configJSON, err := json.Marshal(trigger)
	if err != nil {
		return fmt.Errorf("marshal trigger config: %w", err)
	}

	query := `
		INSERT INTO automation_triggers (id, type, description, enabled, config, trigger_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(id) DO UPDATE SET
			type = excluded.type,
			description = excluded.description,
			enabled = excluded.enabled,
			config = excluded.config,
			trigger_count = excluded.trigger_count,
			updated_at = datetime('now')
	`

	enabled := 0
	if trigger.Enabled {
		enabled = 1
	}

	_, err = a.pdb.Driver().Exec(ctx, query,
		trigger.ID,
		string(trigger.Type),
		trigger.Description,
		enabled,
		string(configJSON),
		trigger.TriggerCount,
	)
	if err != nil {
		return fmt.Errorf("save trigger: %w", err)
	}

	return nil
}

// LoadTrigger loads a trigger by ID.
func (a *ProjectDBAdapter) LoadTrigger(ctx context.Context, id string) (*Trigger, error) {
	query := `
		SELECT id, type, description, enabled, config, last_triggered_at, trigger_count, created_at, updated_at
		FROM automation_triggers
		WHERE id = ?
	`

	row := a.pdb.Driver().QueryRow(ctx, query, id)

	var triggerID string
	var typeStr string
	var description string
	var enabled int
	var configJSON string
	var lastTriggeredAt sql.NullString
	var triggerCount int
	var createdAt, updatedAt string

	err := row.Scan(
		&triggerID,
		&typeStr,
		&description,
		&enabled,
		&configJSON,
		&lastTriggeredAt,
		&triggerCount,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("trigger %s not found", id)
		}
		return nil, fmt.Errorf("load trigger: %w", err)
	}

	// Parse config JSON to get full trigger details (Condition, Action, Cooldown, Mode)
	var trigger Trigger
	if err := json.Unmarshal([]byte(configJSON), &trigger); err != nil {
		return nil, fmt.Errorf("unmarshal trigger config: %w", err)
	}

	// Apply scanned values from database (these are the source of truth, not the JSON)
	trigger.ID = triggerID
	trigger.Type = TriggerType(typeStr)
	trigger.Description = description
	trigger.Enabled = enabled == 1
	trigger.TriggerCount = triggerCount

	// Parse timestamps
	if lastTriggeredAt.Valid {
		t, parseErr := time.Parse(time.RFC3339, lastTriggeredAt.String)
		if parseErr != nil {
			// Try SQLite datetime format
			t, parseErr = time.Parse("2006-01-02 15:04:05", lastTriggeredAt.String)
		}
		if parseErr == nil {
			trigger.LastTriggeredAt = &t
		}
	}

	if t, parseErr := time.Parse(time.RFC3339, createdAt); parseErr == nil {
		trigger.CreatedAt = t
	} else if t, parseErr := time.Parse("2006-01-02 15:04:05", createdAt); parseErr == nil {
		trigger.CreatedAt = t
	}

	if t, parseErr := time.Parse(time.RFC3339, updatedAt); parseErr == nil {
		trigger.UpdatedAt = t
	} else if t, parseErr := time.Parse("2006-01-02 15:04:05", updatedAt); parseErr == nil {
		trigger.UpdatedAt = t
	}

	return &trigger, nil
}

// LoadAllTriggers loads all triggers from the database.
func (a *ProjectDBAdapter) LoadAllTriggers(ctx context.Context) ([]*Trigger, error) {
	query := `
		SELECT id, type, description, enabled, config, last_triggered_at, trigger_count, created_at, updated_at
		FROM automation_triggers
		ORDER BY id
	`

	rows, err := a.pdb.Driver().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query triggers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var triggers []*Trigger
	for rows.Next() {
		var triggerID string
		var typeStr string
		var description string
		var enabled int
		var configJSON string
		var lastTriggeredAt sql.NullString
		var triggerCount int
		var createdAt, updatedAt string

		err := rows.Scan(
			&triggerID,
			&typeStr,
			&description,
			&enabled,
			&configJSON,
			&lastTriggeredAt,
			&triggerCount,
			&createdAt,
			&updatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan trigger: %w", err)
		}

		// Parse config JSON to get full trigger details (Condition, Action, Cooldown, Mode)
		var trigger Trigger
		if err := json.Unmarshal([]byte(configJSON), &trigger); err != nil {
			return nil, fmt.Errorf("unmarshal trigger config: %w", err)
		}

		// Apply scanned values from database (these are the source of truth, not the JSON)
		trigger.ID = triggerID
		trigger.Type = TriggerType(typeStr)
		trigger.Description = description
		trigger.Enabled = enabled == 1
		trigger.TriggerCount = triggerCount

		// Parse timestamps
		if lastTriggeredAt.Valid {
			t, parseErr := time.Parse(time.RFC3339, lastTriggeredAt.String)
			if parseErr != nil {
				// Try SQLite datetime format
				t, parseErr = time.Parse("2006-01-02 15:04:05", lastTriggeredAt.String)
			}
			if parseErr == nil {
				trigger.LastTriggeredAt = &t
			}
		}

		if t, parseErr := time.Parse(time.RFC3339, createdAt); parseErr == nil {
			trigger.CreatedAt = t
		} else if t, parseErr := time.Parse("2006-01-02 15:04:05", createdAt); parseErr == nil {
			trigger.CreatedAt = t
		}

		if t, parseErr := time.Parse(time.RFC3339, updatedAt); parseErr == nil {
			trigger.UpdatedAt = t
		} else if t, parseErr := time.Parse("2006-01-02 15:04:05", updatedAt); parseErr == nil {
			trigger.UpdatedAt = t
		}

		triggers = append(triggers, &trigger)
	}

	return triggers, rows.Err()
}

// UpdateTriggerState updates trigger state after firing.
// Deprecated: Use IncrementTriggerCount for atomic increments.
func (a *ProjectDBAdapter) UpdateTriggerState(ctx context.Context, id string, lastTriggered time.Time, count int) error {
	query := `
		UPDATE automation_triggers
		SET last_triggered_at = ?,
			trigger_count = ?,
			updated_at = datetime('now')
		WHERE id = ?
	`

	_, err := a.pdb.Driver().Exec(ctx, query,
		lastTriggered.Format(time.RFC3339),
		count,
		id,
	)
	if err != nil {
		return fmt.Errorf("update trigger state: %w", err)
	}

	return nil
}

// IncrementTriggerCount atomically increments trigger count and updates last_triggered_at.
// Returns the new count. This avoids race conditions from read-modify-write patterns.
func (a *ProjectDBAdapter) IncrementTriggerCount(ctx context.Context, id string, triggeredAt time.Time) (int, error) {
	query := `
		UPDATE automation_triggers
		SET last_triggered_at = ?,
			trigger_count = trigger_count + 1,
			updated_at = datetime('now')
		WHERE id = ?
	`

	_, err := a.pdb.Driver().Exec(ctx, query,
		triggeredAt.Format(time.RFC3339),
		id,
	)
	if err != nil {
		return 0, fmt.Errorf("increment trigger count: %w", err)
	}

	// Get the new count
	var newCount int
	err = a.pdb.Driver().QueryRow(ctx,
		"SELECT trigger_count FROM automation_triggers WHERE id = ?",
		id,
	).Scan(&newCount)
	if err != nil {
		return 0, fmt.Errorf("get trigger count: %w", err)
	}

	return newCount, nil
}

// SetTriggerEnabled updates the enabled state of a trigger.
// This persists the change to the database.
func (a *ProjectDBAdapter) SetTriggerEnabled(ctx context.Context, id string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	query := `
		UPDATE automation_triggers
		SET enabled = ?,
			updated_at = datetime('now')
		WHERE id = ?
	`

	result, err := a.pdb.Driver().Exec(ctx, query, enabledInt, id)
	if err != nil {
		return fmt.Errorf("set trigger enabled: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("trigger not found: %s", id)
	}

	return nil
}

// GetCounter gets a counter value.
func (a *ProjectDBAdapter) GetCounter(ctx context.Context, triggerID, metric string) (int, error) {
	query := `
		SELECT count FROM trigger_counters
		WHERE trigger_id = ? AND metric = ?
	`

	var count int
	err := a.pdb.Driver().QueryRow(ctx, query, triggerID, metric).Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("get counter: %w", err)
	}

	return count, nil
}

// IncrementCounter increments a counter.
func (a *ProjectDBAdapter) IncrementCounter(ctx context.Context, triggerID, metric string) error {
	query := `
		INSERT INTO trigger_counters (trigger_id, metric, count, last_reset_at)
		VALUES (?, ?, 1, datetime('now'))
		ON CONFLICT(trigger_id, metric) DO UPDATE SET
			count = count + 1
	`

	_, err := a.pdb.Driver().Exec(ctx, query, triggerID, metric)
	if err != nil {
		return fmt.Errorf("increment counter: %w", err)
	}

	return nil
}

// IncrementAndGetCounter atomically increments a counter and returns its new value.
// This prevents race conditions between increment and threshold check.
func (a *ProjectDBAdapter) IncrementAndGetCounter(ctx context.Context, triggerID, metric string) (int, error) {
	query := `
		INSERT INTO trigger_counters (trigger_id, metric, count, last_reset_at)
		VALUES (?, ?, 1, datetime('now'))
		ON CONFLICT(trigger_id, metric) DO UPDATE SET
			count = count + 1
		RETURNING count
	`

	var count int
	err := a.pdb.Driver().QueryRow(ctx, query, triggerID, metric).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("increment and get counter: %w", err)
	}

	return count, nil
}

// ResetCounter resets a counter to zero.
func (a *ProjectDBAdapter) ResetCounter(ctx context.Context, triggerID, metric string) error {
	query := `
		UPDATE trigger_counters
		SET count = 0, last_reset_at = datetime('now')
		WHERE trigger_id = ? AND metric = ?
	`

	_, err := a.pdb.Driver().Exec(ctx, query, triggerID, metric)
	if err != nil {
		return fmt.Errorf("reset counter: %w", err)
	}

	return nil
}

// CreateExecution creates an execution record.
func (a *ProjectDBAdapter) CreateExecution(ctx context.Context, exec *Execution) error {
	query := `
		INSERT INTO trigger_executions (trigger_id, task_id, triggered_at, trigger_reason, status)
		VALUES (?, ?, ?, ?, ?)
	`

	result, err := a.pdb.Driver().Exec(ctx, query,
		exec.TriggerID,
		exec.TaskID,
		exec.TriggeredAt.Format(time.RFC3339),
		exec.TriggerReason,
		string(exec.Status),
	)
	if err != nil {
		return fmt.Errorf("create execution: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		exec.ID = id
	}

	return nil
}

// UpdateExecutionStatus updates an execution's status.
func (a *ProjectDBAdapter) UpdateExecutionStatus(ctx context.Context, id int64, status ExecutionStatus, errorMsg string) error {
	query := `
		UPDATE trigger_executions
		SET status = ?, error_message = ?, completed_at = datetime('now')
		WHERE id = ?
	`

	_, err := a.pdb.Driver().Exec(ctx, query, string(status), errorMsg, id)
	if err != nil {
		return fmt.Errorf("update execution status: %w", err)
	}

	return nil
}

// GetRecentExecutions gets recent executions for a trigger.
func (a *ProjectDBAdapter) GetRecentExecutions(ctx context.Context, triggerID string, limit int) ([]*Execution, error) {
	query := `
		SELECT id, trigger_id, task_id, triggered_at, trigger_reason, status, completed_at, error_message
		FROM trigger_executions
		WHERE trigger_id = ?
		ORDER BY triggered_at DESC
		LIMIT ?
	`

	rows, err := a.pdb.Driver().Query(ctx, query, triggerID, limit)
	if err != nil {
		return nil, fmt.Errorf("query executions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var executions []*Execution
	for rows.Next() {
		var exec Execution
		var taskID sql.NullString
		var triggeredAt, completedAt sql.NullString
		var statusStr string
		var errorMsg sql.NullString

		err := rows.Scan(
			&exec.ID,
			&exec.TriggerID,
			&taskID,
			&triggeredAt,
			&exec.TriggerReason,
			&statusStr,
			&completedAt,
			&errorMsg,
		)
		if err != nil {
			return nil, fmt.Errorf("scan execution: %w", err)
		}

		if taskID.Valid {
			exec.TaskID = taskID.String
		}
		if triggeredAt.Valid {
			exec.TriggeredAt, _ = time.Parse(time.RFC3339, triggeredAt.String)
		}
		exec.Status = ExecutionStatus(statusStr)
		if completedAt.Valid {
			t, _ := time.Parse(time.RFC3339, completedAt.String)
			exec.CompletedAt = &t
		}
		if errorMsg.Valid {
			exec.ErrorMessage = errorMsg.String
		}

		executions = append(executions, &exec)
	}

	return executions, rows.Err()
}

// RecordMetric records a metric value.
func (a *ProjectDBAdapter) RecordMetric(ctx context.Context, metric *Metric) error {
	query := `
		INSERT INTO trigger_metrics (metric, value, task_id, recorded_at)
		VALUES (?, ?, ?, datetime('now'))
	`

	result, err := a.pdb.Driver().Exec(ctx, query,
		metric.Name,
		metric.Value,
		metric.TaskID,
	)
	if err != nil {
		return fmt.Errorf("record metric: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		metric.ID = id
	}

	return nil
}

// GetLatestMetric gets the most recent value for a metric.
func (a *ProjectDBAdapter) GetLatestMetric(ctx context.Context, name string) (*Metric, error) {
	query := `
		SELECT id, metric, value, task_id, recorded_at
		FROM trigger_metrics
		WHERE metric = ?
		ORDER BY recorded_at DESC
		LIMIT 1
	`

	var metric Metric
	var taskID sql.NullString
	var recordedAt string

	err := a.pdb.Driver().QueryRow(ctx, query, name).Scan(
		&metric.ID,
		&metric.Name,
		&metric.Value,
		&taskID,
		&recordedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("metric %s not found", name)
		}
		return nil, fmt.Errorf("get metric: %w", err)
	}

	if taskID.Valid {
		metric.TaskID = taskID.String
	}
	metric.RecordedAt, _ = time.Parse(time.RFC3339, recordedAt)

	return &metric, nil
}

// CreateNotification creates a new notification.
func (a *ProjectDBAdapter) CreateNotification(ctx context.Context, notif *Notification) error {
	query := `
		INSERT INTO notifications (id, type, title, message, source_type, source_id, dismissed, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, 0, datetime('now'), ?)
	`

	var expiresAt sql.NullString
	if notif.ExpiresAt != nil {
		expiresAt.Valid = true
		expiresAt.String = notif.ExpiresAt.Format(time.RFC3339)
	}

	_, err := a.pdb.Driver().Exec(ctx, query,
		notif.ID,
		notif.Type,
		notif.Title,
		notif.Message,
		notif.SourceType,
		notif.SourceID,
		expiresAt,
	)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	return nil
}

// GetActiveNotifications gets all non-dismissed, non-expired notifications.
func (a *ProjectDBAdapter) GetActiveNotifications(ctx context.Context) ([]*Notification, error) {
	query := `
		SELECT id, type, title, message, source_type, source_id, dismissed, created_at, expires_at
		FROM notifications
		WHERE dismissed = 0
		AND (expires_at IS NULL OR expires_at > datetime('now'))
		ORDER BY created_at DESC
	`

	rows, err := a.pdb.Driver().Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query notifications: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var notifications []*Notification
	for rows.Next() {
		var notif Notification
		var dismissed int
		var message, sourceType, sourceID sql.NullString
		var createdAt string
		var expiresAt sql.NullString

		err := rows.Scan(
			&notif.ID,
			&notif.Type,
			&notif.Title,
			&message,
			&sourceType,
			&sourceID,
			&dismissed,
			&createdAt,
			&expiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}

		notif.Dismissed = dismissed == 1
		if message.Valid {
			notif.Message = message.String
		}
		if sourceType.Valid {
			notif.SourceType = sourceType.String
		}
		if sourceID.Valid {
			notif.SourceID = sourceID.String
		}
		notif.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if expiresAt.Valid {
			t, _ := time.Parse(time.RFC3339, expiresAt.String)
			notif.ExpiresAt = &t
		}

		notifications = append(notifications, &notif)
	}

	return notifications, rows.Err()
}

// DismissNotification marks a notification as dismissed.
func (a *ProjectDBAdapter) DismissNotification(ctx context.Context, id string) error {
	query := `UPDATE notifications SET dismissed = 1 WHERE id = ?`

	result, err := a.pdb.Driver().Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("dismiss notification: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("notification %s not found", id)
	}

	return nil
}

// DismissAllNotifications marks all notifications as dismissed.
func (a *ProjectDBAdapter) DismissAllNotifications(ctx context.Context) error {
	query := `UPDATE notifications SET dismissed = 1 WHERE dismissed = 0`

	_, err := a.pdb.Driver().Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("dismiss all notifications: %w", err)
	}

	return nil
}

// DeleteExpiredNotifications removes notifications that have expired.
func (a *ProjectDBAdapter) DeleteExpiredNotifications(ctx context.Context) (int64, error) {
	query := `DELETE FROM notifications WHERE expires_at IS NOT NULL AND expires_at < datetime('now')`

	result, err := a.pdb.Driver().Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("delete expired notifications: %w", err)
	}

	return result.RowsAffected()
}

// GetMaxAutoTaskNumber returns the highest AUTO-XXX task number.
// This is more efficient than loading all tasks when generating new automation task IDs.
func (a *ProjectDBAdapter) GetMaxAutoTaskNumber(ctx context.Context) (int, error) {
	// Use CAST and SUBSTR to extract the numeric portion of AUTO-XXX IDs
	// Tasks with ID format "AUTO-NNN" will have their number extracted
	query := `
		SELECT COALESCE(MAX(CAST(SUBSTR(id, 6) AS INTEGER)), 0)
		FROM tasks
		WHERE id LIKE 'AUTO-%' AND is_automation = 1
	`

	var maxNum int
	err := a.pdb.Driver().QueryRow(ctx, query).Scan(&maxNum)
	if err != nil {
		return 0, fmt.Errorf("get max auto task number: %w", err)
	}

	return maxNum, nil
}

// LoadRecentCompletedTasks loads the N most recently completed tasks.
// This is more efficient than loading all tasks for automation context.
func (a *ProjectDBAdapter) LoadRecentCompletedTasks(ctx context.Context, limit int, automationOnly bool) ([]*TaskSummary, error) {
	query := `
		SELECT id, title, weight, category, completed_at, metadata
		FROM tasks
		WHERE status IN ('completed', 'finished')
	`
	if automationOnly {
		query += ` AND is_automation = 0` // Exclude automation tasks from context
	}
	query += `
		ORDER BY completed_at DESC
		LIMIT ?
	`

	rows, err := a.pdb.Driver().Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent completed tasks: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tasks []*TaskSummary
	for rows.Next() {
		var t TaskSummary
		var completedAt, metadata sql.NullString

		err := rows.Scan(&t.ID, &t.Title, &t.Weight, &t.Category, &completedAt, &metadata)
		if err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}

		if completedAt.Valid {
			if parsed, parseErr := time.Parse(time.RFC3339, completedAt.String); parseErr == nil {
				t.CompletedAt = &parsed
			} else if parsed, parseErr := time.Parse("2006-01-02 15:04:05", completedAt.String); parseErr == nil {
				t.CompletedAt = &parsed
			}
		}

		if metadata.Valid && metadata.String != "" {
			if err := json.Unmarshal([]byte(metadata.String), &t.Metadata); err != nil {
				// Log but don't fail on metadata parse errors
				t.Metadata = nil
			}
		}

		tasks = append(tasks, &t)
	}

	return tasks, rows.Err()
}

// TaskSummary is a lightweight task representation for automation context.
type TaskSummary struct {
	ID          string
	Title       string
	Weight      string
	Category    string
	CompletedAt *time.Time
	Metadata    map[string]string
}
