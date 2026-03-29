package agnogo

import (
	"context"
	"sync"
)

// Storage persists agent sessions between turns.
// Matches Agno's BaseDb pattern: upsert_session, get_session, delete_session, get_sessions.
type Storage interface {
	Load(ctx context.Context, sessionID string) (*Session, error)
	Save(ctx context.Context, session *Session) error
	Delete(ctx context.Context, sessionID string) error
	List(ctx context.Context, limit int) ([]*Session, error)
}

// Knowledge can also be managed through storage.
// Matches Agno's add_to_knowledge pattern.
type KnowledgeStore interface {
	AddKnowledge(ctx context.Context, key, content string) error
	DeleteKnowledge(ctx context.Context, key string) error
	ListKnowledge(ctx context.Context) ([]KnowledgeEntry, error)
}

// KnowledgeEntry is a stored knowledge item.
type KnowledgeEntry struct {
	Key     string `json:"key"`
	Content string `json:"content"`
}

// MemoryStorage is an in-memory storage for testing. Thread-safe.
type MemoryStorage struct {
	mu        sync.RWMutex
	sessions  map[string]*Session
	knowledge map[string]string
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		sessions:  make(map[string]*Session),
		knowledge: make(map[string]string),
	}
}

func (s *MemoryStorage) Load(_ context.Context, sessionID string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return sess, nil
}

func (s *MemoryStorage) Save(_ context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
	return nil
}

func (s *MemoryStorage) Delete(_ context.Context, sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
	return nil
}

func (s *MemoryStorage) List(_ context.Context, limit int) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*Session
	for _, sess := range s.sessions {
		result = append(result, sess)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

// KnowledgeStore methods on MemoryStorage
func (s *MemoryStorage) AddKnowledge(_ context.Context, key, content string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.knowledge[key] = content
	return nil
}

func (s *MemoryStorage) DeleteKnowledge(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.knowledge, key)
	return nil
}

func (s *MemoryStorage) ListKnowledge(_ context.Context) ([]KnowledgeEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var entries []KnowledgeEntry
	for k, v := range s.knowledge {
		entries = append(entries, KnowledgeEntry{Key: k, Content: v})
	}
	return entries, nil
}
