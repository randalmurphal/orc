package db

import (
	"fmt"
	"time"
)

// GetThreadMessages retrieves all messages for a thread ordered by creation time.
func (p *ProjectDB) GetThreadMessages(threadID string) ([]ThreadMessage, error) {
	rows, err := p.Query(threadMessagesQuery(p.Dialect()), threadID)
	if err != nil {
		return nil, fmt.Errorf("query thread messages: %w", err)
	}
	defer func() { _ = rows.Close() }()

	messages := make([]ThreadMessage, 0)
	for rows.Next() {
		message, err := scanThreadMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, *message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate thread messages: %w", err)
	}
	return messages, nil
}

// AddThreadMessage adds a message to a thread.
func (p *ProjectDB) AddThreadMessage(msg *ThreadMessage) error {
	if msg == nil {
		return fmt.Errorf("thread message is required")
	}
	now := time.Now().UTC()
	msg.CreatedAt = now

	result, err := p.Exec(threadMessageInsertQuery(p.Dialect()), msg.ThreadID, msg.Role, msg.Content, msg.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert thread message: %w", err)
	}
	if id, err := result.LastInsertId(); err == nil {
		msg.ID = id
	}

	if err := p.touchThread(msg.ThreadID, now); err != nil {
		return err
	}
	return nil
}
