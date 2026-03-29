// Package postgres provides PostgreSQL session storage for agnogo.
//
//	import "github.com/saeedalam/agnogo/storage/postgres"
//	store, _ := postgres.New(db)
package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/saeedalam/agnogo"
)

// Storage implements agnogo.Storage using PostgreSQL.
type Storage struct {
	db    *sql.DB
	table string
}

// New creates PostgreSQL storage. Creates table if not exists.
func New(db *sql.DB, tableName ...string) (*Storage, error) {
	table := "agnogo_sessions"
	if len(tableName) > 0 && tableName[0] != "" {
		table = tableName[0]
	}
	s := &Storage{db: db, table: table}
	if err := s.createTable(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Storage) createTable() error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL DEFAULT '',
		channel TEXT NOT NULL DEFAULT '',
		history JSONB NOT NULL DEFAULT '[]',
		memory JSONB NOT NULL DEFAULT '{}',
		state JSONB NOT NULL DEFAULT '{}',
		metadata JSONB NOT NULL DEFAULT '{}',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`, s.table)
	_, err := s.db.Exec(q)
	return err
}

func (s *Storage) Load(ctx context.Context, sessionID string) (*agnogo.Session, error) {
	q := fmt.Sprintf(`SELECT user_id, channel, history, memory, state, metadata, created_at, updated_at
		FROM %s WHERE id = $1`, s.table)
	row := s.db.QueryRowContext(ctx, q, sessionID)

	var userID, channel string
	var historyJSON, memoryJSON, stateJSON, metadataJSON []byte
	var createdAt, updatedAt time.Time

	if err := row.Scan(&userID, &channel, &historyJSON, &memoryJSON, &stateJSON, &metadataJSON, &createdAt, &updatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, agnogo.ErrSessionNotFound
		}
		return nil, fmt.Errorf("load session: %w", err)
	}

	sess := agnogo.NewSession(sessionID)
	sess.UserID = userID
	sess.Channel = channel
	sess.CreatedAt = createdAt
	sess.UpdatedAt = updatedAt
	json.Unmarshal(historyJSON, &sess.History)
	json.Unmarshal(memoryJSON, &sess.Memory)
	json.Unmarshal(stateJSON, &sess.State)
	json.Unmarshal(metadataJSON, &sess.Metadata)
	return sess, nil
}

func (s *Storage) Save(ctx context.Context, sess *agnogo.Session) error {
	historyJSON, _ := json.Marshal(sess.History)
	memoryJSON, _ := json.Marshal(sess.Memory)
	stateJSON, _ := json.Marshal(sess.State)
	metadataJSON, _ := json.Marshal(sess.Metadata)

	q := fmt.Sprintf(`INSERT INTO %s (id, user_id, channel, history, memory, state, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (id) DO UPDATE SET
			user_id = EXCLUDED.user_id,
			channel = EXCLUDED.channel,
			history = EXCLUDED.history,
			memory = EXCLUDED.memory,
			state = EXCLUDED.state,
			metadata = EXCLUDED.metadata,
			updated_at = NOW()`, s.table)

	_, err := s.db.ExecContext(ctx, q,
		sess.ID, sess.UserID, sess.Channel,
		historyJSON, memoryJSON, stateJSON, metadataJSON,
	)
	if err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}

// Delete removes a session.
func (s *Storage) Delete(ctx context.Context, sessionID string) error {
	q := fmt.Sprintf("DELETE FROM %s WHERE id = $1", s.table)
	_, err := s.db.ExecContext(ctx, q, sessionID)
	return err
}

// List returns all sessions, ordered by updated_at desc.
func (s *Storage) List(ctx context.Context, limit int) ([]*agnogo.Session, error) {
	if limit <= 0 {
		limit = 50
	}
	q := fmt.Sprintf(`SELECT id, user_id, channel, history, memory, state, metadata, created_at, updated_at
		FROM %s ORDER BY updated_at DESC LIMIT $1`, s.table)

	rows, err := s.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*agnogo.Session
	for rows.Next() {
		var id, userID, channel string
		var historyJSON, memoryJSON, stateJSON, metadataJSON []byte
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &userID, &channel, &historyJSON, &memoryJSON, &stateJSON, &metadataJSON, &createdAt, &updatedAt); err != nil {
			continue
		}
		sess := agnogo.NewSession(id)
		sess.UserID = userID
		sess.Channel = channel
		sess.CreatedAt = createdAt
		sess.UpdatedAt = updatedAt
		json.Unmarshal(historyJSON, &sess.History)
		json.Unmarshal(memoryJSON, &sess.Memory)
		json.Unmarshal(stateJSON, &sess.State)
		json.Unmarshal(metadataJSON, &sess.Metadata)
		sessions = append(sessions, sess)
	}
	return sessions, nil
}
