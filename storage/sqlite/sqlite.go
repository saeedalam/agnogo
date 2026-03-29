// Package sqlite provides SQLite session storage for agnogo.
//
//	import "github.com/saeedalam/agnogo/storage/sqlite"
//	store := sqlite.New("agent.db")
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/saeedalam/agnogo"
)

// Storage implements agnogo.Storage using SQLite.
type Storage struct {
	db    *sql.DB
	table string
}

// New creates a SQLite storage. Creates the table if not exists.
// Requires a SQLite driver like modernc.org/sqlite or mattn/go-sqlite3.
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
		user_id TEXT DEFAULT '',
		channel TEXT DEFAULT '',
		history TEXT DEFAULT '[]',
		memory TEXT DEFAULT '{}',
		state TEXT DEFAULT '{}',
		metadata TEXT DEFAULT '{}',
		created_at TEXT DEFAULT '',
		updated_at TEXT DEFAULT ''
	)`, s.table)
	_, err := s.db.Exec(q)
	return err
}

func (s *Storage) Load(ctx context.Context, sessionID string) (*agnogo.Session, error) {
	q := fmt.Sprintf("SELECT user_id, channel, history, memory, state, metadata, created_at, updated_at FROM %s WHERE id = ?", s.table)
	row := s.db.QueryRowContext(ctx, q, sessionID)

	var userID, channel, historyJSON, memoryJSON, stateJSON, metadataJSON, createdAt, updatedAt string
	if err := row.Scan(&userID, &channel, &historyJSON, &memoryJSON, &stateJSON, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, agnogo.ErrSessionNotFound
	}

	sess := agnogo.NewSession(sessionID)
	sess.UserID = userID
	sess.Channel = channel
	json.Unmarshal([]byte(historyJSON), &sess.History)
	json.Unmarshal([]byte(memoryJSON), &sess.Memory)
	json.Unmarshal([]byte(stateJSON), &sess.State)
	json.Unmarshal([]byte(metadataJSON), &sess.Metadata)
	sess.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	sess.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return sess, nil
}

func (s *Storage) Save(ctx context.Context, sess *agnogo.Session) error {
	historyJSON, _ := json.Marshal(sess.History)
	memoryJSON, _ := json.Marshal(sess.Memory)
	stateJSON, _ := json.Marshal(sess.State)
	metadataJSON, _ := json.Marshal(sess.Metadata)

	q := fmt.Sprintf(`INSERT INTO %s (id, user_id, channel, history, memory, state, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			user_id=excluded.user_id, channel=excluded.channel,
			history=excluded.history, memory=excluded.memory,
			state=excluded.state, metadata=excluded.metadata,
			updated_at=excluded.updated_at`, s.table)

	_, err := s.db.ExecContext(ctx, q,
		sess.ID, sess.UserID, sess.Channel,
		string(historyJSON), string(memoryJSON), string(stateJSON), string(metadataJSON),
		sess.CreatedAt.Format(time.RFC3339), time.Now().Format(time.RFC3339),
	)
	return err
}
