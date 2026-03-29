// Package redis provides Redis session storage for agnogo.
// Uses plain HTTP calls to Redis — no external Redis client dependency.
// For production, you may want to use a proper Redis client (go-redis).
//
//	import "github.com/saeedalam/agnogo/storage/redis"
//	store := redis.New("localhost:6379")
package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/saeedalam/agnogo"
)

// Storage implements agnogo.Storage using Redis.
type Storage struct {
	addr   string
	prefix string // key prefix (default "agnogo:")
	ttl    time.Duration
}

// New creates a Redis storage.
func New(addr string, opts ...Option) *Storage {
	s := &Storage{addr: addr, prefix: "agnogo:", ttl: 24 * time.Hour}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Option configures Redis storage.
type Option func(*Storage)

// WithPrefix sets the key prefix.
func WithPrefix(p string) Option { return func(s *Storage) { s.prefix = p } }

// WithTTL sets session expiry time.
func WithTTL(d time.Duration) Option { return func(s *Storage) { s.ttl = d } }

func (s *Storage) key(sessionID string) string {
	return s.prefix + sessionID
}

func (s *Storage) Load(ctx context.Context, sessionID string) (*agnogo.Session, error) {
	data, err := s.get(ctx, s.key(sessionID))
	if err != nil || data == "" {
		return nil, agnogo.ErrSessionNotFound
	}
	sess := agnogo.NewSession(sessionID)
	if err := json.Unmarshal([]byte(data), sess); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	return sess, nil
}

func (s *Storage) Save(ctx context.Context, sess *agnogo.Session) error {
	data, err := json.Marshal(sess)
	if err != nil {
		return err
	}
	return s.set(ctx, s.key(sess.ID), string(data), s.ttl)
}

// Simple Redis protocol (RESP) client — no external dependency
func (s *Storage) get(ctx context.Context, key string) (string, error) {
	return s.command(ctx, fmt.Sprintf("*2\r\n$3\r\nGET\r\n$%d\r\n%s\r\n", len(key), key))
}

func (s *Storage) set(ctx context.Context, key, value string, ttl time.Duration) error {
	ttlSec := int(ttl.Seconds())
	cmd := fmt.Sprintf("*5\r\n$3\r\nSET\r\n$%d\r\n%s\r\n$%d\r\n%s\r\n$2\r\nEX\r\n$%d\r\n%d\r\n",
		len(key), key, len(value), value, len(fmt.Sprint(ttlSec)), ttlSec)
	_, err := s.command(ctx, cmd)
	return err
}

func (s *Storage) command(ctx context.Context, cmd string) (string, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return "", fmt.Errorf("redis connect: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	conn.Write([]byte(cmd))
	buf := make([]byte, 65536)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}
	resp := string(buf[:n])
	// Parse simple RESP response
	if len(resp) > 0 && resp[0] == '$' {
		// Bulk string
		lines := splitResp(resp)
		if len(lines) >= 2 {
			return lines[1], nil
		}
	}
	if len(resp) > 0 && resp[0] == '-' {
		return "", fmt.Errorf("redis error: %s", resp[1:])
	}
	return resp, nil
}

func splitResp(s string) []string {
	var parts []string
	current := ""
	for i := 0; i < len(s); i++ {
		if i+1 < len(s) && s[i] == '\r' && s[i+1] == '\n' {
			parts = append(parts, current)
			current = ""
			i++ // skip \n
		} else {
			current += string(s[i])
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
