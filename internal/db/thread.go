package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SeqThread is the sequence name for thread ID generation.
const SeqThread = "thread"

// Thread represents a conversation thread stored in the database.
type Thread struct {
	ID           string
	Title        string
	Status       string // "active" or "archived"
	TaskID       string
	InitiativeID string
	SessionID    string
	FileContext  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Messages     []ThreadMessage
}

// ThreadMessage represents a single message within a thread.
type ThreadMessage struct {
	ID        int64
	ThreadID  string
	Role      string // "user" or "assistant"
	Content   string
	CreatedAt time.Time
}

// ThreadListOpts controls filtering for ListThreads.
type ThreadListOpts struct {
	Status string
	TaskID string
}

// GetNextThreadID generates the next sequential thread ID (THR-001, THR-002, ...).
func (p *ProjectDB) GetNextThreadID(ctx context.Context) (string, error) {
	num, err := p.NextSequence(ctx, SeqThread)
	if err != nil {
		return "", fmt.Errorf("get next thread sequence: %w", err)
	}
	return fmt.Sprintf("THR-%03d", num), nil
}

// CreateThread persists a new thread. It generates the ID, sets defaults,
// and populates timestamps on the passed-in struct.
func (p *ProjectDB) CreateThread(t *Thread) error {
	if t.Title == "" {
		return fmt.Errorf("thread title is required")
	}

	id, err := p.GetNextThreadID(context.Background())
	if err != nil {
		return fmt.Errorf("generate thread id: %w", err)
	}
	t.ID = id
	t.Status = "active"
	now := time.Now()
	t.CreatedAt = now
	t.UpdatedAt = now

	_, err = p.Exec(`
		INSERT INTO threads (id, title, status, task_id, initiative_id, session_id, file_context, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, t.ID, t.Title, t.Status, t.TaskID, t.InitiativeID, t.SessionID, t.FileContext,
		t.CreatedAt.Format(time.RFC3339), t.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert thread: %w", err)
	}
	return nil
}

// GetThread retrieves a thread by ID, including all messages ordered by creation time.
// Returns (nil, nil) if not found.
func (p *ProjectDB) GetThread(id string) (*Thread, error) {
	row := p.QueryRow(`
		SELECT id, title, status, task_id, initiative_id, session_id, file_context, created_at, updated_at
		FROM threads WHERE id = ?
	`, id)

	t := &Thread{}
	var createdAt, updatedAt string
	err := row.Scan(&t.ID, &t.Title, &t.Status, &t.TaskID, &t.InitiativeID,
		&t.SessionID, &t.FileContext, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get thread %s: %w", id, err)
	}

	if ts, parseErr := time.Parse(time.RFC3339, createdAt); parseErr == nil {
		t.CreatedAt = ts
	} else if ts, parseErr := time.Parse("2006-01-02 15:04:05", createdAt); parseErr == nil {
		t.CreatedAt = ts
	}
	if ts, parseErr := time.Parse(time.RFC3339, updatedAt); parseErr == nil {
		t.UpdatedAt = ts
	} else if ts, parseErr := time.Parse("2006-01-02 15:04:05", updatedAt); parseErr == nil {
		t.UpdatedAt = ts
	}

	msgs, err := p.GetThreadMessages(id)
	if err != nil {
		return nil, fmt.Errorf("get thread messages for %s: %w", id, err)
	}
	t.Messages = msgs

	return t, nil
}

// GetThreadMessages retrieves all messages for a thread ordered by creation time.
func (p *ProjectDB) GetThreadMessages(threadID string) ([]ThreadMessage, error) {
	rows, err := p.Query(`
		SELECT id, thread_id, role, content, created_at
		FROM thread_messages WHERE thread_id = ? ORDER BY created_at ASC, id ASC
	`, threadID)
	if err != nil {
		return nil, fmt.Errorf("query thread messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	msgs := make([]ThreadMessage, 0)
	for rows.Next() {
		var m ThreadMessage
		var createdAt string
		if err := rows.Scan(&m.ID, &m.ThreadID, &m.Role, &m.Content, &createdAt); err != nil {
			return nil, fmt.Errorf("scan thread message: %w", err)
		}
		if ts, parseErr := time.Parse(time.RFC3339, createdAt); parseErr == nil {
			m.CreatedAt = ts
		} else if ts, parseErr := time.Parse("2006-01-02 15:04:05", createdAt); parseErr == nil {
			m.CreatedAt = ts
		}
		msgs = append(msgs, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread messages: %w", err)
	}
	return msgs, nil
}

// ListThreads returns threads matching the given filters.
// Returns an empty (non-nil) slice if no threads match.
func (p *ProjectDB) ListThreads(opts ThreadListOpts) ([]Thread, error) {
	query := `SELECT id, title, status, task_id, initiative_id, session_id, file_context, created_at, updated_at FROM threads WHERE 1=1`
	var args []any

	if opts.Status != "" {
		query += ` AND status = ?`
		args = append(args, opts.Status)
	}
	if opts.TaskID != "" {
		query += ` AND task_id = ?`
		args = append(args, opts.TaskID)
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := p.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list threads: %w", err)
	}
	defer func() { _ = rows.Close() }()

	threads := make([]Thread, 0)
	for rows.Next() {
		var t Thread
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Title, &t.Status, &t.TaskID, &t.InitiativeID,
			&t.SessionID, &t.FileContext, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		if ts, parseErr := time.Parse(time.RFC3339, createdAt); parseErr == nil {
			t.CreatedAt = ts
		} else if ts, parseErr := time.Parse("2006-01-02 15:04:05", createdAt); parseErr == nil {
			t.CreatedAt = ts
		}
		if ts, parseErr := time.Parse(time.RFC3339, updatedAt); parseErr == nil {
			t.UpdatedAt = ts
		} else if ts, parseErr := time.Parse("2006-01-02 15:04:05", updatedAt); parseErr == nil {
			t.UpdatedAt = ts
		}
		threads = append(threads, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate threads: %w", err)
	}
	return threads, nil
}

// AddThreadMessage adds a message to a thread.
func (p *ProjectDB) AddThreadMessage(msg *ThreadMessage) error {
	now := time.Now()
	msg.CreatedAt = now

	result, err := p.Exec(`
		INSERT INTO thread_messages (thread_id, role, content, created_at)
		VALUES (?, ?, ?, ?)
	`, msg.ThreadID, msg.Role, msg.Content, msg.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert thread message: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		msg.ID = id
	}

	// Update the thread's updated_at timestamp so ListThreads ordering reflects new messages
	if _, err := p.Exec(`UPDATE threads SET updated_at = ? WHERE id = ?`,
		now.Format(time.RFC3339), msg.ThreadID); err != nil {
		return fmt.Errorf("update thread updated_at: %w", err)
	}

	return nil
}

// ArchiveThread sets a thread's status to "archived".
// Returns an error if the thread does not exist.
func (p *ProjectDB) ArchiveThread(id string) error {
	result, err := p.Exec(`
		UPDATE threads SET status = 'archived', updated_at = ? WHERE id = ?
	`, time.Now().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("archive thread %s: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check archive rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("thread %s not found", id)
	}
	return nil
}

// DeleteThread removes a thread and all its messages (via CASCADE).
// Returns an error if the thread does not exist.
func (p *ProjectDB) DeleteThread(id string) error {
	// Delete messages first (in case CASCADE isn't working)
	if _, err := p.Exec(`DELETE FROM thread_messages WHERE thread_id = ?`, id); err != nil {
		return fmt.Errorf("delete thread messages for %s: %w", id, err)
	}

	result, err := p.Exec(`DELETE FROM threads WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete thread %s: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check delete rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("thread %s not found", id)
	}
	return nil
}

// UpdateThreadSessionID updates the session ID for a thread.
func (p *ProjectDB) UpdateThreadSessionID(id, sessionID string) error {
	_, err := p.Exec(`
		UPDATE threads SET session_id = ?, updated_at = ? WHERE id = ?
	`, sessionID, time.Now().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update thread session_id %s: %w", id, err)
	}
	return nil
}
