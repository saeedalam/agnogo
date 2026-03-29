package agnogo

import "context"

// Storage persists agent sessions between turns.
// Implement for your database (Postgres, SQLite, Redis, etc.)
type Storage interface {
	Load(ctx context.Context, sessionID string) (*Session, error)
	Save(ctx context.Context, session *Session) error
}

// MemoryStorage is an in-memory storage for testing.
type MemoryStorage struct {
	sessions map[string]*Session
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{sessions: make(map[string]*Session)}
}

func (s *MemoryStorage) Load(_ context.Context, sessionID string) (*Session, error) {
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return sess, nil
}

func (s *MemoryStorage) Save(_ context.Context, session *Session) error {
	s.sessions[session.ID] = session
	return nil
}
