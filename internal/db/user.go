package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// User represents a user in the system.
// Users are stored in GlobalDB and referenced by user ID in other tables.
type User struct {
	ID        string
	Name      string
	Email     string
	CreatedAt time.Time
}

// GetOrCreateUser returns the user ID for a given name.
// If the user doesn't exist, a new user is created with the given name.
// This operation is idempotent - calling with the same name returns the same ID.
func (g *GlobalDB) GetOrCreateUser(name string) (string, error) {
	// First try to get existing user
	var id string
	err := g.QueryRow("SELECT id FROM users WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("check existing user: %w", err)
	}

	// User doesn't exist, create new one
	id = uuid.New().String()
	_, err = g.Exec(`
		INSERT INTO users (id, name, email, created_at)
		VALUES (?, ?, '', ?)
	`, id, name, time.Now().Format(time.RFC3339))
	if err != nil {
		// Could be a race condition - another process created the user
		// Try to get the existing user again
		err2 := g.QueryRow("SELECT id FROM users WHERE name = ?", name).Scan(&id)
		if err2 == nil {
			return id, nil
		}
		return "", fmt.Errorf("create user: %w", err)
	}

	return id, nil
}

// GetOrCreateUserWithEmail returns the user ID for a given name, creating the user with email if needed.
// If the user already exists, the email is not updated.
func (g *GlobalDB) GetOrCreateUserWithEmail(name, email string) (string, error) {
	// First try to get existing user
	var id string
	err := g.QueryRow("SELECT id FROM users WHERE name = ?", name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("check existing user: %w", err)
	}

	// User doesn't exist, create new one with email
	id = uuid.New().String()
	_, err = g.Exec(`
		INSERT INTO users (id, name, email, created_at)
		VALUES (?, ?, ?, ?)
	`, id, name, email, time.Now().Format(time.RFC3339))
	if err != nil {
		// Could be a race condition - another process created the user
		// Try to get the existing user again
		err2 := g.QueryRow("SELECT id FROM users WHERE name = ?", name).Scan(&id)
		if err2 == nil {
			return id, nil
		}
		return "", fmt.Errorf("create user with email: %w", err)
	}

	return id, nil
}

// GetUser retrieves a user by ID.
// Returns (nil, nil) if the user doesn't exist.
func (g *GlobalDB) GetUser(id string) (*User, error) {
	var u User
	var email sql.NullString
	var createdAt string

	err := g.QueryRow(`
		SELECT id, name, email, created_at FROM users WHERE id = ?
	`, id).Scan(&u.ID, &u.Name, &email, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", id, err)
	}

	if email.Valid {
		u.Email = email.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		u.CreatedAt = ts
	} else if ts, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
		u.CreatedAt = ts
	}

	return &u, nil
}

// GetUserByName retrieves a user by name.
// Returns (nil, nil) if the user doesn't exist.
func (g *GlobalDB) GetUserByName(name string) (*User, error) {
	var u User
	var email sql.NullString
	var createdAt string

	err := g.QueryRow(`
		SELECT id, name, email, created_at FROM users WHERE name = ?
	`, name).Scan(&u.ID, &u.Name, &email, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by name %s: %w", name, err)
	}

	if email.Valid {
		u.Email = email.String
	}
	if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
		u.CreatedAt = ts
	} else if ts, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
		u.CreatedAt = ts
	}

	return &u, nil
}

// ListUsers returns all users in the system.
func (g *GlobalDB) ListUsers() ([]User, error) {
	rows, err := g.Query(`
		SELECT id, name, email, created_at FROM users ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []User
	for rows.Next() {
		var u User
		var email sql.NullString
		var createdAt string

		if err := rows.Scan(&u.ID, &u.Name, &email, &createdAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		if email.Valid {
			u.Email = email.String
		}
		if ts, err := time.Parse(time.RFC3339, createdAt); err == nil {
			u.CreatedAt = ts
		} else if ts, err := time.Parse("2006-01-02 15:04:05", createdAt); err == nil {
			u.CreatedAt = ts
		}

		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}
