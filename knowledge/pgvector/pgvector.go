// Package pgvector provides a PostgreSQL pgvector Knowledge backend for agnogo.
//
//	import "github.com/saeedalam/agnogo/knowledge/pgvector"
//	kb := pgvector.New(pool, pgvector.Config{Table: "chunks", EmbedFunc: embedFn})
package pgvector

import (
	"context"
	"fmt"
	"strings"
)

// DB is a minimal database interface (pgxpool.Pool, sql.DB, etc.)
type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// Row is a minimal row scanner.
type Row interface {
	Scan(dest ...any) error
}

// EmbedFunc generates a vector embedding for a text query.
// Example: use OpenAI text-embedding-3-small.
type EmbedFunc func(ctx context.Context, text string) ([]float32, error)

// Config configures the pgvector knowledge backend.
type Config struct {
	Table       string    // table name (default "chunks")
	ContentCol  string    // content column (default "content")
	EmbeddingCol string   // embedding column (default "embedding")
	EmbedFunc   EmbedFunc // embedding function (required)
	Threshold   float64   // min cosine similarity (default 0.2)
}

// Knowledge implements agnogo.Knowledge using PostgreSQL pgvector.
type Knowledge struct {
	db  DB
	cfg Config
}

// New creates a pgvector knowledge backend.
func New(db DB, cfg Config) *Knowledge {
	if cfg.Table == "" { cfg.Table = "chunks" }
	if cfg.ContentCol == "" { cfg.ContentCol = "content" }
	if cfg.EmbeddingCol == "" { cfg.EmbeddingCol = "embedding" }
	if cfg.Threshold == 0 { cfg.Threshold = 0.2 }
	return &Knowledge{db: db, cfg: cfg}
}

func (k *Knowledge) Search(ctx context.Context, query string, limit int) (string, error) {
	if k.cfg.EmbedFunc == nil {
		return "", fmt.Errorf("embed function not configured")
	}

	embedding, err := k.cfg.EmbedFunc(ctx, query)
	if err != nil {
		return "", fmt.Errorf("embed: %w", err)
	}

	// Format embedding as pgvector literal
	parts := make([]string, len(embedding))
	for i, v := range embedding {
		parts[i] = fmt.Sprintf("%f", v)
	}
	vecStr := "[" + strings.Join(parts, ",") + "]"

	q := fmt.Sprintf(
		`SELECT %s FROM %s WHERE 1 - (%s <=> $1::vector) > $2 ORDER BY %s <=> $1::vector LIMIT $3`,
		k.cfg.ContentCol, k.cfg.Table, k.cfg.EmbeddingCol, k.cfg.EmbeddingCol,
	)

	// Note: this returns one row. For multiple results, use Query + loop.
	// Simplified for the SDK — users can implement Knowledge interface for complex cases.
	var content string
	err = k.db.QueryRow(ctx, q, vecStr, k.cfg.Threshold, limit).Scan(&content)
	if err != nil {
		return "", nil // no results is not an error
	}
	return content, nil
}
