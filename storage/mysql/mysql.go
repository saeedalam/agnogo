// Package mysql provides MySQL session storage for agnogo.
//
//	import "github.com/saeedalam/agnogo/storage/mysql"
//	store, _ := mysql.New(db)
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/saeedalam/agnogo"
)

// Storage implements agnogo.Storage using MySQL.
type Storage struct {
	db    *sql.DB
	table string
}

// New creates MySQL storage. Creates table if not exists.
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
		id VARCHAR(255) PRIMARY KEY,
		user_id VARCHAR(255) DEFAULT '',
		channel VARCHAR(50) DEFAULT '',
		history JSON,
		memory JSON,
		state JSON,
		metadata JSON,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
	)`, s.table)
	_, err := s.db.Exec(q)
	return err
}

func (s *Storage) Load(ctx context.Context, sessionID string) (*agnogo.Session, error) {
	q := fmt.Sprintf("SELECT user_id, channel, history, memory, state, metadata, created_at, updated_at FROM %s WHERE id = ?", s.table)
	row := s.db.QueryRowContext(ctx, q, sessionID)

	var userID, channel string
	var historyJSON, memoryJSON, stateJSON, metadataJSON []byte
	var createdAt, updatedAt time.Time
	if err := row.Scan(&userID, &channel, &historyJSON, &memoryJSON, &stateJSON, &metadataJSON, &createdAt, &updatedAt); err != nil {
		return nil, agnogo.ErrSessionNotFound
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
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			user_id=VALUES(user_id), channel=VALUES(channel),
			history=VALUES(history), memory=VALUES(memory),
			state=VALUES(state), metadata=VALUES(metadata)`, s.table)

	_, err := s.db.ExecContext(ctx, q,
		sess.ID, sess.UserID, sess.Channel,
		historyJSON, memoryJSON, stateJSON, metadataJSON,
	)
	return err
}
